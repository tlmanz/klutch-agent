package agent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/tlmanz/klutch-agent/wire"
)

func TestOSReleaseName(t *testing.T) {
	ubuntu := `NAME="Ubuntu"
VERSION="24.04.1 LTS (Noble Numbat)"
ID=ubuntu
PRETTY_NAME="Ubuntu 24.04.1 LTS"
VERSION_ID="24.04"`
	if got := osReleaseName(ubuntu); got != "Ubuntu 24.04.1 LTS" {
		t.Errorf("got %q, want the PRETTY_NAME", got)
	}

	// Distributions that omit PRETTY_NAME still get a usable label.
	noPretty := "NAME=\"Alpine Linux\"\nVERSION_ID=3.20.3\n"
	if got := osReleaseName(noPretty); got != "Alpine Linux 3.20.3" {
		t.Errorf("got %q, want NAME + VERSION_ID", got)
	}
	if got := osReleaseName("NAME=Void\n"); got != "Void" {
		t.Errorf("got %q, want the bare name when there is no version", got)
	}
	if got := osReleaseName("# nothing useful here\n"); got != "" {
		t.Errorf("got %q, want empty so the caller can fall back", got)
	}
}

func TestHostFactsAreReported(t *testing.T) {
	a := newTestAgent(t)
	a.cfg.Version = "v1.4.2"
	facts := a.hostFacts()

	host, _ := os.Hostname()
	if facts.Machine != host {
		t.Errorf("machine = %q, want this host's name %q", facts.Machine, host)
	}
	if facts.Version != "v1.4.2" {
		t.Errorf("version = %q, want the build version", facts.Version)
	}
	if strings.TrimSpace(facts.OS) == "" {
		t.Error("OS must never be empty: the dashboard would show 'Not reported'")
	}
}

// The Hello fields are optional on the wire so an older agent stays compatible;
// this pins that they are actually sent when the agent knows them, and omitted
// when it does not.
func TestHelloCarriesHostFacts(t *testing.T) {
	b, err := json.Marshal(wire.Hello{
		Printers: []wire.Printer{{Name: "Receipt"}},
		Machine:  "front-desk-pc",
		OS:       "Ubuntu 24.04.1 LTS",
		Version:  "v1.4.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"machine":"front-desk-pc"`, `"os":"Ubuntu 24.04.1 LTS"`, `"version":"v1.4.2"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("hello JSON %s is missing %s", b, want)
		}
	}

	bare, err := json.Marshal(wire.Hello{Printers: []wire.Printer{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(bare), "machine") {
		t.Errorf("hello JSON %s must omit unknown host facts, not send empty strings", bare)
	}
}
