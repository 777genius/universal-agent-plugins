package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestDirectImperativeRepairUsesRecordedBytes(t *testing.T) {
	for _, kind := range []string{"git", "zip", "local-archive"} {
		t.Run(kind, func(t *testing.T) { directImperativeRepairCase(t, kind) })
	}
}

func directImperativeRepairCase(t *testing.T, kind string) {
	t.Helper()
	format := kind
	if kind == "local-archive" {
		format = "zip"
	}
	transport := newDirectTransportFixture(t, format, false)
	fixture := directCLIEnvironment(t, transport)
	kernel := nativeconfig.New()
	fixture.app.Lifecycle.NativeObserver = providers.NativeIdentityObserver{Stager: fixture.app.Lifecycle.Stager, Registry: fixture.app.ClientRegistry, NativeConfig: &kernel}
	originalBytes := append([]byte{}, transport.payload...)
	endpoint := transport.server.URL + "/package.zip"
	if kind == "git" {
		endpoint = transport.server.URL + "/repo.git"
	}
	if kind == "local-archive" {
		endpoint = filepath.Join(t.TempDir(), "package@1.0.zip")
		dependencyWrite(t, endpoint, string(transport.payload))
	}
	dependencyRun(t, fixture, "add", endpoint, "--no-save", "--target", "gemini", "--format", "json")
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := before.Installations[0]
	var target string
	for _, binding := range installation.Clients {
		target = binding.TargetLocator
	}
	targetManifest := filepath.Join(target, "plugin.json")
	manifestBefore := dependencyRead(t, targetManifest)
	directAdvanceTransport(t, transport, format)
	if kind != "git" {
		if kind == "local-archive" {
			dependencyWrite(t, endpoint, string(transport.payload))
		}
		stateBefore := dependencyRead(t, fixture.store.Path)
		if _, _, err := fixture.execute(false, "repair", installation.InstallationID, "--target", "gemini", "--format", "json"); err == nil || !strings.Contains(err.Error(), "SHA256") {
			t.Fatalf("changed archive bytes did not fail closed: %v", err)
		}
		if dependencyRead(t, fixture.store.Path) != stateBefore || dependencyRead(t, targetManifest) != manifestBefore {
			t.Fatal("failed immutable repair changed state or target")
		}
		if kind == "local-archive" {
			dependencyWrite(t, endpoint, string(originalBytes))
		} else {
			transport.mu.Lock()
			transport.payload = originalBytes
			transport.mu.Unlock()
		}

	}
	dependencyRun(t, fixture, "repair", installation.InstallationID, "--target", "gemini", "--format", "json")
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if after.Installations[0].Source.ResolvedRevision != installation.Source.ResolvedRevision || after.Installations[0].Source.TreeDigest != installation.Source.TreeDigest {
		t.Fatal("repair advanced to the moving source")
	}
	if _, err := os.Stat(filepath.Join(target, "revision.txt")); !os.IsNotExist(err) {
		t.Fatal("repair copied newer source bytes")
	}
}

func TestSavedRepairRetainsOldPinAfterSourceAdvance(t *testing.T) {
	for _, format := range []string{"git", "zip"} {
		t.Run(format, func(t *testing.T) {
			transport := newDirectTransportFixture(t, format, false)
			fixture := directCLIEnvironment(t, transport)
			endpoint := transport.server.URL + "/package.zip"
			if format == "git" {
				endpoint = transport.server.URL + "/repo.git"
			}
			dependencyRun(t, fixture, "add", endpoint, "--target", "gemini", "--format", "json")
			pin := directLockedPin(t, fixture)
			state, err := fixture.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			installation := state.Installations[0]
			var target string
			for _, binding := range installation.Clients {
				target = binding.TargetLocator
			}
			dependencyWrite(t, filepath.Join(target, "drift.txt"), "drift")
			directAdvanceTransport(t, transport, format)
			dependencyRun(t, fixture, "repair", installation.InstallationID, "--target", "gemini", "--format", "json")
			after := directLockedPin(t, fixture)
			if after.Revision() != pin.Revision() || after.TreeDigest != pin.TreeDigest {
				t.Fatal("repair changed saved pin")
			}
			if format == "zip" {
				directColdArchiveRefusal(t, fixture, transport)
			}
		})
	}
}

func directColdArchiveRefusal(t *testing.T, original cliFixture, transport *directTransportFixture) {
	t.Helper()
	fresh := directCLIEnvironment(t, transport)
	oldManifest, oldLock := original.app.dependencyPaths()
	manifest, lock := fresh.app.dependencyPaths()
	dependencyWrite(t, manifest, dependencyRead(t, oldManifest))
	dependencyWrite(t, lock, dependencyRead(t, oldLock))
	beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
	if _, _, err := fresh.execute(false, "deps", "sync", "--locked", "--yes", "--format", "json"); err == nil || !strings.Contains(err.Error(), "SHA256") {
		t.Fatalf("cold restore accepted changed archive bytes: %v", err)
	}
	if dependencyRead(t, manifest) != beforeManifest || dependencyRead(t, lock) != beforeLock {
		t.Fatal("failed cold restore changed portable agreement")
	}
	for _, path := range []string{fresh.app.EnvironmentStoreRoot, fresh.store.Path} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("failed restore published %s", filepath.Base(path))
		}
	}
}
