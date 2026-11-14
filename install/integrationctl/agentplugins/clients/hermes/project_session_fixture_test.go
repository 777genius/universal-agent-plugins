package hermes

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// createProjectSessionRoot allocates only an absent child of an existing real
// .hermes directory. Interrupted preparation is retained, never adopted/retried.
func createProjectSessionRoot(ctx context.Context, project string) (receipt domain.NativeObjectOwnership, resultErr error) {
	if err := ctx.Err(); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	if _, err := noAliasDirectory(project); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	parent := filepath.Join(project, ".hermes")
	before, err := noAliasDirectory(parent)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	directory, err := os.OpenRoot(parent)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	heldParent, err := directory.Stat(".")
	if err != nil || !os.SameFile(before, heldParent) {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session parent changed before creation")
	}
	if err := directory.Mkdir("uap-session", 0o700); err != nil {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session requires a fresh owned root: %w", err)
	}
	childInfo, err := directory.Lstat("uap-session")
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	if err := writeSessionRootMarker(directory, childInfo, project); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	after, err := noAliasDirectory(parent)
	if err != nil || !os.SameFile(before, after) {
		return domain.NativeObjectOwnership{}, fmt.Errorf("hermes session parent changed during preparation")
	}
	if err := ctx.Err(); err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return sessionRootObject(project), nil
}

func sessionChildIdentity(parent *os.Root, original os.FileInfo) error {
	named, err := parent.Lstat("uap-session")
	if err != nil || !named.IsDir() || named.Mode()&os.ModeSymlink != 0 || !os.SameFile(original, named) || original.Mode() != named.Mode() {
		return fmt.Errorf("hermes fresh session child changed")
	}
	return nil
}

func writeSessionRootMarker(parent *os.Root, original os.FileInfo, project string) (resultErr error) {
	if err := sessionChildIdentity(parent, original); err != nil {
		return err
	}
	child, err := parent.OpenRoot("uap-session")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, child.Close()) }()
	held, err := child.Stat(".")
	if err != nil || !os.SameFile(original, held) || original.Mode() != held.Mode() {
		return fmt.Errorf("hermes fresh child changed before open")
	}
	marker, err := child.OpenFile(".uap-owner", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := sessionChildIdentity(parent, original); err != nil {
		return errors.Join(err, marker.Close())
	}
	written, writeErr := marker.Write(sessionRootBody(project))
	closeErr := marker.Close()
	if writeErr != nil || closeErr != nil || written != len(sessionRootBody(project)) {
		return fmt.Errorf("hermes session marker creation failed")
	}
	return sessionChildIdentity(parent, original)
}

func sessionBundleFixture(t *testing.T, base string) string {
	t.Helper()
	payload := filepath.Join(base, "inert-bundle")
	launcher := "hermes"
	if runtime.GOOS == "windows" {
		launcher += ".exe"
	}
	target := map[string]string{"linux/amd64": "linux-x64", "darwin/arm64": "darwin-arm64", "windows/amd64": "win32-x64"}[runtime.GOOS+"/"+runtime.GOARCH]
	for _, name := range []string{"hermes-agent", "venv/site", "tools/python", "bin"} {
		if err := os.MkdirAll(filepath.Join(payload, filepath.FromSlash(name)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"bin/" + launcher, "tools/python/python"} {
		if err := os.WriteFile(filepath.Join(payload, filepath.FromSlash(name)), []byte("inert fixture; never execute"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	manifest := fmt.Sprintf(`{"schema":1,"ref":"2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19","target":%q,"repo":"hermes-agent","venv":"venv","store":"tools","launchers":["hermes"],"runtime":{"repoDir":"hermes-agent","toolsDir":"tools","storePython":"tools/python/python","sitePackages":"venv/site","commands":{"hermes":%q}}}`, target, "bin/"+launcher)
	if err := os.WriteFile(filepath.Join(payload, "manifest.json"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(payload, "bin", launcher)
}

// These bytes are the pinned 2bd378 default, also captured in original native
// artifact 14077169877. They contain no newline on any platform.
const sessionDefaultSoul = `You are Hermes Agent, built by Nous Research. Be direct: match the length of your reply to the weight of the ask — a one-line question gets a one-line answer, and finished work gets a short report of what changed, what's verified, and what's left, never a replay of the process. No filler ("Great question," "I'd be happy to"), no restating the request back, no re-summarizing what you already said, no narrating tool calls the user can see. Plain claims over adjectives; when unsure, say so plainly. Agree because it's right, not because the user said it. Depth is earned — give it when the user asks for detail, teaches, or the stakes demand it, not by default.`

func sessionBootstrapFixture(t *testing.T, project string) string {
	t.Helper()
	root := projectSessionRoot(project)
	for _, name := range []string{"cron", "sessions", "logs", "logs/curator", "memories", "pairing", "hooks", "image_cache", "audio_cache", "skills", "backups", "backups/config"} {
		mode := os.FileMode(0o700)
		if name == "backups" || name == "backups/config" {
			mode = 0o755
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(path, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	primary, err := os.ReadFile(filepath.Join(root, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	backup := "backups/config/config.yaml.good.20261113-120000"
	for name, body := range map[string][]byte{"SOUL.md": []byte(sessionDefaultSoul), backup: primary, "logs/agent.log": []byte("observed bootstrap log\n"), "logs/errors.log": nil} {
		mode := os.FileMode(0o600)
		if name == "logs/agent.log" || name == "logs/errors.log" {
			mode = 0o644
		}
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.WriteFile(path, body, mode); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, mode); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(root, filepath.FromSlash(backup))
}
