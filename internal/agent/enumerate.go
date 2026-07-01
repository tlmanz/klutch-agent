package agent

import (
	"bufio"
	"context"
	"log"
	"os/exec"
	"runtime"
	"strings"

	"github.com/tlmanz/klutch-agent/wire"
)

// enumeratePrinters lists every printer connected to this PC. It queries the OS
// spooler (CUPS `lpstat` on Linux/macOS, `Get-Printer` on Windows) and reports
// names + descriptions only; classification (thermal vs document, paper width,
// color) is the operator's job in the dashboard. If enumeration finds nothing (or
// fails), it advertises a single fallback so the agent still appears online.
func enumeratePrinters(ctx context.Context, fallback string) []wire.Printer {
	var printers []wire.Printer
	switch runtime.GOOS {
	case "windows":
		printers = enumerateWindows(ctx)
	default:
		printers = enumerateCUPS(ctx)
	}
	if len(printers) == 0 {
		log.Printf("no OS printers found; advertising fallback %q", fallback)
		return []wire.Printer{{Name: fallback, Description: "fallback (no OS printers detected)"}}
	}
	return printers
}

// enumerateCUPS parses `lpstat -p` output for printer names and their make/model.
func enumerateCUPS(ctx context.Context) []wire.Printer {
	out, err := exec.CommandContext(ctx, "lpstat", "-p").Output()
	if err != nil {
		return nil
	}
	var printers []wire.Printer
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "printer" {
			printers = append(printers, wire.Printer{Name: fields[1], Description: describeCUPS(ctx, fields[1])})
		}
	}
	return printers
}

// describeCUPS returns a printer's make/model via `lpoptions -p`, best-effort.
func describeCUPS(ctx context.Context, name string) string {
	out, err := exec.CommandContext(ctx, "lpoptions", "-p", name).Output()
	if err != nil {
		return ""
	}
	for _, tok := range strings.Fields(string(out)) {
		if m, ok := strings.CutPrefix(tok, "printer-make-and-model="); ok {
			return strings.Trim(m, "'\"")
		}
	}
	return ""
}

// enumerateWindows lists printers via PowerShell Get-Printer.
func enumerateWindows(ctx context.Context) []wire.Printer {
	out, err := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command",
		"Get-Printer | ForEach-Object { \"$($_.Name)|$($_.DriverName)\" }").Output()
	if err != nil {
		return nil
	}
	var printers []wire.Printer
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		name, driver, _ := strings.Cut(line, "|")
		printers = append(printers, wire.Printer{Name: strings.TrimSpace(name), Description: strings.TrimSpace(driver)})
	}
	return printers
}
