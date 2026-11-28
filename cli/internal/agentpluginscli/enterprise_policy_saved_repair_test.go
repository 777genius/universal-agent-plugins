package agentpluginscli

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func savedRepairPolicy(t *testing.T, j *savedJourney, body string) string {
	t.Helper()
	gate := sourcePolicyApp(t, body).InstallationPolicy
	j.cli.app = j.cli.app.withEnterprisePolicyGate(gate)
	return gate.(enterprisepolicy.Files).Paths[0].Path
}

func TestEnterprisePolicySavedRepairRequiresOriginalArchivePin(t *testing.T) {
	for _, pinned := range []bool{false, true} {
		t.Run(fmt.Sprintf("requested_checksum_%t", pinned), func(t *testing.T) {
			j, origin := removalArchiveJourney(t, map[string][]string{"demo": nil})
			source := origin + "/demo.zip"
			if pinned {
				loaded, err := j.cli.app.loadPackage(t.Context(), source)
				if err != nil {
					t.Fatal(err)
				}
				source += "#sha256=" + loaded.envelope.Source.ArchiveSHA256
				if err := loaded.cleanup(); err != nil {
					t.Fatal(err)
				}
			}
			j.run(t, "add", source, "--alias", "demo", "--target", "codex", "--format", "json")
			binding := savedJourneyBinding(t, j.state(t), "codex")
			drift := filepath.Join(binding.TargetLocator, "policy-repair-drift.txt")
			dependencyWrite(t, drift, "damaged managed projection\n")
			before, files := j.state(t), savedJourneyTree(t, binding.TargetLocator)
			savedRepairPolicy(t, &j, "schema_version: 2\nintegrity:\n  require_archive_sha256: true\n")
			_, _, err := j.cli.execute(false, "repair", "demo", "--target", "codex", "--yes", "--format", "json")
			if pinned {
				if err != nil {
					t.Fatalf("original checksum pin refused repair: %v", err)
				}
				if _, err := os.Stat(drift); !os.IsNotExist(err) {
					t.Fatalf("allowed repair did not restore managed bytes: %v", err)
				}
				return
			}
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) || !reflect.DeepEqual(before, j.state(t)) || !reflect.DeepEqual(files, savedJourneyTree(t, binding.TargetLocator)) {
				t.Fatalf("resolved checksum bypassed original pin requirement: %v", err)
			}
		})
	}
}

// This is a real acquired archive graph. Gemini's file projection avoids a
// native process dependency while exercising the production graph and SDK.
func savedRepairClosureJourney(t *testing.T) savedJourney {
	t.Helper()
	j, _ := savedRepairMutableClosureJourney(t)
	return j
}

func savedRepairMutableClosureJourney(t *testing.T) (savedJourney, func()) {
	t.Helper()
	var mu sync.RWMutex
	payloads := map[string][]byte{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.RLock()
		body, ok := payloads[r.URL.Path]
		mu.RUnlock()
		if ok {
			_, _ = w.Write(body)
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	for _, name := range []string{"a", "c"} {
		payloads["/"+name+".zip"] = savedRepairClosureArchive(t, name, server.URL, "denied.example", "1.0.0")
	}
	client := fixtureClient(t, domain.ClientGemini)
	client.ExecutablePath = filepath.Join(t.TempDir(), "gemini")
	fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	j := savedJourney{cli: fixture}
	j.run(t, "add", server.URL+"/a.zip", "--alias", "a", "--target", "gemini", "--format", "json")
	return j, func() {
		body := savedRepairClosureArchive(t, "c", server.URL, "allowed.example", "2.0.0")
		mu.Lock()
		defer mu.Unlock()
		payloads["/c.zip"] = body
	}
}

func savedRepairClosureArchive(t *testing.T, name, origin, host, version string) []byte {
	t.Helper()
	root := writeCLIPlugin(t)
	dependencyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(`{"$schema":%q,"name":%q,"version":%q,"description":"Repair closure fixture"}`, domain.PluginSchemaV1, name, version))
	skill := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(skill, 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(skill, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Repair fixture\n---\nInstructions.\n", name))
	if name == "a" {
		dependencyWrite(t, filepath.Join(root, "uap.toml"), fmt.Sprintf("schema_version = 1\n[dependencies.c]\narchive = %q\n", origin+"/c.zip"))
	} else {
		dependencyWrite(t, filepath.Join(root, "mcp.json"), fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"child":{"type":"streamable-http","url":%q}}}`, "https://"+host+"/mcp"))
	}
	return directArchiveBytes(t, root, "zip", "")
}

func TestEnterprisePolicySavedRepairChecksTransitiveClosure(t *testing.T) {
	for _, surface := range []string{"cli", "management", "after-review"} {
		t.Run(surface, func(t *testing.T) {
			j := savedRepairClosureJourney(t)
			policy := "schema_version: 2\nmcp:\n  deny_hosts: [denied.example]\n"
			if surface == "after-review" {
				policy = "schema_version: 2\nsources:\n  allowed_kinds: [archive]\n"
			}
			path := savedRepairPolicy(t, &j, policy)
			before, files := j.state(t), savedCheckFiles(t, j.cli.root)
			var err error
			if surface == "cli" {
				_, _, err = j.cli.execute(false, "repair", "a", "--target", "gemini", "--yes", "--format", "json")
			} else {
				backend := managementJourneyBackend(t, j)
				var prepared management.Prepared
				prepared, err = backend.Prepare(t.Context(), management.ChangeRequest{Action: management.Repair, Aliases: []string{"a"}, TargetIds: []string{"profile-gemini"}}, newSkillCatalogID())
				if surface == "after-review" {
					if err != nil {
						t.Fatalf("allowed repair review failed: %v", err)
					}
					defer prepared.Close()
					dependencyWrite(t, path, "schema_version: 2\nmcp:\n  deny_hosts: [denied.example]\n")
					_, err = prepared.Apply(t.Context(), nil)
				} else if prepared != nil {
					prepared.Close()
				}
			}
			var denied *domain.InstallPolicyError
			if !errors.As(err, &denied) && !errors.Is(err, domain.ErrInstallPolicyChanged) {
				t.Fatalf("repair omitted the denied required child: %v", err)
			}
			if !reflect.DeepEqual(before, j.state(t)) || !reflect.DeepEqual(files, savedCheckFiles(t, j.cli.root)) {
				t.Fatal("refused repair changed the root or required child")
			}
		})
	}
}

func TestEnterprisePolicySavedRepairAllowsVerifiedClosure(t *testing.T) {
	j := savedRepairClosureJourney(t)
	savedRepairPolicy(t, &j, "schema_version: 2\nsources:\n  allowed_kinds: [archive]\nmcp:\n  allow_hosts: [denied.example]\n")
	before := j.state(t)
	j.run(t, "repair", "a", "--target", "gemini", "--yes", "--format", "json")
	if len(j.state(t).Installations) != len(before.Installations) {
		t.Fatal("allowed repair changed closure membership")
	}
}

func TestEnterprisePolicySavedRepairRefusesDivergentInstalledChild(t *testing.T) {
	j, advanceChild := savedRepairMutableClosureJourney(t)
	_, initial := j.intent(t)
	oldChild, _ := initial.Package("c")
	if err := verifyRepairPolicyDescendants(t.Context(), j.cli.app, initial, "a", "codex"); !errors.Is(err, installer.ErrPlanChanged) {
		t.Fatalf("another target's installed child authorized repair without a selected-target receipt: %v", err)
	}
	// A direct declaration permits refreshing C alone while A still requires it.
	j.run(t, "add", oldChild.Source.Archive.URL, "--alias", "c", "--target", "gemini", "--format", "json")
	advanceChild()
	j.run(t, "deps", "update", "c", "--format", "json")
	_, locked := j.intent(t)
	newChild, _ := locked.Package("c")
	if oldChild.SourceKey != newChild.SourceKey || oldChild.Requirement().RequestedSelector() != newChild.Requirement().RequestedSelector() || oldChild.Revision() == newChild.Revision() || oldChild.TreeDigest == newChild.TreeDigest {
		t.Fatal("fixture did not advance only the desired child revision at the same moving source")
	}
	savedRepairPolicy(t, &j, "schema_version: 2\nmcp:\n  deny_hosts: [denied.example]\n")
	assertSavedRepairDivergenceRefused(t, j)
	// Convergence repairs the damaged root and installs the admitted child.
	// Its desired closure is distinct from a standalone recorded-root repair.
	drift := driftSavedRepairRoot(t, j)
	j.run(t, "install", "--locked", "--scope", "user", "--yes", "--format", "json")
	if _, err := os.Stat(drift); !os.IsNotExist(err) {
		t.Fatalf("convergence did not repair the recorded root: %v", err)
	}
	j.run(t, "repair", "a", "--target", "gemini", "--yes", "--format", "json")
}

func assertSavedRepairDivergenceRefused(t *testing.T, j savedJourney) {
	t.Helper()
	before, files := j.state(t), savedCheckFiles(t, j.cli.root)
	for _, surface := range []string{"cli", "management"} {
		var err error
		if surface == "cli" {
			_, _, err = j.cli.execute(false, "repair", "a", "--target", "gemini", "--yes", "--format", "json")
		} else {
			var prepared management.Prepared
			prepared, err = managementJourneyBackend(t, j).Prepare(t.Context(), management.ChangeRequest{Action: management.Repair, Aliases: []string{"a"}, TargetIds: []string{"profile-gemini"}}, newSkillCatalogID())
			if prepared != nil {
				prepared.Close()
			}
		}
		if !errors.Is(err, installer.ErrPlanChanged) || !strings.Contains(err.Error(), "install --locked") {
			t.Fatalf("%s repair accepted an uninstalled allowed child: %v", surface, err)
		}
		if !reflect.DeepEqual(before, j.state(t)) || !reflect.DeepEqual(files, savedCheckFiles(t, j.cli.root)) {
			t.Fatalf("%s refused repair changed the root or required child", surface)
		}
	}
}

func driftSavedRepairRoot(t *testing.T, j savedJourney) string {
	t.Helper()
	installation, err := savedAliasInstallation(j.state(t), "a", "a")
	if err != nil || installation == nil {
		t.Fatalf("missing recorded root: %v", err)
	}
	binding, err := selectedRepairBinding(*installation, domain.ClientGemini, j.cli.app.scope())
	if err != nil {
		t.Fatal(err)
	}
	drift := filepath.Join(binding.TargetLocator, "policy-repair-drift.txt")
	dependencyWrite(t, drift, "damaged recorded root\n")
	view, err := j.cli.app.Installer.Inspect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if drifted := driftedEnvironmentBindings(view); len(drifted) != 1 || len(drifted[installation.InstallationID]) != 1 {
		t.Fatal("fixture did not damage only the recorded root projection")
	}
	return drift
}
