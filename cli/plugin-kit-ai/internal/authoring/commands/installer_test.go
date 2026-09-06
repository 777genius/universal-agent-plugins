package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

// This is explicitly an installer fixture, not a compatibility service. The
// existing add command constructs its real planner. Detection and scanner are
// no-effect test inputs; no real user profile, runtime or provider is executed.
func TestGeneratedPackagesReachExistingInstallerPlanner(t *testing.T) {
	if runtime.GOOS != "linux" && !(runtime.GOOS == "windows" && (runtime.GOARCH == "amd64" || runtime.GOARCH == "arm64")) {
		t.Skip("writable native authoring requires Linux or Windows amd64/arm64")
	}
	author := commands.App{Projects: project.Service{Scratch: t.TempDir()}, Revision: baseline}
	registry, e := specregistry.New()
	if e != nil {
		t.Fatal(e)
	}
	packageLoader := loader.Loader{Registry: registry}
	for _, tc := range []struct {
		lane       string
		flags      []string
		components int
	}{
		{"skill", nil, 1}, {"mcp-remote", []string{"--url=https://example.invalid/mcp"}, 1},
		{"mcp-stdio", []string{"--runtime=node"}, 1}, {"hybrid", []string{"--mcp=mcp-stdio", "--runtime=node"}, 2},
	} {
		t.Run(tc.lane, func(t *testing.T) {
			fixture, source := t.TempDir(), filepath.Join(t.TempDir(), "demo")
			args := append([]string{"init", source, "--template=" + tc.lane, "--name=demo", "--description=A disposable fixture.", "--format=json"}, tc.flags...)
			_, code, out := execute(t, author, args, false)
			if code != 0 {
				t.Fatalf("generate: %s", out)
			}
			clients := []domain.DetectedClient{}
			for _, id := range []domain.ClientID{domain.ClientCursor, domain.ClientCodex, domain.ClientClaude} {
				root := filepath.Join(fixture, string(id))
				if e := os.Mkdir(root, 0700); e != nil {
					t.Fatal(e)
				}
				clients = append(clients, domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: root, ExecutablePath: filepath.Join(root, "inert-fixture-cli")})
			}
			detector := &fixtureDetector{clients: clients}
			scanner := &fixtureScanner{t: t}
			app := agentpluginscli.App{UserHome: fixture, ManagedRoot: filepath.Join(fixture, "managed"), Detector: detector,
				StateStore: noEffectState{}, SourceAcquirer: sourceacquisition.Acquirer{TempRoot: t.TempDir()}, PackageLoader: packageLoader, NativePackageLoader: loader.OpenAILoader{Loader: packageLoader}, SecurityEvaluator: scanner,
				Lifecycle: usecase.Service{Stager: noEffectStager{}, Activator: noEffectActivator{}},
			}
			before, sourceBefore := tree(t, fixture), tree(t, source)
			for _, client := range clients {
				var out, errout bytes.Buffer
				err := authoringcli.Factory(func() (*cobra.Command, error) { return agentpluginscli.NewRoot(app), nil }).Execute(context.Background(), []string{"add", source, "--target=" + string(client.ClientID), "--dry-run", "--format=json"}, authoringcli.Streams{Out: &out, Err: &errout})
				if err != nil {
					t.Fatalf("existing installer planner %s: %v\n%s\n%s", client.ClientID, err, out.Bytes(), errout.Bytes())
				}
				var result map[string]any
				if e := json.Unmarshal(out.Bytes(), &result); e != nil {
					t.Fatalf("installer JSON: %v\n%s", e, out.Bytes())
				}
				if !bytes.Contains(out.Bytes(), []byte(`"dry_run": true`)) && !bytes.Contains(out.Bytes(), []byte(`"dry_run":true`)) {
					t.Fatalf("missing dry-run evidence: %s", out.Bytes())
				}
				// Structural plan evidence, not merely a successful command or fake client.
				plans := planObjects(result)
				if len(plans) == 0 {
					t.Fatalf("no existing planner results: %s", out.Bytes())
				}
				matched := false
				for _, plan := range plans {
					if plan["client_id"] == string(client.ClientID) {
						components, ok := plan["components"].([]any)
						if !ok || len(components) != tc.components {
							t.Fatalf("component plan mismatch: %+v", plan)
						}
						if plan["status"] == "unsupported" {
							t.Fatalf("fixture planner unsupported: %+v", plan)
						}
						matched = true
						t.Logf("installer fixture: template=%s target=%s status=%v components=%d boundary=existing-add-dry-run-injected-detector-scanner", tc.lane, client.ClientID, plan["status"], len(components))
					}
				}
				if !matched {
					t.Fatalf("wrong target plan: %s", out.Bytes())
				}
			}
			if detector.calls != len(clients) || scanner.calls != len(clients) {
				t.Fatalf("seams not reached: detect=%d scan=%d", detector.calls, scanner.calls)
			}
			if !reflect.DeepEqual(before, tree(t, fixture)) || !reflect.DeepEqual(sourceBefore, tree(t, source)) {
				t.Fatal("dry-run fixture or source mutated")
			}
		})
	}
}
func planObjects(value any) []map[string]any {
	var out []map[string]any
	switch v := value.(type) {
	case map[string]any:
		if _, ok := v["components"]; ok {
			if _, ok := v["client_id"]; ok {
				out = append(out, v)
			}
		}
		for _, x := range v {
			out = append(out, planObjects(x)...)
		}
	case []any:
		for _, x := range v {
			out = append(out, planObjects(x)...)
		}
	}
	return out
}

type fixtureDetector struct {
	clients []domain.DetectedClient
	calls   int
}

func (d *fixtureDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	d.calls++
	return append([]domain.DetectedClient{}, d.clients...), nil
}

type fixtureScanner struct {
	t     *testing.T
	calls int
}

func (s *fixtureScanner) Evaluate(_ context.Context, i domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	s.calls++
	if i.SnapshotRoot == "" || i.TreeDigest == "" || i.ManifestDigest == "" {
		s.t.Fatal("scanner did not receive real acquired package")
	}
	if _, e := os.Stat(filepath.Join(i.SnapshotRoot, "plugin.json")); e != nil {
		s.t.Fatal(e)
	}
	return domain.SecurityAssessment{SchemaVersion: 1, Subject: domain.SecuritySubject{TreeDigest: i.TreeDigest, ManifestDigest: i.ManifestDigest}, Outcome: domain.SecurityNoBlockingFindings}, nil
}

type noEffectState struct{}

func (noEffectState) Load() (domain.StateFileV2, error) {
	return domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}, nil
}
func (noEffectState) Save(domain.StateFileV2) error { panic("dry-run attempted state mutation") }

type noEffectStager struct{}

func (noEffectStager) Stage(context.Context, domain.PackageEnvelope, domain.DeliveryPlan, string, domain.CompatibilityHints) (domain.StagedDelivery, error) {
	panic("dry-run attempted staging")
}
func (noEffectStager) Discard(context.Context, domain.StagedDelivery) error {
	panic("dry-run attempted discard")
}
func (noEffectStager) Verify(context.Context, string, string) error {
	return errors.New("unexpected verification")
}

type noEffectActivator struct{}

func (noEffectActivator) Activate(context.Context, domain.ActivationRequest) (domain.ActivationOutcome, error) {
	panic("dry-run attempted activation")
}
func (noEffectActivator) Deactivate(context.Context, domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	panic("dry-run attempted deactivation")
}
