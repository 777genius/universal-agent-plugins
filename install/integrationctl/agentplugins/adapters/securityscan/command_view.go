package securityscan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const commandViewScheme = "agentplugins-command-view/v1"

// commandView is the package as a client would receive it once its commands
// are delivered: the scanner understands skills, not this installer's command
// directory, so the rendered skills are what it has to see. The signed index
// describes the bare package, so it never stands in for a view.
type commandView struct {
	// skills maps each projected skill name to its client-neutral SKILL.md.
	skills map[string][]byte
	digest string
}

// newCommandView renders every valid command, explicit-only ones included,
// since a later client may deliver them. Without commands there is no view.
func newCommandView(pluginName string, commands map[string]domain.Command) (*commandView, error) {
	if len(commands) == 0 {
		return nil, nil
	}
	view := &commandView{skills: map[string][]byte{}}
	hash := sha256.New()
	hash.Write([]byte(commandViewScheme + "\x00"))
	for _, name := range slices.Sorted(maps.Keys(commands)) {
		files, err := commandext.Marshal(commandext.Render(commands[name], pluginName))
		if err != nil {
			return nil, fmt.Errorf("render command %q for security scan: %w", name, err)
		}
		body := files[commandext.SkillFile]
		view.skills[commandext.SkillName(pluginName, name)] = body
		hash.Write([]byte(name + "\x00" + strconv.Itoa(len(body)) + "\x00"))
		hash.Write(body)
	}
	view.digest = "sha256:" + hex.EncodeToString(hash.Sum(nil))
	return view, nil
}

// cacheKey binds a cached verdict to the rendered view as well as to the
// package and scanner, so new rendering rules are never answered from cache.
func (view *commandView) cacheKey(base string) string {
	digest := sha256.Sum256([]byte(base + "\x00" + view.digest))
	return hex.EncodeToString(digest[:])
}

// materialize copies the snapshot into a private directory and adds the
// rendered skills. A skills entry that is not a real directory is left out,
// as delivery removes it too. The caller removes the returned directory.
func (view *commandView) materialize(snapshotRoot string) (string, string, error) {
	scratch, err := os.MkdirTemp("", "agentplugins-command-view-*")
	if err != nil {
		return "", "", fmt.Errorf("create command scan view: %w", err)
	}
	root := filepath.Join(scratch, "package")
	if err := view.write(snapshotRoot, root); err != nil {
		_ = os.RemoveAll(scratch)
		return "", "", fmt.Errorf("create command scan view: %w", err)
	}
	return scratch, root, nil
}

func (view *commandView) write(snapshotRoot, root string) error {
	entries, err := os.ReadDir(snapshotRoot)
	if err != nil {
		return err
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "skills" && !entry.IsDir() {
			continue
		}
		if _, err := filetree.CopyPathIfExists(filepath.Join(snapshotRoot, entry.Name()), filepath.Join(root, entry.Name())); err != nil {
			return err
		}
	}
	skills := filepath.Join(root, "skills")
	if err := os.MkdirAll(skills, 0o755); err != nil {
		return err
	}
	for _, name := range slices.Sorted(maps.Keys(view.skills)) {
		directory := filepath.Join(skills, name)
		if err := os.Mkdir(directory, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, commandext.SkillFile), view.skills[name], 0o644); err != nil {
			return err
		}
	}
	return nil
}
