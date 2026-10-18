package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Production's adapters lazily create acquisition and policy caches. Put real
// Git acquisition under the global root too, so the isolated fixture cannot
// conceal writes before consent simply because its TempRoot is elsewhere.
type transferCachedSource struct {
	SourceAcquirer
	root string
}

type transferEditingDetector struct {
	clients []domain.DetectedClient
	mutate  func()
}

func (detector *transferEditingDetector) Detect(context.Context) ([]domain.DetectedClient, error) {
	if detector.mutate != nil {
		mutate := detector.mutate
		detector.mutate = nil
		mutate()
	}
	return detector.clients, nil
}

// Already installed, identical bytes must still be checked after preflight.
// Captured candidate equality cannot authorize a stale "unchanged" result.
func transferNoopRecheck(t *testing.T, fixture freshRestoreEnvironment, manifest, lock string) {
	t.Helper()
	for _, changed := range []string{"source", "destination"} {
		t.Run("combined unchanged rechecks "+changed, func(t *testing.T) {
			exchange := t.TempDir()
			transferPair(t, exchange, manifest, lock)
			t.Chdir(exchange)
			path := filepath.Join(exchange, "uap.toml")
			if changed == "destination" {
				path, _ = fixture.app.dependencyPaths()
			}
			var afterEditor map[string]freshRestoreStamp
			fixture.app.Detector = &transferEditingDetector{clients: []domain.DetectedClient{fixture.client}, mutate: func() {
				dependencyWrite(t, path, "# edit during preflight\n"+manifest)
				afterEditor = freshRestoreFiles(t, fixture.root, exchange, fixture.client.ConfigRoot)
			}}
			mutations := fixture.runner.mutations
			out, _, err := fixture.execute(false, "deps", "import", "--install", "--yes", "--format", "json")
			if !errors.Is(err, installer.ErrPlanChanged) || strings.Contains(out, `"outcome": "unchanged"`) {
				t.Fatalf("combined no-op accepted stale %s: %v %s", changed, err, out)
			}
			if afterEditor == nil || !reflect.DeepEqual(afterEditor, freshRestoreFiles(t, fixture.root, exchange, fixture.client.ConfigRoot)) || fixture.runner.mutations != mutations {
				t.Fatal("stale combined no-op wrote files or clobbered the concurrent edit")
			}
		})
	}
}

func (source transferCachedSource) AcquireGitHubVerified(ctx context.Context, repo, commit, path, digest string) (domain.PackageSnapshot, error) {
	if err := os.MkdirAll(source.root, 0700); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return source.SourceAcquirer.AcquireGitHubVerified(ctx, repo, commit, path, digest)
}

type transferCachedDirectory struct {
	DirectoryClient
	root string
}

func (client transferCachedDirectory) Load(ctx context.Context, floor uint64) (directoryv1.VerifiedBundle, error) {
	if err := os.MkdirAll(client.root, 0700); err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	if err := os.WriteFile(filepath.Join(client.root, "directory-cache"), []byte("verified fixture cache"), 0600); err != nil {
		return directoryv1.VerifiedBundle{}, err
	}
	return client.DirectoryClient.Load(ctx, floor)
}

type transferCachedScanner struct {
	ports.PackageSecurityEvaluator
	root string
}

type transferBlockingScanner struct{}

func (transferBlockingScanner) Evaluate(context.Context, domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	return *testSecurityAssessment(1, 0), nil
}

func (scanner transferCachedScanner) Evaluate(ctx context.Context, input domain.SecurityEvaluationInput) (domain.SecurityAssessment, error) {
	if err := os.MkdirAll(scanner.root, 0700); err != nil {
		return domain.SecurityAssessment{}, err
	}
	if err := os.WriteFile(filepath.Join(scanner.root, "security-cache"), []byte("fixture security cache"), 0600); err != nil {
		return domain.SecurityAssessment{}, err
	}
	return scanner.PackageSecurityEvaluator.Evaluate(ctx, input)
}

func (graph transferGraph) staging(t *testing.T) {
	for _, mode := range []string{"decline", "scanner refusal", "security block", "confirmed"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newFreshRestoreEnvironment(t, graph.repo, graph.directory)
			if err := os.RemoveAll(fixture.app.DependenciesRoot); err != nil {
				t.Fatal(err)
			}
			persistentSource := *fixture.source.dependencyTestSource
			persistentSource.TempRoot = filepath.Join(fixture.app.DependenciesRoot, "acquisition")
			fixture.app.SourceAcquirer = transferCachedSource{&persistentSource, persistentSource.TempRoot}
			fixture.app.DirectoryClient = transferCachedDirectory{graph.directory, filepath.Join(fixture.app.DependenciesRoot, "directory-cache")}
			fixture.app.SecurityEvaluator = transferCachedScanner{dependencyTestScanner{}, filepath.Join(fixture.app.DependenciesRoot, "security-cache")}
			roots := make([]string, 0, 3)
			var temporarySource *freshRestoreSource
			fixture.app.SourceAcquirerForDryRun = func(root string) SourceAcquirer {
				roots = append(roots, root)
				delegate := *fixture.source.dependencyTestSource
				delegate.TempRoot = filepath.Join(root, "acquisition")
				temporarySource = &freshRestoreSource{dependencyTestSource: &delegate}
				return transferCachedSource{temporarySource, delegate.TempRoot}
			}
			fixture.app.DirectoryClientForDryRun = func(root string) (DirectoryClient, error) {
				roots = append(roots, root)
				return transferCachedDirectory{graph.directory, filepath.Join(root, "directory-cache")}, nil
			}
			fixture.app.SecurityEvaluatorForDryRun = func(root string) ports.PackageSecurityEvaluator {
				roots = append(roots, root)
				var evaluator ports.PackageSecurityEvaluator = dependencyTestScanner{fail: mode == "scanner refusal"}
				if mode == "security block" {
					evaluator = transferBlockingScanner{}
				}
				return transferCachedScanner{evaluator, filepath.Join(root, "security-cache")}
			}
			before := freshRestoreFiles(t, fixture.root, fixture.client.ConfigRoot)
			var out string
			var err error
			if mode == "confirmed" || mode == "security block" {
				out, _, err = fixture.execute(false, "deps", "import", "--install", "--yes", "--format", "json")
			} else {
				out, _, err = fixture.executeInput(true, "n\n", "deps", "import", "--install")
			}
			if mode == "security block" {
				if err == nil || !strings.Contains(err.Error(), "blocking finding") {
					t.Fatalf("blocking security assessment was accepted: %v %s", err, out)
				}
				result := transferJSON(t, out)
				if string(result["outcome"]) != `"refused"` || string(result["agreement_saved"]) != "false" || !strings.Contains(out, "blocked_by_security_review") || !strings.Contains(out, "SEC330") {
					t.Fatalf("security refusal lost transfer context or security findings: %s", out)
				}
			} else if mode == "scanner refusal" {
				if err == nil || !strings.Contains(err.Error(), "scanner unavailable") {
					t.Fatalf("strict scanner refusal lost: %v", err)
				}
			} else if err != nil {
				t.Fatalf("%s: %v %s", mode, err, out)
			}
			if len(roots) != 3 || temporarySource == nil || temporarySource.downloads == 0 || temporarySource.lookups != 0 {
				t.Fatal("real combined preflight bypassed temporary adapter factories")
			}
			if mode == "confirmed" {
				transferJSON(t, out)
				freshRestoreCheckInstalled(t, fixture, graph.locked)
			} else if fixture.runner.mutations != 0 || !reflect.DeepEqual(before, freshRestoreFiles(t, fixture.root, fixture.client.ConfigRoot)) {
				t.Fatal("unapproved/refused real import wrote persistent agreement, acquisition/cache, store, state or profile")
			}
			for _, root := range roots {
				if _, err := os.Lstat(root); !os.IsNotExist(err) {
					t.Fatalf("temporary preflight root was retained: %s: %v", root, err)
				}
			}
			for _, root := range temporarySource.snapshots {
				if _, err := os.Lstat(root); !os.IsNotExist(err) {
					t.Fatalf("staged package was retained: %s: %v", root, err)
				}
			}
		})
	}
}
