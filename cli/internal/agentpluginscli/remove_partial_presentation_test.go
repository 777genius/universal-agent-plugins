package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// This composes the public command and existing activator under explicit manual
// uninstall confirmation. It does not establish a vendor-native uninstall.
type partialRemovalFailure struct {
	ports.ClientActivator
	cause     error
	armed     bool
	confirmed []domain.ClientID
}

func (a *partialRemovalFailure) Deactivate(ctx context.Context, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.Confirmed {
		a.confirmed = append(a.confirmed, request.Client.ClientID)
		if a.armed && request.Client.ClientID == domain.ClientCursor {
			return domain.DeactivationOutcome{NativeEffect: domain.NativeEffectUnchanged}, a.cause
		}
	}
	return a.ClientActivator.Deactivate(ctx, request)
}

func newPartialRemovalFixture(t *testing.T) (cliFixture, []string, func()) {
	t.Helper()
	f := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCodex), fixtureClient(t, domain.ClientCursor),
	})
	plugin := writeCLIPlugin(t)
	dependencyWrite(t, filepath.Join(plugin, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	if out, stderr, err := f.execute(false, "add", "--no-save", plugin, "--target", "codex,cursor"); err != nil {
		t.Fatalf("install: %v\n%s\n%s", err, out, stderr)
	}
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
		t.Fatalf("installed fixture: %+v, %v", state, err)
	}
	managed := make([]string, 0, len(state.Installations[0].Clients))
	for _, binding := range state.Installations[0].Clients {
		managed = append(managed, binding.TargetLocator)
	}
	foreign := filepath.Join(f.app.ManagedRoot, "foreign-removal-sentinel")
	if err := os.WriteFile(foreign, []byte("preserve unrelated managed-root bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	retained := make([]string, 1, 1+len(state.Installations[0].DataReceipts))
	retained[0] = foreign
	if len(state.Installations[0].DataReceipts) == 0 {
		t.Fatal("fixture has no plugin data to distinguish from managed packages")
	}
	for _, receipt := range state.Installations[0].DataReceipts {
		marker := filepath.Join(receipt.Locator, "partial-removal-marker")
		if err := os.WriteFile(marker, []byte("preserve plugin data\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		retained = append(retained, receipt.Locator)
	}
	before := map[string]map[string]dependencyCheckFileObservation{}
	for _, root := range retained {
		before[root] = observeDependencyCheckFiles(t, root)
	}
	return f, managed, func() {
		t.Helper()
		for root, snapshot := range before {
			assertDependencyCheckFilesUnchanged(t, root, snapshot)
		}
	}
}

func TestGroupedRemovalReportsExternalStageBeforeManagedCleanup(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			f, managed, retained := newPartialRemovalFixture(t)
			before := map[string]map[string]dependencyCheckFileObservation{}
			for _, root := range managed {
				before[root] = observeDependencyCheckFiles(t, root)
			}
			cause := &os.PathError{Op: "deactivate", Path: "private-removal-fixture", Err: errors.New("injected unchanged second client")}
			control := &partialRemovalFailure{ClientActivator: f.app.Lifecycle.Activator, cause: cause, armed: true}
			f.app.Lifecycle.Activator = control
			args := []string{"remove", "demo", "--target", "codex,cursor", "--external-uninstalled", "--yes", "--format", format}
			out, stderr, err := f.execute(false, args...)
			var typed *os.PathError
			if !errors.Is(err, cause) || !errors.As(err, &typed) || typed != cause {
				t.Fatalf("partial removal lost original typed cause: %v\n%s\n%s", err, out, stderr)
			}
			if len(control.confirmed) != 2 || control.confirmed[0] != domain.ClientCodex || control.confirmed[1] != domain.ClientCursor {
				t.Fatalf("did not reach the second confirmed deactivation: %v", control.confirmed)
			}
			for root, snapshot := range before {
				assertDependencyCheckFilesUnchanged(t, root, snapshot)
			}
			retained()
			state, loadErr := f.store.Load()
			if loadErr != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 {
				t.Fatalf("partial removal lost managed bindings: %+v, %v", state, loadErr)
			}
			for _, binding := range state.Installations[0].Clients {
				if binding.NativeActivationAttempt != "" {
					t.Fatal("positively unchanged failure retained an unresolved native attempt")
				}
			}
			if format == "human" {
				if !strings.Contains(out, "  codex: removed from the client; managed cleanup is incomplete\n") || strings.Contains(out, "  codex: removed\n") {
					t.Errorf("partial removal hid the retained managed stage:\n%s", out)
				}
				if !strings.Contains(out, "  cursor: failed\n") {
					t.Errorf("partial removal hid the failed client:\n%s", out)
				}
			} else {
				var envelope struct {
					SchemaVersion int               `json:"schema_version"`
					Command       string            `json:"command"`
					Result        string            `json:"result"`
					Data          removeMultiResult `json:"data"`
				}
				decoder := json.NewDecoder(strings.NewReader(out))
				if err := decoder.Decode(&envelope); err != nil {
					t.Fatalf("partial JSON: %v\n%s", err, out)
				}
				var extra any
				if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
					t.Fatalf("partial removal emitted another JSON value: %v", err)
				}
				data := envelope.Data
				if envelope.SchemaVersion != outputSchemaVersion || envelope.Command != "remove" || envelope.Result != "failure" ||
					!data.Batch || data.Status != string(usecase.GroupPhaseExternalPartialFailure) || data.Succeeded != 1 || data.Failed != 1 ||
					len(data.Targets) != 2 || data.Targets[0].Target != "codex" || data.Targets[0].Status != string(usecase.GroupTargetExternalCompleted) ||
					data.Targets[1].Target != "cursor" || data.Targets[1].Status != string(usecase.GroupTargetExternalFailed) || !data.PluginDataPreserved || data.DataRetained {
					t.Fatalf("partial removal changed the JSON stage/count contract: %+v", envelope)
				}
				if strings.Contains(out, f.root) || strings.Contains(out, cause.Path) {
					t.Fatal("partial JSON exposed private paths")
				}
			}
			// Remove only the fault; no receipt clearing, state edits or recovery waiver.
			control.armed = false
			out, stderr, err = f.execute(false, args...)
			if err != nil || len(control.confirmed) != 4 {
				t.Fatalf("authorized public retry: %v, calls=%v\n%s\n%s", err, control.confirmed, out, stderr)
			}
			for _, root := range managed {
				if _, err := os.Stat(root); !os.IsNotExist(err) {
					t.Fatalf("successful retry retained managed package %s: %v", root, err)
				}
			}
			retained()
			if format == "json" {
				assertVersionedJSON(t, out, "remove")
				assertBatchJSON(t, out, "remove", 2, 0)
			}
		})
	}
}

func TestGroupedRemovalSuccessSeparatesRetainedPluginData(t *testing.T) {
	f, managed, retained := newPartialRemovalFixture(t)
	out, stderr, err := f.execute(false, "remove", "demo", "--target", "codex,cursor", "--external-uninstalled", "--yes")
	if err != nil {
		t.Fatalf("remove: %v\n%s\n%s", err, out, stderr)
	}
	for _, want := range []string{"Remove demo: removed\n", "  codex: removed\n", "  cursor: removed\n", "PLUGIN_DATA: preserved", "--purge-data"} {
		if !strings.Contains(out, want) {
			t.Errorf("successful removal omitted %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "managed cleanup is incomplete") || strings.Contains(out, removalIncompleteNotice) {
		t.Errorf("retained plugin data was confused with incomplete managed cleanup:\n%s", out)
	}
	for _, root := range managed {
		if _, err := os.Stat(root); !os.IsNotExist(err) {
			t.Fatalf("successful removal retained managed package: %s, %v", root, err)
		}
	}
	retained()
	state, err := f.store.Load()
	if err != nil || len(state.Installations) != 1 || !state.Installations[0].DataRetained || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("successful removal lost retained-data ownership: %+v, %v", state, err)
	}
}

// The public committed-interruption oracle remains in claude_interrupted_remove_test.go.
// These boundaries ensure contextual wording changes only external_completed.
func TestGroupedRemovalHumanStageBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, group, target, want string }{
		{"committed finalization", "managed_committed_finalization_failed", string(usecase.GroupTargetManagedCommitted), "cursor: managed committed\n"},
		{"unknown cleanup", string(usecase.GroupPhaseManagedCommitUnknown), string(usecase.GroupTargetExternalPartial), "cursor: removed from the client; managed cleanup is incomplete\n"},
		{"blocked", "manual_step_required", "blocked", "cursor: blocked\n"},
		{"not applied", "manual_step_required", "not_applied", "cursor: not applied\n"},
		{"not attempted", string(usecase.GroupPhaseExternalPartialFailure), string(usecase.GroupTargetExternalNotAttempted), "cursor: external not attempted\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{}
			cmd.SetOut(&out)
			result := removeMultiResult{Plugin: "demo", Status: tc.group, Failed: 1, Targets: []removeTargetResult{{Target: "cursor", Status: tc.target}}}
			if err := renderRemoveMultiResult(cmd, &options{format: "human"}, result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "cursor: removed\n") || strings.Contains(out.String(), "files remain") {
				t.Fatalf("unrelated removal stage was reclassified:\n%s", out.String())
			}
		})
	}
}
