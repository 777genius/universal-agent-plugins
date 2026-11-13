package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/directoryv1"
)

func modernPinnedTestTrust(t *testing.T) pinnedModernDirectoryTrust {
	t.Helper()
	root := t.TempDir()
	fixture := writeConformanceFixtureWithSeed(t, root, 39)
	signed, err := newConformanceDirectoryClient(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return pinnedModernDirectoryTrust{
		FeedID: "pi-modern-v1-test-only",
		Origin: "https://modern-test.invalid/registry/modern/schemas/1/",
		KeyID: signed.Trust.Keys[0].ID,
		PublicKeyBase64: base64.StdEncoding.EncodeToString(signed.Trust.Keys[0].PublicKey),
		Bootstrap: signed.Embedded,
	}
}

func TestPinnedModernDirectoryTrustIsIndependentAndEnvironmentCannotSelectIssuer(t *testing.T) {
	clearDirectoryEnvironment(t)
	root := t.TempDir()
	legacy, err := newDirectoryClient(root)
	if err != nil {
		t.Fatal(err)
	}
	approved := modernPinnedTestTrust(t)
	t.Setenv("AGENTPLUGINS_DIRECTORY_ORIGIN", "https://caller.invalid/registry/schemas/1/")
	modern, err := newPinnedModernDirectoryClient(root, approved)
	if err != nil {
		t.Fatal(err)
	}
	if modern.Origin != approved.Origin || modern.FeedID != approved.FeedID ||
		modern.SignedFeedID() != approved.FeedID || !modern.RequireEmbeddedBootstrap ||
		modern.Cache.Path != filepath.Join(root, "directory-modern-v1-cache.json") ||
		modern.Cache.Path == legacy.Cache.Path || modern.Origin == legacy.Origin ||
		len(modern.Trust.Keys) != 1 || bytes.Equal(modern.Trust.Keys[0].PublicKey, legacy.Trust.Keys[0].PublicKey) {
		t.Fatal("independent pinned modern Directory reused legacy trust, env origin or cache")
	}
	if _, err := modern.Embedded.Verify(legacy.Trust); err == nil {
		t.Fatal("legacy trust unexpectedly authenticated modern embedded bootstrap")
	}
	if _, err := legacy.Embedded.Verify(modern.Trust); err == nil {
		t.Fatal("modern trust unexpectedly authenticated legacy embedded bootstrap")
	}
	// The disposable fixture's transport is injected only after construction;
	// no production environment variable can replace the pinned trust.
	modern.HTTPClient.Transport = conformanceFixtureTransport{
		sequence: 41, snapshot: approved.Bootstrap.Snapshot, envelope: approved.Bootstrap.Envelope,
	}
	modern.Now = func() time.Time { return time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC) }
	got, err := modern.Load(context.Background(), 0)
	if err != nil || got.Snapshot.Sequence != 41 || got.Source != directoryv1.BundleSourceRemote {
		t.Fatalf("pinned modern signed remote load = %+v, err %v", got, err)
	}
	if _, err := modern.Cache.Load(modern.Trust); err != nil {
		t.Fatalf("modern independently signed cache not persisted: %v", err)
	}
	if _, err := modern.Cache.Load(legacy.Trust); err == nil {
		t.Fatal("historical signing key authenticated modern cache")
	}
	if _, err := os.Lstat(legacy.Cache.Path); !os.IsNotExist(err) {
		t.Fatalf("modern load unexpectedly mutated historical cache: %v", err)
	}
}

func TestPinnedModernDirectoryTrustFailsClosedBeforeFilesystemMutation(t *testing.T) {
	clearDirectoryEnvironment(t)
	baseline := modernPinnedTestTrust(t)
	other := modernPinnedTestTrust(t)
	other.Bootstrap = writeDifferentModernBootstrap(t)
	cases := []struct {
		name string
		mutate func(*pinnedModernDirectoryTrust)
	}{
		{"missing_feed", func(c *pinnedModernDirectoryTrust) { c.FeedID = "" }},
		{"unsafe_feed", func(c *pinnedModernDirectoryTrust) { c.FeedID = "../legacy" }},
		{"legacy_origin", func(c *pinnedModernDirectoryTrust) { c.Origin = defaultDirectoryOrigin }},
		{"legacy_origin_default_https_port", func(c *pinnedModernDirectoryTrust) {
			c.Origin = "https://777genius.github.io:443/universal-agent-plugins-registry/registry/schemas/1/"
		}},
		{"legacy_origin_trailing_dns_dot", func(c *pinnedModernDirectoryTrust) {
			c.Origin = "https://777genius.github.io./universal-agent-plugins-registry/registry/schemas/1/"
		}},
		{"http_origin", func(c *pinnedModernDirectoryTrust) { c.Origin = "http://modern.invalid/registry/schemas/1/" }},
		{"wrong_schema_path", func(c *pinnedModernDirectoryTrust) { c.Origin = "https://modern.invalid/registry/other/" }},
		{"legacy_key_id", func(c *pinnedModernDirectoryTrust) { c.KeyID = defaultDirectoryKeyID }},
		{"legacy_key_bytes", func(c *pinnedModernDirectoryTrust) { c.PublicKeyBase64 = defaultDirectoryPublicKey }},
		{"malformed_key", func(c *pinnedModernDirectoryTrust) { c.PublicKeyBase64 = "not-ed25519" }},
		{"missing_bootstrap", func(c *pinnedModernDirectoryTrust) { c.Bootstrap = directoryv1.EmbeddedBundle{} }},
		{"wrong_signed_bootstrap", func(c *pinnedModernDirectoryTrust) { c.Bootstrap = other.Bootstrap }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := baseline
			tc.mutate(&cfg)
			dataRoot := filepath.Join(t.TempDir(), "must-not-exist")
			if _, err := newPinnedModernDirectoryClient(dataRoot, cfg); err == nil {
				t.Fatal("unreviewed, cross-feed or invalid pinned modern authority accepted")
			}
			if _, err := os.Stat(dataRoot); !os.IsNotExist(err) {
				t.Fatalf("rejected trust configuration mutated state root: %v", err)
			}
		})
	}
	if _, err := newPinnedModernDirectoryClient("", baseline); err == nil {
		t.Fatal("empty modern Directory data root accepted")
	}
}

func writeDifferentModernBootstrap(t *testing.T) directoryv1.EmbeddedBundle {
	t.Helper()
	fixture := writeConformanceFixtureWithSeed(t, t.TempDir(), 17)
	client, err := newConformanceDirectoryClient(fixture)
	if err != nil {
		t.Fatal(err)
	}
	return client.Embedded
}

func TestPinnedModernDirectoryCannotUseLegacyKeyWithNewID(t *testing.T) {
	approved := modernPinnedTestTrust(t)
	approved.KeyID = "new-name-for-historical-key"
	approved.PublicKeyBase64 = defaultDirectoryPublicKey
	// Key IDs alone cannot create a distinct trust authority.
	if _, err := newPinnedModernDirectoryClient(t.TempDir(), approved); err == nil ||
		!strings.Contains(err.Error(), "historical signing key") {
		t.Fatalf("legacy key reuse was not rejected: %v", err)
	}
	if raw, err := base64.StdEncoding.DecodeString(approved.PublicKeyBase64); err != nil || len(raw) != ed25519.PublicKeySize {
		t.Fatal("negative fixture must use the actual 32-byte legacy Ed25519 key")
	}
}
