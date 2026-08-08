//go:build !linux && !darwin && !windows

package agent

import "runtime"

// osDescription falls back to the platform name on the systems the agent is not
// built for in CI. Reporting "freebsd" is more useful than reporting nothing.
func osDescription() string { return runtime.GOOS }
