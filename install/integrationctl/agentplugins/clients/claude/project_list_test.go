package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectListingRequiresExactScopePathAndNoLoadErrors(t *testing.T) {
	project := t.TempDir()
	active := filepath.Join(project, ".claude", "skills", "demo-managed")
	for _, change := range []string{"exact", "user-scope", "other-path", "other-project", "nonclean-path", "nonclean-project", "disabled", "load-errors", "malformed-errors", "duplicate"} {
		t.Run(change, func(t *testing.T) {
			entry := map[string]any{"id": "demo@skills-dir", "scope": "project", "enabled": true, "installPath": active, "projectPath": project}
			switch change {
			case "user-scope":
				entry["scope"] = "user"
			case "other-path":
				entry["installPath"] = active + "-other"
			case "other-project":
				entry["projectPath"] = project + "-other"
			case "nonclean-path":
				entry["installPath"] = filepath.Dir(active) + string(filepath.Separator) + "alias" + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(active)
			case "nonclean-project":
				entry["projectPath"] = project + string(filepath.Separator) + "alias" + string(filepath.Separator) + ".."
			case "disabled":
				entry["enabled"] = false
			case "load-errors":
				entry["errors"] = []string{"managed policy blocked load"}
			case "malformed-errors":
				entry["errors"] = "unrecognized"
			}
			entries := []map[string]any{entry}
			if change == "duplicate" {
				entries = append(entries, entry)
			}
			body, err := json.Marshal(entries)
			if err != nil {
				t.Fatal(err)
			}
			got := PluginStatusFromListForScope(body, "demo", active, domain.ScopeProject)
			if (got == StatusInstalled) != (change == "exact") {
				t.Fatalf("%s listing status=%v", change, got)
			}
			if change == "exact" && PluginStatusFromList(body, "demo", active) == StatusInstalled {
				t.Fatal("project listing weakened user-only caller")
			}
		})
	}
}

func TestProjectListingRejectsSymlinkTraversalSpelling(t *testing.T) {
	project, other := t.TempDir(), t.TempDir()
	for _, path := range []*string{&project, &other} {
		physical, err := filepath.EvalSymlinks(*path)
		if err != nil {
			t.Fatal(err)
		}
		*path = physical
	}
	skills := filepath.Join(project, ".claude", "skills")
	for _, path := range []string{skills, filepath.Join(other, "sub"), filepath.Join(other, "demo-managed")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(filepath.Join(other, "sub"), filepath.Join(skills, "alias")); err != nil {
		t.Skipf("directory symlink unavailable: %v", err)
	}
	active := filepath.Join(skills, "demo-managed")
	listingPath := filepath.Join(skills, "alias") + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(active)
	resolved, err := filepath.EvalSymlinks(listingPath)
	if err != nil || resolved != filepath.Join(other, "demo-managed") || filepath.Clean(listingPath) != active {
		t.Fatalf("traversal fixture=%q,%v", resolved, err)
	}
	body, err := json.Marshal([]map[string]any{{"id": "demo@skills-dir", "scope": "project", "enabled": true, "installPath": listingPath, "projectPath": project}})
	if err != nil {
		t.Fatal(err)
	}
	if PluginStatusFromListForScope(body, "demo", active, domain.ScopeProject) == StatusInstalled {
		t.Fatal("native traversal spelling manufactured project identity")
	}
}
