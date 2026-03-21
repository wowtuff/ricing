package toolset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type ReadFileTool struct{}

func (t *ReadFileTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "read_file",
		Description: "Read a file safely. Can return the full file or a specific inclusive line range.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read",
				},
				"start_line": map[string]any{
					"type":        "integer",
					"description": "1-based start line to read. Optional.",
				},
				"end_line": map[string]any{
					"type":        "integer",
					"description": "1-based inclusive end line to read. Optional.",
				},
			},
			"required": []string{"file_path"},
		},
	}
}

func (t *ReadFileTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	filePath, ok := args["file_path"].(string)
	if !ok || strings.TrimSpace(filePath) == "" {
		return nil, utils.LogError("invalid or missing parameter: file_path")
	}

	if err := validatePath(filePath); err != nil {
		return nil, err
	}

	absPath := expandHome(filePath)
	data, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	lines := splitLines(string(data))
	totalLines := len(lines)

	if totalLines == 0 {
		return map[string]any{
			"file_path":      filePath,
			"total_lines":    0,
			"start_line":     0,
			"end_line":       0,
			"content":        "",
			"numbered_lines": "",
		}, nil
	}

	startLine := 1
	endLine := totalLines

	if n, ok := asInt(args["start_line"]); ok {
		startLine = n
	}
	if n, ok := asInt(args["end_line"]); ok {
		endLine = n
	}

	if startLine < 1 {
		startLine = 1
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine > endLine {
		return nil, fmt.Errorf("invalid line range: start_line (%d) > end_line (%d)", startLine, endLine)
	}

	snippet := lines[startLine-1 : endLine]

	return map[string]any{
		"file_path":      filePath,
		"total_lines":    totalLines,
		"start_line":     startLine,
		"end_line":       endLine,
		"content":        strings.Join(snippet, "\n"),
		"numbered_lines": numberLines(snippet, startLine),
	}, nil
}

func expandHome(path string) string {
	path = os.ExpandEnv(path)

	if path == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}

	return path
}

func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	if s == "" {
		return []string{}
	}
	if strings.HasSuffix(s, "\n") {
		s = strings.TrimSuffix(s, "\n")
	}
	return strings.Split(s, "\n")
}

func numberLines(lines []string, start int) string {
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%d: %s\n", start+i, line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
