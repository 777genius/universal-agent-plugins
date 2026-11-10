package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func projectSourceGraph(t *testing.T) (rolloutDirectoryFixture, environment.Requirements) {
	t.Helper()
	f := newRolloutDirectoryFixture(t, []domain.ClientID{domain.ClientCopilot}, nil)
	f.retainV1Only()
	f.cli.app.Scope = domain.ScopeProject
	f.cli.app.SecurityEvaluator = dependencyTestScanner{}
	dependencyWrite(t, filepath.Join(f.v1.root, "uap.toml"), "schema_version = 1\n[dependencies]\n\"owner/child\" = \"2.0.0\"\n")
	dependencyWrite(t, filepath.Join(f.v2.root, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"child","version":"2.0.0","description":"Required Project child"}`)
	childPackage, err := f.cli.app.acquireLocal(context.Background(), f.v2.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = childPackage.cleanup() }()
	loaded, err := f.cli.app.acquireLocal(context.Background(), f.v1.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = loaded.cleanup() }()
	snapshot := &f.directory.bundle.Snapshot
	root := &snapshot.Distributions[0]
	root.Releases[0].TreeDigest = loaded.envelope.TreeDigest
	snapshot.Evidence[0].PackageTreeDigest = loaded.envelope.TreeDigest
	child := *root
	child.ID, child.ProductID = "owner/child", "child"
	child.Releases = append([]domain.DirectoryRelease(nil), root.Releases...)
	child.Releases[0].PackageSource.Revision = strings.Repeat("3", 40)
	child.Releases[0].PackageVersion = f.v2.version
	child.Releases[0].ManifestName = "child"
	child.Releases[0].TreeDigest, child.Releases[0].ManifestDigest = childPackage.envelope.TreeDigest, childPackage.envelope.ManifestDigest
	child.ReleasePolicies = append([]domain.DirectoryReleasePolicy(nil), root.ReleasePolicies...)
	child.ReleasePolicies[0].Targets = append([]domain.DirectoryTarget(nil), root.ReleasePolicies[0].Targets...)
	child.ReleasePolicies[0].CurrentEvidence = []string{"child/materialization/copilot"}
	product := snapshot.Products[0]
	product.ID, product.ManifestName, product.DefaultDistribution = "child", "child", child.ID
	product.Aliases, product.ReservedAliases, product.Distributions = []string{"child"}, []string{"child"}, []string{child.ID}
	evidence := snapshot.Evidence[0]
	evidence.ID, evidence.DistributionID, evidence.PackageTreeDigest = child.ReleasePolicies[0].CurrentEvidence[0], child.ID, child.Releases[0].TreeDigest
	snapshot.Products = append(snapshot.Products, product)
	snapshot.Distributions = append(snapshot.Distributions, child)
	snapshot.Evidence = append(snapshot.Evidence, evidence)
	f.acquirer.revisionRoots[child.Releases[0].PackageSource.Revision] = f.v2.root
	requirements := environment.Requirements{SchemaVersion: 1,
		Dependencies: map[string]environment.Requirement{"root": {Alias: "root", Registry: &environment.RegistrySource{
			Distribution: "rollout-demo", Selector: environment.Selector{Kind: environment.SelectorVersion, Value: "1.0.0"},
		}}}, Targets: map[string][]string{"copilot": {"root"}}}
	return f, requirements
}

func resolveProjectSourceGraph(ctx context.Context, app App, requirements environment.Requirements) (environment.Lock, error) {
	canonical, err := app.canonicalizeDependencyRequirements(ctx, requirements)
	if err != nil {
		return environment.Lock{}, err
	}
	return (environment.Coordinator{EnvironmentID: app.environmentID(), Resolver: dependencyResolver{app: app, requirements: canonical}}).Resolve(ctx, canonical, environment.Lock{}, "")
}

func TestDependencyProjectSourceAliasRootChildAndExactLock(t *testing.T) {
	f, requirements := projectSourceGraph(t)
	ctx := context.Background()
	lock, err := resolveProjectSourceGraph(ctx, f.cli.app, requirements)
	if err != nil || len(lock.Packages) != 2 {
		t.Fatalf("Project signed alias/root/child resolution: %+v %v", lock, err)
	}
	for _, pin := range lock.Packages {
		if !reflect.DeepEqual(pin.Targets, []string{"copilot"}) || pin.Source.Registry == nil {
			t.Fatalf("Project lock borrowed a User backend surface: %+v", pin)
		}
		loaded, err := materializeLockedPackage(ctx, f.cli.app, pin, false, nil)
		if err != nil {
			t.Fatalf("exact Project lock replay %s: %v", pin.Alias, err)
		}
		if loaded.envelope.TreeDigest != pin.TreeDigest || loaded.envelope.ManifestDigest != pin.ManifestDigest {
			t.Fatal("Project restore changed immutable signed identity")
		}
		if err := loaded.cleanup(); err != nil {
			t.Fatal(err)
		}
	}
	user := f.cli.app
	user.Scope = domain.ScopeUser
	requirements.Dependencies["root"].Registry.Distribution = "rollout-demo"
	if _, err := resolveProjectSourceGraph(ctx, user, requirements); !errors.Is(err, domain.ErrDirectoryIneligible) || !strings.Contains(err.Error(), "missing vscode") {
		t.Fatalf("User shared-backend refusal changed: %v", err)
	}
}

func TestDependencyProjectSourceRefusesRequiredChildPolicy(t *testing.T) {
	for _, mutation := range []string{"missing grant", "revoked child", "publication floor", "target floor", "tree digest"} {
		t.Run(mutation, func(t *testing.T) {
			f, requirements := projectSourceGraph(t)
			child := &f.directory.bundle.Snapshot.Distributions[1]
			switch mutation {
			case "missing grant":
				child.ReleasePolicies[0].Targets = nil
			case "revoked child":
				child.ReleasePolicies[0].Status = domain.ReleaseRevoked
			case "publication floor":
				child.ReleasePolicies[0].MinimumInstallerVersion = "99.0.0"
			case "target floor":
				child.ReleasePolicies[0].Targets[0].MinimumInstallerVersion = "99.0.0"
			case "tree digest":
				child.Releases[0].TreeDigest = "sha256:" + strings.Repeat("f", 64)
			}
			if _, err := resolveProjectSourceGraph(context.Background(), f.cli.app, requirements); err == nil {
				t.Fatal("Project source admission authorized an ineligible/mismatched mandatory child")
			}
			if state, err := f.cli.store.Load(); err != nil || len(state.Installations) != 0 {
				t.Fatalf("refused dependency graph changed installations: %+v %v", state, err)
			}
		})
	}
}

func TestDependencyProjectSourceExactLockRejectsChangedPinsAndRevocation(t *testing.T) {
	for _, mutation := range []string{"tree", "manifest", "revoked"} {
		t.Run(mutation, func(t *testing.T) {
			f, requirements := projectSourceGraph(t)
			ctx := context.Background()
			lock, err := resolveProjectSourceGraph(ctx, f.cli.app, requirements)
			if err != nil {
				t.Fatal(err)
			}
			pin := lock.Packages[0]
			switch mutation {
			case "tree":
				pin.TreeDigest = "sha256:" + strings.Repeat("f", 64)
			case "manifest":
				pin.ManifestDigest = "sha256:" + strings.Repeat("f", 64)
			case "revoked":
				f.directory.bundle.Snapshot.Revocations = []domain.DirectoryRevocation{{DistributionID: pin.Source.Registry.DistributionID, ReleaseSequence: pin.Source.Registry.ReleaseSequence}}
			}
			loaded, err := materializeLockedPackage(ctx, f.cli.app, pin, false, nil)
			if loaded.cleanup != nil {
				_ = loaded.cleanup()
			}
			if err == nil {
				t.Fatal("Project exact-lock replay accepted changed identity or revoked publication")
			}
		})
	}
}
