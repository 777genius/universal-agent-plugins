package directoryv1

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// VerifyBundle authenticates the exact received snapshot bytes before decoding
// according to the locally pinned reader. Key IDs are exact and local; no key
// discovery is attempted. Cache and embedded bundles use this same boundary.
func VerifyBundle(snapshotBytes, envelopeBytes []byte, trust TrustStore) (VerifiedBundle, error) {
	if len(trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	envelope, err := ParseEnvelope(envelopeBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	sum := sha256.Sum256(snapshotBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
	if digest != envelope.SnapshotDigest {
		return VerifiedBundle{}, ErrDigestMismatch
	}
	key, err := trustedKey(trust, envelope.KeyID)
	if err != nil {
		return VerifiedBundle{}, err
	}
	signature, err := base64.StdEncoding.Strict().DecodeString(envelope.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return VerifiedBundle{}, fmt.Errorf("%w: malformed base64", ErrInvalidSignature)
	}
	if !ed25519.Verify(key.PublicKey, SnapshotSignatureMessage(snapshotBytes), signature) {
		return VerifiedBundle{}, ErrInvalidSignature
	}
	snapshot, err := parseSnapshot(snapshotBytes, trust.reader)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if envelope.SnapshotSchemaVersion != snapshot.SnapshotSchemaVersion || envelope.Sequence != snapshot.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	return VerifiedBundle{SnapshotBytes: append([]byte(nil), snapshotBytes...), EnvelopeBytes: append([]byte(nil), envelopeBytes...), Snapshot: snapshot, Envelope: envelope, Digest: digest, Source: BundleSourceVerified}, nil
}
