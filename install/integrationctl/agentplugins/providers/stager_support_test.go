package providers

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
)

// tempRoot returns a symlink-free temp directory: native root checks demand
// canonical paths, which t.TempDir() does not guarantee on macOS (/var vs
// /private/var).
func tempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// testStager completes a stager literal with the production path policy and
// the full client registry.
func testStager(base Stager) Stager {
	if base.Paths == nil {
		base.Paths = pathpolicy.Policy{}
	}
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	return base
}

// testActivator completes an activator literal with the full client registry.
func testActivator(base Activator) Activator {
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	if base.NativeConfig == nil {
		kernel := nativeconfig.New()
		base.NativeConfig = &kernel
	}
	return base
}

// testObserver completes an identity observer literal with the full client registry.
func testObserver(base NativeIdentityObserver) NativeIdentityObserver {
	if base.Registry == nil {
		base.Registry = all.Default()
	}
	if base.NativeConfig == nil {
		kernel := nativeconfig.New()
		base.NativeConfig = &kernel
	}
	return base
}
