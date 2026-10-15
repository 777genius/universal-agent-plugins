package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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
				_, err = (dependencyResolver{app: f.app}).resolveGit(context.Background(), requirement, nil, false)
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
