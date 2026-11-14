package agentpluginscli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// Literal fragment plus signs must select the same complete package on first
// install and a cold locked restore, including supporting file bytes.
func TestArchiveLiteralPlusSelectorSavedAndColdRestore(t *testing.T) {
	transport := newDirectTransportFixture(t, "zip", false)
	root := t.TempDir()
	packagePath := filepath.Join(root, "plugins", "c++")
	aliasGraphPackage(t, packagePath, "plus-package")
	dependencyWrite(t, filepath.Join(packagePath, "reference.txt"), "selected c++ package resource")
	transport.payload = directArchiveBytes(t, root, "zip", "publisher-wrapper/")
	checksum := sha256.Sum256(transport.payload)
	source := transport.server.URL + "/package.zip#sha256=" + hex.EncodeToString(checksum[:]) + "&path=plugins/c++"
	fixture := directCLIEnvironment(t, transport)
	dependencyRun(t, fixture, "add", source, "--target", "gemini", "--format", "json")
	pin := directLockedPin(t, fixture)
	if pin.Source.Archive == nil || pin.Source.Archive.Path != "plugins/c++" {
		t.Fatal("literal plus path lost", pin)
	}
	state, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if got := dependencyRead(t, filepath.Join(binding.TargetLocator, "reference.txt")); got != "selected c++ package resource" {
			t.Fatal("wrong package projected", got)
		}
	}
	if err := os.RemoveAll(fixture.app.EnvironmentStoreRoot); err != nil {
		t.Fatal(err)
	}
	directColdRestore(t, fixture, transport, pin, directExpectedPackage(t, packagePath))
}
