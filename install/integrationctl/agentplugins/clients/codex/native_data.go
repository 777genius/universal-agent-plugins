package codex

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// codexNativeDataBase is where Codex keeps PLUGIN_DATA for a plugin whose
// package carries a root Agent Plugins manifest. Codex then ignores the
// projected data path, so the installer's own purge never sees this directory.
func codexNativeDataBase(profileRoot string) string {
	return filepath.Join(profileRoot, "plugins", "data", "agent-plugins")
}

// codexNativeDataDir is Codex's sha256 of "<marketplace>\x00<plugin>" under
// codexNativeDataBase, verified against Codex 0.162.0.
func codexNativeDataDir(profileRoot, marketplace, plugin string) string {
	sum := sha256.Sum256([]byte(marketplace + "\x00" + plugin))
	return filepath.Join(codexNativeDataBase(profileRoot), hex.EncodeToString(sum[:]))
}

// purgeCodexNativeData removes only that one directory, and only when it is a
// strict child of the data root with no symlink below it.
func purgeCodexNativeData(ctx context.Context, profileRoot, marketplace, plugin string) error {
	target := codexNativeDataDir(profileRoot, marketplace, plugin)
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect Codex plugin data: %w", err)
	}
	if err := pathpolicy.RequireContainedChild(codexNativeDataBase(profileRoot), target); err != nil {
		return fmt.Errorf("refuse to purge Codex plugin data: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("purge Codex plugin data: %w", err)
	}
	return nil
}

// purgeRequestedNativeData runs after the native removal is already confirmed.
// A failure here cannot undo it, so it is reported as a manual step instead of
// failing a removal that has happened.
func purgeRequestedNativeData(ctx context.Context, outcome domain.DeactivationOutcome, request domain.DeactivationRequest, marketplace string) domain.DeactivationOutcome {
	if !request.PurgeData {
		return outcome
	}
	profileRoot := request.Client.ConfigRoot
	if err := purgeCodexNativeData(ctx, profileRoot, marketplace, request.DeclaredName); err != nil {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions,
			fmt.Sprintf("Codex kept plugin data that --purge-data could not remove (%v); delete %s yourself", err, codexNativeDataDir(profileRoot, marketplace, request.DeclaredName)))
	}
	return outcome
}
