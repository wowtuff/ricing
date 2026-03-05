package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

// https://pkg.go.dev/github.com/openai/openai-go/v3#section-readme
func Run(ctx context.Context, reg *tools.Registry, userPrompt string) (string, error) {
	// reads from the env
	client := openai.NewClient(
		option.WithAPIKey(""),                              // defaults to os.LookupEnv("OPENAI_API_KEY")
		option.WithBaseURL("https://openrouter.ai/api/v1"), // comment when actual api key to be used
	)
	Tools := buildToolSet(reg)

	messages := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage("You are a smart agent, you provide solutions to user prompts, with no outside knowledge but from the toolset provided to you"),
		openai.UserMessage(userPrompt),
	}

	for {
		completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model:    openai.ChatModelGPT4o,
			Messages: messages,
			Tools:    Tools,
			// MaxTokens: openai.Int(512), //if you using free 4o from openrouter uncomment this, can be removed later once we use our api key
		})
		if err != nil {
			return "", utils.LogError("openai error: %s", err)
		}

		choice := completion.Choices[0]

		if len(choice.Message.ToolCalls) == 0 {
			return choice.Message.Content, nil
		}

		messages = append(messages, choice.Message.ToParam())

		for _, tc := range choice.Message.ToolCalls {
			result := executeTool(ctx, reg, tc)
			messages = append(messages, openai.ToolMessage(result, tc.ID))
		}
	}
}

func executeTool(ctx context.Context, reg *tools.Registry, tc openai.ChatCompletionMessageToolCallUnion) string {
	tool, err := reg.Get(tc.Function.Name)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf(`{"error": "bad arguments: %s"}`, err.Error())
	}

	result, err := tool.Run(ctx, args)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}

	out, _ := json.Marshal(result)
	return string(out)
}

// go openai sdk's version of JSON list of tools
func buildToolSet(reg *tools.Registry) []openai.ChatCompletionToolUnionParam {
	specs := reg.List()
	out := make([]openai.ChatCompletionToolUnionParam, 0, len(specs))

	for _, spec := range specs {
		out = append(out, openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
			Name:        spec.Name,
			Description: openai.String(spec.Description),
			Parameters:  openai.FunctionParameters(spec.ParamSchema),
		}))
	}

	return out
}
