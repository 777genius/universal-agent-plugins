package shared

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

var copilotInstalledEntry = regexp.MustCompile(`^[ \t]+•[ \t]+([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*)[ \t]+\(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\)[ \t]*$`)
var copilotLiveEntry = regexp.MustCompile(`^  • ([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*) \(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\) \(([A-Za-z0-9_-]+)\)$`)

const CopilotLiveHeader = "Live Plugins (loaded from a local marketplace directory, never copied):"
const copilotLiveHeader = CopilotLiveHeader

type CopilotStatus int

const (
	CopilotStatusUnknown CopilotStatus = iota
	CopilotStatusInstalled
	CopilotStatusAbsent
)

var ErrCopilotListContractUnknown = errors.New("Copilot plugin list output is not recognized")

func CopilotLivePluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) (CopilotStatus, bool) {
	document := strings.TrimSuffix(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n")
	lines := strings.Split(document, "\n")
	if len(lines) == 0 || lines[0] != copilotLiveHeader {
		return CopilotStatusUnknown, false
	}
	if len(lines) < 3 || (len(lines)-1)%2 != 0 || strings.TrimSpace(expectedVersion) == "" || strings.TrimSpace(expectedPath) == "" {
		return CopilotStatusUnknown, true
	}
	seen := make(map[string]bool, (len(lines)-1)/2)
	matches := 0
	for index := 1; index < len(lines); index += 2 {
		entry := copilotLiveEntry.FindStringSubmatch(lines[index])
		if len(entry) != 4 || seen[entry[1]] || (entry[3] != "enabled" && entry[3] != "disabled") {
			return CopilotStatusUnknown, true
		}
		seen[entry[1]] = true
		const pathPrefix = "      from "
		if !strings.HasPrefix(lines[index+1], pathPrefix) {
			return CopilotStatusUnknown, true
		}
		listedPath := strings.TrimPrefix(lines[index+1], pathPrefix)
		if listedPath == "" || !filepath.IsAbs(listedPath) || listedPath != filepath.Clean(listedPath) {
			return CopilotStatusUnknown, true
		}
		if entry[1] != expected {
			continue
		}
		if entry[2] != expectedVersion || entry[3] != "enabled" || expectedPath != filepath.Clean(expectedPath) || listedPath != expectedPath {
			return CopilotStatusUnknown, true
		}
		matches++
	}
	if matches == 1 {
		return CopilotStatusInstalled, true
	}
	if matches > 1 {
		return CopilotStatusUnknown, true
	}
	return CopilotStatusAbsent, true
}

func CopilotPluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) CopilotStatus {
	if status, recognized := CopilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		return status
	}
	inInstalledSection := false
	recognizedSection := false
	recognizedEntry := false
	matches := 0
	for _, rawLine := range strings.Split(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(rawLine) == "Installed plugins:" {
			if inInstalledSection {
				return CopilotStatusUnknown
			}
			inInstalledSection = true
			recognizedSection = true
			continue
		}
		if !inInstalledSection {
			continue
		}
		if rawLine != "" && rawLine[0] != ' ' && rawLine[0] != '\t' {
			inInstalledSection = false
			continue
		}
		entry := copilotInstalledEntry.FindStringSubmatch(rawLine)
		if len(entry) == 3 {
			recognizedEntry = true
			if entry[1] == expected {
				if entry[2] != expectedVersion {
					return CopilotStatusUnknown
				}
				matches++
			}
			continue
		}
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" {
			continue
		}
		if trimmed == "No plugins installed." || trimmed == "No plugins installed" {
			recognizedEntry = true
			continue
		}
		lower := strings.ToLower(trimmed)
		if strings.Contains(trimmed, expected) {
			for _, state := range []string{"pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure"} {
				if strings.Contains(lower, state) {
					return CopilotStatusAbsent
				}
			}
			return CopilotStatusUnknown
		}
	}
	if matches == 1 {
		return CopilotStatusInstalled
	}
	if matches > 1 {
		return CopilotStatusUnknown
	}
	if recognizedSection && recognizedEntry {
		return CopilotStatusAbsent
	}
	return CopilotStatusUnknown
}
