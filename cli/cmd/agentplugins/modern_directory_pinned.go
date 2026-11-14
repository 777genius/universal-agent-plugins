package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

// pinnedModernDirectoryTrust contains only compile-time reviewed public material.
// The owner-approved issuer, origin, key and bootstrap do not exist yet; this
// constructor is deliberately not wired into production CLI routing.
// Never populate it from environment variables, remote pointers or test keys.
type pinnedModernDirectoryTrust struct {
	FeedID          string
	Origin          string
	KeyID           string
	PublicKeyBase64 string
	Bootstrap       directoryv1.EmbeddedBundle
}

// newPinnedModernDirectoryClient establishes the trust boundary required by an
// independent schema-1 lineage. It does not authorize any Pi distribution or
// publish a feed. The existing legacy factory and cache remain untouched.
func newPinnedModernDirectoryClient(dataRoot string, approved pinnedModernDirectoryTrust) (*directoryv1.Client, error) {
	if strings.TrimSpace(dataRoot) == "" {
		return nil, fmt.Errorf("modern Directory data root is required")
	}
	if approved.FeedID == "" || approved.FeedID != strings.TrimSpace(approved.FeedID) ||
		strings.ContainsAny(approved.FeedID, "/\\ \t\r\n") {
		return nil, fmt.Errorf("modern Directory requires a distinct canonical feed identity")
	}
	origin, err := validatePinnedModernDirectoryOrigin(approved.Origin)
	if err != nil {
		return nil, err
	}
	if approved.KeyID == "" || approved.KeyID != strings.TrimSpace(approved.KeyID) ||
		approved.KeyID == defaultDirectoryKeyID {
		return nil, fmt.Errorf("modern Directory requires a distinct pinned signing key ID")
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(approved.PublicKeyBase64)
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("modern Directory requires a valid pinned Ed25519 public key")
	}
	legacy, err := base64.StdEncoding.Strict().DecodeString(defaultDirectoryPublicKey)
	if err != nil || len(legacy) != ed25519.PublicKeySize || bytes.Equal(raw, legacy) {
		return nil, fmt.Errorf("modern Directory cannot reuse the historical signing key")
	}
	trust := directoryv1.TrustStore{Keys: []directoryv1.TrustedKey{{
		ID: approved.KeyID, PublicKey: ed25519.PublicKey(raw), State: directoryv1.KeyCurrent,
	}}}
	verified, err := approved.Bootstrap.Verify(trust)
	if err != nil {
		return nil, fmt.Errorf("modern Directory pinned bootstrap failed signature verification: %w", err)
	}
	if verified.Snapshot.SnapshotSchemaVersion != 1 {
		return nil, fmt.Errorf("modern Directory bootstrap must use schema 1")
	}
	return &directoryv1.Client{
		Origin:                   origin,
		FeedID:                   approved.FeedID,
		HTTPClient:               hardenedHTTPClient("Modern Directory"),
		Trust:                    trust,
		Embedded:                 approved.Bootstrap,
		RequireEmbeddedBootstrap: true,
		Cache:                    directoryv1.Cache{Path: filepath.Join(dataRoot, "directory-modern-v1-cache.json")},
	}, nil
}

// validatePinnedModernDirectoryOrigin refuses alternate spellings of the
// historical HTTPS endpoint before constructing a separately trusted client.
func validatePinnedModernDirectoryOrigin(pinned string) (string, error) {
	if pinned == "" || pinned != strings.TrimSpace(pinned) {
		return "", fmt.Errorf("modern Directory requires a pinned origin")
	}
	origin, err := productionFeedOrigin(pinned, pinned, "modern Directory origin")
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(origin)
	if err != nil || !strings.HasSuffix(parsed.Path, "/schemas/1/") {
		return "", fmt.Errorf("modern Directory requires a separate schema-1 HTTPS origin")
	}
	legacyOrigin, err := url.Parse(defaultDirectoryOrigin)
	if err != nil {
		return "", fmt.Errorf("historical Directory origin is invalid: %w", err)
	}
	// A different path on the historical host is NOT a separate HTTPS origin.
	// Default ports, zero-padded ports and trailing DNS dots must not disguise
	// reuse of the legacy hosting authority.
	legacyHost := strings.TrimSuffix(legacyOrigin.Hostname(), ".")
	modernHost := strings.TrimSuffix(parsed.Hostname(), ".")
	// Compare numeric TCP ports: :00443 and :443 reach the same HTTPS
	// endpoint and must not create a second trust authority.
	legacyPort, modernPort := 443, 443
	if port := legacyOrigin.Port(); port != "" {
		legacyPort, err = strconv.Atoi(port)
		if err != nil || legacyPort < 1 || legacyPort > 65535 {
			return "", fmt.Errorf("historical Directory HTTPS port is invalid")
		}
	}
	if port := parsed.Port(); port != "" {
		modernPort, err = strconv.Atoi(port)
		if err != nil || modernPort < 1 || modernPort > 65535 {
			return "", fmt.Errorf("modern Directory HTTPS port is invalid")
		}
	}
	if strings.EqualFold(modernHost, legacyHost) && modernPort == legacyPort {
		return "", fmt.Errorf("modern Directory cannot reuse the historical HTTPS origin")
	}
	return origin, nil
}
