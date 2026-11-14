package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type preparedProjectAdapterID struct{ id domain.ClientID }

func (a preparedProjectAdapterID) ID() domain.ClientID { return a.id }

type preparedProjectInspector struct {
	preparedProjectAdapterID
	finding clients.RegistryFinding
	err     error
	calls   int
	env     clients.Env
	client  domain.DetectedClient
	plan    domain.DeliveryPlan
	managed *domain.ClientBinding
}

func (a *preparedProjectInspector) InspectPreparedRegistry(domain.DeliveryPlan, string, bool) (clients.RegistryFinding, error) {
	return clients.RegistryClear, nil
}
func (a *preparedProjectInspector) UsesNativeRegistryExecutable() bool { return true }
func (a *preparedProjectInspector) InspectNativeRegistry(_ context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	a.calls++
	a.env, a.client, a.plan, a.managed = env, client, plan, managed
	return a.finding, a.err
}

// A distinct injected FileIO lets the test verify the exact supplied kernel
// crosses the composition boundary; this identity-only spy does not mutate it.
type preparedProjectFileIO struct{}

func (*preparedProjectFileIO) ReadNoFollow(string) ([]byte, os.FileMode, bool, error) {
	return nil, 0, false, errors.New("unexpected file read")
}
func (*preparedProjectFileIO) WriteAtomic(string, []byte, os.FileMode) error {
	return errors.New("unexpected file write")
}
func (*preparedProjectFileIO) RemoveNoFollow(string) error {
	return errors.New("unexpected file remove")
}
func preparedProjectPlan(t *testing.T) (domain.DetectedClient, domain.DeliveryPlan) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "A")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(base, "user")
	client := domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: user}
	plan := identityPlan(filepath.Join(root, "managed"))
	plan.ClientID = client.ClientID
	plan.Scope = domain.ScopeProject
	plan.TargetContext = domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: root, UserConfigRoot: user}
	return client, plan
}
func TestPreparedProjectFileFindingRetainsDiscoveryBoundary(t *testing.T) {
	for _, tc := range []struct {
		name    string
		finding clients.RegistryFinding
		owned   bool
		want    domain.NativeIdentityState
		failure bool
	}{
		{"owned", clients.RegistryExpected, true, domain.NativeIdentityManaged, false},
		{"unowned-expected", clients.RegistryExpected, false, domain.NativeIdentityUnmanaged, false},
		{"collision", clients.RegistryCollision, true, domain.NativeIdentityUnmanaged, false},
		{"indeterminate", clients.RegistryIndeterminate, true, domain.NativeIdentityIndeterminate, false},
		{"unknown", clients.RegistryFinding(255), true, domain.NativeIdentityIndeterminate, false},
		{"error", clients.RegistryClear, true, domain.NativeIdentityIndeterminate, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, plan := preparedProjectPlan(t)
			if err := os.MkdirAll(plan.ActivePath, 0700); err != nil {
				t.Fatal(err)
			}
			a := &preparedProjectInspector{preparedProjectAdapterID: preparedProjectAdapterID{client.ClientID}, finding: tc.finding}
			sentinel := errors.New("file observation failed")
			if tc.failure {
				a.err = sentinel
			}
			registry, err := clients.NewRegistry(a)
			if err != nil {
				t.Fatal(err)
			}
			kernel := nativeconfig.NewWithFileIO(&preparedProjectFileIO{})
			runner := &identityRunner{}
			observer := NativeIdentityObserver{Stager: acceptingPackageVerifier{}, Runner: runner, NativeConfig: &kernel, Registry: registry}
			var managed *domain.ClientBinding
			if tc.owned {
				managed = &domain.ClientBinding{ClientID: "codex", Scope: "project", NativeObjects: []domain.NativeObjectOwnership{{Kind: "managed_package_directory", ManagedDigest: "sha256:owned"}}}
			}
			got, err := observer.ObservePreparedIdentity(t.Context(), client, plan, managed)
			if got.State != tc.want || (err != nil) != tc.failure || tc.failure && !errors.Is(err, sentinel) {
				t.Fatalf("finding %+v %v", got, err)
			}
			if got.NativeDiscoveryAttempted || got.NativeDiscoveryReconciled || got.NativeDiscoveryState != domain.NativeIdentityAbsent {
				t.Fatalf("local files became native discovery %+v", got)
			}
			if a.calls != 1 || a.env.Runner != nil || len(runner.commands) != 0 || !reflect.DeepEqual(a.env.NativeConfig, kernel) || a.managed != managed || !reflect.DeepEqual(a.plan, plan) || !reflect.DeepEqual(a.client, client) {
				t.Fatal("file inspector lost supplied authority/kernel or received native runner")
			}
		})
	}
}
func TestPreparedProjectFileObservationRefusesMalformedAuthority(t *testing.T) {
	for _, bad := range []string{"client", "scope", "target-scope", "missing-root", "relative-root", "other-user", "missing-user", "unqualified-policy", "project-alias"} {
		t.Run(bad, func(t *testing.T) {
			client, plan := preparedProjectPlan(t)
			a := &preparedProjectInspector{preparedProjectAdapterID: preparedProjectAdapterID{client.ClientID}, finding: clients.RegistryClear}
			registry, err := clients.NewRegistry(a)
			if err != nil {
				t.Fatal(err)
			}
			kernel := nativeconfig.New()
			switch bad {
			case "client":
				plan.ClientID = domain.ClientCline
			case "scope":
				plan.Scope = domain.ScopeUser
			case "target-scope":
				plan.TargetContext.Scope = domain.ScopeUser
			case "missing-root":
				plan.TargetContext.ProjectRoot = ""
			case "relative-root":
				plan.TargetContext.ProjectRoot = "relative"
			case "other-user":
				plan.TargetContext.UserConfigRoot = filepath.Dir(client.ConfigRoot)
			case "missing-user":
				plan.TargetContext.UserConfigRoot = ""
			case "unqualified-policy":
				client.ClientID = domain.ClientChatGPT
				plan.ClientID = client.ClientID
				a.id = client.ClientID
				registry, err = clients.NewRegistry(a)
				if err != nil {
					t.Fatal(err)
				}
			case "project-alias":
				link := plan.TargetContext.ProjectRoot + "-alias"
				if err := os.Symlink(plan.TargetContext.ProjectRoot, link); err != nil {
					t.Skip("symlink unavailable")
				}
				plan.TargetContext.ProjectRoot = link
			}
			got, err := (NativeIdentityObserver{Registry: registry, NativeConfig: &kernel}).ObservePreparedIdentity(t.Context(), client, plan, nil)
			if err == nil || got.State != domain.NativeIdentityIndeterminate || got.NativeDiscoveryReconciled || a.calls != 0 {
				t.Fatalf("malformed authority entered file registry: %+v %v calls=%d", got, err, a.calls)
			}
		})
	}
}
func TestPreparedProjectFileObservationMissingCapabilityFailsClosed(t *testing.T) {
	for _, missing := range []string{"inspector", "kernel"} {
		t.Run(missing, func(t *testing.T) {
			client, plan := preparedProjectPlan(t)
			var adapter clients.Adapter = preparedProjectAdapterID{client.ClientID}
			kernel := nativeconfig.New()
			observer := NativeIdentityObserver{NativeConfig: &kernel}
			if missing == "kernel" {
				adapter = &preparedProjectInspector{preparedProjectAdapterID: preparedProjectAdapterID{client.ClientID}}
				observer.NativeConfig = nil
			}
			var err error
			observer.Registry, err = clients.NewRegistry(adapter)
			if err != nil {
				t.Fatal(err)
			}
			got, err := observer.ObservePreparedIdentity(t.Context(), client, plan, nil)
			if got.State != domain.NativeIdentityIndeterminate || got.NativeDiscoveryAttempted || got.NativeDiscoveryReconciled {
				t.Fatalf("missing %s was accepted %+v %v", missing, got, err)
			}
		})
	}
}

func TestPreparedProjectCLIStillDoesNotDiscoverNativeRegistry(t *testing.T) {
	client, plan := preparedProjectPlan(t)
	client.ClientID = domain.ClientClaude
	plan.ClientID = client.ClientID
	plan.NativeRegistryExecutable = "/forbidden/claude"
	adapter := &preparedProjectInspector{preparedProjectAdapterID: preparedProjectAdapterID{client.ClientID}, finding: clients.RegistryExpected}
	registry, err := clients.NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}
	runner := &identityRunner{}
	got, err := (NativeIdentityObserver{Registry: registry, Runner: runner}).ObservePreparedIdentity(t.Context(), client, plan, nil)
	if err != nil || got.State != domain.NativeIdentityAbsent || got.NativeDiscoveryAttempted || got.NativeDiscoveryReconciled || adapter.calls != 0 || len(runner.commands) != 0 {
		t.Fatalf("CLI prepared path discovered native registry: %+v %v", got, err)
	}
}
