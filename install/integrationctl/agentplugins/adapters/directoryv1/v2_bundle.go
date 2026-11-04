package directoryv1

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The v2 contract has independent signature and sequence spaces. This reader
// is explicit; the existing production v1 Client and bootstrap do not use it.
const SignatureDomainV2 = "UAP-DIRECTORY-SNAPSHOT-ED25519-V2"

var signaturePrefixV2 = []byte(SignatureDomainV2 + "\x00")

func SnapshotSignatureMessageV2(snapshot []byte) []byte {
	message := make([]byte, 0, len(signaturePrefixV2)+8+len(snapshot))
	message = append(message, signaturePrefixV2...)
	message = binary.BigEndian.AppendUint64(message, uint64(len(snapshot)))
	return append(message, snapshot...)
}

func ParseV2Envelope(body []byte) (Envelope, error) {
	var envelope Envelope
	if err := strictDecode(body, &envelope, MaxEnvelopeBytes); err != nil {
		return envelope, fmt.Errorf("%w: v2 envelope: %v", ErrStrictJSON, err)
	}
	if err := requireRootFields(body, "envelope_schema_version", "snapshot_schema_version",
		"sequence", "key_id", "algorithm", "signature_domain", "snapshot_digest", "signature"); err != nil {
		return envelope, fmt.Errorf("%w: v2 envelope: %v", ErrStrictJSON, err)
	}
	if envelope.EnvelopeSchemaVersion != 2 || envelope.SnapshotSchemaVersion != 2 || envelope.Sequence < 1 ||
		envelope.Algorithm != "Ed25519" || envelope.SignatureDomain != SignatureDomainV2 ||
		!keyIDPattern.MatchString(envelope.KeyID) || !digestPattern.MatchString(envelope.SnapshotDigest) {
		return envelope, strictError("invalid v2 envelope contract")
	}
	if signature, err := base64Strict(envelope.Signature); err != nil || len(signature) != ed25519.SignatureSize {
		return envelope, strictError("invalid v2 envelope signature")
	}
	return envelope, nil
}

func ParseV2Snapshot(body []byte) (domain.DirectorySnapshot, error) {
	var snapshot domain.DirectorySnapshot
	if err := strictDecode(body, &snapshot, MaxSnapshotBytes); err != nil {
		return snapshot, fmt.Errorf("%w: v2 snapshot: %v", ErrStrictJSON, err)
	}
	if err := requireV2SnapshotFields(body); err != nil {
		return snapshot, fmt.Errorf("%w: v2 snapshot: %v", ErrStrictJSON, err)
	}
	if err := validateV2Snapshot(snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

// VerifyV2Bundle checks exact received bytes. V1 signatures cannot authenticate
// V2 data, even if an attacker edits envelope fields or reuses a sequence.
func VerifyV2Bundle(snapshotBytes, envelopeBytes []byte, trust TrustStore) (VerifiedBundle, error) {
	if len(trust.Keys) == 0 {
		return VerifiedBundle{}, ErrNoTrustedKeys
	}
	envelope, err := ParseV2Envelope(envelopeBytes)
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
	if err != nil || len(signature) != ed25519.SignatureSize ||
		!ed25519.Verify(key.PublicKey, SnapshotSignatureMessageV2(snapshotBytes), signature) {
		return VerifiedBundle{}, ErrInvalidSignature
	}
	snapshot, err := ParseV2Snapshot(snapshotBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if snapshot.SnapshotSchemaVersion != envelope.SnapshotSchemaVersion || snapshot.Sequence != envelope.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	return VerifiedBundle{
		SnapshotBytes: append([]byte(nil), snapshotBytes...),
		EnvelopeBytes: append([]byte(nil), envelopeBytes...),
		Snapshot: snapshot, Envelope: envelope, Digest: digest, Source: BundleSourceVerified,
	}, nil
}
