// Package bootstrapio loads the exact, signed Directory artifacts used by
// release bootstrap generation and the explicitly test-only conformance path.
package bootstrapio

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

const maxTrustBytes = 16 << 10

type trustDocument struct {
	SchemaVersion int        `json:"schema_version"`
	Keys          []trustKey `json:"keys"`
}

type trustKey struct {
	KeyID     string `json:"key_id"`
	PublicKey string `json:"public_key"`
}

// LoadVerifiedBundle reads bounded exact bytes, strictly decodes the trust
// document, and delegates all Directory schema, digest, signature, sequence,
// and signing-key binding checks to directoryv1.VerifyBundle.
func LoadVerifiedBundle(snapshotPath, envelopePath, trustPath string) (directoryv1.VerifiedBundle, directoryv1.TrustStore, error) {
	snapshot, err := readBounded(snapshotPath, directoryv1.MaxSnapshotBytes, "snapshot")
	if err != nil {
		return directoryv1.VerifiedBundle{}, directoryv1.TrustStore{}, err
	}
	envelope, err := readBounded(envelopePath, directoryv1.MaxEnvelopeBytes, "envelope")
	if err != nil {
		return directoryv1.VerifiedBundle{}, directoryv1.TrustStore{}, err
	}
	trustBytes, err := readBounded(trustPath, maxTrustBytes, "trust")
	if err != nil {
		return directoryv1.VerifiedBundle{}, directoryv1.TrustStore{}, err
	}
	trust, err := parseTrust(trustBytes)
	if err != nil {
		return directoryv1.VerifiedBundle{}, directoryv1.TrustStore{}, fmt.Errorf("parse Directory trust: %w", err)
	}
	bundle, err := directoryv1.VerifyBundle(snapshot, envelope, trust)
	if err != nil {
		return directoryv1.VerifiedBundle{}, directoryv1.TrustStore{}, fmt.Errorf("verify Directory bootstrap inputs: %w", err)
	}
	return bundle, trust, nil
}

func readBounded(path string, limit int, label string) ([]byte, error) {
	if path == "" {
		return nil, fmt.Errorf("%s path is required", label)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open Directory %s: %w", label, err)
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read Directory %s: %w", label, err)
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("Directory %s is empty", label)
	}
	if len(body) > limit {
		return nil, fmt.Errorf("Directory %s exceeds %d bytes", label, limit)
	}
	return body, nil
}

func parseTrust(body []byte) (directoryv1.TrustStore, error) {
	if !json.Valid(body) {
		return directoryv1.TrustStore{}, errors.New("invalid JSON or trailing data")
	}
	if err := rejectDuplicateKeys(body); err != nil {
		return directoryv1.TrustStore{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var document trustDocument
	if err := decoder.Decode(&document); err != nil {
		return directoryv1.TrustStore{}, err
	}
	if err := ensureEOF(decoder); err != nil {
		return directoryv1.TrustStore{}, err
	}
	if document.SchemaVersion != 1 || len(document.Keys) == 0 {
		return directoryv1.TrustStore{}, errors.New("trust schema 1 requires at least one key")
	}
	store := directoryv1.TrustStore{Keys: make([]directoryv1.TrustedKey, 0, len(document.Keys))}
	seen := map[string]bool{}
	for _, input := range document.Keys {
		if input.KeyID == "" || seen[input.KeyID] {
			return directoryv1.TrustStore{}, fmt.Errorf("empty or duplicate key ID %q", input.KeyID)
		}
		seen[input.KeyID] = true
		publicKey, err := base64.StdEncoding.Strict().DecodeString(input.PublicKey)
		if err != nil || len(publicKey) != ed25519.PublicKeySize {
			return directoryv1.TrustStore{}, fmt.Errorf("key %q has malformed Ed25519 public key", input.KeyID)
		}
		store.Keys = append(store.Keys, directoryv1.TrustedKey{ID: input.KeyID, PublicKey: ed25519.PublicKey(publicKey), State: directoryv1.KeyCurrent})
	}
	return store, nil
}

func rejectDuplicateKeys(body []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := scanValue(decoder); err != nil {
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

func scanValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, compound := token.(json.Delim)
	if !compound {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate or invalid object key %q", key)
			}
			seen[key] = true
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}
	_, err = decoder.Token()
	return err
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
