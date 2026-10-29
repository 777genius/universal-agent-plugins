package gitsource

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/internal/skillscatalog"
	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

type runnerFunc func(context.Context, sourceacquisition.Command) ([]byte, error)

func (f runnerFunc) Run(ctx context.Context, c sourceacquisition.Command) ([]byte, error) {
	return f(ctx, c)
}

func TestHostedRunnerRefusesFurtherWorkAfterUnconfirmedCleanup(t *testing.T) {
	scratch, calls := t.TempDir(), 0
	runner := &hostedRunner{runner: runnerFunc(func(context.Context, sourceacquisition.Command) ([]byte, error) {
		calls++
		return nil, errors.Join(context.DeadlineExceeded, sourceacquisition.ErrGitCleanupUnconfirmed)
	})}
	source := Source{Acquirer: skillsource.Acquirer{Acquirer: sourceacquisition.Acquirer{
		TempRoot: scratch, Runner: runner,
	}, Policy: Policy}}
	const token = "uncertain-git-cleanup-fixture-service-capability"
	handler := skillscatalog.NewResolveHandler(skillsresolve.New(source), token)
	for range 2 {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/skills/resolve", strings.NewReader(`{"repository":"example/fixture"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != 503 {
			t.Fatal("uncertain cleanup was not unavailable", response.Code)
		}
		assertClean(t, scratch)
	}
	if calls != 1 {
		t.Fatal("new Git work started after unconfirmed process cleanup", calls)
	}
}

func TestHostedRunnerCanRetryConfirmedTransportFailure(t *testing.T) {
	calls := 0
	runner := &hostedRunner{runner: runnerFunc(func(context.Context, sourceacquisition.Command) ([]byte, error) {
		calls++
		return nil, errors.New("confirmed transport failure")
	})}
	for range 2 {
		if _, err := runner.Run(t.Context(), sourceacquisition.Command{}); err == nil {
			t.Fatal("transport failure was ignored")
		}
	}
	if calls != 2 {
		t.Fatal("confirmed cleanup incorrectly poisoned the resolver", calls)
	}
}

func TestHostedRunnerRefusesConcurrentWorkAfterUnconfirmedCleanup(t *testing.T) {
	firstStarted, release, extraStarted := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	runner := &hostedRunner{runner: runnerFunc(func(context.Context, sourceacquisition.Command) ([]byte, error) {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-release
			return nil, sourceacquisition.ErrGitCleanupUnconfirmed
		}
		close(extraStarted)
		return []byte("unexpected Git launch"), nil
	})}
	finished := make(chan error, 2)
	run := func() { _, err := runner.Run(t.Context(), sourceacquisition.Command{}); finished <- err }
	go run()
	<-firstStarted
	go run()
	select {
	case <-extraStarted:
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	for range 2 {
		if err := <-finished; !errors.Is(err, sourceacquisition.ErrGitCleanupUnconfirmed) {
			t.Error("concurrent admission escaped cleanup uncertainty", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent Git work bypassed process cleanup admission", calls.Load())
	}
}
