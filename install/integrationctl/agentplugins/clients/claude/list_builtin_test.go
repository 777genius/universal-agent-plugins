package claude

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Rows as printed by Claude Code 2.1.332 `plugin list --json` for bundled plugins.
func claudeBuiltinRows() []map[string]any {
	return []map[string]any{
		{"id": "cc-plugin-agents-md@builtin", "version": "unknown", "scope": "user", "enabled": true, "hasUserConfig": true},
		{"id": "cc-plugin-privacy@builtin", "version": "unknown", "scope": "user", "enabled": true},
	}
}

func TestListingSkipsBuiltinRowsWithoutInstallPath(t *testing.T) {
	active := filepath.Join(t.TempDir(), "claude-config", "skills", "demo-managed")
	localPath := filepath.Join(t.TempDir(), "cache", "x", "1.0.0")
	managed := map[string]any{"id": "demo@skills-dir", "version": "1.0.0", "scope": "user", "enabled": true, "installPath": active}
	for _, test := range []struct {
		name string
		edit func([]map[string]any) []map[string]any
		want ListStatus
	}{
		{"builtin-and-managed", func(rows []map[string]any) []map[string]any { return append(rows, managed) }, StatusInstalled},
		{"managed-and-builtin", func(rows []map[string]any) []map[string]any { return append([]map[string]any{managed}, rows...) }, StatusInstalled},
		{"builtin-only", func(rows []map[string]any) []map[string]any { return rows }, StatusAbsent},
		{"duplicate-builtin", func(rows []map[string]any) []map[string]any { return append(rows, rows[0], managed) }, StatusUnknown},
		{"builtin-empty-path", func(rows []map[string]any) []map[string]any {
			rows[0]["installPath"] = ""
			return append(rows, managed)
		}, StatusUnknown},
		{"builtin-no-enabled", func(rows []map[string]any) []map[string]any { delete(rows[0], "enabled"); return append(rows, managed) }, StatusUnknown},
		{"builtin-no-scope", func(rows []map[string]any) []map[string]any { delete(rows[0], "scope"); return append(rows, managed) }, StatusUnknown},
		{"bare-builtin-id", func(rows []map[string]any) []map[string]any { rows[0]["id"] = "@builtin"; return append(rows, managed) }, StatusUnknown},
		{"other-source-no-path", func(rows []map[string]any) []map[string]any { rows[0]["id"] = "x@market"; return append(rows, managed) }, StatusUnknown},
		{"local-rows-per-project", func(rows []map[string]any) []map[string]any {
			local := func(project string) map[string]any {
				return map[string]any{"id": "x@market", "scope": "local", "enabled": true, "installPath": localPath, "projectPath": project}
			}
			return append(rows, local("/p/one"), local("/p/two"), managed)
		}, StatusInstalled},
		{"duplicate-local-row", func(rows []map[string]any) []map[string]any {
			local := map[string]any{"id": "x@market", "scope": "local", "enabled": true, "installPath": localPath, "projectPath": "/p/one"}
			return append(rows, local, local, managed)
		}, StatusUnknown},
		{"managed-no-path", func(rows []map[string]any) []map[string]any {
			return append(rows, map[string]any{"id": "demo@skills-dir", "scope": "user", "enabled": true})
		}, StatusUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			body, err := json.Marshal(test.edit(claudeBuiltinRows()))
			if err != nil {
				t.Fatal(err)
			}
			if got := PluginStatusFromList(body, "demo", active); got != test.want {
				t.Fatalf("status=%v want=%v body=%s", got, test.want, body)
			}
		})
	}
}

func TestProjectListingSkipsBuiltinRows(t *testing.T) {
	fixture := projectListFixture(t)
	rows := append(claudeBuiltinRows(), fixture.selected)
	body, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if got := PluginStatusFromListForScope(body, "demo", fixture.active, domain.ScopeProject); got != StatusInstalled {
		t.Fatalf("project status=%v", got)
	}
	if got := observeClaudeProjectList(body, "demo", fixture.active, fixture.root, fixture.user); got.state != projectListEnabled {
		t.Fatalf("project observation=%+v", got)
	}
	dup, err := json.Marshal(append(rows, claudeBuiltinRows()[0]))
	if err != nil {
		t.Fatal(err)
	}
	if got := observeClaudeProjectList(dup, "demo", fixture.active, fixture.root, fixture.user); got.state != projectListUnknown {
		t.Fatalf("duplicate builtin observation=%+v", got)
	}
}
