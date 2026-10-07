package usecase

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/contracttest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// This trusted port fixture freezes an inert executable token instead of
// launching a client. Provider tests separately exercise the real native probe.
type tokenClientPreparation struct {
	prepared, processCalls int
	token                  []byte
}

func (p *tokenClientPreparation) PrepareClient(_ context.Context, _ domain.PackageEnvelope, client domain.DetectedClient, _ []domain.NativeObjectOwnership, executable string, inert bool) (domain.OpenCodeHostAuthority, error) {
	if client.ClientID != domain.ClientOpenCode {
		return client.OpenCodeHost, nil
	}
	p.prepared++
	if inert {
		if client.OpenCodeHost == nil {
			return nil, fmt.Errorf("offline qualified host required")
		}
		return client.OpenCodeHost, nil
	}
	if client.OpenCodeHost != nil && p.token != nil {
		if err := p.RevalidateClient(context.Background(), client, domain.DeliveryPlan{}); err != nil {
			return nil, err
		}
		return client.OpenCodeHost, nil
	}
	p.processCalls++
	if executable == "" {
		executable = client.ExecutablePath
	}
	body, err := os.ReadFile(executable)
	if err != nil {
		return nil, err
	}
	p.token = body
	return contracttest.OpenCodeV1Host{}, nil
}

func (p *tokenClientPreparation) RevalidateClient(_ context.Context, client domain.DetectedClient, _ domain.DeliveryPlan) error {
	if client.ClientID != domain.ClientOpenCode {
		return nil
	}
	body, err := os.ReadFile(client.ExecutablePath)
	if err != nil || !bytes.Equal(body, p.token) {
		return clientdetect.ErrProbeTargetChanged
	}
	return nil
}

func preparationClient(t *testing.T) domain.DetectedClient {
	t.Helper()
	root := t.TempDir()
	executable := filepath.Join(root, "inert-executable-token")
	if err := os.WriteFile(executable, []byte("qualified-native-fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	return domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "opencode"), ExecutablePath: executable}
}

// A real grouped add must carry prepared authority into planning and native
// projection, rather than reaching the namespace guard with a nil profile.
func TestOpenCodeGroupPreparesMissingAuthorityBeforeNativeProjection(t *testing.T) {
	service, _, cursor := serviceFixture(t)
	preparer := &tokenClientPreparation{}
	service.ClientPreparation = preparer
	openCode := preparationClient(t)
	openInput := openCodePluginInput(t, openCode, "1.0.0", "sha256:prepared-group", "sha256:prepared-group-manifest", "sh")
	openInput.Client.OpenCodeHost = nil
	cursorInput := openCodePluginInput(t, cursor, "1.0.0", "sha256:prepared-group", "sha256:prepared-group-manifest", "sh")
	result, err := service.AddGroup(t.Context(), GroupInput{Targets: []AddInput{cursorInput, openInput}, OperationGroupID: "prepared-group", Confirmed: true})
	if err != nil || !result.Mutated || preparer.prepared != 1 {
		t.Fatalf("prepared grouped add = %+v, %v, preparations = %d", result, err, preparer.prepared)
	}
	body, err := os.ReadFile(filepath.Join(openCode.ConfigRoot, "opencode.json"))
	if err != nil || !bytes.Contains(body, []byte(`"docs"`)) {
		t.Fatalf("prepared group omitted real native MCP registration: %s, %v", body, err)
	}
}

func TestOpenCodeCompatibilityOnlyGroupDryRunNeverProbes(t *testing.T) {
	service, store, cursor := serviceFixture(t)
	preparer := &tokenClientPreparation{}
	service.ClientPreparation = preparer
	openCode := preparationClient(t)
	openInput := openCodePluginInput(t, openCode, "1.0.0", "sha256:dry-compat", "sha256:dry-compat-manifest", "sh")
	cursorInput := openCodePluginInput(t, cursor, "1.0.0", "sha256:dry-compat", "sha256:dry-compat-manifest", "sh")
	if _, err := service.AddGroup(t.Context(), GroupInput{Targets: []AddInput{cursorInput, openInput}, OperationGroupID: "dry-compat-add", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	preparer.processCalls = 0
	result, err := service.UpdateGroup(t.Context(), GroupInput{Targets: []AddInput{cursorInput}, CompatibilityChecks: []AddInput{openInput}, OperationGroupID: "dry-compat-update", DryRun: true})
	if err != nil || result.Mutated || preparer.processCalls != 0 {
		t.Fatalf("compatibility-only dry-run = %+v, %v, process probes = %d", result, err, preparer.processCalls)
	}
	after, err := os.ReadFile(store.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("dry-run changed installed state: %v", err)
	}
}

type driftAfterVerifyStager struct {
	providers.Stager
	executable string
}

func (s driftAfterVerifyStager) Verify(ctx context.Context, root, digest string) error {
	if err := s.Stager.Verify(ctx, root, digest); err != nil {
		return err
	}
	return os.WriteFile(s.executable, []byte("changed-after-repair-preparation"), 0o600)
}

// Intact package repair can skip identity observation. Drift after managed
// package verification must still prevent native reconstruction at the attempt.
func TestOpenCodeIntactRepairRefusesHostDriftBeforeNativeEffect(t *testing.T) {
	service, _, _ := serviceFixture(t)
	service.ClientPreparation = &tokenClientPreparation{}
	openCode := preparationClient(t)
	input := openCodePluginInput(t, openCode, "1.0.0", "sha256:repair-prepared", "sha256:repair-prepared-manifest", "sh")
	input.Confirmed = true
	if _, err := service.Add(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(openCode.ConfigRoot, "opencode.json")
	if err := os.Remove(config); err != nil {
		t.Fatal(err)
	}
	stager, ok := service.Stager.(providers.Stager)
	if !ok {
		t.Fatal("fixture stager type changed")
	}
	service.Stager = driftAfterVerifyStager{Stager: stager, executable: openCode.ExecutablePath}
	if _, err := service.Repair(t.Context(), input); !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
		t.Fatalf("intact repair accepted changed host: %v", err)
	}
	if _, err := os.Stat(config); !os.IsNotExist(err) {
		t.Fatalf("drifted repair reconstructed native config: %v", err)
	}
}

type driftAfterStageStager struct {
	providers.Stager
	executable string
	staged     *int
}

func (s driftAfterStageStager) Stage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints) (domain.StagedDelivery, error) {
	delivery, err := s.Stager.Stage(ctx, envelope, plan, operationID, hints)
	if err == nil {
		*s.staged++
		err = os.WriteFile(s.executable, []byte("changed-during-stage"), 0o600)
	}
	return delivery, err
}

func (s driftAfterStageStager) StageWithPluginData(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string, hints domain.CompatibilityHints, data string) (domain.StagedDelivery, error) {
	delivery, err := s.Stager.StageWithPluginData(ctx, envelope, plan, operationID, hints, data)
	if err == nil {
		*s.staged++
		err = os.WriteFile(s.executable, []byte("changed-during-stage"), 0o600)
	}
	return delivery, err
}

// Package repair and grouped repair must refuse changed authority after staging,
// before committing either the repaired package or any sibling package.
func TestOpenCodePackageRepairRejectsStageDriftWithoutCommit(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprintf("group_%t", grouped), func(t *testing.T) {
			service, store, cursor := serviceFixture(t)
			service.ClientPreparation = &tokenClientPreparation{}
			openCode := preparationClient(t)
			openInput := openCodePluginInput(t, openCode, "1.0.0", "sha256:stage-drift", "sha256:stage-drift-manifest", "sh")
			cursorInput := openCodePluginInput(t, cursor, "1.0.0", "sha256:stage-drift", "sha256:stage-drift-manifest", "sh")
			added, err := service.AddGroup(t.Context(), GroupInput{Targets: []AddInput{openInput, cursorInput}, OperationGroupID: "stage-drift-add", Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			// Damage the owned package so repair must stage and replace it.
			damaged := filepath.Join(added.Targets[0].Plan.ActivePath, "skills", "docs", "SKILL.md")
			if grouped {
				// Grouped repair accepts positively absent owned packages; corrupt
				// packages are indeterminate at its earlier identity guard. Use real
				// native ownership observation, retaining the native config/skills.
				service.NativeObserver = providerstest.NewObserver(providers.NativeIdentityObserver{Stager: service.Stager})
				if err := os.RemoveAll(added.Targets[0].Plan.ActivePath); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(damaged, []byte("damaged-owned-package"), 0o600); err != nil {
				t.Fatal(err)
			}
			sibling := filepath.Join(added.Targets[1].Plan.ActivePath, "skills", "docs", "SKILL.md")
			siblingBefore, err := os.ReadFile(sibling)
			if err != nil {
				t.Fatal(err)
			}
			stateBefore, err := os.ReadFile(store.Path)
			if err != nil {
				t.Fatal(err)
			}
			nativeBefore, err := os.ReadFile(filepath.Join(openCode.ConfigRoot, "opencode.json"))
			if err != nil {
				t.Fatal(err)
			}
			staged := 0
			service.Stager = driftAfterStageStager{Stager: service.Stager.(providers.Stager), executable: openCode.ExecutablePath, staged: &staged}
			if grouped {
				_, err = service.RepairGroup(t.Context(), GroupInput{Targets: []AddInput{openInput, cursorInput}, OperationGroupID: "stage-drift-repair", Confirmed: true, Repair: true})
			} else {
				openInput.Confirmed = true
				openInput.InstallationID = added.InstallationID
				_, err = service.Repair(t.Context(), openInput)
			}
			if staged == 0 {
				t.Fatalf("repair refused before staging, so host drift was not exercised: %v", err)
			}
			if !errors.Is(err, clientdetect.ErrProbeTargetChanged) {
				t.Fatalf("staging drift accepted: %v", err)
			}
			if grouped {
				if _, err := os.Lstat(added.Targets[0].Plan.ActivePath); !os.IsNotExist(err) {
					t.Fatalf("refused grouped repair recreated package: %v", err)
				}
			} else {
				after, err := os.ReadFile(damaged)
				if err != nil || !bytes.Equal(after, []byte("damaged-owned-package")) {
					t.Fatalf("refused repair changed damaged package: %v", err)
				}
			}
			for _, check := range []struct {
				path   string
				before []byte
			}{{store.Path, stateBefore}, {sibling, siblingBefore}, {filepath.Join(openCode.ConfigRoot, "opencode.json"), nativeBefore}} {
				after, err := os.ReadFile(check.path)
				if err != nil || !bytes.Equal(after, check.before) {
					t.Fatalf("refused repair changed %s: %v", check.path, err)
				}
			}
		})
	}
}
