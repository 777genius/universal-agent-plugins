package conformance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSharedSkillAdmission(t *testing.T) {
	for _, tc := range []struct{ name, extra, code string }{
		{"absent", "", ""},
		{"empty-map", "metadata: {}\n", ""},
		{"strings", "metadata: {empty: '', unicode: 'навык', number: '5'}\n", ""},
		{"number", "metadata: {count: 5}\n", "skill_metadata_type"},
		{"boolean", "metadata: {flag: true}\n", "skill_metadata_type"},
		{"null", "metadata: {value: null}\n", "skill_metadata_type"},
		{"array", "metadata: {value: [x]}\n", "skill_metadata_type"},
		{"nested", "metadata: {value: {nested: x}}\n", "skill_metadata_type"},
		{"numeric-key", "metadata: {5: x}\n", "skill_metadata_type"},
		{"boolean-key", "metadata: {true: x}\n", "skill_metadata_type"},
		{"empty-compatibility", "compatibility: ''\n", "skill_compatibility_length"},
		{"one-codepoint", "compatibility: '界'\n", ""},
		{"whitespace", "compatibility: ' '\n", ""},
		{"max-codepoints", "compatibility: '" + strings.Repeat("界", 500) + "'\n", ""},
		{"over-codepoints", "compatibility: '" + strings.Repeat("界", 501) + "'\n", "skill_compatibility_length"},
		{"null-compatibility", "compatibility: null\n", "skill_optional_type"},
		{"number-compatibility", "compatibility: 5\n", "skill_optional_type"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := skillInput("good", tc.extra)
			body := []byte("---\nname: good\ndescription: Works offline\n" + tc.extra + "---\n")
			skill, err := ParseInstallerSkill("good", body)
			if tc.code != "" {
				var failure *skillFailure
				if !errors.As(err, &failure) || failure.Code != tc.code {
					t.Fatalf("installer error = %v, want %s", err, tc.code)
				}
			} else if err != nil || string(skill.Raw) != string(body) {
				t.Fatalf("installer = %+v, %v", skill, err)
			}
			if tc.code == "" && strings.HasPrefix(tc.extra, "compatibility: ") {
				want := strings.TrimSuffix(strings.TrimPrefix(tc.extra, "compatibility: '"), "'\n")
				if skill.Compatibility != want {
					t.Fatalf("compatibility normalized: %q, want %q", skill.Compatibility, want)
				}
			}
			facts, err := (Decoder{}).DecodeSkill(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if tc.code != "" {
				if facts.Coverage.Skills != Fail || !hasFinding(facts, tc.code, Normative, domain.BoundarySkill) {
					t.Fatalf("author = %+v", facts)
				}
			} else if facts.Coverage.Skills != Pass || string(facts.Package.Skills["good"].Raw) != string(body) {
				t.Fatalf("author = %+v", facts)
			}
		})
	}
}

// Prevent reintroducing the old narrower installer dialect. The UTF-8 limit is
// characters, extension fields stay opaque, and original bytes stay untouched.
func TestInstallerAndAuthorShareNormativeUnicodeAndOpaqueGrammar(t *testing.T) {
	for _, name := range []string{"навык", "école", "a" + strings.Repeat("é", 63)} {
		body := append([]byte{0xef, 0xbb, 0xbf}, []byte("---\r\nname: "+name+"\r\ndescription: Works\r\nfuture: {opaque: [1, true]}\r\n---\r\nbody\r\n")...)
		skill, err := ParseInstallerSkill(name, body)
		if err != nil || string(skill.Raw) != string(body) {
			t.Fatal(name, err)
		}
		facts, err := (Decoder{}).DecodeSkill(context.Background(), SkillInput{name, NewDocument("skills/"+name+"/SKILL.md", Present, body)})
		if err != nil || facts.Coverage.Skills != Pass {
			t.Fatal(name, facts, err)
		}
	}
	_, err := ParseInstallerSkill("good", []byte("---\nname: good\nname: other\ndescription: Works\n---\n"))
	code, unavailable := SkillErrorCode(err)
	if !unavailable || code != "yaml_duplicate_key" {
		t.Fatalf("duplicate YAML evidence = %s %v %v", code, unavailable, err)
	}
}
