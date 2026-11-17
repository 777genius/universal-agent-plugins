package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The directory name is Codex's own and was observed on Codex 0.162.0 for this
// marketplace and plugin, so a change of the formula fails here first.
const (
	observedMarketplace = "agentplugins-5ee57608bcb3"
	observedPlugin      = "local-native-plugin-probe"
	observedDataDir     = "e380e494d08453224af00faa13dea0c26c7168d306573b9d62f691bce8eea760"
)

func TestPurgeCodexNativeDataRemovesOnlyThePluginDirectory(t *testing.T) {
	profile := tempRoot(t)
	base := filepath.Join(profile, "plugins", "data", "agent-plugins")
	target := filepath.Join(base, observedDataDir)
	sibling := filepath.Join(base, "other-plugin-data")
	for _, dir := range []string{target, sibling} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := purgeCodexNativeData(t.Context(), profile, observedMarketplace, observedPlugin); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("plugin data was not purged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(sibling, "state.json")); err != nil {
		t.Fatalf("another plugin's data was touched: %v", err)
	}
	if err := purgeCodexNativeData(t.Context(), profile, observedMarketplace, observedPlugin); err != nil {
		t.Fatalf("an absent directory must not fail: %v", err)
	}
}

func TestPurgeCodexNativeDataRefusesASymlinkedDirectory(t *testing.T) {
	profile := tempRoot(t)
	base := filepath.Join(profile, "plugins", "data", "agent-plugins")
	if err := os.MkdirAll(base, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := tempRoot(t)
	keep := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(keep, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, observedDataDir)); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	if err := purgeCodexNativeData(t.Context(), profile, observedMarketplace, observedPlugin); err == nil {
		t.Fatal("a symlinked data directory must be refused")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatalf("the symlink target was deleted: %v", err)
	}
}

// removalWithData runs a confirmed Codex removal over a profile that holds the
// plugin's native data directory, and reports whether that directory survived.
func removalWithData(t *testing.T, request domain.DeactivationRequest, runner *profileRunner, withCLI bool, registered bool) (domain.DeactivationOutcome, bool, error) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, artifact := tempRoot(t), tempRoot(t)
	marketplace := shared.ManagedMarketplaceName("artifact")
	if registered {
		config := fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"local\"\n", marketplace, artifact)
		if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	data := codexNativeDataDir(root, marketplace, "demo")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	request.Client = domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root}
	request.DeclaredName, request.PhysicalArtifactID, request.ManagedArtifactPath = "demo", "artifact", artifact
	request.Confirmed, request.ExternalUninstalled = true, true
	if withCLI {
		request.BackendExecutable = executable
	}
	outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: runner}, request)
	_, statErr := os.Stat(data)
	return outcome, statErr == nil, err
}

// Deleting user data must follow the user's --purge-data and a removal Codex
// itself confirmed, never an ordinary remove or an unverified one.
func TestCodexRemovalPurgesNativeDataOnlyWhenRequestedAndVerified(t *testing.T) {
	for _, test := range []struct {
		name     string
		purge    bool
		failList bool
		wantData bool
		wantErr  bool
	}{
		{name: "purge requested", purge: true},
		{name: "ordinary remove keeps data", wantData: true},
		{name: "unverified removal keeps data", purge: true, failList: true, wantData: true, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner := &profileRunner{}
			if test.failList {
				runner.fail = "plugin list"
			}
			outcome, kept, err := removalWithData(t, domain.DeactivationRequest{PurgeData: test.purge}, runner, true, true)
			if (err != nil) != test.wantErr || kept != test.wantData || (!test.wantErr && !outcome.ExternalRemovalComplete) {
				t.Fatalf("err=%v kept=%v outcome=%+v", err, kept, outcome)
			}
		})
	}
}

func TestCodexRemovalWithoutCLIPurgesNativeDataWhenNothingIsRegistered(t *testing.T) {
	outcome, kept, err := removalWithData(t, domain.DeactivationRequest{PurgeData: true}, &profileRunner{}, false, false)
	if err != nil || !outcome.ExternalRemovalComplete || kept {
		t.Fatalf("err=%v kept=%v outcome=%+v", err, kept, outcome)
	}
}

// The native removal has already happened, so a data directory that cannot be
// removed must not turn it into a failed removal that repair would redo.
func TestCodexRemovalReportsUnpurgeableDataWithoutFailing(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, artifact, outside := tempRoot(t), tempRoot(t), tempRoot(t)
	marketplace := shared.ManagedMarketplaceName("artifact")
	config := fmt.Sprintf("[marketplaces.%s]\nsource = %q\nsource_type = \"local\"\n", marketplace, artifact)
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	data := codexNativeDataDir(root, marketplace, "demo")
	if err := os.MkdirAll(filepath.Dir(data), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, data); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	outcome, err := New().Deactivate(t.Context(), clients.Env{Runner: &profileRunner{}}, domain.DeactivationRequest{
		Client:       domain.DetectedClient{ClientID: domain.ClientCodex, ConfigRoot: root},
		DeclaredName: "demo", PhysicalArtifactID: "artifact", ManagedArtifactPath: artifact,
		BackendExecutable: executable, Confirmed: true, ExternalUninstalled: true, PurgeData: true,
	})
	if err != nil || !outcome.ExternalRemovalComplete || len(outcome.UserActions) != 1 || !strings.Contains(outcome.UserActions[0], data) {
		t.Fatalf("err=%v outcome=%+v", err, outcome)
	}
}
