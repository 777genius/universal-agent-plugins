package managementhttp

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

type blockedSessionBackend struct {
	*testBackend
	started, canceled, release chan struct{}
	closes                     atomic.Int32
}

func (b *blockedSessionBackend) Session(ctx context.Context) (management.Session, error) {
	close(b.started)
	<-ctx.Done()
	close(b.canceled)
	<-b.release
	return management.Session{}, ctx.Err()
}

func (b *blockedSessionBackend) Close() { b.closes.Add(1) }

func TestShutdownClosesTransportAndWaitsForAdmittedBackend(t *testing.T) {
	backend := &blockedSessionBackend{testBackend: &testBackend{}, started: make(chan struct{}), canceled: make(chan struct{}), release: make(chan struct{})}
	var release sync.Once
	defer release.Do(func() { close(backend.release) })
	server := newTestServer(t, "", "capture", backend)
	bootstrap := server.Bootstrap()
	request, err := http.NewRequestWithContext(t.Context(), "GET", server.URL()+"/api/v1/session", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+bootstrap.Token)
	requestDone := make(chan struct{})
	go func() {
		defer close(requestDone)
		response, _ := http.DefaultClient.Do(request)
		if response != nil {
			_ = response.Body.Close()
		}
	}()
	awaitSignal(t, backend.started)
	go server.Close()
	awaitSignal(t, backend.canceled)
	// The connection is already closed, but application cleanup must wait for
	// the admitted backend call to release its resources.
	awaitSignal(t, requestDone)
	select {
	case <-server.Done():
		t.Fatal("shutdown completed while the backend call was still admitted")
	default:
	}
	if backend.closes.Load() != 0 {
		t.Fatal("backend closed before its admitted call returned")
	}
	release.Do(func() { close(backend.release) })
	awaitSignal(t, server.Done())
	server.Close()
	if backend.closes.Load() != 1 {
		t.Fatal("shutdown did not close the backend exactly once", backend.closes.Load())
	}
}

func awaitSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle signal did not arrive")
	}
}
