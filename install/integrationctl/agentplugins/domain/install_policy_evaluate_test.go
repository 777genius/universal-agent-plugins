package domain

import (
	"encoding/json"
	"testing"
)

func TestInstallPolicyExtensionsUseJSONPresence(t *testing.T) {
	t.Parallel()
	policy := InstallPolicy{SchemaVersion: 2, Components: InstallPolicyComponents{Denied: []string{"extensions"}}}
	for _, tc := range []struct {
		name    string
		raw     string
		present bool
	}{
		{name: "absent"},
		{name: "blank", raw: " \t\n\r "},
		{name: "unicode blank", raw: "\u00a0"},
		{name: "null", raw: " \n null\t "},
		{name: "empty object", raw: "{}"},
		{name: "spaced empty object", raw: "{ \n\t }"},
		{name: "empty array", raw: "[]"},
		{name: "spaced empty array", raw: "[ \r\n\t ]"},
		{name: "empty nested object", raw: `{"vendor": { }}`, present: true},
		{name: "array with null", raw: "[ null ]", present: true},
		{name: "object spelling in string", raw: `"{}"`, present: true},
		{name: "empty string", raw: `""`, present: true},
		{name: "false", raw: "false", present: true},
		{name: "zero", raw: "0", present: true},
		{name: "malformed object", raw: "{", present: true},
		{name: "malformed array", raw: "[ ] trailing", present: true},
		{name: "multiple values", raw: "{} null", present: true},
		{name: "comment", raw: "{} /* empty */", present: true},
		{name: "non JSON whitespace", raw: "\v{}\v", present: true},
		{name: "non JSON unicode padding", raw: "\u00a0{}\u00a0", present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := InstallPolicyRequest{Envelope: &PackageEnvelope{Manifest: PluginManifest{RawExtensions: json.RawMessage(tc.raw)}}}
			violations := EvaluateInstallPolicy(policy, req, InstallPolicySource{})
			if (len(violations) > 0) != tc.present {
				t.Fatalf("raw extensions %q: violations = %+v, want present %v", tc.raw, violations, tc.present)
			}
			if tc.present && (len(violations) != 1 || violations[0].Code != "components.denied" || violations[0].Subject != "extensions") {
				t.Fatalf("unexpected extension refusal: %+v", violations)
			}
		})
	}
}
