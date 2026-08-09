//go:build windows

package oscmd

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hide keeps the child from being given a console. CREATE_NO_WINDOW is the flag
// that actually stops the allocation for a console program such as
// powershell.exe; HideWindow covers the GUI-subsystem case, where a window would
// be created and shown rather than a console.
func hide(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
