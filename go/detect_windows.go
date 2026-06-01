//go:build windows

package main

import (
	"fmt"
	"os"
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
