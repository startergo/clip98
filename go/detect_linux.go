//go:build linux

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
	pid := findQemuPID()
	if pid != "" {
		output, err := exec.Command("lsof", "-p", pid).Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				cols := strings.Fields(line)
				if len(cols) < 9 {
					continue
				}
				fd := cols[3]
				name := cols[len(cols)-1]
				// Skip fd 0-2 (stdin/stdout/stderr = controlling terminal).
				// Must check second char is a letter to avoid matching fd 10, 11, 12...
				if len(fd) >= 2 && (fd[0] == '0' || fd[0] == '1' || fd[0] == '2') && !isDigit(fd[1]) {
					continue
				}
				if strings.HasPrefix(name, "/dev/pts/") {
					fmt.Printf("Auto-detected QEMU serial port: %s\n", name)
					return name
				}
			}
		}
	}

	// Fallback: newest /dev/pts/* device
	ptsDir, err := os.ReadDir("/dev/pts")
	if err != nil {
		return ""
	}

	re := regexp.MustCompile(`^\d+$`)
	type candidate struct {
		path  string
		mtime int64
	}
	var candidates []candidate

	for _, entry := range ptsDir {
		if !re.MatchString(entry.Name()) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:  filepath.Join("/dev/pts", entry.Name()),
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
	if output, err := exec.Command("pgrep", "qemu").Output(); err == nil {
		pid := strings.TrimSpace(string(output))
		if pid != "" {
			return strings.SplitN(pid, "\n", 2)[0]
		}
	}
	return ""
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}
