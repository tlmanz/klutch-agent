package agent

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/tlmanz/klutch-agent/internal/oscmd"
)

// dispatch hands a spooled file to the OS print system, targeting the named
// queue. It does NO content transformation: our CUPS queues are raw, so PDF and
// ESC/POS-raster payloads both pass through unchanged. It returns the OS job
// handle (CUPS request id) when one is available so the job can be tracked and
// controlled; on Windows the PrintTo verb yields no handle and it returns "".
func dispatch(ctx context.Context, printer, path string) (reqID string, err error) {
	return dispatchFile(ctx, printer, path, nil)
}

// dispatchFile is dispatch with extra spooler options (`lp -o fit-to-page`,
// `-n <copies>`, …), used by local printing where the operator chooses them.
// Windows has no equivalent for the PrintTo verb, so they are ignored there.
func dispatchFile(ctx context.Context, printer, path string, opts []string) (reqID string, err error) {
	if strings.TrimSpace(printer) == "" {
		return "", fmt.Errorf("job has no target printer")
	}
	switch runtime.GOOS {
	case "windows":
		return "", dispatchWindows(ctx, printer, path)
	default:
		return dispatchCUPS(ctx, printer, path, opts)
	}
}

// dispatchCUPS submits the file to a CUPS queue via `lp -d <printer> <path>`
// (Linux/macOS) and parses the request id from lp's output ("request id is
// Printer-42 (1 file(s))"). A raw queue forwards the bytes verbatim.
func dispatchCUPS(ctx context.Context, printer, path string, opts []string) (string, error) {
	args := append([]string{"-d", printer}, opts...)
	args = append(args, path)
	out, err := exec.CommandContext(ctx, "lp", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lp: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return parseRequestID(string(out)), nil
}

// parseRequestID pulls the "request id is X" token out of lp's stdout.
func parseRequestID(out string) string {
	if _, after, ok := strings.Cut(out, "request id is "); ok {
		if f := strings.Fields(after); len(f) > 0 {
			return f[0]
		}
	}
	return ""
}

// rawPayloads are the spool-file kinds that are already written in the printer's
// own language, because the backend (or the agent's own ESC/POS encoder)
// rendered them. Nothing may interpret these bytes on the way to the device.
var rawPayloads = map[string]bool{
	".escpos": true, // ESC/POS receipt raster, produced by internal/imaging
	".bin":    true, // any payload kind the backend did not label
	".prn":    true,
	".raw":    true,
	".zpl":    true,
	".epl":    true,
	".cpcl":   true,
}

// dispatchWindows submits the file to a named Windows printer.
//
// A raw payload goes straight into the spooler as a RAW job (printRaw). It
// cannot go through the shell: `Start-Process -Verb PrintTo` asks Windows for an
// application registered to print the file, and no application prints an ESC/POS
// byte stream — the shell answers "No application is associated with the
// specified file for this operation" and the receipt never prints.
//
// Anything that still needs laying out (a PDF, an image) does go through
// PrintTo, which hands it to the application that owns the file type; -Wait
// blocks until it is submitted so a failure surfaces as a real dispatch error.
// Both arguments are single-quoted for PowerShell rather than %q-escaped: the
// spool path is a Windows path, and %q would double every backslash in it.
func dispatchWindows(ctx context.Context, printer, path string) error {
	if rawPayloads[strings.ToLower(filepath.Ext(path))] {
		return printRaw(printer, path, docName(path))
	}
	ps := fmt.Sprintf("Start-Process -FilePath %s -Verb PrintTo -ArgumentList %s -Wait",
		oscmd.Quote(path), oscmd.Quote(printer))
	out, err := oscmd.PowerShell(ctx, ps).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", windowsPrintError(tidyPSError(string(out), err), path))
	}
	return nil
}

// windowsPrintError explains the one failure an operator can do something about:
// Windows has no application registered to print this file type without opening
// it, so the shell refuses before the spooler is ever reached.
func windowsPrintError(msg, path string) string {
	if strings.Contains(strings.ToLower(msg), "no application is associated") {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
		if ext == "" {
			ext = "this kind of"
		}
		return "Windows has no application registered to print " + ext +
			" files without opening them. Install one that supports \"Print to\" (Adobe Acrobat Reader for PDFs), " +
			"or use a receipt printer, whose jobs the agent sends to the spooler directly."
	}
	return "powershell print: " + msg
}

// --- job controls (best-effort, CUPS only) ----------------------------------

// cupsHold pauses a queued/printing job (`lp -i <req> -H hold`).
func cupsHold(ctx context.Context, reqID string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("pause not supported on Windows")
	}
	return run(ctx, "lp", "-i", reqID, "-H", "hold")
}

// cupsRelease resumes a held job (`lp -i <req> -H resume`).
func cupsRelease(ctx context.Context, reqID string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("resume not supported on Windows")
	}
	return run(ctx, "lp", "-i", reqID, "-H", "resume")
}

// cupsCancel removes a job from the queue (`cancel <req>`).
func cupsCancel(ctx context.Context, reqID string) error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("cancel not supported on Windows")
	}
	return run(ctx, "cancel", reqID)
}

// setDefaultPrinter pins the OS default destination.
func setDefaultPrinter(ctx context.Context, name string) error {
	switch runtime.GOOS {
	case "windows":
		out, err := oscmd.PowerShell(ctx,
			fmt.Sprintf("(New-Object -ComObject WScript.Network).SetDefaultPrinter(%s)", oscmd.Quote(name))).CombinedOutput()
		if err != nil {
			return fmt.Errorf("set default printer: %s", tidyPSError(string(out), err))
		}
		return nil
	default:
		return run(ctx, "lpoptions", "-d", name)
	}
}

// run executes a command, folding stderr into the error for diagnostics.
func run(ctx context.Context, name string, args ...string) error {
	out, err := oscmd.Command(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
