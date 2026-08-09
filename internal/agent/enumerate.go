package agent

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"github.com/tlmanz/klutch-agent/internal/oscmd"
	"github.com/tlmanz/klutch-agent/wire"
)

// PrinterInfo is the enriched, UI-facing view of a printer. wire.Printer stays
// the backend contract (name + description only); the extra fields here are
// local, OS-derived display data (status, connection, queue depth, …) that the
// redesigned UI shows but the backend does not need.
type PrinterInfo struct {
	Name        string
	Description string
	Status      string // online | idle | printing | error | offline
	StateReason string // human note, e.g. "Out of paper"
	Connection  string // Wi-Fi | USB | Cloud
	Location    string
	Queued      int
	Default     bool
	Raw         bool // pass-through queue: it has no driver, so it prints bytes verbatim
	// Placeholder marks the stand-in row shown when the OS reports no printers at
	// all. It is NOT a queue: it cannot be printed to, set default, or removed, and
	// treating it as one is how `lpadmin -x "Default Printer"` ended up being run
	// against a name the spooler will not even accept.
	Placeholder bool
}

// toWire projects the enriched set down to the backend contract, dropping the
// placeholder: it is a local hint that this PC has no printers, not a printer.
// Advertising it would put a queue in the dashboard that can be tagged and routed
// to, and every job sent there would fail at the counter with nothing to fix.
func toWire(ps []PrinterInfo) []wire.Printer {
	out := make([]wire.Printer, 0, len(ps))
	for _, p := range ps {
		if p.Placeholder {
			continue
		}
		out = append(out, wire.Printer{Name: p.Name, Description: p.Description})
	}
	return out
}

// enumeratePrinters lists every printer connected to this PC with its live OS
// status. It queries the spooler (CUPS `lpstat`/`lpoptions` on Linux/macOS,
// `Get-Printer` on Windows). If enumeration finds nothing (or fails), it
// advertises a single fallback so the agent still appears online.
func enumeratePrinters(ctx context.Context, fallback string) []PrinterInfo {
	var printers []PrinterInfo
	switch runtime.GOOS {
	case "windows":
		printers = enumerateWindows(ctx)
	default:
		printers = enumerateCUPS(ctx)
	}
	if len(printers) == 0 {
		log.Printf("no OS printers found; advertising fallback %q", fallback)
		return []PrinterInfo{{
			Name: fallback, Description: "No printer is set up on this computer",
			Status: "offline", Connection: "Wi-Fi", Placeholder: true,
		}}
	}
	return printers
}

// enumerateCUPS builds the enriched set from `lpstat`/`lpoptions`. It runs a
// handful of pooled queries (state, default, device URIs, queue) plus one
// per-printer options call, parsing data the old code discarded.
func enumerateCUPS(ctx context.Context) []PrinterInfo {
	out, err := exec.CommandContext(ctx, "lpstat", "-p").Output()
	if err != nil {
		return nil
	}
	defaultName := cupsDefault(ctx)
	devices := cupsDevices(ctx)
	queued := cupsQueueDepth(ctx)

	var printers []PrinterInfo
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "printer" {
			continue
		}
		name := fields[1]
		p := PrinterInfo{
			Name:       name,
			Status:     cupsState(line),
			Connection: connKind(devices[name]),
			Queued:     queued[name],
			Default:    name == defaultName,
		}
		model, location, reason, raw := describeCUPS(ctx, name)
		p.Description = model
		p.Location = location
		p.Raw = raw
		if reason != "" {
			p.Status = "error"
			p.StateReason = reason
		}
		// CUPS can leave a printer marked "processing" after its queue drains; if
		// nothing is actually queued, it is idle, not printing. This keeps the
		// Printers status consistent with the (empty) Jobs list.
		if p.Status == "printing" && p.Queued == 0 {
			p.Status = "online"
		}
		printers = append(printers, p)
	}
	return printers
}

// cupsState maps a `lpstat -p` status line to a UI state keyword.
func cupsState(line string) string {
	l := strings.ToLower(line)
	switch {
	case strings.Contains(l, "now printing"), strings.Contains(l, "processing"):
		return "printing"
	case strings.Contains(l, "disabled"), strings.Contains(l, "stopped"):
		return "offline"
	default:
		return "online"
	}
}

// describeCUPS returns a printer's make/model, location, a human-readable error
// note (from printer-state-reasons) and whether the queue is a raw pass-through
// one, via a single `lpoptions -p` call.
func describeCUPS(ctx context.Context, name string) (model, location, reason string, raw bool) {
	out, err := exec.CommandContext(ctx, "lpoptions", "-p", name).Output()
	if err != nil {
		return "", "", "", false
	}
	// lpoptions prints space-separated key=value tokens; values may be quoted and
	// contain spaces, so scan on `key=` prefixes rather than naive field splits.
	s := string(out)
	model = cupsOpt(s, "printer-make-and-model")
	// A raw queue (what the agent creates for pass-through devices) reports the
	// placeholder model "Local Raw Printer"; its printer-info carries the real
	// device label, so prefer that.
	raw = strings.Contains(strings.ToLower(model), "raw")
	if model == "" || raw {
		if info := cupsOpt(s, "printer-info"); info != "" {
			model = info
		}
	}
	location = cupsOpt(s, "printer-location")
	reason = reasonText(cupsOpt(s, "printer-state-reasons"))
	return model, location, reason, raw
}

// cupsOpt extracts a single `key=value` option value, honoring single quotes.
func cupsOpt(s, key string) string {
	idx := strings.Index(s, key+"=")
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key)+1:]
	if strings.HasPrefix(rest, "'") {
		if end := strings.Index(rest[1:], "'"); end >= 0 {
			return rest[1 : 1+end]
		}
	}
	if f := strings.Fields(rest); len(f) > 0 {
		return f[0]
	}
	return ""
}

// reasonText turns a CUPS state-reason token into a short human note, or "" when
// the printer is fine ("none").
func reasonText(reason string) string {
	reason = strings.TrimSuffix(reason, "-error")
	reason = strings.TrimSuffix(reason, "-warning")
	reason = strings.TrimSuffix(reason, "-report")
	switch reason {
	case "", "none":
		return ""
	case "media-empty", "media-needed":
		return "Out of paper"
	case "media-jam":
		return "Paper jam"
	case "toner-empty", "marker-supply-empty":
		return "Out of toner"
	case "toner-low", "marker-supply-low":
		return "Toner low"
	case "cover-open", "door-open":
		return "Cover open"
	case "offline", "connecting-to-device":
		return "Offline"
	default:
		return strings.ReplaceAll(reason, "-", " ")
	}
}

// cupsDefault returns the system default destination name, or "".
func cupsDefault(ctx context.Context) string {
	out, err := exec.CommandContext(ctx, "lpstat", "-d").Output()
	if err != nil {
		return ""
	}
	// "system default destination: NAME" or "no system default destination"
	if _, after, ok := strings.Cut(string(out), ": "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// cupsDevices maps each printer to its device URI via `lpstat -v`.
func cupsDevices(ctx context.Context) map[string]string {
	m := map[string]string{}
	out, err := exec.CommandContext(ctx, "lpstat", "-v").Output()
	if err != nil {
		return m
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		// "device for NAME: usb://…"
		line := sc.Text()
		rest, ok := strings.CutPrefix(line, "device for ")
		if !ok {
			continue
		}
		name, uri, ok := strings.Cut(rest, ": ")
		if ok {
			m[strings.TrimSpace(name)] = strings.TrimSpace(uri)
		}
	}
	return m
}

// cupsQueueDepth counts pending jobs per printer via `lpstat -o`.
func cupsQueueDepth(ctx context.Context) map[string]int {
	m := map[string]int{}
	out, err := exec.CommandContext(ctx, "lpstat", "-o").Output()
	if err != nil {
		return m
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		// Each line begins with a job id "PRINTER-NNN".
		f := strings.Fields(sc.Text())
		if len(f) == 0 {
			continue
		}
		if name := jobPrinter(f[0]); name != "" {
			m[name]++
		}
	}
	return m
}

// jobPrinter extracts the printer name from a CUPS job id "PRINTER-NNN".
func jobPrinter(jobID string) string {
	i := strings.LastIndex(jobID, "-")
	if i <= 0 {
		return ""
	}
	if _, err := strconv.Atoi(jobID[i+1:]); err != nil {
		return ""
	}
	return jobID[:i]
}

// connKind classifies a device URI into a UI connection label.
func connKind(uri string) string {
	scheme, _, _ := strings.Cut(uri, ":")
	switch strings.ToLower(scheme) {
	case "usb", "ippusb", "hp", "hpfax", "parallel", "serial":
		return "USB"
	case "ipps", "https":
		return "Cloud"
	default:
		if uri == "" {
			return "Wi-Fi"
		}
		return "Wi-Fi"
	}
}

// enumerateWindows lists printers via PowerShell Get-Printer, enriched with
// status, location and port. Queue depth and default flag are best-effort.
func enumerateWindows(ctx context.Context) []PrinterInfo {
	// Pipe-joined columns: Name|Driver|Status|Location|Port
	script := `Get-Printer | ForEach-Object { "$($_.Name)|$($_.DriverName)|$($_.PrinterStatus)|$($_.Location)|$($_.PortName)" }`
	out, err := oscmd.PowerShell(ctx, script).Output()
	if err != nil {
		return nil
	}
	defaultName := windowsDefault(ctx)
	var printers []PrinterInfo
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		cols := strings.Split(line, "|")
		for len(cols) < 5 {
			cols = append(cols, "")
		}
		name := strings.TrimSpace(cols[0])
		driver := strings.TrimSpace(cols[1])
		printers = append(printers, PrinterInfo{
			Name: name,
			// "Generic / Text Only" is what the agent binds for pass-through
			// devices on Windows, the counterpart of a raw CUPS queue.
			Raw:         strings.Contains(strings.ToLower(driver), "text only"),
			Description: driver,
			Status:      windowsState(cols[2]),
			Location:    strings.TrimSpace(cols[3]),
			Connection:  windowsConn(cols[4]),
			Default:     name == defaultName,
		})
	}
	return printers
}

func windowsState(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "printing", "processing":
		return "printing"
	case "offline", "paused", "stopped":
		return "offline"
	case "error", "paperjam", "paperout", "outofpaper", "tonerlow", "nopaper":
		return "error"
	default:
		return "online"
	}
}

func windowsConn(port string) string {
	p := strings.ToUpper(strings.TrimSpace(port))
	switch {
	case strings.HasPrefix(p, "USB"), strings.HasPrefix(p, "LPT"), strings.HasPrefix(p, "COM"):
		return "USB"
	case strings.HasPrefix(p, "WSD"), strings.HasPrefix(p, "IP_"), strings.HasPrefix(p, "192."), strings.Contains(p, "HTTPS"):
		return "Wi-Fi"
	default:
		return "Wi-Fi"
	}
}

func windowsDefault(ctx context.Context) string {
	out, err := oscmd.PowerShell(ctx,
		`(Get-CimInstance -Class Win32_Printer -Filter "Default=True").Name`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
