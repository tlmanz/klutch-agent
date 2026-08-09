//go:build !windows

package oscmd

import "os/exec"

// hide is a no-op: only Windows hands a child process a console of its own.
func hide(*exec.Cmd) {}
