package providers_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectDevinExactReceiptAuthorityRefusesBeforeEffects(t *testing.T) {
	for _, mismatch := range []string{"unknown-kind", "id", "duplicate", "other-project", "source-traversal", "nonclean-path"} {
		t.Run(mismatch, func(t *testing.T) {
			f := newDevinProjectFixture(t, false)
			root := f.request.Plan.NativeRegistryRoot
			objects := append([]domain.NativeObjectOwnership(nil), f.request.Delivery.NativeObjects...)
			skill := 0
			for i, object := range objects {
				if object.Kind == windsurf.DevinProjectSkillKind {
					skill = i
				}
			}
			switch mismatch {
			case "unknown-kind":
				objects[skill].Kind = "foreign_kind"
			case "id":
				objects[skill].ObjectID = "devin-skill:foreign"
			case "duplicate":
				objects = append(objects, objects[skill])
			case "other-project":
				objects[skill].Path = filepath.Join(f.other, ".devin", "skills", "docs")
			case "source-traversal":
				objects[skill].SourceRelative = "skills/../docs"
			case "nonclean-path":
				objects[skill].Path = filepath.Dir(objects[skill].Path) + string(filepath.Separator) + ".." + string(filepath.Separator) + "skills" + string(filepath.Separator) + "docs"
			}
			f.request.Delivery.NativeObjects = objects
			before := projectGeminiInventory(t, root)
			if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil {
				t.Fatal("forged project native ownership was admitted")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) {
				t.Fatal("receipt refusal changed native files")
			}
		})
	}
}

func TestProjectDevinRegistryRejectsCopiedBinding(t *testing.T) {
	f := newDevinProjectFixture(t, false)
	// Only the existing automatic intent is qualified for this project lane.
	outcome, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	devinAssertPrepared(t, outcome, err)
	binding := domain.ClientBinding{ClientID: "windsurf", Scope: "project", PhysicalArtifact: f.request.Plan.PhysicalArtifactID, TargetLocator: f.request.Plan.ActivePath, NativeProfileRoot: f.request.Plan.NativeRegistryRoot, NativeObjects: outcome.NativeObjects}
	adapter := windsurf.New()
	finding, err := adapter.InspectNativeRegistry(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, f.request.Client, f.request.Plan, &binding)
	if err != nil || finding != clients.RegistryExpected {
		t.Fatalf("exact project registry = %v,%v", finding, err)
	}
	for _, change := range []string{"client", "scope", "artifact", "package", "root"} {
		copied := binding
		switch change {
		case "client":
			copied.ClientID = "codex"
		case "scope":
			copied.Scope = "user"
		case "artifact":
			copied.PhysicalArtifact = "different"
		case "package":
			copied.TargetLocator = filepath.Join(f.other, ".uap", "managed", "clients", "windsurf", binding.PhysicalArtifact)
		case "root":
			copied.NativeProfileRoot = filepath.Join(f.other, ".devin")
		}
		if err := adapter.ValidateBindingProfile(binding.NativeProfileRoot, copied); change != "scope" && err == nil {
			t.Fatalf("direct copied %s binding admitted", change)
		}
		if _, err := adapter.InspectNativeRegistry(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, f.request.Client, f.request.Plan, &copied); err == nil {
			t.Fatalf("copied %s binding admitted", change)
		}
	}
}
