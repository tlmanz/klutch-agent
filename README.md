# klutch-agent

Desktop **print agent** for [Klutch](https://klutch.lk), the workshop-management
SaaS for Sri Lankan repair centers. It runs on a shop PC, holds an authenticated
WebSocket to the Klutch backend, advertises the PC's printers, and prints the
jobs pushed down to it: PDFs for document printers and ESC/POS rasters for
thermal receipt printers.

The agent holds **no business logic**. The backend renders every document
(layout, tax, and Sinhala / Tamil / English text); the agent only transports
bytes to the OS spooler and reports status. It shares only wire DTOs with the
backend (never domain types), which is why it lives in its own repository and is
released and updated independently.

## Features

- **One self-updating binary.** No separate service scripts: the app installs
  itself as a login item, runs in the system tray, and updates itself from its
  GitHub Releases (checksum-verified, then relaunches).
- **UI** (Fyne, native, cross-platform):
  - **Printers** — everything detected on this PC and advertised to Klutch.
  - **Jobs** — history of every print job with status (OK / FAILED) and the
    error or document reference, kept in a local SQLite database.
  - **Updates** — current version, check now, install now, and an
    auto-update toggle.
  - **Settings** — backend server URL, re-enrollment, auto-update, and
    "start at login".
- **Local storage** — SQLite (pure-Go `modernc.org/sqlite`, no CGO for the DB)
  under the per-user data directory, so job history and the printer list survive
  restarts.
- **Device-token auth** — enrolls once with a one-time pairing code from the
  dashboard; the token is revocable, tenant + branch scoped, and print-only.

## Install

### Windows

Download **`klutch-agent-setup.exe`** from the
[latest release](../../releases/latest) and run it. The wizard installs the app,
adds Start-menu + desktop shortcuts, and starts it at login (minimised to the
tray). On first run it asks for the **backend server URL** and a **one-time
pairing code** (dashboard → Printers → enroll agent).

### Linux

Download `klutch-agent-linux-amd64` (or `-arm64`), then:

```bash
chmod +x klutch-agent-linux-amd64
./klutch-agent-linux-amd64            # first run: enter server + pairing code
```

Toggle **Settings → Start automatically when I log in** to add an XDG autostart
entry. CUPS must be installed so the agent can enumerate and print
(`sudo apt install cups`).

### macOS

Download `klutch-agent-darwin-arm64` (Apple silicon) or `-amd64` (Intel),
`chmod +x`, and run it. Enable "Start at login" from Settings.

## Usage

The window closes to the system tray and keeps printing in the background. The
tray menu offers **Open**, **Check for updates**, and **Quit**.

Run headless (no UI, e.g. an unattended box):

```bash
klutch-agent -headless -server https://api.example.com -pairing-code ABC123
```

### Configuration

Flags override environment variables. In-app **Settings** persist to the local
store and win on the next launch.

| Flag | Env | Default | Meaning |
|---|---|---|---|
| `-server` | `KLUTCH_AGENT_SERVER` | – | backend base URL |
| `-pairing-code` | `KLUTCH_AGENT_PAIRING_CODE` | – | one-time enrollment code |
| `-token` | `KLUTCH_AGENT_TOKEN` | – | pre-issued device token (provisioning; skips pairing) |
| `-data` | `KLUTCH_AGENT_DATA_DIR` | per-user config dir | SQLite DB + spool |
| `-update-url` | `KLUTCH_AGENT_UPDATE_URL` | releases manifest | self-update manifest URL |
| `-no-update` | – | off | disable self-update |
| `-tray` | – | off | start minimised to the tray |
| `-headless` | – | off | run without the GUI |
| `-version` | – | – | print version and exit |

Data lives under the OS per-user config directory (e.g.
`~/.config/klutch-agent` on Linux, `%AppData%\klutch-agent` on Windows).

## How self-update works

The agent periodically fetches a small `manifest.json` from its release channel:

```json
{
  "version": "v1.2.3",
  "artifacts": {
    "linux/amd64":   { "url": "https://github.com/tlmanz/klutch-agent/releases/download/v1.2.3/klutch-agent-linux-amd64", "sha256": "…" },
    "windows/amd64": { "url": ".../klutch-agent-windows-amd64.exe", "sha256": "…" }
  }
}
```

If a newer build exists for this OS/arch, it downloads the raw binary, verifies
the SHA-256, atomically replaces its own executable (rename-aside on Windows),
and relaunches. `manifest.json` is published as a release asset, so
`releases/latest/download/manifest.json` always points at the newest version.
Point `-update-url` elsewhere (any static host) to use a different channel.

## Build from source

Fyne needs CGO + system OpenGL/X11 headers.

```bash
make deps-linux     # Debian/Ubuntu: gcc, OpenGL + X11 headers
make build          # -> bin/klutch-agent
make run            # build and run the GUI
make test           # tests
```

## Release

Push a tag; [`.github/workflows/release.yml`](.github/workflows/release.yml)
builds every platform **natively** (no cross-compile), builds the Windows NSIS
installer, generates `manifest.json`, and publishes a GitHub Release:

```bash
git tag v1.2.3 && git push origin v1.2.3
```

## Layout

```
main.go                 entry: flags, headless vs GUI, wiring
internal/agent/         connection loop, enumerate, dispatch, self-update (UI-agnostic)
internal/store/         SQLite: jobs, printers, settings
internal/ui/            Fyne UI: tabs, tray, first-run wizard
internal/autostart/     login autostart (Linux .desktop / Windows registry / macOS plist)
wire/                   WebSocket DTOs shared with the backend (synced copy)
installer/windows/      NSIS wizard installer script
scripts/gen-manifest.sh self-update manifest generator
```

The `wire` package is a byte-compatible copy of the backend's
`internal/printing/wire`; change both sides together.
