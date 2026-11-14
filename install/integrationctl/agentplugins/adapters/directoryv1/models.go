// Package directoryv1 implements the untrusted transport, strict schema-1
// decoding, cryptographic verification, and last-known-good persistence for the
// Universal Agent Plugins Directory.
package directoryv1

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	SnapshotSchemaVersion = 1
	EnvelopeSchemaVersion = 1
	PointerSchemaVersion  = 1
	SignatureDomain       = "UAP-DIRECTORY-SNAPSHOT-ED25519-V1"
	MaxLatestBytes        = 16 << 10
	MaxEnvelopeBytes      = 16 << 10
	MaxSnapshotBytes      = 4 << 20
)

var signaturePrefix = []byte(SignatureDomain + "\x00")

type Pointer struct {
	PointerSchemaVersion  int           `json:"pointer_schema_version"`
	SnapshotSchemaVersion int           `json:"snapshot_schema_version"`
	Sequence              uint64        `json:"sequence"`
	SnapshotPath          string        `json:"snapshot_path"`
	EnvelopePath          string        `json:"envelope_path"`
	FetchContract         FetchContract `json:"fetch_contract"`
}

type FetchContract struct {
	HTTPSRequired                bool `json:"https_required"`
	SameOriginRedirectsOnly      bool `json:"same_origin_redirects_only"`
	ForwardCredentialsOnRedirect bool `json:"forward_credentials_on_redirect"`
	MaxRedirects                 int  `json:"max_redirects"`
	LatestMaxBytes               int  `json:"latest_max_bytes"`
	SnapshotMaxBytes             int  `json:"snapshot_max_bytes"`
	EnvelopeMaxBytes             int  `json:"envelope_max_bytes"`
	RetryAttempts                int  `json:"retry_attempts"`
}

type Envelope struct {
	EnvelopeSchemaVersion int    `json:"envelope_schema_version"`
	SnapshotSchemaVersion int    `json:"snapshot_schema_version"`
	Sequence              uint64 `json:"sequence"`
	KeyID                 string `json:"key_id"`
	Algorithm             string `json:"algorithm"`
	SignatureDomain       string `json:"signature_domain"`
	SnapshotDigest        string `json:"snapshot_digest"`
	Signature             string `json:"signature"`
}

type KeyState string

const (
	KeyCurrent KeyState = "current"
	KeyNext    KeyState = "next"
	KeyRetired KeyState = "retired"
)

type TrustedKey struct {
	ID        string
	PublicKey ed25519.PublicKey
	State     KeyState
}

type TrustStore struct{ Keys []TrustedKey }

type VerifiedBundle struct {
	SnapshotBytes []byte
	EnvelopeBytes []byte
	Snapshot      domain.DirectorySnapshot
	Envelope      Envelope
	Digest        string
	Source        BundleSource
}

type BundleSource string

const (
	BundleSourceVerified BundleSource = "verified"
	BundleSourceRemote   BundleSource = "remote"
	BundleSourceCache    BundleSource = "cache"
	BundleSourceEmbedded BundleSource = "embedded"
)

var (
	ErrStrictJSON       = errors.New("directory JSON is not strict schema 1")
	ErrUnknownKey       = errors.New("unknown directory signing key")
	ErrRetiredKey       = errors.New("retired directory signing key")
	ErrNoTrustedKeys    = errors.New("no trusted directory signing keys configured")
	ErrDigestMismatch   = errors.New("directory snapshot digest mismatch")
	ErrInvalidSignature = errors.New("invalid directory snapshot signature")
	ErrSequenceMismatch = errors.New("directory schema or sequence mismatch")
)

var (
	keyIDPattern        = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]*[a-z0-9])?$`)
	simpleIDPattern     = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	evidenceIDPattern   = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]*$`)
	distributionPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*/[a-z0-9]+(?:-[a-z0-9]+)*$`)
	repositoryPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	shaPattern          = regexp.MustCompile(`^[0-9a-f]{40}$`)
	digestPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	publicationPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
	timestampPattern    = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$`)
	semverPattern       = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	iconPathPattern     = regexp.MustCompile(`^assets/plugin-icons/[A-Za-z0-9._-]+$`)
	workflowPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9][A-Za-z0-9._-]*/\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$`)
	sourceRefPattern    = regexp.MustCompile(`^refs/heads/[A-Za-z0-9._/-]+$`)
)

func ParsePointer(body []byte) (Pointer, error) {
	var value Pointer
	if err := strictDecode(body, &value, MaxLatestBytes); err != nil {
		return value, fmt.Errorf("%w: pointer: %v", ErrStrictJSON, err)
	}
	if err := requireRootFields(body, "pointer_schema_version", "snapshot_schema_version", "sequence", "snapshot_path", "envelope_path", "fetch_contract"); err != nil {
		return value, fmt.Errorf("%w: pointer: %v", ErrStrictJSON, err)
	}
	var root map[string]json.RawMessage
	_ = json.Unmarshal(body, &root)
	var contract map[string]json.RawMessage
	if err := json.Unmarshal(root["fetch_contract"], &contract); err != nil {
		return value, fmt.Errorf("%w: fetch contract: %v", ErrStrictJSON, err)
	}
	if err := requiredMap(contract, "https_required", "same_origin_redirects_only", "forward_credentials_on_redirect", "max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes", "retry_attempts"); err != nil {
		return value, fmt.Errorf("%w: fetch contract: %v", ErrStrictJSON, err)
	}
	for _, field := range []string{"https_required", "same_origin_redirects_only", "forward_credentials_on_redirect", "max_redirects", "latest_max_bytes", "snapshot_max_bytes", "envelope_max_bytes", "retry_attempts"} {
		if isJSONNull(contract[field]) {
			return value, fmt.Errorf("%w: fetch contract field %q cannot be null", ErrStrictJSON, field)
		}
	}
	if err := validatePointer(value); err != nil {
		return value, err
	}
	return value, nil
}

func ParseEnvelope(body []byte) (Envelope, error) {
	var value Envelope
	if err := strictDecode(body, &value, MaxEnvelopeBytes); err != nil {
		return value, fmt.Errorf("%w: envelope: %v", ErrStrictJSON, err)
	}
	if err := requireRootFields(body, "envelope_schema_version", "snapshot_schema_version", "sequence", "key_id", "algorithm", "signature_domain", "snapshot_digest", "signature"); err != nil {
		return value, fmt.Errorf("%w: envelope: %v", ErrStrictJSON, err)
	}
	if err := validateEnvelope(value); err != nil {
		return value, err
	}
	return value, nil
}

func ParseSnapshot(body []byte) (domain.DirectorySnapshot, error) {
	var value domain.DirectorySnapshot
	if err := strictDecode(body, &value, MaxSnapshotBytes); err != nil {
		return value, fmt.Errorf("%w: snapshot: %v", ErrStrictJSON, err)
	}
	if err := requireSnapshotFields(body); err != nil {
		return value, fmt.Errorf("%w: snapshot: %v", ErrStrictJSON, err)
	}
	if err := validateSnapshot(value); err != nil {
		return value, err
	}
	return value, nil
}

// VerifyBundle authenticates the exact received snapshot bytes. Key IDs are
// exact and local; no key discovery is attempted.
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
	snapshot, err := ParseSnapshot(snapshotBytes)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if envelope.SnapshotSchemaVersion != snapshot.SnapshotSchemaVersion || envelope.Sequence != snapshot.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	return VerifiedBundle{SnapshotBytes: append([]byte(nil), snapshotBytes...), EnvelopeBytes: append([]byte(nil), envelopeBytes...), Snapshot: snapshot, Envelope: envelope, Digest: digest, Source: BundleSourceVerified}, nil
}

// SnapshotSignatureMessage returns the byte-exact, domain-separated message
// authenticated by a Directory snapshot envelope.
func SnapshotSignatureMessage(snapshotBytes []byte) []byte {
	message := make([]byte, 0, len(signaturePrefix)+8+len(snapshotBytes))
	message = append(message, signaturePrefix...)
	message = binary.BigEndian.AppendUint64(message, uint64(len(snapshotBytes)))
	return append(message, snapshotBytes...)
}

func VerifyPointerBundle(pointer Pointer, snapshotBytes, envelopeBytes []byte, trust TrustStore) (VerifiedBundle, error) {
	bundle, err := VerifyBundle(snapshotBytes, envelopeBytes, trust)
	if err != nil {
		return VerifiedBundle{}, err
	}
	if pointer.SnapshotSchemaVersion != bundle.Snapshot.SnapshotSchemaVersion || pointer.Sequence != bundle.Snapshot.Sequence || pointer.Sequence != bundle.Envelope.Sequence {
		return VerifiedBundle{}, ErrSequenceMismatch
	}
	return bundle, nil
}

func trustedKey(store TrustStore, id string) (TrustedKey, error) {
	seen := map[string]bool{}
	for _, key := range store.Keys {
		if seen[key.ID] {
			return TrustedKey{}, fmt.Errorf("%w: duplicate configured key ID %s", ErrUnknownKey, key.ID)
		}
		seen[key.ID] = true
		if !keyIDPattern.MatchString(key.ID) || len(key.PublicKey) != ed25519.PublicKeySize {
			return TrustedKey{}, fmt.Errorf("%w: malformed configured key %s", ErrUnknownKey, key.ID)
		}
		if key.State != KeyCurrent && key.State != KeyNext && key.State != KeyRetired {
			return TrustedKey{}, fmt.Errorf("%w: %s has invalid state", ErrUnknownKey, key.ID)
		}
	}
	for _, key := range store.Keys {
		if key.ID != id {
			continue
		}
		if key.State == KeyRetired {
			return TrustedKey{}, fmt.Errorf("%w: %s", ErrRetiredKey, id)
		}
		return key, nil
	}
	return TrustedKey{}, fmt.Errorf("%w: %s", ErrUnknownKey, id)
}

func strictDecode(body []byte, destination any, limit int) error {
	if len(body) == 0 {
		return errors.New("empty body")
	}
	if len(body) > limit {
		return fmt.Errorf("body exceeds %d bytes", limit)
	}
	if !json.Valid(body) {
		return errors.New("invalid JSON or trailing data")
	}
	if err := rejectDuplicateKeys(body); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if err := ensureEOF(decoder); err != nil {
		return err
	}
	return nil
}

func ensureEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return errors.New("trailing JSON")
		}
		return err
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return errors.New("JSON null is not allowed by directory schema 1")
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		keys := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if _, exists := keys[key]; exists {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return errors.New("unterminated object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return errors.New("unterminated array")
		}
	default:
		return errors.New("unexpected JSON delimiter")
	}
	return nil
}
