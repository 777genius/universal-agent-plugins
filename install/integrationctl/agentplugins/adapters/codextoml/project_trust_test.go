package codextoml

import (
	"bytes"
	"strings"
	"testing"
)

func TestTrustProjectPreservesSettingsAcrossNativeTOMLForms(t *testing.T) {
	for _, body := range []string{
		"", "# retained\nmodel = 'test'\n[projects.\"/other\"]\ntrust_level = 'untrusted'\n",
		"[projects.\"/selected\"]\n# keep comment\nother = true\n",
		"projects = { '/selected' = {other = true}, '/other' = {trust_level = 'untrusted'} }\n",
		"projects = { '/other' = {trust_level = 'untrusted'} }\n",
		"projects.'/selected'.other = true\n",
		"[projects]\n'/other' = {trust_level = 'untrusted'}\n",
	} {
		next, err := TrustProject([]byte(body), "/selected")
		if err != nil {
			t.Fatalf("body=%s err=%v", body, err)
		}
		if strings.Contains(body, "# retained") && !bytes.Contains(next, []byte("# retained")) {
			t.Fatal("comment removed")
		}
		again, err := TrustProject(next, "/selected")
		if err != nil || !bytes.Equal(next, again) {
			t.Fatalf("not idempotent: %v", err)
		}
	}
	for _, body := range []string{
		"projects = false", "[projects]\n'/selected' = false", "[projects.'/selected']\ntrust_level = 'untrusted'", "[projects.'/selected']\ntrust_level = true", "[projects.'/selected']\ntrust_level = 'trusted'\ntrust_level = 'untrusted'", "{}",
	} {
		if _, err := TrustProject([]byte(body), "/selected"); err == nil {
			t.Fatalf("explicit or malformed policy accepted: %s", body)
		}
	}
	// Key escaping is delegated to the TOML AST, never string interpolation.
	if _, err := TrustProject(nil, "/selected/with\"quote\\slash"); err != nil {
		t.Fatal(err)
	}
}
