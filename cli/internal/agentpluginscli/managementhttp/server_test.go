package managementhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/managementstate"
)

type testBackend struct {
	makePlan func(management.ChangeRequest, string) management.Prepared
}

func (*testBackend) Session(context.Context) (management.Session, error) {
	return management.Session{}, nil
}
func (*testBackend) Catalog(context.Context, string) (management.Catalog, error) {
	return management.Catalog{}, nil
}
func (*testBackend) CatalogDetail(context.Context, string) (management.CatalogEntry, error) {
	return management.CatalogEntry{}, nil
}
func (*testBackend) SkillsCatalog(context.Context, management.SourceRequest) (management.SkillCatalog, error) {
	return management.SkillCatalog{}, nil
}
func (*testBackend) Inspect(context.Context) (management.Environment, error) {
	return management.Environment{}, nil
}
func (*testBackend) CheckUpdates(context.Context, management.UpdateCheckRequest) (management.UpdateCheck, error) {
	return management.UpdateCheck{}, nil
}
func (b *testBackend) Prepare(_ context.Context, in management.ChangeRequest, id string) (management.Prepared, error) {
	return b.makePlan(in, id), nil
}
func (b *testBackend) PrepareLocalSkill(_ context.Context, in management.LocalSkillRequest, id string) (management.Prepared, error) {
	return b.makePlan(management.ChangeRequest{Action: management.LocalSkill, OperationId: in.RestoreOperationId}, id), nil
}

type testPrepared struct {
	apply    func(context.Context) (management.Outcome, error)
	checkErr error
	closed   atomic.Int32
	calls    atomic.Int32
}

func (*testPrepared) Review() management.Plan {
	return management.Plan{Confirmable: true, Actions: []management.PlanAction{{Action: "change"}}}
}
func (p *testPrepared) Check(context.Context) error { return p.checkErr }
func (p *testPrepared) Apply(ctx context.Context, _ func(management.Progress)) (management.Outcome, error) {
	p.calls.Add(1)
	if p.apply != nil {
		return p.apply(ctx)
	}
	return knownSuccess(), nil
}
func (*testPrepared) OperationIDs() []string { return []string{"retained-native-id"} }
func (p *testPrepared) Close()               { p.closed.Add(1) }
func knownSuccess() management.Outcome {
	return management.Outcome{State: management.Succeeded, Result: management.OperationResult{Outcome: "completed", Mutated: true}}
}

type testServer struct {
	*Server
	store *managementstate.Store
	opts  management.Options
	root  string
}

func newTestServer(t *testing.T, root, capture string, backend management.Backend, lifetime ...time.Duration) *testServer {
	t.Helper()
	if root == "" {
		root = t.TempDir()
	}
	if backend == nil {
		backend = &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return &testPrepared{} }}
	}
	store, err := managementstate.NewStore(root, "stable-environment")
	if err != nil {
		t.Fatal(err)
	}
	lock, err := managementstate.NewLock(root)
	if err != nil {
		t.Fatal(err)
	}
	opts := management.Options{Backend: backend, Store: store, Lock: lock, IntentRoot: filepath.Join(root, "intent"), EnvironmentID: "stable-environment", ContextID: capture, EngineVersion: "test"}
	if len(lifetime) > 0 {
		opts.PlanLifetime = lifetime[0]
	}
	application, err := management.NewApplication(t.Context(), opts)
	if err != nil {
		t.Fatal(err)
	}
	server, err := Start(t.Context(), application)
	if err != nil {
		application.Close()
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	return &testServer{Server: server, store: store, opts: opts, root: root}
}

func awaitExpiry(t *testing.T, review management.Plan) {
	t.Helper()
	select {
	case <-time.After(time.Until(review.ExpiresAt)):
	case <-t.Context().Done():
		t.Fatal(t.Context().Err())
	}
}

func requestJSON(t *testing.T, s *testServer, method, path string, body any, status int, out any) {
	t.Helper()
	var data io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(t.Context(), method, s.URL()+path, data)
	if err != nil {
		t.Fatal(err)
	}
	boot := s.Bootstrap()
	req.Header.Set("Authorization", "Bearer "+boot.Token)
	req.Header.Set("Origin", boot.Origin)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	encoded, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != status {
		t.Fatalf("%s %s: status %d want %d: %s", method, path, res.StatusCode, status, encoded)
	}
	if out != nil {
		if err = json.Unmarshal(encoded, out); err != nil {
			t.Fatalf("decode %s: %v", encoded, err)
		}
	}
}
func prepareTest(t *testing.T, s *testServer, in management.ChangeRequest) management.Plan {
	t.Helper()
	var p management.Plan
	requestJSON(t, s, "POST", "/api/v1/plans", in, 200, &p)
	return p
}
func applyTest(t *testing.T, s *testServer, p management.Plan, id string) management.Operation {
	t.Helper()
	var op management.Operation
	requestJSON(t, s, "POST", "/api/v1/plans/"+p.PlanId+"/apply", management.ApplyRequest{RequestId: id}, 202, &op)
	return op
}
func awaitTerminal(t *testing.T, s *testServer, id string) management.Operation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var op management.Operation
		requestJSON(t, s, "GET", "/api/v1/operations/"+id, nil, 200, &op)
		if op.State != management.Queued && op.State != management.Running {
			return op
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operation did not finish")
	return management.Operation{}
}

func TestAcceptedReplaySurvivesExpiryAndNewCapturedContext(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	p := &testPrepared{apply: func(ctx context.Context) (management.Outcome, error) {
		close(started)
		select {
		case <-release:
			return knownSuccess(), nil
		case <-ctx.Done():
			return management.Outcome{}, ctx.Err()
		}
	}}
	backend := &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return p }}
	root := t.TempDir()
	s := newTestServer(t, root, "capture-before", backend, time.Second)
	review := prepareTest(t, s, management.ChangeRequest{Action: management.Install, Source: "source"})
	op := applyTest(t, s, review, "request-one-123456")
	<-started
	awaitExpiry(t, review)
	replay := applyTest(t, s, review, "request-one-123456")
	if replay.OperationId != op.OperationId || p.calls.Load() != 1 {
		t.Fatal("accepted retry duplicated work")
	}
	var conflict management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans/a-different-plan/apply", management.ApplyRequest{RequestId: "request-one-123456"}, 409, &conflict)
	if conflict.Code != "REQUEST_ID_CONFLICT" {
		t.Fatal(conflict)
	}
	close(release)
	done := awaitTerminal(t, s, op.OperationId)
	if done.State != management.Succeeded || done.Result == nil {
		t.Fatal(done)
	}
	s.Close()
	next := newTestServer(t, root, "capture-after", backend)
	replay = applyTest(t, next, review, "request-one-123456")
	if replay.OperationId != op.OperationId || replay.ContextId != "capture-before" || p.calls.Load() != 1 {
		t.Fatal("restart lost durable correlation")
	}
}

func TestUnacceptedExpiredReviewDoesNotConsumeRequestIdentity(t *testing.T) {
	expired, fresh := &testPrepared{}, &testPrepared{}
	var preparations atomic.Int32
	s := newTestServer(t, "", "captured", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared {
		if preparations.Add(1) == 1 {
			return expired
		}
		return fresh
	}}, time.Second)
	review := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	awaitExpiry(t, review)
	const requestID = "expired-review-123456"
	var refusal management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+review.PlanId+"/apply", management.ApplyRequest{RequestId: requestID}, 409, &refusal)
	if refusal.Code != "PLAN_STALE" || expired.calls.Load() != 0 {
		t.Fatal("unaccepted expired review reached backend effects", refusal, expired.calls.Load())
	}
	if record, err := s.store.ByRequest(requestID); err != nil || record != nil {
		t.Fatal("expired review reserved durable request identity", record, err)
	}

	replacement := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	op := applyTest(t, s, replacement, requestID)
	if done := awaitTerminal(t, s, op.OperationId); done.State != management.Succeeded || fresh.calls.Load() != 1 || expired.calls.Load() != 0 {
		t.Fatal("fresh review could not use the refused request identity", done, fresh.calls.Load(), expired.calls.Load())
	}
	if record, err := s.store.ByRequest(requestID); err != nil || record == nil || record.Operation.PlanId != replacement.PlanId {
		t.Fatal("request identity was not accepted for the fresh review", record, err)
	}
}

func TestSharedManagementLockAdmitsOnlyOneOwner(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	first := &testPrepared{apply: func(ctx context.Context) (management.Outcome, error) {
		close(started)
		select {
		case <-release:
			return knownSuccess(), nil
		case <-ctx.Done():
			return management.Outcome{}, ctx.Err()
		}
	}}
	root := t.TempDir()
	s1 := newTestServer(t, root, "one", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return first }})
	s2 := newTestServer(t, root, "two", nil)
	a := prepareTest(t, s1, management.ChangeRequest{Action: management.Sync})
	b := prepareTest(t, s2, management.ChangeRequest{Action: management.Sync})
	op := applyTest(t, s1, a, "first-request-123456")
	<-started
	var refusal management.APIError
	requestJSON(t, s2, "POST", "/api/v1/plans/"+b.PlanId+"/apply", management.ApplyRequest{RequestId: "second-request-123456"}, 409, &refusal)
	if refusal.Code != "OPERATION_IN_PROGRESS" {
		t.Fatal(refusal)
	}
	close(release)
	awaitTerminal(t, s1, op.OperationId)
	second := applyTest(t, s2, b, "second-request-123456")
	if awaitTerminal(t, s2, second.OperationId).State != management.Succeeded {
		t.Fatal("second owner could not proceed after release")
	}
}

func TestWaitingDoesNotCancelAcceptedEffectsAndExplicitCancelDoes(t *testing.T) {
	started := make(chan struct{})
	p := &testPrepared{apply: func(ctx context.Context) (management.Outcome, error) {
		close(started)
		<-ctx.Done()
		return management.Outcome{State: management.Canceled, Result: management.OperationResult{Outcome: "unchanged"}}, ctx.Err()
	}}
	s := newTestServer(t, "", "captured", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return p }})
	review := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	op := applyTest(t, s, review, "cancellation-123456")
	<-started
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", s.URL()+"/api/v1/operations/"+op.OperationId, http.NoBody)
	res, err := http.DefaultClient.Do(req)
	if res != nil {
		defer func() { _ = res.Body.Close() }()
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled wait, got %v", err)
	}
	var live management.Operation
	requestJSON(t, s, "GET", "/api/v1/operations/"+op.OperationId, nil, 200, &live)
	if live.State != management.Running {
		t.Fatal("wait cancellation changed accepted operation")
	}
	requestJSON(t, s, "POST", "/api/v1/operations/"+op.OperationId+"/cancel", nil, 202, nil)
	if got := awaitTerminal(t, s, op.OperationId); got.State != management.Canceled || got.Result == nil || got.Result.Mutated {
		t.Fatal(got)
	}
}

func TestStaleAndDiscardedReviewsHaveNoEffects(t *testing.T) {
	p := &testPrepared{checkErr: management.ErrPlanStale}
	s := newTestServer(t, "", "captured", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return p }})
	review := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	var refusal management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+review.PlanId+"/apply", management.ApplyRequest{RequestId: "stale-request-123456"}, 409, &refusal)
	if refusal.Code != "PLAN_STALE" || p.calls.Load() != 0 {
		t.Fatal("stale review changed files")
	}
	requestJSON(t, s, "DELETE", "/api/v1/plans/"+review.PlanId, nil, 204, nil)
	requestJSON(t, s, "DELETE", "/api/v1/plans/"+review.PlanId, nil, 204, nil)
	if p.closed.Load() != 1 {
		t.Fatal("discard did not release retained authority once")
	}
}

func TestHTTPAuthorityAndStrictJSON(t *testing.T) {
	s := newTestServer(t, "", "captured", nil)
	for _, tc := range []struct {
		name, token, origin, host, body string
		status                          int
	}{
		{name: "missing token", origin: s.URL(), body: `{"action":"sync"}`, status: 401},
		{name: "wrong origin", token: s.token, origin: "https://untrusted.example", body: `{"action":"sync"}`, status: 403},
		{name: "host alias", token: s.token, origin: s.URL(), host: "localhost:1", body: `{"action":"sync"}`, status: 403},
		{name: "unknown field", token: s.token, origin: s.URL(), body: `{"action":"sync","authority":"forged"}`, status: 400},
		{name: "two documents", token: s.token, origin: s.URL(), body: `{"action":"sync"}{}`, status: 400},
		{name: "oversized body", token: s.token, origin: s.URL(), body: `{"action":"sync","source":"` + strings.Repeat("x", 1<<20) + `"}`, status: 400},
		{name: "local action wrong route", token: s.token, origin: s.URL(), body: `{"action":"local_skill"}`, status: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(t.Context(), "POST", s.URL()+"/api/v1/plans", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set("Origin", tc.origin)
			if tc.host != "" {
				req.Host = tc.host
			}
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = res.Body.Close() }()
			if res.StatusCode != tc.status {
				t.Fatalf("status %d want %d", res.StatusCode, tc.status)
			}
		})
	}
}
