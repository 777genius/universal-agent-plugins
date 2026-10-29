//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func windContractGitInit(t *testing.T, w *windComponentFixture, scope string) (string, windProfileRecord, []byte) {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	root := w.recipes[scope].Project
	command := exec.CommandContext(w.f.ctx, git, "-c", "core.hooksPath=/dev/null", "init", "--quiet")
	command.Dir = root
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + w.f.base.Home, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1"}
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("inert Git init: %v %s", err, out)
	}
	path := filepath.Join(root, ".git", "info", "exclude")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := windOwnedTree(w.f.ctx, filepath.Join(root, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	return path, tree["info/exclude"], raw
}

func TestWindsurfComponentGitPreparationPreservesOriginalBytesAndFreezesOnce(t *testing.T) {
	w := windComponentInert(t)
	for _, scope := range []string{"a", "b"} {
		path, before, original := windContractGitInit(t, w, scope)
		if err := w.gitFreeze("git-" + scope); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(raw, append(append([]byte{}, original...), []byte(".devin/config.local.json\n.devin/mcp_config.local.json\n")...)) {
			t.Fatal("original Git bytes or exact source-derived rules differ", err)
		}
		root := filepath.Dir(filepath.Dir(path))
		after := w.trees[root]["info/exclude"]
		if before.Kind != after.Kind || before.Mode != after.Mode || before.Device != after.Device || before.Inode != after.Inode || before.UID != after.UID || before.GID != after.GID || before.Links != after.Links {
			t.Fatal("preparation replaced Git exclude authority")
		}
		if err := windFrozenTrees(w.f.ctx, w.trees); err != nil {
			t.Fatal(err)
		}
		if w.gitFreeze("git-"+scope) == nil || w.trees[root]["info/exclude"] != after {
			t.Fatal("already frozen Git authority renewed")
		}
		if _, err := os.Stat(filepath.Join(w.recipes[scope].Project, ".devin", "config.local.json")); !os.IsNotExist(err) {
			t.Fatal("local config authored")
		}
	}
}

func TestWindsurfComponentGitPreparationKeepsUnknownBytesAndExistingRulesUntouched(t *testing.T) {
	for _, body := range []string{"# caller bytes\x00 preserved", "# caller\r\n.devin/mcp_config.local.json\r\n.devin/config.local.json\n"} {
		w := windComponentInert(t)
		path, _, _ := windContractGitInit(t, w, "a")
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
		root := filepath.Dir(filepath.Dir(path))
		before, err := windOwnedTree(w.f.ctx, root)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.gitFreeze("git-a"); err != nil {
			t.Fatal(err)
		}
		raw, err := os.ReadFile(path)
		if err != nil || !bytes.HasPrefix(raw, []byte(body)) {
			t.Fatal("original unknown bytes reconstructed", err)
		}
		if bytes.Contains([]byte(body), []byte(".devin/config.local.json")) {
			if !bytes.Equal(raw, []byte(body)) || !reflect.DeepEqual(before, w.trees[root]) {
				t.Fatal("already present rules changed Git bytes or metadata")
			}
		} else if string(raw[len(body):]) != "\n.devin/config.local.json\n.devin/mcp_config.local.json\n" {
			t.Fatal("missing separator or wrong authored rules")
		}
	}
}

func TestWindsurfComponentGitNoWriteRefusesSameByteTimestampRenewal(t *testing.T) {
	w := windComponentInert(t)
	path, _, _ := windContractGitInit(t, w, "a")
	body := []byte(".devin/config.local.json\n.devin/mcp_config.local.json\n")
	if err := os.WriteFile(path, body, 0644); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if file.Close() != nil {
			t.Error("inert held file close")
		}
	}()
	before, err := windFileStat(file)
	if err != nil {
		t.Fatal(err)
	}
	windContractGitMutation(t, path, body, "timestamp")
	if _, err = windGitExcludeAppend(file, before, body); err == nil {
		t.Fatal("same-byte no-write timestamp mutation adopted")
	}
}

func TestWindsurfComponentGitPreparationRefusesChangedAuthority(t *testing.T) {
	for _, kind := range []string{"missing", "alias", "hardlink", "mode", "owner", "replacement", "changed-bytes", "parent-alias"} {
		t.Run(kind, func(t *testing.T) {
			w := windComponentInert(t)
			path, before, raw := windContractGitInit(t, w, "a")
			windContractGitMutation(t, path, raw, kind)
			if _, err := windGitExcludePrepare(filepath.Dir(path), before); err == nil {
				t.Fatal("changed initial exclude authority accepted", kind)
			}
			if _, frozen := w.trees[filepath.Dir(filepath.Dir(path))]; frozen {
				t.Fatal("refused Git preparation registered a baseline")
			}
		})
	}
}

func windContractGitMutation(t *testing.T, path string, raw []byte, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "missing":
		err = os.Remove(path)
	case "alias":
		err = os.Rename(path, path+"-original")
		if err == nil {
			err = os.Symlink(path+"-original", path)
		}
	case "hardlink":
		err = os.Link(path, path+"-link")
	case "mode":
		err = os.Chmod(path, 0600)
	case "owner":
		if os.Getuid() != 0 {
			t.Skip("actual ownership control requires root")
		}
		err = os.Chown(path, 1, 1)
	case "replacement":
		err = os.WriteFile(path+"-replacement", raw, 0644)
		if err == nil {
			err = os.Rename(path+"-replacement", path)
		}
	case "changed-bytes":
		err = os.WriteFile(path, append(append([]byte{}, raw...), 'x'), 0644)
	case "parent-alias":
		dir := filepath.Dir(path)
		err = os.Rename(dir, dir+"-original")
		if err == nil {
			err = os.Symlink(dir+"-original", dir)
		}
	case "timestamp":
		stamp := time.Now().Add(time.Hour)
		err = os.Chtimes(path, stamp, stamp)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestWindsurfComponentPreparedGitRemainsFrozenAgainstLaterWrites(t *testing.T) {
	for _, kind := range []string{"changed-bytes", "replacement", "timestamp", "missing", "alias", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			w := windComponentInert(t)
			path, _, _ := windContractGitInit(t, w, "a")
			if err := w.gitFreeze("git-a"); err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			windContractGitMutation(t, path, raw, kind)
			if windFrozenTrees(w.f.ctx, w.trees) == nil || w.gitFreeze("git-a") == nil {
				t.Fatal("later Git mutation or baseline renewal accepted", kind)
			}
		})
	}
}
