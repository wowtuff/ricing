package toolset

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type previewProfile struct {
	ID             string
	Label          string
	Distro         string
	Desktop        string
	SessionType    string
	DockerfilePath string
	DefaultGroup   string
}

func previewProfiles() map[string]previewProfile {
	root := repoRootPath()
	return map[string]previewProfile{
		"arch-hyprland": {
			ID:             "arch-hyprland",
			Label:          "Arch Hyprland",
			Distro:         "arch",
			Desktop:        "hyprland",
			SessionType:    "wayland",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-hyprland", "Dockerfile"),
			DefaultGroup:   "desktop",
		},
		"arch-i3": {
			ID:             "arch-i3",
			Label:          "Arch i3",
			Distro:         "arch",
			Desktop:        "i3",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-i3", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"debian-i3": {
			ID:             "debian-i3",
			Label:          "Debian i3",
			Distro:         "debian",
			Desktop:        "i3",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "debian-i3", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-gnome": {
			ID:             "arch-gnome",
			Label:          "Arch GNOME",
			Distro:         "arch",
			Desktop:        "gnome",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-gnome", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-plasma": {
			ID:             "arch-plasma",
			Label:          "Arch Plasma",
			Distro:         "arch",
			Desktop:        "plasma",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-plasma", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-xfce": {
			ID:             "arch-xfce",
			Label:          "Arch XFCE",
			Distro:         "arch",
			Desktop:        "xfce",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-xfce", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-cinnamon": {
			ID:             "arch-cinnamon",
			Label:          "Arch Cinnamon",
			Distro:         "arch",
			Desktop:        "cinnamon",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-cinnamon", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-mate": {
			ID:             "arch-mate",
			Label:          "Arch MATE",
			Distro:         "arch",
			Desktop:        "mate",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-mate", "Dockerfile"),
			DefaultGroup:   "main",
		},
		"arch-lxqt": {
			ID:             "arch-lxqt",
			Label:          "Arch LXQt",
			Distro:         "arch",
			Desktop:        "lxqt",
			SessionType:    "x11",
			DockerfilePath: filepath.Join(root, "hyprland", "profiles", "arch-lxqt", "Dockerfile"),
			DefaultGroup:   "main",
		},
	}
}

func sortedPreviewProfiles() []previewProfile {
	all := previewProfiles()
	ids := make([]string, 0, len(all))
	for id := range all {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]previewProfile, 0, len(ids))
	for _, id := range ids {
		out = append(out, all[id])
	}
	return out
}

func previewProfileByID(id string) (previewProfile, bool) {
	profile, ok := previewProfiles()[strings.TrimSpace(id)]
	return profile, ok
}

func repoRootPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", ".."))
}

func hyprlandRootPath() string {
	return filepath.Join(repoRootPath(), "hyprland")
}

func previewScriptPath() string {
	return filepath.Join(hyprlandRootPath(), "run.sh")
}

func previewURL() string {
	return "http://127.0.0.1:6090/?autoconnect=1&resize=remote"
}

func findBashPath() string {
	candidates := []string{}
	if bash, err := execLookPath("bash"); err == nil {
		if runtime.GOOS != "windows" || !strings.Contains(strings.ToLower(bash), "windowsapps\\bash.exe") {
			return bash
		}
	}
	if runtime.GOOS == "windows" {
		if programFiles := strings.TrimSpace(os.Getenv("ProgramFiles")); programFiles != "" {
			candidates = append(candidates, filepath.Join(programFiles, "Git", "bin", "bash.exe"))
		}
		if programFilesX86 := strings.TrimSpace(os.Getenv("ProgramFiles(x86)")); programFilesX86 != "" {
			candidates = append(candidates, filepath.Join(programFilesX86, "Git", "bin", "bash.exe"))
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return ""
}
