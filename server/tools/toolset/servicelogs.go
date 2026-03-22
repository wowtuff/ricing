package toolset

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type ServiceLogsTool struct{}

type journalQueryResult struct {
	Scope   string
	Entries []map[string]any
	Stderr  string
	Kind    string
	Message string
	OK      bool
}

func (t *ServiceLogsTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "get_service_logs",
		Description: "Read journalctl logs for a specific systemd unit or service.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"service": map[string]any{
					"type":        "string",
					"description": "Service or unit name, for example nginx or ssh.service",
				},
				"scope": map[string]any{
					"type":        "string",
					"description": "system, user, or auto (default)",
				},
				"lines": map[string]any{
					"type":        "integer",
					"description": "Maximum number of log lines to return (default 200, max 500)",
				},
				"since": map[string]any{
					"type":        "string",
					"description": "Optional journalctl --since value, for example 1 hour ago or 2026-03-22 10:00:00",
				},
				"until": map[string]any{
					"type":        "string",
					"description": "Optional journalctl --until value",
				},
				"priority": map[string]any{
					"type":        "string",
					"description": "Optional journal priority or range, for example err, warning, or 3..6",
				},
			},
			"required": []string{"service"},
		},
	}
}

func (t *ServiceLogsTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	service, ok := args["service"].(string)
	if !ok || strings.TrimSpace(service) == "" {
		return nil, utils.LogError("invalid or missing parameter: service")
	}

	unit, err := normalizeUnitName(service)
	if err != nil {
		return map[string]any{
			"ok":      false,
			"service": strings.TrimSpace(service),
			"error":   err.Error(),
		}, nil
	}

	if !commandExists("journalctl") {
		return map[string]any{
			"ok":      false,
			"service": unit,
			"error":   "journalctl is not available on this system",
		}, nil
	}

	if !dirExists("/run/systemd/system") {
		return map[string]any{
			"ok":      false,
			"service": unit,
			"error":   "systemd does not appear to be running as init on this system",
		}, nil
	}

	scope := strings.ToLower(strings.TrimSpace(asMapString(args["scope"])))
	if scope == "" {
		scope = "auto"
	}
	if scope != "auto" && scope != "system" && scope != "user" {
		return nil, utils.LogError("invalid parameter: scope must be auto, system, or user")
	}

	lines := 200
	if n, ok := asInt(args["lines"]); ok {
		lines = n
	}
	if lines < 1 {
		lines = 1
	}
	if lines > 500 {
		lines = 500
	}

	since := strings.TrimSpace(asMapString(args["since"]))
	until := strings.TrimSpace(asMapString(args["until"]))
	priority, err := journalPriorityArg(args["priority"])
	if err != nil {
		return nil, utils.LogError("invalid parameter: priority")
	}

	requestedScope := scope
	tryScopes := []string{scope}
	if scope == "auto" {
		tryScopes = []string{"system", "user"}
	}

	warnings := []string{}
	permissionMessages := []string{}
	var lastResult journalQueryResult

	for _, currentScope := range tryScopes {
		result := queryJournal(ctx, unit, currentScope, lines, since, until, priority)
		lastResult = result

		if result.Stderr != "" && result.Kind != "permission" {
			warnings = append(warnings, result.Stderr)
		}

		switch {
		case result.OK && len(result.Entries) > 0:
			return map[string]any{
				"ok":              true,
				"service":         unit,
				"scope_requested": requestedScope,
				"scope_used":      currentScope,
				"lines_requested": lines,
				"entry_count":     len(result.Entries),
				"entries":         result.Entries,
				"warnings":        uniqueSorted(warnings),
			}, nil

		case result.OK:
			continue

		case result.Kind == "permission":
			permissionMessages = append(permissionMessages, result.Message)
			continue

		case result.Kind == "invalid":
			return map[string]any{
				"ok":              false,
				"service":         unit,
				"scope_requested": requestedScope,
				"scope_used":      currentScope,
				"error":           result.Message,
				"warnings":        uniqueSorted(warnings),
			}, nil

		default:
			warnings = append(warnings, result.Message)
		}
	}

	if len(permissionMessages) > 0 {
		return map[string]any{
			"ok":              false,
			"service":         unit,
			"scope_requested": requestedScope,
			"scope_used":      lastResult.Scope,
			"error":           "permission denied while reading the journal",
			"permission_hint": "try the current user session for user services, or grant access via adm/systemd-journal/wheel/root",
			"warnings":        uniqueSorted(append(warnings, permissionMessages...)),
		}, nil
	}

	return map[string]any{
		"ok":              true,
		"service":         unit,
		"scope_requested": requestedScope,
		"scope_used":      lastResult.Scope,
		"lines_requested": lines,
		"entry_count":     0,
		"entries":         []map[string]any{},
		"warnings":        uniqueSorted(warnings),
	}, nil
}

func normalizeUnitName(service string) (string, error) {
	unit := strings.TrimSpace(service)
	if unit == "" {
		return "", fmt.Errorf("service name is empty")
	}
	if strings.ContainsAny(unit, "/\\ \t\n") {
		return "", fmt.Errorf("service name contains unsupported characters")
	}

	allowedSuffixes := []string{
		".service",
		".socket",
		".timer",
		".target",
		".mount",
		".path",
		".scope",
		".slice",
		".device",
		".automount",
		".swap",
	}
	for _, suffix := range allowedSuffixes {
		if strings.HasSuffix(unit, suffix) {
			return unit, nil
		}
	}
	return unit + ".service", nil
}

func journalPriorityArg(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return strings.TrimSpace(v), nil
	case float64:
		return strconv.Itoa(int(v)), nil
	case int:
		return strconv.Itoa(v), nil
	default:
		return "", fmt.Errorf("unsupported priority value")
	}
}

func queryJournal(ctx context.Context, unit, scope string, lines int, since, until, priority string) journalQueryResult {
	args := []string{"--no-pager", "--output=json", "--unit=" + unit, "--lines=" + strconv.Itoa(lines)}
	if scope == "user" {
		args = append(args, "--user")
		if runtimeDir := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); runtimeDir == "" {
			return journalQueryResult{
				Scope:   scope,
				Kind:    "permission",
				Message: "user journal access requires an active user session",
			}
		}
	}
	if since != "" {
		args = append(args, "--since", since)
	}
	if until != "" {
		args = append(args, "--until", until)
	}
	if priority != "" {
		args = append(args, "--priority", priority)
	}

	res := runCommand(ctx, 10*time.Second, "journalctl", args...)
	stderr := strings.TrimSpace(res.Stderr)
	entries := parseJournalEntries(res.Stdout)

	if res.Err == nil {
		return journalQueryResult{Scope: scope, Entries: entries, Stderr: stderr, OK: true}
	}

	lower := strings.ToLower(stderr)
	switch {
	case strings.Contains(lower, "insufficient permissions"), strings.Contains(lower, "permission denied"), strings.Contains(lower, "not permitted"):
		return journalQueryResult{Scope: scope, Entries: entries, Stderr: stderr, Kind: "permission", Message: firstNonEmpty(stderr, res.Err.Error())}
	case strings.Contains(lower, "invalid argument"), strings.Contains(lower, "failed to add match"), strings.Contains(lower, "bad unit"):
		return journalQueryResult{Scope: scope, Entries: entries, Stderr: stderr, Kind: "invalid", Message: firstNonEmpty(stderr, res.Err.Error())}
	default:
		return journalQueryResult{Scope: scope, Entries: entries, Stderr: stderr, Kind: "error", Message: firstNonEmpty(stderr, res.Err.Error())}
	}
}

func parseJournalEntries(stdout string) []map[string]any {
	entries := []map[string]any{}
	for _, line := range splitLines(stdout) {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		entry := map[string]any{
			"timestamp":     journalTimestamp(raw["__REALTIME_TIMESTAMP"]),
			"message":       journalFieldString(raw["MESSAGE"]),
			"priority":      journalPriorityName(journalFieldString(raw["PRIORITY"])),
			"priority_code": journalFieldString(raw["PRIORITY"]),
			"unit":          firstNonEmpty(journalFieldString(raw["_SYSTEMD_UNIT"]), journalFieldString(raw["UNIT"])),
			"identifier":    journalFieldString(raw["SYSLOG_IDENTIFIER"]),
			"pid":           journalFieldString(raw["_PID"]),
			"comm":          journalFieldString(raw["_COMM"]),
			"transport":     journalFieldString(raw["_TRANSPORT"]),
			"invocation_id": journalFieldString(raw["_SYSTEMD_INVOCATION_ID"]),
		}
		entries = append(entries, entry)
	}
	return entries
}

func journalFieldString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			text := journalFieldString(item)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	case []string:
		return strings.Join(v, " ")
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func journalTimestamp(value any) string {
	text := journalFieldString(value)
	if text == "" {
		return ""
	}
	micros, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return text
	}
	return time.Unix(0, micros*1000).Format(time.RFC3339Nano)
}

func journalPriorityName(code string) string {
	switch strings.TrimSpace(code) {
	case "0":
		return "emerg"
	case "1":
		return "alert"
	case "2":
		return "crit"
	case "3":
		return "err"
	case "4":
		return "warning"
	case "5":
		return "notice"
	case "6":
		return "info"
	case "7":
		return "debug"
	default:
		return code
	}
}
