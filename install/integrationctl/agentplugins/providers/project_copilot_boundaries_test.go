package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/copilot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestProjectCopilotRefusesUnsafeConfigWithoutEffects(t *testing.T) {
	for name, mutate := range map[string]func(*testing.T, *copilotProjectFixture) nativeconfig.Kernel{
		"bare native map": func(t *testing.T, f *copilotProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, f.config, `{"foreign":{"type":"http","url":"https://foreign.test/mcp"}}`)
			return nativeconfig.New()
		},
		"malformed JSON": func(t *testing.T, f *copilotProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, f.config, `{"mcpServers":{},}`)
			return nativeconfig.New()
		},
		"disabled foreign same-name": func(t *testing.T, f *copilotProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, f.config, `{"mcpServers":{"docs":{"type":"http","url":"https://foreign.test/mcp","enabled":false}}}`)
			return nativeconfig.New()
		},
		"foreign skill": func(t *testing.T, f *copilotProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, filepath.Join(f.project, ".github", "skills", "docs", "SKILL.md"), "foreign claimed name\n")
			return nativeconfig.New()
		},
		"effective config changed after stage": func(t *testing.T, f *copilotProjectFixture) nativeconfig.Kernel {
			projectGeminiWrite(t, filepath.Join(f.project, ".mcp.json"), copilotForeignConfig)
			return nativeconfig.New()
		},
		"native writer unavailable": func(*testing.T, *copilotProjectFixture) nativeconfig.Kernel {
			return nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
				return nil, errors.New("TEST native writer unavailable")
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			f := newCopilotProjectFixture(t, false, false)
			kernel := mutate(t, f)
			before := projectGeminiInventory(t, f.base)
			if _, err := f.activator(kernel).Activate(context.Background(), f.request); err == nil {
				t.Fatal("unsafe Copilot project activation succeeded")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
				t.Fatal("refused Copilot activation left filesystem effects")
			}
		})
	}
}

func TestProjectCopilotPreparedCollisionAndExactBindingAuthority(t *testing.T) {
	f := newCopilotProjectFixture(t, false, false)
	adapter := copilot.New()
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
	copilotPrepared(t, out, err)
	plan.PreviousNativeObjects = out.NativeObjects
	binding := domain.ClientBinding{Scope: "project", PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath,
		NativeProfileRoot: f.project, NativeObjects: out.NativeObjects}
	if err := adapter.ValidateBindingProfile(f.project, binding); err != nil {
		t.Fatal(err)
	}
	finding, err = observer.ObservePreparedIdentity(context.Background(), f.request.Client, plan, nil)
	if err != nil || finding.State != domain.NativeIdentityUnmanaged {
		t.Fatalf("unowned native components were adopted: %+v, %v", finding, err)
	}
	for _, change := range []func(*domain.ClientBinding){
		func(b *domain.ClientBinding) { b.Scope = "user" },
		func(b *domain.ClientBinding) { b.PhysicalArtifact = "other" },
		func(b *domain.ClientBinding) { b.TargetLocator = filepath.Join(f.other, "artifact") },
		func(b *domain.ClientBinding) { b.NativeProfileRoot = f.other },
	} {
		wrong := binding
		change(&wrong)
		if _, err := adapter.InspectNativeRegistry(context.Background(), clientsCopilotEnv(), f.request.Client, plan, &wrong); err == nil {
			t.Fatalf("mismatched binding accepted: %+v", wrong)
		}
	}
}

func TestProjectCopilotScopeAndAliasCannotReachUserOrOtherProject(t *testing.T) {
	f := newCopilotProjectFixture(t, false, false)
	for _, change := range []func(*domain.ActivationRequest){
		func(r *domain.ActivationRequest) { r.Plan.Scope = domain.ScopeUser },
		func(r *domain.ActivationRequest) { r.Plan.NativeRegistryRoot = f.user },
		func(r *domain.ActivationRequest) { r.Plan.TargetContext.UserConfigRoot = f.other },
	} {
		request := f.request
		change(&request)
		before := projectGeminiInventory(t, f.base)
		if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), request); err == nil {
			t.Fatal("mismatched Copilot project context succeeded")
		}
		if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
			t.Fatal("scope refusal changed bytes")
		}
	}
	if err := os.Rename(filepath.Join(f.project, ".github"), filepath.Join(f.project, "github-preserved")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.other, ".github"), filepath.Join(f.project, ".github")); err != nil {
		t.Skipf("test symlink unavailable: %v", err)
	}
	other := projectGeminiInventory(t, f.other)
	if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil {
		t.Fatal("Copilot project followed an alias into B")
	}
	if !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
		t.Fatal("alias refusal changed B")
	}
}

func TestProjectCopilotStaleOwnedNativeMemberCannotBeRemovedOrOverwritten(t *testing.T) {
	f := newCopilotProjectFixture(t, false, false)
	a := f.activator(nativeconfig.New())
	out, err := a.Activate(context.Background(), f.request)
	copilotPrepared(t, out, err)
	projectGeminiWrite(t, f.config, `{"mcpServers":{"docs":{"type":"http","url":"https://external.test/changed","enabled":false},"foreign":{"type":"http","url":"https://foreign.test/mcp"}}}`)
	before := projectGeminiInventory(t, f.base)
	f.request.PreviousNativeObjects = out.NativeObjects
	if _, err := a.Activate(context.Background(), f.request); err == nil {
		t.Fatal("Copilot overwrote a stale owned MCP member")
	}
	if _, err := a.Deactivate(context.Background(), f.remove(out.NativeObjects)); err == nil {
		t.Fatal("Copilot removed a stale owned MCP member")
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("stale receipt refusal changed native or package bytes")
	}
}
