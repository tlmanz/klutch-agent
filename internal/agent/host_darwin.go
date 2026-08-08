package agent

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// osDescription asks sw_vers for the product name and version ("macOS 15.1").
// It runs once per process (see hostOnce), so the subprocess cost is paid at the
// first connect and never again.
func osDescription() string {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	name := swVers(ctx, "-productName")
	version := swVers(ctx, "-productVersion")
	switch {
	case name != "" && version != "":
		return name + " " + version
	case name != "":
		return name
	default:
		return "macOS"
	}
}

func swVers(ctx context.Context, arg string) string {
	out, err := exec.CommandContext(ctx, "sw_vers", arg).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
