package agentpluginscli

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

const piModernFixtureFeed = "test-only-pi-modern-v1"

// piMarketingCohortFixture models the exact four-package saved dependency shape
// of Marketing Kit, with independently committed immutable Git sources.
// Directory policies are TEST-ONLY preverified inputs. Cryptographic separation
// is tested at the real signed-v1 client boundary in PR #868; these fixtures
// must never become production publication/materialization evidence.
func piMarketingCohortFixture(t *testing.T, scope domain.InstallScope) (freshRestoreEnvironment, *fixedDirectoryClient, environment.Requirements) {
	t.Helper()
	fixture, repo, directory := dependencyReviewFixture(t)
	pi := fixtureClient(t, domain.ClientPi)
	fixture.client = pi
	fixture.app.Detector = staticDetector{clients: []domain.DetectedClient{pi}}
	fixture.app.Scope = scope
	if scope == domain.ScopeProject {
		fixture.app.ProjectRoot = t.TempDir()
	}

	for _, pkg := range []struct{ alias, version string }{
		{"notion", "0.1.0"},
		{"hubspot-crm", "0.1.0"},
		{"playwright-bridge", "0.0.80"},
		{"marketing-kit", "0.1.0"},
	} {
		freshRestorePackage(t, fixture, repo, pkg.alias, pkg.version, pkg.alias, "", 1, true)
	}
	root := directory.bundle.Snapshot.Distributions[3].Releases[0]
	dependencyReviewReviseRelease(t, fixture, repo, root, func(root string) {
		dependencyWrite(t, filepath.Join(root, "uap.toml"), `schema_version = 1
[dependencies]
"acme/notion" = "0.1.0"
"acme/hubspot-crm" = "0.1.0"
"acme/playwright-bridge" = "0.0.80"
`)
	})

	// The test-only modern cohort grants Pi on all four immutable releases.
	// The real legacy signed feed remains Pi-free.
	for i := range directory.bundle.Snapshot.Distributions {
		policy := &directory.bundle.Snapshot.Distributions[i].ReleasePolicies[0]
		if len(policy.Targets) != 1 {
			t.Fatalf("unexpected preverified policy: %+v", policy)
		}
		policy.Targets[0].Client = domain.ClientPi
	}
	for i := range directory.bundle.Snapshot.Evidence {
		directory.bundle.Snapshot.Evidence[i].Client = domain.ClientPi
		directory.bundle.Snapshot.Evidence[i].ClientVersion = pi.Version
	}
	directory.bundle = signPiModernTestOnlyCohort(t, directory.bundle.Snapshot)
	fixture.app.DirectoryClient = lockedFeedTestClient{
		fixedDirectoryClient: directory, feedID: piModernFixtureFeed,
	}
	requirements := environment.Requirements{
		SchemaVersion: 1,
		Dependencies: map[string]environment.Requirement{
			"marketing-kit": {
				Alias: "marketing-kit",
				Registry: &environment.RegistrySource{
					Distribution: "acme/marketing-kit",
					Selector: environment.Selector{
						Kind: environment.SelectorVersion, Value: "0.1.0",
					},
				},
			},
		},
		Targets: map[string][]string{"pi": {"marketing-kit"}},
	}
	return fixture, directory, requirements
}

func signPiModernTestOnlyCohort(t *testing.T, snapshot domain.DirectorySnapshot) directoryv1.VerifiedBundle {
	t.Helper()
	// All provenance is synthetic and confined to this disposable Go test.
	// These signatures are proof of v1 verification mechanics, never publisher
	// authority, release readiness or real materialization evidence.
	snapshot.Sequence = 41
	snapshot.PublicationID = "test-only-pi-modern-cohort"
	snapshot.GeneratedAt = "2026-11-12T00:00:00Z"
	snapshot.ExpiresAt = "2026-12-11T00:00:00Z"
	snapshot.Revocations = []domain.DirectoryRevocation{}
	for i := range snapshot.Products {
		snapshot.Products[i].Categories = []string{"tools"}
		snapshot.Products[i].MinimumCapabilities = domain.DirectoryMinimumCapabilities{
			Skills: "required", MCP: "optional",
		}
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	seed := bytes.Repeat([]byte{71}, ed25519.SeedSize)
	privateKey := ed25519.NewKeyFromSeed(seed)
	digest := sha256.Sum256(snapshotBytes)
	const keyID = "pi-test-cohort-only"
	envelope := directoryv1.Envelope{
		EnvelopeSchemaVersion: 1,
		SnapshotSchemaVersion: 1,
		Sequence: snapshot.Sequence,
		KeyID: keyID,
		Algorithm: "Ed25519",
		SignatureDomain: directoryv1.SignatureDomain,
		SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]),
		Signature: base64.StdEncoding.EncodeToString(
			ed25519.Sign(privateKey, directoryv1.SnapshotSignatureMessage(snapshotBytes))),
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: keyID, PublicKey: privateKey.Public().(ed25519.PublicKey), State: directoryv1.KeyCurrent,
	}}}
	verified, err := directoryv1.VerifyBundle(snapshotBytes, envelopeBytes, trust)
	if err != nil || verified.Source != directoryv1.BundleSourceVerified {
		t.Fatalf("test-only signed v1 cohort was not authenticated: %v", err)
	}
	return verified
}

// This is a source/graph/lock proof, NOT a signed-production or native Pi E2E.
// It exercises the real saved dependency solver and immutable acquisition;
// it cannot replace a test-only cryptographically verified cohort, real native
// receipts or a protected 72/72 modern Directory acceptance run.
func TestPiModernFixtureResolvesMarketingKitSavedClosure(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			fixture, _, requirements := piMarketingCohortFixture(t, scope)
			lock, err := resolveProjectSourceGraph(context.Background(), fixture.app, requirements)
			if err != nil {
				t.Fatalf("Pi saved dependency resolution (%s): %v", scope, err)
			}
			if len(lock.Packages) != 4 {
				t.Fatalf("Pi saved graph must contain Marketing Kit and 3 required children: %+v", lock)
			}
			wanted := map[string]string{
				"marketing-kit": "0.1.0",
				"notion": "0.1.0",
				"hubspot-crm": "0.1.0",
				"playwright-bridge": "0.0.80",
			}
			for _, pin := range lock.Packages {
				version, exists := wanted[pin.Alias]
				if !exists || pin.Version != version || len(pin.Targets) != 1 || pin.Targets[0] != "pi" ||
					pin.Source.Registry == nil || pin.Source.Registry.FeedID != piModernFixtureFeed ||
					pin.Source.Registry.ReleaseSequence != 1 || pin.TreeDigest == "" {
					t.Fatalf("missing Pi target, test-feed provenance or immutable release: %+v", pin)
				}
				loaded, err := materializeLockedPackage(context.Background(), fixture.app, pin, false, nil)
				if err != nil {
					t.Fatalf("verified immutable acquisition of %s: %v", pin.Alias, err)
				}
				if loaded.envelope.TreeDigest != pin.TreeDigest || loaded.envelope.ManifestDigest != pin.ManifestDigest {
					t.Fatalf("acquired %s differs from saved immutable pin", pin.Alias)
				}
				if err := loaded.cleanup(); err != nil {
					t.Fatal(err)
				}
				delete(wanted, pin.Alias)
			}
			if len(wanted) != 0 {
				t.Fatalf("Pi saved graph omitted required children: %v", wanted)
			}
		})
	}
}

func TestPiModernFixtureRefusesMissingSignedHubSpotTarget(t *testing.T) {
	for _, scope := range []domain.InstallScope{domain.ScopeUser, domain.ScopeProject} {
		t.Run(string(scope), func(t *testing.T) {
			fixture, directory, requirements := piMarketingCohortFixture(t, scope)
			for i := range directory.bundle.Snapshot.Distributions {
				distribution := &directory.bundle.Snapshot.Distributions[i]
				if distribution.ID == "acme/hubspot-crm" {
					distribution.ReleasePolicies[0].Targets = nil
				}
			}
			_, err := resolveProjectSourceGraph(context.Background(), fixture.app, requirements)
			if err == nil {
				t.Fatal("Marketing Kit accepted a mandatory HubSpot child without a Pi target")
			}
			state, stateErr := fixture.store.Load()
			if stateErr != nil || len(state.Installations) != 0 {
				t.Fatalf("denied Pi closure changed installed state: %+v, %v", state, stateErr)
			}
			if strings.Contains(err.Error(), "already installed") {
				t.Fatalf("incorrect fallback to unrelated installed dependency: %v", err)
			}
		})
	}
}
