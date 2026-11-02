package providers_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func clientsKimiEnv() clients.Env { return clients.Env{NativeConfig: nativeconfig.New()} }

func TestProjectKimiStdioBindsImmutablePackageAndSelectedData(t *testing.T) {
	if !managedstdio.Supported() {
		t.Skip("managed stdio platform unsupported")
	}
	f := newKimiProjectFixture(t, true, true)
	out, err := f.activator(nativeconfig.New()).Activate(context.Background(), f.request)
	kimiPrepared(t, out, err)
	body, err := os.ReadFile(f.config)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"transport":"stdio"`, `"PLUGIN_DATA"`, "stdio-v1"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("Kimi launcher contract omitted %s: %s", want, body)
		}
	}
	active := f.request.Plan.ActivePath
	projectGeminiWrite(t, filepath.Join(f.other, "data", "marker"), "other data\n")
	before := projectGeminiInventory(t, f.base)
	objects, err := f.stager.ProjectActiveNative(context.Background(), f.envelope, f.request.Plan, f.request.Delivery.ArtifactDigest, f.data)
	if err != nil || len(objects) != 2 {
		t.Fatalf("correct receipt projection = %+v, %v", objects, err)
	}
	for _, selected := range []string{"", "relative", filepath.Join(f.other, "data")} {
		if _, err := f.stager.ProjectActiveNative(context.Background(), f.envelope, f.request.Plan, f.request.Delivery.ArtifactDigest, selected); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
			t.Fatalf("wrong selected stdio data %q = %v", selected, err)
		}
	}
	if !reflect.DeepEqual(before, projectGeminiInventory(t, f.base)) {
		t.Fatal("active receipt inspection rewrote immutable package, native config, user or B")
	}
	if _, err := os.Stat(filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)); err != nil {
		t.Fatal(err)
	}
}
