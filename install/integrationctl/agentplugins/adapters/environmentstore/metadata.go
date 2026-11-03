package environmentstore

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// MaxMetadataBytes is the serialized store-envelope ceiling, including the
// writer's final newline. Both publication and inspection enforce this limit.
const MaxMetadataBytes int64 = 4 << 20

func decodeMetadata(body []byte) (Metadata, error) {
	if int64(len(body)) > MaxMetadataBytes {
		return Metadata{}, fmt.Errorf("%w: metadata exceeds %d bytes", ErrMismatch, MaxMetadataBytes)
	}
	if !utf8.Valid(body) {
		return Metadata{}, fmt.Errorf("%w: metadata contains invalid UTF-8", ErrMismatch)
	}
	if err := conformance.RejectDuplicateJSONKeysFolded(body); err != nil {
		return Metadata{}, fmt.Errorf("%w: decode environment store metadata: %w", ErrMismatch, err)
	}
	var metadata Metadata
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return Metadata{}, fmt.Errorf("%w: decode environment store metadata: %w", ErrMismatch, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return Metadata{}, fmt.Errorf("%w: metadata contains trailing JSON values", ErrMismatch)
	}
	return canonicalMetadata(metadata)
}

func canonicalMetadata(metadata Metadata) (Metadata, error) {
	value := strings.TrimPrefix(metadata.TreeDigest, "sha256:")
	if metadata.SchemaVersion != 1 || metadata.DigestAlgorithm != domain.TreeDigestAlgorithm || value == metadata.TreeDigest || !digestHex.MatchString(value) {
		return Metadata{}, fmt.Errorf("%w: unsupported or invalid immutable metadata", ErrMismatch)
	}
	if len(metadata.ExecutableFiles) > domain.DefaultMaxFiles {
		return Metadata{}, fmt.Errorf("%w: executable inventory exceeds %d members", ErrMismatch, domain.DefaultMaxFiles)
	}
	if err := environment.ValidateExecutableInventory(metadata.ExecutableFiles); err != nil {
		return Metadata{}, fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	metadata.ExecutableFiles = append([]string{}, metadata.ExecutableFiles...)
	slices.Sort(metadata.ExecutableFiles)
	return metadata, nil
}

func metadataForSnapshot(snapshot domain.PackageSnapshot) (Metadata, error) {
	return canonicalMetadata(Metadata{SchemaVersion: 1, DigestAlgorithm: snapshot.DigestAlgorithm, TreeDigest: snapshot.TreeDigest,
		ExecutableFiles: snapshot.ExecutableFiles, Source: snapshot.Source})
}

func encodeMetadata(metadata Metadata) ([]byte, error) {
	metadata, err := canonicalMetadata(metadata)
	if err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	if int64(len(body))+1 > MaxMetadataBytes {
		return nil, fmt.Errorf("%w: metadata exceeds %d bytes", ErrMismatch, MaxMetadataBytes)
	}
	return append(body, '\n'), nil
}

func MetadataDigest(metadata Metadata) string {
	body, _ := json.Marshal(metadata)
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}
