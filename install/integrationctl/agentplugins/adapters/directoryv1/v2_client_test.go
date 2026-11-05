package directoryv1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestV2ClientFreshBootstrapAndOfflineExactCache(t *testing.T) {
	snapshot, envelope, trust := v2SignedFixture(t)
	pointer := v2PointerFixture(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/registry/schemas/2/latest.json":
			data, _ := json.Marshal(pointer)
			_, _ = w.Write(data)
		case "/registry/schemas/2/snapshots/00000000000000000001.json":
			_, _ = w.Write(snapshot)
		case "/registry/schemas/2/snapshots/00000000000000000001.envelope.json":
			_, _ = w.Write(envelope)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	client := V2Client{
		Transport: Client{
			Origin:                   server.URL + "/registry/schemas/2/",
			Trust:                    trust,
			HTTPClient:               server.Client(),
			AllowHTTPForTests:        true,
			RequireEmbeddedBootstrap: true,
			Now:                      func() time.Time { return fixtureNow },
		},
		Cache:    Cache{Path: filepath.Join(t.TempDir(), "v2-cache.json")},
		Embedded: EmbeddedBundle{Snapshot: snapshot, Envelope: envelope},
	}
	accepted, err := client.Load(context.Background(), 0)
	if err != nil || accepted.Source != BundleSourceRemote {
		t.Fatalf("v2 fresh bootstrap rejected: %+v %v", accepted, err)
	}
	server.Close()
	offline, err := client.Load(context.Background(), 1)
	if err != nil || offline.Digest != accepted.Digest {
		t.Fatalf("v2 offline cache could not preserve exact signature: %+v %v", offline, err)
	}
	if _, err := client.Load(context.Background(), 2); !errors.Is(err, ErrRollback) {
		t.Fatalf("installed high-water mark was ignored: %v", err)
	}
	if _, err := client.Transport.fetchV2(context.Background()); err == nil {
		t.Fatal("network fixture should no longer be available")
	}
	// Read-only inspection may show expired signed provenance; it must never
	// enable an install from a stale snapshot or ignore receipt floors.
	client.Transport.Now = func() time.Time { return fixtureNow.Add(365 * 24 * time.Hour) }
	local, err := client.LoadLocal(1)
	if err != nil || local.Digest != accepted.Digest || local.Source != BundleSourceCache {
		t.Fatalf("v2 read-only offline provenance unavailable: %+v %v", local, err)
	}
	if _, err := client.LoadLocal(2); !errors.Is(err, ErrRollback) {
		t.Fatalf("v2 read-only inspection ignored installed floor: %v", err)
	}
	if _, err := client.Load(context.Background(), 1); !errors.Is(err, ErrExpired) {
		t.Fatalf("expired v2 provenance was allowed to authorize install: %v", err)
	}
}

func TestV2ClientRefusesV1BootstrapAndSharedCachePath(t *testing.T) {
	snapshot, envelope, trust := v2SignedFixture(t)
	client := V2Client{
		Transport: Client{Trust: trust, RequireEmbeddedBootstrap: true,
			Cache: Cache{Path: filepath.Join(t.TempDir(), "schema-1.json")}},
		Cache:    Cache{Path: filepath.Join(t.TempDir(), "schema-2.json")},
		Embedded: EmbeddedBundle{Snapshot: snapshot, Envelope: envelope},
	}
	client.Cache.Path = client.Transport.Cache.Path
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("v2 accepted same v1/v2 cache path: %v", err)
	}
	if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read-only v2 accepted same v1/v2 cache path: %v", err)
	}
	// Equivalent lexical paths must not bypass the v1/v2 cache boundary.
	client.Cache.Path = filepath.Dir(client.Transport.Cache.Path) + "/./" + filepath.Base(client.Transport.Cache.Path)
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("v2 accepted normalized v1 cache alias: %v", err)
	}
	if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read-only v2 accepted normalized v1 cache alias: %v", err)
	}
	client.Cache.Path = filepath.Join(t.TempDir(), "schema-2.json")
	client.Embedded.Envelope = []byte("{}")
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrBootstrapNotReady) {
		t.Fatalf("v2 accepted invalid embedded signing authority: %v", err)
	}
}

func TestV2ClientRejectsCacheAliasesBeforeNetworkOrWrites(t *testing.T) {
	_, _, trust := v2SignedFixture(t)
	root := t.TempDir()
	realRoot := filepath.Join(root, "real")
	if err := os.Mkdir(realRoot, 0700); err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, v1Path, v2Path string) {
		t.Helper()
		client := V2Client{
			Transport: Client{Trust: trust, Cache: Cache{Path: v1Path}},
			Cache:     Cache{Path: v2Path},
		}
		if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) ||
			!strings.Contains(err.Error(), "v1/v2 cache path collision") {
			t.Fatalf("install-time loader accepted cache alias: %v", err)
		}
		if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) ||
			!strings.Contains(err.Error(), "v1/v2 cache path collision") {
			t.Fatalf("read-only loader accepted cache alias: %v", err)
		}
		if _, err := os.Stat(v1Path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("alias refusal changed v1 cache: %v", err)
		}
	}

	t.Run("relative and absolute aliases", func(t *testing.T) {
		absolute := filepath.Join(realRoot, "shared.json")
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		relative, err := filepath.Rel(cwd, absolute)
		if err != nil {
			t.Skipf("relative alias cannot span OS volumes: %v", err)
		}
		check(t, absolute, relative)
	})
	t.Run("case-folded cache filename aliases", func(t *testing.T) {
		check(t, filepath.Join(realRoot, "mixed-case-cache.json"), filepath.Join(realRoot, "MIXED-CASE-CACHE.JSON"))
	})
	t.Run("symlink parent of uncreated cache", func(t *testing.T) {
		alias := filepath.Join(root, "alias")
		if err := os.Symlink(realRoot, alias); err != nil {
			t.Skipf("directory symlinks unavailable on this host: %v", err)
		}
		check(t, filepath.Join(realRoot, "future", "shared.json"), filepath.Join(alias, "future", "shared.json"))
	})
}

func TestV2ClientRejectsExistingCacheFileAliases(t *testing.T) {
	_, _, trust := v2SignedFixture(t)
	root := t.TempDir()
	original := filepath.Join(root, "v1-cache.json")
	const marker = "foreign cache must remain untouched"
	if err := os.WriteFile(original, []byte(marker), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		link func(string, string) error
	}{
		{"file symlink", os.Symlink},
		{"hard link", os.Link},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alias := filepath.Join(root, strings.ReplaceAll(tc.name, " ", "-")+".json")
			if err := tc.link(original, alias); err != nil {
				t.Skipf("file alias unavailable on this host: %v", err)
			}
			client := V2Client{
				Transport: Client{Trust: trust, Cache: Cache{Path: original}},
				Cache:     Cache{Path: alias},
			}
			if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) ||
				!strings.Contains(err.Error(), "v1/v2 cache path collision") {
				t.Fatalf("install-time v2 accepted existing file alias: %v", err)
			}
			if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) ||
				!strings.Contains(err.Error(), "v1/v2 cache path collision") {
				t.Fatalf("read-only v2 accepted existing file alias: %v", err)
			}
			if content, err := os.ReadFile(original); err != nil || string(content) != marker {
				t.Fatalf("existing v1 cache mutated: %q, %v", content, err)
			}
		})
	}
}

func TestV2ClientRejectsDanglingCacheFilenameSymlink(t *testing.T) {
	_, _, trust := v2SignedFixture(t)
	root := t.TempDir()
	dangling := filepath.Join(root, "dangling-v2.json")
	if err := os.Symlink(filepath.Join(root, "uncreated-target.json"), dangling); err != nil {
		t.Skipf("file symlinks unavailable on this host: %v", err)
	}
	client := V2Client{
		Transport: Client{Trust: trust, Cache: Cache{Path: filepath.Join(root, "v1.json")}},
		Cache:     Cache{Path: dangling},
	}
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("v2 accepted dangling file symlink: %v", err)
	}
	if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read-only v2 accepted dangling file symlink: %v", err)
	}
}
