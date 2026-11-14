package skillspicker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// This boundary test fails if accepted apply inherits the HTTP request context,
// concurrent confirmations cause two effects, or refresh loses a fast result.
func TestTransportAdmitsOneEffectAndRetainsTerminalOperation(t *testing.T) {
	release := make(chan struct{})
	started := make(chan context.Context, 2)
	var effects atomic.Int32
	var closes atomic.Int32
	callbacks := testCallbacks(func() Prepared {
		return &testPrepared{close: func() { closes.Add(1) }, apply: func(ctx context.Context) (Outcome, error) {
			effects.Add(1)
			started <- ctx
			select {
			case <-release:
			case <-ctx.Done():
				return Outcome{}, ctx.Err()
			}
			return Outcome{State: "partial", Items: []OperationItem{{CandidateId: "candidate", TargetId: "codex", Status: "failed"}}, Result: OperationResult{Message: "One target failed", RepairInstructions: []string{"Run repair before retrying"}}}, nil
		}}
	})
	server, err := Start(context.Background(), Options{Source: Source{Repository: "owner/repo"}, Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	plan1 := client.plan()
	plan2 := client.plan()
	applyContext, cancel := context.WithCancel(context.Background())
	status, body := client.request(applyContext, "POST", "/api/v1/plans/"+plan1.PlanId+"/apply", "", nil)
	if status != 202 {
		t.Fatalf("apply: %d %s", status, body)
	}
	cancel()
	var accepted ApplyResponse
	mustDecode(t, body, &accepted)
	workerContext := <-started
	if workerContext.Err() != nil {
		t.Fatalf("HTTP completion canceled apply: %v", workerContext.Err())
	}
	status, body = client.request(context.Background(), "POST", "/api/v1/plans/"+plan2.PlanId+"/apply", "", nil)
	if status != 409 || !bytes.Contains(body, []byte("OPERATION_IN_PROGRESS")) {
		t.Fatalf("second plan: %d %s", status, body)
	}
	status, body = client.request(context.Background(), "POST", "/api/v1/plans/"+plan1.PlanId+"/apply", "", nil)
	var repeated ApplyResponse
	mustDecode(t, body, &repeated)
	if status != 202 || repeated.OperationId != accepted.OperationId {
		t.Fatalf("repeat: %d %s", status, body)
	}
	close(release)
	var operation Operation
	eventually(t, func() bool {
		status, body = client.request(context.Background(), "GET", "/api/v1/operations/"+accepted.OperationId, "", nil)
		mustDecode(t, body, &operation)
		return operation.State == "partial"
	})
	if len(operation.Items) != 1 || operation.Items[0].Status != "failed" || operation.Result == nil || len(operation.Result.RepairInstructions) != 1 {
		t.Fatalf("partial outcome lost: %+v", operation)
	}
	status, body = client.request(context.Background(), "GET", "/api/v1/session", "", nil)
	var session Session
	mustDecode(t, body, &session)
	if session.OperationId == nil || *session.OperationId != accepted.OperationId || session.State != "failed" {
		t.Fatalf("refresh lost operation: %s", body)
	}
	status, body = client.request(context.Background(), "POST", "/api/v1/plans/"+plan1.PlanId+"/apply", "", nil)
	mustDecode(t, body, &repeated)
	if effects.Load() != 1 || repeated.OperationId != accepted.OperationId {
		t.Fatalf("effect repeated: %d %s", effects.Load(), body)
	}
	eventually(t, func() bool { return closes.Load() == 2 })
	if closes.Load() != 2 {
		t.Fatalf("accepted/discarded prepared workspaces not released: %d", closes.Load())
	}
}

// This boundary catches capability/Origin bypass, malformed JSON admission,
// arbitrary IDs reaching application policy, and stale plans creating effects.
func TestTransportValidatesAuthorityInputsAndStaleFacts(t *testing.T) {
	var effects, preparations atomic.Int32
	callbacks := testCallbacks(func() Prepared {
		preparations.Add(1)
		return &testPrepared{check: func(context.Context) error { return ErrPlanStale }, apply: func(context.Context) (Outcome, error) { effects.Add(1); return Outcome{State: "succeeded"}, nil }}
	})
	server, err := Start(context.Background(), Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	for _, test := range []struct {
		method, path, body string
		headers            map[string]string
		status             int
	}{
		{"GET", "/api/v1/session", "", map[string]string{"Authorization": ""}, 401},
		{"GET", "/api/v1/session", "", map[string]string{"Host": "attacker.invalid"}, 403},
		{"POST", "/api/v1/catalog", `{"scopePath":""}`, map[string]string{"Origin": "https://attacker.invalid"}, 403},
		{"POST", "/api/v1/catalog", `{}`, nil, 400},
		{"POST", "/api/v1/catalog", `{"scopePath":"../escape"}`, nil, 400},
		{"POST", "/api/v1/catalog", `{"scopePath":"","shell":"rm"}`, nil, 400},
		{"POST", "/api/v1/catalog", `{"scopePath":""} {}`, nil, 400},
		{"POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["arbitrary-path"],"targetIds":["codex"]}`, nil, 400},
		{"POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["candidate"],"targetIds":["unknown-target"]}`, nil, 422},
	} {
		status, body := client.request(context.Background(), test.method, test.path, test.body, test.headers)
		if status != test.status {
			t.Fatalf("%s %s: got %d want %d: %s", test.method, test.path, status, test.status, body)
		}
	}
	if preparations.Load() != 0 {
		t.Fatalf("invalid IDs reached prepare: %d", preparations.Load())
	}
	plan := client.plan()
	status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
	if status != 409 || !bytes.Contains(body, []byte("PLAN_STALE")) || effects.Load() != 0 {
		t.Fatalf("stale applied: %d %s effects=%d", status, body, effects.Load())
	}
	status, body = client.request(context.Background(), "GET", "/api/v1/catalog", "", nil)
	if status != 200 || bytes.Contains(body, []byte(":null")) {
		t.Fatalf("required arrays: %d %s", status, body)
	}
}

// A scan is retained across HTTP 202 and prevents idle expiry. Once idle, the
// session returns 410 and frees review resources without accepting another apply.
func TestTransportScanLifetimeRescanAndExpiry(t *testing.T) {
	release := make(chan struct{})
	scopes := make(chan string, 3)
	var scans atomic.Int32
	var closed atomic.Int32
	callbacks := testCallbacks(func() Prepared { return &testPrepared{close: func() { closed.Add(1) }} })
	callbacks.Scan = func(ctx context.Context, scope string) (Catalog, error) {
		scopes <- scope
		if scans.Add(1) == 2 {
			select {
			case <-release:
			case <-ctx.Done():
				return Catalog{}, ctx.Err()
			}
		}
		return testCatalog(scope), nil
	}
	server, err := Start(context.Background(), Options{Source: Source{Scope: "narrow", SourceRevision: stringPtr("same-sha")}, Callbacks: callbacks, IdleLifetime: 80 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	<-scopes
	client.plan()
	requestCtx, cancel := context.WithCancel(context.Background())
	status, body := client.request(requestCtx, "POST", "/api/v1/catalog", `{"scopePath":""}`, nil)
	cancel()
	if status != 202 {
		t.Fatalf("rescan: %d %s", status, body)
	}
	var scanSession Session
	mustDecode(t, body, &scanSession)
	if scanSession.State != "scanning" {
		t.Fatalf("202 is not session: %s", body)
	}
	<-scopes
	time.Sleep(160 * time.Millisecond)
	status, body = client.request(context.Background(), "GET", "/api/v1/session", "", nil)
	if status != 200 {
		t.Fatalf("scan expired with active worker: %d %s", status, body)
	}
	close(release)
	client.waitReady()
	if closed.Load() != 1 {
		t.Fatalf("rescan retained an unaccepted review: %d", closed.Load())
	}
	select {
	case <-server.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("idle session did not expire")
	}
	status, body = client.request(context.Background(), "GET", "/api/v1/session", "", nil)
	if status != 410 || !bytes.Contains(body, []byte("SESSION_EXPIRED")) {
		t.Fatalf("expiry: %d %s", status, body)
	}
}

type testPrepared struct {
	check func(context.Context) error
	apply func(context.Context) (Outcome, error)
	close func()
}

func (p *testPrepared) Review() Plan {
	return Plan{Confirmable: true, Actions: []Action{{CandidateId: "candidate", TargetId: "codex", Alias: "review", Action: "install", DestinationPath: "/disposable/skills/review"}}}
}
func (p *testPrepared) Check(ctx context.Context) error {
	if p.check != nil {
		return p.check(ctx)
	}
	return nil
}
func (p *testPrepared) Apply(ctx context.Context) (Outcome, error) {
	if p.apply != nil {
		return p.apply(ctx)
	}
	return Outcome{State: "succeeded"}, nil
}
func (p *testPrepared) Close() {
	if p.close != nil {
		p.close()
	}
}
func testCatalog(scope string) Catalog {
	return Catalog{CatalogId: "catalog", CatalogDigest: "digest", Repository: "owner/repo", SourceRevision: "same-sha", Scope: scope, Complete: true, Counts: CatalogCounts{Total: 1, Available: 1}, Candidates: []Candidate{{Id: "candidate", Name: "review", Path: "skills/review", Status: "available"}}}
}
func testCallbacks(prepare func() Prepared) Callbacks {
	return Callbacks{Scan: func(_ context.Context, scope string) (Catalog, error) { return testCatalog(scope), nil }, Preview: func(context.Context, string) (Preview, error) { return Preview{Format: "markdown"}, nil }, Targets: func(context.Context) ([]Target, error) { return []Target{{Id: "codex", Available: true}}, nil }, Prepare: func(context.Context, PlanRequest) (Prepared, error) { return prepare(), nil }}
}

type pickerClient struct {
	t             *testing.T
	origin, token string
	http          *http.Client
}

func newPickerClient(t *testing.T, s *Server) *pickerClient {
	t.Helper()
	u, err := url.Parse(s.URL())
	if err != nil {
		t.Fatal(err)
	}
	fragment, _ := url.ParseQuery(u.Fragment)
	token := fragment.Get("capability")
	if len(token) != 64 {
		t.Fatalf("capability must be 256 bits: %q", token)
	}
	u.Fragment = ""
	return &pickerClient{t: t, origin: strings.TrimSuffix(u.String(), "/"), token: token, http: &http.Client{Timeout: 2 * time.Second}}
}
func (c *pickerClient) request(ctx context.Context, method, path, body string, headers map[string]string) (int, []byte) {
	c.t.Helper()
	request, err := http.NewRequestWithContext(ctx, method, c.origin+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	request.Header.Set("Origin", c.origin)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		if key == "Host" {
			request.Host = value
		} else {
			request.Header.Set(key, value)
		}
	}
	response, err := c.http.Do(request)
	if err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	result, err := io.ReadAll(response.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	return response.StatusCode, result
}
func (c *pickerClient) waitReady() {
	c.t.Helper()
	eventually(c.t, func() bool {
		status, body := c.request(context.Background(), "GET", "/api/v1/session", "", nil)
		if status != 200 {
			c.t.Fatalf("session %d %s", status, body)
		}
		var result Session
		mustDecode(c.t, body, &result)
		return result.State == "ready"
	})
}
func (c *pickerClient) plan() Plan {
	c.t.Helper()
	status, body := c.request(context.Background(), "POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["candidate"],"targetIds":["codex"]}`, nil)
	if status != 200 {
		c.t.Fatalf("plan %d %s", status, body)
	}
	var plan Plan
	mustDecode(c.t, body, &plan)
	return plan
}
func mustDecode(t *testing.T, body []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(body, v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
}
func stringPtr(value string) *string { return &value }
func eventually(t *testing.T, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("timed out waiting for picker state")
}

// Retained accepted reviews serve idempotency; they must not consume the bound
// on temporary unaccepted review workspaces or permanently exhaust a session.
func TestTransportAcceptedPlanRetentionDoesNotExhaustReviewLimit(t *testing.T) {
	var effects atomic.Int32
	callbacks := testCallbacks(func() Prepared {
		return &testPrepared{apply: func(context.Context) (Outcome, error) { effects.Add(1); return Outcome{State: "succeeded"}, nil }}
	})
	server, err := Start(context.Background(), Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	var firstPlanID, firstOperationID string
	for i := range 17 {
		plan := client.plan()
		status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
		if status != 202 {
			t.Fatalf("apply %d: %d %s", i, status, body)
		}
		var accepted ApplyResponse
		mustDecode(t, body, &accepted)
		if i == 0 {
			firstPlanID = plan.PlanId
			firstOperationID = accepted.OperationId
		}
		eventually(t, func() bool {
			_, body := client.request(context.Background(), "GET", "/api/v1/operations/"+accepted.OperationId, "", nil)
			var operation Operation
			mustDecode(t, body, &operation)
			return operation.State == "succeeded"
		})
	}
	status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+firstPlanID+"/apply", "", nil)
	var repeated ApplyResponse
	mustDecode(t, body, &repeated)
	if status != 202 || repeated.OperationId != firstOperationID || effects.Load() != 17 {
		t.Fatalf("retained operation repeated: %d %s effects=%d", status, body, effects.Load())
	}
}

// Expiry must be checked at admission, after a potentially slow read-only
// installer preflight, rather than only when confirmation first arrives.
func TestTransportPlanExpiringDuringCheckHasNoEffects(t *testing.T) {
	var effects atomic.Int32
	callbacks := testCallbacks(func() Prepared {
		return &testPrepared{check: func(context.Context) error { time.Sleep(200 * time.Millisecond); return nil }, apply: func(context.Context) (Outcome, error) { effects.Add(1); return Outcome{State: "succeeded"}, nil }}
	})
	server, err := Start(context.Background(), Options{Callbacks: callbacks, PlanLifetime: 150 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	plan := client.plan()
	status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
	if status != 409 || !bytes.Contains(body, []byte("PLAN_STALE")) || effects.Load() != 0 {
		t.Fatalf("expired confirmation created effects: %d %s effects=%d", status, body, effects.Load())
	}
}

// Application-selected guidance survives transport while wrapped provider text
// stays private. This also makes a rejected review actionable in the browser.
func TestTransportReviewRefusalExposesOnlySafeGuidance(t *testing.T) {
	callbacks := testCallbacks(func() Prepared { return &testPrepared{} })
	callbacks.Prepare = func(context.Context, PlanRequest) (Prepared, error) {
		return nil, fmt.Errorf("provider credential=private-secret: %w", &ReviewRefusal{Code: "SECURITY_REVIEW_BLOCKED", Message: "Remove the blocked Skill and review the remaining selection."})
	}
	server, err := Start(context.Background(), Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	status, body := client.request(context.Background(), "POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["candidate"],"targetIds":["codex"]}`, nil)
	if status != 422 || !bytes.Contains(body, []byte("SECURITY_REVIEW_BLOCKED")) || !bytes.Contains(body, []byte("Remove the blocked Skill")) || bytes.Contains(body, []byte("private-secret")) {
		t.Fatalf("unsafe or unactionable review refusal: %d %s", status, body)
	}
}
