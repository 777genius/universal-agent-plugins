package agentpluginscli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type cliPreparedHost struct {
	contracttest.OpenCodeV1Host
	token []byte
}

type cliPreparationFixture struct {
	calls, baselines, probes int
	driftOnApply             bool
}

func (p *cliPreparationFixture) PrepareClient(ctx context.Context, _ domain.PackageEnvelope, client domain.DetectedClient, _ []domain.NativeObjectOwnership, _ string, inert bool) (domain.OpenCodeHostAuthority, error) {
	if client.ClientID != domain.ClientOpenCode {
		return client.OpenCodeHost, nil
	}
	p.calls++
	if inert && client.OpenCodeHost == nil {
		return nil, fmt.Errorf("offline qualified host required")
	}
	if p.calls == 2 && p.driftOnApply {
		if err := os.WriteFile(client.ExecutablePath, []byte("qualified-V2-after-consent"), 0o600); err != nil {
			return nil, err
		}
	}
	if _, ok := client.OpenCodeHost.(*cliPreparedHost); ok {
		if !inert {
			if err := p.RevalidateClient(ctx, client, domain.DeliveryPlan{}); err != nil {
				return nil, err
			}
		}
		return client.OpenCodeHost, nil
	}
	if inert {
		return client.OpenCodeHost, nil
	}
	p.probes++
	token, err := os.ReadFile(client.ExecutablePath)
	if err != nil {
		return nil, err
	}
	p.baselines++
	return &cliPreparedHost{token: token}, nil
}

func (p *cliPreparationFixture) RevalidateClient(_ context.Context, client domain.DetectedClient, _ domain.DeliveryPlan) error {
	if client.ClientID != domain.ClientOpenCode {
		return nil
	}
	host, ok := client.OpenCodeHost.(*cliPreparedHost)
	if !ok {
		return fmt.Errorf("frozen snapshot missing")
	}
	token, err := os.ReadFile(client.ExecutablePath)
	if err != nil || !bytes.Equal(token, host.token) {
		return clientdetect.ErrProbeTargetChanged
	}
	return nil
}

func openCodePreparationCLI(t *testing.T, grouped bool) (cliFixture, *cliPreparationFixture, string, string) {
	t.Helper()
	openCode := fixtureClient(t, domain.ClientOpenCode)
	// This consumer starts without prepared authority, even when the shared
	// client fixture supplies a qualified profile for unrelated SDK tests.
	openCode.OpenCodeHost = nil
	openCode.ExecutablePath = filepath.Join(t.TempDir(), "inert-native-token")
	if err := os.WriteFile(openCode.ExecutablePath, []byte("qualified-V1-before-consent"), 0o600); err != nil {
		t.Fatal(err)
	}
	clients := []domain.DetectedClient{openCode}
	target := "opencode"
	if grouped {
		clients = append(clients, fixtureClient(t, domain.ClientCursor))
		target += ",cursor"
	}
	fixture := newCLIFixture(t, clients)
	preparation := &cliPreparationFixture{}
	fixture.app.Lifecycle.ClientPreparation = preparation
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	return fixture, preparation, plugin, target
}

// The public CLI performs a read-only preview before apply. A missing initial
// snapshot is prepared once for normal operations; true --dry-run never probes.
func TestOpenCodeCLINormalPreviewPreparesOnceAndDryRunNeverProbes(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprintf("group_%t", grouped), func(t *testing.T) {
			fixture, preparer, plugin, target := openCodePreparationCLI(t, grouped)
			if _, _, err := fixture.execute(false, "add", plugin, "--target", target, "--dry-run", "--format", "json"); err == nil || !strings.Contains(err.Error(), "offline qualified host") {
				t.Fatalf("unprepared offline CLI accepted: %v", err)
			}
			if preparer.probes != 0 {
				t.Fatalf("dry-run probed: %d", preparer.probes)
			}
			preparer.calls = 0
			if _, stderr, err := fixture.execute(false, "add", plugin, "--target", target, "--format", "json"); err != nil {
				t.Fatalf("normal preview/apply failed: %v, %s", err, stderr)
			}
			if preparer.baselines != 1 {
				t.Fatalf("operation selected %d baselines", preparer.baselines)
			}
			state, err := fixture.store.Load()
			if err != nil || len(state.Installations) != 1 {
				t.Fatalf("normal operation did not commit: %+v, %v", state, err)
			}
		})
	}
}

// Changing to another qualified version after the reviewed preview must refuse
// apply. Re-preparing a fresh baseline on the second invocation fails this test.
func TestOpenCodeCLIApplyRejectsDriftFromReviewedSnapshot(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprintf("group_%t", grouped), func(t *testing.T) {
			fixture, preparer, plugin, target := openCodePreparationCLI(t, grouped)
			preparer.driftOnApply = true
			if _, _, err := fixture.execute(false, "add", plugin, "--target", target, "--format", "json"); !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
				t.Fatalf("CLI apply accepted replacement baseline: %v", err)
			}
			if preparer.baselines != 1 {
				t.Fatalf("consented operation rebaselined %d times", preparer.baselines)
			}
			state, err := fixture.store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("refused CLI mutated state: %+v, %v", state, err)
			}
		})
	}
}
