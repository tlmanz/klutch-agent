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
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynedesktop "fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/tlmanz/klutch-agent/internal/agent"
	"github.com/tlmanz/klutch-agent/internal/autostart"
	"github.com/tlmanz/klutch-agent/internal/desktop"
	"github.com/tlmanz/klutch-agent/internal/store"
	"github.com/tlmanz/klutch-agent/wire"
)

// appIcon is the embedded Klutch icon reused for the window and tray so the
// running app matches the installed launcher entry.
var appIcon = fyne.NewStaticResource("icon.png", desktop.IconBytes())

// UI holds the Fyne app, window, and the widgets refreshed from agent state.
type UI struct {
	ctx         context.Context
	app         fyne.App
	win         fyne.Window
	ag          *agent.Agent
	startHidden bool

	// header / app bar
	statusBadge *statusBadge
	serverText  *canvas.Text
	versionText *canvas.Text

	// printers tab
	printers      []wire.Printer
	printerTable  *widget.Table
	printersCount *statCard

	// jobs tab
	jobs        []store.JobRecord
	jobTable    *widget.Table
	statPrinted *statCard
	statFailed  *statCard

	// settings tab
	serverEntry      *widget.Entry
	autoUpdateCheck  *widget.Check
	autostartCheck   *widget.Check
	appInstallStatus *widget.Label
	appInstallBtn    *widget.Button
	appRemoveBtn     *widget.Button

	// updates tab
	updateStatus *widget.Label
	checkBtn     *widget.Button
	installBtn   *widget.Button
}

// New builds the UI around an agent. version is the running build's version.
func New(ctx context.Context, ag *agent.Agent) *UI {
	a := app.NewWithID(autostart.AppID)
	a.Settings().SetTheme(newKlutchTheme())
	a.SetIcon(appIcon)
	w := a.NewWindow("Klutch Print Agent")
	w.Resize(fyne.NewSize(860, 600))

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
	header := u.buildHeader()

	tabs := container.NewAppTabs(
		container.NewTabItemWithIcon("Printers", theme.ComputerIcon(), u.buildPrintersTab()),
		container.NewTabItemWithIcon("Jobs", theme.DocumentIcon(), u.buildJobsTab()),
		container.NewTabItemWithIcon("Updates", theme.DownloadIcon(), u.buildUpdatesTab()),
		container.NewTabItemWithIcon("Settings", theme.SettingsIcon(), u.buildSettingsTab()),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	u.win.SetContent(container.NewBorder(header, nil, nil, nil, container.NewPadded(tabs)))

	// Close hides to tray so the agent keeps printing in the background.
	u.win.SetCloseIntercept(func() { u.win.Hide() })
	u.buildTray()
}

// buildHeader is the top app bar: brand mark + title on the left, live status
// badge + backend/version on the right, over a surface-colored strip.
func (u *UI) buildHeader() fyne.CanvasObject {
	// Brand mark: amber rounded square with a dark "K".
	mark := canvas.NewRectangle(brandPrimary)
	mark.CornerRadius = 9
	k := canvas.NewText("K", brandOnPrimary)
	k.TextStyle = fyne.TextStyle{Bold: true}
	k.TextSize = 20
	k.Alignment = fyne.TextAlignCenter
	markBox := container.NewGridWrap(fyne.NewSize(38, 38), container.NewStack(mark, container.NewCenter(k)))

	title := canvas.NewText("Klutch Print Agent", theme.Color(theme.ColorNameForeground))
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 18
	u.serverText = canvas.NewText("", mutedColor())
	u.serverText.TextSize = 12
	titleBox := container.NewVBox(title, u.serverText)

	left := container.NewHBox(container.NewCenter(markBox), titleBox)

	u.statusBadge = newStatusBadge()
	u.versionText = canvas.NewText("", mutedColor())
	u.versionText.TextSize = 12
	u.versionText.Alignment = fyne.TextAlignTrailing
	right := container.NewVBox(
		container.NewHBox(layout.NewSpacer(), u.statusBadge.obj),
		u.versionText,
	)

	bar := container.NewBorder(nil, nil, left, right)

	bg := canvas.NewRectangle(surfaceColor())
	sep := canvas.NewRectangle(strokeColor())
	sep.SetMinSize(fyne.NewSize(0, 1))
	return container.NewStack(bg, container.NewBorder(nil, sep, nil, nil, container.NewPadded(bar)))
}

// buildTray adds a system-tray menu (open / check updates / quit) on desktops
// that support it.
func (u *UI) buildTray() {
	desk, ok := u.app.(fynedesktop.App)
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
	desk.SetSystemTrayIcon(appIcon)
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
				l.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				l.SetText(p.Description)
				l.TextStyle = fyne.TextStyle{}
			}
		},
	)
	u.printerTable.ShowHeaderRow = true
	u.printerTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	u.printerTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := []string{"Printer", "Description"}
		o.(*widget.Label).SetText(h[id.Col])
	}
	u.printerTable.SetColumnWidth(0, 280)
	u.printerTable.SetColumnWidth(1, 480)

	u.printersCount = newStatCard("Printers detected", brandPrimary)

	head := container.NewBorder(nil, nil,
		sectionTitle("Connected printers", "Advertised to Klutch. Tag each one's type and paper width in the dashboard."),
		container.NewGridWrap(fyne.NewSize(190, 72), u.printersCount.obj),
	)

	return container.NewBorder(
		container.NewVBox(head, widget.NewSeparator()),
		nil, nil, nil,
		card(u.printerTable),
	)
}

func (u *UI) buildJobsTab() fyne.CanvasObject {
	u.jobTable = widget.NewTable(
		func() (int, int) { return len(u.jobs), 5 },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.TableCellID, o fyne.CanvasObject) {
			l := o.(*widget.Label)
			l.TextStyle = fyne.TextStyle{}
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
					l.SetText("● OK")
				} else {
					l.SetText("● Failed")
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
	u.jobTable.CreateHeader = func() fyne.CanvasObject {
		return widget.NewLabelWithStyle("", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	}
	u.jobTable.UpdateHeader = func(id widget.TableCellID, o fyne.CanvasObject) {
		h := []string{"Time", "Printer", "Kind", "Status", "Detail"}
		o.(*widget.Label).SetText(h[id.Col])
	}
	u.jobTable.SetColumnWidth(0, 170)
	u.jobTable.SetColumnWidth(1, 170)
	u.jobTable.SetColumnWidth(2, 110)
	u.jobTable.SetColumnWidth(3, 90)
	u.jobTable.SetColumnWidth(4, 280)

	u.statPrinted = newStatCard("Printed", brandSuccess())
	u.statFailed = newStatCard("Failed", brandError())

	refresh := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() { go u.refresh() })
	stats := container.NewGridWithColumns(2, u.statPrinted.obj, u.statFailed.obj)
	head := container.NewBorder(nil, nil, nil, container.NewVBox(refresh, layout.NewSpacer()), stats)

	return container.NewBorder(
		container.NewVBox(head, widget.NewSeparator()),
		nil, nil, nil,
		card(u.jobTable),
	)
}

func (u *UI) buildUpdatesTab() fyne.CanvasObject {
	u.updateStatus = widget.NewLabel("")
	u.updateStatus.Wrapping = fyne.TextWrapWord
	u.checkBtn = widget.NewButtonWithIcon("Check now", theme.SearchIcon(), func() { u.doCheck() })
	u.installBtn = widget.NewButtonWithIcon("Install update", theme.DownloadIcon(), func() { u.doInstall() })
	u.installBtn.Importance = widget.HighImportance
	u.installBtn.Disable()

	help := widget.NewLabel("The agent updates itself from its release channel. Enable automatic updates in Settings to install them without prompting.")
	help.Wrapping = fyne.TextWrapWord

	body := container.NewVBox(
		sectionTitle("Software updates", ""),
		u.updateStatus,
		widget.NewSeparator(),
		container.NewHBox(u.checkBtn, u.installBtn),
		widget.NewSeparator(),
		help,
	)
	return container.NewVScroll(container.NewVBox(card(body)))
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
	reEnroll.Importance = widget.HighImportance

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

	connection := container.NewVBox(
		sectionTitle("Connection", "Where this agent sends its heartbeat and receives jobs."),
		widget.NewForm(
			widget.NewFormItem("Backend server", container.NewBorder(nil, nil, nil, saveServer, u.serverEntry)),
			widget.NewFormItem("Enrollment", reEnroll),
		),
	)

	preferences := container.NewVBox(
		sectionTitle("Preferences", "How the agent behaves on this PC."),
		u.autoUpdateCheck,
		u.autostartCheck,
	)

	u.appInstallStatus = widget.NewLabel("")
	u.appInstallStatus.Wrapping = fyne.TextWrapWord
	u.appInstallBtn = widget.NewButtonWithIcon("Add to Applications", theme.DownloadIcon(), func() {
		u.doInstallApp(u.syncInstallState)
	})
	u.appInstallBtn.Importance = widget.HighImportance
	u.appRemoveBtn = widget.NewButtonWithIcon("Remove", theme.CancelIcon(), func() {
		go func() {
			err := desktop.Uninstall()
			fyne.Do(func() {
				if err != nil {
					dialog.ShowError(err, u.win)
				}
				u.syncInstallState()
			})
		}()
	})
	application := container.NewVBox(
		sectionTitle("Application", "Install Klutch Agent so you can reopen it from your applications menu after quitting."),
		u.appInstallStatus,
		container.NewHBox(u.appInstallBtn, u.appRemoveBtn),
	)
	u.syncInstallState()

	return container.NewVScroll(container.NewVBox(card(connection), card(application), card(preferences)))
}

// syncInstallState reflects whether the agent is registered in the OS launcher,
// toggling the install/remove buttons accordingly.
func (u *UI) syncInstallState() {
	if u.appInstallStatus == nil {
		return
	}
	installed, err := desktop.IsInstalled()
	switch {
	case err != nil:
		u.appInstallStatus.SetText("Could not determine install status.")
		u.appInstallBtn.Enable()
		u.appRemoveBtn.Disable()
	case installed:
		u.appInstallStatus.SetText("Installed — find “Klutch Agent” in your applications menu (Start menu on Windows).")
		u.appInstallBtn.Disable()
		u.appRemoveBtn.Enable()
	default:
		u.appInstallStatus.SetText("Not installed on this PC yet.")
		u.appInstallBtn.Enable()
		u.appRemoveBtn.Disable()
	}
}

// doInstallApp registers the agent in the OS launcher (copying the binary to a
// stable location), reporting success or failure. onDone runs on success.
func (u *UI) doInstallApp(onDone func()) {
	go func() {
		path, err := desktop.Install()
		fyne.Do(func() {
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			dialog.ShowInformation("Added to Applications",
				"Klutch Agent is now in your applications menu.\n\nInstalled to:\n"+path, u.win)
			if onDone != nil {
				onDone()
			}
		})
	}()
}

// offerInstall prompts (once) to add the agent to the applications menu, so it
// can be reopened after quitting. It is a no-op if already installed.
func (u *UI) offerInstall() {
	if installed, err := desktop.IsInstalled(); err != nil || installed {
		return
	}
	dialog.ShowConfirm("Add to Applications",
		"Add Klutch Agent to your applications so you can reopen it after quitting?",
		func(ok bool) {
			if ok {
				u.doInstallApp(u.syncInstallState)
			}
		}, u.win)
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

	// First-run: prompt for server + pairing code if not enrolled yet. If already
	// enrolled but not yet added to the applications menu, offer to install it so
	// the operator can reopen it after quitting.
	enrolled := u.ag.Snapshot().Enrolled
	if !enrolled {
		go func() {
			time.Sleep(400 * time.Millisecond)
			fyne.Do(func() { u.showEnrollDialog() })
		}()
	} else if !u.startHidden {
		go func() {
			time.Sleep(600 * time.Millisecond)
			fyne.Do(func() { u.offerInstall() })
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

		// Status badge: color-coded by connection state.
		switch {
		case !st.Enrolled:
			u.statusBadge.set("Not enrolled", brandPrimary)
		case st.Connected:
			u.statusBadge.set("Connected", brandSuccess())
		default:
			label := "Disconnected"
			if st.LastError != "" {
				label = "Disconnected"
			}
			u.statusBadge.set(label, brandError())
		}

		server := st.Server
		if server == "" {
			server = "No backend configured"
		}
		u.serverText.Text = server
		u.serverText.Refresh()
		u.versionText.Text = "v" + trimV(st.Version)
		u.versionText.Refresh()

		u.statPrinted.set(fmt.Sprintf("%d", st.JobsOK))
		u.statFailed.set(fmt.Sprintf("%d", st.JobsFailed))
		u.printersCount.set(fmt.Sprintf("%d", len(st.Printers)))

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
	codeRow := container.NewVBox(widget.NewLabelWithStyle("Pairing code", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), code)
	tokenRow := container.NewVBox(widget.NewLabelWithStyle("Device token", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), token)
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
	mode.Horizontal = true
	mode.SetSelected("Pairing code")

	content := container.NewVBox(
		widget.NewLabelWithStyle("Backend server", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), server,
		widget.NewLabelWithStyle("Connect using", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), mode,
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
				// After connecting, offer to add the agent to the applications
				// menu (sequenced so the dialogs don't stack).
				info := dialog.NewInformation("Connected", "This agent is now connected to Klutch.", u.win)
				info.SetOnClosed(func() { u.offerInstall() })
				info.Show()
			})
			u.refresh()
		}()
	}, u.win)
	d.Resize(fyne.NewSize(460, 360))
	d.Show()
}

func trimV(v string) string {
	if len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		return v[1:]
	}
	return v
}
