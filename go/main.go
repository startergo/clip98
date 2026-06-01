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

	if err := setRawMode(f); err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}

	fmt.Printf("Connected to %s\n", portPath)

	done := make(chan struct{})
	closeOnce := sync.Once{}
	shutdown := func() {
		closeOnce.Do(func() { close(done) })
	}
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
				// Write all bytes, handling partial writes
				if err := writeAll(f, []byte(text)); err != nil {
					fmt.Fprintf(os.Stderr, "Serial write failed: %v\n", err)
					last.mu.Unlock()
					shutdown()
					return
				}
				// Only update state after successful write
				last.text = text
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
					shutdown()
					return
				}
				time.Sleep(100 * time.Millisecond)
				continue
			}
			consecutiveErrors = 0
			if n == 0 {
				// VMIN=0 returns (0, nil) when idle — don't busy-spin
				time.Sleep(10 * time.Millisecond)
				continue
			}

			pending += string(buf[:n])
			for {
				idx := strings.IndexRune(pending, rune(delimiter))
				if idx == -1 {
					break
				}
				part := pending[:idx]
				pending = pending[idx+1:]
				if part == "" {
					continue
				}
				if err := clipboard.WriteAll(part); err != nil {
					fmt.Fprintf(os.Stderr, "Clipboard write failed: %v\n", err)
					continue
				}
				// Only update state after successful clipboard write
				last.mu.Lock()
				last.text = part
				last.mu.Unlock()
				fmt.Printf("[recv] %s\n", truncate(part, 60))
			}
		}
	}()

	// Wait for either Ctrl+C or reader-initiated shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sig:
		fmt.Println("\nShutting down...")
	case <-done:
		fmt.Println("\nConnection lost.")
	}
	shutdown()
}

// writeAll writes all bytes, handling partial writes.
func writeAll(f *os.File, data []byte) error {
	for len(data) > 0 {
		n, err := f.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

// detectPort returns the serial port path from env, CLI arg, or auto-detection.
func detectPort() string {
	if p := os.Getenv("SERIAL_PORT"); p != "" {
		return normalizePortPath(p)
	}
	if len(os.Args) > 1 {
		return normalizePortPath(os.Args[1])
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
	runes := []rune(clean)
	if len(runes) <= maxLen {
		return clean
	}
	return string(runes[:maxLen]) + "..."
}
