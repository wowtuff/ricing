package toolset

import (
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wowtuff/ricing/tools"
)

type SystemInfoTool struct{}

func (t *SystemInfoTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "get_system_info",
		Description: "Get Linux system, session, and appearance information.",
		ParamSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}
}

func (t *SystemInfoTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = args

	osRelease := readOSRelease()
	osName := firstNonEmpty(osRelease["PRETTY_NAME"], osRelease["NAME"], osRelease["ID"])
	distro := normalizeDistro(osName)
	packageManagers := installedManagersForDistro(distro)
	if len(packageManagers) == 0 {
		packageManagers = discoverPackageManagers()
	}

	hostname, _ := os.Hostname()
	bootID := strings.TrimSpace(readFileOrEmpty("/proc/sys/kernel/random/boot_id"))
	uptimeSeconds := parseProcUptime()
	bootTimeUnix := parseBootTimeUnix(uptimeSeconds)
	loadAvg := parseLoadAverage()
	cpuModel, cpuCount := parseCPUInfo()
	memInfo := parseMemInfo()
	desktop, desktopRaw, sessionType, desktopIDs := currentDesktopInfo()
	appearance := collectAppearanceState(ctx)
	availableGTKThemes := gtkThemeNames()

	userName := firstNonEmpty(os.Getenv("USER"), os.Getenv("LOGNAME"))
	homeDir, _ := os.UserHomeDir()

	return map[string]any{
		"ok":       true,
		"hostname": hostname,
		"os": map[string]any{
			"name":           osRelease["NAME"],
			"pretty_name":    osRelease["PRETTY_NAME"],
			"id":             osRelease["ID"],
			"version_id":     osRelease["VERSION_ID"],
			"version":        osRelease["VERSION"],
			"id_like":        splitWordsOrEmpty(osRelease["ID_LIKE"]),
			"ansi_color":     osRelease["ANSI_COLOR"],
			"home_url":       osRelease["HOME_URL"],
			"support_url":    osRelease["SUPPORT_URL"],
			"bug_report_url": osRelease["BUG_REPORT_URL"],
		},
		"kernel": map[string]any{
			"sysname": readCommandValue(ctx, 2*time.Second, "uname", "-s"),
			"release": readCommandValue(ctx, 2*time.Second, "uname", "-r"),
			"version": readCommandValue(ctx, 2*time.Second, "uname", "-v"),
			"machine": readCommandValue(ctx, 2*time.Second, "uname", "-m"),
		},
		"boot": map[string]any{
			"boot_id":         bootID,
			"uptime_seconds":  uptimeSeconds,
			"boot_time_unix":  bootTimeUnix,
			"load_average":    loadAvg,
			"init_process":    strings.TrimSpace(readFileOrEmpty("/proc/1/comm")),
			"systemd_present": dirExists("/run/systemd/system"),
			"journalctl":      commandExists("journalctl"),
		},
		"hardware": map[string]any{
			"architecture": runtime.GOARCH,
			"machine":      readCommandValue(ctx, 2*time.Second, "uname", "-m"),
			"cpu_model":    cpuModel,
			"cpu_count":    cpuCount,
		},
		"memory": memInfo,
		"session": map[string]any{
			"desktop":             desktop,
			"desktop_raw":         desktopRaw,
			"desktop_identifiers": desktopIDs,
			"session_type":        sessionType,
			"user":                userName,
			"uid":                 os.Getuid(),
			"euid":                os.Geteuid(),
			"home":                homeDir,
			"shell":               os.Getenv("SHELL"),
			"lang":                firstNonEmpty(os.Getenv("LANG"), os.Getenv("LC_ALL")),
		},
		"package_managers":             packageManagers,
		"appearance":                   appearance,
		"available_gtk_themes":         availableGTKThemes,
		"available_gtk_themes_by_mode": groupThemesByMode(availableGTKThemes),
	}, nil
}

func readOSRelease() map[string]string {
	_, content, err := readFirstExisting("/etc/os-release", "/usr/lib/os-release")
	if err != nil {
		return map[string]string{}
	}
	return parseKeyValueFile(content)
}

func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func parseProcUptime() float64 {
	fields := strings.Fields(readFileOrEmpty("/proc/uptime"))
	if len(fields) == 0 {
		return 0
	}
	n, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return n
}

func parseBootTimeUnix(uptimeSeconds float64) int64 {
	for _, line := range splitLines(readFileOrEmpty("/proc/stat")) {
		if !strings.HasPrefix(line, "btime ") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			break
		}
		value, err := strconv.ParseInt(parts[1], 10, 64)
		if err == nil {
			return value
		}
	}
	if uptimeSeconds <= 0 {
		return 0
	}
	return time.Now().Add(-time.Duration(uptimeSeconds * float64(time.Second))).Unix()
}

func parseLoadAverage() []float64 {
	parts := strings.Fields(readFileOrEmpty("/proc/loadavg"))
	loads := []float64{}
	for i := 0; i < len(parts) && i < 3; i++ {
		value, err := strconv.ParseFloat(parts[i], 64)
		if err != nil {
			continue
		}
		loads = append(loads, value)
	}
	return loads
}

func parseCPUInfo() (string, int) {
	model := ""
	count := 0
	for _, line := range splitLines(readFileOrEmpty("/proc/cpuinfo")) {
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		value := strings.TrimSpace(line[idx+1:])
		switch key {
		case "processor":
			count++
		case "model name", "Hardware", "Processor":
			if model == "" && value != "" {
				model = value
			}
		}
	}
	if count == 0 {
		count = runtime.NumCPU()
	}
	return model, count
}

func parseMemInfo() map[string]any {
	values := map[string]int64{}
	for _, line := range splitLines(readFileOrEmpty("/proc/meminfo")) {
		idx := strings.Index(line, ":")
		if idx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		fields := strings.Fields(strings.TrimSpace(line[idx+1:]))
		if len(fields) == 0 {
			continue
		}
		value, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			continue
		}
		values[key] = value
	}

	return map[string]any{
		"mem_total_kib":     values["MemTotal"],
		"mem_available_kib": values["MemAvailable"],
		"mem_free_kib":      values["MemFree"],
		"swap_total_kib":    values["SwapTotal"],
		"swap_free_kib":     values["SwapFree"],
		"buffers_kib":       values["Buffers"],
		"cached_kib":        values["Cached"],
		"shared_kib":        values["Shmem"],
	}
}

func installedManagersForDistro(distro string) []string {
	managers := managersForDistro(distro)
	available := make([]string, 0, len(managers))
	for _, manager := range managers {
		if commandExists(manager) {
			available = append(available, manager)
		}
	}
	return uniqueSorted(available)
}

func discoverPackageManagers() []string {
	candidates := []string{"apt", "dnf", "yum", "zypper", "pacman", "paru", "yay", "apk"}
	found := []string{}
	for _, candidate := range candidates {
		if commandExists(candidate) {
			found = append(found, candidate)
		}
	}
	return found
}

func splitWordsOrEmpty(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{}
	}
	return strings.Fields(value)
}

func collectAppearanceState(ctx context.Context) map[string]any {
	desktop, desktopRaw, sessionType, identifiers := currentDesktopInfo()
	appearance := map[string]any{
		"desktop":             desktop,
		"desktop_raw":         desktopRaw,
		"desktop_identifiers": identifiers,
		"session_type":        sessionType,
	}

	if commandExists("gsettings") {
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.gnome.desktop.interface", "color-scheme"); value != "" {
			appearance["gsettings_color_scheme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.gnome.desktop.interface", "gtk-theme"); value != "" {
			appearance["gsettings_gtk_theme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.gnome.desktop.interface", "icon-theme"); value != "" {
			appearance["gsettings_icon_theme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.desktop.interface", "gtk-theme"); value != "" {
			appearance["cinnamon_gtk_theme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.desktop.wm.preferences", "theme"); value != "" {
			appearance["cinnamon_wm_theme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "gsettings", "get", "org.cinnamon.theme", "name"); value != "" {
			appearance["cinnamon_shell_theme"] = value
		}
	}

	if commandExists("xfconf-query") {
		if value := readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", "xsettings", "-p", "/Net/ThemeName"); value != "" {
			appearance["xfce_gtk_theme"] = value
		}
		if value := readCommandValue(ctx, 2*time.Second, "xfconf-query", "-c", "xfwm4", "-p", "/general/theme"); value != "" {
			appearance["xfce_wm_theme"] = value
		}
	}

	if value, err := readINIValue("~/.config/kdeglobals", "General", "ColorScheme"); err == nil {
		appearance["kde_color_scheme"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/plasmarc", "Theme", "name"); err == nil {
		appearance["plasma_desktop_theme"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/gtk-3.0/settings.ini", "Settings", "gtk-theme-name"); err == nil {
		appearance["gtk3_theme"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/gtk-3.0/settings.ini", "Settings", "gtk-application-prefer-dark-theme"); err == nil {
		appearance["gtk3_prefer_dark"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/gtk-4.0/settings.ini", "Settings", "gtk-theme-name"); err == nil {
		appearance["gtk4_theme"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/gtk-4.0/settings.ini", "Settings", "gtk-application-prefer-dark-theme"); err == nil {
		appearance["gtk4_prefer_dark"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/qt5ct/qt5ct.conf", "Appearance", "color_scheme_path"); err == nil {
		appearance["qt5ct_color_scheme_path"] = strings.TrimSpace(value)
	}
	if value, err := readINIValue("~/.config/qt6ct/qt6ct.conf", "Appearance", "color_scheme_path"); err == nil {
		appearance["qt6ct_color_scheme_path"] = strings.TrimSpace(value)
	}

	appearance["mode_guess"] = guessAppearanceMode(appearance)
	return appearance
}

func guessAppearanceMode(appearance map[string]any) string {
	if value := asMapString(appearance["gsettings_color_scheme"]); value != "" {
		switch value {
		case "prefer-dark":
			return "dark"
		case "prefer-light", "default":
			return "light"
		}
	}
	if value := asMapString(appearance["gtk3_prefer_dark"]); value != "" {
		if value == "true" || value == "1" {
			return "dark"
		}
		if value == "false" || value == "0" {
			return "light"
		}
	}
	if value := asMapString(appearance["gtk4_prefer_dark"]); value != "" {
		if value == "true" || value == "1" {
			return "dark"
		}
		if value == "false" || value == "0" {
			return "light"
		}
	}

	keys := []string{
		"kde_color_scheme",
		"plasma_desktop_theme",
		"gsettings_gtk_theme",
		"cinnamon_gtk_theme",
		"cinnamon_wm_theme",
		"cinnamon_shell_theme",
		"xfce_gtk_theme",
		"xfce_wm_theme",
		"gtk3_theme",
		"gtk4_theme",
		"qt5ct_color_scheme_path",
		"qt6ct_color_scheme_path",
	}
	for _, key := range keys {
		mode := themeModeHint(asMapString(appearance[key]))
		if mode != "" {
			return mode
		}
	}
	return "unknown"
}

func asMapString(value any) string {
	s, _ := value.(string)
	return strings.TrimSpace(s)
}
