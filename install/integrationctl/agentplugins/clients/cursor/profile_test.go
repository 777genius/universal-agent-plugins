package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestPublicIdentityCanonicalSelectedRoot(t *testing.T) {
	base := testBase(t)
	physicalRoot := filepath.Join(base, "TEST-physical")
	mustMkdir(t, physicalRoot)
	alias := filepath.Join(base, "TEST-alias")
	if err := os.Symlink(physicalRoot, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	engine := testEngine(t, filepath.Join(base, "state"), "")
	request := installer.IdentityRequest{ClientID: "cursor", InstallationID: "00000000-0000-4000-8000-000000000001", DeclaredName: "test-plugin", ClientConfigRoot: filepath.Join(physicalRoot, "missing", "profile")}
	physical, err := engine.ReserveIdentity(request)
	if err != nil {
		t.Fatal(err)
	}
	request.ClientConfigRoot = filepath.Join(alias, "missing", "profile")
	throughAlias, err := engine.ReserveIdentity(request)
	if err != nil || throughAlias != physical || physical.TargetPath == "" {
		t.Fatalf("physical=%+v alias=%+v err=%v", physical, throughAlias, err)
	}
	if _, err := os.Lstat(filepath.Join(physicalRoot, "missing")); !os.IsNotExist(err) {
		t.Fatalf("identity created parents: %v", err)
	}
	file := filepath.Join(base, "file")
	mustWrite(t, file, "TEST file", 0600)
	for _, root := range []string{"", "relative", base + string(filepath.Separator), file, filepath.Join(file, "child")} {
		_, _, err := New().TargetRoot(domain.DetectedClient{ConfigRoot: root}, domain.PackageNative, filepath.Join(base, "managed"))
		if err == nil {
			t.Errorf("accepted invalid root %q", root)
		}
	}
}

func TestSelectedProfileValidatorRejectsOtherBinding(t *testing.T) {
	base := testBase(t)
	selected, other := filepath.Join(base, "TEST-A"), filepath.Join(base, "TEST-B")
	mustMkdir(t, selected)
	mustMkdir(t, other)
	binding := domain.ClientBinding{PhysicalArtifact: "test-plugin-123456789abc", TargetLocator: filepath.Join(other, "plugins", "local", "test-plugin-123456789abc")}
	registry, err := clients.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	validator, ok := clients.As[clients.ProfileBindingValidator](registry, domain.ClientCursor)
	if !ok {
		t.Fatal("selected-profile validation capability is missing")
	}
	if err := validator.ValidateBindingProfile(selected, binding); err == nil {
		t.Fatal("foreign profile accepted")
	}
	binding.TargetLocator = filepath.Join(selected, "plugins", "local", binding.PhysicalArtifact)
	if err := validator.ValidateBindingProfile(selected, binding); err != nil {
		t.Fatalf("exact recorded package rejected: %v", err)
	}
}

func testBase(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
}
func mustWrite(t *testing.T, path, body string, mode os.FileMode) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}
func testEngine(t *testing.T, root, helper string) *installer.Engine {
	t.Helper()
	registry, err := clients.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	engine, err := installer.New(installer.Config{StateRoot: root, Registry: registry, HelperExecutable: helper, TrustedLocalPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}
