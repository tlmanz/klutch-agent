//go:build windows

package agent

import "os"

// swapFile replaces dst with src on Windows, where a running .exe cannot be
// deleted or overwritten but CAN be renamed. So we move the running binary aside
// to "<dst>.old", then rename the new binary into place; the leftover ".old" is
// cleaned up on the next start (RemoveOldBinary). On failure it rolls the old
// binary back so the app is never left without an executable.
func swapFile(src, dst string) error {
	old := dst + ".old"
	_ = os.Remove(old)
	if err := os.Rename(dst, old); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		_ = os.Rename(old, dst) // roll back
		return err
	}
	return nil
}
