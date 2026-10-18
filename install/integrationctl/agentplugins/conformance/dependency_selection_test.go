package conformance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A declared dependency of the wrong shape must never disappear into an
// ignored extension; exact source bytes and the existing digest stay intact.
func TestVersionSelectionPluginDeclarations(t *testing.T) {
	for _, test := range []struct {
		name, value string
		valid       bool
		count       int
	}{
		{"absent", ``, true, 0}, {"empty", `,"dependencies":{}`, true, 0},
		{"versions", `,"dependencies":{"acme/a":"^1.2","acme/b":"1.2.3"}`, true, 2},
		{"null", `,"dependencies":null`, false, 0}, {"array", `,"dependencies":[]`, false, 0},
		{"number", `,"dependencies":42`, false, 0}, {"value type", `,"dependencies":{"acme/a":2}`, false, 0},
		{"empty selector", `,"dependencies":{"acme/a":""}`, false, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"demo","version":"legacy","extensions":{"` + domain.ExtensionNamespace + `":{"other":true` + test.value + `}}}`)
			d := decoder(t)
			manifest, _, digest, err := (InstallerDecoder{Registry: d.Registry}).Plugin(body)
			if !test.valid {
				if err == nil {
					t.Fatal("invalid declaration accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Dependencies) != test.count {
				t.Fatalf("dependencies=%v", manifest.Dependencies)
			}
			if test.count > 0 && (manifest.Dependencies["acme/a"] != "^1.2" || manifest.Dependencies["acme/b"] != "1.2.3") {
				t.Fatalf("selectors changed: %v", manifest.Dependencies)
			}
			sum := sha256.Sum256(body)
			if digest != "sha256:"+hex.EncodeToString(sum[:]) || !bytes.Equal(manifest.Raw, body) || !bytes.Contains(manifest.Extensions[domain.ExtensionNamespace], []byte(`"other":true`)) || len(manifest.RawExtensions) == 0 {
				t.Fatalf("lossless identity changed: %+v %s", manifest, digest)
			}
		})
	}
}
