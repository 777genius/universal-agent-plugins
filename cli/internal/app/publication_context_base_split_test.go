package app

import "testing"

func TestNormalizePublicationRootInputDefaultsDot(t *testing.T) {
	t.Parallel()
	if got := normalizePublicationRootInput(" \t "); got != "." {
		t.Fatalf("root = %q, want %q", got, ".")
	}
}

func TestValidatePublicationTargetInputRejectsUnsupported(t *testing.T) {
	t.Parallel()
	if _, err := validatePublicationTargetInput("gemini", "supports only %q or %q"); err == nil {
		t.Fatal("expected unsupported target error")
	}
}

func TestValidatePublicationDestInputRejectsBlank(t *testing.T) {
	t.Parallel()
	if _, err := validatePublicationDestInput("   ", "dest required"); err == nil {
		t.Fatal("expected missing dest error")
	}
}
