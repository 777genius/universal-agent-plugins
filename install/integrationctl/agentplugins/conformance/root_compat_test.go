package conformance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func opaqueRootFixture() []byte {
	var b strings.Builder
	b.WriteString(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","future":{`)
	for i := 0; i < 60000; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `"k%05d":0`, i)
	}
	b.WriteString(`}}`)
	return []byte(b.String())
}

func TestRootCompatibilityAllocations(t *testing.T) {
	body := opaqueRootFixture()
	var scanErr error
	allocs := testing.AllocsPerRun(3, func() {
		_, scanErr = scanJSON(context.Background(), body, Limits{}, duplicateRoot)
	})
	if scanErr != nil {
		t.Fatal(scanErr)
	}
	// Allow generous runtime variation, but no allocation per opaque member.
	if allocs > 2000 {
		t.Fatalf("root scan allocated %.0f times for %d bytes; ceiling 2000", allocs, len(body))
	}
	t.Logf("root scan: bytes=%d allocations=%.0f", len(body), allocs)
}

func BenchmarkRootCompatibility(b *testing.B) {
	body := opaqueRootFixture()
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := scanJSON(context.Background(), body, Limits{}, duplicateRoot); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRootCompatibilityStructure(t *testing.T) {
	for _, tc := range []struct{ name, body, diagnostic string }{
		{"nested-object", `{"future":{"x":1,"x":2}}`, ""},
		{"nested-array", `{"future":[{"x":1,"x":2}]}`, ""},
		{"root-escaped-duplicate", `{"x":1,"\u0078":2}`, `JSON object contains duplicate field "x"`},
		{"root-array", `[]`, "document must be a JSON object"},
		{"root-null", `null`, "document must be a JSON object"},
		{"trailing", `{} {}`, "document contains multiple JSON values"},
		{"nested-syntax", `{"future":{"x":}}`, "invalid character"},
		{"unterminated", `{"future":[1`, "unexpected EOF"},
		{"trailing-syntax", `{} !`, "invalid character"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, check := range []func([]byte) error{
				rejectDuplicateTopLevelObjectKeys,
				func(b []byte) error { _, _, e := DecodeJSONObject(b); return e },
				func(b []byte) error { var raw map[string]json.RawMessage; return DecodeRawJSONObject(b, &raw) },
			} {
				err := check([]byte(tc.body))
				if tc.diagnostic == "" {
					if err != nil {
						t.Fatal(err)
					}
				} else if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
					t.Fatalf("got %v, want %s", err, tc.diagnostic)
				}
			}
			if tc.diagnostic == "" && RejectDuplicateJSONKeys([]byte(tc.body)) == nil {
				t.Fatal("all-key checking lost nested duplicate")
			}
		})
	}
	for _, depth := range []int{10000, 10001} {
		body := []byte(`{"future":` + strings.Repeat("[", depth-1) + "0" + strings.Repeat("]", depth-1) + "}")
		err := rejectDuplicateTopLevelObjectKeys(body)
		if (err == nil) != (depth == 10000) {
			t.Fatalf("depth %d: %v", depth, err)
		}
	}
	// Explicit bounds must still traverse descendants, even with root policy.
	for _, limits := range []Limits{{Tokens: 3}, {Members: 1}, {Depth: 1}} {
		if _, err := scanJSON(context.Background(), []byte(`{"future":{"x":1}}`), limits, duplicateRoot); err == nil {
			t.Fatalf("ignored limits: %+v", limits)
		}
	}
}

func TestRootCompatibilityInstallerAndAuthorPolicies(t *testing.T) {
	d := decoder(t)
	installer := InstallerDecoder{Registry: d.Registry}
	for _, field := range []string{"future", "extensions"} {
		body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","` + field + `":{"example.fixture":{"x":1,"x":2}}}`)
		_, diagnostics, _, err := installer.Plugin(body)
		if field == "extensions" {
			if err == nil || !strings.Contains(err.Error(), `duplicate field "x"`) {
				t.Fatalf("known field duplicate: %v", err)
			}
		} else if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "plugin_unknown_field" || diagnostics[0].Item != "future" {
			t.Fatalf("opaque duplicate disposition: %v %+v", err, diagnostics)
		}
	}
	for _, tc := range []struct{ code, value string }{
		{"document_member_limit", func() string {
			var b strings.Builder
			b.WriteByte('{')
			for i := 0; i <= DefaultLimits().Members; i++ {
				if i > 0 {
					b.WriteByte(',')
				}
				fmt.Fprintf(&b, `"k%d":0`, i)
			}
			b.WriteByte('}')
			return b.String()
		}()},
		{"document_token_limit", "[" + strings.Repeat("0,", DefaultLimits().Tokens) + "0]"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","future":` + tc.value + `}`)
			if _, _, err := DecodeJSONObject(body); err != nil {
				t.Fatal(err)
			}
			if _, diagnostics, _, err := installer.Plugin(body); err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "plugin_unknown_field" {
				t.Fatalf("compatibility imposed author bounds: %v %+v", err, diagnostics)
			}
			input := minimal()
			input.Plugin = NewDocument("plugin.json", Present, body)
			f, err := d.Decode(context.Background(), input)
			if err != nil || f.Conformance != NotEvaluated || f.Coverage.Complete || !hasFinding(f, tc.code, HostSafety, domain.BoundaryPlugin) {
				t.Fatalf("author preflight changed: %+v %v", f, err)
			}
		})
	}
}
