package toolset

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/wowtuff/ricing/tools"
)

type CmdTool struct{}

func (t *CmdTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "cmd",
		Description: "Execute Cmd commands safely. Can navigate directories, read files, modify user files and configs. Blocks access to sensitive system paths and dangerous operations.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The command to execute (ls, cd, cat, echo, etc.)",
				},
				"args": map[string]any{
					"type":        "array",
					"description": "Command arguments as separate items",
					"items": map[string]any{
						"type": "string",
					},
				},
				"timeout_seconds": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds (default: 30)",
				},
			},
			"required": []string{"command", "args"},
		},
	}
}

func (t *CmdTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	command, ok := args["command"].(string)
	if !ok {
		return nil, fmt.Errorf("command must be a string")
	}

	var cmdArgs []string
	if argsRaw, exists := args["args"]; exists {
		argsSlice, ok := argsRaw.([]interface{})
		if !ok {
			return nil, fmt.Errorf("args must be an array")
		}
		for _, arg := range argsSlice {
			cmdArgs = append(cmdArgs, fmt.Sprintf("%v", arg))
		}
	}

	if err := validateCommand(command, cmdArgs); err != nil {
		return nil, err
	}

	timeout := time.Duration(30) * time.Second
	if timeoutRaw, exists := args["timeout_seconds"]; exists {
		if timeoutVal, ok := timeoutRaw.(float64); ok {
			timeout = time.Duration(timeoutVal) * time.Second
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, command, cmdArgs...)
	output, err := cmd.CombinedOutput()

	result := map[string]any{
		"output": string(output),
		"status": "success",
	}

	if err != nil {
		result["error"] = err.Error()
		result["status"] = "error"
	}

	return result, nil
}

func validateCommand(command string, args []string) error {
	dangerousCommands := map[string]bool{
		"rm":        true,
		"rmdir":     true,
		"mkfs":      true,
		"shred":     true,
		"dd":        true,
		"wipefs":    true,
		"fdisk":     true,
		"parted":    true,
		"format":    true,
		"chroot":    true,
		"mount":     true,
		"umount":    true,
		"sudo":      true,
		"su":        true,
		"passwd":    true,
		"useradd":   true,
		"userdel":   true,
		"usermod":   true,
		"groupadd":  true,
		"groupdel":  true,
		"groupmod":  true,
		"visudo":    true,
		"iptables":  true,
		"ufw":       true,
		"firewall":  true,
		"systemctl": true,
		"service":   true,
		"reboot":    true,
		"shutdown":  true,
		"halt":      true,
		"poweroff":  true,
	}

	if dangerousCommands[command] {
		return fmt.Errorf("command '%s' is not allowed", command)
	}

	allArgs := strings.Join(args, " ")

	dangerousPatterns := []string{
		" -rf ",
		" -r ",
		" -f ",
		"$(rm",
		"`;rm",
		"`rm",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(allArgs, pattern) {
			return fmt.Errorf("dangerous operation pattern detected")
		}
	}

	for _, arg := range args {
		if err := validatePath(arg); err != nil {
			return err
		}
	}

	return nil
}

func validatePath(path string) error {
	expandedPath := path
	if strings.HasPrefix(path, "~") {
		expandedPath = strings.Replace(path, "~", "$HOME", 1)
	}

	var absPath string
	var err error

	if strings.HasPrefix(expandedPath, "/") {
		absPath = expandedPath
	} else {
		absPath, err = filepath.Abs(expandedPath)
		if err != nil {
			return nil
		}
	}

	blockedPaths := []string{
		"/etc/",
		"/sys/",
		"/proc/",
		"/dev/",
		"/root/",
		"/boot/",
		"/bin/",
		"/sbin/",
		"/usr/bin/",
		"/usr/sbin/",
		"/lib/",
		"/lib64/",
		"/usr/lib/",
		"/usr/lib64/",
		"/var/",
		"/.ssh/",
		"/.gnupg/",
		"/.aws/",
		"/.azure/",
		"/.docker/",
		"/.kube/",
		"/opt/",
		"/srv/",
		"/.git/",
		"/.github/",
		"/.gitconfig",
		".env",
		".env.local",
		".env.*.local",
	}

	lowerPath := strings.ToLower(absPath)
	for _, blocked := range blockedPaths {
		if strings.HasPrefix(lowerPath, blocked) || strings.Contains(lowerPath, blocked) {
			return fmt.Errorf("access to path '%s' is not allowed", path)
		}
	}

	sensitiveFiles := []string{
		".ssh",
		".gnupg",
		".aws",
		".azure",
		".docker",
		".kube",
		".env",
		".env.local",
		".gitconfig",
		".credentials",
		".token",
	}

	for _, sensitiveFile := range sensitiveFiles {
		if strings.HasSuffix(lowerPath, sensitiveFile) || strings.Contains(lowerPath, "/"+sensitiveFile) {
			return fmt.Errorf("access to sensitive file '%s' is not allowed", path)
		}
	}

	return nil
}
