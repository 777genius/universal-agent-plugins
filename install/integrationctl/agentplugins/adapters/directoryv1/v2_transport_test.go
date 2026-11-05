package directoryv1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestV2TransportFetchesExactScopedSignedArtifacts(t *testing.T) {
	snapshot, envelope, trust := v2SignedFixture(t)
	pointer := v2PointerFixture(t)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/registry/schemas/2/latest.json":
			_ = json.NewEncoder(w).Encode(pointer)
		case "/registry/schemas/2/snapshots/00000000000000000001.json":
			_, _ = w.Write(snapshot)
		case "/registry/schemas/2/snapshots/00000000000000000001.envelope.json":
			_, _ = w.Write(envelope)
		default:
			http.NotFound(w, r)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()
	client := Client{Origin: server.URL + "/registry/schemas/2/",
		Trust: trust, AllowHTTPForTests: true, HTTPClient: server.Client()}
	got, err := client.fetchV2(context.Background())
	if err != nil || got.Source != BundleSourceRemote || got.Snapshot.Sequence != 1 ||
		got.Snapshot.SnapshotSchemaVersion != 2 {
		t.Fatalf("v2 authenticated transport rejected: %+v %v", got, err)
	}
	// The v1 client must not treat the same signed v2 response as its own feed.
	if _, err := client.fetch(context.Background()); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v1 transport silently accepted v2 latest pointer: %v", err)
	}
	pointer.FetchContract.ForwardCredentialsOnRedirect = true
	if _, err := client.fetchV2(context.Background()); !errors.Is(err, ErrStrictJSON) {
		t.Fatalf("v2 accepted credential-forwarding pointer: %v", err)
	}
	pointer.FetchContract.ForwardCredentialsOnRedirect = false
	snapshot = append([]byte(nil), snapshot...)
	snapshot[len(snapshot)-2] ^= 1
	if _, err := client.fetchV2(context.Background()); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("v2 transport did not reject modified snapshot: %v", err)
	}
}
