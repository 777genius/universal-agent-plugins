//go:build linux && grokreadonlynative

package grok

import (
	"context"
	"flag"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

var (
	readonlyNativeHome    = flag.String("grok-readonly-home", "", "original disposable HOME")
	readonlyNativeProject = flag.String("grok-readonly-project", "", "original disposable project")
	readonlyNativeBinary  = flag.String("grok-readonly-native", "", "original pinned native executable")
	readonlyNativePath    = flag.String("grok-readonly-path", "", "exact installed plugin path")
	readonlyNativeName    = flag.String("grok-readonly-name", "", "exact installed plugin name")
	readonlyNativeVersion = flag.String("grok-readonly-version", "", "exact installed plugin version")
)

// This tagged test runs only through the existing supervised native Journey.
// Missing fixture inputs fail; a normal Core run does not select or skip it.
func TestEnablementReadonlyNativeInventory(t *testing.T) {
	for _, path := range []string{*readonlyNativeHome, *readonlyNativeProject, *readonlyNativeBinary, *readonlyNativePath} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			t.Fatal("original absolute native fixture inputs are required")
		}
	}
	if *readonlyNativeName == "" || *readonlyNativeVersion == "" {
		t.Fatal("original selected package identity is required")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	recovery := EnablementRecovery{ProjectRoot: *readonlyNativeProject,
		UserRoot: filepath.Join(*readonlyNativeHome, ".grok"), Kernel: nativeconfig.New()}
	profile, err := recovery.openEnablementProfile(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := profile.close(); err != nil {
			t.Error(err)
		}
	})
	inputs, err := captureEnablementInventoryInputs(ctx, profile, *readonlyNativeBinary, *readonlyNativeProject, *readonlyNativeHome)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := inputs.close(); err != nil {
			t.Error(err)
		}
	})
	for range 2 {
		rows, err := readEnablementInventory(ctx, process.OS{}, inputs, profile.config.Original())
		if err != nil {
			t.Fatal(err)
		}
		selected, err := selectEnablementPlugin(rows, *readonlyNativePath, *readonlyNativeName, *readonlyNativeVersion)
		if err != nil || selected.enabled || selected.trusted {
			t.Fatal("native inventory differs from the original untrusted disabled fixture", err)
		}
	}
	fmt.Println("GROK_READONLY_NATIVE_INVENTORY_PASS")
}
