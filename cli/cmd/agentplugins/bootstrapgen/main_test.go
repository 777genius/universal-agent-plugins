package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

func TestRunGeneratesChecksAndRejectsUnusableReleaseBootstrap(t *testing.T) {
	snapshot, envelope, trust, keyID, publicKey := writeGeneratorFixture(t)
	generated := filepath.Join(t.TempDir(), "directory_bootstrap_generated.go")
	base := []string{"-snapshot", snapshot, "-envelope", envelope, "-trust", trust, "-expected-key-id", keyID, "-expected-public-key", publicKey}
	if err := run(append(append([]string{}, base...), "-output", generated, "-release-at", "2026-08-21T00:00:00Z")); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(first), "Sequence:       41,") || !strings.Contains(string(first), "SnapshotBase64:") || !strings.Contains(string(first), "EnvelopeBase64:") {
		t.Fatalf("generated bootstrap does not contain the verified nonzero tuple:\n%s", first)
	}
	if err := run(append(append([]string{}, base...), "-check", generated, "-release-at", "2026-08-21T00:00:00Z")); err != nil {
		t.Fatalf("reproducibility check: %v", err)
	}
	if err := os.WriteFile(generated, append(first, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(append(append([]string{}, base...), "-check", generated)); err == nil || !strings.Contains(err.Error(), "not reproducibly generated") {
		t.Fatalf("inconsistent generated source error = %v", err)
	}
	if err := run(append(append([]string{}, base...), "-output", filepath.Join(t.TempDir(), "expired.go"), "-release-at", "2026-09-19T11:00:00Z")); err == nil || !strings.Contains(err.Error(), "not valid for release") {
		t.Fatalf("expired release input error = %v", err)
	}
	wrongBinding := append([]string{}, base...)
	wrongBinding[len(wrongBinding)-1] = base64.StdEncoding.EncodeToString(make([]byte, ed25519.PublicKeySize))
	if err := run(append(wrongBinding, "-output", filepath.Join(t.TempDir(), "wrong.go"))); err == nil || !strings.Contains(err.Error(), "compiled production key binding") {
		t.Fatalf("wrong compiled key binding error = %v", err)
	}
	if err := run([]string{"-snapshot", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("missing/unsigned bootstrap inputs were accepted")
	}
}

func writeGeneratorFixture(t *testing.T) (snapshotPath, envelopePath, trustPath, keyID, publicKeyBase64 string) {
	t.Helper()
	root := t.TempDir()
	keyID = "release-test-key"
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = 29
	}
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKeyBase64 = base64.StdEncoding.EncodeToString(privateKey.Public().(ed25519.PublicKey))
	snapshot := []byte(`{"snapshot_schema_version":1,"sequence":41,"publication_id":"release-test-41","source_commit":"abcdefabcdefabcdefabcdefabcdefabcdefabcd","generated_at":"2026-08-20T11:00:00Z","expires_at":"2026-09-19T11:00:00Z","products":[{"schema_version":1,"id":"tool","display_name":"Tool","description":"Tool description","manifest_name":"tool","aliases":["tool"],"reserved_aliases":["tool"],"categories":["tools"],"minimum_capabilities":{"skills":"optional","mcp":"required"},"default_distribution":"owner/tool","distributions":["owner/tool"]}],"distributions":[{"schema_version":1,"id":"owner/tool","product_id":"tool","kind":"upstream","status":"active","packager":"owner","releases":[{"sequence":1,"package_version":"1.0.0","manifest_name":"tool","agent_plugins_schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","package_source":{"repository":"owner/repo","revision":"0123456789012345678901234567890123456789","path":"plugin"},"tree_digest_algorithm":"agentplugins-tree-sha256-v1","tree_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","manifest_digest":"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","components":["mcp"],"published_at":"2026-08-20T10:00:00Z"}],"release_policies":[{"release_sequence":1,"status":"active","minimum_installer_version":"1.0.0","targets":[{"client":"cursor","scopes":["user"],"delivery":"managed","authentication":"not_required"}],"current_evidence":[]}]}],"evidence":[],"revocations":[]}` + "\n")
	digest := sha256.Sum256(snapshot)
	signature := ed25519.Sign(privateKey, directoryv1.SnapshotSignatureMessage(snapshot))
	envelope, err := json.Marshal(directoryv1.Envelope{EnvelopeSchemaVersion: 1, SnapshotSchemaVersion: 1, Sequence: 41, KeyID: keyID, Algorithm: "Ed25519", SignatureDomain: directoryv1.SignatureDomain, SnapshotDigest: "sha256:" + hex.EncodeToString(digest[:]), Signature: base64.StdEncoding.EncodeToString(signature)})
	if err != nil {
		t.Fatal(err)
	}
	snapshotPath = filepath.Join(root, "snapshot.json")
	envelopePath = filepath.Join(root, "envelope.json")
	trustPath = filepath.Join(root, "trusted-keys.json")
	trust := []byte(fmt.Sprintf(`{"schema_version":1,"keys":[{"key_id":%q,"public_key":%q}]}`, keyID, publicKeyBase64))
	for path, body := range map[string][]byte{snapshotPath: snapshot, envelopePath: envelope, trustPath: trust} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return snapshotPath, envelopePath, trustPath, keyID, publicKeyBase64
}
