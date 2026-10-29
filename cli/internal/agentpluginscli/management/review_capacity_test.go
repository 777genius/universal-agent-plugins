package management

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestReviewCapacityDoesNotPreventApplyingRetainedPlan(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		body       any
	}{
		{name: "dependency", path: "/api/v1/plans", body: ChangeRequest{Action: Sync}},
		{name: "local skill", path: "/api/v1/skills/local/plans", body: map[string]any{"action": "upsert", "rootId": "local", "skillName": "draft", "files": []any{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var prepared atomic.Int32
			server := newTestServer(t, "", "capture", &testBackend{makePlan: func(ChangeRequest, string) Prepared {
				prepared.Add(1)
				return &testPrepared{}
			}})
			var review Plan
			for range 16 {
				requestJSON(t, server, http.MethodPost, tc.path, tc.body, http.StatusOK, &review)
			}
			var refusal APIError
			requestJSON(t, server, http.MethodPost, tc.path, tc.body, http.StatusConflict, &refusal)
			if refusal.Code != "REVIEW_LIMIT" || prepared.Load() != 16 {
				t.Fatalf("review capacity did not refuse new preparation: %+v, calls=%d", refusal, prepared.Load())
			}
			op := applyTest(t, server, review, "capacity-request-123456")
			if got := awaitTerminal(t, server, op.OperationId); got.State != Succeeded {
				t.Fatalf("retained plan could not complete at capacity: %+v", got)
			}
			requestJSON(t, server, http.MethodPost, tc.path, tc.body, http.StatusOK, nil)
			if prepared.Load() != 17 {
				t.Fatalf("applying a retained plan did not release its review slot: %d", prepared.Load())
			}
			if replay := applyTest(t, server, review, "capacity-request-123456"); replay.OperationId != op.OperationId {
				t.Fatal("replay at review capacity did not observe the accepted operation")
			}
		})
	}
}
