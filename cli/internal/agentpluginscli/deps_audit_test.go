package agentpluginscli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/dependencymanifest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type dependencyFixtureSource struct {
	SourceAcquirer
	root string
}

func (s dependencyFixtureSource) AcquireGitHub(ctx context.Context, _, _, _ string) (domain.PackageSnapshot, error) {
	return (packagedigest.Builder{}).Snapshot(ctx, s.root, domain.SourceIdentity{})
}
func (s dependencyFixtureSource) AcquireGitHubVerified(ctx context.Context, repo, rev, path, _ string) (domain.PackageSnapshot, error) {
	return s.AcquireGitHub(ctx, repo, rev, path)
}

type dependencyForbiddenLoader struct{ called *bool }

func (l dependencyForbiddenLoader) Load(context.Context, domain.LoadInput) (domain.PackageEnvelope, error) {
	*l.called = true
	return domain.PackageEnvelope{}, errors.New("native loader must not run")
}

func TestDependencyNonstandardBytesNeverReachStore(t *testing.T) {
	for _, mode := range []string{"git", "rematerialize", "registry"} {
		t.Run(mode, func(t *testing.T) {
			f := newCLIFixture(t, nil)
			root := t.TempDir()
			if err := os.Mkdir(filepath.Join(root, ".claude-plugin"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "plugin.json"), []byte(`{"name":"native-only"}`), 0600); err != nil {
				t.Fatal(err)
			}
			f.app.EnvironmentStoreRoot = filepath.Join(t.TempDir(), "store")
			f.app.SourceAcquirer = dependencyFixtureSource{root: root}
			called := false
			f.app.NativePackageLoader = dependencyForbiddenLoader{called: &called}
			requirement := environment.Requirement{Alias: "demo", Git: &environment.GitSource{Repository: "acme/demo", Selector: environment.Selector{Kind: environment.SelectorCommit, Value: strings.Repeat("1", 40)}}}
			var err error
			switch mode {
			case "git":
				_, err = (dependencyResolver{app: f.app}).resolveGit(context.Background(), requirement, []string{"codex"}, nil, false)
			case "rematerialize":
				_, err = materializeLockedPackage(context.Background(), f.app, environment.LockedPackage{Alias: "demo", DigestAlgorithm: domain.TreeDigestAlgorithm, TreeDigest: "sha256:" + strings.Repeat("a", 64), Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: "acme/demo", Commit: strings.Repeat("1", 40)}}}, true, nil)
			case "registry":
				fixture := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCodex}, []domain.ClientID{domain.ClientCodex})
				f.app.DirectoryClient = fixture.cli.app.DirectoryClient
				_, err = f.app.acquireDirectory(context.Background(), "rollout-demo", packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, Targets: []domain.ClientID{domain.ClientCodex}, DesiredReleaseSequence: 1})
			}
			if err == nil || !strings.Contains(err.Error(), "root standard plugin.json") {
				t.Fatalf("nonstandard %s: %v", mode, err)
			}
			if called {
				t.Fatal("dependency reached native fallback")
			}
			if _, err := os.Stat(f.app.EnvironmentStoreRoot); !os.IsNotExist(err) {
				t.Fatal("nonstandard dependency wrote store")
			}
		})
	}
}

func TestDependencySyncShowsPlanBeforeDeclinedConfirmation(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetIn(strings.NewReader("n\n"))
	actions := []dependencyAction{{Alias: "demo", ClientID: "codex", Action: environment.ActionRemove}}
	err := applyDependencySync(cmd, App{Terminal: true}, &options{format: "human"}, dependencySyncRequest{prepared: actions}, false)
	if err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "remove demo/codex") || !strings.Contains(text, "No changes made.") || strings.Index(text, "remove demo/codex") > strings.Index(text, "Apply the preflighted") {
		t.Fatal(text)
	}
}

// A retired JSON declaration must fail visibly even when an equivalent TOML
// exists; local imperative installation must never silently install a leaf.
func TestDependencyLegacyJSONRefusedBeforeImperativeApply(t *testing.T) {
	for _, withTOML := range []bool{false, true} {
		t.Run(map[bool]string{false: "JSON only", true: "JSON and TOML"}[withTOML], func(t *testing.T) {
			f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
			root := writeCLIPlugin(t)
			var manifest map[string]any
			if err := json.Unmarshal([]byte(dependencyRead(t, filepath.Join(root, "plugin.json"))), &manifest); err != nil {
				t.Fatal(err)
			}
			manifest["extensions"] = map[string]any{domain.ExtensionNamespace: map[string]any{"dependencies": map[string]string{"acme/context": "^2.0"}}}
			body, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			dependencyWrite(t, filepath.Join(root, "plugin.json"), string(body))
			if withTOML {
				dependencyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n[dependencies]\n\"acme/context\" = '^2.0'\n")
			}
			before, err := f.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			_, _, err = f.execute(false, "add", "--no-save", root, "--target", "cursor")
			if !errors.Is(err, dependencymanifest.ErrDependencyDeclarationMoved) || !strings.Contains(err.Error(), "uap.toml") {
				t.Fatalf("legacy declaration refusal: %v", err)
			}
			after, err := f.store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("refusal changed state: %v", err)
			}
		})
	}
}

func TestDependencyTOMLRefusedByImperativeInstall(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	root := writeCLIPlugin(t)
	dependencyWrite(t, filepath.Join(root, "uap.toml"), "schema_version = 1\n[dependencies]\n\"acme/context\" = '^2.0'\n")
	before, err := f.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = f.execute(false, "add", "--no-save", root, "--target", "cursor")
	if err == nil || !strings.Contains(err.Error(), "managed saved") {
		t.Fatalf("imperative requirements refusal: %v", err)
	}
	after, err := f.store.Load()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("refusal changed state: %v", err)
	}
}

// A sidecar symlink may resolve inside the captured package, but inaccessible or
// escaping sidecars must fail closed instead of silently dropping graph edges.
func TestDependencyFactsContainedSidecarFiles(t *testing.T) {
	for _, mode := range []string{"missing", "regular", "contained symlink", "dangling symlink", "outside symlink", "directory", "empty file"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			sidecar := filepath.Join(root, "uap.toml")
			content := "schema_version = 1\n[dependencies]\n\"acme/context\" = '^2.0'\n"
			switch mode {
			case "regular":
				dependencyWrite(t, sidecar, content)
			case "empty file":
				dependencyWrite(t, sidecar, "")
			case "directory":
				if err := os.Mkdir(sidecar, 0700); err != nil {
					t.Fatal(err)
				}
			case "contained symlink":
				if err := os.Mkdir(filepath.Join(root, "author"), 0700); err != nil {
					t.Fatal(err)
				}
				dependencyWrite(t, filepath.Join(root, "author", "requirements.toml"), content)
				if err := os.Symlink("author/requirements.toml", sidecar); err != nil {
					t.Fatal(err)
				}
			case "dangling symlink":
				if err := os.Symlink("missing.toml", sidecar); err != nil {
					t.Fatal(err)
				}
			case "outside symlink":
				outside := filepath.Join(t.TempDir(), "requirements.toml")
				dependencyWrite(t, outside, content)
				if err := os.Symlink(outside, sidecar); err != nil {
					t.Fatal(err)
				}
			}
			facts, err := packageDependencyFacts(domain.PackageEnvelope{SnapshotRoot: root})
			switch mode {
			case "missing":
				if err != nil || len(facts) != 0 {
					t.Fatalf("missing sidecar: %+v %v", facts, err)
				}
			case "regular", "contained symlink":
				if err != nil || len(facts) != 1 || facts["acme/context"].Registry == nil || facts["acme/context"].Registry.Selector.Value != "^2.0" {
					t.Fatalf("valid sidecar: %+v %v", facts, err)
				}
			default:
				if err == nil {
					t.Fatalf("invalid sidecar treated as leaf: %+v", facts)
				}
			}
		})
	}
}
