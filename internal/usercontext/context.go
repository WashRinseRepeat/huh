package usercontext

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	"huh/internal/config"
)

type SystemContext struct {
	OS         string
	Distro     string
	Shell      string
	PackageMgr string
	Hardware   string            // CPU/RAM (if enabled)
	Clipboard  string            // Detected clipboard tool
	Custom     map[string]string // User-defined context
}

func GetContext() SystemContext {
	ctx := SystemContext{
		OS:     runtime.GOOS,
		Custom: make(map[string]string),
	}

	// 1. Detect Distro
	if ctx.OS == "linux" {
		ctx.Distro = getDistroName()
		ctx.PackageMgr = detectPackageManager()
		ctx.Clipboard = detectClipboard()
	}

	// 2. Detect Shell
	ctx.Shell = os.Getenv("SHELL")
	if ctx.Shell == "" {
		if runtime.GOOS == "windows" {
			ctx.Shell = "powershell"
		} else {
			ctx.Shell = "bash"
		}
	}
	// Clean shell path to name (e.g. /bin/fish -> fish)
	parts := strings.Split(ctx.Shell, "/")
	ctx.Shell = parts[len(parts)-1]

	// 3. Hardware (if configured)
	if config.AppConfig.Context["level"] == "hardware" {
		ctx.Hardware = getHardwareInfo()
	}

	// 4. Merge Config Context (Overrides and Custom)
	for k, v := range config.AppConfig.Context {
		switch k {
		case "os":
			ctx.OS = v
		case "distro":
			ctx.Distro = v
		case "shell":
			ctx.Shell = v
		case "package_mgr":
			ctx.PackageMgr = v
		case "hardware":
			// handled above via level check, but if user sets it directly as string, could override?
			// Let's assume 'hardware' key in config is mostly for level control, but if they want to spoof it:
			if v != "basic" && v != "hardware" { // strict check? nah, let's just let it be custom if it's not a control flag
				ctx.Custom[k] = v // treat as custom for now to avoid conflict with "level" logic
			}
		case "clipboard":
			ctx.Clipboard = v
		case "level":
			// ignore, used for logic
		default:
			ctx.Custom[k] = v
		}
	}

	return ctx
}

func getDistroName() string {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "linux (unknown)"
	}
	content := string(data)
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "linux"
}

func detectPackageManager() string {
	if _, err := exec.LookPath("pacman"); err == nil {
		return "pacman"
	}
	if _, err := exec.LookPath("apt"); err == nil {
		return "apt"
	}
	if _, err := exec.LookPath("dnf"); err == nil {
		return "dnf"
	}
	return "unknown"
}

func detectClipboard() string {
	// Wayland check
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			return "wl-clipboard"
		}
	}
	// X11 / standard
	if _, err := exec.LookPath("xclip"); err == nil {
		return "xclip"
	}
	return "unknown"
}

func getHardwareInfo() string {
	// Simplified implementation
	// In a real app, we might parse /proc/cpuinfo or use a library
	return "CPU: detected, RAM: detected" // Placeholder
}
