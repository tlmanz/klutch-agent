package agent

import (
	"os"
	"strings"
	"sync"
)

// The host facts the agent reports about itself in Hello: which machine it runs
// on, what it runs on, and which build it is. The dashboard's agent detail
// screen shows them, and until an agent sends them each row reads "Not
// reported". They are display-only: the server never routes or authorizes on
// them, because a device could claim anything here.

// HostFacts is what this machine says about itself.
type HostFacts struct {
	Machine string // hostname
	OS      string // human-readable OS name and version
	Version string // this agent build
}

// hostOnce caches the OS description. Naming the OS costs a file read on Linux
// and a syscall on Windows, and the answer cannot change while the process runs,
// so a reconnect loop backing off against an unreachable server must not repeat
// it on every attempt.
var (
	hostOnce sync.Once
	cachedOS string
)

// hostFacts gathers what this machine reports about itself. Every field is
// best-effort: a machine that will not name itself still gets to print, so a
// failure leaves the field empty and the server keeps whatever it last stored.
func (a *Agent) hostFacts() HostFacts {
	hostOnce.Do(func() { cachedOS = osDescription() })
	machine, _ := os.Hostname()
	return HostFacts{Machine: machine, OS: cachedOS, Version: a.cfg.Version}
}

// osReleaseName pulls the display name out of an /etc/os-release file. The file
// is shell-style KEY=value with optionally quoted values; PRETTY_NAME is the one
// meant for humans ("Ubuntu 24.04.1 LTS"), with NAME+VERSION_ID as the fallback
// for the distributions that omit it.
func osReleaseName(content string) string {
	fields := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		key, val, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		fields[key] = strings.Trim(strings.TrimSpace(val), `"'`)
	}
	if pretty := fields["PRETTY_NAME"]; pretty != "" {
		return pretty
	}
	if name := fields["NAME"]; name != "" {
		return strings.TrimSpace(name + " " + fields["VERSION_ID"])
	}
	return ""
}
