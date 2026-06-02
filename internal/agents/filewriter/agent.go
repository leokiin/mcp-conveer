// Package filewriter provides an agent that writes a step result field to a file on disk.
// It extracts project_dir and track_id from the parse-task step, then writes
// {project_dir}/docs/plan/{track_id}/{filename} with the content from another step.
package filewriter

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Agent writes a single file derived from pipeline context.
type Agent struct {
	name         string
	contentStep  string // step name whose result contains the file content
	contentField string // JSON field in that step's result
	filename     string // output filename, e.g. "spec.md"
}

// New creates a file-writer agent.
func New(name, contentStep, contentField, filename string) *Agent {
	return &Agent{
		name:         name,
		contentStep:  contentStep,
		contentField: contentField,
		filename:     filename,
	}
}

func (a *Agent) Name() string { return a.name }

func (a *Agent) Handle(_ context.Context, task string) (json.RawMessage, error) {
	sections := parseSections(task)

	parseRaw, ok := sections["parse-task"]
	if !ok {
		return nil, fmt.Errorf("filewriter %s: parse-task section missing from context", a.name)
	}
	var meta struct {
		ProjectDir string `json:"project_dir"`
		TrackID    string `json:"track_id"`
	}
	if err := json.Unmarshal([]byte(parseRaw), &meta); err != nil {
		return nil, fmt.Errorf("filewriter %s: parse parse-task: %w", a.name, err)
	}
	if meta.ProjectDir == "" || meta.TrackID == "" {
		return nil, fmt.Errorf("filewriter %s: project_dir or track_id empty in parse-task result", a.name)
	}

	contentRaw, ok := sections[a.contentStep]
	if !ok {
		return nil, fmt.Errorf("filewriter %s: step %q missing from context", a.name, a.contentStep)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(contentRaw), &obj); err != nil {
		return nil, fmt.Errorf("filewriter %s: parse %s result: %w", a.name, a.contentStep, err)
	}
	content, ok := obj[a.contentField].(string)
	if !ok {
		return nil, fmt.Errorf("filewriter %s: field %q not found or not a string in %s", a.name, a.contentField, a.contentStep)
	}

	outDir := filepath.Join(meta.ProjectDir, "docs", "plan", meta.TrackID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("filewriter %s: mkdir %s: %w", a.name, outDir, err)
	}
	outPath := filepath.Join(outDir, a.filename)
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		return nil, fmt.Errorf("filewriter %s: write %s: %w", a.name, outPath, err)
	}

	return json.Marshal(map[string]string{"path": outPath, "status": "written"})
}

// parseSections splits the enriched task string built by buildTaskWithContext into
// a map of step_name → raw JSON body.
func parseSections(task string) map[string]string {
	sections := make(map[string]string)

	const marker = "--- Results from previous steps ---"
	idx := strings.Index(task, marker)
	if idx < 0 {
		return sections
	}

	rest := task[idx+len(marker):]
	// Each section starts with "\n[step-name]\n"
	parts := strings.Split(rest, "\n[")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		end := strings.IndexByte(part, ']')
		if end < 0 {
			continue
		}
		name := part[:end]
		body := strings.TrimSpace(part[end+1:])
		sections[name] = body
	}
	return sections
}
