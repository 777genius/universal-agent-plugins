package managementhttp

import (
	"context"
	"strings"
	"testing"
)

type catalogDiscardTestBackend struct {
	*testBackend
	calls chan string
}

func (b *catalogDiscardTestBackend) DiscardSkillsCatalog(_ context.Context, id string) error {
	b.calls <- id
	return nil
}

func TestSkillCatalogDiscardTransport(t *testing.T) {
	backend := &catalogDiscardTestBackend{testBackend: &testBackend{}, calls: make(chan string, 2)}
	server := newTestServer(t, "", "capture", backend)
	for range 2 {
		requestJSON(t, server, "DELETE", "/api/v1/skills/catalog/catalog-one", nil, 204, nil)
		if id := <-backend.calls; id != "catalog-one" {
			t.Fatal("discard changed the retained identity", id)
		}
	}
	requestJSON(t, server, "DELETE", "/api/v1/skills/catalog/"+strings.Repeat("x", 257), nil, 400, nil)
	if len(backend.calls) != 0 {
		t.Fatal("invalid identity reached the backend")
	}
	unsupported := newTestServer(t, "", "capture", nil)
	requestJSON(t, unsupported, "DELETE", "/api/v1/skills/catalog/catalog-one", nil, 422, nil)
}
