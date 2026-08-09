//go:build windows

package agent

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Raw printing on Windows goes straight to the spooler through winspool.drv:
// OpenPrinter → StartDocPrinter(datatype "RAW") → WritePrinter → EndDocPrinter.
// That is the exact counterpart of a raw CUPS queue — the bytes reach the device
// untouched, no driver renders anything, and no application is involved.
//
// Shelling out to `Start-Process -Verb PrintTo` cannot do this job: that verb
// asks the shell to find an application registered to print the file, and an
// ESC/POS byte stream has no such application ("No application is associated
// with the specified file for this operation"), which is how every receipt job
// on Windows used to fail.

var (
	winspool             = windows.NewLazySystemDLL("winspool.drv")
	procOpenPrinterW     = winspool.NewProc("OpenPrinterW")
	procClosePrinter     = winspool.NewProc("ClosePrinter")
	procStartDocPrinterW = winspool.NewProc("StartDocPrinterW")
	procEndDocPrinter    = winspool.NewProc("EndDocPrinter")
	procStartPagePrinter = winspool.NewProc("StartPagePrinter")
	procEndPagePrinter   = winspool.NewProc("EndPagePrinter")
	procWritePrinter     = winspool.NewProc("WritePrinter")
)

// docInfo1 is DOC_INFO_1W: the job's name in the Windows queue, an optional
// output file (never set — we want the device), and the datatype.
type docInfo1 struct {
	docName    *uint16
	outputFile *uint16
	datatype   *uint16
}

// printRaw sends the file at path to the named printer as a raw job, listed in
// the Windows queue under doc.
func printRaw(printer, path, doc string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read spooled file: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("the spooled file is empty")
	}

	name, err := windows.UTF16PtrFromString(printer)
	if err != nil {
		return fmt.Errorf("printer name %q: %w", printer, err)
	}
	docName, err := windows.UTF16PtrFromString(doc)
	if err != nil {
		return fmt.Errorf("document name %q: %w", doc, err)
	}
	datatype, err := windows.UTF16PtrFromString("RAW")
	if err != nil {
		return err
	}

	var h windows.Handle
	if r, _, e := procOpenPrinterW.Call(
		uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(&h)), 0,
	); r == 0 {
		return fmt.Errorf("open printer %q: %w", printer, e)
	}
	defer procClosePrinter.Call(uintptr(h))

	info := docInfo1{docName: docName, datatype: datatype}
	// StartDocPrinter returns the spooler's job id, 0 on failure.
	r, _, e := procStartDocPrinterW.Call(uintptr(h), 1, uintptr(unsafe.Pointer(&info)))
	runtime.KeepAlive(info)
	if r == 0 {
		return fmt.Errorf("start print job on %q: %w", printer, e)
	}
	defer procEndDocPrinter.Call(uintptr(h))

	if r, _, e := procStartPagePrinter.Call(uintptr(h)); r == 0 {
		return fmt.Errorf("start page on %q: %w", printer, e)
	}
	defer procEndPagePrinter.Call(uintptr(h))

	// WritePrinter may accept less than it was offered, so keep going until the
	// whole payload is in the spooler.
	for off := 0; off < len(data); {
		var written uint32
		r, _, e := procWritePrinter.Call(
			uintptr(h),
			uintptr(unsafe.Pointer(&data[off])),
			uintptr(len(data)-off),
			uintptr(unsafe.Pointer(&written)),
		)
		runtime.KeepAlive(data)
		if r == 0 {
			return fmt.Errorf("send to %q: %w", printer, e)
		}
		if written == 0 {
			return fmt.Errorf("the spooler accepted none of the remaining %d bytes for %q", len(data)-off, printer)
		}
		off += int(written)
	}
	return nil
}
