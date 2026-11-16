package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
)

// purgeCodexNativeData removes the PLUGIN_DATA directory Codex keeps for a
// plugin whose package carries a root Agent Plugins manifest. Codex then
// ignores the projected data path, so the installer's own purge never sees it.
// The directory name is Codex's sha256 of "<marketplace>\x00<plugin>" under
// <profile>/plugins/data/agent-plugins, verified against Codex 0.162.0.
func purgeCodexNativeData(profileRoot, marketplace, plugin string) error {
	sum := sha256.Sum256([]byte(marketplace + "\x00" + plugin))
	base := filepath.Join(profileRoot, "plugins", "data", "agent-plugins")
	target := filepath.Join(base, hex.EncodeToString(sum[:]))
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect Codex plugin data: %w", err)
	}
	if err := pathpolicy.RequireContainedChild(base, target); err != nil {
		return fmt.Errorf("refuse to purge Codex plugin data: %w", err)
	}
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("purge Codex plugin data: %w", err)
	}
	return nil
}
