package toolset

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/wowtuff/ricing/tools"
)

var (
	ErrAuthFailed = errors.New("authentication failed")
)

func isRoot() bool {
	return os.Geteuid() == 0
}

type InstallPackageTool struct{}

func (t InstallPackageTool) Specs() tools.ToolSpec {
	return tools.ToolSpec{
		Name:        "install_package",
		Description: "Installs a required package using the appropiate package manager for a Linux Distro",
		ParamSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"package": map[string]any{
					"type":        "string",
					"description": "Name of the package to install",
				},
			},
			"required": []string{"package"},
		},
	}
}

func (t InstallPackageTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	pkgName, ok := args["package"].(string)
	if !ok || strings.TrimSpace(pkgName) == "" {
		return nil, errors.New(`missing or invalid "package"`)
	}

	distro, err := detectDistro(ctx)
	if err != nil {
		return map[string]any{
			"ok":      false,
			"message": "failed to detect distro",
			"error":   err.Error(),
		}, err
	}

	managers := managersForDistro(distro)
	if len(managers) == 0 {
		return map[string]any{
			"ok":      false,
			"message": "no supported package managers found for distro",
			"distro":  distro,
		}, fmt.Errorf("unsupported distro: %s", distro)
	}

	for _, mgr := range managers {
		if !commandExists(mgr) {
			continue
		}

		canInstall, reason := managerCanInstall(ctx, mgr, pkgName)
		if !canInstall {
			_ = reason
			continue
		}

		if err := installWithManager(ctx, mgr, pkgName); err != nil {
			if errors.Is(err, ErrAuthFailed) {
				return map[string]any{
					"ok":      false,
					"message": "sudo authentication is required",
					"package": pkgName,
					"error":   err.Error(),
				}, nil
			}

			continue
		}

		return map[string]any{
			"ok":       true,
			"message":  "package installed successfully",
			"package":  pkgName,
			"continue": true,
		}, nil
	}

	return map[string]any{
		"ok":      false,
		"message": "could not install package with any available manager",
		"package": pkgName,
	}, fmt.Errorf("failed to install package %q", pkgName)
}

func detectDistro(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "hostnamectl").Output()
	if err != nil {
		return "", err
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Operating System:") {
			osName := strings.TrimSpace(strings.TrimPrefix(line, "Operating System:"))
			return normalizeDistro(osName), nil
		}
	}

	return "", errors.New("could not find Operating System in hostnamectl output")
}

func normalizeDistro(osName string) string {
	osName = strings.TrimSpace(osName)

	known := []string{
		"Manjaro",
		"Arch",
		"Ubuntu",
		"Debian",
		"Linux Mint",
		"Pop!_OS",
		"Fedora",
		"CentOS",
		"RHEL",
		"Red Hat Enterprise Linux",
		"Rocky Linux",
		"AlmaLinux",
		"openSUSE",
		"SLES",
		"Alpine",
	}

	for _, k := range known {
		if strings.Contains(strings.ToLower(osName), strings.ToLower(k)) {
			return k
		}
	}

	return osName
}

func managersForDistro(distro string) []string {
	distroManagers := map[string][]string{
		"Manjaro":                  {"pacman", "yay", "paru"},
		"Arch":                     {"pacman", "yay", "paru"},
		"Ubuntu":                   {"apt"},
		"Debian":                   {"apt"},
		"Linux Mint":               {"apt"},
		"Pop!_OS":                  {"apt"},
		"Fedora":                   {"dnf"},
		"CentOS":                   {"dnf", "yum"},
		"RHEL":                     {"dnf", "yum"},
		"Red Hat Enterprise Linux": {"dnf", "yum"},
		"Rocky Linux":              {"dnf", "yum"},
		"AlmaLinux":                {"dnf", "yum"},
		"openSUSE":                 {"zypper"},
		"SLES":                     {"zypper"},
		"Alpine":                   {"apk"},
	}

	if managers, ok := distroManagers[distro]; ok {
		return managers
	}

	return nil
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func managerCanInstall(ctx context.Context, mgr, pkg string) (bool, string) {
	var cmd *exec.Cmd

	switch mgr {
	case "apt":
		cmd = exec.CommandContext(ctx, "apt-cache", "show", pkg)
	case "dnf":
		cmd = exec.CommandContext(ctx, "dnf", "info", pkg)
	case "yum":
		cmd = exec.CommandContext(ctx, "yum", "info", pkg)
	case "zypper":
		cmd = exec.CommandContext(ctx, "zypper", "--non-interactive", "info", pkg)
	case "pacman":
		cmd = exec.CommandContext(ctx, "pacman", "-Si", pkg)
	case "yay":
		cmd = exec.CommandContext(ctx, "yay", "-Si", pkg)
	case "paru":
		cmd = exec.CommandContext(ctx, "paru", "-Si", pkg)
	case "apk":
		cmd = exec.CommandContext(ctx, "apk", "search", "-e", pkg)
	default:
		return false, "unsupported manager"
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(out)
	}
	return true, string(out)
}

func installWithManager(ctx context.Context, mgr, pkg string) error {
	var cmd *exec.Cmd

	switch mgr {
	case "apt":
		return runWithPrivilegePrompt(ctx, "apt install -y "+pkg, "apt", "install", "-y", pkg)
	case "dnf":
		return runWithPrivilegePrompt(ctx, "dnf install -y "+pkg, "dnf", "install", "-y", pkg)
	case "yum":
		return runWithPrivilegePrompt(ctx, "yum install -y "+pkg, "yum", "install", "-y", pkg)
	case "zypper":
		return runWithPrivilegePrompt(ctx, "zypper --non-interactive install "+pkg, "zypper", "--non-interactive", "install", pkg)
	case "pacman":
		return runWithPrivilegePrompt(ctx, "pacman -S --noconfirm "+pkg, "pacman", "-S", "--noconfirm", pkg)
	case "apk":
		return runWithPrivilegePrompt(ctx, "apk add "+pkg, "apk", "add", pkg)
	case "yay":
		cmd = exec.CommandContext(ctx, "yay", "-S", "--noconfirm", pkg)
	case "paru":
		cmd = exec.CommandContext(ctx, "paru", "-S", "--noconfirm", pkg)
	default:
		return fmt.Errorf("unsupported manager: %s", mgr)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed with %s: %v: %s", mgr, err, string(out))
	}
	return nil
}

func runWithPrivilegePrompt(ctx context.Context, operation, name string, args ...string) error {
	if commandExists("sudo") == false {
		return errors.New("sudo is required but was not found")
	}

	if isRoot() {
		cmd := exec.CommandContext(ctx, name, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("command failed: %v: %s", err, string(out))
		}
		return nil
	}
	cmd := exec.CommandContext(ctx, "sudo", append([]string{"-n", name}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.ToLower(string(out))
		if strings.Contains(s, "a password is required") ||
			strings.Contains(s, "a terminal is required") ||
			strings.Contains(s, "no tty present") ||
			strings.Contains(s, "sorry, try again") ||
			strings.Contains(s, "incorrect password") {
			return fmt.Errorf("%w: run `sudo -v` in the terminal running ricingd to authorize %s, then try again", ErrAuthFailed, operation)
		}
		return fmt.Errorf("install failed with sudo: %v: %s", err, string(out))
	}

	return nil
}
