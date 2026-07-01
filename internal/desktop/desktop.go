// Package desktop integrates the agent into the OS application launcher so the
// operator can reopen it after quitting. Where autostart makes the app run at
// login, desktop makes it *findable*: it copies the running binary to a stable
// per-user location and registers a launcher entry (Linux .desktop file in the
// applications menu, Windows Start-menu shortcut) pointing at it.
//
// This is the counterpart to the release installers (the Windows NSIS package):
// it lets a user who just downloaded and ran the raw binary turn it into a
// proper installed application, no installer required.
//
// Each platform uses its native mechanism (see desktop_<goos>.go):
//   - Linux:   ~/.local/bin/<exe> + ~/.local/share/applications/<id>.desktop
//   - Windows: %LOCALAPPDATA%\Programs\Klutch Agent\<exe> + a Start-menu .lnk
package desktop

import (
	_ "embed"

	"github.com/tlmanz/klutch-agent/internal/autostart"
)

// appID / appName reuse the autostart identifiers so the launcher entry, the
// login item, and the Fyne app all share one identity.
const (
	appID   = autostart.AppID
	appName = autostart.AppName
)

// icon is the app icon, installed alongside the launcher entry and reused by the
// UI for the window/tray so the installed app and the running app match.
//
//go:embed icon.png
var icon []byte

// IconBytes returns the embedded PNG app icon.
func IconBytes() []byte { return icon }

// Install copies the running binary to a stable per-user location and registers
// it in the OS application launcher. It is idempotent: re-running refreshes the
// entry. Returns the path the app was installed to.
func Install() (string, error) { return install() }

// Uninstall removes the launcher entry (and the installed copy when it is not
// the binary currently running). A no-op if nothing is installed.
func Uninstall() error { return uninstall() }

// IsInstalled reports whether a launcher entry currently exists.
func IsInstalled() (bool, error) { return isInstalled() }

// InstalledPath is where Install places the binary for this platform.
func InstalledPath() (string, error) { return installedPath() }
