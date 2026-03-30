package toolset

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/wowtuff/ricing/tools"
)

type EditPreviewDockerfileTool struct{}

type dockerfileInstallBlock struct {
	Name     string
	Start    int
	End      int
	Indent   string
	Packages []string
}

func (t *EditPreviewDockerfileTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "edit_preview_dockerfile",
		Description: "Add or remove package install entries inside a preview Dockerfile. Use this to manage package downloads for the desktop preview images without hand-editing the package list.",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"profile": map[string]any{
					"type":        "string",
					"description": "Preview profile id such as arch-hyprland, arch-i3, debian-i3, arch-gnome, arch-plasma, arch-xfce, arch-cinnamon, arch-mate, or arch-lxqt.",
				},
				"install_group": map[string]any{
					"type":        "string",
					"description": "Optional install section name. Use base or desktop for arch-hyprland. Other profiles use main.",
				},
				"add_packages": map[string]any{
					"type":        "array",
					"description": "Package names to add to the selected install list.",
					"items": map[string]any{
						"type": "string",
					},
				},
				"remove_packages": map[string]any{
					"type":        "array",
					"description": "Package names to remove from the selected install list.",
					"items": map[string]any{
						"type": "string",
					},
				},
			},
			"required": []string{"profile"},
		},
	}
}

func (t *EditPreviewDockerfileTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	_ = ctx
	profileID := strings.TrimSpace(asMapString(args["profile"]))
	profile, ok := previewProfileByID(profileID)
	if !ok {
		return nil, fmt.Errorf("unknown preview profile: %s", profileID)
	}
	addPackages := stringListArg(args["add_packages"])
	removePackages := stringListArg(args["remove_packages"])
	if len(addPackages) == 0 && len(removePackages) == 0 {
		return nil, fmt.Errorf("add_packages or remove_packages is required")
	}
	raw, err := os.ReadFile(profile.DockerfilePath)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	blocks := parseDockerfileInstallBlocks(lines, profile)
	if len(blocks) == 0 {
		return nil, fmt.Errorf("no editable install block found in %s", profile.DockerfilePath)
	}
	group := strings.TrimSpace(asMapString(args["install_group"]))
	if group == "" {
		group = profile.DefaultGroup
	}
	index := -1
	for i := range blocks {
		if blocks[i].Name == group {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, fmt.Errorf("install group %s not found in %s", group, profile.DockerfilePath)
	}
	block := blocks[index]
	packageIndex := map[string]int{}
	current := make([]string, 0, len(block.Packages))
	for _, pkg := range block.Packages {
		key := strings.ToLower(strings.TrimSpace(pkg))
		if key == "" {
			continue
		}
		if _, exists := packageIndex[key]; exists {
			continue
		}
		packageIndex[key] = len(current)
		current = append(current, pkg)
	}
	added := []string{}
	for _, pkg := range addPackages {
		key := strings.ToLower(strings.TrimSpace(pkg))
		if key == "" {
			continue
		}
		if _, exists := packageIndex[key]; exists {
			continue
		}
		packageIndex[key] = len(current)
		current = append(current, pkg)
		added = append(added, pkg)
	}
	removed := []string{}
	if len(removePackages) > 0 {
		removeSet := map[string]bool{}
		for _, pkg := range removePackages {
			key := strings.ToLower(strings.TrimSpace(pkg))
			if key != "" {
				removeSet[key] = true
			}
		}
		next := make([]string, 0, len(current))
		for _, pkg := range current {
			if removeSet[strings.ToLower(strings.TrimSpace(pkg))] {
				removed = append(removed, pkg)
				continue
			}
			next = append(next, pkg)
		}
		current = next
	}
	if len(current) == 0 {
		return nil, fmt.Errorf("refusing to leave %s with an empty install block", profile.DockerfilePath)
	}
	replacement := make([]string, 0, len(current))
	for i, pkg := range current {
		line := block.Indent + pkg
		if i < len(current)-1 {
			line += " \\"
		}
		replacement = append(replacement, line)
	}
	updated := append([]string{}, lines[:block.Start]...)
	updated = append(updated, replacement...)
	updated = append(updated, lines[block.End+1:]...)
	content := strings.Join(updated, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(profile.DockerfilePath, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok":               true,
		"profile":          profile.ID,
		"dockerfile_path":  profile.DockerfilePath,
		"install_group":    group,
		"added_packages":   added,
		"removed_packages": removed,
		"packages":         current,
	}, nil
}

func parseDockerfileInstallBlocks(lines []string, profile previewProfile) []dockerfileInstallBlock {
	blocks := []dockerfileInstallBlock{}
	pacmanCount := 0
	aptCount := 0
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "pacman -S --noconfirm") {
			start := i + 1
			end := start - 1
			indent := "    "
			packages := []string{}
			for j := start; j < len(lines); j++ {
				current := lines[j]
				if strings.TrimSpace(current) == "" {
					break
				}
				if strings.TrimLeft(current, " \t") == current {
					break
				}
				clean := strings.TrimSpace(strings.TrimSuffix(current, "\\"))
				if clean == "" {
					continue
				}
				packages = append(packages, clean)
				end = j
			}
			if len(packages) == 0 {
				continue
			}
			name := "main"
			pacmanCount++
			if profile.ID == "arch-hyprland" {
				if pacmanCount == 1 {
					name = "base"
				} else {
					name = "desktop"
				}
			}
			blocks = append(blocks, dockerfileInstallBlock{
				Name:     name,
				Start:    start,
				End:      end,
				Indent:   indent,
				Packages: packages,
			})
			i = end
			continue
		}
		if strings.Contains(trimmed, "apt-get install -y --no-install-recommends") {
			start := i + 1
			end := start - 1
			indent := "    "
			packages := []string{}
			for j := start; j < len(lines); j++ {
				current := lines[j]
				trimmedCurrent := strings.TrimSpace(current)
				if trimmedCurrent == "" || strings.HasPrefix(trimmedCurrent, "&& ") {
					break
				}
				clean := strings.TrimSpace(strings.TrimSuffix(current, "\\"))
				if clean == "" {
					continue
				}
				packages = append(packages, clean)
				end = j
			}
			if len(packages) == 0 {
				continue
			}
			name := "main"
			aptCount++
			if aptCount > 1 {
				name = fmt.Sprintf("main_%d", aptCount)
			}
			blocks = append(blocks, dockerfileInstallBlock{
				Name:     name,
				Start:    start,
				End:      end,
				Indent:   indent,
				Packages: packages,
			})
			i = end
		}
	}
	return blocks
}

func stringListArg(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(list))
	for _, raw := range list {
		item := strings.TrimSpace(asMapString(raw))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}
