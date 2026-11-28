package agentpluginscli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func invalidRemovalPolicy(t *testing.T, app App) App {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	dependencyWrite(t, path, "schema_version: 2\ncomponents:\n  unknown_rule: true\n")
	files := &enterprisepolicy.Files{Paths: []enterprisepolicy.Path{{Layer: "deployment", Path: path, Required: true}}}
	app.PolicyFiles = files
	return app.withEnterprisePolicyGate(files)
}

// Real HTTPS archives keep these saved-graph regressions independent of Git
// subprocess support. Native client responses use the existing inert fixture.
func removalArchiveJourney(t *testing.T, graph map[string][]string) (savedJourney, string) {
	t.Helper()
	payloads := map[string][]byte{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := payloads[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	for name, children := range graph {
		root := t.TempDir()
		skill := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(skill, 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(`{"$schema":%q,"name":%q,"version":"1.0.0","description":"Disposable policy removal package"}`, domain.PluginSchemaV1, name))
		dependencyWrite(t, filepath.Join(skill, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Policy removal fixture\n---\nInstructions for %s.\n", name, name))
		manifest := "schema_version = 1\n"
		for _, child := range children {
			manifest += fmt.Sprintf("[dependencies.%s]\narchive = %q\n", child, server.URL+"/"+child+".zip")
		}
		dependencyWrite(t, filepath.Join(root, "uap.toml"), manifest)
		payloads["/"+name+".zip"] = directArchiveBytes(t, root, "zip", "")
	}
	j := newSavedJourney(t, "")
	j.cli.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	return j, server.URL
}

func TestEnterprisePolicyRemovalKeepsUnselectedRootWithInvalidConfiguration(t *testing.T) {
	for _, sourceKind := range []string{"archive", "local-skill"} {
		for _, surface := range []string{"cli", "management"} {
			t.Run(sourceKind+"/"+surface, func(t *testing.T) {
				j, source := removalArchiveJourney(t, map[string][]string{"demo": nil, "keep": nil})
				alias := "demo"
				if sourceKind == "archive" {
					j.run(t, "add", source+"/demo.zip", "--alias", alias, "--target", "codex", "--format", "json")
				} else {
					sourceParent, err := filepath.EvalSymlinks(t.TempDir())
					if err != nil {
						t.Fatal(err)
					}
					root := filepath.Join(sourceParent, "review")
					if err := os.MkdirAll(root, 0700); err != nil {
						t.Fatal(err)
					}
					dependencyWrite(t, filepath.Join(root, "SKILL.md"), "---\nname: review\ndescription: Review local code\n---\nLocal instructions.\n")
					j.run(t, "skills", "add", root, "--target", "codex", "--yes", "--format", "json")
					alias = "review"
				}
				j.run(t, "add", source+"/keep.zip", "--alias", "keep", "--target", "claude", "--format", "json")
				retained := savedJourneyBinding(t, j.state(t), "claude")
				retainedFiles := savedJourneyTree(t, retained.TargetLocator)
				j.cli.app = invalidRemovalPolicy(t, j.cli.app)
				if surface == "cli" {
					j.run(t, "remove", alias, "--target", "codex", "--external-uninstalled", "--format", "json")
				} else {
					backend := managementJourneyBackend(t, j)
					managementApply(t, managementPrepare(t, backend, management.ChangeRequest{
						Action: management.Remove, Aliases: []string{alias}, TargetIds: []string{"profile-codex"},
					}))
				}
				requirements, lock := j.intent(t)
				if _, present := requirements.Dependencies[alias]; present || len(requirements.Dependencies) != 1 || len(lock.Packages) != 1 {
					t.Fatalf("removal did not reduce the saved agreement: %+v %+v", requirements, lock)
				}
				if got := savedJourneyBinding(t, j.state(t), "claude"); !reflect.DeepEqual(retained, got) || !reflect.DeepEqual(retainedFiles, savedJourneyTree(t, got.TargetLocator)) {
					t.Fatal("removal changed an unselected retained package")
				}
				for _, receipt := range j.state(t).EnvironmentReceipts {
					if receipt.Alias == alias {
						t.Fatal("removed binding retained its live environment receipt")
					}
				}
				before := j.state(t)
				if _, _, err := j.cli.execute(false, "install", "--locked", "--yes", "--format", "json"); err == nil || !reflect.DeepEqual(before, j.state(t)) {
					t.Fatalf("no-op installation inherited removal permission: %v", err)
				}
			})
		}
	}
}

func retainedPolicyRemovalFixture(t *testing.T) savedJourney {
	t.Helper()
	j, source := removalArchiveJourney(t, map[string][]string{"a": {"c"}, "c": nil})
	j.run(t, "add", source+"/a.zip", "--alias", "a", "--target", "codex", "--format", "json")
	j.run(t, "add", source+"/c.zip", "--alias", "c", "--target", "codex", "--format", "json")
	j.cli.app = invalidRemovalPolicy(t, j.cli.app)
	return j
}

func TestEnterprisePolicyRemovalOfDeclarationRetainsRequiredBindings(t *testing.T) {
	for _, surface := range []string{"cli", "management"} {
		t.Run(surface, func(t *testing.T) {
			j := retainedPolicyRemovalFixture(t)
			before := j.state(t)
			_, locked := j.intent(t)
			retained, ok := locked.Package("c")
			if !ok || len(retained.RequiredBy) != 1 {
				t.Fatal("fixture has no shared retained dependency", locked)
			}
			if surface == "cli" {
				j.run(t, "remove", "c", "--format", "json")
			} else {
				backend := managementJourneyBackend(t, j)
				prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Remove, Aliases: []string{"c"}})
				for _, action := range prepared.Review().Actions {
					if action.Action != "unchanged" {
						t.Fatal("declaration removal planned an installer mutation", prepared.Review())
					}
				}
				managementApply(t, prepared)
			}
			requirements, nextLock := j.intent(t)
			if len(requirements.Dependencies) != 1 {
				t.Fatalf("retained declaration was not removed: %+v", requirements)
			}
			if after := j.state(t); !reflect.DeepEqual(before, after) {
				t.Fatal("declaration-only removal touched installer state")
			}
			next, ok := nextLock.Package("c")
			if !ok || next.TreeDigest != retained.TreeDigest || !reflect.DeepEqual(next.RequiredBy, retained.RequiredBy) {
				t.Fatal("declaration removal changed required transitive pin")
			}
		})
	}
}
