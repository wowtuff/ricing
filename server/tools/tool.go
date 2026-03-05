package tools

import (
	"context"
)

// https://developers.openai.com/api/docs/guides/function-calling/

// tools = [
//     {
//         "type": "function",
//         "name": "get_horoscope",
//         "description": "Get today's horoscope for an astrological sign.",
//         "parameters": {
//             "type": "object",
//             "properties": {
//                 "sign": {
//                     "type": "string",
//                     "description": "An astrological sign like Taurus or Aquarius",
//                 },
//             },
//             "required": ["sign"],
//         },
//     },
// ]
//

type ToolSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ParamSchema map[string]any `json:"parameters"`
}

type Tool interface {
	Specs() ToolSpec
	Run(ctx context.Context, args map[string]any) (map[string]any, error)
}
