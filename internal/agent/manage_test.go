package agent

import (
	"errors"
	"strings"
	"testing"
)

// Real `lpinfo -l -v` output: a plugged-in ESC/POS receipt printer with no queue,
// a queue-backed network printer, and the bare backend names CUPS always lists.
const lpinfoOut = `Device: uri = socket
        class = network
        info = AppSocket/HP JetDirect
        make-and-model = Unknown
        device-id =
        location =
Device: uri = usb://Printer/POS-80?serial=936C0C663532
        class = direct
        info = Printer POS-80
        make-and-model = Printer POS-80
        device-id = MFG:Printer;MDL:POS-80;CMD:ESC/POS;CLS:PRINTER;
        location =
Device: uri = ipp://192.168.1.42/ipp/print
        class = network
        info = HP LaserJet Pro M428
        make-and-model = HP LaserJet Pro M428
        device-id = MFG:HP;MDL:LaserJet Pro M428;CMD:PDF,PCLm;
        location = Office
Device: uri = hp
        class = direct
        info = HP Printer (HPLIP)
        make-and-model = Unknown
        device-id =
        location =
`

func TestParseLPInfo(t *testing.T) {
	devs := parseLPInfo(lpinfoOut, map[string]string{"ipp://192.168.1.42/ipp/print": "Office-Laser"})

	// The bare backend names ("socket", "hp") address nothing and must be dropped.
	if len(devs) != 2 {
		t.Fatalf("got %d devices, want 2 (bare backend names must be skipped): %+v", len(devs), devs)
	}

	usb := devs[0]
	if usb.URI != "usb://Printer/POS-80?serial=936C0C663532" {
		t.Fatalf("first device = %q, want the USB printer", usb.URI)
	}
	if usb.Installed {
		t.Error("the USB device has no queue; it must not be marked installed")
	}
	if usb.Connection != "USB" {
		t.Errorf("connection = %q, want USB", usb.Connection)
	}
	if usb.Name != "Printer_POS-80" {
		t.Errorf("suggested name = %q, want Printer_POS-80", usb.Name)
	}
	if usb.Driver != DriverRaw {
		t.Errorf("driver = %q, want raw for an ESC/POS device", usb.Driver)
	}

	net := devs[1]
	if !net.Installed || net.Queue != "Office-Laser" {
		t.Errorf("network device installed=%v queue=%q, want true/Office-Laser", net.Installed, net.Queue)
	}
	if net.Driver != DriverEverywhere {
		t.Errorf("driver = %q, want everywhere for an IPP device", net.Driver)
	}
}

func TestDriverFor(t *testing.T) {
	cases := []struct {
		uri, deviceID, want string
	}{
		{"ipp://host/ipp/print", "MFG:HP;CMD:PDF,PCLm;", DriverEverywhere},
		{"ipps://host/ipp/print", "", DriverEverywhere},
		// An IPP-attached receipt printer still needs its own command set.
		{"ippusb://Star/TSP143", "MFG:Star;CMD:ESC/POS;", DriverRaw},
		{"usb://Printer/POS-80", "MFG:Printer;CMD:ESC/POS;", DriverRaw},
		{"socket://192.168.1.9:9100", "", DriverRaw},
		{"serial:/dev/ttyS1?baud=115200", "", DriverRaw},
	}
	for _, c := range cases {
		if got := driverFor(c.uri, c.deviceID); got != c.want {
			t.Errorf("driverFor(%q,%q) = %q, want %q", c.uri, c.deviceID, got, c.want)
		}
	}
}

func TestSanitizeQueueName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Printer POS-80", "Printer_POS-80"},
		{"HP LaserJet/Pro", "HP_LaserJetPro"},
		{"  spaced  ", "spaced"},
		{"###", ""},
		{"Épson TM-T20", "pson_TM-T20"},
	}
	for _, c := range cases {
		if got := sanitizeQueueName(c.in); got != c.want {
			t.Errorf("sanitizeQueueName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestValidQueueName(t *testing.T) {
	if err := validQueueName("Front_Desk-01"); err != nil {
		t.Errorf("a plain name must be accepted: %v", err)
	}
	for _, bad := range []string{"", "has space", "slash/name", "hash#name"} {
		if err := validQueueName(bad); err == nil {
			t.Errorf("validQueueName(%q) = nil, want an error", bad)
		}
	}
}

// The exact text Add-Printer produced on a PC with no "Generic / Text Only"
// driver installed — the blob the Add-printer dialog used to show verbatim.
const addPrinterErr = `Add-Printer : The specified driver does not exist. Use add-printerdriver to add a new driver, or specify an
existing driver.
At line:1 char:1
+ Add-Printer -Name "Printer" -DriverName "Generic / Text Only" -PortNa ...
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : NotSpecified: (MSFT_Printer:ROOT/StandardCimv2/MSFT_Printer) [Add-Printer], CimException
    + FullyQualifiedErrorId : HRESULT 0x80070705,Add-Printer
`

func TestTidyPSError(t *testing.T) {
	got := tidyPSError(addPrinterErr, nil)
	if strings.Contains(got, "At line:") || strings.Contains(got, "CategoryInfo") {
		t.Errorf("the source echo and CategoryInfo block must be dropped, got %q", got)
	}
	// The HRESULT is the one token worth keeping from the tail: it is what
	// windowsPrinterError matches on.
	if !strings.Contains(got, "0x80070705") {
		t.Errorf("the HRESULT must survive, got %q", got)
	}
	if got := tidyPSError("   ", errors.New("exit status 1")); got != "exit status 1" {
		t.Errorf("with no output the exec error is all we have, got %q", got)
	}
}

func TestWindowsPrinterError(t *testing.T) {
	msg := windowsPrinterError(tidyPSError(addPrinterErr, nil))
	if !strings.Contains(msg, winRawDriver) || !strings.Contains(msg, "Add manually") {
		t.Errorf("a missing driver must tell the operator how to install it, got %q", msg)
	}
	if msg := windowsPrinterError("Add-Printer : Access is denied."); !strings.Contains(msg, "administrator") {
		t.Errorf("access denied must point at elevation, got %q", msg)
	}
	// Anything we do not recognise is passed through rather than swallowed.
	if msg := windowsPrinterError("the spooler exploded"); msg != "the spooler exploded" {
		t.Errorf("unknown errors must pass through, got %q", msg)
	}
}

func TestNeedsElevation(t *testing.T) {
	for _, yes := range []string{"Access is denied.", "HRESULT 0x80070005", "The requested operation requires elevation."} {
		if !needsElevation(yes) {
			t.Errorf("needsElevation(%q) = false, want true", yes)
		}
	}
	if needsElevation("The specified driver does not exist.") {
		t.Error("a missing driver is not an elevation problem; retrying as admin would only re-fail")
	}
}

func TestDeviceLabel(t *testing.T) {
	cases := []struct{ in, want string }{
		{"usb://Printer/POS-80?serial=936C0C663532", "Printer POS-80"},
		{"socket://192.168.1.9:9100", "192.168.1.9:9100"},
		{"ipp://host/ipp/print", "host ipp print"},
	}
	for _, c := range cases {
		if got := deviceLabel(c.in); got != c.want {
			t.Errorf("deviceLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
