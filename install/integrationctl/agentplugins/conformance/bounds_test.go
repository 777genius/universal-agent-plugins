package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestBoundedDocumentDecoding(t *testing.T) {
	d := decoder(t)
	i := minimal()
	size := len(i.Plugin.body)
	for _, delta := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("bytes-%d", delta), func(t *testing.T) {
			bounded := d
			bounded.Limits.PluginBytes = size + delta
			f, e := bounded.Decode(context.Background(), i)
			if e != nil || (f.Conformance == Pass) != (delta >= 0) {
				t.Fatal(f, e)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		limits Limits
		body   string
		ok     bool
	}{
		{"tokens-exact", Limits{Tokens: 4}, `{"a":1}`, true}, {"tokens-over", Limits{Tokens: 3}, `{"a":1}`, false},
		{"members-exact", Limits{Members: 2}, `{"a":1,"b":2}`, true}, {"members-over", Limits{Members: 1}, `{"a":1,"b":2}`, false},
		{"depth-exact", Limits{Depth: 2}, `{"a":{"b":1}}`, true}, {"depth-over", Limits{Depth: 1}, `{"a":{"b":1}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, e := scanJSON(context.Background(), []byte(tc.body), tc.limits.bounded(), duplicateRecord)
			if (e == nil) != tc.ok {
				t.Fatal(e)
			}
		})
	}
	for _, where := range []string{"name", "opaque"} {
		raw := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","` + where + `":"`)
		raw = append(raw, 0xff)
		raw = append(raw, []byte(`"}`)...)
		i := minimal()
		i.Plugin = NewDocument("plugin.json", Present, raw)
		f, e := d.Decode(context.Background(), i)
		if e != nil || f.Conformance != NotEvaluated || !hasFinding(f, "document_utf8_invalid", HostSafety, domain.BoundaryPlugin) {
			t.Fatal(f, e)
		}
	}
	bounded := d
	bounded.Limits.AggregateBytes = size - 1
	f, e := bounded.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated || f.Coverage.Complete {
		t.Fatal(f, e)
	}
	bounded.Limits.AggregateBytes = size
	f, e = bounded.Decode(context.Background(), i)
	if e != nil || f.Conformance != Pass {
		t.Fatal(f, e)
	}
	bounded = d
	bounded.Limits.Members = 2
	i.Skills = []SkillInput{skillInput("a", ""), skillInput("b", ""), skillInput("c", "")}
	f, e = bounded.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated {
		t.Fatal(f, e)
	}
}
func TestBoundedYAMLAndExpansion(t *testing.T) {
	d := decoder(t)
	i := skillInput("good", "")
	data, e := frontmatterBytes(i.Document.body)
	if e != nil {
		t.Fatal(e)
	}
	for _, delta := range []int{-1, 0, 1} {
		bounded := d
		bounded.Limits.FrontmatterBytes = len(data) + delta
		f, e := bounded.DecodeSkill(context.Background(), i)
		if e != nil || (f.Coverage.Skills == Pass) != (delta >= 0) {
			t.Fatal(f, e)
		}
	}
	for _, tc := range []struct {
		extra  string
		limits Limits
		code   string
	}{
		{"future: {a: {b: {c: 1}}}\n", Limits{Depth: 2}, "yaml_depth_limit"},
		{"future: [a,b,c,d,e]\n", Limits{Tokens: 8}, "yaml_node_limit"},
		{"future: {a: 1,b: 2,c: 3}\n", Limits{Members: 4}, "yaml_member_limit"},
		{"future: &a [1,2,3,4,5]\nfuture2: [*a,*a,*a,*a,*a]\n", Limits{Tokens: 25}, "yaml_node_limit"},
	} {
		bounded := d
		bounded.Limits = tc.limits
		f, e := bounded.DecodeSkill(context.Background(), skillInput("good", tc.extra))
		if e != nil || f.Coverage.Skills != NotEvaluated || !hasFinding(f, tc.code, HostSafety, domain.BoundarySkill) {
			t.Fatalf("%s %+v %v", tc.code, f, e)
		}
	}
}
func TestDiagnosticCapIsSafeAndDeterministic(t *testing.T) {
	d := decoder(t)
	d.Limits.Diagnostics = 3
	fields := `"$schema":"` + domain.PluginSchemaV1 + `","name":"good"`
	for n := 0; n < 20; n++ {
		fields += fmt.Sprintf(",%q:true", fmt.Sprintf("secret-%03d", n))
	}
	i := minimal()
	i.Plugin = NewDocument("plugin.json", Present, []byte("{"+fields+"}"))
	var expected string
	for n := 0; n < 20; n++ {
		f, e := d.Decode(context.Background(), i)
		if e != nil || len(f.Findings) != 3 || f.Conformance != Fail || f.Coverage.Complete {
			t.Fatalf("%+v %v", f, e)
		}
		if !hasFinding(f, "diagnostics_truncated", HostSafety, domain.BoundaryPlugin) {
			t.Fatal(f)
		}
		b, _ := json.Marshal(f)
		if strings.Contains(string(b), "secret-") {
			t.Fatal(string(b))
		}
		if n > 0 && expected != string(b) {
			t.Fatal("unstable capped findings")
		}
		expected = string(b)
	}
}

// A deterministic context cancels during the token walk, not only before entry.
type countingContext struct {
	calls int
	after int
}

func (c *countingContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *countingContext) Done() <-chan struct{}       { return nil }
func (c *countingContext) Value(any) any               { return nil }
func (c *countingContext) Err() error {
	c.calls++
	if c.calls > c.after {
		return context.Canceled
	}
	return nil
}
func TestCancellationMidDocument(t *testing.T) {
	d := decoder(t)
	ctx := &countingContext{after: 10}
	i := minimal()
	i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"good","unknown":[`+strings.Repeat("1,", 1000)+`2]}`))
	_, e := d.Decode(ctx, i)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}

type failedRegistry struct {
	SchemaRegistry
	digest string
}

func (r failedRegistry) Digest(string) (string, bool) { return r.digest, true }
func (r failedRegistry) Validate(string, any) error   { return errors.New("private-engine-secret") }
func TestRegistryIntegrityAndErrorClassification(t *testing.T) {
	d := decoder(t)
	for _, r := range []SchemaRegistry{nil, failedRegistry{d.Registry, "bad"}, failedRegistry{d.Registry, ProfileIdentities()[2].Digest}} {
		broken := Decoder{Registry: r}
		f, e := broken.Decode(context.Background(), minimal())
		if e != nil || f.Conformance != NotEvaluated || !hasFinding(f, "schema_registry_unavailable", InstallerPolicy, domain.BoundaryPlugin) {
			t.Fatal(f, e)
		}
		b, _ := json.Marshal(f)
		if strings.Contains(string(b), "private-engine-secret") {
			t.Fatal(string(b))
		}
	}
	for _, p := range ProfileIdentities() {
		if p.ID == "" || p.Revision == "" || len(p.Digest) != 71 {
			t.Fatalf("unpinned profile %+v", p)
		}
	}
}

// These injected failures are engine consistency failures, never author violations.
type inconsistentRegistry struct {
	SchemaRegistry
	calls int
	typed bool
}

func (r *inconsistentRegistry) Digest(uri string) (string, bool) {
	return r.SchemaRegistry.(interface{ Digest(string) (string, bool) }).Digest(uri)
}
func (r *inconsistentRegistry) Validate(uri string, value any) error {
	r.calls++
	if r.typed {
		return nil
	}
	if r.calls == 1 {
		return r.SchemaRegistry.Validate(uri, value)
	}
	return errors.New("private engine failure")
}
func TestTypedDecodeAndSecondRegistryFailureAreEngineFailures(t *testing.T) {
	d := decoder(t)
	for _, typed := range []bool{false, true} {
		registry := &inconsistentRegistry{SchemaRegistry: d.Registry, typed: typed}
		broken := Decoder{Registry: registry}
		i := minimal()
		if typed {
			i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"good","version":{}}`))
		}
		f, e := broken.Decode(context.Background(), i)
		if e != nil || f.Conformance != NotEvaluated {
			t.Fatalf("typed=%v: %+v %v", typed, f, e)
		}
		code := "plugin_schema_invalid"
		if typed {
			code = "plugin_manifest_decode_failed"
		}
		if !hasFinding(f, code, InstallerPolicy, domain.BoundaryPlugin) {
			t.Fatal(f)
		}
	}
}
func TestDuplicateFindingSpanIdentity(t *testing.T) {
	d := decoder(t)
	i := minimal()
	i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"good","opaque":{"a":1,"a":2,"b":1,"b":2}}`))
	f, e := d.Decode(context.Background(), i)
	if e != nil {
		t.Fatal(e)
	}
	ids := map[string]bool{}
	for _, v := range f.Findings {
		if v.Code == "json_duplicate_key" {
			if v.Ordinal == 0 || ids[v.ID] {
				t.Fatal("duplicate spans collapsed")
			}
			ids[v.ID] = true
		}
	}
	if len(ids) != 2 {
		t.Fatal(f.Findings)
	}
}

func TestYAMLDepthBeforeNodeAllocation(t *testing.T) {
	d := decoder(t)
	for _, extra := range []string{"future: " + strings.Repeat("[", 11000) + "x" + strings.Repeat("]", 11000) + "\n", strings.Repeat("  ", 100) + "future: x\n"} {
		f, e := d.DecodeSkill(context.Background(), skillInput("good", extra))
		if e != nil {
			t.Fatal(e)
		}
		if strings.Contains(extra, "[") && (!hasFinding(f, "yaml_depth_limit", HostSafety, domain.BoundarySkill) || f.Coverage.Skills != NotEvaluated) {
			t.Fatal(f)
		}
	}
	// Opaque quoted and block-scalar delimiters consume bytes, not YAML nesting.
	for _, extra := range []string{"license: '" + strings.Repeat("[", 1000) + "'\n", "license: |\n  " + strings.Repeat("[", 1000) + "\n"} {
		f, e := d.DecodeSkill(context.Background(), skillInput("good", extra))
		if e != nil || f.Coverage.Skills != Pass {
			t.Fatal(f, e)
		}
	}
}
