package directoryv1

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func v2SignedFixture(t *testing.T) ([]byte, []byte, TrustStore) {
	t.Helper()
	key, private := fixtureKey("v2-test", KeyCurrent, 18)
	snapshot := fixtureSnapshot(15)
	snapshot.SnapshotSchemaVersion, snapshot.Sequence = 2, 1
	for i := range snapshot.Distributions {
		distribution := &snapshot.Distributions[i]
		distribution.SchemaVersion = 2
		for j := range distribution.ReleasePolicies {
			policy := &distribution.ReleasePolicies[j]
			for k := range policy.Targets {
				grant := &policy.Targets[k]
				grant.Scope, grant.Scopes = domain.ScopeUser, nil
				grant.MinimumInstallerVersion = policy.MinimumInstallerVersion
				grant.CurrentEvidence = append([]string(nil), policy.CurrentEvidence...)
			}
		}
	}
	snapshotBytes, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(snapshotBytes)
	envelope := Envelope{EnvelopeSchemaVersion: 2, SnapshotSchemaVersion: 2,
		Sequence: 1, KeyID: key.ID, Algorithm: "Ed25519", SignatureDomain: SignatureDomainV2,
		SnapshotDigest: "sha256:" + hex.EncodeToString(sum[:]),
		Signature:      base64.StdEncoding.EncodeToString(ed25519.Sign(private, SnapshotSignatureMessageV2(snapshotBytes))),
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return snapshotBytes, envelopeBytes, TrustStore{Keys: []TrustedKey{key}}
}

func TestV2VerifyExactSignedBundleAndV1Isolation(t *testing.T) {
	snapshot, envelope, trust := v2SignedFixture(t)
	bundle, err := VerifyV2Bundle(snapshot, envelope, trust)
	if err != nil || bundle.Snapshot.Sequence != 1 || bundle.Snapshot.SnapshotSchemaVersion != 2 {
		t.Fatalf("valid v2 signature rejected: %+v %v", bundle, err)
	}
	if _, err := VerifyBundle(snapshot, envelope, trust); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v1 parser accepted signed v2 bytes: %v", err)
	}
	corrupt := append([]byte(nil), snapshot...)
	corrupt[len(corrupt)-2] ^= 1
	if _, err := VerifyV2Bundle(corrupt, envelope, trust); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("v2 tamper not rejected before decode: %v", err)
	}
	unknown := TrustStore{Keys: []TrustedKey{trust.Keys[0]}}
	unknown.Keys[0].ID = "unknown"
	if _, err := VerifyV2Bundle(snapshot, envelope, unknown); !errors.Is(err, ErrUnknownKey) {
		t.Fatalf("v2 signing key changed without refusal: %v", err)
	}
}

func TestV2RejectsScopeOrGrantDowngrade(t *testing.T) {
	snapshot, _, _ := v2SignedFixture(t)
	var raw map[string]any
	if err := json.Unmarshal(snapshot, &raw); err != nil {
		t.Fatal(err)
	}
	distribution := raw["distributions"].([]any)[0].(map[string]any)
	policy := distribution["release_policies"].([]any)[0].(map[string]any)
	targets := policy["targets"].([]any)
	grant := targets[0].(map[string]any)
	grant["scope"] = "workspace"
	invalid, _ := json.Marshal(raw)
	if _, err := ParseV2Snapshot(invalid); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("unknown v2 scope accepted: %v", err)
	}
	grant["scope"] = "user"
	grant["minimum_installer_version"] = "0.0.1"
	invalid, _ = json.Marshal(raw)
	if _, err := ParseV2Snapshot(invalid); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("downgraded v2 grant floor accepted: %v", err)
	}
	grant["minimum_installer_version"] = policy["minimum_installer_version"]
	grant["scopes"] = []string{"project"}
	invalid, _ = json.Marshal(raw)
	if _, err := ParseV2Snapshot(invalid); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v2 grant borrowed legacy scopes: %v", err)
	}
}

func TestV2RejectsMissingImmutablePackageVersion(t *testing.T) {
	snapshot, _, _ := v2SignedFixture(t)
	for _, tc := range []struct {
		name   string
		mutate func(map[string]any)
		valid  bool
	}{
		{"omitted", func(release map[string]any) { delete(release, "package_version") }, false},
		{"explicit empty", func(release map[string]any) { release["package_version"] = "" }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var raw map[string]any
			if err := json.Unmarshal(snapshot, &raw); err != nil {
				t.Fatal(err)
			}
			distribution := raw["distributions"].([]any)[0].(map[string]any)
			release := distribution["releases"].([]any)[0].(map[string]any)
			tc.mutate(release)
			payload, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			_, err = ParseV2Snapshot(payload)
			if tc.valid && err != nil || !tc.valid && !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("v2 package_version %s validity=%t: %v", tc.name, tc.valid, err)
			}
		})
	}
}

func TestV2RejectsInvalidRawUTF8InArtifactPath(t *testing.T) {
	signed, _, _ := v2SignedFixture(t)
	var original domain.DirectorySnapshot
	if err := json.Unmarshal(signed, &original); err != nil {
		t.Fatal(err)
	}
	if len(original.Evidence) == 0 || original.Evidence[0].Artifact.Path == "" {
		t.Fatal("fixture lacks a v2 artifact path")
	}
	path, err := json.Marshal(original.Evidence[0].Artifact.Path)
	if err != nil {
		t.Fatal(err)
	}
	marker := append([]byte("\"path\":"), path...)
	position := bytes.Index(signed, marker)
	if position < 0 {
		t.Fatal("v2 artifact path not present in signed wire")
	}
	invalid := append([]byte(nil), signed...)
	invalid[position+len("\"path\":\"")] = 0xff
	if !json.Valid(invalid) {
		t.Fatal("invalid UTF-8 negative control must remain JSON syntax-valid")
	}
	var normalized domain.DirectorySnapshot
	if err := json.Unmarshal(invalid, &normalized); err != nil {
		t.Fatal("encoding/json was expected to repair the invalid byte", err)
	}
	if _, err := ParseV2Snapshot(invalid); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v2 parser accepted raw invalid UTF-8 in an artifact path: %v", err)
	}
}
