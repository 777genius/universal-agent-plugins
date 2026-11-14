package providers_test

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
)

func TestProjectDevinMigrationAndLocalShadowingRefuseWithoutEffects(t *testing.T) {
	for _, name := range []string{"config.json", "config.local.json", "mcp_config.local.json"} {
		t.Run(name, func(t *testing.T) {
			f := newDevinProjectFixture(t, false)
			root := f.request.Plan.NativeRegistryRoot
			projectGeminiWrite(t, filepath.Join(root, name), `{"mcpServers":{"docs":{"url":"https://foreign.test","disabled":true}},"policy":"keep"}`)
			before, user, other := projectGeminiInventory(t, root), projectGeminiInventory(t, f.user), projectGeminiInventory(t, f.other)
			input := clients.PlanInput{Client: f.request.Client, TargetContext: f.request.Plan.TargetContext, Envelope: f.envelope}
			plan := f.request.Plan
			if err := windsurf.New().CheckPlanPrecondition(input, &plan); err == nil {
				t.Fatal("migration/shadowing passed planning")
			}
			if _, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request); err == nil {
				t.Fatal("changed local policy passed activation")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) || !reflect.DeepEqual(user, projectGeminiInventory(t, f.user)) || !reflect.DeepEqual(other, projectGeminiInventory(t, f.other)) {
				t.Fatal("policy refusal changed scope bytes")
			}
		})
	}
}

func TestProjectDevinExactArtifactAndDeliveryCannotBeRebound(t *testing.T) {
	for _, change := range []string{"existing-package", "artifact", "empty-delivery", "other-delivery"} {
		t.Run(change, func(t *testing.T) {
			f := newDevinProjectFixture(t, false)
			root := f.request.Plan.NativeRegistryRoot
			switch change {
			case "existing-package":
				f.request.Plan.ActivePath = f.source
				f.request.Delivery.ActivePath = f.source
			case "artifact":
				f.request.Plan.PhysicalArtifactID = "other-artifact"
			case "empty-delivery":
				f.request.Delivery.ActivePath = ""
			case "other-delivery":
				f.request.Delivery.ActivePath = f.source
			}
			before := projectGeminiInventory(t, root)
			if _, err := windsurf.New().Activate(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, f.request); err == nil {
				t.Fatal("rebound delivery admitted")
			}
			if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) {
				t.Fatal("artifact refusal changed native files")
			}
		})
	}
}

func TestProjectDevinLocalOverrideForeignEntriesArePreserved(t *testing.T) {
	f := newDevinProjectFixture(t, false)
	root := f.request.Plan.NativeRegistryRoot
	override := filepath.Join(root, "mcp_config.local.json")
	const foreign = `{"mcpServers":{"other":{"url":"https://foreign.test","transport":"http","disabled":true}},"policy":"keep"}`
	projectGeminiWrite(t, override, foreign)
	out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	devinAssertPrepared(t, out, err)
	projectGeminiAssert(t, override, foreign)
	// Removal relies on recorded ownership even if later local policy shadows it.
	projectGeminiWrite(t, override, `{"mcpServers":{"docs":{"disabled":true}}}`)
	removal := f.removal(out.NativeObjects)
	removal.Client.Version = ""
	removal.Client.ExecutablePath = ""
	outRemove, err := f.activator(nativeconfig.New()).Deactivate(context.Background(), removal)
	if err != nil || !outRemove.ExternalRemovalComplete {
		t.Fatalf("recorded removal=%+v,%v", outRemove, err)
	}
	projectGeminiAssert(t, override, `{"mcpServers":{"docs":{"disabled":true}}}`)
}

func TestProjectDevinCommentedNativePolicyPreservesBytesAndRefusesUnsafeEntries(t *testing.T) {
	for _, name := range []string{"config.json", "config.local.json", "mcp_config.local.json"} {
		for _, unsafe := range []bool{false, true} {
			t.Run(name+fmt.Sprint(unsafe), func(t *testing.T) {
				f := newDevinProjectFixture(t, false)
				root := f.request.Plan.NativeRegistryRoot
				body := `{ // foreign native policy
"policy":"keep" /* foreign comment */}`
				if name == "mcp_config.local.json" {
					body = `{ // foreign local override
"mcpServers":{"other":{"url":"https://foreign.test","disabled":true}},"policy":"keep"}`
				}
				if unsafe {
					body = `{ // native legacy or local-shadow refusal
"mcpServers":{"docs":{"url":"https://foreign.test","disabled":true}}}`
				}
				path := filepath.Join(root, name)
				projectGeminiWrite(t, path, body)
				before := projectGeminiInventory(t, root)
				out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
				if unsafe {
					if err == nil {
						t.Fatal("commented unsafe policy admitted")
					}
					if !reflect.DeepEqual(before, projectGeminiInventory(t, root)) {
						t.Fatal("commented refusal changed bytes")
					}
				} else {
					devinAssertPrepared(t, out, err)
				}
				projectGeminiAssert(t, path, body)
			})
		}
	}
}
