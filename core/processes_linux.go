package core

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

func listProcesses() ([]Process, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, errors.Wrap(err, "list processes")
	}
	var processes []Process
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		directory := filepath.Join("/proc", entry.Name())
		comm, err := os.ReadFile(filepath.Join(directory, "comm"))
		if err != nil {
			continue
		}
		path, _ := os.Readlink(filepath.Join(directory, "exe"))
		name := strings.TrimSpace(string(comm))
		if path != "" {
			// comm is truncated to 15 characters.
			name = filepath.Base(path)
		}
		processes = append(processes, Process{Name: name, Path: path})
	}
	return processes, nil
}
