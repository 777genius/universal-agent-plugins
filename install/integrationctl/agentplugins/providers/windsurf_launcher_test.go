package providers

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"os"
	"path/filepath"
	"testing"
)

func TestWindsurfHelperOwnedArtifactDigestAndCollision(t *testing.T) {
	stager := windsurfFixtureStager(t)
	envelope := windsurfTestEnvelope(t, "helper")
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}
	if err := stager.PreflightManagedStdio(envelope.SnapshotRoot); err != nil {
		t.Fatal(err)
	}
	delivery, err := stager.Stage(context.Background(), envelope, plan, "helper", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(delivery.StagingPath, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
	if _, err := os.Stat(helper); err != nil {
		t.Fatal(err)
	}
	if err := stager.Verify(context.Background(), delivery.StagingPath, delivery.ArtifactDigest); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(helper, []byte("tampered"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := stager.Verify(context.Background(), delivery.StagingPath, delivery.ArtifactDigest); err == nil {
		t.Fatal("helper tamper was not part of artifact digest")
	}
	reserved := filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(managedstdio.RelativeDirectory))
	writeTestFile(t, reserved, "authored")
	if err := stager.PreflightManagedStdio(envelope.SnapshotRoot); err == nil {
		t.Fatal("authored collision accepted")
	}
	if _, err := stager.Stage(context.Background(), envelope, plan, "collision-helper", domain.CompatibilityHints{}); err == nil {
		t.Fatal("late collision accepted")
	}
	body, _ := os.ReadFile(reserved)
	if string(body) != "authored" {
		t.Fatal("authored collision changed")
	}
}

func TestWindsurfHelperChangeChangesArtifactWithoutSourceMutation(t *testing.T) {
	envelope := windsurfTestEnvelope(t, "pins")
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}
	stager := windsurfFixtureStager(t)
	first, err := stager.Stage(context.Background(), envelope, plan, "first-pin", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "cli-b")
	if err := os.WriteFile(path, []byte("fixture-B"), 0700); err != nil {
		t.Fatal(err)
	}
	stager.LauncherSource, err = managedstdio.NewSource(path, "B")
	if err != nil {
		t.Fatal(err)
	}
	second, err := stager.Stage(context.Background(), envelope, plan, "second-pin", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	if first.ArtifactDigest == second.ArtifactDigest {
		t.Fatal("helper version absent from artifact digest")
	}
	if err := stager.Verify(context.Background(), first.StagingPath, first.ArtifactDigest); err != nil {
		t.Fatal("old immutable artifact changed", err)
	}
	if _, err := os.Stat(filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(managedstdio.RelativeDirectory))); !os.IsNotExist(err) {
		t.Fatal("source snapshot mutated")
	}
}
