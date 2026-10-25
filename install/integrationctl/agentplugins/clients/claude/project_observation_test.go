package claude

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

type claudeProjectListFixture struct {
	root, user, active          string
	selected, other, suppressed map[string]any
}

func projectListFixture(t *testing.T) claudeProjectListFixture {
	t.Helper()
	base := t.TempDir()
	root := filepath.Join(base, "A")
	user := filepath.Join(base, "user", ".claude")
	active := filepath.Join(root, ".claude", "skills", "demo-managed")
	selected := map[string]any{"id": "demo@skills-dir", "scope": "project", "enabled": true, "installPath": active, "projectPath": root}
	other := map[string]any{"id": "demo@skills-dir", "scope": "user", "enabled": true, "installPath": filepath.Join(user, "skills", "demo-user")}
	suppressed := map[string]any{"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project", "enabled": false, "installPath": "", "notes": []string{claudeUntrustedProjectNote}}
	return claudeProjectListFixture{root: root, user: user, active: active, selected: selected, other: other, suppressed: suppressed}
}

func TestProjectObservationSeparatesBlockedDiscoveryFromIdentity(t *testing.T) {
	for _, test := range []struct {
		name string
		want projectListState
	}{
		{"absent", projectListAbsent}, {"exact", projectListEnabled}, {"disabled", projectListDisabled},
		{"suppressed", projectListSuppressed}, {"opaque-suppression", projectListSuppressed}, {"user-source", projectListOtherSource},
		{"user-and-suppressed", projectListSuppressed}, {"suppressed-and-user", projectListSuppressed},
		{"exact-and-user", projectListUnknown}, {"user-and-exact", projectListUnknown}, {"exact-and-suppressed", projectListUnknown},
		{"suppressed-and-exact", projectListUnknown}, {"duplicate-suppression", projectListUnknown}, {"duplicate-exact", projectListUnknown},
		{"foreign-project", projectListForeign}, {"other-project", projectListUnknown}, {"load-errors", projectListUnknown}, {"load-notes", projectListUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := projectListFixture(t)
			root, user, active := fixture.root, fixture.user, fixture.active
			selected, other, suppressed := fixture.selected, fixture.other, fixture.suppressed
			rows := []map[string]any{}
			switch test.name {
			case "exact":
				rows = append(rows, selected)
			case "disabled":
				selected["enabled"] = false
				rows = append(rows, selected)
			case "suppressed":
				rows = append(rows, suppressed)
			case "opaque-suppression":
				suppressed["notes"] = []string{"A future native suppression reason."}
				rows = append(rows, suppressed)
			case "user-source":
				rows = append(rows, other)
			case "user-and-suppressed":
				rows = append(rows, other, suppressed)
			case "suppressed-and-user":
				rows = append(rows, suppressed, other)
			case "exact-and-user":
				rows = append(rows, selected, other)
			case "user-and-exact":
				rows = append(rows, other, selected)
			case "exact-and-suppressed":
				rows = append(rows, selected, suppressed)
			case "suppressed-and-exact":
				rows = append(rows, suppressed, selected)
			case "duplicate-suppression":
				rows = append(rows, suppressed, suppressed)
			case "duplicate-exact":
				rows = append(rows, selected, selected)
			case "foreign-project":
				selected["installPath"] = active + "-foreign"
				rows = append(rows, selected)
			case "other-project":
				selected["projectPath"] = filepath.Join(filepath.Dir(root), "B")
				rows = append(rows, selected)
			case "load-errors", "load-notes":
				key := "errors"
				if test.name == "load-notes" {
					key = "notes"
				}
				selected[key] = []string{"Not loaded"}
				rows = append(rows, selected)
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			got := observeClaudeProjectList(body, "demo", active, root, user)
			if got.state != test.want {
				t.Fatalf("observation=%+v,want=%v", got, test.want)
			}
			if test.name == "opaque-suppression" && got.untrusted {
				t.Fatal("unknown notes manufactured trust evidence")
			}
			if test.name == "suppressed" && !got.untrusted {
				t.Fatal("known trust note lost")
			}
			if PluginStatusFromList(body, "demo", active) == StatusInstalled {
				t.Fatal("project observer weakened strict user contract")
			}
		})
	}
}

func TestProjectSuppressionRejectsMalformedNearMisses(t *testing.T) {
	for _, change := range []string{"id", "scope", "version", "enabled", "path", "notes-string", "notes-empty", "notes-blank", "notes-type", "notes-long", "extra", "mixed-malformed", "duplicate-key", "trailing"} {
		t.Run(change, func(t *testing.T) {
			fixture := projectListFixture(t)
			root, user, active, sentinel := fixture.root, fixture.user, fixture.active, fixture.suppressed
			switch change {
			case "id":
				sentinel["id"] = "suppressed@skills-dir"
			case "scope":
				sentinel["scope"] = "user"
			case "version":
				sentinel["version"] = "1.0.0"
			case "enabled":
				sentinel["enabled"] = true
			case "path":
				sentinel["installPath"] = active
			case "notes-string":
				sentinel["notes"] = "trust"
			case "notes-empty":
				sentinel["notes"] = []string{}
			case "notes-blank":
				sentinel["notes"] = []string{" "}
			case "notes-type":
				sentinel["notes"] = []any{true}
			case "notes-long":
				sentinel["notes"] = []string{string(make([]byte, 4097))}
			case "extra":
				sentinel["errors"] = []string{}
			}
			rows := []map[string]any{sentinel}
			if change == "mixed-malformed" {
				rows = append(rows, map[string]any{"id": "unrelated", "scope": "user", "enabled": true, "installPath": "relative"})
			}
			body, err := json.Marshal(rows)
			if err != nil {
				t.Fatal(err)
			}
			if change == "duplicate-key" {
				body = []byte(`[{"id":"(suppressed)@skills-dir","id":"(suppressed)@skills-dir"}]`)
			}
			if change == "trailing" {
				body = append(body, []byte(` {}`)...)
			}
			if got := observeClaudeProjectList(body, "demo", active, root, user); got.state != projectListUnknown {
				t.Fatalf("malformed listing became %+v", got)
			}
		})
	}
}
