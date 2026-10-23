package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kimi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestProjectKimiRefusesUnsafeConfigWithoutEffects(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, *kimiProjectFixture) nativeconfig.Kernel{
		"malformed JSON": func(t *testing.T, f *kimiProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, f.config, `{"mcpServers":{},}`)
			return nativeconfig.New()
		},
		"disabled foreign same-name": func(t *testing.T, f *kimiProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, f.config, `{"mcpServers":{"docs":{"transport":"http","url":"https://foreign.test/mcp","enabled":false}}}`)
			return nativeconfig.New()
		},
		"foreign skill": func(t *testing.T, f *kimiProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, filepath.Join(f.project, ".kimi-code", "skills", "docs", "SKILL.md"), "foreign claimed name\n")
			return nativeconfig.New()
		},
		"native writer unavailable": func(*testing.T, *kimiProjectFixture) nativeconfig.Kernel {
			return nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				return nil, errors.New("TEST native writer unavailable")
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newKimiProjectFixture(t, false, false)
			kernel := mutate(t, f)
			before := projectGeminiInventory(t, f.base)
			if _, err := f.activator(kernel).Activate(context.Background(), f.request); err == nil {
				t.Fatal("unsafe Kimi project activation succeeded")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("refused Kimi activation left filesystem effects")
			}
		})
	}
}

func TestProjectKimiPreparedCollisionAndExactBindingAuthority(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	adapter := kimi.New()
	kernel := nativeconfig.New()
	observer := providers.NativeIdentityObserver{Registry: f.registry, NativeConfig: &kernel}
	plan := f.request.Plan
	plan.DeclaredName = "demo"
	before := projectGeminiInventory(t, f.base)
	finding, err := observer.ObservePreparedIdentity(context.Background(), f.request.Client, plan, nil)
	if err != nil || finding.NativeDiscoveryAttempted {
		t.Fatalf("read-only prepared inspection = %+v, %v", finding, err)
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("prepared inspection created state")
	}
	out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	kimiPrepared(t, out, err)
	plan.PreviousNativeObjects = out.NativeObjects
	binding := domain.ClientBinding{ClientID: "kimi", Scope: "project", PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath,
		NativeProfileRoot: filepath.Join(f.project, ".kimi-code"), NativeObjects: out.NativeObjects}
	if err := adapter.ValidateBindingProfile(filepath.Join(f.project, ".kimi-code"), binding); err != nil {
		t.Fatal(err)
	}
	copied := binding
	copied.ClientID = "codex"
	before = projectGeminiInventory(t, f.base)
	if err := adapter.ValidateBindingProfile(binding.NativeProfileRoot, copied); err == nil || !strings.Contains(err.Error(), "recorded project client differs") {
		t.Fatalf("direct project profile validator accepted another client: %v", err)
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("direct copied-client refusal changed bytes")
	}
	finding, err = observer.ObservePreparedIdentity(context.Background(), f.request.Client, plan, nil)
	if err != nil || finding.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("unowned native components were adopted: %+v, %v", finding, err)
	}
	for _, change := range []func(*domain.ClientBinding){
		func(b *domain.ClientBinding) { b.Scope = "user" },
		func(b *domain.ClientBinding) { b.ClientID = "codex" },
		func(b *domain.ClientBinding) { b.PhysicalArtifact = "other" },
		func(b *domain.ClientBinding) { b.TargetLocator = filepath.Join(f.other, "artifact") },
		func(b *domain.ClientBinding) { b.NativeProfileRoot = f.other },
	} {
		wrong := binding
		change(&wrong)
		if _, err := adapter.InspectNativeRegistry(context.Background(), clientsKimiEnv(), f.request.Client, plan, &wrong); err == nil {
			t.Fatalf("mismatched binding accepted: %+v", wrong)
		}
	}
}

func TestProjectKimiScopeAndAliasCannotReachUserOrOtherProject(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	for _, change := range []func(*domain.ActivationRequest){
		func(r *domain.ActivationRequest) { r.Plan.Scope = domain.ScopeUser },
		func(r *domain.ActivationRequest) { r.Plan.NativeRegistryRoot = f.user },
		func(r *domain.ActivationRequest) { r.Plan.TargetContext.UserConfigRoot = f.other },
	} {
		request := f.request
		change(&request)
		before := projectGeminiInventory(t, f.base)
		if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), request); err == nil {
			t.Fatal("mismatched Kimi project context succeeded")
		}
		if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
			t.Fatal("scope refusal changed bytes")
		}
	}
	if err := os.Rename(filepath.Join(f.project, ".kimi-code"), filepath.Join(f.project, "github-preserved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.other, ".kimi-code"), filepath.Join(f.project, ".kimi-code")); err != nil {
		t.Skipf("test symlink unavailable: %v", err)
	}
	other := projectGeminiInventory(t, f.other)
	if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil {
		t.Fatal("Kimi project followed an alias into B")
	}
	if !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
		t.Fatal("alias refusal changed B")
	}
}

func TestProjectKimiStaleOwnedNativeMemberCannotBeRemovedOrOverwritten(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	a := f.activator(nativeconfig.New())
	out, err := a.Activate(context.Background(), f.request)
	kimiPrepared(t, out, err)
	projectGeminiWrite(t, f.config, `{"mcpServers":{"docs":{"transport":"http","url":"https://external.test/changed","enabled":false},"foreign":{"transport":"http","url":"https://foreign.test/mcp"}}}`)
	before := projectGeminiInventory(t, f.base)
	f.request.PreviousNativeObjects = out.NativeObjects
	if _, err := a.Activate(context.Background(), f.request); err == nil {
		t.Fatal("Kimi overwrote a stale owned MCP member")
	}
	if _, err := a.Deactivate(context.Background(), f.remove(out.NativeObjects)); err == nil {
		t.Fatal("Kimi removed a stale owned MCP member")
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("stale receipt refusal changed native or package bytes")
	}
}

func TestProjectKimiRejectsReboundPhysicalArtifactBeforeEffects(t *testing.T) {
	for _, mismatch := range []string{"in-project-package", "physical-artifact"} {
		t.Run(mismatch, func(t *testing.T) {
			f := newKimiProjectFixture(t, false, false)
			request := f.request
			if mismatch == "in-project-package" {
				request.Plan.ActivePath = filepath.Join(f.project, "other-package")
				request.Delivery.ActivePath = request.Plan.ActivePath
				projectGeminiWrite(t, filepath.Join(request.Plan.ActivePath, "keep.txt"), "other project child\n")
			} else {
				request.Plan.PhysicalArtifactID = "another-artifact"
			}
			before := projectGeminiInventory(t, f.base)
			adapter := kimi.New()
			operations := map[string]func() error{
				"composed activation": func() error {
					_, err := f.activator(nativeconfig.New()).Activate(context.Background(), request)
					return err
				},
				"direct activation": func() error {
					_, err := adapter.Activate(context.Background(), clientsKimiEnv(), request)
					return err
				},
				"direct projection": func() error {
					_, err := adapter.Project(context.Background(), clients.ProjectionInput{StagingPath: request.Delivery.ActivePath, Plan: request.Plan, Envelope: f.envelope})
					return err
				},
				"direct active reuse": func() error {
					_, err := adapter.ProjectActiveNative(context.Background(), request.Plan.ActivePath, f.envelope, request.Plan, "")
					return err
				},
			}
			for name, operation := range operations {
				if err := operation(); err == nil || !strings.Contains(err.Error(), "managed package differs") {
					t.Fatalf("%s accepted rebound %s: %v", name, mismatch, err)
				}
				if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
					t.Fatalf("%s changed native/package/user/B bytes", name)
				}
			}
		})
	}
}

func TestProjectKimiDirectActivationRequiresExactDeliveredArtifact(t *testing.T) {
	f := newKimiProjectFixture(t, false, false)
	other := filepath.Join(f.project, "other-delivered-package")
	projectGeminiWrite(t, filepath.Join(other, "keep.txt"), "foreign in-project package\n")
	before := projectGeminiInventory(t, f.base)
	adapter := kimi.New()
	for _, path := range []string{other, ""} {
		request := f.request
		request.Delivery.ActivePath = path
		if err := adapter.PreflightActivation(clientsKimiEnv(), request); (path == "") != (err == nil) {
			t.Fatalf("pre-staging empty delivery/project mismatch admission %q = %v", path, err)
		}
		if _, err := adapter.Activate(context.Background(), clientsKimiEnv(), request); err == nil || !strings.Contains(err.Error(), "delivery differs") {
			t.Fatalf("direct activation accepted delivery-only mismatch %q: %v", path, err)
		}
		if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
			t.Fatal("delivery-only mismatch changed project/native/user/B bytes")
		}
	}
}
