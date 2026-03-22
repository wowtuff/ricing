package toolset

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type execResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

func runCommand(ctx context.Context, timeout time.Duration, name string, args ...string) execResult {
	cmdCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		cmdCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	return execResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode(err),
		Err:      err,
	}
}

func exitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func readCommandValue(ctx context.Context, timeout time.Duration, name string, args ...string) string {
	res := runCommand(ctx, timeout, name, args...)
	if res.Err != nil {
		return ""
	}
	return trimQuotes(strings.TrimSpace(res.Stdout))
}

func fileExists(path string) bool {
	info, err := os.Stat(expandHome(path))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(expandHome(path))
	if err != nil {
		return false
	}
	return info.IsDir()
}

func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func readFirstExisting(paths ...string) (string, string, error) {
	for _, path := range paths {
		absPath := expandHome(path)
		data, err := os.ReadFile(absPath)
		if err == nil {
			return absPath, string(data), nil
		}
	}
	return "", "", fmt.Errorf("no readable file found")
}

func parseKeyValueFile(content string) map[string]string {
	values := map[string]string{}
	for _, line := range splitLines(content) {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		values[key] = trimQuotes(value)
	}
	return values
}

func currentDesktopInfo() (string, string, string, []string) {
	raw := firstNonEmpty(
		os.Getenv("XDG_CURRENT_DESKTOP"),
		os.Getenv("XDG_SESSION_DESKTOP"),
		os.Getenv("DESKTOP_SESSION"),
	)
	sessionType := strings.ToLower(strings.TrimSpace(os.Getenv("XDG_SESSION_TYPE")))
	if sessionType == "" {
		sessionType = "unknown"
	}

	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ':' || r == ';'
	})
	identifiers := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part == "" {
			continue
		}
		identifiers = append(identifiers, part)
	}
	if len(identifiers) == 0 && raw != "" {
		identifiers = append(identifiers, strings.ToLower(strings.TrimSpace(raw)))
	}

	desktop := "unknown"
	for _, id := range identifiers {
		switch {
		case strings.Contains(id, "gnome"):
			desktop = "gnome"
		case strings.Contains(id, "budgie"):
			desktop = "budgie"
		case strings.Contains(id, "cinnamon"):
			desktop = "cinnamon"
		case strings.Contains(id, "kde") || strings.Contains(id, "plasma"):
			desktop = "plasma"
		case strings.Contains(id, "xfce"):
			desktop = "xfce"
		case strings.Contains(id, "sway"):
			desktop = "sway"
		case strings.Contains(id, "hypr"):
			desktop = "hyprland"
		case strings.Contains(id, "river"):
			desktop = "river"
		case strings.Contains(id, "labwc"):
			desktop = "labwc"
		}
		if desktop != "unknown" {
			break
		}
	}

	return desktop, raw, sessionType, identifiers
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func themeModeHint(name string) string {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" {
		return ""
	}
	if strings.Contains(lower, "dark") || strings.Contains(lower, "night") {
		return "dark"
	}
	if strings.Contains(lower, "light") || strings.Contains(lower, "day") {
		return "light"
	}
	return ""
}

func normalizeThemeKey(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"-dark-", "-",
		"_dark_", "_",
		" dark ", " ",
		"-light-", "-",
		"_light_", "_",
		" light ", " ",
		"dark", "",
		"light", "",
		"night", "",
		"day", "",
		"-", "",
		"_", "",
		" ", "",
		".", "",
	)
	return replacer.Replace(name)
}

func uniqueSorted(values []string) []string {
	seen := map[string]string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; !ok {
			seen[key] = trimmed
		}
	}
	out := make([]string, 0, len(seen))
	for _, value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func listDirEntries(paths []string, suffix string, onlyDirs bool) []string {
	entries := []string{}
	for _, searchPath := range paths {
		absPath := expandHome(searchPath)
		dirs, err := os.ReadDir(absPath)
		if err != nil {
			continue
		}
		for _, entry := range dirs {
			if onlyDirs && !entry.IsDir() {
				continue
			}
			if !onlyDirs && entry.IsDir() {
				continue
			}
			name := entry.Name()
			if suffix != "" {
				if !strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
					continue
				}
				name = strings.TrimSuffix(name, suffix)
			}
			entries = append(entries, name)
		}
	}
	return uniqueSorted(entries)
}

func readINIValue(path, section, key string) (string, error) {
	data, err := os.ReadFile(expandHome(path))
	if err != nil {
		return "", err
	}

	currentSection := ""
	for _, line := range splitLines(string(data)) {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			currentSection = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			continue
		}
		if currentSection != section {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx == -1 {
			continue
		}
		currentKey := strings.TrimSpace(trimmed[:idx])
		if strings.EqualFold(currentKey, key) {
			return strings.TrimSpace(trimmed[idx+1:]), nil
		}
	}

	return "", fmt.Errorf("%s:%s not found", section, key)
}

func upsertINIValue(path, section, key, value string) error {
	absPath := expandHome(path)
	lines := []string{}
	if data, err := os.ReadFile(absPath); err == nil {
		lines = splitLines(string(data))
	}

	sectionStart := -1
	sectionEnd := len(lines)
	keyIndex := -1
	currentSection := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if currentSection == section && sectionEnd == len(lines) {
				sectionEnd = i
			}
			currentSection = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
			if currentSection == section && sectionStart == -1 {
				sectionStart = i
			}
			continue
		}
		if currentSection != section {
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ";") {
			continue
		}
		idx := strings.Index(trimmed, "=")
		if idx == -1 {
			continue
		}
		currentKey := strings.TrimSpace(trimmed[:idx])
		if strings.EqualFold(currentKey, key) {
			keyIndex = i
		}
	}

	if sectionStart != -1 && sectionEnd == len(lines) {
		sectionEnd = len(lines)
	}

	entry := fmt.Sprintf("%s=%s", key, value)
	if keyIndex != -1 {
		lines[keyIndex] = entry
	} else if sectionStart != -1 {
		before := append([]string{}, lines[:sectionEnd]...)
		after := append([]string{}, lines[sectionEnd:]...)
		before = append(before, entry)
		lines = append(before, after...)
	} else {
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("[%s]", section), entry)
	}

	dir := filepath.Dir(absPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	out := strings.Join(lines, "\n")
	if out != "" {
		out += "\n"
	}
	return os.WriteFile(absPath, []byte(out), 0o644)
}
