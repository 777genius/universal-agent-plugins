package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSnapshotRequestPreservesAssessedExecutableInventory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "helper"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, files := range [][]string{{}, {"bin/helper"}} {
		original, err := (packagedigest.Builder{TempRoot: t.TempDir()}).SnapshotWithExecutables(ctx, root, domain.SourceIdentity{}, files)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = packagedigest.Remove(original) })
		req := Request{PackageRoot: original.Root, ExecutableFiles: files}
		copied, err := snapshotRequestPackage(ctx, t.TempDir(), req)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = packagedigest.Remove(copied) })
		if copied.TreeDigest != original.TreeDigest {
			t.Fatalf("snapshot digest changed: %s != %s", copied.TreeDigest, original.TreeDigest)
		}
		engine := &Engine{}
		assessment := Assessment{TreeDigest: original.TreeDigest, Outcome: AssessmentAllow}
		if err := engine.assessSnapshot(ctx, copied, &assessment); err != nil {
			t.Fatal(err)
		}
		assessment.TreeDigest = "sha256:wrong"
		if err := engine.assessSnapshot(ctx, copied, &assessment); !errors.Is(err, ErrAssessmentRejected) {
			t.Fatalf("mismatched assessment accepted: %v", err)
		}
	}
}

func TestSnapshotRequestRejectsInvalidExecutableInventory(t *testing.T) {
	for _, relative := range []string{"", ".", "../outside", "/outside", `bin\helper`, "bin/../helper", "missing"} {
		_, err := snapshotRequestPackage(context.Background(), t.TempDir(), Request{PackageRoot: t.TempDir(), ExecutableFiles: []string{relative}})
		if !errors.Is(err, ErrInvalidRequest) {
			t.Fatalf("path %q: %v", relative, err)
		}
	}
}

func TestAcquiredSnapshotRequestPreservesVerifiedProvenanceAndInventory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"acquired"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	source := domain.SourceIdentity{RequestedSource: "github:acme/tools@0123456789012345678901234567890123456789//pkg",
		CanonicalSource: "github:acme/tools@0123456789012345678901234567890123456789//pkg", Repository: "acme/tools",
		PackageSubpath: "pkg", ResolvedRevision: "0123456789012345678901234567890123456789", SourceBindingHint: "uap-dependency-git"}
	original, err := (packagedigest.Builder{TempRoot: t.TempDir()}).SnapshotWithExecutables(ctx, root, source, []string{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(original) }()
	acquired := &domain.AcquiredSnapshot{TreeDigest: original.TreeDigest, DigestAlgorithm: original.DigestAlgorithm,
		ExecutableFiles: []string{}, Source: source, OriginMode: domain.OriginModeDirect}
	copied, err := snapshotRequestPackage(ctx, t.TempDir(), Request{PackageRoot: root, AcquiredSnapshot: acquired})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(copied) }()
	if copied.Source != source || copied.TreeDigest != original.TreeDigest || copied.ExecutableFiles == nil ||
		len(copied.ExecutableFiles) != 0 || copied.OriginMode != domain.OriginModeDirect {
		t.Fatalf("acquired identity was rewritten: %+v", copied)
	}
	acquired.Source.ResolvedRevision = strings.Repeat("9", 40)
	if _, err := snapshotRequestPackage(ctx, t.TempDir(), Request{PackageRoot: root, AcquiredSnapshot: acquired}); err == nil {
		t.Fatal("changed acquired provenance was accepted")
	}
}

func TestPreparedDeliveryPresentationIsCopied(t *testing.T) {
	prepared := &PreparedOperation{plan: Plan{Delivery: DeliveryPlan{Components: []ComponentDecision{{Name: "fixture"}}, UserActions: []string{"action"}, LocalActions: []string{"local"}, Warnings: []string{"warning"}, Diagnostics: []PlanDiagnostic{{Code: "code"}}}}}
	plan := prepared.Plan()
	plan.Delivery.Components[0].Name = "changed"
	plan.Delivery.UserActions[0] = "changed"
	plan.Delivery.LocalActions[0] = "changed"
	plan.Delivery.Warnings[0] = "changed"
	plan.Delivery.Diagnostics[0].Code = "changed"
	got := prepared.Plan().Delivery
	if got.Components[0].Name != "fixture" || got.UserActions[0] != "action" || got.LocalActions[0] != "local" || got.Warnings[0] != "warning" || got.Diagnostics[0].Code != "code" {
		t.Fatalf("presentation aliases handle: %+v", got)
	}
}

func TestCloneAcquiredRequestRetainsEmptyExecutablesAndOwnsCompatibility(t *testing.T) {
	original := Request{AcquiredSnapshot: &domain.AcquiredSnapshot{ExecutableFiles: []string{}, Compatibility: domain.CompatibilityHints{Compatibility: map[string]domain.CatalogCompatibility{"codex": {EvidenceOutcomes: map[string]string{"a": "before"}, AppBinding: &domain.CatalogAppBinding{AppKey: "before"}}}}}}
	copied := clonePrepareRequest(original)
	item := original.AcquiredSnapshot.Compatibility.Compatibility["codex"]
	item.EvidenceOutcomes["a"] = "after"
	item.AppBinding.AppKey = "after"
	got := copied.AcquiredSnapshot.Compatibility.Compatibility["codex"]
	if got.EvidenceOutcomes["a"] != "before" || got.AppBinding.AppKey != "before" || copied.AcquiredSnapshot.ExecutableFiles == nil {
		t.Fatalf("caller retained mutable prepared request: %+v", copied.AcquiredSnapshot)
	}
}
