package directoryv1

import (
	"crypto/ed25519"
	"fmt"
	"regexp"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TrustStore binds locally reviewed keys to their reader contract. The zero
// profile preserves historical schema-1 decoding, including unknown-ID refusal.
type TrustStore struct {
	Keys   []TrustedKey
	reader snapshotReader
}

type snapshotReader uint8

const (
	historicalReader snapshotReader = iota
	modernReader
)

var clientIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// NewModernTrustStore selects additive client-ID decoding for an independently
// pinned modern schema-1 issuer. It never discovers or authorizes signing keys.
// Authenticated future IDs remain in the snapshot; local client definitions
// still control installation eligibility. Wire metadata cannot select a reader.
// This independent lineage requires explicit evidence trust from sequence one.
func NewModernTrustStore(keys []TrustedKey) TrustStore {
	trust := TrustStore{Keys: make([]TrustedKey, len(keys)), reader: modernReader}
	for i, key := range keys {
		trust.Keys[i] = key
		trust.Keys[i].PublicKey = append(ed25519.PublicKey(nil), key.PublicKey...)
	}
	return trust
}

// ParseSnapshot preserves the historical strict schema-1 contract. Modern
// decoding is available only through a bundle authenticated by its TrustStore.
func ParseSnapshot(body []byte) (domain.DirectorySnapshot, error) {
	return parseSnapshot(body, historicalReader)
}

func parseSnapshot(body []byte, reader snapshotReader) (domain.DirectorySnapshot, error) {
	var value domain.DirectorySnapshot
	if err := strictDecode(body, &value, MaxSnapshotBytes); err != nil {
		return value, fmt.Errorf("%w: snapshot: %v", ErrStrictJSON, err)
	}
	if err := requireSnapshotFields(body, reader); err != nil {
		return value, fmt.Errorf("%w: snapshot: %v", ErrStrictJSON, err)
	}
	if err := validateSnapshot(value, reader); err != nil {
		return value, err
	}
	return value, nil
}

func (reader snapshotReader) validClient(client domain.ClientID) bool {
	return domain.IsSupportedClient(client) ||
		(reader == modernReader && clientIDPattern.MatchString(string(client)))
}

func (reader snapshotReader) legacyEvidence(sequence uint64) bool {
	return reader == historicalReader && sequence < domain.DirectoryEvidenceTrustCutoverSequence
}
