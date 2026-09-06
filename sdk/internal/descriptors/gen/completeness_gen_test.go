package gen

import (
	"github.com/777genius/plugin-kit-ai/sdk/internal/descriptors/defs"
	"testing"
)

func TestGeneratedRegistryCompleteness(t *testing.T) {
	profiles := defs.Profiles()
	events := defs.Events()
	if len(profiles) != 6 {
		t.Fatalf("profiles count = %d", len(profiles))
	}
	if len(events) != 31 {
		t.Fatalf("events count = %d", len(events))
	}
	entries := AllSupportEntries()
	if len(entries) != len(events) {
		t.Fatalf("support entries = %d want %d", len(entries), len(events))
	}
	for _, event := range events {
		if _, ok := Lookup(event.Platform, event.Event); !ok {
			t.Fatalf("missing descriptor %s/%s", event.Platform, event.Event)
		}
		if event.Invocation.Kind == "" {
			t.Fatalf("missing invocation kind for %s/%s", event.Platform, event.Event)
		}
		if event.Contract.Maturity == "" {
			t.Fatalf("missing contract maturity for %s/%s", event.Platform, event.Event)
		}
		if event.DecodeFunc == "" || event.EncodeFunc == "" {
			t.Fatalf("missing codec refs for %s/%s", event.Platform, event.Event)
		}
		if event.Registrar.MethodName == "" || event.Registrar.WrapFunc == "" {
			t.Fatalf("missing registrar metadata for %s/%s", event.Platform, event.Event)
		}
	}
	for _, profile := range profiles {
		if profile.Status == "" {
			t.Fatalf("missing status for %s", profile.Platform)
		}
		if len(profile.TransportModes) == 0 {
			t.Fatalf("missing transport modes for %s", profile.Platform)
		}
		if profile.Status != "deferred" {
			if len(profile.Scaffold.RequiredFiles) == 0 || len(profile.Scaffold.TemplateFiles) == 0 {
				t.Fatalf("missing scaffold metadata for %s", profile.Platform)
			}
			if len(profile.Validate.RequiredFiles) == 0 {
				t.Fatalf("missing validate metadata for %s", profile.Platform)
			}
		}
	}
}

func TestSupportEntriesPreserveEventOrder(t *testing.T) {
	t.Parallel()

	entries := AllSupportEntries()
	events := defs.Events()
	if len(entries) != len(events) {
		t.Fatalf("support entries = %d want %d", len(entries), len(events))
	}
	for i, event := range events {
		entry := entries[i]
		if entry.Platform != event.Platform || entry.Event != event.Event {
			t.Fatalf("entry[%d] = %s/%s want %s/%s", i, entry.Platform, entry.Event, event.Platform, event.Event)
		}
	}
}
