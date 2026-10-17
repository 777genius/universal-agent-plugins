package providers

import (
	"testing"
)

func TestGrokPluginStatusAcceptsLiveListContract(t *testing.T) {
	t.Parallel()
	live := []byte(`[
  {
    "status": "installed",
    "name": "grok-basic",
    "repo_key": "grok-basic-eb3232f5",
    "version": "0.1.0",
    "path": "/tmp/grok/installed-plugins/grok-basic-eb3232f5",
    "source": "/tmp/src/grok-basic",
    "marketplace": null
  }
]`)
	if got := grokPluginStatus(live, "grok-basic"); got != grokStatusInstalled {
		t.Fatalf("live listing = %d", got)
	}
	if got := grokPluginStatus([]byte("[]"), "grok-basic"); got != grokStatusAbsent {
		t.Fatalf("empty listing = %d", got)
	}
	if got := grokPluginStatus(live, "other"); got != grokStatusAbsent {
		t.Fatalf("missing name = %d", got)
	}
}

func TestGrokPluginStatusRejectsUnknownContracts(t *testing.T) {
	t.Parallel()
	tests := []string{
		"",
		"No plugins installed.",
		`{"plugins":[]}`,
		`[{"name":"grok-basic"}]`,
		`[{"status":"installed"}]`,
		`[{"name":"grok-basic","status":"unknown"}]`,
		`[{"name":"demo","status":"installed"},{"name":"demo","status":"installed"}]`,
		`[] extra`,
	}
	for _, body := range tests {
		if got := grokPluginStatus([]byte(body), "demo"); got != grokStatusUnknown {
			t.Fatalf("body %q = %d", body, got)
		}
	}
}

func TestGrokRegistryFindingMapsOwnership(t *testing.T) {
	t.Parallel()
	listed := []byte(`[{"name":"demo","status":"installed"}]`)
	if got := grokRegistryFinding(listed, "demo", true); got != registryExpected {
		t.Fatalf("owned = %d", got)
	}
	if got := grokRegistryFinding(listed, "demo", false); got != registryCollision {
		t.Fatalf("unowned = %d", got)
	}
	if got := grokRegistryFinding([]byte("[]"), "demo", false); got != registryClear {
		t.Fatalf("empty = %d", got)
	}
	if got := grokRegistryFinding([]byte(`{"plugins":[]}`), "demo", false); got != registryIndeterminate {
		t.Fatalf("unknown = %d", got)
	}
}
