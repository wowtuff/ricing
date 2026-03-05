package toolset

import (
	"context"
	"os/exec"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type NotifyTool struct{}

func (n NotifyTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "notify",
		Description: "Send final result message to a user.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"type":        "string",
					"description": "The notification message",
				},
			},
			"required": []string{"message"},
		},
	}
}

func (n NotifyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	message, ok := args["message"].(string)
	if !ok {
		return nil, utils.LogError("invalid or missing parameter: message")
	}

	cmd := exec.Command("notify-send", message)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status": "sent",
	}, nil
}
