package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// UAP declarations are opaque standard extension data, even when their shape
// cannot describe dependencies. Only the manager assigns dependency semantics.
func TestPluginDependencyExtensionIsOpaque(t *testing.T) {
	for _, test := range []struct{ name, value string }{
		{"absent", ``}, {"empty", `,"dependencies":{}`},
		{"versions", `,"dependencies":{"acme/a":"^1.2","acme/b":"1.2.3"}`},
		{"null", `,"dependencies":null`}, {"array", `,"dependencies":[]`},
		{"number", `,"dependencies":42`}, {"value type", `,"dependencies":{"acme/a":2}`},
		{"empty selector", `,"dependencies":{"acme/a":""}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			extension := `{"other":true` + test.value + `}`
			extensions := `{"` + domain.ExtensionNamespace + `":` + extension + `}`
			body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"demo","version":"legacy","extensions":` + extensions + `}`)
			d := decoder(t)
			manifest, diagnostics, digest, err := (InstallerDecoder{Registry: d.Registry}).Plugin(body)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			if digest != "sha256:"+hex.EncodeToString(sum[:]) || !bytes.Equal(manifest.Raw, body) || string(manifest.Extensions[domain.ExtensionNamespace]) != extension || string(manifest.RawExtensions) != extensions || len(diagnostics) != 0 {
				t.Fatalf("lossless identity changed: %+v %s diagnostics=%v", manifest, digest, diagnostics)
			}
			facts, err := d.DecodePlugin(context.Background(), NewDocument("plugin.json", Present, body))
			if err != nil || facts.Package == nil || facts.Coverage.Plugin != Pass || len(facts.Findings) != 0 {
				t.Fatalf("UAP extension changed standard facts: %+v %v", facts, err)
			}
		})
	}
}
