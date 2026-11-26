package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/windsurf"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Exercise the real detector's no-probe discovery followed by its bounded
// targeted version seam. No executable or native client is started by this test.
func TestDependencyNativeSiblingVersionUsesOriginalDetectedProfile(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(root, "xdg", "devin")
	if err := os.MkdirAll(profile, 0700); err != nil {
		t.Fatal(err)
	}
	registry, err := clients.NewRegistry(windsurf.New())
	if err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(root, "bin", "devin")
	probes := 0
	detector := clientdetect.Detector{
		HomeDir: root, GOOS: "linux", Registry: registry,
		Environment: map[string]string{"XDG_CONFIG_HOME": filepath.Dir(profile)},
		Lstat:       os.Lstat, ReadDir: os.ReadDir, EvalSymlinks: filepath.EvalSymlinks,
		LookPath: func(name string) (string, error) {
			if name == "devin" {
				return executable, nil
			}
			return "", os.ErrNotExist
		},
		ProbeVersionWithEnvironment: func(_ context.Context, path string, _ []string) (string, error) {
			probes++
			if path != executable {
				t.Fatal("version probe changed selected executable", path)
			}
			return "3000.12.57", nil
		},
	}
	original, err := detector.Detect(t.Context())
	if err != nil || len(original) != 1 || original[0].ConfigRoot != "" || original[0].NativeConfigRoot != profile || original[0].Version != "" || probes != 0 {
		t.Fatal("observational discovery invented legacy/version authority", original, err, probes)
	}
	adapter := dependencyInstaller{app: App{Scope: domain.ScopeUser, ClientRegistry: registry, Detector: detector},
		detected: map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: original[0]}}
	view := installer.Inspection{Installations: []installer.InspectedInstallation{{InstallationID: "same-installation",
		Bindings: []installer.InspectedBinding{{ClientID: "cline", BindingID: "selected"}, {ClientID: "windsurf", BindingID: "native"}}}}}
	if err := adapter.ensureDependencySiblingVersions(t.Context(), view, "same-installation", "cline", installer.OpRemove); err != nil || probes != 0 {
		t.Fatal("removal triggered a native sibling version probe", err, probes)
	}
	if err := adapter.ensureDependencySiblingVersions(t.Context(), view, "different-installation", "cline", installer.OpUpdate); err != nil || probes != 0 {
		t.Fatal("unrelated installation triggered a native version probe", err, probes)
	}
	if err := adapter.ensureDependencySiblingVersions(t.Context(), view, "same-installation", "cline", installer.OpUpdate); err != nil {
		t.Fatal(err)
	}
	request := installer.Request{ClientID: "cline"}
	if err := adapter.populateDependencyTargets(view, "same-installation", &request); err != nil {
		t.Fatal(err)
	}
	if probes != 1 || len(request.KnownTargets) != 1 || request.KnownTargets[0].ConfigRoot != "" ||
		request.KnownTargets[0].NativeConfigRoot != profile || request.KnownTargets[0].Version != "3000.12.57" {
		t.Fatal("native-only User sibling did not retain its actual targeted observation", request.KnownTargets, probes)
	}
	if err := adapter.ensureDependencySiblingVersions(t.Context(), view, "same-installation", "cline", installer.OpUpdate); err != nil || probes != 1 {
		t.Fatal("already observed version was probed again", err, probes)
	}
	if _, err := os.Lstat(filepath.Join(root, ".codeium")); !os.IsNotExist(err) {
		t.Fatal("native-only observation created a Cascade profile", err)
	}
}

func TestDependencyNativeSiblingVersionRejectsChangedProfile(t *testing.T) {
	original := projectVersionDevin()
	original.NativeConfigRoot = projectVersionFixturePath("xdg", "devin")
	probed := original
	probed.Version, probed.NativeConfigRoot = "3000.12.57", projectVersionFixturePath("other", "devin")
	detector := &projectVersionDetector{probed: []domain.DetectedClient{probed}}
	adapter := dependencyInstaller{app: App{Scope: domain.ScopeUser, ClientRegistry: projectVersionRegistry(t), Detector: detector},
		detected: map[domain.ClientID]domain.DetectedClient{domain.ClientWindsurf: original}}
	view := installer.Inspection{Installations: []installer.InspectedInstallation{{InstallationID: "same-installation",
		Bindings: []installer.InspectedBinding{{ClientID: "windsurf", BindingID: "native"}}}}}
	if err := adapter.ensureDependencySiblingVersions(t.Context(), view, "same-installation", "cline", installer.OpUpdate); err == nil ||
		adapter.detected[domain.ClientWindsurf].Version != "" || adapter.detected[domain.ClientWindsurf].NativeConfigRoot != original.NativeConfigRoot {
		t.Fatal("changed native profile supplied a sibling version", err)
	}
}
