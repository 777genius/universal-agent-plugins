package vscodelocalhooks

import (
	"fmt"
	"strings"
)

func validateTarget(target Target) error {
	switch target.Shell {
	case LinuxSH, MacOSSH:
		if target.ComSpec == "" && target.SystemRoot == "" {
			return nil
		}
	case WindowsPowerShell51:
		if windowsAbsolute(target.SystemRoot) &&
			strings.EqualFold(target.ComSpec, target.SystemRoot+`\System32\cmd.exe`) {
			return nil
		}
	}
	return fmt.Errorf("%w: executor snapshot", ErrUnsupported)
}

// windowsAbsolute deliberately accepts only ordinary drive-absolute paths.
// This is a prepared representation, not a filesystem/path-policy framework.
func windowsAbsolute(value string) bool {
	if validateLiteral(value) != nil || len(value) < 4 || value[1:3] != `:\` {
		return false
	}
	if (value[0] < 'A' || value[0] > 'Z') && (value[0] < 'a' || value[0] > 'z') {
		return false
	}
	if strings.ContainsAny(value[3:], `/:*?"<>|`) {
		return false
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return false
		}
	}
	for _, segment := range strings.Split(value[3:], `\`) {
		if segment == "" || strings.HasSuffix(segment, " ") || strings.HasSuffix(segment, ".") {
			return false
		}
		name := strings.ToUpper(strings.TrimRight(strings.SplitN(segment, ".", 2)[0], " "))
		if reservedWindowsName(name) {
			return false
		}
	}
	return true
}

func reservedWindowsName(name string) bool {
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" {
		return true
	}
	if strings.HasPrefix(name, "COM") || strings.HasPrefix(name, "LPT") {
		switch name[3:] {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³":
			return true
		}
	}
	return false
}

func validateWindowsArgument(value string) error {
	// Legacy Windows PowerShell native argument binding drops empty strings and
	// reinterprets embedded double quotes/trailing backslashes. Smart single quotes
	// are PowerShell delimiters even inside ASCII-quoted strings. Refuse these
	// forms until actual Windows CI proves a wider byte-preserving representation.
	if value == "" || strings.ContainsAny(value, "\"\u2018\u2019\u201a\u201b") || strings.HasSuffix(value, `\`) {
		return fmt.Errorf("%w: Windows native argument binding", ErrUnsupported)
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return fmt.Errorf("%w: Windows control character", ErrUnsupported)
		}
	}
	return nil
}
