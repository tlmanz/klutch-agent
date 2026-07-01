//go:build !linux && !windows

package desktop

import "errors"

// errUnsupported is returned on platforms without a launcher-integration
// implementation (only Linux and Windows are shipped).
var errUnsupported = errors.New("installing to applications is not supported on this platform")

func install() (string, error) { return "", errUnsupported }

func uninstall() error { return errUnsupported }

func isInstalled() (bool, error) { return false, nil }

func installedPath() (string, error) { return "", errUnsupported }
