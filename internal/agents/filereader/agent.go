// Package filereader provides an agent that reads a file or lists a directory
// from a path extracted from a previous step's result.
// Missing paths are not an error — the agent returns empty content/entries instead.
package filereader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	ModeReadFile      = "read_file"
	ModeListDirectory = "list_directory"
)

// Agent reads a file or lists a directory identified by a field in a prior step's JSON result.
type Agent struct {
	name      string
	pathStep  string // step name whose result contains the path
	pathField string // JSON field name holding the path string
	mode      string // ModeReadFile or ModeListDirectory
}

// New creates a file-reader agent.
func New(name, pathStep, pathField, mode string) *Agent {
	return &Agent{name: name, pathStep: pathStep, pathField: pathField, mode: mode}
}

func (a *Agent) Name() string { return a.name }

func (a *Agent) Handle(_ context.Context, task string) (json.RawMessage, error) {
	path, err := a.extractPath(task)
	if err != nil {
		return nil, err
	}

	switch a.mode {
	case ModeReadFile:
		return a.readFile(path)
	case ModeListDirectory:
		return a.listDir(path)
	default:
		return nil, fmt.Errorf("filereader %s: unknown mode %q", a.name, a.mode)
	}
}

func (a *Agent) readFile(path string) (json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return json.Marshal(map[string]string{"content": ""})
	}
	if err != nil {
		return nil, fmt.Errorf("filereader %s: read %s: %w", a.name, path, err)
	}
	return json.Marshal(map[string]string{"content": string(data)})
}

func (a *Agent) listDir(path string) (json.RawMessage, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return json.Marshal(map[string][]string{"entries": {}})
	}
	if err != nil {
		return nil, fmt.Errorf("filereader %s: list %s: %w", a.name, path, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name()+"/")
		} else {
			names = append(names, e.Name())
		}
	}
	return json.Marshal(map[string][]string{"entries": names})
}

func (a *Agent) extractPath(task string) (string, error) {
	sections := parseSections(task)

	raw, ok := sections[a.pathStep]
	if !ok {
		return "", fmt.Errorf("filereader %s: step %q not found in context", a.name, a.pathStep)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return "", fmt.Errorf("filereader %s: parse %s result: %w", a.name, a.pathStep, err)
	}
	path, _ := obj[a.pathField].(string)
	if path == "" {
		return "", fmt.Errorf("filereader %s: field %q not found or empty in %s", a.name, a.pathField, a.pathStep)
	}
	return path, nil
}

func parseSections(task string) map[string]string {
	sections := make(map[string]string)
	const marker = "--- Results from previous steps ---"
	idx := strings.Index(task, marker)
	if idx < 0 {
		return sections
	}
	for _, part := range strings.Split(task[idx+len(marker):], "\n[") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		end := strings.IndexByte(part, ']')
		if end < 0 {
			continue
		}
		sections[part[:end]] = strings.TrimSpace(part[end+1:])
	}
	return sections
}
