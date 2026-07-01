// Package autostart registers (or removes) the agent as a per-user login item so
// it starts automatically when the operator logs in. This is the agent's
// "always on" story for the Fyne desktop build: it runs in the user session
// rather than as a system service (a GUI app needs a desktop), so it comes up
// with the till's normal login. The Settings tab toggles it.
//
// Each platform uses its native mechanism (see autostart_<goos>.go):
//   - Linux:   an XDG ~/.config/autostart/<id>.desktop entry
//   - Windows: an HKCU\...\CurrentVersion\Run registry value
//   - macOS:   a ~/Library/LaunchAgents/<id>.plist LaunchAgent
package autostart

// AppID is the reverse-DNS identifier used for the autostart entry's filename /
// registry value, matching the Fyne app id.
const AppID = "lk.klutch.agent"

// AppName is the human-facing label shown by the OS where applicable.
const AppName = "Klutch Agent"

// Enable registers the current executable to launch at login with the given
// arguments (e.g. "-tray" so it starts minimized).
func Enable(args []string) error { return enable(args) }

// Disable removes the login entry. It is a no-op if none exists.
func Disable() error { return disable() }

// IsEnabled reports whether the login entry currently exists.
func IsEnabled() (bool, error) { return isEnabled() }
