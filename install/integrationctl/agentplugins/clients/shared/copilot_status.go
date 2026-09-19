package shared

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

// CopilotStatus is the recognized shape of a Copilot / VS Code plugin listing.
type CopilotStatus int

const (
	CopilotStatusUnknown CopilotStatus = iota
	CopilotStatusInstalled
	CopilotStatusAbsent
)

// ErrCopilotListContractUnknown marks a listing whose shape this adapter does
// not recognize. Callers treat it as a manual verification, not a proof of
// absence.
var ErrCopilotListContractUnknown = errors.New("the Copilot plugin list output is not recognized")

var CopilotInstalledEntry = regexp.MustCompile(`^[ \t]+•[ \t]+([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*)[ \t]+\(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\)[ \t]*$`)
var copilotLiveEntry = regexp.MustCompile(`^  • ([A-Za-z0-9][A-Za-z0-9._-]*@[A-Za-z0-9][A-Za-z0-9._-]*) \(v([0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?)\) \(([A-Za-z0-9_-]+)\)$`)

const CopilotLiveHeader = "Live Plugins (loaded from a local marketplace directory, never copied):"

// CopilotPluginStatus classifies a `plugin list` document for the expected
// identity, version and managed path.
func CopilotPluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) CopilotStatus {
	if status, recognized := CopilotLivePluginStatus(stdout, expected, expectedVersion, expectedPath); recognized {
		return status
	}
	return copilotInstalledPluginStatus(stdout, expected, expectedVersion)
}

// CopilotLivePluginStatus classifies the live-plugin listing. The second
// result is whether the document used that listing at all.
func CopilotLivePluginStatus(stdout []byte, expected, expectedVersion, expectedPath string) (CopilotStatus, bool) {
	document := strings.TrimSuffix(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n")
	lines := strings.Split(document, "\n")
	if len(lines) == 0 || lines[0] != CopilotLiveHeader {
		return CopilotStatusUnknown, false
	}
	if len(lines) < 3 || (len(lines)-1)%2 != 0 || strings.TrimSpace(expectedVersion) == "" || strings.TrimSpace(expectedPath) == "" {
		return CopilotStatusUnknown, true
	}
	return classifyCopilotLiveEntries(lines, expected, expectedVersion, expectedPath)
}

func classifyCopilotLiveEntries(lines []string, expected, expectedVersion, expectedPath string) (CopilotStatus, bool) {
	seen := make(map[string]bool, (len(lines)-1)/2)
	matches := 0
	for index := 1; index < len(lines); index += 2 {
		match, ok := copilotLivePair(lines, index, seen, expected, expectedVersion, expectedPath)
		if !ok {
			return CopilotStatusUnknown, true
		}
		if match {
			matches++
		}
	}
	if matches == 1 {
		return CopilotStatusInstalled, true
	}
	if matches > 1 {
		return CopilotStatusUnknown, true
	}
	return CopilotStatusAbsent, true
}

func copilotLivePair(lines []string, index int, seen map[string]bool, expected, expectedVersion, expectedPath string) (match bool, ok bool) {
	entry := copilotLiveEntry.FindStringSubmatch(lines[index])
	if len(entry) != 4 || seen[entry[1]] || (entry[3] != "enabled" && entry[3] != "disabled") {
		return false, false
	}
	seen[entry[1]] = true
	const pathPrefix = "      from "
	if !strings.HasPrefix(lines[index+1], pathPrefix) {
		return false, false
	}
	listedPath := strings.TrimPrefix(lines[index+1], pathPrefix)
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

type copilotInstalledScan struct {
	inSection         bool
	recognizedSection bool
	recognizedEntry   bool
	matches           int
}

func copilotInstalledPluginStatus(stdout []byte, expected, expectedVersion string) CopilotStatus {
	scan := copilotInstalledScan{}
	for _, rawLine := range strings.Split(strings.ReplaceAll(string(stdout), "\r\n", "\n"), "\n") {
		status, stop := scan.consume(rawLine, expected, expectedVersion)
		if stop {
			return status
		}
	}
	return copilotInstalledMatchStatus(scan.matches, scan.recognizedSection, scan.recognizedEntry)
}

func (scan *copilotInstalledScan) consume(rawLine, expected, expectedVersion string) (CopilotStatus, bool) {
	if strings.TrimSpace(rawLine) == "Installed plugins:" {
		if scan.inSection {
			return CopilotStatusUnknown, true
		}
		scan.inSection = true
		scan.recognizedSection = true
		return CopilotStatusUnknown, false
	}
	if !scan.inSection {
		return CopilotStatusUnknown, false
	}
	if rawLine != "" && rawLine[0] != ' ' && rawLine[0] != '\t' {
		scan.inSection = false
		return CopilotStatusUnknown, false
	}
	return scan.consumeBody(rawLine, expected, expectedVersion)
}

func (scan *copilotInstalledScan) consumeBody(rawLine, expected, expectedVersion string) (CopilotStatus, bool) {
	entry := CopilotInstalledEntry.FindStringSubmatch(rawLine)
	if len(entry) == 3 {
		scan.recognizedEntry = true
		if entry[1] != expected {
			return CopilotStatusUnknown, false
		}
		if entry[2] != expectedVersion {
			return CopilotStatusUnknown, true
		}
		scan.matches++
		return CopilotStatusUnknown, false
	}
	return scan.consumeNote(rawLine, expected)
}

func (scan *copilotInstalledScan) consumeNote(rawLine, expected string) (CopilotStatus, bool) {
	trimmed := strings.TrimSpace(rawLine)
	if trimmed == "" {
		return CopilotStatusUnknown, false
	}
	if trimmed == "No plugins installed." || trimmed == "No plugins installed" {
		scan.recognizedEntry = true
		return CopilotStatusUnknown, false
	}
	if !strings.Contains(trimmed, expected) {
		return CopilotStatusUnknown, false
	}
	lower := strings.ToLower(trimmed)
	for _, state := range []string{"pending", "disconnected", "disabled", "auth-required", "auth required", "authentication required", "error", "failed", "failure"} {
		if strings.Contains(lower, state) {
			return CopilotStatusAbsent, true
		}
	}
	return CopilotStatusUnknown, true
}

func copilotInstalledMatchStatus(matches int, recognizedSection, recognizedEntry bool) CopilotStatus {
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
