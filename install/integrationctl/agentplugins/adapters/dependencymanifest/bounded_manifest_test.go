package dependencymanifest

import (
	"errors"
	"strings"
	"testing"
)

func TestDependencyManifestBoundedDecode(t *testing.T) {
	const limit = 4 << 20
	prefix := "schema_version = 1\n#"
	atLimit := []byte(prefix + strings.Repeat("x", limit-len(prefix)))
	document, err := Decode(atLimit, "captured/uap.toml")
	if err != nil {
		t.Fatalf("valid document at the byte ceiling: %v", err)
	}
	if _, err := document.Root(); err != nil {
		t.Fatalf("root view at the byte ceiling: %v", err)
	}
	if _, err := document.Package(); err != nil {
		t.Fatalf("package view at the byte ceiling: %v", err)
	}
	for _, body := range [][]byte{append(atLimit, 'x'), []byte(strings.Repeat("[", limit+1))} {
		_, err := Decode(body, "captured/uap.toml")
		var diagnostic *Error
		if !errors.As(err, &diagnostic) || diagnostic.Source != "captured/uap.toml" || diagnostic.Line != 1 || !strings.Contains(err.Error(), "exceeds 4194304 bytes") {
			t.Fatalf("oversized input must fail at byte admission before TOML parse: %v", err)
		}
	}
}
