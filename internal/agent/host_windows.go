package agent

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// osDescription names the Windows release from the real OS version. It uses
// RtlGetVersion rather than the registry's ProductName, which still reads
// "Windows 10" on Windows 11, or a PowerShell query, which would cost a
// subprocess on the connect path. Build 22000 is where Windows 11 begins.
func osDescription() string {
	v := windows.RtlGetVersion()
	if v == nil {
		return "Windows"
	}
	name := "Windows"
	switch {
	case v.MajorVersion > 10:
		name = fmt.Sprintf("Windows %d", v.MajorVersion)
	case v.MajorVersion == 10 && v.BuildNumber >= 22000:
		name = "Windows 11"
	case v.MajorVersion == 10:
		name = "Windows 10"
	case v.MajorVersion == 6:
		// 6.1 = 7, 6.2 = 8, 6.3 = 8.1; all long out of support but still in shops.
		name = fmt.Sprintf("Windows 6.%d", v.MinorVersion)
	}
	return fmt.Sprintf("%s (build %d)", name, v.BuildNumber)
}
