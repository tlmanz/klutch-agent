//go:build windows

package autostart

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// runKey is the per-user autostart registry key. Values here launch at login for
// the current user only (no admin rights needed).
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

func enable(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	cmd := `"` + exe + `"`
	if len(args) > 0 {
		cmd += " " + strings.Join(args, " ")
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue(AppName, cmd)
}

func disable() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return nil
		}
		return err
	}
	defer k.Close()
	if err := k.DeleteValue(AppName); err != nil && err != registry.ErrNotExist {
		return err
	}
	return nil
}

func isEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			return false, nil
		}
		return false, err
	}
	defer k.Close()
	_, _, err = k.GetStringValue(AppName)
	if err == registry.ErrNotExist {
		return false, nil
	}
	return err == nil, err
}
