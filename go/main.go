package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/atotto/clipboard"
)

const (
	pollInterval = 1 * time.Second
	delimiter    = 0x0C // form feed (\f)
)

type clipState struct {
	mu   sync.Mutex
	text string
}

func main() {
	portPath := detectPort()
	if portPath == "" {
		fmt.Fprintf(os.Stderr, "No serial port found.\n")
		fmt.Fprintf(os.Stderr, "Usage: clip98 <serial_port>\n")
		fmt.Fprintf(os.Stderr, "  Override with SERIAL_PORT env var or CLI argument.\n")
		os.Exit(1)
	}

	f, err := os.OpenFile(portPath, os.O_RDWR, 0)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", portPath, err)
	}
	defer f.Close()

	// Disable PTY echo and line processing
	if err := setRawMode(f); err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}

	fmt.Printf("Connected to %s\n", portPath)

	done := make(chan struct{})
	last := &clipState{}

	// Writer: poll host clipboard → send to serial port
	go func() {
		for {
			select {
			case <-done:
				return
			default:
			}

			text, err := clipboard.ReadAll()
			if err != nil {
				time.Sleep(pollInterval)
				continue
			}

			last.mu.Lock()
			if text != "" && text != last.text {
				last.text = text
				// Write string bytes directly — no intermediate buffer truncation
				f.Write([]byte(text))
				// No \f delimiter — Windows ReceiveData reads until serial timeout
				fmt.Printf("[send] %s\n", truncate(text, 60))
			}
			last.mu.Unlock()

			time.Sleep(pollInterval)
		}
	}()

	// Reader: receive from serial port → write to host clipboard
	go func() {
		buf := make([]byte, 4096)
		var pending string
		consecutiveErrors := 0
		for {
			select {
			case <-done:
				return
			default:
			}

			n, err := f.Read(buf)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= 10 {
					fmt.Fprintf(os.Stderr, "Serial port read failed repeatedly: %v\n", err)
					close(done)
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			consecutiveErrors = 0
			if n == 0 {
				continue
			}

			// Buffer incoming data and only process complete messages
			// (terminated by \f from the Windows side)
			pending += string(buf[:n])
			for {
				idx := strings.IndexRune(pending, rune(delimiter))
				if idx == -1 {
					break // No complete message yet, wait for more data
				}
				part := strings.TrimRight(pending[:idx], "\r\n")
				pending = pending[idx+1:]
				if part == "" {
					continue
				}
				// Update shared state under lock, then release before
				// the slow external clipboard call to avoid blocking the writer.
				last.mu.Lock()
				last.text = part
				last.mu.Unlock()
				clipboard.WriteAll(part)
				fmt.Printf("[recv] %s\n", truncate(part, 60))
			}
		}
	}()

	// Wait for Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down...")
	close(done)
}

// detectPort returns the serial port path from env, CLI arg, or auto-detection.
func detectPort() string {
	if p := os.Getenv("SERIAL_PORT"); p != "" {
		return p
	}
	if len(os.Args) > 1 {
		return os.Args[1]
	}
	return autoDetectPty()
}

func truncate(s string, maxLen int) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 && r != 127 { // Keep printable chars including Unicode
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	clean := b.String()
	for strings.Contains(clean, "  ") {
		clean = strings.ReplaceAll(clean, "  ", " ")
	}
	clean = strings.TrimSpace(clean)
	// Count runes, not bytes, for truncation
	runes := []rune(clean)
	if len(runes) <= maxLen {
		return clean
	}
	return string(runes[:maxLen]) + "..."
}
