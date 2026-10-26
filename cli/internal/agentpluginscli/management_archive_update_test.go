package agentpluginscli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

const managementArchiveName = "uap-desktop-plugin-matrix"
const managementArchiveSkill = "uap-desktop-matrix-guide"

func managementArchiveSkillBody(revision int) string {
	return fmt.Sprintf("---\nname: %s\ndescription: Inert desktop plugin lifecycle fixture\n---\nDesktop plugin matrix revision %d.\n", managementArchiveSkill, revision)
}

func managementArchiveBytes(t *testing.T, revision int) []byte {
	t.Helper()
	var result bytes.Buffer
	compressed := gzip.NewWriter(&result)
	tw := tar.NewWriter(compressed)
	for _, file := range []struct{ name, body string }{
		{"plugin.json", fmt.Sprintf(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":%q,"version":"%d.0.0","description":"Disposable multi-client archive lifecycle"}`, managementArchiveName, revision)},
		{"skills/" + managementArchiveSkill + "/SKILL.md", managementArchiveSkillBody(revision)},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: file.name, Mode: 0600, Size: int64(len(file.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(file.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

// Real HTTPS acquisition, graph, installer, receipts and file projection;
// native client/scanner responses retain the existing controlled test seams.
func TestManagementMutableArchiveUpdateRetainsAllTargets(t *testing.T) {
	for _, withOpenCode := range []bool{false, true} {
		name := "claude_codex_control"
		if withOpenCode {
			name = "claude_codex_opencode"
		}
		t.Run(name, func(t *testing.T) {
			var mu sync.Mutex
			current := managementArchiveBytes(t, 1)
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				_, _ = w.Write(current)
			}))
			defer server.Close()
			ids := []domain.ClientID{domain.ClientClaude, domain.ClientCodex}
			if withOpenCode {
				ids = append(ids, domain.ClientOpenCode)
			}
			detected := make([]domain.DetectedClient, 0, len(ids))
			targets := make([]string, 0, len(ids))
			for _, id := range ids {
				client := fixtureClient(t, id)
				client.ExecutablePath = filepath.Join(t.TempDir(), string(id))
				detected = append(detected, client)
				targets = append(targets, "profile-"+string(id))
			}
			probes := 0
			fixture, _ := newDependencyHarness(t, t.TempDir(), detected, installer.Config{
				Runner: &dependencyClaudeListing{root: detected[0].ConfigRoot}, EnableNativeObserver: true,
				OpenCodeProbe: func(_ context.Context, target clientdetect.ProbeTarget) (clientdetect.ProbeEvidence, error) {
					probes++
					if !withOpenCode || target.Executable != detected[2].ExecutablePath {
						t.Fatal("unselected host was probed", target.Executable)
					}
					return clientdetect.ProbeEvidence{VersionEvidence: opencodehost.VersionEvidence{Version: "1.18.34", Source: "executable_version", ProbeStatus: "ok", ExecutableIdentity: "fixture:" + target.Executable}}, nil
				},
			})
			fixture.app.SourceAcquirer = sourceacquisition.Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			journey := savedJourney{cli: fixture}
			backend := managementJourneyBackend(t, journey)
			managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install,
				Source: server.URL + "/fixture.tar.gz", Alias: managementArchiveName, TargetIds: targets}))
			beforeState := journey.state(t)
			_, beforeLock := journey.intent(t)
			manifest, lockPath := fixture.app.dependencyPaths()
			beforeManifest, beforeLockBytes, beforeStateBytes := dependencyRead(t, manifest), dependencyRead(t, lockPath), dependencyRead(t, fixture.store.Path)
			mu.Lock()
			current = managementArchiveBytes(t, 2)
			mu.Unlock()
			prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Update, Aliases: []string{managementArchiveName}})
			if beforeManifest != dependencyRead(t, manifest) || beforeLockBytes != dependencyRead(t, lockPath) || beforeStateBytes != dependencyRead(t, fixture.store.Path) {
				t.Fatal("update preview changed authoritative state or intent")
			}
			review := prepared.Review()
			if len(review.Actions) != len(targets) {
				t.Fatal("review lost targets", review.Actions)
			}
			for index, action := range review.Actions {
				if action.Action != "update" || action.Alias != managementArchiveName || action.TargetId != targets[index] || action.ToVersion != "2.0.0" {
					t.Fatal("review changed exact target/revision", action)
				}
			}
			managementApply(t, prepared)
			requirements, locked := journey.intent(t)
			state := journey.state(t)
			if len(state.Installations) != 1 || state.Installations[0].InstallationID != beforeState.Installations[0].InstallationID || len(state.EnvironmentReceipts) != len(targets) || len(state.Installations[0].Clients) != len(targets) {
				t.Fatal("update lost shared installation/owned target identities", state)
			}
			if requirements.Dependencies[managementArchiveName].Archive.SHA256 != "" || locked.Packages[0].Version != "2.0.0" || locked.Packages[0].Source.Archive.SHA256 == beforeLock.Packages[0].Source.Archive.SHA256 {
				t.Fatal("mutable archive did not resolve and retain its new exact version", locked)
			}
			for id, binding := range state.Installations[0].Clients {
				if _, ok := beforeState.Installations[0].Clients[id]; !ok {
					t.Fatal("update changed a binding identity", id)
				}
				root := binding.TargetLocator
				if binding.ClientID == "claude" {
					root = filepath.Join(root, ".agentplugins-runtime")
				}
				if binding.ClientID == "opencode" {
					root = detected[2].ConfigRoot
				}
				if body := dependencyRead(t, filepath.Join(root, "skills", managementArchiveSkill, "SKILL.md")); body != managementArchiveSkillBody(2) {
					t.Fatal("target retained old delivered Skill bytes", binding.ClientID, body)
				}
			}
			if !withOpenCode && probes != 0 || withOpenCode && probes == 0 {
				t.Fatal("host preparation did not follow the actual target set", probes)
			}
		})
	}
}
