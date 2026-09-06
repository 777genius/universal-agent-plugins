package loader

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"testing"
)

func TestPortableReservedDeclaredNames(t *testing.T) {
	for _, name := range []string{"con", "con.foo", "aux.tools", "nul.x", "com1.plugin", "lpt9.plugin"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeMinimalPlugin(t, root, name)
			pkg, err := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if err != nil || pkg.Manifest.Name != name {
				t.Fatalf("declared name changed/rejected: %+v %v", pkg.Manifest, err)
			}
		})
	}
}
