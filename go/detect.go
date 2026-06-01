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
	// Try each QEMU PID until we find one with a serial PTY
	pids := findQemuPIDs()
	for _, pid := range pids {
		output, err := exec.Command("lsof", "-p", pid).Output()
		if err != nil {
			continue
		}
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

	// Fallback: newest /dev/ttys* device (excluding own terminal)
	own := ownTTY()
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
		fullPath := filepath.Join("/dev", entry.Name())
		if fullPath == own {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:  fullPath,
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

func findQemuPIDs() []string {
	for _, args := range [][]string{{"pgrep", "-f", "qemu-system"}, {"pgrep", "qemu"}} {
		if output, err := exec.Command(args[0], args[1:]...).Output(); err == nil {
			pids := strings.Fields(strings.TrimSpace(string(output)))
			if len(pids) > 0 {
				return pids
			}
		}
	}
	return nil
}

func pad3(s string) string {
	for len(s) < 3 {
		s = "0" + s
	}
	return s
}

// normalizePortPath is a no-op on macOS.
func normalizePortPath(p string) string {
	return p
}

// ownTTY returns the path of this process's controlling terminal, or "".
func ownTTY() string {
	p, err := os.Readlink("/dev/fd/0")
	if err == nil && strings.HasPrefix(p, "/dev/") {
		return p
	}
	return ""
}
