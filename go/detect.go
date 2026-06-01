//go:build darwin

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func autoDetectPty() string {
	// Find QEMU PID
	pid := findQemuPID()
	if pid != "" {
		// Find /dev/ptmx fd and extract minor device number from lsof
		output, err := exec.Command("lsof", "-p", pid).Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				cols := strings.Fields(line)
				if len(cols) < 9 {
					continue
				}
				name := cols[len(cols)-1]
				if name != "/dev/ptmx" {
					continue
				}
				// DEVICE column: "15,17" → minor=17 → /dev/ttys017
				device := cols[5]
				parts := strings.SplitN(device, ",", 2)
				if len(parts) != 2 {
					continue
				}
				minor := parts[1]
				slave := fmt.Sprintf("/dev/ttys%s", pad3(minor))
				if _, err := os.Stat(slave); err == nil {
					fmt.Printf("Auto-detected QEMU serial port: %s\n", slave)
					return slave
				}
			}
		}
	}

	// Fallback: newest /dev/ttys* device
	devDir, err := os.ReadDir("/dev")
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`^ttys\d+$`)
	type candidate struct {
		path  string
		mtime int64
	}
	var candidates []candidate

	for _, entry := range devDir {
		if !re.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:  filepath.Join("/dev", entry.Name()),
			mtime: info.ModTime().UnixNano(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].mtime > candidates[j].mtime
	})

	if len(candidates) > 0 {
		fmt.Printf("No QEMU process found, falling back to newest PTY: %s\n", candidates[0].path)
		return candidates[0].path
	}
	return ""
}

func findQemuPID() string {
	if output, err := exec.Command("pgrep", "-f", "qemu-system").Output(); err == nil {
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			return strings.SplitN(pid, "\n", 2)[0]
		}
	}
	// Fallback: pgrep qemu
	if output, err := exec.Command("pgrep", "qemu").Output(); err == nil {
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			return strings.SplitN(pid, "\n", 2)[0]
		}
	}
	return ""
}

func pad3(s string) string {
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}
