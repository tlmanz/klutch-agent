//go:build !windows

package agent

import "fmt"

// printRaw exists only so dispatch.go compiles everywhere. Off Windows a raw job
// goes to a raw CUPS queue through `lp`, which needs no special case.
func printRaw(printer, path, doc string) error {
	return fmt.Errorf("raw spooler printing is only used on Windows")
}
