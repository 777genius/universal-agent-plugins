package conformance

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
)

func FuzzPluginManifest(f *testing.F) {
	registry, err := specregistry.New()
	if err != nil {
		f.Fatal(err)
	}
	decoder := InstallerDecoder{Registry: registry}
	for _, seed := range [][]byte{
		[]byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","version":"1.0.0","description":"demo"}`),
		[]byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo","name":"duplicate"}`),
		[]byte(`{"$schema":`),
		[]byte(`[]`),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body []byte) {
		if len(body) > 1<<20 {
			t.Skip()
		}
		decoder.Plugin(body)
	})
}
