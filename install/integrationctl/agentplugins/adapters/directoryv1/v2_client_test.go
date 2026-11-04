package directoryv1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
		Transport: Client{Origin: server.URL + "/registry/schemas/2/",
			Trust: trust, HTTPClient: server.Client(), AllowHTTPForTests: true,
			RequireEmbeddedBootstrap: true, Now: func() time.Time { return fixtureNow }},
		Cache: Cache{Path: filepath.Join(t.TempDir(), "v2-cache.json")},
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
		Cache: Cache{Path: filepath.Join(t.TempDir(), "schema-2.json")},
		Embedded: EmbeddedBundle{Snapshot: snapshot, Envelope: envelope},
	}
	client.Cache.Path = client.Transport.Cache.Path
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("v2 accepted same v1/v2 cache path: %v", err)
	}
	if _, err := client.LoadLocal(0); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("read-only v2 accepted same v1/v2 cache path: %v", err)
	}
	client.Cache.Path = filepath.Join(t.TempDir(), "schema-2.json")
	client.Embedded.Envelope = []byte("{}")
	if _, err := client.Load(context.Background(), 0); !errors.Is(err, ErrBootstrapNotReady) {
		t.Fatalf("v2 accepted invalid embedded signing authority: %v", err)
	}
}
