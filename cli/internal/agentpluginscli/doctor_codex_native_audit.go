package agentpluginscli

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const doctorNativeFileMax = 1 << 20
const doctorNativeObjectMax = 512

// doctorCodexNativeFindings checks only selected, recorded, project-owned
// objects. The authoritative client state is untouched; this is not a native
// startup, MCP tool call, authentication or trust attestation.
func doctorCodexNativeFindings(state domain.StateFileV2, selected *domain.Installation) []doctorFinding {
	installations := state.Installations
	if selected != nil {
		installations = []domain.Installation{*selected}
	}
	var findings []doctorFinding
	for _, installation := range installations {
		for _, binding := range installation.Clients {
			if binding.ClientID != "codex" || binding.Scope != string(domain.ScopeProject) ||
				binding.Materialization != domain.MaterializationMaterialized {
				continue
			}
			findings = append(findings, doctorCodexBindingFindings(installation, binding)...)
		}
	}
	sortDoctorFindings(findings)
	return findings
}

func sortDoctorFindings(findings []doctorFinding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if a.InstallationID != b.InstallationID {
			return a.InstallationID < b.InstallationID
		}
		if a.ClientID != b.ClientID {
			return a.ClientID < b.ClientID
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Status != b.Status {
			return a.Status < b.Status
		}
		return a.Subject < b.Subject
	})
}

func doctorCodexBindingFindings(installation domain.Installation, binding domain.ClientBinding) []doctorFinding {
	report := func(status, code, explanation string) doctorFinding {
		action := "Inspect the original project-native object receipts and use an explicit, reviewed repair with --scope project --project-root <PROJECT_ROOT>; do not adopt foreign files"
		return scopedFinding(status, code, installation, binding.ClientID, explanation, action)
	}
	// Older records may not contain these facts. Missing evidence is an unknown
	// qualification state, not a fabricated corruption or successful scan.
	if binding.NativeProfileRoot == "" || len(binding.NativeObjects) == 0 {
		return []doctorFinding{report("unknown", "codex_project_receipts_unavailable", "exact project-native ownership evidence is missing; no native files were read")}
	}
	rootPath := binding.NativeProfileRoot
	if !filepath.IsAbs(rootPath) || filepath.Clean(rootPath) != rootPath {
		return []doctorFinding{report("degraded", "codex_project_root_invalid", "the recorded project root is not a canonical absolute path")}
	}
	fi, err := os.Lstat(rootPath)
	if err != nil || !fi.IsDir() || fi.Mode()&os.ModeSymlink != 0 {
		return []doctorFinding{report("unknown", "codex_project_root_unavailable", "the recorded project root cannot be safely inspected as a regular directory")}
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return []doctorFinding{report("unknown", "codex_project_root_unavailable", "the selected native project root could not be opened safely")}
	}
	defer root.Close()

	var findings []doctorFinding
	appendCode := func(status, code, explanation string) {
		findings = append(findings, report(status, code, explanation))
	}
	if len(binding.NativeObjects) > doctorNativeObjectMax {
		appendCode("unknown", "codex_project_inventory_too_large", "project-native object inventory exceeds the safe observational bound")
		return findings
	}
	seen := map[string]bool{}
	seenPaths := map[string]bool{}
	var mcpNames []string
	var mcpPath string
	for _, object := range binding.NativeObjects {
		var expected string
		var wantDirectory bool
		switch object.Kind {
		case "codex_project_skill_directory":
			wantDirectory = true
			if !doctorNativeLeafName(object.LogicalName) {
				appendCode("degraded", "codex_project_invalid_skill_receipt", "a recorded project Skill identity is not a single native Skill name")
				continue
			}
			expected = filepath.Join(rootPath, ".agents", "skills", object.LogicalName)
		case "codex_project_mcp_server":
			if !doctorNativeLeafName(object.LogicalName) {
				appendCode("degraded", "codex_project_invalid_mcp_receipt", "a recorded project MCP identity is not a single native server name")
				continue
			}
			expected = filepath.Join(rootPath, ".codex", "config.toml")
			mcpNames = append(mcpNames, object.LogicalName)
			mcpPath = expected
		default:
			continue // The user's foreign and other client-native objects are not ours.
		}
		key := object.Kind + "\x00" + object.LogicalName
		if seen[key] {
			appendCode("degraded", "codex_project_duplicate_receipt", "recorded project-native object identities are duplicated")
			continue
		}
		seen[key] = true
		if object.ProtectionClass != "managed" && object.ProtectionClass != "managed_entry" {
			appendCode("unknown", "codex_project_ownership_unverified", "the project-native object has no supported manager ownership class")
			continue
		}
		if object.Path != expected {
			appendCode("degraded", "codex_project_path_mismatch", "the recorded native object does not match Codex's exact project-local path")
			continue
		}
		pathKey := object.Kind + "\x00" + object.Path
		if seenPaths[pathKey] {
			// Several MCP server members intentionally share one TOML document.
			if object.Kind == "codex_project_mcp_server" {
				continue
			}
			appendCode("degraded", "codex_project_duplicate_path", "multiple Skill identities claim the same native path")
			continue
		}
		seenPaths[pathKey] = true
		rel, err := filepath.Rel(rootPath, expected)
		if err != nil {
			appendCode("degraded", "codex_project_path_mismatch", "the native object cannot be resolved beneath its selected project")
			continue
		}
		code := doctorNativePathStatus(root, rel, wantDirectory)
		if code != "" {
			appendCode(doctorNativeStatusSeverity(code), "codex_project_"+code, "the recorded native component is missing, inaccessible, linked or has the wrong filesystem type")
			continue
		}
		if wantDirectory {
			skillPath := filepath.Join(rel, "SKILL.md")
			if issue := doctorNativePathStatus(root, skillPath, false); issue != "" {
				appendCode(doctorNativeStatusSeverity(issue), "codex_project_skill_"+issue, "the recorded project Skill lacks a safe, regular SKILL.md file")
			}
		}
	}
	if len(mcpNames) > 0 && mcpPath != "" {
		rel, err := filepath.Rel(rootPath, mcpPath)
		if err != nil {
			appendCode("degraded", "codex_project_path_mismatch", "recorded MCP configuration is outside the selected project")
		} else if doctorNativePathStatus(root, rel, false) == "" {
			contents, issue := doctorNativeRead(root, rel)
			if issue != "" {
				appendCode(doctorNativeStatusSeverity(issue), "codex_project_mcp_"+issue, "project MCP configuration cannot be inspected safely")
			} else {
				var document map[string]any
				if err := toml.Unmarshal(contents, &document); err != nil {
					appendCode("degraded", "codex_project_mcp_invalid", "the recorded project MCP TOML configuration is invalid")
				} else {
					servers, ok := document["mcp_servers"].(map[string]any)
					if !ok {
						appendCode("degraded", "codex_project_mcp_missing", "the project MCP server table is missing")
					} else {
						for _, name := range mcpNames {
							if _, exists := servers[name]; !exists {
								appendCode("degraded", "codex_project_mcp_member_missing", "a recorded project MCP server member is missing from native configuration")
							}
						}
					}
				}
			}
		}
	}
	return findings
}

func doctorNativeLeafName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\") && len(name) <= 255
}

// Root.Lstat/Open are traversal-resistant across Linux, macOS and Windows.
// Inspect each prefix to reject even in-root symlinks and Windows reparse-point
// redirections, then open only through the same root handle.
func doctorNativePathStatus(root *os.Root, relative string, directory bool) string {
	if relative == "" || filepath.IsAbs(relative) || filepath.Clean(relative) != relative ||
		relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "path_invalid"
	}
	segments := strings.Split(relative, string(filepath.Separator))
	if len(segments) > 32 {
		return "path_invalid"
	}
	walk := ""
	for i, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "path_invalid"
		}
		walk = filepath.Join(walk, segment)
		info, err := root.Lstat(walk)
		switch {
		case errors.Is(err, os.ErrNotExist):
			return "component_missing"
		case err != nil:
			return "component_unreadable"
		case info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0:
			return "component_symlink"
		case i < len(segments)-1 && !info.IsDir():
			return "component_wrong_type"
		case i == len(segments)-1 && directory && !info.IsDir():
			return "component_wrong_type"
		case i == len(segments)-1 && !directory && !info.Mode().IsRegular():
			return "component_wrong_type"
		}
	}
	return ""
}

func doctorNativeRead(root *os.Root, path string) ([]byte, string) {
	file, err := root.Open(path)
	if err != nil {
		return nil, "component_unreadable"
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, doctorNativeFileMax+1))
	if err != nil {
		return nil, "component_unreadable"
	}
	if len(body) > doctorNativeFileMax {
		return nil, "component_too_large"
	}
	return body, ""
}

func doctorNativeStatusSeverity(code string) string {
	switch code {
	case "component_unreadable", "component_too_large":
		return "unknown"
	default:
		return "degraded"
	}
}
