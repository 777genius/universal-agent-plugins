package claude

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestClaudeProjectMarketplaceNativeSchemas(t *testing.T) {
	r := newClaudeMarketplaceRequest(t)
	market := ProjectMarketplaceName(r.Plan.PhysicalArtifactID, r.Plan.TargetContext.ProjectRoot)
	cache := filepath.Join(r.Client.ConfigRoot, "plugins", "cache", market, r.DeclaredName, "1.0.0")
	for _, test := range []struct {
		name string
		want projectListState
	}{
		{"2.1.263", projectListEnabled}, {"2.1.325", projectListEnabled}, {"untrusted-with-exact-install", projectListEnabled},
		{"unrelated-other-project", projectListEnabled}, {"disabled", projectListDisabled}, {"same-id-other-project", projectListForeign},
		{"user-scope", projectListForeign}, {"wrong-cache", projectListForeign}, {"wrong-folder", projectListUnknown},
		{"partial-modern-fields", projectListUnknown}, {"wrong-version", projectListUnknown}, {"load-error", projectListUnknown}, {"duplicate", projectListUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			row := map[string]any{"id": "demo@" + market, "version": "1.0.0", "scope": "project", "projectPath": r.Plan.TargetContext.ProjectRoot, "installPath": cache, "enabled": true}
			rows := []map[string]any{row}
			if test.name == "2.1.325" || test.name == "wrong-folder" || test.name == "partial-modern-fields" {
				row["readFromFolder"], row["folderVersion"], row["projectEnabled"] = filepath.Join(r.Plan.ActivePath, claudeProjectBundleDirectory), "1.0.0", true
			}
			switch test.name {
			case "untrusted-with-exact-install":
				rows = append(rows, map[string]any{"id": "(suppressed)@skills-dir", "version": "unknown", "scope": "project", "enabled": false, "installPath": "", "notes": []string{claudeUntrustedProjectNote}})
			case "unrelated-other-project":
				rows = append(rows, map[string]any{"id": "foreign@other", "version": "1.0.0", "scope": "project", "projectPath": filepath.Join(filepath.Dir(r.Plan.TargetContext.ProjectRoot), "B"), "installPath": filepath.Join(r.Client.ConfigRoot, "plugins/cache/other/foreign/1.0.0"), "enabled": false})
			case "disabled":
				row["enabled"] = false
			case "same-id-other-project":
				row["projectPath"] = filepath.Join(filepath.Dir(r.Plan.TargetContext.ProjectRoot), "B")
			case "user-scope":
				row["scope"] = "user"
			case "wrong-cache":
				row["installPath"] = cache + "-foreign"
			case "wrong-folder":
				row["readFromFolder"] = r.Plan.ActivePath
			case "partial-modern-fields":
				delete(row, "folderVersion")
			case "wrong-version":
				row["version"] = "2.0.0"
				row["installPath"] = filepath.Join(filepath.Dir(cache), "2.0.0")
			case "load-error":
				row["errors"] = []string{"Not loaded"}
			case "duplicate":
				rows = append(rows, row)
			}
			body, _ := json.Marshal(rows)
			if got := observeClaudeMarketplacePlugin(body, r.DeclaredName, r.Plan.PhysicalArtifactID, r.Plan.ActivePath, r.Plan.TargetContext.ProjectRoot, r.Client.ConfigRoot, "1.0.0"); got != test.want {
				t.Fatalf("state=%v want=%v body=%s", got, test.want, body)
			}
		})
	}
}

func TestClaudeProjectMarketplaceNamespacesAndForeignBindings(t *testing.T) {
	r := newClaudeMarketplaceRequest(t)
	project := r.Plan.TargetContext.ProjectRoot
	a := ProjectMarketplaceName(r.Plan.PhysicalArtifactID, project)
	b := ProjectMarketplaceName(r.Plan.PhysicalArtifactID, filepath.Join(filepath.Dir(project), "B"))
	if a == b || a != ProjectMarketplaceName(r.Plan.PhysicalArtifactID, project+string(filepath.Separator)) {
		t.Fatal("project namespace collision or unclean identity")
	}
	cache := filepath.Join(r.Client.ConfigRoot, "plugins", "cache", a, r.DeclaredName, "1.0.0")
	id := r.DeclaredName + "@" + a
	indexPath := filepath.Join(r.Client.ConfigRoot, "plugins", "installed_plugins.json")
	row := map[string]any{"scope": "project", "projectPath": project, "installPath": cache, "version": "1.0.0"}
	for _, condition := range []string{"exact", "other-project", "user-scope", "duplicate", "foreign-plugin"} {
		rows := []map[string]any{shared.CloneObject(row)}
		plugins := map[string]any{id: rows}
		switch condition {
		case "other-project":
			rows[0]["projectPath"] = filepath.Join(filepath.Dir(project), "B")
		case "user-scope":
			rows[0]["scope"] = "user"
		case "duplicate":
			plugins[id] = append(rows, rows[0])
		case "foreign-plugin":
			plugins["foreign@"+a] = []any{row}
		}
		if err := shared.WriteJSON(indexPath, map[string]any{"version": 2, "plugins": plugins}); err != nil {
			t.Fatal(err)
		}
		err := verifyClaudeProjectBindings(r.Client.ConfigRoot, r.DeclaredName, r.Plan.PhysicalArtifactID, project, cache)
		if (err == nil) != (condition == "exact") {
			t.Fatalf("%s binding=%v", condition, err)
		}
	}
	knownPath := filepath.Join(r.Client.ConfigRoot, "plugins", "known_marketplaces.json")
	if err := shared.WriteJSON(knownPath, map[string]any{a: map[string]any{"source": map[string]any{"source": "directory", "path": r.Plan.ActivePath + "-foreign"}, "installLocation": r.Plan.ActivePath + "-foreign"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := claudeProjectMarketplaceRegistered(r.Client.ConfigRoot, r.Plan.ActivePath, r.Plan.PhysicalArtifactID); err == nil {
		t.Fatal("adopted foreign marketplace")
	}
}

type claudeMarketplaceRunnerFunc func(context.Context, legacyports.Command) (legacyports.CommandResult, error)

func (f claudeMarketplaceRunnerFunc) Run(ctx context.Context, cmd legacyports.Command) (legacyports.CommandResult, error) {
	return f(ctx, cmd)
}

func TestClaudeProjectMarketplaceFailureEffectsAndReadOnlyVerification(t *testing.T) {
	for _, condition := range []string{"no-child", "unknown-child", "after-marketplace", "verify-only"} {
		t.Run(condition, func(t *testing.T) {
			request := newClaudeMarketplaceRequest(t)
			request.VerifyOnly = condition == "verify-only"
			mutations := 0
			runner := claudeMarketplaceRunnerFunc(func(_ context.Context, cmd legacyports.Command) (legacyports.CommandResult, error) {
				if reflect.DeepEqual(cmd.Argv[1:], []string{"plugin", "list", "--json"}) {
					return legacyports.CommandResult{Stdout: []byte("[]")}, nil
				}
				mutations++
				if condition == "verify-only" {
					t.Fatal("verify-only mutated native state")
				}
				if mutations == 1 && condition == "after-marketplace" {
					market := ProjectMarketplaceName(request.Plan.PhysicalArtifactID, request.Plan.TargetContext.ProjectRoot)
					err := shared.WriteJSON(filepath.Join(request.Client.ConfigRoot, "plugins", "known_marketplaces.json"), map[string]any{market: map[string]any{"source": map[string]any{"source": "directory", "path": request.Plan.ActivePath}, "installLocation": request.Plan.ActivePath}})
					return legacyports.CommandResult{}, err
				}
				return legacyports.CommandResult{NoChildStarted: condition != "unknown-child"}, fmt.Errorf("inert native failure")
			})
			out, err := New().Activate(t.Context(), clients.Env{Runner: runner}, request)
			want := domain.NativeEffectUnchanged
			if condition == "unknown-child" || condition == "after-marketplace" {
				want = domain.NativeEffectUncertain
			}
			if err == nil || out.NativeEffect != want || len(out.NativeObjects) != 0 {
				t.Fatalf("failure effect=%+v error=%v", out, err)
			}
			if condition == "verify-only" && !errors.Is(err, shared.ErrRecognizedNegativeEvidence) {
				t.Fatalf("missing native negative evidence: %v", err)
			}
			if _, err := os.Stat(filepath.Join(request.Plan.TargetContext.ProjectRoot, ".claude.json")); !os.IsNotExist(err) {
				t.Fatal("product manufactured project trust")
			}
		})
	}
}
