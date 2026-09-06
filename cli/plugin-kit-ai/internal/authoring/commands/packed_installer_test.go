package commands_test

// Shared planner fixtures remain in ordinary native coverage.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
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

type packedProject struct{ Product, Lane, Source string }

func within(root, p string) bool {
	return p == root || strings.HasPrefix(p, root+string(filepath.Separator))
}

type packedEntry struct {
	Mode   fs.FileMode
	Size   int64
	Digest string
}

func packedTree(t *testing.T, root string) map[string]packedEntry {
	t.Helper()
	result := map[string]packedEntry{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		if !st.IsDir() && !st.Mode().IsRegular() {
			return fmt.Errorf("unsafe fixture object: %s", p)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		e := packedEntry{Mode: st.Mode()}
		if !st.IsDir() {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(b)
			e.Size = int64(len(b))
			e.Digest = hex.EncodeToString(sum[:])
		}
		result[rel] = e
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// Wrap the existing no-effect scanner and prove the acquired bytes came from
// this exact source. Acquisition intentionally normalizes modes; preservation
// separately compares every original directory/file mode and entry.
type packedScanner struct {
	fixtureScanner
	source, scratch string
	tree            map[string]packedEntry
}

func (s *packedScanner) Evaluate(ctx context.Context, in domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	if in.SnapshotRoot == s.source || !within(s.scratch, in.SnapshotRoot) {
		s.t.Fatal("scanner bypassed disposable acquisition")
	}
	got := packedTree(s.t, in.SnapshotRoot)
	if len(got) != len(s.tree) {
		s.t.Fatalf("acquisition entries differ: %d != %d", len(got), len(s.tree))
	}
	for p, want := range s.tree {
		e, ok := got[p]
		if !ok || e.Mode.IsDir() != want.Mode.IsDir() || e.Size != want.Size || e.Digest != want.Digest {
			s.t.Fatalf("acquired source mismatch: %s", p)
		}
	}
	return s.fixtureScanner.Evaluate(ctx, in)
}

func packedPlanner(t *testing.T, p packedProject) []map[string]any {
	t.Helper()
	fixture, scratch := t.TempDir(), t.TempDir()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	packageLoader := loader.Loader{Registry: registry}
	envelope, err := packageLoader.Load(context.Background(), domain.LoadInput{SnapshotRoot: p.Source})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{}
	for name := range envelope.Skills {
		want = append(want, "skill:"+name)
	}
	for name := range envelope.MCP.Servers {
		want = append(want, "mcp_server:"+name)
	}
	sort.Strings(want)
	// Native journey adds extra-skill to every template after initial generation.
	skills, servers := 1, 1
	if p.Lane == "skill" || strings.HasPrefix(p.Lane, "hybrid-") {
		skills = 2
	}
	if p.Lane == "skill" {
		servers = 0
	}
	if len(envelope.Skills) != skills || len(envelope.MCP.Servers) != servers {
		t.Fatalf("wrong completed lane inventory: skills=%d servers=%d", len(envelope.Skills), len(envelope.MCP.Servers))
	}
	if _, ok := envelope.Skills["extra-skill"]; !ok {
		t.Fatal("missing native journey extra-skill")
	}
	for _, server := range envelope.MCP.Servers {
		transport := "stdio"
		if strings.HasSuffix(p.Lane, "remote") {
			transport = "streamable-http"
		}
		if server.Type != transport {
			t.Fatalf("wrong lane transport: %s", server.Type)
		}
	}
	clients := []domain.DetectedClient{}
	for _, id := range []domain.ClientID{domain.ClientCursor, domain.ClientCodex, domain.ClientClaude} {
		root := filepath.Join(fixture, string(id))
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		clients = append(clients, domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: root, ExecutablePath: filepath.Join(root, "inert-fixture-cli")})
	}
	detector := &fixtureDetector{clients: clients}
	sourceBefore, fixtureBefore, scratchBefore := packedTree(t, p.Source), packedTree(t, fixture), packedTree(t, scratch)
	scanner := &packedScanner{fixtureScanner: fixtureScanner{t: t}, source: p.Source, scratch: scratch, tree: sourceBefore}
	app := agentpluginscli.App{UserHome: fixture, ManagedRoot: filepath.Join(fixture, "managed"), Detector: detector, StateStore: noEffectState{}, SourceAcquirer: sourceacquisition.Acquirer{TempRoot: scratch}, PackageLoader: packageLoader, NativePackageLoader: loader.OpenAILoader{Loader: packageLoader}, SecurityEvaluator: scanner, Lifecycle: usecase.Service{Stager: noEffectStager{}, Activator: noEffectActivator{}}}
	var results []map[string]any
	for _, client := range clients {
		var out, stderr bytes.Buffer
		err := authoringcli.Factory(func() (*cobra.Command, error) { return agentpluginscli.NewRoot(app), nil }).Execute(context.Background(), []string{"add", p.Source, "--target=" + string(client.ClientID), "--dry-run", "--format=json"}, authoringcli.Streams{Out: &out, Err: &stderr})
		if err != nil || stderr.Len() != 0 {
			t.Fatalf("planner: %v\n%s\n%s", err, out.String(), stderr.String())
		}
		var report struct {
			Result string `json:"result"`
			Data   struct {
				DryRun bool `json:"dry_run"`
			} `json:"data"`
		}
		if err := json.Unmarshal(out.Bytes(), &report); err != nil || !report.Data.DryRun || report.Result != "success" {
			t.Fatalf("not successful structured dry-run: %v\n%s", err, out.String())
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		plans := planObjects(result)
		if len(plans) == 0 {
			t.Fatal("missing structured plan")
		}
		wantStatus := string(domain.PlanManualActivationRequired)
		if client.ClientID == domain.ClientClaude {
			wantStatus = string(domain.PlanReady)
		}
		for _, plan := range plans {
			if plan["client_id"] != string(client.ClientID) || plan["status"] != wantStatus || plan["scope"] != "user" {
				t.Fatalf("wrong target/status: %+v", plan)
			}
			b, _ := json.Marshal(plan["components"])
			var components []domain.ComponentDecision
			if err := json.Unmarshal(b, &components); err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, v := range components {
				if v.Support == "" || v.Support == domain.SupportUnsupported {
					t.Fatalf("unsupported component: %+v", v)
				}
				got = append(got, string(v.Kind)+":"+v.Name)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("wrong components: got %v want %v", got, want)
			}
		}
		results = append(results, map[string]any{"product": p.Product, "lane": p.Lane, "target": client.ClientID, "report": result})
		if !reflect.DeepEqual(sourceBefore, packedTree(t, p.Source)) || !reflect.DeepEqual(fixtureBefore, packedTree(t, fixture)) || !reflect.DeepEqual(scratchBefore, packedTree(t, scratch)) {
			t.Fatal("dry-run mutated source/client fixture or leaked acquisition entries")
		}
	}
	if detector.calls != 3 || scanner.calls != 3 {
		t.Fatalf("seams not reached: %d/%d", detector.calls, scanner.calls)
	}
	return results
}

func TestPackedInstallerSourceHarness(t *testing.T) {
	if (runtime.GOOS != "linux" && runtime.GOOS != "windows") || (runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64") {
		t.Skip("source harness requires supported Linux/Windows amd64/arm64 host")
	}
	author := commands.App{Projects: project.Service{Scratch: t.TempDir()}, Revision: publicRevision, PublicContract: true}
	for _, lane := range []string{"skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"} {
		t.Run(lane, func(t *testing.T) {
			source := filepath.Join(t.TempDir(), lane)
			template := lane
			if strings.HasPrefix(lane, "hybrid-") {
				template = "hybrid"
			}
			args := []string{"init", source, "--name=demo", "--description=Disposable fixture.", "--template=" + template, "--format=json"}
			if strings.HasSuffix(lane, "remote") {
				args = append(args, "--url=https://docs.example.com/mcp")
			}
			if strings.HasSuffix(lane, "stdio") {
				args = append(args, "--runtime=node")
			}
			if template == "hybrid" {
				args = append(args, "--mcp-template=mcp-"+strings.TrimPrefix(lane, "hybrid-"))
			}
			_, code, out := publicRun(t, author, args, false)
			if code != 0 {
				t.Fatalf("SOURCE HARNESS generation: %s", out)
			}
			_, code, out = publicRun(t, author, []string{"skills", "init", "extra-skill", source, "--description=Disposable fixture.", "--format=json"}, false)
			if code != 0 {
				t.Fatalf("SOURCE HARNESS extra skill: %s", out)
			}
			packedPlanner(t, packedProject{Product: "SOURCE HARNESS ONLY", Lane: lane, Source: source})
		})
	}
}
