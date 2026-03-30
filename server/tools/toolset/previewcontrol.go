package toolset

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/wowtuff/ricing/tools"
)

type PreviewControlTool struct{}

func (t *PreviewControlTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "preview_control",
		Description: "Manage the Docker-backed desktop preview environment. Use this after editing files under hyprland/ to build, launch, refresh, inspect, stop, or open a preview in the browser.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"description": "One of: list, build, up, restart, refresh, status, stop, open.",
				},
				"profile": map[string]any{
					"type":        "string",
					"description": "Preview profile id such as arch-hyprland, arch-i3, debian-i3, arch-gnome, arch-plasma, arch-xfce, arch-cinnamon, arch-mate, or arch-lxqt.",
				},
				"rebuild": map[string]any{
					"type":        "boolean",
					"description": "When true, rebuild the Docker image before starting the preview.",
				},
				"open_browser": map[string]any{
					"type":        "boolean",
					"description": "When true, open the running preview URL in a separate browser window or tab.",
				},
				"timeout_seconds": map[string]any{
					"type":        "integer",
					"description": "Optional command timeout in seconds.",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *PreviewControlTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	action := strings.ToLower(strings.TrimSpace(asMapString(args["action"])))
	if action == "" {
		return nil, fmt.Errorf("invalid or missing parameter: action")
	}
	if action == "list" {
		profiles := make([]map[string]any, 0, len(sortedPreviewProfiles()))
		for _, profile := range sortedPreviewProfiles() {
			profiles = append(profiles, map[string]any{
				"id":           profile.ID,
				"label":        profile.Label,
				"distro":       profile.Distro,
				"desktop":      profile.Desktop,
				"session_type": profile.SessionType,
				"dockerfile":   profile.DockerfilePath,
			})
		}
		return map[string]any{
			"ok":          true,
			"action":      action,
			"preview_url": previewURL(),
			"profiles":    profiles,
		}, nil
	}
	profileID := strings.TrimSpace(asMapString(args["profile"]))
	profile, ok := previewProfileByID(profileID)
	if !ok {
		return nil, fmt.Errorf("unknown preview profile: %s", profileID)
	}
	timeout := 10 * time.Minute
	if seconds, ok := asInt(args["timeout_seconds"]); ok && seconds > 0 {
		timeout = time.Duration(seconds) * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	rebuild, _ := args["rebuild"].(bool)
	openBrowserFlag, _ := args["open_browser"].(bool)
	outputs := []string{}
	switch action {
	case "build":
		command := "build"
		if rebuild {
			command = "rebuild"
		}
		out, err := runPreviewCommand(runCtx, command, profile.ID)
		outputs = append(outputs, out)
		if err != nil {
			return previewToolError(action, profile, outputs, openBrowserFlag, err)
		}
	case "up":
		if rebuild {
			out, err := runPreviewCommand(runCtx, "rebuild", profile.ID)
			outputs = append(outputs, out)
			if err != nil {
				return previewToolError(action, profile, outputs, openBrowserFlag, err)
			}
		}
		out, err := runPreviewCommand(runCtx, "up", profile.ID)
		outputs = append(outputs, out)
		if err != nil {
			return previewToolError(action, profile, outputs, openBrowserFlag, err)
		}
	case "restart":
		if rebuild {
			out, err := runPreviewCommand(runCtx, "rebuild", profile.ID)
			outputs = append(outputs, out)
			if err != nil {
				return previewToolError(action, profile, outputs, openBrowserFlag, err)
			}
		}
		out, err := runPreviewCommand(runCtx, "restart", profile.ID)
		outputs = append(outputs, out)
		if err != nil {
			return previewToolError(action, profile, outputs, openBrowserFlag, err)
		}
	case "refresh", "status", "stop":
		out, err := runPreviewCommand(runCtx, action, profile.ID)
		outputs = append(outputs, out)
		if err != nil {
			return previewToolError(action, profile, outputs, openBrowserFlag, err)
		}
	case "open":
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
	opened := false
	if openBrowserFlag || action == "open" {
		if err := openPreviewBrowser(previewURL()); err != nil {
			return previewToolError(action, profile, outputs, true, err)
		}
		opened = true
	}
	status := "ok"
	if action == "stop" {
		status = "stopped"
	} else if action == "status" {
		status = "reported"
	} else if action == "build" {
		status = "built"
	} else {
		status = "running"
	}
	return map[string]any{
		"ok":              true,
		"action":          action,
		"profile":         profile.ID,
		"profile_label":   profile.Label,
		"status":          status,
		"preview_url":     previewURL(),
		"open_browser":    openBrowserFlag || action == "open",
		"opened_browser":  opened,
		"command_output":  strings.TrimSpace(strings.Join(filterNonEmpty(outputs), "\n\n")),
		"dockerfile_path": profile.DockerfilePath,
	}, nil
}

func runPreviewCommand(ctx context.Context, action, profile string) (string, error) {
	bashPath := findBashPath()
	if bashPath == "" {
		return "", fmt.Errorf("bash was not found on this machine")
	}
	scriptPath := previewScriptPath()
	if runtime.GOOS == "windows" {
		scriptPath = filepath.ToSlash(scriptPath)
	}
	cmd := exec.CommandContext(ctx, bashPath, scriptPath, action, "--profile", profile)
	cmd.Dir = repoRootPath()
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func previewToolError(action string, profile previewProfile, outputs []string, openBrowser bool, err error) (map[string]any, error) {
	return map[string]any{
		"ok":              false,
		"action":          action,
		"profile":         profile.ID,
		"profile_label":   profile.Label,
		"status":          "failed",
		"preview_url":     previewURL(),
		"open_browser":    openBrowser,
		"command_output":  strings.TrimSpace(strings.Join(filterNonEmpty(outputs), "\n\n")),
		"dockerfile_path": profile.DockerfilePath,
	}, err
}

func filterNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func openPreviewBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}
