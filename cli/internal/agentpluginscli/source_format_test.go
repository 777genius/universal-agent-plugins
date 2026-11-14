package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func TestAcquiredSnapshotSelectsValidPortablePackage(t *testing.T) {
	t.Parallel()
	app := formatSelectionApp(t)
	root := writeCLIPlugin(t)

	envelope, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.FormatID != domain.FormatIDAgentPluginsV1 {
		t.Fatalf("selected format = %q", envelope.FormatID)
	}
}

func TestAcquiredSnapshotSelectsValidOfficialOnlyPackage(t *testing.T) {
	t.Parallel()
	app := formatSelectionApp(t)
	root := writeOfficialManifest(t, `{"name":"official-demo","version":"1.0.0"}`)

	envelope, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if envelope.FormatID != domain.FormatIDOpenAIPlugin {
		t.Fatalf("selected format = %q", envelope.FormatID)
	}
}

func TestAcquiredSnapshotInvalidPortableRefusesOfficialFallback(t *testing.T) {
	t.Parallel()
	app := formatSelectionApp(t)
	root := writeOfficialManifest(t, `{"name":"official-demo"}`)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{"name":`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root})
	assertLoadDiagnostic(t, err, "plugin_manifest_malformed", "plugin.json")
}

func TestAcquiredSnapshotNonRegularPortableManifestRefusesFallback(t *testing.T) {
	t.Parallel()
	app := formatSelectionApp(t)
	root := writeOfficialManifest(t, `{"name":"official-demo"}`)
	if err := os.Mkdir(filepath.Join(root, "plugin.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root})
	assertLoadDiagnostic(t, err, "plugin_manifest_read_failed", "plugin.json")
}

func TestAcquiredSnapshotWithoutEitherManifestKeepsMissingDiagnostic(t *testing.T) {
	t.Parallel()
	app := formatSelectionApp(t)

	_, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: t.TempDir()})
	assertLoadDiagnostic(t, err, "plugin_manifest_missing", "plugin.json")
	if err == nil || err.Error() != "expected root plugin.json or .codex-plugin/plugin.json" {
		t.Fatalf("missing-manifest diagnostic = %v", err)
	}
}

func TestAcquiredSnapshotUsesExplicitInjectedLoaderDeterministically(t *testing.T) {
	t.Parallel()
	portableFailure := errors.New("portable failure")
	portable := &recordingPackageLoader{err: portableFailure}
	native := &recordingPackageLoader{envelope: domain.PackageEnvelope{FormatID: domain.FormatIDOpenAIPlugin}}
	app := App{PackageLoader: portable, NativePackageLoader: native}
	root := writeOfficialManifest(t, `{"name":"official-demo"}`)
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if !errors.Is(err, portableFailure) {
		t.Fatalf("portable failure was not preserved: %v", err)
	}
	if portable.calls != 1 || native.calls != 0 {
		t.Fatalf("loader calls after portable selection = portable %d, native %d", portable.calls, native.calls)
	}
	if err := os.Remove(filepath.Join(root, "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root}); err != nil {
		t.Fatal(err)
	}
	if portable.calls != 1 || native.calls != 1 {
		t.Fatalf("loader calls after native selection = portable %d, native %d", portable.calls, native.calls)
	}
	if err := os.Remove(filepath.Join(root, ".codex-plugin", "plugin.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := app.loadAcquiredSnapshot(context.Background(), domain.LoadInput{SnapshotRoot: root}); err != nil {
		t.Fatal(err)
	}
	if portable.calls != 1 || native.calls != 2 {
		t.Fatalf("loader calls after missing selection = portable %d, native %d", portable.calls, native.calls)
	}
}

type recordingPackageLoader struct {
	calls    int
	envelope domain.PackageEnvelope
	err      error
}

func (loader *recordingPackageLoader) Load(_ context.Context, _ domain.LoadInput) (domain.PackageEnvelope, error) {
	loader.calls++
	return loader.envelope, loader.err
}

func formatSelectionApp(t *testing.T) App {
	t.Helper()
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	portable := loader.Loader{Registry: registry}
	return App{PackageLoader: portable, NativePackageLoader: loader.OpenAILoader{Loader: portable}}
}

func writeOfficialManifest(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, ".codex-plugin")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func assertLoadDiagnostic(t *testing.T, err error, code, path string) {
	t.Helper()
	var loadErr *domain.LoadError
	if !errors.As(err, &loadErr) || loadErr.Diagnostic.Code != code || loadErr.Diagnostic.Path != path {
		t.Fatalf("load error = %v, want code %q at %q", err, code, path)
	}
}

var _ ports.PackageLoader = (*recordingPackageLoader)(nil)
