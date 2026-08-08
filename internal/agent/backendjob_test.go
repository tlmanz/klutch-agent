package agent

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tlmanz/klutch-agent/internal/imaging"
	"github.com/tlmanz/klutch-agent/wire"
)

// spooledFor runs a backend job through handleJob and returns the bytes that
// reached the spool file. The dispatch itself fails (there is no such printer on
// the test machine), which is fine: the file is written before the spooler is
// asked, and the file is what a real printer would receive.
func spooledFor(t *testing.T, a *Agent, job wire.Job) []byte {
	t.Helper()
	a.handleJob(t.Context(), job)
	matches, err := filepath.Glob(filepath.Join(a.cfg.SpoolDir, job.ID+".*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("spooled files for %s = %v (%v), want one", job.ID, matches, err)
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read spooled file: %v", err)
	}
	return data
}

// TestBackendReceiptGetsThisPrintersTearOff: the backend renders the receipt and
// its stream stops at the last printed row, so the agent - the only party standing
// next to the device - appends the feed that carries those rows past the tear bar.
// Without it, tearing a Klutch receipt rips through its own footer, while a receipt
// printed locally from this app comes out fine: the same printer behaving two
// different ways depending on who drew the pixels.
func TestBackendReceiptGetsThisPrintersTearOff(t *testing.T) {
	a := newTestAgent(t)
	if err := a.SetTearOffMM("Receipt", 42); err != nil {
		t.Fatalf("set tear-off: %v", err)
	}

	raster := []byte{0x1B, 0x40, 0x1D, 0x76, 0x30, 0x00, 0x01, 0x00, 0x01, 0x00, 0xFF}
	data := spooledFor(t, a, wire.Job{
		ID: "job-1", Kind: "escpos_raster", PrinterName: "Receipt",
		PayloadB64: base64.StdEncoding.EncodeToString(raster),
	})

	if !bytes.HasPrefix(data, raster) {
		t.Fatal("the backend's rendered bytes must reach the printer unchanged")
	}
	want := imaging.Finish(imaging.MMToDots(42), false)
	if !bytes.HasSuffix(data, want) {
		t.Fatalf("spooled tail = %x, want the 42mm tear-off feed %x", data[len(raster):], want)
	}
}

// TestBackendReceiptUsesTheDefaultFeedWhenUnmeasured: a printer nobody has measured
// still has to clear its tear bar, so it gets the default rather than no feed.
func TestBackendReceiptUsesTheDefaultFeedWhenUnmeasured(t *testing.T) {
	a := newTestAgent(t)
	raster := []byte{0x1B, 0x40}
	data := spooledFor(t, a, wire.Job{
		ID: "job-2", Kind: "escpos_raster", PrinterName: "Never Measured",
		PayloadB64: base64.StdEncoding.EncodeToString(raster),
	})
	if want := imaging.Finish(imaging.MMToDots(DefaultTearOffMM), false); !bytes.HasSuffix(data, want) {
		t.Fatalf("spooled tail = %x, want the default feed %x", data[len(raster):], want)
	}
}

// TestBackendPDFIsUntouched: only a receipt roll is torn by hand. A PDF goes to a
// page printer that ejects the sheet itself, and appending ESC/POS to it would put
// stray bytes in a document the driver is about to parse.
func TestBackendPDFIsUntouched(t *testing.T) {
	a := newTestAgent(t)
	pdf := []byte("%PDF-1.4\n%%EOF\n")
	data := spooledFor(t, a, wire.Job{
		ID: "job-3", Kind: "pdf", PrinterName: "Office A4",
		PayloadB64: base64.StdEncoding.EncodeToString(pdf),
	})
	if !bytes.Equal(data, pdf) {
		t.Fatalf("spooled PDF = %q, want it byte-for-byte as sent", data)
	}
}

// TestPlaceholderIsNotAPrinter: when the OS reports no printers the list shows a
// stand-in row so the screen is not blank. It is not a queue - and its invented
// name ("Default Printer") is not even a legal CUPS queue name - so acting on it
// used to reach the spooler and come back with "lpadmin: printer name can only
// contain printable characters", which reads as a removal that broke rather than
// a row that was never removable. It also must never be advertised to the
// backend, where it would appear as a printer the dashboard can route jobs to.
func TestPlaceholderIsNotAPrinter(t *testing.T) {
	a := newTestAgent(t)
	placeholder := PrinterInfo{Name: "Default Printer", Placeholder: true, Status: "offline"}
	withPrinters(a, placeholder)

	if err := a.RemovePrinter(t.Context(), "Default Printer"); err == nil {
		t.Fatal("removing the placeholder must be refused here, not handed to the spooler")
	} else if strings.Contains(err.Error(), "lpadmin") {
		t.Fatalf("the refusal must explain the row, not quote the spooler: %v", err)
	}
	if err := a.SetDefaultPrinter("Default Printer"); err == nil {
		t.Fatal("the placeholder cannot be made the default printer")
	}
	if got := toWire([]PrinterInfo{placeholder}); len(got) != 0 {
		t.Fatalf("advertised %v to the backend, want nothing: it is not a printer", got)
	}
}
