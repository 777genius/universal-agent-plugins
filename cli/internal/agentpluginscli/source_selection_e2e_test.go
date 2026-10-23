package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func directPackageLayout(t *testing.T, paths ...string) string {
	t.Helper()
	source := writeCLIPlugin(t)
	writeCLIMCP(t, source)
	root := t.TempDir()
	for _, packagePath := range paths {
		directWalkPackage(t, source, func(name string, body []byte) error {
			target := filepath.Join(root, packagePath, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			return os.WriteFile(target, body, 0600)
		})
	}
	return root
}

func TestDirectArchiveSelectionUsesOneDownloadAndLogicalPath(t *testing.T) {
	for _, count := range []int{1, 2} {
		t.Run(strings.Repeat("package", count), func(t *testing.T) {
			transport := newDirectTransportFixture(t, "zip", false)
			fixture := directCLIEnvironment(t, transport)
			paths := []string{"plugins/demo"}
			if count == 2 {
				paths = append(paths, "plugins/other")
			}
			transport.payload = directArchiveBytes(t, directPackageLayout(t, paths...), "zip", "changing-provider-wrapper/")
			endpoint := transport.server.URL + "/packages.zip"
			_, _, err := fixture.execute(false, "add", endpoint, "--target", "gemini", "--format", "json")
			if transport.downloads != 1 {
				t.Fatalf("selection downloaded source %d times", transport.downloads)
			}
			if count == 2 {
				if err == nil || !strings.Contains(err.Error(), "multiple packages") {
					t.Fatalf("multiple candidates were not refused: %v", err)
				}
				if _, err := os.Stat(fixture.app.EnvironmentStoreRoot); !os.IsNotExist(err) {
					t.Fatal("failed selection published store")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			pin := directLockedPin(t, fixture)
			if pin.Source.Archive.Path != "plugins/demo" {
				t.Fatalf("provider wrapper entered logical path: %+v", pin.Source.Archive)
			}
			transport.mu.Lock()
			transport.payload = directArchiveBytes(t, directPackageLayout(t, paths...), "zip", "next-provider-wrapper/")
			transport.mu.Unlock()
			dependencyRun(t, fixture, "update", pin.Alias, "--target", "gemini", "--format", "json")
			after := directLockedPin(t, fixture)
			if after.SourceKey != pin.SourceKey || after.Source.Archive.Path != pin.Source.Archive.Path {
				t.Fatal("wrapper update changed selected source identity")
			}
		})
	}
}

func TestDirectRecognizedInvalidPackagesNeverInstallOrFallback(t *testing.T) {
	for _, failure := range []string{"corrupt-zip", "raw-skill", "malformed-root"} {
		t.Run(failure, func(t *testing.T) {
			transport := newDirectTransportFixture(t, "zip", false)
			fixture := directCLIEnvironment(t, transport)
			switch failure {
			case "corrupt-zip":
				transport.payload = []byte("PK\x03\x04corrupt archive")
			case "raw-skill":
				root := t.TempDir()
				dependencyWrite(t, filepath.Join(root, "SKILL.md"), "---\nname: raw\ndescription: raw skill\n---\nInstructions")
				transport.payload = directArchiveBytes(t, root, "zip", "")
			case "malformed-root":
				root := directPackageLayout(t, "nested/demo")
				dependencyWrite(t, filepath.Join(root, "plugin.json"), "{malformed")
				transport.payload = directArchiveBytes(t, root, "zip", "")
			}
			_, _, err := fixture.execute(false, "add", transport.server.URL+"/package.zip", "--target", "gemini", "--format", "json")
			if err == nil {
				t.Fatal("invalid complete-package source installed")
			}
			if transport.downloads != 1 {
				t.Fatalf("recognized archive retried as Git: %d requests", transport.downloads)
			}
			manifest, lock := fixture.app.dependencyPaths()
			for _, path := range []string{manifest, lock, fixture.app.EnvironmentStoreRoot, fixture.store.Path} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("invalid source published %s", filepath.Base(path))
				}
			}
		})
	}
}
