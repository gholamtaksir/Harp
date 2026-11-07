package core

import (
	"path/filepath"
	"sort"
	"strings"
)

// Process is a running program, used to pick applications for rules.
type Process struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// ListProcesses returns the running programs, one entry per executable name.
func ListProcesses() ([]Process, error) {
	processes, err := listProcesses()
	if err != nil {
		return nil, err
	}
	byName := map[string]Process{}
	for _, process := range processes {
		if process.Name == "" && process.Path != "" {
			process.Name = filepath.Base(process.Path)
		}
		if process.Name == "" {
			continue
		}
		key := strings.ToLower(process.Name)
		if existing, ok := byName[key]; !ok || (existing.Path == "" && process.Path != "") {
			byName[key] = process
		}
	}
	result := make([]Process, 0, len(byName))
	for _, process := range byName {
		result = append(result, process)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}
