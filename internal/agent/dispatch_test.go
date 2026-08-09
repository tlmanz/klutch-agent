package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

// The exact refusal Windows gave for an ESC/POS spool file: the shell looks for
// an application that prints .escpos, there is none, and the job never reached
// the spooler at all.
const noAssocErr = `Start-Process : This command cannot be run due to the error: No application is associated with the specified file for this operation.
At line:1 char:1
+ Start-Process -FilePath 'C:\Users\ops\AppData\Roaming\klutch-agent\sp ...
+ ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
    + CategoryInfo          : InvalidOperation: (:) [Start-Process], InvalidOperationException
`

func TestRawPayloads(t *testing.T) {
	// A payload the backend or the ESC/POS encoder already rendered must go to
	// the spooler as raw bytes; routing it through the shell is what failed.
	for _, raw := range []string{`C:\spool\job.escpos`, `C:\spool\job.bin`, `C:\spool\LABEL.ZPL`} {
		if !rawPayloads[strings.ToLower(filepath.Ext(raw))] {
			t.Errorf("%s must be dispatched as a raw job", raw)
		}
	}
	// These still need an application to lay them out.
	for _, doc := range []string{`C:\spool\job.pdf`, `C:\spool\job.png`, `C:\Users\ops\invoice.docx`} {
		if rawPayloads[strings.ToLower(filepath.Ext(doc))] {
			t.Errorf("%s needs rendering; sending its bytes raw would print garbage", doc)
		}
	}
}

func TestWindowsPrintError(t *testing.T) {
	msg := windowsPrintError(tidyPSError(noAssocErr, nil), `C:\spool\job.pdf`)
	if !strings.Contains(msg, "pdf") || !strings.Contains(msg, "Print to") {
		t.Errorf("a missing association must name the file type and the fix, got %q", msg)
	}
	if strings.Contains(msg, "CategoryInfo") {
		t.Errorf("the PowerShell dump must not reach the operator, got %q", msg)
	}
	// Anything else keeps its original text rather than being explained away.
	if msg := windowsPrintError("the spooler is not running", `C:\spool\job.pdf`); !strings.Contains(msg, "the spooler is not running") {
		t.Errorf("unknown errors must pass through, got %q", msg)
	}
}
