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
)

func modernEnvelopeForBytes(t *testing.T, body, template []byte, private ed25519.PrivateKey) []byte {
	t.Helper()
	var envelope Envelope
	if err := json.Unmarshal(template, &envelope); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(body)
	envelope.SnapshotDigest = "sha256:" + hex.EncodeToString(digest[:])
	envelope.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, SnapshotSignatureMessage(body)))
	encoded, err := json.Marshal(envelope)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func TestModernReaderAuthenticatesBeforeDecodingFutureRecords(t *testing.T) {
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	body, template, _ := signedSnapshotFixture(t, futureClientSnapshot(41), key, private)
	body = bytes.Replace(body, []byte(`"future-test-agent"`), []byte(`"../unsafe"`), 1)
	_, otherPrivate := fixtureKey("untrusted-test-only", KeyCurrent, 35)
	untrusted := modernEnvelopeForBytes(t, body, template, otherPrivate)
	trust := NewModernTrustStore([]TrustedKey{key})
	if _, err := VerifyBundle(body, untrusted, trust); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("future records were interpreted before authentication: %v", err)
	}
	trusted := modernEnvelopeForBytes(t, body, template, private)
	if _, err := VerifyBundle(body, trusted, trust); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("authenticated malformed identity accepted: %v", err)
	}
	if _, err := VerifyBundle(body, trusted, NewModernTrustStore(nil)); !errors.Is(err, ErrNoTrustedKeys) {
		t.Fatalf("modern profile implicitly authorized a signing key: %v", err)
	}
}

func TestModernReaderRetainsStrictWireShapeForFutureRecords(t *testing.T) {
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	body, template, _ := signedSnapshotFixture(t, futureClientSnapshot(41), key, private)
	trust := NewModernTrustStore([]TrustedKey{key})
	targetIdentity := `"client":"future-test-agent","scopes"`
	evidenceIdentity := `"client":"future-test-agent","client_version"`
	cases := []struct{ name, old, replacement string }{
		{"unknown root field", `"snapshot_schema_version":1`, `"reader_profile":"modern","snapshot_schema_version":1`},
		{"duplicate root field", `"snapshot_schema_version":1`, `"snapshot_schema_version":1,"snapshot_schema_version":1`},
		{"target scoped field", targetIdentity, `"client":"future-test-agent","scope":null,"scopes"`},
		{"target closure field", targetIdentity, `"client":"future-test-agent","canonical_closure":null,"scopes"`},
		{"target evidence field", targetIdentity, `"client":"future-test-agent","current_evidence":null,"scopes"`},
		{"target floor field", targetIdentity, `"client":"future-test-agent","minimum_installer_version":null,"scopes"`},
		{"target unknown field", targetIdentity, `"client":"future-test-agent","unknown":true,"scopes"`},
		{"target null scopes", `"client":"future-test-agent","scopes":["user"]`, `"client":"future-test-agent","scopes":null`},
		{"evidence scoped field", evidenceIdentity, `"client":"future-test-agent","scope":null,"client_version"`},
		{"evidence closure field", evidenceIdentity, `"client":"future-test-agent","closure_digest":null,"client_version"`},
		{"evidence unknown field", evidenceIdentity, `"client":"future-test-agent","unknown":true,"client_version"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := bytes.Replace(body, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(body, changed) {
				t.Fatal("wire mutation did not exercise its field")
			}
			envelope := modernEnvelopeForBytes(t, changed, template, private)
			if _, err := VerifyBundle(changed, envelope, trust); !errors.Is(err, ErrStrictJSON) {
				t.Fatalf("modern reader accepted a schema-1 wire violation: %v", err)
			}
		})
	}
}

func TestModernReaderFactoryCopiesReviewedKeyMaterial(t *testing.T) {
	key, private := fixtureKey("modern-reader-test-only", KeyCurrent, 34)
	keys := []TrustedKey{key}
	trust := NewModernTrustStore(keys)
	body, envelope, _ := signedSnapshotFixture(t, futureClientSnapshot(41), key, private)
	keys[0].ID = "changed"
	keys[0].PublicKey[0] ^= 1
	if _, err := VerifyBundle(body, envelope, trust); err != nil {
		t.Fatalf("caller mutation changed reviewed modern key material: %v", err)
	}
}
