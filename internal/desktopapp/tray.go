package desktopapp

import (
	_ "embed"
	"runtime"

	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tlmanz/klutch-agent/internal/desktop"
)

// trayICO is used on Windows (systray wants ICO there); Linux/macOS use the PNG.
//
//go:embed icon.ico
var trayICO []byte

// runTray shows the system-tray menu (Open / Compact panel / Check for updates /
// Quit). It runs on its own goroutine and is defensive: a tray failure (e.g. a
// Linux desktop without a StatusNotifier host) must never take down the app -
// the window + single-instance relaunch remain the fallback.
func (a *App) runTray() {
	defer func() { _ = recover() }()
	systray.Run(a.trayReady, func() {})
}

func (a *App) trayReady() {
	if runtime.GOOS == "windows" {
		systray.SetIcon(trayICO)
	} else {
		systray.SetIcon(desktop.IconBytes())
	}
	systray.SetTitle("Klutch")
	systray.SetTooltip("Klutch Print Agent")

	mOpen := systray.AddMenuItem("Open", "Show the main window")
	mCompact := systray.AddMenuItem("Compact panel", "Show the compact panel")
	systray.AddSeparator()
	mCheck := systray.AddMenuItem("Check for updates", "Check for a newer version")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Quit", "Quit Klutch")

	mOpen.Click(func() { a.showMode("full") })
	mCompact.Click(func() { a.showMode("compact") })
	mCheck.Click(func() { go func() { _, _ = a.ag.CheckForUpdate(a.appCtx) }() })
	mQuit.Click(func() {
		if a.ctx != nil {
			wruntime.Quit(a.ctx)
		}
	})
}

// showMode surfaces the window and switches it between full and compact layouts.
func (a *App) showMode(mode string) {
	if a.ctx == nil {
		return
	}
	wruntime.EventsEmit(a.ctx, "mode", mode)
	wruntime.WindowShow(a.ctx)
	wruntime.WindowUnminimise(a.ctx)
}
