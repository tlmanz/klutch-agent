package agent

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/tlmanz/klutch-agent/internal/oscmd"
)

// This file covers printer *administration*: discovering devices the OS can see
// but has no queue for (the usual reason a freshly plugged USB printer never
// shows up), creating a queue for one, and deleting a queue again. Enumeration
// (enumerate.go) only ever reports queues that already exist, so without this an
// operator had to leave the app and use the CUPS web UI / Windows settings.

// DeviceInfo is a printer device the OS can reach. Installed reports whether a
// queue already points at it, so the UI can grey those out instead of offering a
// duplicate.
type DeviceInfo struct {
	URI        string // backend device URI, e.g. usb://Printer/POS-80?serial=…
	Name       string // suggested queue name (already sanitised)
	Info       string // human label from the backend, e.g. "Printer POS-80"
	MakeModel  string
	Connection string // USB | Wi-Fi | Cloud
	Driver     string // suggested driver id ("raw" / "everywhere")
	Installed  bool
	Queue      string // existing queue name when Installed
}

// Driver ids offered to the UI. "raw" is the agent's normal choice: the backend
// renders (PDF / ESC-POS raster) and the queue must pass those bytes through
// untouched (see dispatch.go).
const (
	DriverRaw        = "raw"
	DriverEverywhere = "everywhere"
)

// DiscoverDevices lists every printer device the OS can see, installed or not.
func (a *Agent) DiscoverDevices(ctx context.Context) ([]DeviceInfo, error) {
	var (
		devs []DeviceInfo
		err  error
	)
	switch runtime.GOOS {
	case "windows":
		devs, err = discoverWindows(ctx)
	default:
		devs, err = discoverCUPS(ctx)
	}
	if err != nil {
		return nil, err
	}
	// Un-installed devices first (those are what the operator came here for),
	// then locally-attached before network, then by label.
	sort.SliceStable(devs, func(i, j int) bool {
		if devs[i].Installed != devs[j].Installed {
			return !devs[i].Installed
		}
		if (devs[i].Connection == "USB") != (devs[j].Connection == "USB") {
			return devs[i].Connection == "USB"
		}
		return devs[i].Info < devs[j].Info
	})
	return devs, nil
}

// AddPrinter creates a queue named name pointing at uri and refreshes state so
// the new printer appears immediately. driver may be empty, in which case the
// device's suggested driver is used. The backend learns about it on the next
// enumeration tick (watchPrinters re-advertises when the set changes).
func (a *Agent) AddPrinter(ctx context.Context, name, uri, driver string) error {
	name = strings.TrimSpace(name)
	if err := validQueueName(name); err != nil {
		return err
	}
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return fmt.Errorf("choose a printer device to add")
	}
	for _, p := range a.Snapshot().Printers {
		if strings.EqualFold(p.Name, name) {
			return fmt.Errorf("a printer named %q already exists", name)
		}
	}
	if driver == "" {
		driver = driverFor(uri, "")
	}

	var err error
	if runtime.GOOS == "windows" {
		err = addPrinterWindows(ctx, name, uri, driver)
	} else {
		err = addPrinterCUPS(ctx, name, uri, driver)
	}
	if err != nil {
		return err
	}
	a.log.Printf("added printer %q → %s (driver %s)", name, uri, driver)
	a.refreshPrinters(ctx)
	return nil
}

// RemovePrinter deletes a queue from the OS print system and forgets it locally
// (so it does not reappear from the store on the next launch).
func (a *Agent) RemovePrinter(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("no printer named")
	}
	if a.isPlaceholder(name) {
		return fmt.Errorf("%q is not a printer on this computer - it is a placeholder shown because none is set up", name)
	}
	var err error
	if runtime.GOOS == "windows" {
		err = psRun(ctx, fmt.Sprintf("Remove-Printer -Name %s -ErrorAction Stop", oscmd.Quote(name)))
	} else {
		err = adminRun(ctx, lpadminPath(), "-x", name)
	}
	if err != nil {
		return err
	}
	if derr := a.store.DeletePrinter(name); derr != nil {
		a.log.Printf("warning: forget printer %q: %v", name, derr)
	}
	a.log.Printf("removed printer %q", name)
	a.refreshPrinters(ctx)
	return nil
}

// isPlaceholder reports whether name is the stand-in row shown when this PC has
// no printers at all. Nothing that talks to the spooler may act on it: it has no
// queue behind it, and its stand-in name is not even a legal queue name, so the
// spooler's complaint ("printer name can only contain printable characters")
// describes a name we invented rather than anything the operator did.
func (a *Agent) isPlaceholder(name string) bool {
	for _, p := range a.Snapshot().Printers {
		if p.Name == name {
			return p.Placeholder
		}
	}
	return false
}

// refreshPrinters re-enumerates the OS queues into state right away, so the UI
// does not have to wait for the next watchPrinters tick after an add/remove.
func (a *Agent) refreshPrinters(ctx context.Context) {
	a.recordPrinters(enumeratePrinters(ctx, a.cfg.FallbackPrinter))
}

// --- CUPS (Linux / macOS) ----------------------------------------------------

// discoverCUPS parses `lpinfo -l -v`, which lists one block per device:
//
//	Device: uri = usb://Printer/POS-80?serial=936C0C663532
//	        class = direct
//	        info = Printer POS-80
//	        make-and-model = Printer POS-80
//	        device-id = MFG:Printer;MDL:POS-80;CMD:ESC/POS;…
func discoverCUPS(ctx context.Context) ([]DeviceInfo, error) {
	out, err := exec.CommandContext(ctx, "lpinfo", "-l", "-v").Output()
	if err != nil {
		return nil, fmt.Errorf("lpinfo: %w (is CUPS running?)", err)
	}
	installed := map[string]string{} // device URI → queue name
	for queue, uri := range cupsDevices(ctx) {
		installed[uri] = queue
	}
	return parseLPInfo(string(out), installed), nil
}

// parseLPInfo turns `lpinfo -l -v` output into devices, marking those an existing
// queue (installed: device URI → queue name) already points at.
func parseLPInfo(out string, installed map[string]string) []DeviceInfo {
	var (
		devs []DeviceInfo
		cur  *DeviceInfo
		id   string
	)
	flush := func() {
		if cur == nil {
			return
		}
		// Bare backend names ("socket", "hp") are schemes, not devices; only a
		// URI with a scheme separator addresses something real.
		if strings.Contains(cur.URI, ":") {
			cur.Driver = driverFor(cur.URI, id)
			cur.Connection = connKind(cur.URI)
			cur.Name = suggestName(cur.Info, cur.MakeModel, cur.URI)
			cur.Queue, cur.Installed = installed[cur.URI]
			devs = append(devs, *cur)
		}
		cur, id = nil, ""
	}
	sc := bufio.NewScanner(strings.NewReader(out))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, val, ok := strings.Cut(line, " = ")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch key {
		case "Device: uri":
			flush()
			cur = &DeviceInfo{URI: val}
		case "info":
			if cur != nil {
				cur.Info = val
			}
		case "make-and-model":
			if cur != nil && !strings.EqualFold(val, "unknown") {
				cur.MakeModel = val
			}
		case "device-id":
			id = val
		}
	}
	flush()
	return devs
}

// addPrinterCUPS creates and enables the queue. -E must come last: before -p it
// would mean "encrypt", after it means "enable and accept jobs".
func addPrinterCUPS(ctx context.Context, name, uri, driver string) error {
	args := []string{"-p", name, "-v", uri, "-m", driver}
	if desc := strings.TrimSpace(deviceLabel(uri)); desc != "" {
		args = append(args, "-D", desc)
	}
	args = append(args, "-E")
	return adminRun(ctx, lpadminPath(), args...)
}

// deviceLabel derives a short description from a device URI (usb://Printer/POS-80
// → "Printer POS-80"), used as the queue's printer-info.
func deviceLabel(uri string) string {
	rest := uri
	if _, after, ok := strings.Cut(uri, "://"); ok {
		rest = after
	}
	rest, _, _ = strings.Cut(rest, "?")
	rest = strings.ReplaceAll(strings.Trim(rest, "/"), "/", " ")
	return strings.TrimSpace(rest)
}

// driverFor picks a queue driver for a device. IPP devices speak a standard the
// driverless "everywhere" model handles; everything else gets a raw queue, which
// is what the agent wants anyway since the backend sends rendered bytes.
func driverFor(uri, deviceID string) string {
	scheme, _, _ := strings.Cut(uri, ":")
	switch strings.ToLower(scheme) {
	case "ipp", "ipps", "dnssd", "ippusb":
		// A device that advertises a page-description language of its own (ESC/POS
		// receipt printers, ZPL label printers) must never be driven by a raster
		// driver: raw is the only safe choice.
		if !rawLanguage(deviceID) {
			return DriverEverywhere
		}
	}
	return DriverRaw
}

// rawLanguage reports whether an IEEE-1284 device id advertises a command set we
// pass through verbatim.
func rawLanguage(deviceID string) bool {
	id := strings.ToUpper(deviceID)
	for _, cmd := range []string{"ESC/POS", "ESCPOS", "ZPL", "EPL", "CPCL", "STAR"} {
		if strings.Contains(id, cmd) {
			return true
		}
	}
	return false
}

// suggestName turns a device label into a queue name CUPS will accept.
func suggestName(info, makeModel, uri string) string {
	for _, cand := range []string{info, makeModel, deviceLabel(uri)} {
		if n := sanitizeQueueName(cand); n != "" {
			return n
		}
	}
	return "Printer"
}

// sanitizeQueueName maps a free-text label onto the CUPS name charset: printable
// ASCII without space, "/", "#" or "\"; we keep it stricter (word characters,
// "-", ".", "_") to stay safe on Windows too.
func sanitizeQueueName(s string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '.':
			b.WriteRune(r)
		case r == ' ' || r == '_':
			b.WriteRune('_')
		}
	}
	name := strings.Trim(b.String(), "_-.")
	if len(name) > 60 {
		name = strings.Trim(name[:60], "_-.")
	}
	return name
}

// validQueueName rejects names the spooler would refuse, with a message the UI
// can show as-is.
func validQueueName(name string) error {
	if name == "" {
		return fmt.Errorf("enter a name for the printer")
	}
	if len(name) > 127 {
		return fmt.Errorf("name is too long (127 characters max)")
	}
	if name != sanitizeQueueName(name) {
		return fmt.Errorf("use letters, numbers, dashes and underscores only (no spaces or /)")
	}
	return nil
}

// lpadminPath finds lpadmin, which lives in /usr/sbin on most distributions and
// so can be missing from the PATH of a desktop-launched process.
func lpadminPath() string {
	if p, err := exec.LookPath("lpadmin"); err == nil {
		return p
	}
	for _, p := range []string{"/usr/sbin/lpadmin", "/usr/bin/lpadmin", "/sbin/lpadmin"} {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "lpadmin"
}

// adminRun executes an administrative command. Stdin is left empty so a spooler
// that wants a password fails fast instead of hanging on a prompt; when the
// failure is an authorisation one we retry through pkexec, which raises the
// desktop's own authentication dialog.
func adminRun(ctx context.Context, name string, args ...string) error {
	out, err := oscmd.Command(ctx, name, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if runtime.GOOS == "linux" && needsAuth(msg) {
		if pk, lerr := exec.LookPath("pkexec"); lerr == nil {
			out2, err2 := oscmd.Command(ctx, pk, append([]string{name}, args...)...).CombinedOutput()
			if err2 == nil {
				return nil
			}
			msg = strings.TrimSpace(string(out2))
			err = err2
		}
	}
	if msg == "" {
		msg = err.Error()
	}
	return fmt.Errorf("%s: %s", filepath.Base(name), msg)
}

// needsAuth spots the spooler's "you are not allowed to do this" replies.
func needsAuth(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{"forbidden", "not authorized", "unauthorized", "authentication", "permission denied", "access denied"} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// --- Windows -----------------------------------------------------------------

// discoverWindows lists printer ports with no queue bound to them. Windows
// installs a driver for most USB printers by itself; what is left over is a port
// (USB001, a TCP/IP port, …) an operator still has to attach a queue to.
func discoverWindows(ctx context.Context) ([]DeviceInfo, error) {
	script := `$used = @(Get-Printer | ForEach-Object { $_.PortName }); ` +
		`Get-PrinterPort | ForEach-Object { "$($_.Name)|$($_.Description)|$(if ($used -contains $_.Name) { 'yes' } else { 'no' })" }`
	out, err := oscmd.PowerShell(ctx, script).Output()
	if err != nil {
		return nil, fmt.Errorf("Get-PrinterPort: %w", err)
	}
	var devs []DeviceInfo
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		cols := strings.Split(strings.TrimSpace(sc.Text()), "|")
		if len(cols) < 3 || cols[0] == "" {
			continue
		}
		port := strings.TrimSpace(cols[0])
		devs = append(devs, DeviceInfo{
			URI:        port,
			Name:       sanitizeQueueName(port),
			Info:       strings.TrimSpace(cols[1]),
			Connection: windowsConn(port),
			Driver:     DriverRaw,
			Installed:  cols[2] == "yes",
		})
	}
	return devs, nil
}

// winRawDriver is the Windows counterpart of a raw CUPS queue: it ships with the
// OS and passes bytes through untouched, which is what an ESC/POS or ZPL device
// needs since the backend has already rendered what it prints.
const winRawDriver = "Generic / Text Only"

// addPrinterWindows binds a queue to a port. Windows will not create a queue for
// a driver that is not *installed*, and shipping with the OS is not the same as
// being installed: on a machine where nothing has ever used it, Add-Printer
// fails with 0x80070705 ("the specified driver does not exist"). So install the
// driver first when it is missing — Add-PrinterDriver pulls it from the driver
// store, and naming ntprint.inf explicitly gets past a store that has not
// indexed it yet.
func addPrinterWindows(ctx context.Context, name, port, driver string) error {
	if driver == "" || driver == DriverRaw || driver == DriverEverywhere {
		driver = winRawDriver
	}
	script := fmt.Sprintf(`$driver = %s
if (-not (Get-PrinterDriver -Name $driver -ErrorAction SilentlyContinue)) {
  try { Add-PrinterDriver -Name $driver -ErrorAction Stop }
  catch { Add-PrinterDriver -Name $driver -InfPath (Join-Path $env:SystemRoot 'inf\ntprint.inf') -ErrorAction Stop }
}
Add-Printer -Name %s -DriverName $driver -PortName %s -ErrorAction Stop`,
		oscmd.Quote(driver), oscmd.Quote(name), oscmd.Quote(port))
	return psRun(ctx, script)
}

// psRun executes a printer-administration script. Creating a queue or installing
// a driver is an administrative act, and since the point-and-print hardening
// updates a standard user cannot do either; when the spooler says so we re-run
// the same script elevated, which raises Windows' own UAC prompt — the
// counterpart of the pkexec retry adminRun does on Linux.
func psRun(ctx context.Context, script string) error {
	out, err := oscmd.PowerShell(ctx, script).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := tidyPSError(string(out), err)
	if needsElevation(msg) {
		eout, eerr := runElevated(ctx, script)
		if eerr == nil {
			return nil
		}
		msg = tidyPSError(string(eout), eerr)
	}
	return fmt.Errorf("%s", windowsPrinterError(msg))
}

// runElevated re-runs script through a UAC-elevated PowerShell. The elevated
// process is a separate one launched by the shell, so its output cannot be piped
// back: the script writes any failure to a file we read afterwards, and the
// launcher forwards the child's exit code.
func runElevated(ctx context.Context, script string) ([]byte, error) {
	dir, err := os.MkdirTemp("", "klutch-printer")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	ps1 := filepath.Join(dir, "task.ps1")
	logf := filepath.Join(dir, "task.log")
	body := "$ErrorActionPreference = 'Stop'\ntry {\n" + script +
		"\n} catch {\n  $_ | Out-String | Set-Content -LiteralPath " + oscmd.Quote(logf) + "\n  exit 1\n}\nexit 0\n"
	if err := os.WriteFile(ps1, []byte(body), 0o600); err != nil {
		return nil, err
	}

	launch := fmt.Sprintf(
		`$p = Start-Process -FilePath 'powershell.exe' `+
			`-ArgumentList '-NoProfile','-NonInteractive','-ExecutionPolicy','Bypass','-File',%s `+
			`-Verb RunAs -WindowStyle Hidden -Wait -PassThru; exit $p.ExitCode`, oscmd.Quote(ps1))
	out, err := oscmd.PowerShell(ctx, launch).CombinedOutput()
	if err == nil {
		return nil, nil
	}
	// The elevated script's own error is the useful one; the launcher only ever
	// reports the non-zero exit code.
	if b, rerr := os.ReadFile(logf); rerr == nil && strings.TrimSpace(string(b)) != "" {
		return b, err
	}
	return out, err
}

// windowsPrinterError turns a spooler complaint into something an operator can
// act on. The raw text is a PowerShell exception dump — HRESULTs, a CimException
// class path, the offending source line — which is what the Add-printer dialog
// was pasting on screen verbatim.
func windowsPrinterError(msg string) string {
	l := strings.ToLower(msg)
	switch {
	case strings.Contains(l, "0x80070705"), strings.Contains(l, "specified driver does not exist"):
		return "Windows does not have the \"" + winRawDriver + "\" printer driver installed and it could not be added automatically. " +
			"Add it once from Settings > Bluetooth & devices > Printers & scanners > Add manually > \"Add a local printer\", then try again."
	case needsElevation(l):
		return "Adding a printer needs administrator rights. Approve the Windows prompt, or right-click Klutch Agent and choose \"Run as administrator\", then try again."
	case strings.Contains(l, "already exists"):
		return "A printer with that name already exists on this computer."
	case strings.Contains(l, "0x80070002"), strings.Contains(l, "cannot find"):
		return "Windows can no longer see that port. Click Rescan and pick the device again."
	case msg == "":
		return "the printer could not be added"
	}
	return msg
}

// needsElevation spots the several ways Windows says "you are not an admin".
func needsElevation(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{
		"access is denied", "access denied", "0x80070005",
		"requires elevation", "unauthorizedaccess", "administrator",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// tidyPSError collapses a PowerShell error dump to its first sentence, dropping
// the "At line:1 char:1 + …" source echo and the CategoryInfo/FullyQualifiedErrorId
// block that follow it — except for the HRESULT, which is the one token in there
// worth keeping because windowsPrinterError matches on it.
func tidyPSError(out string, err error) string {
	msg := strings.TrimSpace(out)
	if msg == "" {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	head := msg
	if i := strings.Index(head, "At line:"); i >= 0 {
		head = head[:i]
	}
	if hr := strings.Index(msg, "HRESULT "); hr >= 0 {
		code := strings.Fields(strings.ReplaceAll(msg[hr+len("HRESULT "):], ",", " "))
		if len(code) > 0 {
			head += " (" + code[0] + ")"
		}
	}
	return strings.Join(strings.Fields(head), " ")
}
