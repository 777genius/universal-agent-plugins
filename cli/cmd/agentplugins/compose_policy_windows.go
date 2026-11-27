package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func machinePolicyPath() (string, error) {
	root, err := registeredWindowsProgramData()
	if err != nil {
		return "", fmt.Errorf("locate machine installation policy: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("verify machine installation policy directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("machine ProgramData registration is not a directory")
	}
	return filepath.Join(root, "AgentPlugins", "policy.yaml"), nil
}

// The Shell known-folder lookup expands the caller's environment. The protected
// machine registration and shared Windows directory retain the same authority
// when a public CLI process has an isolated or overridden environment.
func registeredWindowsProgramData() (string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE,
		"SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\ProfileList",
		registry.QUERY_VALUE|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("open machine ProgramData registration: %w", err)
	}
	value, kind, readErr := key.GetStringValue("ProgramData")
	if err := errors.Join(readErr, key.Close()); err != nil {
		return "", fmt.Errorf("read machine ProgramData registration: %w", err)
	}
	systemWindowsDir := ""
	if kind == registry.EXPAND_SZ && strings.Contains(value, "%") {
		systemWindowsDir, err = windows.GetSystemWindowsDirectory()
		if err != nil {
			return "", fmt.Errorf("locate shared Windows directory: %w", err)
		}
	}
	return resolveWindowsProgramData(value, kind, systemWindowsDir)
}

func resolveWindowsProgramData(value string, kind uint32, systemWindowsDir string) (string, error) {
	if kind == registry.EXPAND_SZ {
		var err error
		value, err = expandWindowsProgramData(value, systemWindowsDir)
		if err != nil {
			return "", err
		}
	} else if kind != registry.SZ {
		return "", fmt.Errorf("unsupported machine ProgramData registration type %d", kind)
	}
	if value == "" || strings.IndexByte(value, 0) >= 0 || !filepath.IsAbs(value) {
		return "", fmt.Errorf("machine ProgramData registration must be an absolute directory")
	}
	return filepath.Clean(value), nil
}

func expandWindowsProgramData(value, systemWindowsDir string) (string, error) {
	parts := strings.Split(value, "%")
	if len(parts)%2 == 0 {
		return "", fmt.Errorf("machine ProgramData registration contains an unclosed variable")
	}
	if len(parts) == 1 {
		return value, nil
	}
	volume := filepath.VolumeName(systemWindowsDir)
	// Windows documents a bare drive when its own directory is the drive root.
	// This normalization applies only to the trusted OS result, not registration.
	if len(systemWindowsDir) == 2 && systemWindowsDir == volume {
		systemWindowsDir += "\\"
	}
	if volume == "" || strings.IndexByte(systemWindowsDir, 0) >= 0 || !filepath.IsAbs(systemWindowsDir) {
		return "", fmt.Errorf("shared Windows directory must be an absolute directory")
	}
	for i := 1; i < len(parts); i += 2 {
		switch strings.ToLower(parts[i]) {
		case "systemdrive":
			parts[i] = volume
		case "systemroot", "windir":
			parts[i] = systemWindowsDir
		default:
			return "", fmt.Errorf("unsupported machine ProgramData variable %q", parts[i])
		}
	}
	return strings.Join(parts, ""), nil
}
