//go:build windows

package main

import "os"

// setRawMode is a no-op on Windows — COM ports are already raw.
func setRawMode(f *os.File) error {
	return nil
}
