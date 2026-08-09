// Package oscmd runs helper programs without letting a console window appear.
//
// The agent is a GUI process: it owns no console of its own, so on Windows every
// helper it starts gets a brand new one allocated — and since printer
// enumeration shells out to PowerShell every few seconds, that is a console
// window flashing over whatever the operator is doing, all day. Building every
// command here keeps the "no window" decision in one platform-specific place
// instead of a flag each call site has to remember, and gives PowerShell a
// single canonical invocation.
package oscmd

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Command is exec.CommandContext with the platform's "no console window"
// attributes applied. It is a plain exec.Cmd everywhere but Windows.
func Command(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	hide(cmd)
	return cmd
}

// PowerShell builds a command that runs script through Windows PowerShell.
// -NoProfile keeps a user's profile script from printing banners into output we
// parse; -NonInteractive makes anything that wants a prompt fail fast instead of
// hanging a GUI process that can never answer it.
func PowerShell(ctx context.Context, script string) *exec.Cmd {
	return Command(ctx, powerShell(), "-NoProfile", "-NonInteractive", "-Command", script)
}

// powerShell resolves powershell.exe by absolute path where it can: a process
// launched from Explorer or a shortcut inherits whatever PATH the session has,
// and a PATH without System32 is a common way for a bare "powershell" to fail to
// resolve at all.
func powerShell() string {
	if root := os.Getenv("SystemRoot"); root != "" {
		p := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "powershell"
}

// Quote renders s as a PowerShell single-quoted literal, where the only escape
// is a doubled quote. Go's %q is the wrong tool for this: it escapes the
// backslashes in a Windows path, and PowerShell (which treats backslash as an
// ordinary character) would then be handed "C:\\Users\\…" — a different string
// than the one we meant.
func Quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
