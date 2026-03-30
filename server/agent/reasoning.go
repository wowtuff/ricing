package agent

import "strings"

func normalizeReasoningEffort(input string) string {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", "auto", "default":
		return ""
	case "none", "minimal", "low", "medium", "high", "xhigh":
		return strings.ToLower(strings.TrimSpace(input))
	default:
		return ""
	}
}

func reasoningEffortSupported(model string) bool {
	model = strings.ToLower(strings.TrimSpace(model))
	if model == "" {
		return false
	}
	if slash := strings.LastIndex(model, "/"); slash >= 0 {
		model = model[slash+1:]
	}
	return strings.HasPrefix(model, "gpt-5") ||
		strings.HasPrefix(model, "o1") ||
		strings.HasPrefix(model, "o3") ||
		strings.HasPrefix(model, "o4")
}
