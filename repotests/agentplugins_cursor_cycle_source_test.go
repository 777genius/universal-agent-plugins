//go:build linux

package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (c *cursorMCPProjectCycle) prepare(t *testing.T) {
	t.Helper()
	f := c.f
	if err := f.check(); err != nil {
		t.Fatal(err)
	}
	var err error
	c.installerBody, err = projectNativeRuntimeRead(f.installer, 64<<20)
	if err != nil || clineAttemptSHA(c.installerBody) != f.held[f.installer].digest {
		t.Fatal("Cursor cycle trusted launcher bytes differ", err)
	}
	c.probeBody, err = projectNativeRuntimeRead(f.base.Probe, 32<<20)
	if err != nil || clineAttemptSHA(c.probeBody) != f.held[f.base.Probe].digest {
		t.Fatal("Cursor cycle trusted probe bytes differ", err)
	}
	user := filepath.Join(f.base.Home, ".cursor", "mcp.json")
	c.forget(user)
	for path, body := range map[string][]byte{user: []byte("{\"mcpServers\":{}}\n"), filepath.Join(f.base.Home, "foreign-owned.txt"): []byte("caller-owned Cursor control\n"), f.base.Events: {}} {
		projectNativeRuntimeWrite(t, path, body, 0600)
		if path != f.base.Events {
			if err := c.acceptFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	c.probeInfo, err = os.Lstat(f.base.Events)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := os.Lstat(filepath.Dir(user))
	if err != nil {
		t.Fatal(err)
	}
	f.approvalParents = map[string]os.FileInfo{filepath.Dir(user): parent}
	for _, scope := range []string{"a", "b"} {
		project := f.base.Project
		if scope == "b" {
			project = f.other
		}
		source := f.base.Package
		if scope == "b" {
			source = filepath.Join(f.base.Root, "package-B")
		}
		r := &cursorCycleRecipe{Scope: scope, Project: project, Source: source, Version: "1.0.0", Config: filepath.Join(project, ".cursor", "mcp.json")}
		c.recipes[scope] = r
		foreign := filepath.Join(project, ".cursor", "foreign-owned.txt")
		projectNativeRuntimeWrite(t, foreign, []byte("caller-owned "+scope+"\n"), 0600)
		if err := c.acceptFile(foreign, []byte("caller-owned "+scope+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
		c.author(t, r)
		c.acceptTree(t, filepath.Dir(r.Config), map[string][]byte{"foreign-owned.txt": []byte("caller-owned " + scope + "\n")}, false)
	}
	f.evidence["cycle_layout"] = map[string]any{"root": f.base.Root, "home": f.base.Home, "a": f.base.Project, "b": f.other, "outside": f.outside, "source_a": c.recipes["a"].Source, "source_b": c.recipes["b"].Source, "nonce": f.base.Nonce, "installer": f.installer, "client": f.client, "git": f.git, "installer_sha256": clineAttemptSHA(c.installerBody), "probe_sha256": clineAttemptSHA(c.probeBody)}
	c.proof(t, "original-controls", map[string]any{"user_config": user, "user_body": c.stream(t, []byte("{\"mcpServers\":{}}\n")), "foreign_user": filepath.Join(f.base.Home, "foreign-owned.txt"), "approval_controls": append([]string(nil), f.absent[3:]...)})
}

func (c *cursorMCPProjectCycle) author(t *testing.T, r *cursorCycleRecipe) {
	t.Helper()
	r.Revision = r.Scope + "-" + r.Version
	r.Nonce = clineAttemptSHA([]byte(c.f.base.Nonce + "\x00" + r.Revision))[:32]
	manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": cursorCycleName, "version": r.Version})
	server := map[string]any{"type": "stdio", "command": "./bin/probe", "args": []string{"cursor-component"}, "cwd": "${PLUGIN_ROOT}", "env": map[string]string{"UAP_TEST_ROOT": c.f.base.Root, "UAP_TEST_EVENTS": c.f.base.Events, "UAP_TEST_NONCE": r.Nonce, "UAP_TEST_REVISION": r.Revision}}
	mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{"uapcursora": server}})
	r.Authored = map[string][]byte{"plugin.json": append(manifest, '\n'), "mcp.json": append(mcp, '\n'), "bin/probe": c.probeBody}
	entries := make([]packagedigest.CapturedEntry, 1, 1+len(r.Authored))
	entries[0] = packagedigest.CapturedEntry{Path: "bin", Kind: "directory"}
	for name, body := range r.Authored {
		path, mode := filepath.Join(r.Source, filepath.FromSlash(name)), os.FileMode(0600)
		if name == "bin/probe" {
			mode = 0700
		}
		if held, exists := c.f.held[path]; exists {
			if held.digest != clineAttemptSHA(body) {
				if !c.revising || r.Scope != "a" || name == "bin/probe" || cursorCycleReplaceSource(path, held, body) != nil {
					t.Fatal("Cursor controlled source revision refused")
				}
				delete(c.f.held, path)
			}
		} else {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatal("Cursor initial source path must be absent")
			}
			projectNativeRuntimeWrite(t, path, body, mode)
		}
		entries = append(entries, packagedigest.CapturedEntry{Path: name, Kind: "file", Executable: name == "bin/probe", Content: body})
	}
	var err error
	r.Digest, err = packagedigest.DigestCaptured(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	c.acceptTree(t, r.Source, r.Authored, true)
	c.proof(t, "source-"+r.Scope+"-"+r.Version, map[string]any{"scope": r.Scope, "source": r.Source, "version": r.Version, "revision": r.Revision, "nonce": r.Nonce, "digest": r.Digest, "manifest": c.stream(t, r.Authored["plugin.json"]), "mcp": c.stream(t, r.Authored["mcp.json"]), "probe_sha256": clineAttemptSHA(c.probeBody)})
}

func (c *cursorMCPProjectCycle) revisedA(t *testing.T) {
	t.Helper()
	c.check(t)
	r := c.recipes["a"]
	delete(c.trees, r.Source)
	r.Version = "1.1.0"
	c.revising = true
	c.author(t, r)
	c.revising = false
	c.check(t)
}

func cursorCycleReplaceSource(path string, want cursorProjectHeld, body []byte) (err error) {
	if err := projectNativeRuntimeParents(path); err != nil {
		return err
	}
	fd, err := syscall.Open(path, syscall.O_WRONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	file := os.NewFile(uintptr(fd), "held-Cursor-source")
	defer func() {
		if file.Close() != nil {
			err = fmt.Errorf("Cursor source close")
		}
	}()
	info, err := file.Stat()
	if err != nil || !os.SameFile(info, want.info) || !projectNativeRuntimeStableStat(info.Sys(), want.info.Sys()) {
		return fmt.Errorf("Cursor source changed before revision")
	}
	if err := file.Truncate(0); err != nil {
		return err
	}
	if n, err := file.Write(body); err != nil || n != len(body) {
		return fmt.Errorf("Cursor source revision write")
	}
	return nil
}
