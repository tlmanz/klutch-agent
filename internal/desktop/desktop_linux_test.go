//go:build linux

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallUninstall exercises the full launcher-integration round trip in an
// isolated HOME: install writes the .desktop entry, icon, and a binary copy;
// uninstall removes them.
func TestInstallUninstall(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	if ok, err := IsInstalled(); err != nil || ok {
		t.Fatalf("IsInstalled before install = %v, %v; want false, nil", ok, err)
	}

	target, err := Install()
	if err != nil {
		t.Fatalf("Install: %v", err)
	}

	// The binary was copied to ~/.local/bin.
	wantBin := filepath.Join(home, ".local", "bin", "klutch-agent")
	if target != wantBin {
		t.Errorf("installed path = %q; want %q", target, wantBin)
	}
	if _, err := os.Stat(wantBin); err != nil {
		t.Errorf("binary not copied: %v", err)
	}

	// The .desktop entry exists and points at the installed binary.
	entry := filepath.Join(home, ".local", "share", "applications", appID+".desktop")
	data, err := os.ReadFile(entry)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	if !strings.Contains(string(data), "Exec="+wantBin) {
		t.Errorf("desktop entry Exec does not point at %q:\n%s", wantBin, data)
	}
	if !strings.Contains(string(data), "Icon="+appID) {
		t.Errorf("desktop entry missing Icon=%s:\n%s", appID, data)
	}

	// The icon was installed.
	iconFile := filepath.Join(home, ".local", "share", "icons", "hicolor", "256x256", "apps", appID+".png")
	if _, err := os.Stat(iconFile); err != nil {
		t.Errorf("icon not installed: %v", err)
	}

	if ok, err := IsInstalled(); err != nil || !ok {
		t.Fatalf("IsInstalled after install = %v, %v; want true, nil", ok, err)
	}

	if err := Uninstall(); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Errorf("desktop entry still present after uninstall: %v", err)
	}
	if ok, _ := IsInstalled(); ok {
		t.Errorf("IsInstalled after uninstall = true; want false")
	}
}
