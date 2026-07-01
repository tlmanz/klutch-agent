// Package ui is the Fyne desktop front-end for the print agent. It observes the
// agent (Snapshot + Subscribe) and drives it through its exported methods; it
// holds no printing logic of its own. Closing the window hides the app to the
// system tray so it keeps printing in the background.
package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/tlmanz/klutch-agent/internal/agent"
	"github.com/tlmanz/klutch-agent/internal/autostart"
	"github.com/tlmanz/klutch-agent/internal/store"
	"github.com/tlmanz/klutch-agent/wire"
)

// UI holds the Fyne app, window, and the widgets refreshed from agent state.
type UI struct {
	ctx         context.Context
	app         fyne.App
	win         fyne.Window
	ag          *agent.Agent
	startHidden bool

	// header
	statusLabel  *widget.Label
	serverLabel  *widget.Label
	versionLabel *widget.Label

	// printers tab
	printers     []wire.Printer
	printerTable *widget.Table

	// jobs tab
	jobs      []store.JobRecord
	jobTable  *widget.Table
	jobsCount *widget.Label

	// settings tab
	serverEntry     *widget.Entry
	autoUpdateCheck *widget.Check
	autostartCheck  *widget.Check

	// updates tab
	updateStatus *widget.Label
	checkBtn     *widget.Button
	installBtn   *widget.Button
}

// New builds the UI around an agent. version is the running build's version.
func New(ctx context.Context, ag *agent.Agent) *UI {
	a := app.NewWithID(autostart.AppID)
	a.SetIcon(theme.ComputerIcon())
	w := a.NewWindow("Klutch Print Agent")
	w.Resize(fyne.NewSize(780, 540))

	u := &UI{ctx: ctx, app: a, win: w, ag: ag}
	u.build()
	return u
}

// SetStartHidden makes Run start with the window hidden (tray-only), used by the
// "-tray" autostart flag. It is ignored on first run (not enrolled), where the
// window is shown so the operator can complete enrollment.
func (u *UI) SetStartHidden(b bool) { u.startHidden = b }

// build assembles the window content, tray menu, and close-to-tray behaviour.
func (u *UI) build() {
	u.statusLabel = widget.NewLabel("")
	u.serverLabel = widget.NewLabel("")
	u.versionLabel = widget.NewLabel("")
	header := container.NewHBox(
		widget.NewLabelWithStyle("Status:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		u.statusLabel,
		widget.NewSeparator(),
		u.serverLabel,
		widget.NewSeparator(),
		u.versionLabel,
	)

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Printers", theme.ComputerIcon(), u.buildPrintersTab()),
		container.NewTabItemWithIcon("Jobs", theme.DocumentIcon(), u.buildJobsTab()),
		container.NewTabItemWithIcon("Updates", theme.DownloadIcon(), u.buildUpdatesTab()),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), u.buildSettingsTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	u.win.SetContent(container.NewBorder(container.NewVBox(header, widget.NewSeparator()), nil, nil, nil, tabs))

	// Close hides to tray so the agent keeps printing in the background.
	u.win.SetCloseIntercept(func() { u.win.Hide() })
	u.buildTray()
}

// buildTray adds a system-tray menu (open / check updates / quit) on desktops
// that support it.
func (u *UI) buildTray() {
	desk, ok := u.app.(desktop.App)
	if !ok {
		return
	}
	m := fyne.NewMenu("Klutch Agent",
		fyne.NewMenuItem("Open", func() { u.win.Show() }),
		fyne.NewMenuItem("Check for updates", func() { u.doCheck() }),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem("Quit", func() { u.app.Quit() }),
	)
	desk.SetSystemTrayMenu(m)
	desk.SetSystemTrayIcon(theme.ComputerIcon())
}

func (u *UI) buildPrintersTab() fyne.CanvasObject {
	u.printerTable = widget.NewTable(
		func() (int, int) { return len(u.printers), 2 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(u.printers) {
				l.SetText("")
				return
			}
			p := u.printers[id.Row]
			if id.Col == 0 {
				l.SetText(p.Name)
			} else {
				l.SetText(p.Description)
			}
		},
	)
	u.printerTable.ShowHeaderRow = true
	u.printerTable.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	u.printerTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := []string{"Printer", "Description"}
		o.(*widget.Label).SetText(h[id.Col])
	}
	u.printerTable.SetColumnWidth(0, 260)
	u.printerTable.SetColumnWidth(1, 460)

	help := widget.NewLabel("Printers detected on this PC and advertised to Klutch. Tag each one's type and paper width in the dashboard.")
	help.Wrapping = fyne.TextWrapWord
	return container.NewBorder(help, nil, nil, nil, u.printerTable)
}

func (u *UI) buildJobsTab() fyne.CanvasObject {
	u.jobsCount = widget.NewLabel("")
	u.jobTable = widget.NewTable(
		func() (int, int) { return len(u.jobs), 5 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			if id.Row >= len(u.jobs) {
				l.SetText("")
				return
			}
			j := u.jobs[id.Row]
			switch id.Col {
			case 0:
				l.SetText(j.FinishedAt.Format("2006-01-02 15:04:05"))
			case 1:
				l.SetText(j.Printer)
			case 2:
				l.SetText(j.Kind)
			case 3:
				if j.Status == "ok" {
					l.SetText("OK")
				} else {
					l.SetText("FAILED")
				}
			case 4:
				if j.Error != "" {
					l.SetText(j.Error)
				} else {
					l.SetText(j.DocumentRef)
				}
			}
		},
	)
	u.jobTable.ShowHeaderRow = true
	u.jobTable.CreateHeader = func() fyne.CanvasObject { return widget.NewLabel("") }
	u.jobTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := []string{"Time", "Printer", "Kind", "Status", "Detail"}
		o.(*widget.Label).SetText(h[id.Col])
	}
	u.jobTable.SetColumnWidth(0, 160)
	u.jobTable.SetColumnWidth(1, 160)
	u.jobTable.SetColumnWidth(2, 100)
	u.jobTable.SetColumnWidth(3, 80)
	u.jobTable.SetColumnWidth(4, 260)

	refresh := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() { go u.refresh() })
	top := container.NewHBox(u.jobsCount, widget.NewSeparator(), refresh)
	return container.NewBorder(top, nil, nil, nil, u.jobTable)
}

func (u *UI) buildUpdatesTab() fyne.CanvasObject {
	u.updateStatus = widget.NewLabel("")
	u.checkBtn = widget.NewButtonWithIcon("Check now", theme.SearchIcon(), func() { u.doCheck() })
	u.installBtn = widget.NewButtonWithIcon("Install update", theme.DownloadIcon(), func() { u.doInstall() })
	u.installBtn.Importance = widget.HighImportance
	u.installBtn.Disable()

	help := widget.NewLabel("The agent updates itself from its release channel. Enable automatic updates in Settings to install them without prompting.")
	help.Wrapping = fyne.TextWrapWord
	return container.NewVBox(
		u.updateStatus,
		container.NewHBox(u.checkBtn, u.installBtn),
		widget.NewSeparator(),
		help,
	)
}

func (u *UI) buildSettingsTab() fyne.CanvasObject {
	u.serverEntry = widget.NewEntry()
	saveServer := widget.NewButtonWithIcon("Save & reconnect", theme.ConfirmIcon(), func() {
		s := u.serverEntry.Text
		if s == "" {
			return
		}
		if err := u.ag.SetServer(s); err != nil {
			dialog.ShowError(err, u.win)
		}
	})

	reEnroll := widget.NewButtonWithIcon("Set up / reconnect", theme.LoginIcon(), func() { u.showEnrollDialog() })

	u.autoUpdateCheck = widget.NewCheck("Install updates automatically", func(b bool) {
		if err := u.ag.SetAutoUpdate(b); err != nil {
			dialog.ShowError(err, u.win)
		}
	})
	u.autostartCheck = widget.NewCheck("Start automatically when I log in", func(b bool) {
		var err error
		if b {
			err = autostart.Enable([]string{"-tray"})
		} else {
			err = autostart.Disable()
		}
		if err != nil {
			dialog.ShowError(err, u.win)
		}
	})

	form := widget.NewForm(
		widget.NewFormItem("Backend server", container.NewBorder(nil, nil, nil, saveServer, u.serverEntry)),
		widget.NewFormItem("Enrollment", reEnroll),
		widget.NewFormItem("Updates", u.autoUpdateCheck),
		widget.NewFormItem("Startup", u.autostartCheck),
	)
	return container.NewVScroll(form)
}

// Run wires up the refresh loop, shows the first-run enrollment wizard if needed,
// and blocks running the Fyne event loop until the app quits.
func (u *UI) Run() {
	// Seed autostart checkbox from the OS (best-effort).
	if on, err := autostart.IsEnabled(); err == nil {
		u.autostartCheck.SetChecked(on)
	}

	ch, cancel := u.ag.Subscribe()
	go func() {
		defer cancel()
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		u.refresh()
		for {
			select {
			case <-u.ctx.Done():
				return
			case <-ch:
				u.refresh()
			case <-t.C:
				u.refresh()
			}
		}
	}()

	// First-run: prompt for server + pairing code if not enrolled yet.
	enrolled := u.ag.Snapshot().Enrolled
	if !enrolled {
		go func() {
			time.Sleep(400 * time.Millisecond)
			fyne.Do(func() { u.showEnrollDialog() })
		}()
	}

	// Start hidden only when explicitly asked AND already enrolled; otherwise
	// show the window so the operator can act.
	if u.startHidden && enrolled {
		u.win.SetCloseIntercept(func() { u.win.Hide() })
		u.app.Run()
		return
	}
	u.win.ShowAndRun()
}

// refresh pulls the latest agent state + jobs and repaints the widgets. Safe to
// call from any goroutine; UI mutations are marshalled onto the main thread.
func (u *UI) refresh() {
	st := u.ag.Snapshot()
	jobs, _ := u.ag.RecentJobs(200)
	fyne.Do(func() {
		u.printers = st.Printers
		u.jobs = jobs

		status := "Disconnected"
		switch {
		case !st.Enrolled:
			status = "Not enrolled"
		case st.Connected:
			status = "Connected"
		}
		if st.LastError != "" && !st.Connected {
			status += " (" + st.LastError + ")"
		}
		u.statusLabel.SetText(status)
		u.serverLabel.SetText(st.Server)
		u.versionLabel.SetText("v" + trimV(st.Version))
		u.jobsCount.SetText(fmt.Sprintf("%d printed, %d failed", st.JobsOK, st.JobsFailed))

		if u.serverEntry.Text == "" {
			u.serverEntry.SetText(st.Server)
		}
		u.autoUpdateCheck.SetChecked(st.AutoUpdate)

		if st.AvailableVersion != "" {
			u.updateStatus.SetText("Update available: " + st.AvailableVersion + " (current " + st.Version + ")")
			u.installBtn.Enable()
		} else {
			last := "never"
			if !st.LastCheck.IsZero() {
				last = st.LastCheck.Format("2006-01-02 15:04")
			}
			u.updateStatus.SetText(fmt.Sprintf("Up to date (%s). Last checked: %s", st.Version, last))
			u.installBtn.Disable()
		}

		u.printerTable.Refresh()
		u.jobTable.Refresh()
	})
}

// doCheck runs a manual update check with a progress dialog.
func (u *UI) doCheck() {
	prog := dialog.NewCustomWithoutButtons("Checking for updates", widget.NewProgressBarInfinite(), u.win)
	prog.Show()
	go func() {
		avail, err := u.ag.CheckForUpdate(u.ctx)
		fyne.Do(func() {
			prog.Hide()
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			if avail == "" {
				dialog.ShowInformation("Up to date", "You have the latest version.", u.win)
			} else {
				dialog.ShowInformation("Update available", "Version "+avail+" is available. Install it from the Updates tab.", u.win)
			}
		})
		u.refresh()
	}()
}

// doInstall downloads and applies an update (the process relaunches on success).
func (u *UI) doInstall() {
	dialog.ShowConfirm("Install update", "The agent will update and restart. Continue?", func(ok bool) {
		if !ok {
			return
		}
		prog := dialog.NewCustomWithoutButtons("Installing update", widget.NewProgressBarInfinite(), u.win)
		prog.Show()
		go func() {
			err := u.ag.ApplyUpdate(u.ctx) // on success this relaunches and does not return
			fyne.Do(func() {
				prog.Hide()
				if err != nil {
					dialog.ShowError(err, u.win)
				}
			})
		}()
	}, u.win)
}

// showEnrollDialog is the setup wizard: it collects the backend URL and connects
// this agent either by redeeming a one-time pairing code (the usual flow) or by
// pasting a pre-issued device token (provisioning). No command line needed.
func (u *UI) showEnrollDialog() {
	st := u.ag.Snapshot()

	server := widget.NewEntry()
	server.SetText(st.Server)
	server.SetPlaceHolder("https://api.example.com")

	code := widget.NewEntry()
	code.SetPlaceHolder("one-time pairing code from the dashboard")
	token := widget.NewMultiLineEntry()
	token.SetPlaceHolder("paste the device token")
	token.Wrapping = fyne.TextWrapBreak

	// Two connection methods; only the selected one's field is shown.
	codeRow := container.NewVBox(widget.NewLabel("Pairing code"), code)
	tokenRow := container.NewVBox(widget.NewLabel("Device token"), token)
	tokenRow.Hide()
	mode := widget.NewRadioGroup([]string{"Pairing code", "Device token"}, func(sel string) {
		if sel == "Device token" {
			codeRow.Hide()
			tokenRow.Show()
		} else {
			tokenRow.Hide()
			codeRow.Show()
		}
	})
	mode.SetSelected("Pairing code")

	content := container.NewVBox(
		widget.NewLabel("Backend server"), server,
		widget.NewLabel("Connect using"), mode,
		codeRow, tokenRow,
	)

	d := dialog.NewCustomConfirm("Set up this agent", "Connect", "Cancel", content, func(ok bool) {
		if !ok {
			return
		}
		useToken := mode.Selected == "Device token"
		if useToken && token.Text == "" {
			dialog.ShowError(fmt.Errorf("enter a device token"), u.win)
			return
		}
		if !useToken && code.Text == "" {
			dialog.ShowError(fmt.Errorf("enter a pairing code"), u.win)
			return
		}
		prog := dialog.NewCustomWithoutButtons("Connecting", widget.NewProgressBarInfinite(), u.win)
		prog.Show()
		go func() {
			if server.Text != "" && server.Text != st.Server {
				_ = u.ag.SetServer(server.Text)
			}
			var err error
			if useToken {
				err = u.ag.SetToken(strings.TrimSpace(token.Text))
			} else {
				err = u.ag.Enroll(u.ctx, code.Text)
			}
			fyne.Do(func() {
				prog.Hide()
				if err != nil {
					dialog.ShowError(err, u.win)
					return
				}
				dialog.ShowInformation("Connected", "This agent is now connected to Klutch.", u.win)
			})
			u.refresh()
		}()
	}, u.win)
	d.Resize(fyne.NewSize(440, 340))
	d.Show()
}

func trimV(v string) string {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}
