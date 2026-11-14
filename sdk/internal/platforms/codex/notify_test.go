package codex

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/sdk/internal/runtime"
)

func TestDecodeNotify_RejectsOversizedPayload(t *testing.T) {
	raw := `{"client":"` + strings.Repeat("a", runtime.MaxPayloadBytes) + `"}`
	_, _, err := DecodeNotify(runtime.Envelope{
		Args: []string{"plugin-kit-ai", "notify", raw},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds max payload size") {
		t.Fatalf("err = %v", err)
	}
}
