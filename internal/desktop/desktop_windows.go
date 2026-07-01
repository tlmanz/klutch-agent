//go:build windows

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Windows desktop integration installs into the per-user profile (no admin
// rights, unlike the NSIS package which targets Program Files):
//   - binary   → %LOCALAPPDATA%\Programs\Klutch Agent\klutch-agent.exe
//   - shortcut → %APPDATA%\Microsoft\Windows\Start Menu\Programs\Klutch Agent.lnk
// so the app is reachable from the Start menu and its search.

func installDir() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", fmt.Errorf("LOCALAPPDATA not set")
	}
	return filepath.Join(base, "Programs", appName), nil
}

func installedPath() (string, error) {
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "klutch-agent.exe"), nil
}

func shortcutPath() (string, error) {
	base := os.Getenv("APPDATA")
	if base == "" {
		return "", fmt.Errorf("APPDATA not set")
	}
	return filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs", appName+".lnk"), nil
}

func install() (string, error) {
	target, err := installedPath()
	if err != nil {
		return "", err
	}

	src, err := currentExe()
	if err != nil {
		return "", err
	}
	if src != target {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := copyFile(src, target, 0o755); err != nil {
			return "", fmt.Errorf("copy binary: %w", err)
		}
	}

	lnk, err := shortcutPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(lnk), 0o755); err != nil {
		return "", err
	}
	if err := createShortcut(lnk, target); err != nil {
		return "", fmt.Errorf("create Start-menu shortcut: %w", err)
	}
	return target, nil
}

// createShortcut writes a .lnk via the WScript.Shell COM object (present on all
// supported Windows), pointing at exe and taking its icon from the exe.
func createShortcut(lnk, exe string) error {
	ps := fmt.Sprintf(
		`$w = New-Object -ComObject WScript.Shell; `+
			`$s = $w.CreateShortcut(%q); `+
			`$s.TargetPath = %q; `+
			`$s.WorkingDirectory = %q; `+
			`$s.IconLocation = %q; `+
			`$s.Description = 'Klutch print agent'; `+
			`$s.Save()`,
		lnk, exe, filepath.Dir(exe), exe+",0")
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

func uninstall() error {
	lnk, err := shortcutPath()
	if err != nil {
		return err
	}
	if err := os.Remove(lnk); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Remove the installed binary unless it is the one currently running (a
	// running .exe is locked on Windows and cannot be deleted).
	if target, err := installedPath(); err == nil {
		if cur, err := currentExe(); err != nil || cur != target {
			_ = os.Remove(target)
		}
	}
	return nil
}

func isInstalled() (bool, error) {
	lnk, err := shortcutPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(lnk)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
