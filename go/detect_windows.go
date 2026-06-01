//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
)

func autoDetectPty() string {
	// On Windows with QEMU, -serial pty maps to a named pipe or COM port.
	// Enumerate COM1..COM256 and collect all openable ports.
	var found []string
	for i := 1; i <= 256; i++ {
		name := fmt.Sprintf("COM%d", i)
		f, err := os.OpenFile("\\\\.\\"+name, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			found = append(found, name)
		}
	}

	switch len(found) {
	case 0:
		return ""
	case 1:
		fmt.Printf("Auto-detected serial port: %s\n", found[0])
		return "\\\\.\\" + found[0]
	default:
		fmt.Fprintf(os.Stderr, "Multiple COM ports found: %s\n", strings.Join(found, ", "))
		fmt.Fprintf(os.Stderr, "Cannot auto-detect — specify the port explicitly:\n")
		fmt.Fprintf(os.Stderr, "  clip98 COM3\n  SERIAL_PORT=COM3 clip98\n")
		return ""
	}
}

// normalizePortPath converts COMx to \\.\COMx for os.OpenFile.
func normalizePortPath(p string) string {
	if len(p) >= 4 && strings.EqualFold(p[:3], "COM") {
		return `\\.\` + p
	}
	return p
}

// ownTTY returns "" on Windows (no TTY exclusion needed).
func ownTTY() string {
	return ""
}
