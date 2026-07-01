//go:build linux

package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Linux desktop integration follows the XDG Base Directory + Desktop Entry
// specs, all under the user's home (no root needed):
//   - binary → ~/.local/bin/klutch-agent
//   - icon   → ~/.local/share/icons/hicolor/256x256/apps/<id>.png
//   - entry  → ~/.local/share/applications/<id>.desktop
// so the app appears in the GNOME/KDE/… application menu.

func localBin() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "bin"), nil
}

func dataHome() (string, error) {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share"), nil
}

func installedPath() (string, error) {
	bin, err := localBin()
	if err != nil {
		return "", err
	}
	return filepath.Join(bin, "klutch-agent"), nil
}

func desktopEntryPath() (string, error) {
	share, err := dataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(share, "applications", appID+".desktop"), nil
}

func iconPath() (string, error) {
	share, err := dataHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(share, "icons", "hicolor", "256x256", "apps", appID+".png"), nil
}

func install() (string, error) {
	target, err := installedPath()
	if err != nil {
		return "", err
	}

	// Copy the running binary to the stable location (skip if we already are it).
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

	// Install the icon (best-effort: a missing icon must not fail the install).
	if ip, err := iconPath(); err == nil {
		if mkErr := os.MkdirAll(filepath.Dir(ip), 0o755); mkErr == nil {
			_ = os.WriteFile(ip, icon, 0o644)
		}
	}

	// Write the .desktop launcher entry.
	entry, err := desktopEntryPath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(entry), 0o755); err != nil {
		return "", err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=Klutch print agent
Exec=%s
Icon=%s
Terminal=false
Categories=Utility;Office;
StartupNotify=true
`, appName, target, appID)
	if err := os.WriteFile(entry, []byte(content), 0o644); err != nil {
		return "", err
	}

	// Refresh the menu database so the entry shows up promptly (best-effort).
	if dir := filepath.Dir(entry); dir != "" {
		if bin, err := exec.LookPath("update-desktop-database"); err == nil {
			_ = exec.Command(bin, dir).Run()
		}
	}
	return target, nil
}

func uninstall() error {
	entry, err := desktopEntryPath()
	if err != nil {
		return err
	}
	if err := os.Remove(entry); err != nil && !os.IsNotExist(err) {
		return err
	}
	if ip, err := iconPath(); err == nil {
		_ = os.Remove(ip)
	}
	// Remove the installed binary unless it is the one currently running.
	if target, err := installedPath(); err == nil {
		if cur, err := currentExe(); err != nil || cur != target {
			_ = os.Remove(target)
		}
	}
	if dir := filepath.Dir(entry); dir != "" {
		if bin, err := exec.LookPath("update-desktop-database"); err == nil {
			_ = exec.Command(bin, dir).Run()
		}
	}
	return nil
}

func isInstalled() (bool, error) {
	entry, err := desktopEntryPath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(entry)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}
