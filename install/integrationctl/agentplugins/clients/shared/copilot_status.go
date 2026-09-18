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

var ErrCopilotListContractUnknown = errors.New("the Copilot plugin list output is not recognized")

func CopilotLivePluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) (CopilotStatus, bool) {
	document := strings.TrimSuffix(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n")
	lines := strings.Split(document, "\n")
	if len(lines) == 0 || lines[0] != copilotLiveHeader {
		return CopilotStatusUnknown, false
	}
	if len(lines) < 3 || (len(lines)-1)%2 != 0 || strings.TrimSpace(expectedVersion) == "" || strings.TrimSpace(expectedPath) == "" {
		return CopilotStatusUnknown, true
	}
	matches, ok := countCopilotLiveMatches(lines, expected, expectedVersion, expectedPath)
	if !ok {
		return CopilotStatusUnknown, true
	}
	if matches == 1 {
		return CopilotStatusInstalled, true
	}
	if matches > 1 {
		return CopilotStatusUnknown, true
	}
	return CopilotStatusAbsent, true
}

func countCopilotLiveMatches(lines []string, expected, expectedVersion, expectedPath string) (int, bool) {
	seen := make(map[string]bool, (len(lines)-1)/2)
	matches := 0
	for index := 1; index < len(lines); index += 2 {
		matched, ok := copilotLivePairMatch(lines[index], lines[index+1], seen, expected, expectedVersion, expectedPath)
		if !ok {
			return 0, false
		}
		if matched {
			matches++
		}
	}
	return matches, true
}

func copilotLivePairMatch(entryLine, pathLine string, seen map[string]bool, expected, expectedVersion, expectedPath string) (bool, bool) {
	entry := copilotLiveEntry.FindStringSubmatch(entryLine)
	if len(entry) != 4 || seen[entry[1]] || (entry[3] != "enabled" && entry[3] != "disabled") {
		return false, false
	}
	seen[entry[1]] = true
	const pathPrefix = "      from "
	if !strings.HasPrefix(pathLine, pathPrefix) {
		return false, false
	}
	listedPath := strings.TrimPrefix(pathLine, pathPrefix)
	if listedPath == "" || !filepath.IsAbs(listedPath) || listedPath != filepath.Clean(listedPath) {
		return false, false
	}
	if entry[1] != expected {
		return false, true
	}
	if entry[2] != expectedVersion || entry[3] != "enabled" || expectedPath != filepath.Clean(expectedPath) || listedPath != expectedPath {
		return false, false
	}
	return true, true
}

func CopilotPluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) CopilotStatus {
	if status, recognized := CopilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		return status
	}
	scan := copilotPluginScan{}
	for _, rawLine := range strings.Split(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n") {
		if status, done := scan.consume(rawLine, expected, expectedVersion); done {
			return status
		}
	}
	return scan.result()
}

type copilotPluginScan struct {
	inInstalledSection bool
	recognizedSection  bool
	recognizedEntry    bool
	matches            int
}

func (scan *copilotPluginScan) consume(rawLine, expected, expectedVersion string) (CopilotStatus, bool) {
	if strings.TrimSpace(rawLine) == "Installed plugins:" {
		if scan.inInstalledSection {
			return CopilotStatusUnknown, true
		}
		scan.inInstalledSection = true
		scan.recognizedSection = true
		return 0, false
	}
	if !scan.inInstalledSection {
		return 0, false
	}
	if rawLine != "" && rawLine[0] != ' ' && rawLine[0] != '\t' {
		scan.inInstalledSection = false
		return 0, false
	}
	entry := copilotInstalledEntry.FindStringSubmatch(rawLine)
	if len(entry) == 3 {
		scan.recognizedEntry = true
		if entry[1] == expected {
			if entry[2] != expectedVersion {
				return CopilotStatusUnknown, true
			}
			scan.matches++
		}
		return 0, false
	}
	return scan.consumeRemark(rawLine, expected)
}

func (scan *copilotPluginScan) consumeRemark(rawLine, expected string) (CopilotStatus, bool) {
	trimmed := strings.TrimSpace(rawLine)
	if trimmed == "" {
		return 0, false
	}
	if trimmed == "No plugins installed." || trimmed == "No plugins installed" {
		scan.recognizedEntry = true
		return 0, false
	}
	lower := strings.ToLower(trimmed)
	if !strings.Contains(trimmed, expected) {
		return 0, false
	}
	for _, state := range []string{"pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure"} {
		if strings.Contains(lower, state) {
			return CopilotStatusAbsent, true
		}
	}
	return CopilotStatusUnknown, true
}

func (scan copilotPluginScan) result() CopilotStatus {
	if scan.matches == 1 {
		return CopilotStatusInstalled
	}
	if scan.matches > 1 {
		return CopilotStatusUnknown
	}
	if scan.recognizedSection && scan.recognizedEntry {
		return CopilotStatusAbsent
	}
	return CopilotStatusUnknown
}
