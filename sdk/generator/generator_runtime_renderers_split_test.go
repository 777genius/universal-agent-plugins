package generator

import (
	"strings"
	"testing"
)

func TestRenderRegistryIncludesLookup(t *testing.T) {
	t.Parallel()

	m, err := loadModel()
	if err != nil {
		t.Fatal(err)
	}
	body := renderRegistry(m)
	if !strings.Contains(body, "func Lookup(platform runtime.PlatformID, event runtime.EventID)") {
		t.Fatalf("renderRegistry() missing lookup:\n%s", body)
	}
}

func TestRenderResolversIncludesUsageMessage(t *testing.T) {
	t.Parallel()

	m, err := loadModel()
	if err != nil {
		t.Fatal(err)
	}
	body := renderResolvers(m)
	if !strings.Contains(body, `usage: <binary> <hookName>`) {
		t.Fatalf("renderResolvers() missing usage message:\n%s", body)
	}
}
