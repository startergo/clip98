//go:build darwin

package main

import (
	"os"

	"golang.org/x/sys/unix"
)

// setRawMode disables echo, canonical mode, and signal processing
// on the serial port so data passes through unmodified.
func setRawMode(f *os.File) error {
	fd := int(f.Fd())

	termios, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return err
	}

	// Disable echo, canonical mode, and signal characters
	termios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	// Disable input processing (including software flow control)
	termios.Iflag &^= unix.ICRNL | unix.INLCR | unix.IGNCR | unix.IXON | unix.IXOFF | unix.ISTRIP
	// Disable output processing
	termios.Oflag &^= unix.OPOST

	// Set read timeout so f.Read() doesn't block forever (allows clean shutdown)
	termios.Cc[unix.VMIN] = 0
	termios.Cc[unix.VTIME] = 1 // 100ms timeout

	return unix.IoctlSetTermios(fd, unix.TIOCSETA, termios)
}
