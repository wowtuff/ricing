package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type Config struct {
	Backend string `json:"backend"`
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	URL     string `json:"url"`
}

type RunOption func(*Config)

func WithConfig(cfg Config) RunOption {
	return func(c *Config) {
		c.Backend = cfg.Backend
		c.Model = cfg.Model
		c.APIKey = cfg.APIKey
	}
}

func loadConfig() (*Config, error) {
	path := os.ExpandEnv("$HOME/.ricing/config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return &Config{Backend: "chatgpt"}, nil
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("bad config: %w", err)
	}
	return &cfg, nil
}

func Run(ctx context.Context, reg *tools.Registry, userPrompt string, opts ...RunOption) (string, error) {
	cfg, err := loadConfig()
	if err != nil {
		return "", utils.LogError("config error: %s", err)
	}

	for _, opt := range opts {
		opt(cfg)
	}

	var backend Backend
	switch cfg.Backend {
	case "chatgpt", "":
		backend, err = chatGPT()
	case "openai":
		backend, err = restBackendFn("https://api.openai.com/v1", cfg.Model, cfg.APIKey)
	case "openrouter":
		backend, err = restBackendFn("https://openrouter.ai/api/v1", cfg.Model, cfg.APIKey)
	case "mistral":
		backend, err = restBackendFn("https://api.mistral.ai/v1", cfg.Model, cfg.APIKey)
	case "anthropic":
		backend, err = anthropic(cfg.Model, cfg.APIKey)
	case "gemini":
		backend, err = gemini(cfg.Model, cfg.APIKey)
	case "local":
		// cfg.Model = model name, cfg.URL = server URL
		backend, err = restBackendFn(cfg.URL, cfg.Model, "")
	default:
		return "", utils.LogError("unknown backend: %s", cfg.Backend)
	}
	if err != nil {
		return "", utils.LogError("backend error: %s", err)
	}

	messages := []Message{
		{Role: "user", Content: userPrompt},
	}

	toolSpecs := reg.List()

	for {
		result, err := backend.Complete(ctx, messages, toolSpecs)
		if err != nil {
			return "", utils.LogError("completion error: %s", err)
		}

		if len(result.ToolCalls) == 0 {
			return result.Content, nil
		}

		messages = append(messages, Message{
			Role:      "assistant",
			ToolCalls: result.ToolCalls,
		})

		for _, tc := range result.ToolCalls {
			output := executeToolByName(ctx, reg, tc)
			messages = append(messages, Message{
				Role:       "tool",
				Content:    output,
				ToolCallID: tc.ID,
			})
		}
	}
}

func executeToolByName(ctx context.Context, reg *tools.Registry, tc ToolCall) string {
	tool, err := reg.Get(tc.Name)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "bad arguments: %s"}`, err.Error())
	}

	result, err := tool.Run(ctx, args)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	out, _ := json.Marshal(result)
	return string(out)
}
