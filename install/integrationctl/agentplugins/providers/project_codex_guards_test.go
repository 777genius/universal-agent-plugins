package providers_test

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectCodexRejectsChangedContextBeforeNativeEffects(t *testing.T) {
	for _, change := range []string{"user-registry", "original-user", "plan-scope", "context-scope"} {
		t.Run(change, func(t *testing.T) {
			f := newProjectCodexFixture(t)
			r := f.stage(t, domain.CompatibilityHints{})
			switch change {
			case "user-registry":
				r.Plan.NativeRegistryRoot = f.user
			case "original-user":
				r.Plan.TargetContext.UserConfigRoot = f.b
			case "plan-scope":
				r.Plan.Scope = domain.ScopeUser
			case "context-scope":
				r.Plan.TargetContext.Scope = domain.ScopeUser
			}
			before, user := projectGeminiInventory(t, f.a), projectGeminiInventory(t, f.user)
			if _, err := f.activator.Activate(t.Context(), r); err == nil {
				t.Fatal("changed project context reached native activation")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.a)) || !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) {
				t.Fatal("refused context changed project or user files")
			}
		})
	}
}

func TestProjectCodexMalformedCollisionAndWriterFailurePreserveNativeFiles(t *testing.T) {
	for _, name := range []string{"malformed", "foreign-disabled", "writer-failure"} {
		t.Run(name, func(t *testing.T) {
			f := newProjectCodexFixture(t)
			r := f.stage(t, domain.CompatibilityHints{})
			actor := f.activator
			switch name {
			case "malformed":
				projectCodexWrite(t, filepath.Join(f.a, ".codex", "config.toml"), "[mcp_servers\n")
			case "foreign-disabled":
				projectCodexWrite(t, filepath.Join(f.a, ".codex", "config.toml"), "[mcp_servers.docs]\nurl='https://foreign.test/keep'\nenabled=false\n")
			case "writer-failure":
				kernel := nativeconfig.NewWithLockAcquirer(func(nativeconfig.Paths, nativeconfig.Codec) (func() error, error) {
					return nil, errors.New("TEST writer unavailable")
				})
				actor.NativeConfig = &kernel
			}
			before, user, b := projectGeminiInventory(t, f.a), projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.b)
			if _, err := actor.Activate(t.Context(), r); err == nil {
				t.Fatal("invalid/failed project native mutation succeeded")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, f.a)) || !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(b, projectGeminiInventory(t, f.b)) {
				t.Fatal("failed project mutation left native effects or consumed foreign files")
			}
		})
	}
}

func TestProjectCodexRegistryUsesProjectComponentsAndExactBinding(t *testing.T) {
	f := newProjectCodexFixture(t)
	r := f.stage(t, domain.CompatibilityHints{})
	outcome, err := f.activator.Activate(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	r.Plan.PreviousNativeObjects = outcome.NativeObjects
	binding := domain.ClientBinding{Scope: string(domain.ScopeProject), NativeProfileRoot: f.a, PhysicalArtifact: r.Plan.PhysicalArtifactID, NativeObjects: outcome.NativeObjects}
	adapter := codex.New()
	finding, err := adapter.InspectNativeRegistry(t.Context(), clients.Env{NativeConfig: f.kernel}, r.Client, r.Plan, &binding)
	if err != nil || finding != clients.RegistryExpected {
		t.Fatalf("project component registry = %v, %v", finding, err)
	}
	if finding, err := adapter.InspectPreparedRegistry(r.Plan, "demo", true); err != nil || finding != clients.RegistryExpected {
		t.Fatalf("read-only prepared project registry = %v, %v", finding, err)
	}
	binding.PhysicalArtifact = "different-artifact"
	if _, err := adapter.InspectNativeRegistry(t.Context(), clients.Env{NativeConfig: f.kernel}, r.Client, r.Plan, &binding); err == nil {
		t.Fatal("another artifact supplied project ownership")
	}
}
