package agentpluginscli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestKiroPrepareCLIPlainAndJSON(t *testing.T) {
	for _, format := range []string{"human", "json"} {
		t.Run(format, func(t *testing.T) {
			client := fixtureClient(t, domain.ClientKiro)
			client.ExecutablePath = "/fixture/kiro-cli"
			fixture := newCLIFixture(t, []domain.DetectedClient{client})
			fixture.app.Lifecycle.Activator = providers.Activator{}
			plugin := writeCLIPlugin(t)
			writeCLIMCP(t, plugin)
			out, _, err := fixture.execute(false, "add", plugin, "--target", "kiro", "--prepare", "--format", format)
			if err != nil {
				t.Fatalf("%s: %v", out, err)
			}
			if !strings.Contains(out, "prepared") || !strings.Contains(out, "Runtime connections have not been verified") {
				t.Fatalf("missing honest guidance: %s", out)
			}
			if format == "json" {
				var response struct {
					Data addMultiResult `json:"data"`
				}
				if err := json.Unmarshal([]byte(out), &response); err != nil {
					t.Fatal(err)
				}
				if len(response.Data.Targets) != 1 {
					t.Fatalf("missing target: %s", out)
				}
				result := response.Data.Targets[0].Output.Result
				if result.Plan.InstallIntent != domain.InstallIntentPrepare || result.Activation.Activation != domain.ActivationPrepared || result.Activation.Verification != domain.VerificationPackageValid || result.Activation.ActivationAttested {
					t.Fatalf("false JSON claim: %s", out)
				}
			}
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			for _, binding := range state.Installations[0].Clients {
				if binding.InstallIntent != domain.InstallIntentPrepare {
					t.Fatal("lost intent")
				}
			}
			out, _, err = fixture.execute(false, "add", plugin, "--target", "kiro", "--format", format)
			if err != nil || !strings.Contains(out, "prepared") {
				t.Fatalf("repeat %s: %v", out, err)
			}
			if _, err := os.Stat(filepath.Join(client.ConfigRoot, "settings", "mcp.json")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPrepareRejectsInvalidTargetsBeforeAcquisition(t *testing.T) {
	for _, target := range []string{"cursor", "kiro,cursor", ""} {
		t.Run(target, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			_, _, err := fixture.execute(false, "add", "context7", "--target", target, "--prepare")
			if err == nil || !strings.Contains(err.Error(), "--prepare requires --target kiro") {
				t.Fatalf("invalid target: %v", err)
			}
			state, _ := fixture.store.Load()
			if len(state.Installations) != 0 {
				t.Fatal("mutated")
			}
		})
	}
}

func TestDetectedKiroOffersExplicitPreparationWithoutChangingOtherTargets(t *testing.T) {
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/fixture/kiro-cli"
	cursor := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{kiro, cursor})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	loaded, err := fixture.app.loadPackageFor(context.Background(), plugin, fixture.app.addResolutionRequest(plugin, nil))
	if err != nil {
		t.Fatal(err)
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	intents := map[domain.ClientID]domain.InstallIntent{}
	eligible, skipped := fixture.app.compatibleLoadedTargets(context.Background(), loaded, []domain.DetectedClient{kiro, cursor}, intents)
	if len(eligible) != 2 || len(skipped) != 0 || intents[domain.ClientKiro] != domain.InstallIntentPrepare || intents[domain.ClientCursor] != "" {
		t.Fatalf("selection: %+v %+v %+v", eligible, skipped, intents)
	}
	if !strings.Contains(eligible[0].DisplayName, "prepare configuration") {
		t.Fatalf("unlabelled preparation: %+v", eligible)
	}
}

func TestInteractiveKiroPreparationPersistsThroughConfirmedSelection(t *testing.T) {
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/fixture/kiro-cli"
	cursor := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{kiro, cursor})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	out, _, err := fixture.executeInput(true, "\ny\n", "add", plugin, "--plain")
	if err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("state %+v: %v; %s", state, err, out)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID == string(domain.ClientKiro) {
			if binding.InstallIntent != domain.InstallIntentPrepare {
				t.Fatal("Kiro was not prepared")
			}
		} else if binding.InstallIntent != "" {
			t.Fatal("other target intent changed")
		}
	}
	if len(state.Installations[0].Clients) != 2 {
		t.Fatalf("missing selected target: %s", out)
	}
}

func TestPreparedLifecycleVersionDetectionNeverLaunchesKiro(t *testing.T) {
	kiro := fixtureClient(t, domain.ClientKiro)
	kiro.ExecutablePath = "/fixture/kiro-cli"
	fixture := newCLIFixture(t, []domain.DetectedClient{kiro})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	if out, _, err := fixture.execute(false, "add", plugin, "--target", "kiro", "--prepare"); err != nil {
		t.Fatalf("%s: %v", out, err)
	}
	detector := &observedProbingDetector{clients: []domain.DetectedClient{kiro, fixtureClient(t, domain.ClientCursor)}}
	fixture.app.Detector = detector
	intents, err := fixture.app.addLifecycleIntents(context.Background(), plugin, string(domain.ScopeUser), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.detectForLifecycle(context.Background(), true, intents); err != nil {
		t.Fatal(err)
	}
	if detector.probeCalls != 0 || len(detector.targets) != 1 || detector.targets[0] != domain.ClientCursor {
		t.Fatalf("unsafe version probes: %+v", detector)
	}
}

// This exercises the normal Directory boundary with disposable fixture data;
// it does not attest the live catalog or contact the OAuth endpoint.
func TestKiroPreparationUsesNormalDirectoryResolution(t *testing.T) {
	for _, supported := range []bool{true, false} {
		t.Run(map[bool]string{true: "supported", false: "unsupported"}[supported], func(t *testing.T) {
			targets := []domain.ClientID{domain.ClientCursor}
			if supported {
				targets = append(targets, domain.ClientKiro)
			}
			rollout := newRolloutDirectoryFixture(t, targets, targets)
			kiro := fixtureClient(t, domain.ClientKiro)
			kiro.ExecutablePath = "/fixture/kiro-cli"
			detector := &observedProbingDetector{clients: []domain.DetectedClient{kiro}}
			rollout.cli.app.Detector = detector
			const mcp = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://mcp.context7.com/mcp/oauth"}}}`
			if err := os.WriteFile(filepath.Join(rollout.v1.root, "mcp.json"), []byte(mcp), 0600); err != nil {
				t.Fatal(err)
			}
			loaded, err := rollout.cli.app.acquireLocal(context.Background(), rollout.v1.root)
			if err != nil {
				t.Fatal(err)
			}
			release := &rollout.directory.bundle.Snapshot.Distributions[0].Releases[0]
			release.TreeDigest = loaded.envelope.TreeDigest
			release.ManifestDigest = loaded.envelope.ManifestDigest
			for i := range rollout.directory.bundle.Snapshot.Evidence {
				evidence := &rollout.directory.bundle.Snapshot.Evidence[i]
				if evidence.ReleaseSequence == 1 {
					evidence.PackageTreeDigest = release.TreeDigest
				}
			}
			if err := loaded.cleanup(); err != nil {
				t.Fatal(err)
			}
			rollout.directory.bundle.Snapshot.Products[0].Aliases = append(rollout.directory.bundle.Snapshot.Products[0].Aliases, "context7")
			out, _, err := rollout.cli.execute(false, "add", "context7", "--target", "kiro", "--prepare", "--format", "json")
			if !supported {
				if err == nil || rollout.acquirer.verifiedCalls != 0 {
					t.Fatalf("bypassed compatibility: %s %v", out, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%s: %v", out, err)
			}
			if rollout.acquirer.verifiedCalls != 1 || rollout.acquirer.directGitCalls != 0 || detector.probeCalls != 0 || detector.targetedCalls != 0 {
				t.Fatalf("wrong acquisition/probes: %+v %+v", rollout.acquirer, detector)
			}
			state, err := rollout.cli.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if state.Installations[0].OriginMode != domain.OriginModeDirectory || state.Installations[0].Source.ResolvedRevision != rollout.v1.revision {
				t.Fatal("lost exact Directory origin")
			}
			body, err := os.ReadFile(filepath.Join(kiro.ConfigRoot, "settings", "mcp.json"))
			if err != nil || !strings.Contains(string(body), "https://mcp.context7.com/mcp/oauth") {
				t.Fatalf("OAuth URL changed: %s %v", body, err)
			}
			if out, _, err := rollout.cli.execute(false, "repair", "demo", "--target", "kiro", "--format", "json"); err != nil {
				t.Fatalf("repair %s: %v", out, err)
			}
			if detector.probeCalls != 0 || detector.targetedCalls != 0 {
				t.Fatal("prepared repair executed Kiro version probe")
			}
			// A new alias must recover the same binding intent before probing.
			rollout.directory.bundle.Snapshot.Distributions[0].Releases = rollout.directory.bundle.Snapshot.Distributions[0].Releases[:1]
			rollout.directory.bundle.Snapshot.Distributions[0].ReleasePolicies = rollout.directory.bundle.Snapshot.Distributions[0].ReleasePolicies[:1]
			rollout.directory.bundle.Snapshot.Products[0].Aliases = append(rollout.directory.bundle.Snapshot.Products[0].Aliases, "new-context-alias")
			for _, args := range [][]string{
				{"add", "new-context-alias", "--target", "kiro"},
				{"update", "demo", "--target", "kiro"},
				{"remove", "demo", "--target", "kiro"},
				{"add", "new-context-alias", "--target", "kiro"},
			} {
				if out, _, err := rollout.cli.execute(false, args...); err != nil {
					t.Fatalf("%v: %s %v", args, out, err)
				}
				if detector.probeCalls != 0 || detector.targetedCalls != 0 {
					t.Fatalf("prepared operation %v executed Kiro version probe", args)
				}
			}
		})
	}
}
