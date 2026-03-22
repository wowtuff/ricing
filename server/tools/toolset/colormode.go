package toolset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/wowtuff/ricing/tools"
	"github.com/wowtuff/ricing/utils"
)

type ColorModeTool struct{}

func (t *ColorModeTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "set_color_mode",
		Description: "Switch Linux applications and desktop settings between dark and light mode.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"mode": map[string]any{
					"type":        "string",
					"description": "Target mode: dark or light",
				},
				"gtk_theme": map[string]any{
					"type":        "string",
					"description": "Optional exact GTK theme to apply for this switch",
				},
				"gtk_theme_dark": map[string]any{
					"type":        "string",
					"description": "Optional preferred GTK theme to use when mode=dark",
				},
				"gtk_theme_light": map[string]any{
					"type":        "string",
					"description": "Optional preferred GTK theme to use when mode=light",
				},
				"desktop": map[string]any{
					"type":        "string",
					"description": "Optional desktop override: auto, gnome, budgie, cinnamon, plasma, xfce, or generic",
				},
			},
			"required": []string{"mode"},
		},
	}
}

func (t *ColorModeTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	mode := strings.ToLower(strings.TrimSpace(asMapString(args["mode"])))
	if mode != "dark" && mode != "light" {
		return nil, utils.LogError("invalid or missing parameter: mode")
	}

	desktopArg := strings.ToLower(strings.TrimSpace(asMapString(args["desktop"])))
	if desktopArg == "" {
		desktopArg = "auto"
	}
	if !containsString([]string{"auto", "gnome", "budgie", "cinnamon", "plasma", "xfce", "generic"}, desktopArg) {
		return nil, utils.LogError("invalid parameter: desktop")
	}

	detectedDesktop, detectedRaw, sessionType, identifiers := currentDesktopInfo()
	effectiveDesktop := detectedDesktop
	if desktopArg != "auto" {
		effectiveDesktop = desktopArg
	}
	if effectiveDesktop == "unknown" || effectiveDesktop == "sway" || effectiveDesktop == "hyprland" || effectiveDesktop == "river" || effectiveDesktop == "labwc" {
		effectiveDesktop = "generic"
	}

	before := collectAppearanceState(ctx)
	actions := []map[string]any{}
	warnings := []string{}
	restartHints := []string{}
	availableGTKThemes := gtkThemeNames()
	gtkThemeExact := trimQuotes(strings.TrimSpace(asMapString(args["gtk_theme"])))
	gtkThemeDark := trimQuotes(strings.TrimSpace(asMapString(args["gtk_theme_dark"])))
	gtkThemeLight := trimQuotes(strings.TrimSpace(asMapString(args["gtk_theme_light"])))

	targetGTKTheme, gtkThemeSource, gtkThemeWarnings := resolveGTKTargetTheme(ctx, mode, gtkThemeExact, gtkThemeDark, gtkThemeLight, availableGTKThemes)
	warnings = append(warnings, gtkThemeWarnings...)

	switch effectiveDesktop {
	case "gnome", "budgie":
		applyGNOMEColorScheme(ctx, mode, &actions, &warnings)
		applyGTKThemeGSettings(ctx, targetGTKTheme, &actions, &warnings)
	case "cinnamon":
		applyGNOMEColorScheme(ctx, mode, &actions, &warnings)
		applyCinnamonThemes(ctx, mode, targetGTKTheme, availableGTKThemes, &actions, &warnings)
	case "xfce":
		applyGNOMEColorScheme(ctx, mode, &actions, &warnings)
		applyXfceThemes(ctx, mode, targetGTKTheme, availableGTKThemes, &actions, &warnings)
	case "plasma":
		applyPlasmaColorScheme(ctx, mode, &actions, &warnings)
		applyPlasmaDesktopTheme(ctx, mode, &actions, &warnings)
		applyGNOMEColorScheme(ctx, mode, &actions, &warnings)
		applyGTKThemeGSettings(ctx, targetGTKTheme, &actions, &warnings)
	default:
		applyGNOMEColorScheme(ctx, mode, &actions, &warnings)
		applyGTKThemeGSettings(ctx, targetGTKTheme, &actions, &warnings)
	}

	applyGTKSettingsFiles(mode, targetGTKTheme, &actions, &warnings)
	applyQtctPalette(ctx, "qt5ct", mode, &actions, &warnings, &restartHints)
	applyQtctPalette(ctx, "qt6ct", mode, &actions, &warnings, &restartHints)

	after := collectAppearanceState(ctx)
	ok := hasSuccessfulColorAction(actions)
	if !ok && len(warnings) == 0 {
		warnings = append(warnings, "no compatible desktop or toolkit backend was updated")
	}

	return map[string]any{
		"ok":                  ok,
		"mode":                mode,
		"desktop_requested":   desktopArg,
		"desktop_detected":    detectedDesktop,
		"desktop_effective":   effectiveDesktop,
		"desktop_raw":         detectedRaw,
		"desktop_identifiers": identifiers,
		"session_type":        sessionType,
		"target_gtk_theme":    targetGTKTheme,
		"gtk_theme_selection": map[string]any{
			"source":          gtkThemeSource,
			"requested":       gtkThemeExact,
			"requested_dark":  gtkThemeDark,
			"requested_light": gtkThemeLight,
		},
		"available_gtk_themes":         availableGTKThemes,
		"available_gtk_themes_by_mode": groupThemesByMode(availableGTKThemes),
		"applied_backends":             actions,
		"warnings":                     uniqueSorted(warnings),
		"restart_recommended_for":      uniqueSorted(restartHints),
		"before":                       before,
		"after":                        after,
	}, nil
}

func applyGNOMEColorScheme(ctx context.Context, mode string, actions *[]map[string]any, warnings *[]string) {
	if !commandExists("gsettings") {
		addColorAction(actions, "gsettings:org.gnome.desktop.interface/color-scheme", "skipped", "gsettings not available")
		return
	}

	current := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.gnome.desktop.interface", "color-scheme")
	target := "prefer-dark"
	fallback := ""
	if mode == "light" {
		target = "prefer-light"
		fallback = "default"
	}

	if current == target || (fallback != "" && current == fallback) {
		addColorAction(actions, "gsettings:org.gnome.desktop.interface/color-scheme", "unchanged", current)
		return
	}

	res := runCommand(ctx, 3*time.Second, "gsettings", "set", "org.gnome.desktop.interface", "color-scheme", target)
	usedValue := target
	if res.Err != nil && fallback != "" {
		fallbackRes := runCommand(ctx, 3*time.Second, "gsettings", "set", "org.gnome.desktop.interface", "color-scheme", fallback)
		if fallbackRes.Err == nil {
			res = fallbackRes
			usedValue = fallback
		}
	}

	if res.Err != nil {
		if isMissingSettingsBackendError(res.Stderr) {
			addColorAction(actions, "gsettings:org.gnome.desktop.interface/color-scheme", "skipped", strings.TrimSpace(res.Stderr))
			return
		}
		addColorAction(actions, "gsettings:org.gnome.desktop.interface/color-scheme", "failed", strings.TrimSpace(res.Stderr))
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
		return
	}

	addColorAction(actions, "gsettings:org.gnome.desktop.interface/color-scheme", "changed", usedValue)
}

func applyGTKThemeGSettings(ctx context.Context, theme string, actions *[]map[string]any, warnings *[]string) {
	if strings.TrimSpace(theme) == "" {
		addColorAction(actions, "gsettings:org.gnome.desktop.interface/gtk-theme", "skipped", "no matching GTK theme variant found")
		return
	}
	applyGSettingsString(ctx, "gsettings:org.gnome.desktop.interface/gtk-theme", "org.gnome.desktop.interface", "gtk-theme", theme, actions, warnings)
}

func applyCinnamonThemes(ctx context.Context, mode, targetGTKTheme string, availableThemes []string, actions *[]map[string]any, warnings *[]string) {
	gtkCurrent := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.desktop.interface", "gtk-theme")
	gtkTarget := firstNonEmpty(resolveProvidedThemeName(targetGTKTheme, availableThemes), chooseVariantName(gtkCurrent, mode, availableThemes, gtkThemeDefaults(mode)))
	if gtkTarget == "" {
		addColorAction(actions, "gsettings:org.cinnamon.desktop.interface/gtk-theme", "skipped", "no matching Cinnamon GTK theme variant found")
	} else {
		applyGSettingsString(ctx, "gsettings:org.cinnamon.desktop.interface/gtk-theme", "org.cinnamon.desktop.interface", "gtk-theme", gtkTarget, actions, warnings)
	}

	wmCurrent := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.desktop.wm.preferences", "theme")
	wmTarget := chooseVariantName(firstNonEmpty(gtkTarget, wmCurrent), mode, availableThemes, gtkThemeDefaults(mode))
	if wmTarget == "" {
		addColorAction(actions, "gsettings:org.cinnamon.desktop.wm.preferences/theme", "skipped", "no matching Cinnamon window theme variant found")
	} else {
		applyGSettingsString(ctx, "gsettings:org.cinnamon.desktop.wm.preferences/theme", "org.cinnamon.desktop.wm.preferences", "theme", wmTarget, actions, warnings)
	}

	shellCurrent := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.theme", "name")
	shellTarget := chooseVariantName(firstNonEmpty(gtkTarget, shellCurrent), mode, availableThemes, gtkThemeDefaults(mode))
	if shellTarget == "" {
		addColorAction(actions, "gsettings:org.cinnamon.theme/name", "skipped", "no matching Cinnamon shell theme variant found")
	} else {
		applyGSettingsString(ctx, "gsettings:org.cinnamon.theme/name", "org.cinnamon.theme", "name", shellTarget, actions, warnings)
	}
}

func applyXfceThemes(ctx context.Context, mode, targetGTKTheme string, availableThemes []string, actions *[]map[string]any, warnings *[]string) {
	gtkCurrent := readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", "xsettings", "-p", "/Net/ThemeName")
	gtkTarget := firstNonEmpty(resolveProvidedThemeName(targetGTKTheme, availableThemes), chooseVariantName(gtkCurrent, mode, availableThemes, gtkThemeDefaults(mode)))
	if gtkTarget == "" {
		addColorAction(actions, "xfconf:xsettings:/Net/ThemeName", "skipped", "no matching Xfce GTK theme variant found")
	} else {
		applyXfconfString(ctx, "xfconf:xsettings:/Net/ThemeName", "xsettings", "/Net/ThemeName", gtkTarget, actions, warnings)
	}

	wmCurrent := readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", "xfwm4", "-p", "/general/theme")
	wmTarget := chooseVariantName(firstNonEmpty(gtkTarget, wmCurrent), mode, availableThemes, gtkThemeDefaults(mode))
	if wmTarget == "" {
		addColorAction(actions, "xfconf:xfwm4:/general/theme", "skipped", "no matching Xfce window theme variant found")
	} else {
		applyXfconfString(ctx, "xfconf:xfwm4:/general/theme", "xfwm4", "/general/theme", wmTarget, actions, warnings)
	}
}

func applyPlasmaColorScheme(ctx context.Context, mode string, actions *[]map[string]any, warnings *[]string) {
	current := firstNonEmpty(
		readCommandValue(ctx, 2*time.Second, "kreadconfig6", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme"),
		readCommandValue(ctx, 2*time.Second, "kreadconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme"),
		trimQuotes(readINIOrEmpty("~/.config/kdeglobals", "General", "ColorScheme")),
	)
	available := plasmaColorSchemes()
	target := chooseVariantName(current, mode, available, plasmaColorSchemeDefaults(mode))
	if target == "" {
		addColorAction(actions, "plasma:colorscheme", "skipped", "no matching Plasma color scheme found")
		return
	}
	if current == target {
		addColorAction(actions, "plasma:colorscheme", "unchanged", target)
		return
	}

	if commandExists("plasma-apply-colorscheme") {
		res := runCommand(ctx, 5*time.Second, "plasma-apply-colorscheme", target)
		if res.Err == nil {
			addColorAction(actions, "plasma:colorscheme", "changed", target)
			return
		}
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
	}

	kwrite := firstNonEmpty(findCommand("kwriteconfig6"), findCommand("kwriteconfig5"))
	if kwrite != "" {
		res := runCommand(ctx, 5*time.Second, kwrite, "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme", target, "--notify")
		if res.Err == nil {
			notifyKDEPaletteChange(ctx, warnings)
			addColorAction(actions, "plasma:colorscheme", "changed", target)
			return
		}
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
	}

	status, detail, err := applyINIValues("~/.config/kdeglobals", "General", map[string]string{"ColorScheme": target})
	if err != nil {
		addColorAction(actions, "plasma:colorscheme", "failed", err.Error())
		addWarning(warnings, err.Error())
		return
	}
	notifyKDEPaletteChange(ctx, warnings)
	addColorAction(actions, "plasma:colorscheme", status, detail)
}

func applyPlasmaDesktopTheme(ctx context.Context, mode string, actions *[]map[string]any, warnings *[]string) {
	current := trimQuotes(readINIOrEmpty("~/.config/plasmarc", "Theme", "name"))
	available := plasmaDesktopThemes()
	target := chooseVariantName(current, mode, available, plasmaDesktopThemeDefaults(mode))
	if target == "" {
		addColorAction(actions, "plasma:desktoptheme", "skipped", "no matching Plasma desktop theme found")
		return
	}
	if current == target {
		addColorAction(actions, "plasma:desktoptheme", "unchanged", target)
		return
	}

	if commandExists("plasma-apply-desktoptheme") {
		res := runCommand(ctx, 5*time.Second, "plasma-apply-desktoptheme", target)
		if res.Err == nil {
			addColorAction(actions, "plasma:desktoptheme", "changed", target)
			return
		}
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
	}

	status, detail, err := applyINIValues("~/.config/plasmarc", "Theme", map[string]string{"name": target})
	if err != nil {
		addColorAction(actions, "plasma:desktoptheme", "failed", err.Error())
		addWarning(warnings, err.Error())
		return
	}
	addColorAction(actions, "plasma:desktoptheme", status, detail)
}

func applyGTKSettingsFiles(mode, targetTheme string, actions *[]map[string]any, warnings *[]string) {
	preferDark := "false"
	if mode == "dark" {
		preferDark = "true"
	}

	settings := map[string]string{
		"gtk-application-prefer-dark-theme": preferDark,
	}
	if targetTheme != "" {
		settings["gtk-theme-name"] = targetTheme
	}

	paths := map[string]string{
		"gtk-settings:gtk3": "~/.config/gtk-3.0/settings.ini",
		"gtk-settings:gtk4": "~/.config/gtk-4.0/settings.ini",
	}
	for backend, path := range paths {
		status, detail, err := applyINIValues(path, "Settings", settings)
		if err != nil {
			addColorAction(actions, backend, "failed", err.Error())
			addWarning(warnings, err.Error())
			continue
		}
		addColorAction(actions, backend, status, detail)
	}
}

func applyQtctPalette(ctx context.Context, toolName, mode string, actions *[]map[string]any, warnings *[]string, restartHints *[]string) {
	configPath := fmt.Sprintf("~/.config/%s/%s.conf", toolName, toolName)
	backend := fmt.Sprintf("%s:palette", toolName)
	if !fileExists(configPath) {
		addColorAction(actions, backend, "skipped", "config file not present")
		return
	}

	currentPath := trimQuotes(readINIOrEmpty(configPath, "Appearance", "color_scheme_path"))
	currentName := strings.TrimSuffix(filepath.Base(currentPath), filepath.Ext(currentPath))
	available := qtctPaletteNames(toolName)
	targetName := chooseVariantName(currentName, mode, available, qtctPaletteDefaults(mode))
	if targetName == "" {
		addColorAction(actions, backend, "skipped", "no matching Qt palette found")
		return
	}
	targetPath := resolveNamedFile(qtctPalettePaths(toolName), targetName, ".conf")
	if targetPath == "" {
		addColorAction(actions, backend, "skipped", "target palette file not found")
		return
	}

	status, detail, err := applyINIValues(configPath, "Appearance", map[string]string{
		"custom_palette":    "true",
		"color_scheme_path": targetPath,
	})
	if err != nil {
		addColorAction(actions, backend, "failed", err.Error())
		addWarning(warnings, err.Error())
		return
	}
	addColorAction(actions, backend, status, detail)
	*restartHints = append(*restartHints, "some Qt applications may need a restart to pick up the new palette")
	_ = ctx
}

func applyGSettingsString(ctx context.Context, backend, schema, key, value string, actions *[]map[string]any, warnings *[]string) {
	if !commandExists("gsettings") {
		addColorAction(actions, backend, "skipped", "gsettings not available")
		return
	}
	current := readCommandValue(ctx, 2*time.Second, "gsettings", "get", schema, key)
	if current == value {
		addColorAction(actions, backend, "unchanged", value)
		return
	}
	res := runCommand(ctx, 3*time.Second, "gsettings", "set", schema, key, value)
	if res.Err != nil {
		if isMissingSettingsBackendError(res.Stderr) {
			addColorAction(actions, backend, "skipped", strings.TrimSpace(res.Stderr))
			return
		}
		addColorAction(actions, backend, "failed", strings.TrimSpace(res.Stderr))
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
		return
	}
	addColorAction(actions, backend, "changed", value)
}

func applyXfconfString(ctx context.Context, backend, channel, property, value string, actions *[]map[string]any, warnings *[]string) {
	if !commandExists("xfconf-query") {
		addColorAction(actions, backend, "skipped", "xfconf-query not available")
		return
	}
	current := readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", channel, "-p", property)
	if current == value {
		addColorAction(actions, backend, "unchanged", value)
		return
	}
	res := runCommand(ctx, 3*time.Second, "xfconf-query", "-c", channel, "-p", property, "-s", value)
	if res.Err != nil && strings.Contains(strings.ToLower(res.Stderr), "property does not exist") {
		res = runCommand(ctx, 3*time.Second, "xfconf-query", "-c", channel, "-p", property, "-n", "-t", "string", "-s", value)
	}
	if res.Err != nil {
		addColorAction(actions, backend, "failed", strings.TrimSpace(res.Stderr))
		addWarning(warnings, firstNonEmpty(res.Stderr, res.Err.Error()))
		return
	}
	addColorAction(actions, backend, "changed", value)
}

func applyINIValues(path, section string, values map[string]string) (string, string, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	changed := []string{}
	for _, key := range keys {
		target := values[key]
		current, err := readINIValue(path, section, key)
		current = trimQuotes(strings.TrimSpace(current))
		if err == nil && current == target {
			continue
		}
		if err := upsertINIValue(path, section, key, target); err != nil {
			return "failed", "", err
		}
		changed = append(changed, fmt.Sprintf("%s=%s", key, target))
	}

	if len(changed) == 0 {
		return "unchanged", filepath.Base(expandHome(path)), nil
	}
	return "changed", strings.Join(changed, ", "), nil
}

func resolveGTKTargetTheme(ctx context.Context, mode, exact, darkTheme, lightTheme string, available []string) (string, string, []string) {
	warnings := []string{}
	if exact != "" {
		resolved, found := resolveProvidedThemeNameWithMatch(exact, available)
		if !found && len(available) > 0 {
			warnings = append(warnings, fmt.Sprintf("requested gtk_theme %q was not found in installed GTK themes; applying it anyway", exact))
		}
		return resolved, "gtk_theme", warnings
	}

	modeSpecific := lightTheme
	source := "gtk_theme_light"
	if mode == "dark" {
		modeSpecific = darkTheme
		source = "gtk_theme_dark"
	}
	if modeSpecific != "" {
		resolved, found := resolveProvidedThemeNameWithMatch(modeSpecific, available)
		if !found && len(available) > 0 {
			warnings = append(warnings, fmt.Sprintf("requested %s %q was not found in installed GTK themes; applying it anyway", source, modeSpecific))
		}
		return resolved, source, warnings
	}

	current := firstNonEmpty(
		readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.gnome.desktop.interface", "gtk-theme"),
		readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.desktop.interface", "gtk-theme"),
		readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", "xsettings", "-p", "/Net/ThemeName"),
		trimQuotes(readINIOrEmpty("~/.config/gtk-3.0/settings.ini", "Settings", "gtk-theme-name")),
		trimQuotes(readINIOrEmpty("~/.config/gtk-4.0/settings.ini", "Settings", "gtk-theme-name")),
	)
	target := chooseVariantName(current, mode, available, gtkThemeDefaults(mode))
	if target != "" {
		return target, "auto_variant", warnings
	}
	return "", "auto_variant", warnings
}

func gtkThemeNames() []string {
	return listDirEntries([]string{
		"~/.themes",
		"~/.local/share/themes",
		"/usr/local/share/themes",
		"/usr/share/themes",
	}, "", true)
}

func plasmaColorSchemes() []string {
	return listDirEntries([]string{
		"~/.local/share/color-schemes",
		"/usr/local/share/color-schemes",
		"/usr/share/color-schemes",
	}, ".colors", false)
}

func plasmaDesktopThemes() []string {
	return listDirEntries([]string{
		"~/.local/share/plasma/desktoptheme",
		"/usr/local/share/plasma/desktoptheme",
		"/usr/share/plasma/desktoptheme",
	}, "", true)
}

func qtctPaletteNames(toolName string) []string {
	return listDirEntries(qtctPalettePaths(toolName), ".conf", false)
}

func qtctPalettePaths(toolName string) []string {
	return []string{
		fmt.Sprintf("~/.config/%s/colors", toolName),
		fmt.Sprintf("~/.local/share/%s/colors", toolName),
		fmt.Sprintf("/usr/share/%s/colors", toolName),
	}
}

func chooseVariantName(current, mode string, available, defaults []string) string {
	current = trimQuotes(strings.TrimSpace(current))
	if current != "" {
		if matched := matchAvailableName(current, available); matched != "" {
			hint := themeModeHint(matched)
			if hint == mode || (mode == "light" && hint == "") {
				return matched
			}
		}

		base := normalizeThemeKey(current)
		explicit := []string{}
		neutral := []string{}
		for _, candidate := range available {
			if normalizeThemeKey(candidate) != base {
				continue
			}
			hint := themeModeHint(candidate)
			if hint == mode {
				explicit = append(explicit, candidate)
			} else if hint == "" {
				neutral = append(neutral, candidate)
			}
		}
		if len(explicit) > 0 {
			return explicit[0]
		}
		if mode == "light" && len(neutral) > 0 {
			return neutral[0]
		}
	}

	for _, candidate := range defaults {
		if matched := matchAvailableName(candidate, available); matched != "" {
			return matched
		}
	}
	if fallback := fallbackThemeForMode(mode, available); fallback != "" {
		return fallback
	}
	return ""
}

func resolveProvidedThemeName(name string, available []string) string {
	resolved, _ := resolveProvidedThemeNameWithMatch(name, available)
	return resolved
}

func resolveProvidedThemeNameWithMatch(name string, available []string) (string, bool) {
	name = trimQuotes(strings.TrimSpace(name))
	if name == "" {
		return "", false
	}
	if matched := matchAvailableName(name, available); matched != "" {
		return matched, true
	}
	return name, false
}

func fallbackThemeForMode(mode string, available []string) string {
	explicit := []string{}
	neutral := []string{}
	for _, candidate := range available {
		hint := themeModeHint(candidate)
		switch hint {
		case mode:
			explicit = append(explicit, candidate)
		case "":
			neutral = append(neutral, candidate)
		}
	}
	if len(explicit) > 0 {
		return explicit[0]
	}
	if mode == "light" && len(neutral) > 0 {
		return neutral[0]
	}
	return ""
}

func groupThemesByMode(themes []string) map[string]any {
	dark := []string{}
	light := []string{}
	neutral := []string{}
	for _, theme := range themes {
		switch themeModeHint(theme) {
		case "dark":
			dark = append(dark, theme)
		case "light":
			light = append(light, theme)
		default:
			neutral = append(neutral, theme)
		}
	}
	return map[string]any{
		"dark":    dark,
		"light":   light,
		"neutral": neutral,
	}
}

func matchAvailableName(target string, available []string) string {
	for _, candidate := range available {
		if strings.EqualFold(strings.TrimSpace(target), strings.TrimSpace(candidate)) {
			return candidate
		}
	}
	return ""
}

func resolveNamedFile(paths []string, name, suffix string) string {
	for _, searchPath := range paths {
		entries, err := os.ReadDir(expandHome(searchPath))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			candidate := entry.Name()
			if suffix != "" && !strings.HasSuffix(strings.ToLower(candidate), strings.ToLower(suffix)) {
				continue
			}
			base := candidate
			if suffix != "" {
				base = strings.TrimSuffix(candidate, suffix)
			}
			if strings.EqualFold(base, name) {
				return filepath.Join(expandHome(searchPath), candidate)
			}
		}
	}
	return ""
}

func gtkThemeDefaults(mode string) []string {
	if mode == "dark" {
		return []string{"Adwaita-dark", "Yaru-dark", "Mint-Y-Dark", "Breeze-Dark"}
	}
	return []string{"Adwaita", "Yaru", "Mint-Y", "Breeze"}
}

func plasmaColorSchemeDefaults(mode string) []string {
	if mode == "dark" {
		return []string{"BreezeDark", "Breeze Dark", "Breeze-Dark"}
	}
	return []string{"BreezeLight", "Breeze Light", "Breeze"}
}

func plasmaDesktopThemeDefaults(mode string) []string {
	if mode == "dark" {
		return []string{"breeze-dark", "breezedark"}
	}
	return []string{"default", "breeze"}
}

func qtctPaletteDefaults(mode string) []string {
	if mode == "dark" {
		return []string{"BreezeDark", "Adwaita-Dark", "AdwaitaDark"}
	}
	return []string{"Breeze", "BreezeLight", "Adwaita"}
}

func notifyKDEPaletteChange(ctx context.Context, warnings *[]string) {
	if !commandExists("dbus-send") {
		return
	}
	res := runCommand(ctx, 3*time.Second, "dbus-send", "--session", "--type=signal", "/KGlobalSettings", "org.kde.KGlobalSettings.notifyChange", "int32:0", "int32:0")
	if res.Err != nil && strings.TrimSpace(res.Stderr) != "" {
		addWarning(warnings, res.Stderr)
	}
}

func readINIOrEmpty(path, section, key string) string {
	value, err := readINIValue(path, section, key)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func findCommand(name string) string {
	path, err := execLookPath(name)
	if err != nil {
		return ""
	}
	return path
}

func execLookPath(name string) (string, error) {
	if !commandExists(name) {
		return "", fmt.Errorf("command not found: %s", name)
	}
	return name, nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isMissingSettingsBackendError(stderr string) bool {
	lower := strings.ToLower(stderr)
	return strings.Contains(lower, "no such schema") || strings.Contains(lower, "no such key") || strings.Contains(lower, "schema") && strings.Contains(lower, "not installed")
}

func addColorAction(actions *[]map[string]any, backend, status, details string) {
	action := map[string]any{
		"backend": backend,
		"status":  status,
	}
	details = strings.TrimSpace(details)
	if details != "" {
		action["details"] = details
	}
	*actions = append(*actions, action)
}

func addWarning(warnings *[]string, message string) {
	message = strings.TrimSpace(message)
	if message == "" {
		return
	}
	*warnings = append(*warnings, message)
}

func hasSuccessfulColorAction(actions []map[string]any) bool {
	for _, action := range actions {
		status, _ := action["status"].(string)
		if status == "changed" || status == "unchanged" {
			return true
		}
	}
	return false
}
