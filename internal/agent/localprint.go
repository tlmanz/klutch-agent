package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tlmanz/klutch-agent/internal/imaging"
	"github.com/tlmanz/klutch-agent/internal/store"
)

// Printing a file the operator picked on this machine, rather than one the
// backend sent. The same pipeline feeds the preview and the printer, so the
// preview is not an approximation: for a receipt printer it is the exact dot
// pattern, at the exact head width, that the ESC/POS raster will fire.

// PrintOptions is one local print request, shared by preview and print so the
// two can never drift apart.
type PrintOptions struct {
	Path      string // file on this machine
	Printer   string // queue name
	Mode      string // color | gray | mono
	Dither    bool
	Threshold int
	Rotate    int
	Invert    bool
	WidthPx   int  // target width in printer dots; 0 = leave the size alone
	Copies    int  // 1..99
	Cut       bool // receipt printers: cut the paper when done
	TearOffMM int  // blank paper fed after the image so the tear bar clears it
	Align     int  // 0 left, 1 centre, 2 right (receipt printers)
	Media     string
	FitToPage bool
}

// Tear-off feed. A receipt head prints some distance before the tear bar, so the
// last few millimetres of a job are still inside the mechanism when it stops.
// Feeding that distance afterwards is what keeps the tear from going through the
// content. No ESC/POS command reports it - it is a property of the chassis, not
// something the firmware exposes - so it is measured once per printer and
// remembered.
//
// These live here and nowhere else: the agent is the only party standing next to
// the printer, so the backend renders the receipt and this decides how the paper
// finishes. The same physical printer therefore behaves the same whether the
// receipt came from the server or a file was printed from this screen.
//
// The maximum is a guard against a mistyped number, not a hardware limit - a
// common tear bar sits 15-25mm past the head, so 100mm is several times the worst
// case and anything beyond it is a slip of the keyboard that would trail 10cm of
// blank roll off every receipt.
const (
	DefaultTearOffMM = 30
	MaxTearOffMM     = 100
)

// PreviewResult is what the UI renders: a PNG of the prepared image plus the
// facts the operator needs to judge it.
type PreviewResult struct {
	DataURL   string // "data:image/png;base64,…"; empty when there is nothing to show
	Width     int    // prepared size in printer dots
	Height    int
	SrcW      int // the file's own size, for the "3024 × 4032 → 576 × 768" line
	SrcH      int
	Format    string // png | jpeg | gif
	Printable bool   // false when the file cannot be printed at all
	Image     bool   // false for files that are handed to the spooler untouched
	Note      string // why there is no preview, when there is none
	Raw       bool   // the chosen queue is a pass-through (receipt) queue

	// Receipt printers only: the tear-off feed in force (millimetres and dots) and
	// how much paper the whole job consumes, so the UI can draw the tear line
	// where it will actually fall.
	TearOffMM int
	TearOffPx int
	LengthMM  int
}

// preview caches the last decoded file: the UI re-renders on every slider move,
// and decoding a 12-megapixel photo each time would make the controls lag.
type decodedFile struct {
	path    string
	modTime time.Time
	size    int64
	img     image.Image
	format  string
}

type previewCache struct {
	mu   sync.Mutex
	last *decodedFile
}

func (c *previewCache) load(path string) (*decodedFile, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if l := c.last; l != nil && l.path == path && l.modTime.Equal(fi.ModTime()) && l.size == fi.Size() {
		return l, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, format, err := imaging.Decode(f)
	if err != nil {
		return nil, err
	}
	c.last = &decodedFile{path: path, modTime: fi.ModTime(), size: fi.Size(), img: img, format: format}
	return c.last, nil
}

// previewMaxHeight caps how tall a preview PNG gets. A receipt of a tall photo
// is legitimately thousands of dots long; beyond this the data URL costs more
// than the extra detail is worth.
const previewMaxHeight = 4000

// resolve fills in the options the chosen queue dictates, so preview and print
// agree. A raw queue can only fire dots, so black & white is not optional there.
func (a *Agent) resolve(o PrintOptions) (PrintOptions, bool) {
	raw := false
	for _, p := range a.Snapshot().Printers {
		if p.Name == o.Printer {
			raw = p.Raw
			break
		}
	}
	if raw {
		o.Mode = imaging.ModeMono
		if o.WidthPx <= 0 {
			o.WidthPx = imaging.Width80mm
		}
		if o.TearOffMM < 0 {
			o.TearOffMM = a.TearOffMM(o.Printer)
		}
		if o.TearOffMM > MaxTearOffMM {
			o.TearOffMM = MaxTearOffMM
		}
	} else {
		o.TearOffMM = 0
		// Driver-backed queues scale the page themselves; forcing a dot width
		// would only throw detail away.
		o.WidthPx = 0
	}
	if o.Mode == "" {
		o.Mode = imaging.ModeColor
	}
	if o.Threshold <= 0 || o.Threshold >= 255 {
		o.Threshold = imaging.DefaultThreshold
	}
	if o.Copies < 1 {
		o.Copies = 1
	}
	if o.Copies > 99 {
		o.Copies = 99
	}
	return o, raw
}

func (o PrintOptions) imagingOptions() imaging.Options {
	return imaging.Options{
		Mode: o.Mode, Dither: o.Dither, Threshold: o.Threshold,
		Rotate: o.Rotate, Invert: o.Invert, WidthPx: o.WidthPx,
	}
}

// PreviewFile renders what would be printed. Files that are not images (a PDF,
// a text file) have no preview: they go to the spooler untouched, which is
// reported rather than guessed at.
func (a *Agent) PreviewFile(o PrintOptions) (PreviewResult, error) {
	if strings.TrimSpace(o.Path) == "" {
		return PreviewResult{}, fmt.Errorf("choose a file to print")
	}
	if _, err := os.Stat(o.Path); err != nil {
		return PreviewResult{}, fmt.Errorf("cannot read %s", filepath.Base(o.Path))
	}
	opts, raw := a.resolve(o)

	dec, err := a.previews.load(o.Path)
	if err != nil {
		// Not an image: still printable, just not previewable.
		res := PreviewResult{Printable: !raw, Raw: raw}
		if raw {
			res.Note = fmt.Sprintf("%s is a pass-through (receipt) queue: it can only print images, not %s files.",
				o.Printer, strings.TrimPrefix(strings.ToUpper(filepath.Ext(o.Path)), "."))
		} else {
			res.Note = "No preview for this file type. It is sent to the printer unchanged, and the printer's own driver lays it out."
		}
		return res, nil
	}

	iopts := opts.imagingOptions()
	iopts.MaxHeight = previewMaxHeight
	prepared := imaging.Prepare(dec.img, iopts)
	pngBytes, err := imaging.EncodePNG(prepared)
	if err != nil {
		return PreviewResult{}, err
	}
	b := prepared.Bounds()
	src := dec.img.Bounds()
	res := PreviewResult{
		DataURL:   "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes),
		Width:     b.Dx(),
		Height:    b.Dy(),
		SrcW:      src.Dx(),
		SrcH:      src.Dy(),
		Format:    dec.format,
		Printable: true,
		Image:     true,
		Raw:       raw,
	}
	if raw {
		res.TearOffMM = opts.TearOffMM
		res.TearOffPx = imaging.MMToDots(opts.TearOffMM)
		res.LengthMM = imaging.DotsToMM(b.Dy()) + opts.TearOffMM
	}
	return res, nil
}

// PrintFile prepares a local file and sends it to the chosen queue, tracking the
// result in the Jobs screen exactly like a job that arrived from the backend.
func (a *Agent) PrintFile(ctx context.Context, o PrintOptions) (string, error) {
	if strings.TrimSpace(o.Path) == "" {
		return "", fmt.Errorf("choose a file to print")
	}
	if strings.TrimSpace(o.Printer) == "" {
		return "", fmt.Errorf("choose a printer")
	}
	opts, raw := a.resolve(o)
	if err := os.MkdirAll(a.cfg.SpoolDir, 0o755); err != nil {
		return "", fmt.Errorf("spool dir: %w", err)
	}

	id := fmt.Sprintf("local-%d", time.Now().UnixNano())
	kind, path := "file", o.Path
	var lpOpts []string

	dec, err := a.previews.load(o.Path)
	switch {
	case err != nil && raw:
		return "", fmt.Errorf("%s only prints images: %v", o.Printer, err)
	case err != nil:
		// Hand the file to the spooler untouched and let the driver lay it out.
		lpOpts = pageOptions(opts)
	default:
		prepared := imaging.Prepare(dec.img, opts.imagingOptions())
		if raw {
			kind = "escpos_raster"
			path = filepath.Join(a.cfg.SpoolDir, id+".escpos")
			data := imaging.ESCPOS(prepared, imaging.ESCPOSOptions{
				Align: opts.Align, FeedDots: imaging.MMToDots(opts.TearOffMM), Cut: opts.Cut,
			})
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return "", err
			}
		} else {
			kind = "image"
			path = filepath.Join(a.cfg.SpoolDir, id+".png")
			data, err := imaging.EncodePNG(prepared)
			if err != nil {
				return "", err
			}
			if err := os.WriteFile(path, data, 0o644); err != nil {
				return "", err
			}
			lpOpts = pageOptions(opts)
		}
	}
	if opts.Copies > 1 {
		lpOpts = append(lpOpts, "-n", fmt.Sprint(opts.Copies))
	}

	rec := store.JobRecord{
		ID: id, Printer: opts.Printer, Kind: kind,
		DocumentRef: o.Path, ReceivedAt: time.Now(),
	}
	if fi, err := os.Stat(path); err == nil {
		rec.Bytes = int(fi.Size())
	}
	a.putJob(JobInfo{
		ID: id, Printer: opts.Printer, Doc: docName(o.Path), Kind: kind,
		State: "printing", Percent: -1, Started: time.Now(),
	})

	reqID, err := dispatchFile(ctx, opts.Printer, path, lpOpts)
	if err != nil {
		rec.FinishedAt = time.Now()
		rec.Status = "failed"
		rec.Error = err.Error()
		a.recordOutcome(rec)
		a.removeJob(id)
		return "", err
	}
	a.log.Printf("printed local file %q → %q (%s, req %q)", o.Path, opts.Printer, kind, reqID)

	if reqID == "" {
		rec.FinishedAt = time.Now()
		rec.Status = "ok"
		a.recordOutcome(rec)
		a.removeJob(id)
	} else {
		a.updateJob(id, func(j *JobInfo) { j.ReqID = reqID })
		go a.trackCUPSJob(a.jobCtx(), rec, reqID)
	}
	return id, nil
}

// TearOffMM returns the feed distance measured for a printer, or the default
// when it has never been set. It is stored per printer because it is a fact
// about that chassis, not a preference.
func (a *Agent) TearOffMM(printer string) int {
	v, err := a.store.Setting(store.KeyTearOff(printer))
	if err != nil || v == "" {
		return DefaultTearOffMM
	}
	mm, err := strconv.Atoi(v)
	if err != nil || mm < 0 {
		return DefaultTearOffMM
	}
	if mm > MaxTearOffMM {
		return MaxTearOffMM
	}
	return mm
}

// CutEnabled reports whether this printer should cut the paper at the end of a
// job. Off unless someone has said otherwise: most counters run a printer with no
// cutter, and one that has none simply ignores the command, so an unset printer
// tearing by hand is the safe assumption rather than a promise we cannot keep.
func (a *Agent) CutEnabled(printer string) bool {
	v, err := a.store.Setting(store.KeyCut(printer))
	return err == nil && v == "1"
}

// SetCutEnabled remembers whether a printer has a cutter to use.
func (a *Agent) SetCutEnabled(printer string, on bool) error {
	if strings.TrimSpace(printer) == "" {
		return fmt.Errorf("no printer named")
	}
	v := "0"
	if on {
		v = "1"
	}
	return a.store.SetSetting(store.KeyCut(printer), v)
}

// SetTearOffMM remembers the feed distance for a printer.
func (a *Agent) SetTearOffMM(printer string, mm int) error {
	if strings.TrimSpace(printer) == "" {
		return fmt.Errorf("no printer named")
	}
	if mm < 0 {
		mm = 0
	}
	if mm > MaxTearOffMM {
		mm = MaxTearOffMM
	}
	return a.store.SetSetting(store.KeyTearOff(printer), strconv.Itoa(mm))
}

// pageOptions maps the page-printer settings onto `lp -o` flags. Receipt queues
// take none of these: a raw queue has no driver to interpret them.
func pageOptions(o PrintOptions) []string {
	var opts []string
	if o.FitToPage {
		opts = append(opts, "-o", "fit-to-page")
	}
	if m := strings.TrimSpace(o.Media); m != "" {
		opts = append(opts, "-o", "media="+m)
	}
	return opts
}
