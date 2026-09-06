package hostdetect

import (
	"strings"
	"testing"
)

type fakeEnv map[string]string

func (f fakeEnv) LookupEnv(key string) (string, bool) {
	v, ok := f[key]
	return v, ok
}

const codexStopPayload = `{"session_id":"01a05cd8-b495-7f80-a36b-cc0aa98efc05","turn_id":"01a05cd8-b51b-7343-8b75-b2d4ad9e276e","transcript_path":"/tmp/rollout.jsonl","cwd":"/tmp","hook_event_name":"Stop","model":"gpt-5.6-sol","permission_mode":"bypassPermissions","stop_hook_active":false,"last_assistant_message":"OK"}`

const claudeStopPayload = `{"session_id":"abc","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","hook_event_name":"Stop","stop_hook_active":false}`

func TestDetectOverrideWins(t *testing.T) {
	t.Parallel()

	env := fakeEnv{"SOME_MARKER": "1"}
	got, err := Detect(DefaultRegistry(), "codex", env, []byte(claudeStopPayload))
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformCodex {
		t.Fatalf("Detect() = %q, want %q", got, PlatformCodex)
	}
}

func TestDetectOverrideCaseAndSpace(t *testing.T) {
	t.Parallel()

	got, err := Detect(DefaultRegistry(), "  Claude ", nil, nil)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformClaude {
		t.Fatalf("Detect() = %q, want %q", got, PlatformClaude)
	}
}

func TestDetectUnknownOverrideIsError(t *testing.T) {
	t.Parallel()

	got, err := Detect(DefaultRegistry(), "cursor", nil, []byte(codexStopPayload))
	if err == nil {
		t.Fatal("Detect() expected error for unknown override")
	}
	if got != PlatformUnknown {
		t.Fatalf("Detect() = %q, want PlatformUnknown", got)
	}
}

func TestDetectEnvMarkerMatch(t *testing.T) {
	t.Parallel()

	registry := Registry{
		{Platform: PlatformCodex, EnvMarkers: []string{"FAKE_CODEX_MARKER"}},
		{Platform: PlatformClaude, EnvMarkers: []string{"FAKE_CLAUDE_MARKER"}},
	}
	env := fakeEnv{"FAKE_CODEX_MARKER": ""}
	got, err := Detect(registry, "", env, nil)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformCodex {
		t.Fatalf("Detect() = %q, want %q", got, PlatformCodex)
	}
}

func TestDetectEnvMarkerOrderResolvesAmbiguity(t *testing.T) {
	t.Parallel()

	registry := Registry{
		{Platform: PlatformCodex, EnvMarkers: []string{"SHARED"}},
		{Platform: PlatformClaude, EnvMarkers: []string{"SHARED"}},
	}
	got, err := Detect(registry, "", fakeEnv{"SHARED": "1"}, nil)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformCodex {
		t.Fatalf("Detect() = %q, want first registry entry %q", got, PlatformCodex)
	}
}

func TestDetectCodexPayloadSniff(t *testing.T) {
	t.Parallel()

	got, err := Detect(DefaultRegistry(), "", nil, []byte(codexStopPayload))
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformCodex {
		t.Fatalf("Detect() = %q, want %q", got, PlatformCodex)
	}
}

func TestDetectClaudePayloadSniff(t *testing.T) {
	t.Parallel()

	got, err := Detect(DefaultRegistry(), "", nil, []byte(claudeStopPayload))
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformClaude {
		t.Fatalf("Detect() = %q, want %q", got, PlatformClaude)
	}
}

func TestDetectFailsClosed(t *testing.T) {
	t.Parallel()

	cases := map[string][]byte{
		"empty":       nil,
		"malformed":   []byte("{not json"),
		"non-object":  []byte(`["a"]`),
		"no markers":  []byte(`{"foo":"bar"}`),
		"only values": []byte(`{"session_id":"x"}`),
	}
	for name, payload := range cases {
		got, err := Detect(DefaultRegistry(), "", nil, payload)
		if err != nil {
			t.Fatalf("%s: Detect() error = %v", name, err)
		}
		if got != PlatformUnknown {
			t.Fatalf("%s: Detect() = %q, want PlatformUnknown", name, got)
		}
	}
}

func TestDetectOversizedPayloadSkipsSniff(t *testing.T) {
	t.Parallel()

	filler := strings.Repeat("a", 1<<20)
	oversized := []byte(`{"turn_id":"x","pad":"` + filler + `"}`)
	got, err := Detect(DefaultRegistry(), "", nil, oversized)
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	if got != PlatformUnknown {
		t.Fatalf("Detect() = %q, want PlatformUnknown for oversized payload", got)
	}
}

func TestDefaultRegistryIsIsolated(t *testing.T) {
	t.Parallel()

	first := DefaultRegistry()
	first[0] = Signal{Platform: PlatformClaude}
	second := DefaultRegistry()
	if second[0].Platform != PlatformCodex {
		t.Fatalf("DefaultRegistry()[0] = %q, want %q after mutating a prior copy", second[0].Platform, PlatformCodex)
	}
}
