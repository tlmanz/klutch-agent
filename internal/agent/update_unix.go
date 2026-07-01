//go:build !windows

package agent

import "os"

// swapFile atomically replaces dst with src. On Unix a rename over an open file
// succeeds: the running process keeps executing the old inode, and the relaunched
// process picks up the new one.
func swapFile(src, dst string) error {
	return os.Rename(src, dst)
}
