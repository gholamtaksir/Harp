//go:build !windows && !linux

package core

import (
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

func listProcesses() ([]Process, error) {
	output, err := exec.Command("ps", "-axo", "comm=").Output()
	if err != nil {
		return nil, errors.Wrap(err, "list processes")
	}
	var processes []Process
	for _, line := range strings.Split(string(output), "\n") {
		path := strings.TrimSpace(line)
		if path == "" {
			continue
		}
		process := Process{Name: filepath.Base(path)}
		if filepath.IsAbs(path) {
			process.Path = path
		}
		processes = append(processes, process)
	}
	return processes, nil
}
