package agent

import (
	"os"
	"runtime"
)

// osDescription names the distribution from /etc/os-release, the interface every
// systemd-era distribution provides ("Ubuntu 24.04.1 LTS", "Fedora Linux 41").
// /usr/lib/os-release is the vendor copy for systems with no writable /etc.
func osDescription() string {
	for _, path := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if name := osReleaseName(string(b)); name != "" {
			return name
		}
	}
	return runtime.GOOS
}
