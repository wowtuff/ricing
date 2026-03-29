package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

// config holds backend selection and credentials for a run
type Config struct {
	Backend         string `json:"backend"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	APIKey          string `json:"api_key"`
	URL             string `json:"url"`
}

// runoption is a functional option that modifies a Config before a run
type RunOption func(*Config)

// withconfig returns a RunOption that applies all fields from cfg at once
func WithConfig(cfg Config) RunOption {
	return func(c *Config) {
		c.Backend = cfg.Backend
		c.Model = cfg.Model
		c.ReasoningEffort = cfg.ReasoningEffort
		c.APIKey = cfg.APIKey
		c.URL = cfg.URL
	}
}

// loadconfig reads ~/.ricing/config.json and returns its contents, if the file is missing it returns a sensible default (chatgpt backend)
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

// run is the main agentic loop — it picks the right backend, sends the prompt, and keeps going until the model stops asking for tool calls
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
		backend, err = chatGPT(cfg.Model, cfg.ReasoningEffort)
	case "openai":
		backend, err = restBackendFn("https://api.openai.com/v1", cfg.Model, cfg.APIKey, cfg.ReasoningEffort)
	case "openrouter":
		backend, err = restBackendFn("https://openrouter.ai/api/v1", cfg.Model, cfg.APIKey, cfg.ReasoningEffort)
	case "mistral":
		backend, err = restBackendFn("https://api.mistral.ai/v1", cfg.Model, cfg.APIKey, cfg.ReasoningEffort)
	case "anthropic":
		backend, err = anthropic(cfg.Model, cfg.APIKey)
	case "gemini":
		backend, err = gemini(cfg.Model, cfg.APIKey)
	case "local":
		// cfg.Model = model name, cfg.URL = server URL
		backend, err = restBackendFn(cfg.URL, cfg.Model, "", "")
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

const PingPrompt = "hi! what is your name?"

// Ping sends a test prompt to verify the connection works
// Returns the AI's response or an error
func Ping(ctx context.Context, opts RunOptions) (string, error) {
	var out string
	err := RunStream(ctx, nil, opts, PingPrompt, StreamSink{
		OnDelta: func(text string) {
			out += text
		},
	})
	return out, err
}

// executetoolbyname looks up a tool in the registry by name and runs it, returning the JSON-encoded result (or an error object if anything goes wrong)
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
