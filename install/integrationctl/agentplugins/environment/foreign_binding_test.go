package environment

import (
	"errors"
	"strings"
	"testing"
)

// The error fires exactly where it did, but now names every client that holds
// the source outside the environment so hosts can say how to remove each one.
func TestForeignBindingErrorNamesEveryUnmanagedClient(t *testing.T) {
	pkg := LockedPackage{Alias: "demo", SourceKey: "git:demo"}
	observed := []BindingObservation{
		{InstallationID: "inst-b", ClientID: "cursor", SourceKey: "git:demo"},
		{InstallationID: "inst-a", ClientID: "codex", SourceKey: "git:demo"},
		{InstallationID: "inst-a", ClientID: "claude", SourceKey: "git:demo"},
		{InstallationID: "inst-c", ClientID: "gemini", SourceKey: "git:other"},
		{InstallationID: "inst-d", ClientID: "kiro", SourceKey: "git:demo", Managed: true, Alias: "demo"},
	}
	_, err := validateDesiredBinding(pkg, "codex", BindingObservation{}, observed)
	var foreign *ForeignBindingError
	if !errors.As(err, &foreign) {
		t.Fatalf("error = %v", err)
	}
	want := []ForeignBinding{{"inst-a", "claude"}, {"inst-a", "codex"}, {"inst-b", "cursor"}}
	if foreign.Alias != "demo" || len(foreign.Bindings) != len(want) {
		t.Fatalf("bindings = %+v", foreign.Bindings)
	}
	for i := range want {
		if foreign.Bindings[i] != want[i] {
			t.Fatalf("bindings = %+v, want %+v", foreign.Bindings, want)
		}
	}
	if msg := err.Error(); !strings.HasPrefix(msg, "foreign: demo has an unmanaged binding in installations inst-a, inst-b;") || strings.Contains(msg, "inst-c") {
		t.Fatalf("message = %q", msg)
	}
	// Unrelated or owned bindings never raise it.
	if _, err := validateDesiredBinding(pkg, "codex", BindingObservation{}, observed[3:4]); err != nil {
		t.Fatalf("other source blocked: %v", err)
	}
}
