package agentpluginscli

import (
	"errors"
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
	payloads := map[string][]byte{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if body, ok := payloads[r.URL.Path]; ok {
			_, _ = w.Write(body)
		} else {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	for _, name := range []string{"a", "c"} {
		root := writeCLIPlugin(t)
		dependencyWrite(t, filepath.Join(root, "plugin.json"), fmt.Sprintf(`{"$schema":%q,"name":%q,"version":"1.0.0","description":"Repair closure fixture"}`, domain.PluginSchemaV1, name))
		skill := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(skill, 0700); err != nil {
			t.Fatal(err)
		}
		dependencyWrite(t, filepath.Join(skill, "SKILL.md"), fmt.Sprintf("---\nname: %s\ndescription: Repair fixture\n---\nInstructions.\n", name))
		if name == "a" {
			dependencyWrite(t, filepath.Join(root, "uap.toml"), fmt.Sprintf("schema_version = 1\n[dependencies.c]\narchive = %q\n", server.URL+"/c.zip"))
		} else {
			dependencyWrite(t, filepath.Join(root, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"child":{"type":"streamable-http","url":"https://denied.example/mcp"}}}`)
		}
		payloads["/"+name+".zip"] = directArchiveBytes(t, root, "zip", "")
	}
	client := fixtureClient(t, domain.ClientGemini)
	client.ExecutablePath = filepath.Join(t.TempDir(), "gemini")
	fixture, _ := newDependencyHarness(t, "", []domain.DetectedClient{client}, installer.Config{})
	fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	j := savedJourney{cli: fixture}
	j.run(t, "add", server.URL+"/a.zip", "--alias", "a", "--target", "gemini", "--format", "json")
	return j
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
