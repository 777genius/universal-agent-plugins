package skillspicker

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHandoffClosesUnacceptedReviewAndRefusesLateAdmission(t *testing.T) {
	var closed, effects atomic.Int32
	server, err := Start(context.Background(), Options{Callbacks: testCallbacks(func() Prepared {
		return &testPrepared{close: func() { closed.Add(1) }, apply: func(context.Context) (Outcome, error) {
			effects.Add(1)
			return Outcome{State: "succeeded"}, nil
		}}
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	plan := client.plan()
	if !server.CloseUnused() {
		t.Fatal("unaccepted review prevented terminal handoff")
	}
	if closed.Load() != 1 || effects.Load() != 0 {
		t.Fatalf("closed=%d effects=%d", closed.Load(), effects.Load())
	}
	request := httptest.NewRequest("POST", server.origin+"/api/v1/plans/"+plan.PlanId+"/apply", nil)
	request.Header.Set("Authorization", "Bearer "+server.token)
	request.Header.Set("Origin", server.origin)
	response := httptest.NewRecorder()
	server.guard(http.HandlerFunc(server.apply)).ServeHTTP(response, request)
	if response.Code != 410 || effects.Load() != 0 {
		t.Fatalf("late admission=%d body=%s effects=%d", response.Code, response.Body.String(), effects.Load())
	}
}

func TestHandoffClosesInFlightScanWithoutDeadlock(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	callbacks := testCallbacks(func() Prepared { return &testPrepared{} })
	callbacks.Scan = func(ctx context.Context, _ string) (Catalog, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return Catalog{}, ctx.Err()
	}
	server, err := Start(context.Background(), Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	<-started
	finished := make(chan bool, 1)
	go func() { finished <- server.CloseUnused() }()
	select {
	case ok := <-finished:
		if !ok {
			t.Fatal("scan blocked unused handoff")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("handoff waited for scan under mutex")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("scan cleanup not joined")
	}
}

func TestHandoffRefusesCurrentPreparationThenAllowsFailedReview(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	callbacks := testCallbacks(func() Prepared { return &testPrepared{} })
	callbacks.Prepare = func(context.Context, PlanRequest) (Prepared, error) {
		close(started)
		<-release
		return nil, errors.New("fixture preparation refusal")
	}
	server, err := Start(context.Background(), Options{Callbacks: callbacks})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	response := make(chan int, 1)
	go func() {
		status, _ := client.request(context.Background(), "POST", "/api/v1/plans", `{"catalogId":"catalog","catalogDigest":"digest","selectedIds":["candidate"],"targetIds":["codex"]}`, nil)
		response <- status
	}()
	<-started
	if server.CloseUnused() {
		t.Fatal("handoff cancelled in-flight preparation")
	}
	if server.ctx.Err() != nil {
		t.Fatal("refused handoff cancelled server")
	}
	close(release)
	if status := <-response; status != 422 {
		t.Fatalf("failed preparation=%d", status)
	}
	if !server.CloseUnused() {
		t.Fatal("failed preparation permanently blocked unused handoff")
	}
}

func TestHandoffPreservesAcceptedAndCompletedOperation(t *testing.T) {
	started := make(chan context.Context, 1)
	release := make(chan struct{})
	var effects atomic.Int32
	server, err := Start(context.Background(), Options{Callbacks: testCallbacks(func() Prepared {
		return &testPrepared{apply: func(ctx context.Context) (Outcome, error) {
			effects.Add(1)
			started <- ctx
			select {
			case <-release:
			case <-ctx.Done():
				return Outcome{}, ctx.Err()
			}
			return Outcome{State: "succeeded"}, nil
		}}
	})})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	client := newPickerClient(t, server)
	client.waitReady()
	plan := client.plan()
	status, body := client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
	if status != 202 {
		t.Fatalf("apply=%d %s", status, body)
	}
	var accepted ApplyResponse
	mustDecode(t, body, &accepted)
	worker := <-started
	if server.CloseUnused() || worker.Err() != nil {
		t.Fatal("handoff cancelled accepted operation")
	}
	close(release)
	eventually(t, func() bool {
		server.mu.Lock()
		defer server.mu.Unlock()
		return !server.busy
	})
	if server.CloseUnused() || worker.Err() != nil || effects.Load() != 1 {
		t.Fatalf("completed operation lost: err=%v effects=%d", worker.Err(), effects.Load())
	}
	status, body = client.request(context.Background(), "POST", "/api/v1/plans/"+plan.PlanId+"/apply", "", nil)
	var repeated ApplyResponse
	mustDecode(t, body, &repeated)
	if status != 202 || repeated.OperationId != accepted.OperationId || effects.Load() != 1 {
		t.Fatalf("identity replay=%d %s effects=%d", status, body, effects.Load())
	}
}

func TestHandoffAndApplyRaceAdmitOnlyOneOwner(t *testing.T) {
	for range 30 {
		var effects atomic.Int32
		server, err := Start(context.Background(), Options{Callbacks: testCallbacks(func() Prepared {
			return &testPrepared{apply: func(context.Context) (Outcome, error) {
				effects.Add(1)
				return Outcome{State: "succeeded"}, nil
			}}
		})})
		if err != nil {
			t.Fatal(err)
		}
		client := newPickerClient(t, server)
		client.waitReady()
		plan := client.plan()
		gate := make(chan struct{})
		applied := make(chan int, 1)
		handoff := make(chan bool, 1)
		go func() {
			<-gate
			request := httptest.NewRequest("POST", "/api/v1/plans/"+plan.PlanId+"/apply", nil)
			request.SetPathValue("id", plan.PlanId)
			response := httptest.NewRecorder()
			server.apply(response, request)
			applied <- response.Code
		}()
		go func() {
			<-gate
			handoff <- server.CloseUnused()
		}()
		close(gate)
		status, taken := <-applied, <-handoff
		if taken {
			if status != 410 || effects.Load() != 0 {
				t.Fatalf("terminal takeover raced effects: status=%d effects=%d", status, effects.Load())
			}
		} else {
			if status != 202 {
				t.Fatalf("browser owner status=%d", status)
			}
			eventually(t, func() bool { return effects.Load() == 1 })
			if server.CloseUnused() {
				t.Fatal("accepted browser operation allowed later takeover")
			}
		}
		server.Close()
	}
}
