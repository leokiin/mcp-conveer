// Package multireader provides an agent that reads multiple source files
// whose paths are listed in a previous step's JSON array field.
// Results are truncated to maxLines per file to keep context manageable.
package multireader

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	defaultMaxFiles = 10
	defaultMaxLines = 200
)

// FileContent holds the path and (possibly truncated) content of one file.
type FileContent struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Truncated bool  `json:"truncated,omitempty"`
}

// Agent reads a list of file paths from a previous step's JSON field
// and returns their contents.
type Agent struct {
	name       string
	filesStep  string // step name whose result contains the file list
	filesField string // JSON field name holding []string of paths
	maxFiles   int
	maxLines   int
}

// New creates a multi-file-reader agent.
func New(name, filesStep, filesField string, maxFiles, maxLines int) *Agent {
	if maxFiles <= 0 {
		maxFiles = defaultMaxFiles
	}
	if maxLines <= 0 {
		maxLines = defaultMaxLines
	}
	return &Agent{
		name:       name,
		filesStep:  filesStep,
		filesField: filesField,
		maxFiles:   maxFiles,
		maxLines:   maxLines,
	}
}

func (a *Agent) Name() string { return a.name }

func (a *Agent) Handle(_ context.Context, task string) (json.RawMessage, error) {
	paths, err := a.extractPaths(task)
	if err != nil {
		return nil, err
	}

	if len(paths) > a.maxFiles {
		paths = paths[:a.maxFiles]
	}

	results := make([]FileContent, 0, len(paths))
	for _, p := range paths {
		fc, err := a.readFile(p)
		if err != nil {
			// Non-fatal: log missing/unreadable files as empty content.
			results = append(results, FileContent{Path: p, Content: fmt.Sprintf("[error: %s]", err.Error())})
			continue
		}
		results = append(results, fc)
	}

	return json.Marshal(map[string][]FileContent{"files": results})
}

func (a *Agent) readFile(path string) (FileContent, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileContent{}, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) == a.maxLines {
			// Drain to check if file has more content.
			if scanner.Scan() {
				return FileContent{
					Path:      path,
					Content:   strings.Join(lines, "\n"),
					Truncated: true,
				}, nil
			}
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return FileContent{}, err
	}
	return FileContent{Path: path, Content: strings.Join(lines, "\n")}, nil
}

func (a *Agent) extractPaths(task string) ([]string, error) {
	sections := parseSections(task)

	raw, ok := sections[a.filesStep]
	if !ok {
		return nil, fmt.Errorf("multireader %s: step %q not found in context", a.name, a.filesStep)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, fmt.Errorf("multireader %s: parse %s result: %w", a.name, a.filesStep, err)
	}

	fieldRaw, ok := obj[a.filesField]
	if !ok {
		return nil, fmt.Errorf("multireader %s: field %q not found in %s", a.name, a.filesField, a.filesStep)
	}

	var paths []string
	if err := json.Unmarshal(fieldRaw, &paths); err != nil {
		return nil, fmt.Errorf("multireader %s: field %q is not a string array: %w", a.name, a.filesField, err)
	}
	return paths, nil
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
