package desktopapp

import (
	"os"
	"time"

	runtime "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/tlmanz/klutch-agent/internal/agent"
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
	Raw         bool   `json:"raw"`    // pass-through queue (receipt printer): prints dots, not pages
	Status      string `json:"status"` // online|idle|printing|error|offline
	StateReason string `json:"stateReason"`
	Connection  string `json:"connection"` // Wi-Fi|USB|Cloud
	Location    string `json:"location"`
	Queued      int    `json:"queued"`
	Default     bool   `json:"default"`
	// Placeholder marks the stand-in row shown when this PC has no printers: it is
	// a hint, not a queue, so the UI must not offer actions that talk to the spooler.
	Placeholder bool `json:"placeholder"`
}

// DeviceDTO is a printer device the OS can see, whether or not a queue exists
// for it. The Add-printer dialog lists these.
type DeviceDTO struct {
	URI        string `json:"uri"`
	Name       string `json:"name"` // suggested queue name
	Info       string `json:"info"`
	MakeModel  string `json:"makeModel"`
	Connection string `json:"connection"`
	Driver     string `json:"driver"`
	Installed  bool   `json:"installed"`
	Queue      string `json:"queue"`
}

// PrintOptionsDTO is one local-print request from the print screen. It feeds
// both PreviewFile and PrintFile so the preview cannot drift from the output.
type PrintOptionsDTO struct {
	Path      string `json:"path"`
	Printer   string `json:"printer"`
	Mode      string `json:"mode"` // color | gray | mono
	Dither    bool   `json:"dither"`
	Threshold int    `json:"threshold"` // 1..254
	Rotate    int    `json:"rotate"`    // 0 | 90 | 180 | 270
	Invert    bool   `json:"invert"`
	WidthPx   int    `json:"widthPx"` // receipt head width in dots
	Copies    int    `json:"copies"`
	Cut       bool   `json:"cut"`
	TearOffMM int    `json:"tearOffMm"` // blank feed after the image; -1 = use the saved value
	Align     int    `json:"align"`     // 0 left, 1 centre, 2 right
	Media     string `json:"media"`     // A4, Letter, … ("" = printer default)
	FitToPage bool   `json:"fitToPage"`
}

// PreviewDTO is the rendered preview plus the numbers shown beside it.
type PreviewDTO struct {
	DataURL   string `json:"dataUrl"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	SrcWidth  int    `json:"srcWidth"`
	SrcHeight int    `json:"srcHeight"`
	Format    string `json:"format"`
	Printable bool   `json:"printable"`
	Image     bool   `json:"image"`
	Note      string `json:"note"`
	Raw       bool   `json:"raw"`
	TearOffMM int    `json:"tearOffMm"`
	TearOffPx int    `json:"tearOffPx"`
	LengthMM  int    `json:"lengthMm"`
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
			Name: p.Name, Model: p.Description, Raw: p.Raw, Status: p.Status, StateReason: p.StateReason,
			Connection: p.Connection, Location: p.Location, Queued: p.Queued, Default: p.Default,
			Placeholder: p.Placeholder,
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
func (a *App) Reconnect() error                    { return a.ag.Reconnect() }
func (a *App) Disconnect() error                   { return a.ag.Disconnect() }
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

// DiscoverDevices lists printer devices the OS can reach (USB, network, …) so
// the operator can create a queue for one the spooler did not set up itself.
func (a *App) DiscoverDevices() ([]DeviceDTO, error) {
	devs, err := a.ag.DiscoverDevices(a.appCtx)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceDTO, 0, len(devs))
	for _, d := range devs {
		out = append(out, DeviceDTO{
			URI: d.URI, Name: d.Name, Info: d.Info, MakeModel: d.MakeModel,
			Connection: d.Connection, Driver: d.Driver, Installed: d.Installed, Queue: d.Queue,
		})
	}
	return out, nil
}

// AddPrinter creates a queue for a discovered device. driver may be "" to accept
// the suggestion that came with the device.
func (a *App) AddPrinter(name, uri, driver string) error {
	return a.ag.AddPrinter(a.appCtx, name, uri, driver)
}

// RemovePrinter deletes a queue from the OS print system.
func (a *App) RemovePrinter(name string) error { return a.ag.RemovePrinter(a.appCtx, name) }

// --- local file printing -----------------------------------------------------

// PickFile opens the OS file chooser and returns the chosen path ("" if the
// dialog was dismissed).
func (a *App) PickFile() (string, error) {
	if a.ctx == nil {
		return "", nil
	}
	return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Choose a file to print",
		Filters: []runtime.FileFilter{
			{DisplayName: "Images (*.png, *.jpg, *.jpeg, *.gif)", Pattern: "*.png;*.jpg;*.jpeg;*.gif"},
			{DisplayName: "Documents (*.pdf, *.txt)", Pattern: "*.pdf;*.txt"},
			{DisplayName: "All files", Pattern: "*.*"},
		},
	})
}

// PreviewLocalFile renders exactly what the chosen options would print.
func (a *App) PreviewLocalFile(o PrintOptionsDTO) (PreviewDTO, error) {
	res, err := a.ag.PreviewFile(o.toAgent())
	if err != nil {
		return PreviewDTO{}, err
	}
	return PreviewDTO{
		DataURL: res.DataURL, Width: res.Width, Height: res.Height,
		SrcWidth: res.SrcW, SrcHeight: res.SrcH, Format: res.Format,
		Printable: res.Printable, Image: res.Image, Note: res.Note, Raw: res.Raw,
		TearOffMM: res.TearOffMM, TearOffPx: res.TearOffPx, LengthMM: res.LengthMM,
	}, nil
}

// TearOffMM returns the saved tear-off feed for a printer (the default when it
// has never been measured), and SetTearOffMM records a new measurement.
func (a *App) TearOffMM(printer string) int              { return a.ag.TearOffMM(printer) }
func (a *App) SetTearOffMM(printer string, mm int) error { return a.ag.SetTearOffMM(printer, mm) }

// CutEnabled reports whether this printer cuts the paper at the end of a job, and
// SetCutEnabled records it. Per printer, because having a cutter is a property of
// the machine that no printer reports - one without a cutter ignores the command,
// so the answer can only come from whoever is standing in front of it.
func (a *App) CutEnabled(printer string) bool              { return a.ag.CutEnabled(printer) }
func (a *App) SetCutEnabled(printer string, on bool) error { return a.ag.SetCutEnabled(printer, on) }

// PrintLocalFile sends the prepared file to the chosen queue and returns the job id.
func (a *App) PrintLocalFile(o PrintOptionsDTO) (string, error) {
	return a.ag.PrintFile(a.appCtx, o.toAgent())
}

func (o PrintOptionsDTO) toAgent() agent.PrintOptions {
	return agent.PrintOptions{
		Path: o.Path, Printer: o.Printer, Mode: o.Mode, Dither: o.Dither,
		Threshold: o.Threshold, Rotate: o.Rotate, Invert: o.Invert, WidthPx: o.WidthPx,
		Copies: o.Copies, Cut: o.Cut, TearOffMM: o.TearOffMM, Align: o.Align,
		Media: o.Media, FitToPage: o.FitToPage,
	}
}

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
