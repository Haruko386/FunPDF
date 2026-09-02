//go:build !windows

package service

import "errors"

// pickDirectory is a no-op stub on non-Windows platforms.
func pickDirectory() (string, error) {
	return "", errors.New("folder picker is only supported on Windows")
}
