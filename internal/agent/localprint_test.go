package agent

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tlmanz/klutch-agent/internal/imaging"
)

// writePNG puts a small colour image on disk and returns its path.
func writePNG(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), 90, 200, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "sample.png")
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// withPrinters seeds the agent's state with a printer set (normally filled by
// enumeration against the real spooler).
func withPrinters(a *Agent, ps ...PrinterInfo) {
	a.mutate(func(s *State) { s.Printers = ps })
}

func TestPreviewForcesMonoOnAPassThroughQueue(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "Receipt", Raw: true})
	path := writePNG(t, 1200, 600)

	// Ask for full colour: the queue cannot do it, so the preview must show what
	// will really happen rather than a colour image the operator never gets.
	res, err := a.PreviewFile(PrintOptions{Path: path, Printer: "Receipt", Mode: imaging.ModeColor})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !res.Raw {
		t.Error("the preview must report that the queue is pass-through")
	}
	if res.Width != imaging.Width80mm {
		t.Errorf("width = %d dots, want the 80mm default of %d", res.Width, imaging.Width80mm)
	}
	if res.Height != 288 { // 600 * 576/1200
		t.Errorf("height = %d, want 288 (aspect preserved)", res.Height)
	}
	if !strings.HasPrefix(res.DataURL, "data:image/png;base64,") {
		t.Error("preview must come back as an inline PNG the UI can render")
	}
	if res.SrcW != 1200 || res.SrcH != 600 {
		t.Errorf("source size = %dx%d, want the file's own 1200x600", res.SrcW, res.SrcH)
	}
}

func TestPreviewLeavesPageQueuesAtFullSize(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "Laser", Raw: false})
	path := writePNG(t, 300, 200)

	res, err := a.PreviewFile(PrintOptions{Path: path, Printer: "Laser", Mode: imaging.ModeGray})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if res.Width != 300 || res.Height != 200 {
		t.Errorf("size = %dx%d, want the original 300x200: a driver queue scales the page itself",
			res.Width, res.Height)
	}
}

func TestPreviewOfANonImage(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "Laser"}, PrinterInfo{Name: "Receipt", Raw: true})
	path := filepath.Join(t.TempDir(), "invoice.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.7\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// A driver queue can still print it, untouched.
	res, err := a.PreviewFile(PrintOptions{Path: path, Printer: "Laser"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !res.Printable || res.Image || res.DataURL != "" || res.Note == "" {
		t.Errorf("a PDF on a driver queue: want printable with a note and no preview, got %+v", res)
	}

	// A receipt queue cannot: say so instead of sending it garbage.
	res, err = a.PreviewFile(PrintOptions{Path: path, Printer: "Receipt"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if res.Printable {
		t.Error("a PDF must not be reported as printable on a pass-through queue")
	}
}

func TestPreviewRejectsAMissingFile(t *testing.T) {
	a := newTestAgent(t)
	if _, err := a.PreviewFile(PrintOptions{Path: "/no/such/file.png", Printer: "x"}); err == nil {
		t.Fatal("a missing file must be reported, not previewed")
	}
	if _, err := a.PreviewFile(PrintOptions{Printer: "x"}); err == nil {
		t.Fatal("an empty path must ask for a file")
	}
}

func TestPrintToAPassThroughQueueSpoolsESCPOS(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "no-such-queue-for-tests", Raw: true})
	path := writePNG(t, 200, 100)

	// The dispatch itself fails (there is no such queue), which is fine: what
	// matters is that the bytes prepared for it are ESC/POS raster, not a PNG.
	_, err := a.PrintFile(context.Background(), PrintOptions{
		Path: path, Printer: "no-such-queue-for-tests", Cut: true,
	})
	if err == nil {
		t.Skip("a queue by this name exists on this machine; skipping the failure path")
	}

	spooled, _ := filepath.Glob(filepath.Join(a.cfg.SpoolDir, "local-*.escpos"))
	if len(spooled) != 1 {
		t.Fatalf("spooled files = %v, want one .escpos", spooled)
	}
	data, err := os.ReadFile(spooled[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(data, []byte{0x1b, 0x40}) {
		t.Error("the spooled file must start with the ESC @ reset a receipt printer expects")
	}
	if !bytes.Contains(data, []byte{0x1d, 0x76, 0x30}) {
		t.Error("the spooled file must contain a GS v 0 raster command")
	}
	if !bytes.HasSuffix(data, []byte{0x1d, 0x56, 0x42, 0x00}) {
		t.Error("with Cut set, the job must end in a partial cut")
	}

	// The failure is recorded like any other job, so it shows up in history.
	jobs, err := a.RecentJobs(10)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("recent jobs = %v (err %v), want the failed local job", jobs, err)
	}
	if jobs[0].Status != "failed" || jobs[0].Kind != "escpos_raster" {
		t.Errorf("recorded job = %+v, want a failed escpos_raster job", jobs[0])
	}
	if jobs[0].DocumentRef != path {
		t.Errorf("document ref = %q, want the file that was printed", jobs[0].DocumentRef)
	}
}

func TestPrintToAPageQueueSpoolsPNG(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "no-such-queue-for-tests", Raw: false})
	path := writePNG(t, 120, 80)

	_, err := a.PrintFile(context.Background(), PrintOptions{
		Path: path, Printer: "no-such-queue-for-tests", Mode: imaging.ModeGray, FitToPage: true,
	})
	if err == nil {
		t.Skip("a queue by this name exists on this machine; skipping the failure path")
	}
	spooled, _ := filepath.Glob(filepath.Join(a.cfg.SpoolDir, "local-*.png"))
	if len(spooled) != 1 {
		t.Fatalf("spooled files = %v, want one .png for a driver-backed queue", spooled)
	}
}

func TestTearOffIsMeasuredOncePerPrinter(t *testing.T) {
	a := newTestAgent(t)
	if got := a.TearOffMM("Receipt"); got != DefaultTearOffMM {
		t.Errorf("unmeasured printer = %d mm, want the %d mm default", got, DefaultTearOffMM)
	}
	if err := a.SetTearOffMM("Receipt", 22); err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := a.TearOffMM("Receipt"); got != 22 {
		t.Errorf("saved tear-off = %d mm, want 22", got)
	}
	// It belongs to that chassis, not to the app.
	if got := a.TearOffMM("Other"); got != DefaultTearOffMM {
		t.Errorf("a different printer = %d mm, want the default", got)
	}
	// Zero is a real answer (a printer whose cutter handles it), not "unset".
	if err := a.SetTearOffMM("Receipt", 0); err != nil {
		t.Fatal(err)
	}
	if got := a.TearOffMM("Receipt"); got != 0 {
		t.Errorf("tear-off = %d mm, want the 0 that was saved", got)
	}
	if err := a.SetTearOffMM("Receipt", 500); err != nil {
		t.Fatal(err)
	}
	if got := a.TearOffMM("Receipt"); got != MaxTearOffMM {
		t.Errorf("tear-off = %d mm, want it clamped to %d", got, MaxTearOffMM)
	}
}

func TestPreviewReportsTheTearOffAndPaperUsed(t *testing.T) {
	a := newTestAgent(t)
	withPrinters(a, PrinterInfo{Name: "Receipt", Raw: true})
	if err := a.SetTearOffMM("Receipt", 20); err != nil {
		t.Fatal(err)
	}
	path := writePNG(t, 576, 400) // 400 dots ≈ 50 mm of paper

	// -1 means "use what was measured for this printer".
	res, err := a.PreviewFile(PrintOptions{Path: path, Printer: "Receipt", TearOffMM: -1})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if res.TearOffMM != 20 {
		t.Errorf("tear-off = %d mm, want the saved 20", res.TearOffMM)
	}
	if res.TearOffPx != imaging.MMToDots(20) {
		t.Errorf("tear-off = %d dots, want %d", res.TearOffPx, imaging.MMToDots(20))
	}
	if want := imaging.DotsToMM(400) + 20; res.LengthMM != want {
		t.Errorf("paper used = %d mm, want %d (image plus feed)", res.LengthMM, want)
	}

	// A page printer has no tear bar to clear.
	withPrinters(a, PrinterInfo{Name: "Laser"})
	res, err = a.PreviewFile(PrintOptions{Path: path, Printer: "Laser", TearOffMM: -1})
	if err != nil {
		t.Fatal(err)
	}
	if res.TearOffMM != 0 || res.LengthMM != 0 {
		t.Errorf("page printer preview = %+v, want no tear-off reported", res)
	}
}

func TestPageOptions(t *testing.T) {
	got := pageOptions(PrintOptions{FitToPage: true, Media: "A4"})
	want := []string{"-o", "fit-to-page", "-o", "media=A4"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("lp options = %v, want %v", got, want)
	}
	if got := pageOptions(PrintOptions{}); len(got) != 0 {
		t.Errorf("with nothing selected, lp options = %v, want none", got)
	}
}
