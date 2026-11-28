package enterprisepolicy

import (
	"errors"
	"strings"
	"testing"
)

func TestDecodePolicySchemaCompatibilityAndNativeTypes(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"empty": "", "unsupported": "schema_version: 3\n",
		"string-version": "schema_version: '2'\n", "missing-version": "sources: {}\n",
		"unknown":            "schema_version: 2\nsource: {}\n",
		"unknown-nested":     "schema_version: 2\nsources:\n  allow_host: [github.com]\n",
		"duplicate":          "schema_version: 2\nschema_version: 2\n",
		"duplicate-nested":   "schema_version: 2\nsources:\n  deny_hosts: []\n  deny_hosts: [bad.test]\n",
		"null-section":       "schema_version: 2\nsources: null\n",
		"null-list":          "schema_version: 2\nsources:\n  allow_hosts: null\n",
		"numeric-host":       "schema_version: 2\nsources:\n  allow_hosts: [123]\n",
		"string-bool":        "schema_version: 2\nintegrity:\n  require_pinned_git: 'true'\n",
		"tagged-yes":         "schema_version: 2\nintegrity:\n  require_pinned_git: !!bool yes\n",
		"alias":              "schema_version: 2\nsources: &a {allow_hosts: [github.com]}\nmcp: *a\n",
		"merge":              "schema_version: 2\nsources:\n  <<: {allow_hosts: [github.com]}\n",
		"multiple":           "schema_version: 2\n---\nschema_version: 2\n",
		"tagged-map":         "!custom\nschema_version: 2\n",
		"bad-host":           "schema_version: 2\nsources:\n  allow_hosts: [GitHub.com]\n",
		"bad-target":         "schema_version: 2\ninstall:\n  allowed_targets: [not-an-agent]\n",
		"bad-kind":           "schema_version: 2\nsources:\n  allowed_kinds: [registry]\n",
		"bad-transport":      "schema_version: 2\nmcp:\n  allowed_transports: [tcp]\n",
		"hostless-repo":      "schema_version: 2\nsources:\n  allow_repositories: [acme/repo]\n",
		"middle-wildcard":    "schema_version: 2\nsources:\n  allow_repositories: [github.com/a*/repo]\n",
		"empty-repo":         "schema_version: 2\nsources:\n  allow_repositories: [github.com/acme/.git]\n",
		"legacy-new-field":   "schema_version: 1\nsources:\n  allowed_kinds: [local]\n",
		"legacy-new-section": "schema_version: 1\ninstall: {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Decode([]byte(body))
			var config *ConfigError
			if !errors.As(err, &config) {
				t.Fatalf("expected typed configuration refusal: %v", err)
			}
		})
	}
	legacy, err := Decode([]byte("schema_version: 1\nsources:\n  allow_hosts: [github.com]\nintegrity:\n  require_pinned_git: true\nmcp:\n  allowed_transports: [stdio, streamable-http]\n"))
	if err != nil || !legacy.Integrity.RequirePinnedGit || !legacy.Sources.AllowHosts.Present {
		t.Fatalf("legacy policy rules changed: %+v %v", legacy, err)
	}
	neutral, err := Decode([]byte("schema_version: 2\n"))
	if err != nil || neutral.RestrictionCount() != 0 {
		t.Fatalf("schema 2 must be neutral by default: %+v %v", neutral, err)
	}
	empty, err := Decode([]byte("schema_version: 2\nsources:\n  allow_hosts: []\n"))
	if err != nil || !empty.Sources.AllowHosts.Present || empty.RestrictionCount() != 1 {
		t.Fatalf("explicit empty allow-list lost: %+v %v", empty, err)
	}
}

func TestDecodeBoundAndDiagnosticsDoNotEchoPolicyValues(t *testing.T) {
	t.Parallel()
	secret := "sensitive-policy-value-123"
	_, err := Decode([]byte("schema_version: 2\nsources:\n  allow_hosts: ['https://" + secret + "@host.test']\n"))
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("unsafe parser diagnostic: %v", err)
	}
	_, err = Decode([]byte(strings.Repeat(" ", int(MaxBytes)+1)))
	if err == nil {
		t.Fatal("oversized policy accepted")
	}
}
