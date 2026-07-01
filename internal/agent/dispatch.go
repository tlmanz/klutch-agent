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
// ESC/POS-raster payloads both pass through unchanged. Returns an error if the OS
// did not accept the job so the caller ACKs a real failure, not a false success.
func dispatch(ctx context.Context, printer, path string) error {
	if strings.TrimSpace(printer) == "" {
		return fmt.Errorf("job has no target printer")
	}
	switch runtime.GOOS {
	case "windows":
		return dispatchWindows(ctx, printer, path)
	default:
		return dispatchCUPS(ctx, printer, path)
	}
}

// dispatchCUPS submits the file to a CUPS queue via `lp -d <printer> <path>`
// (Linux/macOS). A raw queue forwards the bytes verbatim.
func dispatchCUPS(ctx context.Context, printer, path string) error {
	out, err := exec.CommandContext(ctx, "lp", "-d", printer, path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("lp: %v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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
