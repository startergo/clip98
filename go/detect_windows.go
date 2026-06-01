//go:build windows

package main

import (
	"fmt"
	"os"
	"strings"
)

func autoDetectPty() string {
	// On Windows with QEMU, -serial pty maps to a named pipe or COM port.
	// Enumerate COM1..COM256 and return the first one that exists.
	for i := 1; i <= 256; i++ {
		name := fmt.Sprintf("COM%d", i)
		f, err := os.OpenFile("\\\\.\\"+name, os.O_RDWR, 0)
		if err == nil {
			f.Close()
			fmt.Printf("Auto-detected serial port: %s\n", name)
			return "\\\\.\\" + name
		}
	}
	return ""
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
