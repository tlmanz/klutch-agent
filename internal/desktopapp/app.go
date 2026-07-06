// Package desktopapp is the Wails (webview) front-end for the print agent. It
// wraps the UI-agnostic *agent.Agent: the React frontend calls the exported App
// methods (auto-bound to TS) and listens for "state" events emitted whenever the
// agent's snapshot changes. It replaces the former Fyne UI; the agent core,
// store, self-update, tray, autostart and desktop-install packages are reused
// unchanged.
package desktopapp

import (
	"context"
	"embed"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tlmanz/klutch-agent/internal/agent"
	"github.com/tlmanz/klutch-agent/internal/store"
)

// App is the bound object exposed to the frontend. Its exported methods become
// TypeScript bindings under frontend/wailsjs/go/desktopapp/App.
type App struct {
	appCtx context.Context // process lifetime (from main); bounds the event pump
	ctx    context.Context // Wails runtime context; set in startup
	ag     *agent.Agent

	// registerActivate hands the single-instance guard a callback that surfaces
	// the window when a second launch occurs.
	registerActivate func(func())
}

// Run boots the Wails application and blocks until it exits. assets is the
// embedded frontend/dist FS (declared in package main because go:embed cannot
// reach across parent directories). startHidden maps to the -tray flag.
func Run(ctx context.Context, ag *agent.Agent, assets embed.FS, startHidden bool, registerActivate func(func())) error {
	app := &App{appCtx: ctx, ag: ag, registerActivate: registerActivate}
	return wails.Run(&options.App{
		Title:             "Klutch Print Agent",
		Width:             1080,
		Height:            720,
		MinWidth:          940,
		MinHeight:         600,
		StartHidden:       startHidden,
		HideWindowOnClose: true, // close-to-tray: keep printing in the background
		AssetServer:       &assetserver.Options{Assets: assets},
		BackgroundColour:  &options.RGBA{R: 0x0c, G: 0x0f, B: 0x14, A: 1},
		OnStartup:         app.startup,
		Bind:              []any{app},
	})
}

// startup captures the runtime context, registers the activate hook, wires
// desktop notifications, starts the tray, and streams state to the frontend.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if a.registerActivate != nil {
		a.registerActivate(a.Activate)
	}
	a.ag.SetOutcomeHook(a.onOutcome)
	go a.pump()
	go a.runTray()
}

// notifyPayload is emitted to the frontend, which raises an HTML5 notification
// (Wails v2 has no cross-platform native notification API).
type notifyPayload struct {
	Kind    string `json:"kind"` // done | failed
	Doc     string `json:"doc"`
	Printer string `json:"printer"`
	Error   string `json:"error"`
}

// onOutcome fires for each terminal job; it emits a notify event gated by the
// user's preferences.
func (a *App) onOutcome(rec store.JobRecord) {
	if a.ctx == nil {
		return
	}
	st := a.ag.Snapshot()
	switch {
	case rec.Status == "ok" && st.NotifyDone:
		runtime.EventsEmit(a.ctx, "notify", notifyPayload{Kind: "done", Doc: docName(rec.DocumentRef), Printer: rec.Printer})
	case rec.Status == "failed" && st.NotifyFailed:
		runtime.EventsEmit(a.ctx, "notify", notifyPayload{Kind: "failed", Doc: docName(rec.DocumentRef), Error: rec.Error})
	}
}

// pump forwards agent-state changes to the frontend as "state" events, with a
// slow ticker as a backstop (mirrors the old Fyne refresh loop).
func (a *App) pump() {
	ch, cancel := a.ag.Subscribe()
	defer cancel()
	t := time.NewTicker(2 * time.Second)
	defer t.Stop()
	emit := func() { runtime.EventsEmit(a.ctx, "state", a.buildState()) }
	emit()
	for {
		select {
		case <-a.appCtx.Done():
			return
		case <-ch:
			emit()
		case <-t.C:
			emit()
		}
	}
}

// Activate surfaces the main window (called by the single-instance guard when a
// second launch is attempted). Safe once startup has run.
func (a *App) Activate() {
	if a.ctx == nil {
		return
	}
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// GetState returns the current snapshot (view DTO) for the initial render.
func (a *App) GetState() StateDTO { return a.buildState() }
