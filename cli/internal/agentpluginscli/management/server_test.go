package management

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
)

type testBackend struct {
	makePlan func(ChangeRequest, string) Prepared
}

func (*testBackend) Session(context.Context) (Session, error)         { return Session{}, nil }
func (*testBackend) Catalog(context.Context, string) (Catalog, error) { return Catalog{}, nil }
func (*testBackend) CatalogDetail(context.Context, string) (CatalogEntry, error) {
	return CatalogEntry{}, nil
}
func (*testBackend) SkillsCatalog(context.Context, SourceRequest) (SkillCatalog, error) {
	return SkillCatalog{}, nil
}
func (*testBackend) Inspect(context.Context) (Environment, error) { return Environment{}, nil }
func (b *testBackend) Prepare(_ context.Context, in ChangeRequest, id string) (Prepared, error) {
	return b.makePlan(in, id), nil
}
func (b *testBackend) PrepareLocalSkill(_ context.Context, in LocalSkillRequest, id string) (Prepared, error) {
	return b.makePlan(ChangeRequest{Action: LocalSkill, OperationId: in.RestoreOperationId}, id), nil
}

type testPrepared struct {
	apply    func(context.Context) (Outcome, error)
	checkErr error
	closed   atomic.Int32
	calls    atomic.Int32
}

func (*testPrepared) Review() Plan {
	return Plan{Confirmable: true, Actions: []PlanAction{{Action: "change"}}}
}
func (p *testPrepared) Check(context.Context) error { return p.checkErr }
func (p *testPrepared) Apply(ctx context.Context, _ func(Progress)) (Outcome, error) {
	p.calls.Add(1)
	if p.apply != nil {
		return p.apply(ctx)
	}
	return knownSuccess(), nil
}
func (*testPrepared) OperationIDs() []string { return []string{"retained-native-id"} }
func (p *testPrepared) Close()               { p.closed.Add(1) }
func knownSuccess() Outcome {
	return Outcome{State: Succeeded, Result: OperationResult{Outcome: "completed", Mutated: true}}
}

func newTestServer(t *testing.T, root, capture string, backend *testBackend) *Server {
	t.Helper()
	if root == "" {
		root = t.TempDir()
	}
	if backend == nil {
		backend = &testBackend{makePlan: func(ChangeRequest, string) Prepared { return &testPrepared{} }}
	}
	s, err := Start(t.Context(), Options{Backend: backend, StateRoot: root, IntentRoot: filepath.Join(root, "intent"), EnvironmentID: "stable-environment", ContextID: capture, EngineVersion: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func requestJSON(t *testing.T, s *Server, method, path string, body any, status int, out any) {
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
func prepareTest(t *testing.T, s *Server, in ChangeRequest) Plan {
	t.Helper()
	var p Plan
	requestJSON(t, s, "POST", "/api/v1/plans", in, 200, &p)
	return p
}
func applyTest(t *testing.T, s *Server, p Plan, id string) Operation {
	t.Helper()
	var op Operation
	requestJSON(t, s, "POST", "/api/v1/plans/"+p.PlanId+"/apply", ApplyRequest{RequestId: id}, 202, &op)
	return op
}
func awaitTerminal(t *testing.T, s *Server, id string) Operation {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var op Operation
		requestJSON(t, s, "GET", "/api/v1/operations/"+id, nil, 200, &op)
		if terminal(op.State) {
			return op
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("operation did not finish")
	return Operation{}
}

func TestAcceptedReplaySurvivesExpiryAndNewCapturedContext(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	p := &testPrepared{apply: func(ctx context.Context) (Outcome, error) {
		close(started)
		select {
		case <-release:
			return knownSuccess(), nil
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		}
	}}
	backend := &testBackend{makePlan: func(ChangeRequest, string) Prepared { return p }}
	root := t.TempDir()
	s := newTestServer(t, root, "capture-before", backend)
	review := prepareTest(t, s, ChangeRequest{Action: Install, Source: "source"})
	op := applyTest(t, s, review, "request-one-123456")
	<-started
	s.mu.Lock()
	s.plans[review.PlanId].review.ExpiresAt = time.Now().Add(-time.Minute)
	s.mu.Unlock()
	replay := applyTest(t, s, review, "request-one-123456")
	if replay.OperationId != op.OperationId || p.calls.Load() != 1 {
		t.Fatal("accepted retry duplicated work")
	}
	var conflict APIError
	requestJSON(t, s, "POST", "/api/v1/plans/a-different-plan/apply", ApplyRequest{RequestId: "request-one-123456"}, 409, &conflict)
	if conflict.Code != "REQUEST_ID_CONFLICT" {
		t.Fatal(conflict)
	}
	close(release)
	done := awaitTerminal(t, s, op.OperationId)
	if done.State != Succeeded || done.Result == nil {
		t.Fatal(done)
	}
	s.Close()
	next := newTestServer(t, root, "capture-after", backend)
	replay = applyTest(t, next, review, "request-one-123456")
	if replay.OperationId != op.OperationId || replay.ContextId != "capture-before" || p.calls.Load() != 1 {
		t.Fatal("restart lost durable correlation")
	}
}

func TestSharedManagementLockAdmitsOnlyOneOwner(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	first := &testPrepared{apply: func(ctx context.Context) (Outcome, error) {
		close(started)
		select {
		case <-release:
			return knownSuccess(), nil
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		}
	}}
	root := t.TempDir()
	s1 := newTestServer(t, root, "one", &testBackend{makePlan: func(ChangeRequest, string) Prepared { return first }})
	s2 := newTestServer(t, root, "two", nil)
	a := prepareTest(t, s1, ChangeRequest{Action: Sync})
	b := prepareTest(t, s2, ChangeRequest{Action: Sync})
	op := applyTest(t, s1, a, "first-request-123456")
	<-started
	var refusal APIError
	requestJSON(t, s2, "POST", "/api/v1/plans/"+b.PlanId+"/apply", ApplyRequest{RequestId: "second-request-123456"}, 409, &refusal)
	if refusal.Code != "OPERATION_IN_PROGRESS" {
		t.Fatal(refusal)
	}
	close(release)
	awaitTerminal(t, s1, op.OperationId)
	second := applyTest(t, s2, b, "second-request-123456")
	if awaitTerminal(t, s2, second.OperationId).State != Succeeded {
		t.Fatal("second owner could not proceed after release")
	}
}

func TestWaitingDoesNotCancelAcceptedEffectsAndExplicitCancelDoes(t *testing.T) {
	started := make(chan struct{})
	p := &testPrepared{apply: func(ctx context.Context) (Outcome, error) {
		close(started)
		<-ctx.Done()
		return Outcome{State: Canceled, Result: OperationResult{Outcome: "unchanged"}}, ctx.Err()
	}}
	s := newTestServer(t, "", "captured", &testBackend{makePlan: func(ChangeRequest, string) Prepared { return p }})
	review := prepareTest(t, s, ChangeRequest{Action: Sync})
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
	live, ok := s.memoryOperation(op.OperationId)
	if !ok || live.State != Running {
		t.Fatal("wait cancellation changed accepted operation")
	}
	requestJSON(t, s, "POST", "/api/v1/operations/"+op.OperationId+"/cancel", nil, 202, nil)
	if got := awaitTerminal(t, s, op.OperationId); got.State != Canceled || got.Result == nil || got.Result.Mutated {
		t.Fatal(got)
	}
}

func TestStaleAndDiscardedReviewsHaveNoEffects(t *testing.T) {
	p := &testPrepared{checkErr: ErrPlanStale}
	s := newTestServer(t, "", "captured", &testBackend{makePlan: func(ChangeRequest, string) Prepared { return p }})
	review := prepareTest(t, s, ChangeRequest{Action: Sync})
	var refusal APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+review.PlanId+"/apply", ApplyRequest{RequestId: "stale-request-123456"}, 409, &refusal)
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
