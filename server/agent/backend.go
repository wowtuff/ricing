package agent

import (
	"context"

	"github.com/wowtuff/ricing/tools"
)

type Message struct {
	Role       string
	Content    string
	Images     []ImageInput
	ToolCallID string
	ToolCalls  []ToolCall
}

type ImageInput struct {
	URL    string
	Detail string
}

func defaultInstructions(hasImages bool) string {
	if hasImages {
		return "You are a smart agent. Solve the user's request using the conversation context and the available tools when needed. If the user attached images, those images are already included directly in the conversation input. Analyze attached images visually first, and do not call tools just to open, inspect, or OCR an attached image unless the user explicitly asks for file-level investigation or the provided image input is insufficient."
	}
	return "You are a smart agent. Solve the user's request using the conversation context and the available tools when needed."
}

func messageSetHasImages(messages []Message) bool {
	for _, message := range messages {
		if len(message.Images) > 0 {
			return true
		}
	}
	return false
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments string
}

type CompletionResult struct {
	Content   string
	ToolCalls []ToolCall
}

type Backend interface {
	Complete(ctx context.Context, messages []Message, tools []tools.ToolSpec) (*CompletionResult, error)
}
