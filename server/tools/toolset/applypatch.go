package toolset

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type ApplyPatchTool struct{}

type patchEdit struct {
	StartLine int
	EndLine   int
	NewText   string
}

func (t *ApplyPatchTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "apply_patch",
		Description: "Apply one or more safe line-based edits to a file. Each edit replaces an inclusive line range with new text.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"file_path": map[string]any{
					"type":        "string",
					"description": "Path to the file to modify",
				},
				"edits": map[string]any{
					"type":        "array",
					"description": "List of edits. Each edit replaces start_line..end_line inclusive with new_text.",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"start_line": map[string]any{
								"type":        "integer",
								"description": "1-based inclusive start line",
							},
							"end_line": map[string]any{
								"type":        "integer",
								"description": "1-based inclusive end line",
							},
							"new_text": map[string]any{
								"type":        "string",
								"description": "Replacement text for the specified range. Use empty string to delete the range.",
							},
						},
						"required": []string{"start_line", "end_line", "new_text"},
					},
				},
			},
			"required": []string{"file_path", "edits"},
		},
	}
}

func (t *ApplyPatchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	filePath, ok := args["file_path"].(string)
	if !ok || strings.TrimSpace(filePath) == "" {
		return nil, utils.LogError("invalid or missing parameter: file_path")
	}

	if err := validatePath(filePath); err != nil {
		return nil, err
	}

	rawEdits, ok := args["edits"].([]interface{})
	if !ok || len(rawEdits) == 0 {
		return nil, utils.LogError("invalid or missing parameter: edits")
	}

	edits := make([]patchEdit, 0, len(rawEdits))
	for i, raw := range rawEdits {
		obj, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("edit %d must be an object", i)
		}

		startLine, ok := asInt(obj["start_line"])
		if !ok {
			return nil, fmt.Errorf("edit %d missing or invalid start_line", i)
		}

		endLine, ok := asInt(obj["end_line"])
		if !ok {
			return nil, fmt.Errorf("edit %d missing or invalid end_line", i)
		}

		newText, ok := obj["new_text"].(string)
		if !ok {
			return nil, fmt.Errorf("edit %d missing or invalid new_text", i)
		}

		edits = append(edits, patchEdit{
			StartLine: startLine,
			EndLine:   endLine,
			NewText:   newText,
		})
	}

	absPath := expandHome(filePath)
	originalBytes, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	original := strings.ReplaceAll(string(originalBytes), "\r\n", "\n")
	hadTrailingNewline := strings.HasSuffix(original, "\n")
	lines := splitLines(original)

	if len(lines) == 0 && len(edits) > 0 {
		return nil, fmt.Errorf("cannot apply line-based edits to an empty file")
	}

	for i, e := range edits {
		if e.StartLine < 1 || e.EndLine < e.StartLine || e.EndLine > len(lines) {
			return nil, fmt.Errorf(
				"edit %d has invalid range %d-%d for file with %d lines",
				i, e.StartLine, e.EndLine, len(lines),
			)
		}
	}

	sort.Slice(edits, func(i, j int) bool {
		if edits[i].StartLine == edits[j].StartLine {
			return edits[i].EndLine < edits[j].EndLine
		}
		return edits[i].StartLine < edits[j].StartLine
	})

	for i := 1; i < len(edits); i++ {
		prev := edits[i-1]
		curr := edits[i]
		if curr.StartLine <= prev.EndLine {
			return nil, fmt.Errorf(
				"overlapping edits are not allowed: %d-%d overlaps with %d-%d",
				prev.StartLine, prev.EndLine, curr.StartLine, curr.EndLine,
			)
		}
	}

	sort.Slice(edits, func(i, j int) bool {
		return edits[i].StartLine > edits[j].StartLine
	})

	for _, e := range edits {
		var replacement []string
		if e.NewText != "" {
			replacement = splitLines(e.NewText)
		} else {
			replacement = []string{}
		}

		before := append([]string{}, lines[:e.StartLine-1]...)
		after := append([]string{}, lines[e.EndLine:]...)
		lines = append(before, append(replacement, after...)...)
	}

	out := strings.Join(lines, "\n")
	if hadTrailingNewline {
		out += "\n"
	}

	if err := os.WriteFile(absPath, []byte(out), 0o644); err != nil {
		return nil, err
	}

	return map[string]any{
		"status":        "success",
		"file_path":     filePath,
		"applied_edits": len(edits),
		"total_lines":   len(lines),
	}, nil
}
