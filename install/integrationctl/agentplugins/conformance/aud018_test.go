package conformance

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"strings"
	"testing"
)

func TestExactKeysAndOpaqueRaw(t *testing.T) {
	d := decoder(t)
	for _, order := range []bool{false, true} {
		for _, unknown := range []string{`"Name":"evil"`, `"Name":5`, `"$SCHEMA":"secret"`, `"Version":{"secret":1}`, `"Author":5`, `"Name":{"a":1,"a":2}`} {
			fields := `"$schema":"` + domain.PluginSchemaV1 + `","name":"good","version":"plain"`
			if order {
				fields = unknown + "," + fields
			} else {
				fields += "," + unknown
			}
			body := []byte("{" + fields + "}")
			i := minimal()
			i.Plugin = NewDocument("plugin.json", Present, body)
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Package == nil || f.Package.Manifest.Name != "good" || f.Package.Manifest.SchemaURI != domain.PluginSchemaV1 || f.Package.Manifest.Version != "plain" || f.Conformance != Fail {
				t.Fatalf("exact keys: %+v %v", f, e)
			}
			if string(f.Package.Manifest.Raw) != string(body) {
				t.Fatal("raw lost")
			}
			raw, ok := f.Document("plugin.json")
			if !ok || string(raw.Bytes()) != string(body) {
				t.Fatal("private document lost")
			}
		}
	}
	raw := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","extensions":{"x":{"integer":9007199254740993,"exponent":1e9999}}}`)
	i := minimal()
	i.Plugin = NewDocument("plugin.json", Present, raw)
	f, e := d.Decode(context.Background(), i)
	if e != nil || f.Conformance != Pass || !strings.Contains(string(f.Package.Manifest.RawExtensions), "1e9999") {
		t.Fatal("number was coerced")
	}
	raw[0] = '!'
	if f.Package.Manifest.Raw[0] != '{' {
		t.Fatal("input aliases raw storage")
	}
}
