package agent

import (
	"context"

	"github.com/wowtuff/ricing/tools"
)

type Message struct {
	Role       string
	Content    string
	ToolCallID string
	ToolCalls  []ToolCall
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
