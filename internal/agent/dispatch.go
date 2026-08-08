package agent

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
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

// dispatchWindows submits the file to a named Windows printer via PowerShell.
// PrintTo hands the file to the spooler for the named queue; -Wait blocks until
// it is submitted so an exec failure surfaces as a real dispatch error.
func dispatchWindows(ctx context.Context, printer, path string) error {
	ps := fmt.Sprintf("Start-Process -FilePath %q -Verb PrintTo -ArgumentList %q -Wait", path, printer)
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", ps).CombinedOutput()
	if err != nil {
		return fmt.Errorf("powershell print: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
		return run(ctx, "powershell", "-NoProfile", "-Command",
			fmt.Sprintf("(New-Object -ComObject WScript.Network).SetDefaultPrinter(%q)", name))
	default:
		return run(ctx, "lpoptions", "-d", name)
	}
}

// run executes a command, folding stderr into the error for diagnostics.
func run(ctx context.Context, name string, args ...string) error {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %v: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}
