package desktopapp

import (
	"os"
	"time"

	"github.com/tlmanz/klutch-agent/internal/autostart"
	"github.com/tlmanz/klutch-agent/internal/desktop"
)

// This file defines the JSON-friendly view models bound to the frontend and the
// action methods it calls. A dedicated DTO (rather than binding agent.State
// directly) keeps clean TypeScript types - notably it renders timestamps as
// strings, which the Wails binding generator cannot derive from time.Time.

// StateDTO is the full snapshot the frontend renders. It bundles printers, live
// jobs, recent history, connection, update and settings state in one payload.
type StateDTO struct {
	Server           string          `json:"server"`
	Host             string          `json:"host"`
	Enrolled         bool            `json:"enrolled"`
	Connected        bool            `json:"connected"`
	LastError        string          `json:"lastError"`
	Version          string          `json:"version"`
	AvailableVersion string          `json:"availableVersion"`
	LastCheck        string          `json:"lastCheck"` // RFC3339 or ""
	AutoUpdate       bool            `json:"autoUpdate"`
	DefaultPrinter   string          `json:"defaultPrinter"`
	Theme            string          `json:"theme"`
	NotifyDone       bool            `json:"notifyDone"`
	NotifyFailed     bool            `json:"notifyFailed"`
	NotifyWeekly     bool            `json:"notifyWeekly"`
	JobsOK           int             `json:"jobsOk"`
	JobsFailed       int             `json:"jobsFailed"`
	Printers         []PrinterDTO    `json:"printers"`
	ActiveJobs       []JobDTO        `json:"activeJobs"`
	RecentJobs       []JobHistoryDTO `json:"recentJobs"`
}

type PrinterDTO struct {
	Name        string `json:"name"`
	Model       string `json:"model"`
	Status      string `json:"status"` // online|idle|printing|error|offline
	StateReason string `json:"stateReason"`
	Connection  string `json:"connection"` // Wi-Fi|USB|Cloud
	Location    string `json:"location"`
	Queued      int    `json:"queued"`
	Default     bool   `json:"default"`
}

type JobDTO struct {
	ID      string  `json:"id"`
	Printer string  `json:"printer"`
	Doc     string  `json:"doc"`
	Kind    string  `json:"kind"`
	State   string  `json:"state"`   // queued|printing|paused
	Percent float64 `json:"percent"` // -1 = indeterminate
}

type JobHistoryDTO struct {
	ID         string `json:"id"`
	Printer    string `json:"printer"`
	Doc        string `json:"doc"`
	Kind       string `json:"kind"`
	Status     string `json:"status"` // ok|failed
	Error      string `json:"error"`
	FinishedAt string `json:"finishedAt"` // RFC3339
}

// buildState maps the agent snapshot + recent history into the view DTO.
func (a *App) buildState() StateDTO {
	st := a.ag.Snapshot()
	host, _ := os.Hostname()
	dto := StateDTO{
		Server: st.Server, Host: host, Enrolled: st.Enrolled, Connected: st.Connected,
		LastError: st.LastError, Version: st.Version, AvailableVersion: st.AvailableVersion,
		AutoUpdate: st.AutoUpdate, DefaultPrinter: st.DefaultPrinter, Theme: st.Theme,
		NotifyDone: st.NotifyDone, NotifyFailed: st.NotifyFailed, NotifyWeekly: st.NotifyWeekly,
		JobsOK: st.JobsOK, JobsFailed: st.JobsFailed,
		// Non-nil so JSON emits [] rather than null (the frontend indexes these).
		Printers: []PrinterDTO{}, ActiveJobs: []JobDTO{}, RecentJobs: []JobHistoryDTO{},
	}
	if !st.LastCheck.IsZero() {
		dto.LastCheck = st.LastCheck.Format(time.RFC3339)
	}
	for _, p := range st.Printers {
		dto.Printers = append(dto.Printers, PrinterDTO{
			Name: p.Name, Model: p.Description, Status: p.Status, StateReason: p.StateReason,
			Connection: p.Connection, Location: p.Location, Queued: p.Queued, Default: p.Default,
		})
	}
	for _, j := range st.ActiveJobs {
		dto.ActiveJobs = append(dto.ActiveJobs, JobDTO{
			ID: j.ID, Printer: j.Printer, Doc: j.Doc, Kind: j.Kind, State: j.State, Percent: j.Percent,
		})
	}
	if jobs, err := a.ag.RecentJobs(200); err == nil {
		for _, r := range jobs {
			dto.RecentJobs = append(dto.RecentJobs, JobHistoryDTO{
				ID: r.ID, Printer: r.Printer, Doc: docName(r.DocumentRef), Kind: r.Kind,
				Status: r.Status, Error: r.Error, FinishedAt: r.FinishedAt.Format(time.RFC3339),
			})
		}
	}
	return dto
}

// docName is a friendly document label from a document ref/path.
func docName(ref string) string {
	if ref == "" {
		return "Print job"
	}
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '/' || ref[i] == '\\' {
			return ref[i+1:]
		}
	}
	return ref
}

// --- bound actions (become TS bindings; errors reject the JS promise) --------

func (a *App) SetServer(url string) error          { return a.ag.SetServer(url) }
func (a *App) Enroll(code string) error            { return a.ag.Enroll(a.appCtx, code) }
func (a *App) SetToken(token string) error         { return a.ag.SetToken(token) }
func (a *App) SetDefaultPrinter(name string) error { return a.ag.SetDefaultPrinter(name) }
func (a *App) SetTheme(theme string) error         { return a.ag.SetTheme(theme) }
func (a *App) SetAutoUpdate(on bool) error         { return a.ag.SetAutoUpdate(on) }
func (a *App) SetNotifyDone(on bool) error         { return a.ag.SetNotifyDone(on) }
func (a *App) SetNotifyFailed(on bool) error       { return a.ag.SetNotifyFailed(on) }
func (a *App) SetNotifyWeekly(on bool) error       { return a.ag.SetNotifyWeekly(on) }
func (a *App) PauseJob(id string) error            { return a.ag.PauseJob(id) }
func (a *App) ResumeJob(id string) error           { return a.ag.ResumeJob(id) }
func (a *App) CancelJob(id string) error           { return a.ag.CancelJob(id) }
func (a *App) ReprintJob(id string) error          { return a.ag.ReprintJob(id) }
func (a *App) PauseAll() error                     { return a.ag.PauseAll() }

// CheckForUpdate returns the available version ("" if up to date).
func (a *App) CheckForUpdate() (string, error) { return a.ag.CheckForUpdate(a.appCtx) }

// ApplyUpdate installs the pending update (relaunches on success).
func (a *App) ApplyUpdate() error { return a.ag.ApplyUpdate(a.appCtx) }

// --- OS launcher / autostart wrappers ---------------------------------------

func (a *App) InstallApp() (string, error) { return desktop.Install() }
func (a *App) UninstallApp() error         { return desktop.Uninstall() }
func (a *App) IsAppInstalled() (bool, error) {
	return desktop.IsInstalled()
}

func (a *App) SetAutostart(on bool) error {
	if on {
		return autostart.Enable([]string{"-tray"})
	}
	return autostart.Disable()
}
func (a *App) AutostartEnabled() (bool, error) { return autostart.IsEnabled() }
