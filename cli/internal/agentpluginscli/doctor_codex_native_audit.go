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

// One observer is scoped to exactly one Codex project binding and never exposes
// raw native configuration, paths or credentials in a finding.
type codexDoctorInspection struct {
	installation domain.Installation
	binding      domain.ClientBinding
	rootPath     string
	root         *os.Root
	seen         map[string]bool
	mcpNames     []string
	findings     []doctorFinding
}

func (d *codexDoctorInspection) add(status, code, explanation string) {
	d.findings = append(d.findings, scopedFinding(status, code, d.installation, d.binding.ClientID,
		explanation, "Inspect the original project-native object receipts and use an explicit, reviewed repair with --scope project --project-root \"<PROJECT_ROOT>\"; do not adopt foreign files"))
}

func doctorCodexBindingFindings(installation domain.Installation, binding domain.ClientBinding) []doctorFinding {
	d := &codexDoctorInspection{installation: installation, binding: binding,
		rootPath: binding.NativeProfileRoot, seen: map[string]bool{}}
	if d.rootPath == "" || len(binding.NativeObjects) == 0 {
		d.add("unknown", "codex_project_receipts_unavailable",
			"exact project-native ownership evidence is missing; no native files were read")
		return d.findings
	}
	if !filepath.IsAbs(d.rootPath) || filepath.Clean(d.rootPath) != d.rootPath {
		d.add("degraded", "codex_project_root_invalid",
			"the recorded project root is not a canonical absolute path")
		return d.findings
	}
	info, err := os.Lstat(d.rootPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		d.add("unknown", "codex_project_root_unavailable",
			"the recorded project root cannot be safely inspected as a regular directory")
		return d.findings
	}
	d.root, err = os.OpenRoot(d.rootPath)
	if err != nil {
		d.add("unknown", "codex_project_root_unavailable",
			"the selected native project root could not be opened safely")
		return d.findings
	}
	defer func() { _ = d.root.Close() }()
	opened, err := d.root.Stat(".")
	if err != nil || !os.SameFile(info, opened) {
		d.add("unknown", "codex_project_root_changed",
			"the recorded project root changed during the read-only inspection")
		return d.findings
	}
	if len(binding.NativeObjects) > doctorNativeObjectMax {
		d.add("unknown", "codex_project_inventory_too_large",
			"project-native object inventory exceeds the safe observational bound")
		return d.findings
	}
	for _, object := range binding.NativeObjects {
		d.inspectObject(object)
	}
	d.inspectMCP()
	return d.findings
}

func (d *codexDoctorInspection) inspectObject(object domain.NativeObjectOwnership) {
	var expected string
	var directory bool
	switch object.Kind {
	case "codex_project_skill_directory":
		if !doctorNativeLeafName(object.LogicalName) {
			d.add("degraded", "codex_project_invalid_skill_receipt",
				"a recorded project Skill identity is not a single native Skill name")
			return
		}
		expected, directory = filepath.Join(d.rootPath, ".agents", "skills", object.LogicalName), true
	case "codex_project_mcp_server":
		if !doctorNativeLeafName(object.LogicalName) {
			d.add("degraded", "codex_project_invalid_mcp_receipt",
				"a recorded project MCP identity is not a single native server name")
			return
		}
		expected = filepath.Join(d.rootPath, ".codex", "config.toml")
		d.mcpNames = append(d.mcpNames, object.LogicalName)
	default:
		return // Foreign objects and unrelated client surfaces are not ours.
	}
	identity := object.Kind + "\x00" + object.LogicalName
	if d.seen[identity] {
		d.add("degraded", "codex_project_duplicate_receipt",
			"recorded project-native object identities are duplicated")
		return
	}
	d.seen[identity] = true
	if object.ProtectionClass != "managed" && object.ProtectionClass != "managed_entry" {
		d.add("unknown", "codex_project_ownership_unverified",
			"the project-native object has no supported manager ownership class")
		return
	}
	if object.Path != expected {
		d.add("degraded", "codex_project_path_mismatch",
			"the recorded native object does not match Codex's exact project-local path")
		return
	}
	relative, err := filepath.Rel(d.rootPath, expected)
	if err != nil {
		d.add("degraded", "codex_project_path_mismatch",
			"the native object cannot be resolved beneath its selected project")
		return
	}
	if issue := doctorNativePathStatus(d.root, relative, directory); issue != "" {
		d.add(doctorNativeStatusSeverity(issue), "codex_project_"+issue,
			"the recorded native component is missing, inaccessible, linked or has the wrong filesystem type")
		return
	}
	if directory {
		if issue := doctorNativePathStatus(d.root, filepath.Join(relative, "SKILL.md"), false); issue != "" {
			d.add(doctorNativeStatusSeverity(issue), "codex_project_skill_"+issue,
				"the recorded project Skill lacks a safe, regular SKILL.md file")
		}
	}
}

func (d *codexDoctorInspection) inspectMCP() {
	if len(d.mcpNames) == 0 {
		return
	}
	relative := filepath.Join(".codex", "config.toml")
	if doctorNativePathStatus(d.root, relative, false) != "" {
		return // Per-object path diagnostics already reported the missing native file.
	}
	contents, issue := doctorNativeRead(d.root, relative)
	if issue != "" {
		d.add(doctorNativeStatusSeverity(issue), "codex_project_mcp_"+issue,
			"project MCP configuration cannot be inspected safely")
		return
	}
	var document map[string]any
	if err := toml.Unmarshal(contents, &document); err != nil {
		d.add("degraded", "codex_project_mcp_invalid",
			"the recorded project MCP TOML configuration is invalid")
		return
	}
	servers, ok := document["mcp_servers"].(map[string]any)
	if !ok {
		d.add("degraded", "codex_project_mcp_missing", "the project MCP server table is missing")
		return
	}
	for _, name := range d.mcpNames {
		if _, found := servers[name]; !found {
			d.add("degraded", "codex_project_mcp_member_missing",
				"a recorded project MCP server member is missing from native configuration")
		}
	}
}

func doctorNativeLeafName(name string) bool {
	return name != "" && name != "." && name != ".." &&
		!strings.ContainsAny(name, "/\\") && len(name) <= 255
}

// Root.Lstat/Open are traversal-resistant across Linux, macOS and Windows.
// Inspect each prefix to reject even in-root symlinks and Windows reparse-point
// redirections, then open only through the same root handle.
func doctorNativePathStatus(root *os.Root, relative string, directory bool) string {
	if !doctorNativeRelativePathValid(relative) {
		return "path_invalid"
	}
	parts := strings.Split(relative, string(filepath.Separator))
	path := ""
	for index, part := range parts {
		path = filepath.Join(path, part)
		if issue := doctorNativeEntryStatus(root, path, index == len(parts)-1, directory); issue != "" {
			return issue
		}
	}
	return ""
}

func doctorNativeRelativePathValid(relative string) bool {
	if relative == "" || relative == "." || relative == ".." || filepath.IsAbs(relative) ||
		filepath.Clean(relative) != relative || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		len(relative) > 4096 {
		return false
	}
	parts := strings.Split(relative, string(filepath.Separator))
	if len(parts) > 32 {
		return false
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func doctorNativeEntryStatus(root *os.Root, path string, last, directory bool) string {
	info, err := root.Lstat(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "component_missing"
	case err != nil:
		return "component_unreadable"
	case info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0:
		return "component_symlink"
	case !last && !info.IsDir():
		return "component_wrong_type"
	case last && directory && !info.IsDir():
		return "component_wrong_type"
	case last && !directory && !info.Mode().IsRegular():
		return "component_wrong_type"
	default:
		return ""
	}
}

func doctorNativeRead(root *os.Root, path string) ([]byte, string) {
	file, err := root.Open(path)
	if err != nil {
		return nil, "component_unreadable"
	}
	defer func() { _ = file.Close() }()
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
