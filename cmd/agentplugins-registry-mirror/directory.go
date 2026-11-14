package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

// Only reviewed composition can supply this profile. Production still constructs
// the historical profile; neither input bytes nor public flags can select modern.
type directoryMirrorProfile struct {
	origin     *url.URL
	feedID     string
	trust      directoryv1.TrustStore
	trustBytes []byte
}

type modernMirrorAuthority struct {
	FeedID, Origin, KeyID, PublicKeyBase64 string
}

func legacyDirectoryProfile(base *url.URL) directoryMirrorProfile {
	return directoryMirrorProfile{origin: base.ResolveReference(&url.URL{Path: "registry/schemas/1/"})}
}

// No production instance exists until a reviewed activation supplies the real
// independent issuer's public facts. The initial pinned key matches the CLI seam.
func newModernDirectoryProfile(approved modernMirrorAuthority) (directoryMirrorProfile, error) {
	if approved.FeedID == "" || approved.FeedID != strings.TrimSpace(approved.FeedID) || strings.ContainsAny(approved.FeedID, "/\\ \t\r\n") {
		return directoryMirrorProfile{}, errors.New("modern Directory requires a distinct canonical feed identity")
	}
	origin, err := independentDirectoryOrigin(approved.Origin)
	if err != nil {
		return directoryMirrorProfile{}, err
	}
	key, err := decodeKey(approved.KeyID, approved.PublicKeyBase64, string(directoryv1.KeyCurrent))
	if err != nil || approved.KeyID == "" || approved.KeyID != strings.TrimSpace(approved.KeyID) || approved.KeyID == directoryKeyID {
		return directoryMirrorProfile{}, errors.New("modern Directory requires a distinct pinned signing key")
	}
	legacy, err := base64.StdEncoding.Strict().DecodeString(directoryPublicKey)
	if err != nil || len(legacy) != ed25519.PublicKeySize || bytes.Equal(key.PublicKey, legacy) {
		return directoryMirrorProfile{}, errors.New("modern Directory cannot reuse historical public key bytes")
	}
	body, err := json.Marshal(map[string]any{"schema_version": 1, "keys": []map[string]string{{
		"key_id": approved.KeyID, "public_key": approved.PublicKeyBase64, "state": string(directoryv1.KeyCurrent),
	}}})
	if err != nil {
		return directoryMirrorProfile{}, err
	}
	return directoryMirrorProfile{
		origin: origin, feedID: approved.FeedID,
		trust: directoryv1.NewModernTrustStore([]directoryv1.TrustedKey{key}), trustBytes: append(body, '\n'),
	}, nil
}

func independentDirectoryOrigin(raw string) (*url.URL, error) {
	if raw != strings.TrimSpace(raw) {
		return nil, errors.New("modern Directory origin must be canonical")
	}
	origin, err := safeOrigin(raw)
	if err != nil || !strings.HasSuffix(origin.Path, "/schemas/1/") {
		return nil, errors.New("modern Directory requires a separate schema-1 HTTPS origin")
	}
	legacy, err := safeOrigin(defaultPagesOrigin)
	if err != nil {
		return nil, err
	}
	port, err := directoryOriginPort(origin)
	if err != nil {
		return nil, err
	}
	legacyPort, err := directoryOriginPort(legacy)
	if err != nil {
		return nil, err
	}
	if port == legacyPort && strings.EqualFold(strings.TrimSuffix(origin.Hostname(), "."), strings.TrimSuffix(legacy.Hostname(), ".")) {
		return nil, errors.New("modern Directory cannot reuse the historical HTTPS origin")
	}
	return origin, nil
}

func directoryOriginPort(origin *url.URL) (int, error) {
	if origin.Port() == "" {
		return 443, nil
	}
	port, err := strconv.Atoi(origin.Port())
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("directory HTTPS port is invalid")
	}
	return port, nil
}

type directoryResult struct {
	pointerBytes, snapshotBytes, envelopeBytes, trustBytes []byte
	stem, feedID                                           string
	bundle                                                 directoryv1.VerifiedBundle
}

func (profile directoryMirrorProfile) signingTrust(registry string, snapshotBytes []byte) ([]byte, directoryv1.TrustStore, error) {
	if profile.feedID != "" {
		return profile.trustBytes, profile.trust, nil
	}
	// Historical lookup behavior stays strict. This source identity is not
	// accepted as authentic until VerifyPointerBundle succeeds below.
	snapshot, err := directoryv1.ParseSnapshot(snapshotBytes)
	if err != nil {
		return nil, directoryv1.TrustStore{}, err
	}
	return fetchDirectoryTrust(registry, snapshot.SourceCommit)
}

func fetchDirectoryWithProfile(client *http.Client, registry string, profile directoryMirrorProfile) (directoryResult, error) {
	pointerBytes, err := get(client, profile.origin.ResolveReference(&url.URL{Path: "latest.json"}).String(), directoryv1.MaxLatestBytes)
	if err != nil {
		return directoryResult{}, err
	}
	pointer, err := directoryv1.ParsePointer(pointerBytes)
	if err != nil {
		return directoryResult{}, err
	}
	if len(pointerBytes) > pointer.FetchContract.LatestMaxBytes {
		return directoryResult{}, errors.New("latest pointer exceeds declared limit")
	}
	getArtifact := func(relative string, limit int) ([]byte, error) {
		return get(client, profile.origin.ResolveReference(&url.URL{Path: relative}).String(), limit)
	}
	snapshotBytes, err := getArtifact(pointer.SnapshotPath, pointer.FetchContract.SnapshotMaxBytes)
	if err != nil {
		return directoryResult{}, err
	}
	envelopeBytes, err := getArtifact(pointer.EnvelopePath, pointer.FetchContract.EnvelopeMaxBytes)
	if err != nil {
		return directoryResult{}, err
	}
	trustBytes, trust, err := profile.signingTrust(registry, snapshotBytes)
	if err != nil {
		return directoryResult{}, err
	}
	bundle, err := directoryv1.VerifyPointerBundle(pointer, snapshotBytes, envelopeBytes, trust)
	if err != nil {
		return directoryResult{}, err
	}
	return directoryResult{
		pointerBytes: pointerBytes, snapshotBytes: snapshotBytes, envelopeBytes: envelopeBytes, trustBytes: trustBytes,
		stem: fmt.Sprintf("%020d", pointer.Sequence), feedID: profile.feedID, bundle: bundle,
	}, nil
}

func directoryFiles(directory directoryResult) []mirrorFile {
	root, trust := "registry/schemas/1/", "registry/publication/trusted-keys.json"
	if directory.feedID != "" {
		root, trust = "registry/modern/schemas/1/", "registry/publication/modern/trusted-keys.json"
	}
	return []mirrorFile{
		{rel: root + "latest.json", body: directory.pointerBytes},
		{rel: root + "snapshots/" + directory.stem + ".json", body: directory.snapshotBytes},
		{rel: root + "snapshots/" + directory.stem + ".envelope.json", body: directory.envelopeBytes},
		{rel: trust, body: directory.trustBytes},
	}
}
