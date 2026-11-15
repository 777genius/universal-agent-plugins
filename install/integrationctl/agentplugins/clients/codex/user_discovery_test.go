package codex

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestCodexUserDiscoveryRequiresNativeCacheAndPluginIdentity(t *testing.T) {
	for _, scenario := range []string{"valid", "wrong-plugin", "wrong-path", "disabled", "wrong-name", "duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			_, request, _ := projectDiscoveryFixture(t)
			request.DeclaredName = "demo"
			request.Plan.DeclaredVersion = "1.0.0"
			request.Plan.PhysicalArtifactID = "owned"
			marketplace := shared.ManagedMarketplaceName("owned")
			pluginID := "demo@" + marketplace
			skills := []any{}
			for _, name := range []string{"first", "second"} {
				skills = append(skills, map[string]any{"name": "demo:" + name, "scope": "user", "enabled": true, "pluginId": pluginID, "path": filepath.Join(request.Client.ConfigRoot, "plugins", "cache", marketplace, "demo", "1.0.0", "skills", name, "SKILL.md")})
			}
			first := skills[0].(map[string]any)
			switch scenario {
			case "wrong-plugin":
				first["pluginId"] = "demo@foreign"
			case "wrong-path":
				first["path"] = filepath.Join(request.Client.ConfigRoot, "foreign", "SKILL.md")
			case "disabled":
				first["enabled"] = false
			case "wrong-name":
				first["name"] = "first"
			case "duplicate":
				skills = append(skills, first)
			}
			listing := map[string]any{"data": []any{map[string]any{"cwd": request.Delivery.ActivePath, "errors": []any{}, "skills": skills}}}
			err := verifyCodexUserSkills(listing, request, marketplace, pluginID, "1.0.0")
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("scenario=%s err=%v", scenario, err)
			}
		})
	}
}

type canceledUserDiscoveryRunner struct {
	cancel  context.CancelFunc
	listing []byte
}

func (runner *canceledUserDiscoveryRunner) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return legacyports.CommandResult{Stdout: runner.listing}, nil
}
func (runner *canceledUserDiscoveryRunner) RunDuplexToCompletion(ctx context.Context, _ legacyports.Command, _ func(io.Writer, io.Reader) error) error {
	runner.cancel()
	return ctx.Err()
}
func TestCodexUserDiscoveryPropagatesCallerCancellation(t *testing.T) {
	env, request, _ := projectDiscoveryFixture(t)
	request.VerifyOnly = true
	request.DeclaredName = "demo"
	request.Plan.PhysicalArtifactID = "owned"
	request.Plan.Scope = domain.ScopeUser
	request.Plan.NativeRegistryRoot = request.Client.ConfigRoot
	market := shared.ManagedMarketplaceName("owned")
	listing, err := json.Marshal(map[string]any{"installed": []any{map[string]any{"pluginId": "demo@" + market, "name": "demo", "marketplaceName": market, "installed": true, "enabled": true, "version": "1.0.0", "source": map[string]any{"source": "local", "path": request.Delivery.ActivePath}}}})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	env.Runner = &canceledUserDiscoveryRunner{cancel: cancel, listing: listing}
	outcome, err := completeCodexActivation(ctx, env, request, shared.StartedActivation(request))
	if !errors.Is(err, context.Canceled) || outcome.Activation != domain.ActivationPrepared {
		t.Fatalf("cancellation hidden: outcome=%+v err=%v", outcome, err)
	}
}

func TestCodexCacheVersionComesFromExactNativeSource(t *testing.T) {
	for _, scenario := range []string{"default-version", "older-native-default", "authored-mismatch", "foreign-source", "unsafe-version", "disabled"} {
		t.Run(scenario, func(t *testing.T) {
			_, request, _ := projectDiscoveryFixture(t)
			entry := map[string]any{"version": "1.0.0", "installed": true, "enabled": true, "source": map[string]any{"source": "local", "path": request.Delivery.ActivePath}}
			switch scenario {
			case "older-native-default":
				entry["version"] = "local"
			case "authored-mismatch":
				request.Plan.DeclaredVersion = "2.0.0"
			case "foreign-source":
				entry["source"].(map[string]any)["path"] = filepath.Join(request.Client.ConfigRoot, "foreign")
			case "unsafe-version":
				entry["version"] = "../foreign"
			case "disabled":
				entry["enabled"] = false
			}
			version, err := codexInstalledUserVersion(entry, request)
			valid := scenario == "default-version" || scenario == "older-native-default"
			if (err == nil) != valid || valid && version != entry["version"] {
				t.Fatalf("version=%q err=%v", version, err)
			}
		})
	}
}
