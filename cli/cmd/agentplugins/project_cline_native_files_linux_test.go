//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func clineGateClass(label string) string {
	for _, item := range []struct {
		class  string
		labels []string
	}{
		{"git", []string{"git-user", "git-A", "git-B"}},
		{"public_uap", []string{"add-user", "add-A", "add-B", "repeat-add-A", "unchanged-update-A", "intact-repair-A", "update-A", "missing-repair-A", "remove-A", "add-same-user", "add-same-A", "add-same-B"}},
		{"native", []string{"version", "initial-A-skills", "initial-B-skills", "default-A", "default-B", "override-A", "override-B", "cross-A-from-B", "restart-default-A", "stable-A-skills", "stable-A-mcp", "updated-A-skills", "updated-A-mcp", "repaired-A-skills", "repaired-A-mcp", "removed-A-skills", "removed-A-mcp", "same-user-skills", "same-A-skills", "same-B-skills", "same-user-mcp", "same-A-mcp", "same-B-mcp"}},
		{"cleanup", []string{"cleanup"}},
	} {
		for _, name := range item.labels {
			if label == name {
				return item.class
			}
		}
	}
	return ""
}
func clineGateExpectedUsed() map[string]bool {
	result := map[string]bool{}
	for _, label := range []string{"git-user", "git-A", "git-B", "add-user", "add-A", "add-B", "repeat-add-A", "unchanged-update-A", "intact-repair-A", "update-A", "missing-repair-A", "remove-A", "add-same-user", "add-same-A", "add-same-B", "version", "initial-A-skills", "initial-B-skills", "default-A", "default-B", "override-A", "override-B", "cross-A-from-B", "restart-default-A", "stable-A-skills", "stable-A-mcp", "updated-A-skills", "updated-A-mcp", "repaired-A-skills", "repaired-A-mcp", "removed-A-skills", "removed-A-mcp", "same-user-skills", "same-A-skills", "same-B-skills", "same-user-mcp", "same-A-mcp", "same-B-mcp"} {
		result[label] = true
	}
	return result
}
func (f *clineGate) admit(label, class string) {
	f.t.Helper()
	if f.used[label] || clineGateClass(label) != class || len(f.used) >= 60 {
		f.t.Fatal("command outside finite table")
	}
}
func (f *clineGate) gitRoots() {
	for _, item := range []struct{ label, path string }{{"git-user", f.user}, {"git-A", f.A}, {"git-B", f.B}} {
		f.admit(item.label, "git")
		args := []string{"-c", "core.hooksPath=/dev/null", "init", "--quiet", "--template=", "--initial-branch=main"}
		r := captureClineGate(f.ctx, "/usr/bin/git", item.path, f.env(""), args, 10*time.Second)
		f.record(item.label, "git", item.path, append([]string{"/usr/bin/git"}, args...), r)
		if r.err != nil || r.exit != 0 || r.overflow || len(r.stderr) != 0 {
			f.t.Fatal("fresh inert Git boundary failed")
		}
		info, err := os.Lstat(filepath.Join(item.path, ".git"))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			f.t.Fatal("exact Git boundary missing")
		}
	}
}
func clineGateBinaryPin(path string) (result error) {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || st.Nlink != 1 || info.Size() != 132511200 || !filepath.IsAbs(path) {
		return fmt.Errorf("native binary type/length/path differs")
	}
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, file.Close()) }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return fmt.Errorf("native binary opened identity differs")
	}
	hash := sha256.New()
	n, err := io.Copy(hash, io.LimitReader(file, 132511201))
	if err != nil {
		return err
	}
	after, ae := file.Stat()
	current, ce := os.Lstat(path)
	if ae != nil || ce != nil || n != 132511200 || !os.SameFile(info, after) || !os.SameFile(info, current) || !after.ModTime().Equal(info.ModTime()) || !current.ModTime().Equal(info.ModTime()) || hex.EncodeToString(hash.Sum(nil)) != clineGatePin {
		return fmt.Errorf("native binary pin/held identity changed")
	}
	return nil
}
func (f *clineGate) begin(stage string) {
	f.current = stage
	f.stages[stage] = map[string]string{"status": "attempting", "reason": "in progress"}
}
func (f *clineGate) cleanup() clineGateCapture {
	info, err := os.Lstat(f.root)
	if err != nil {
		return clineGateCapture{exit: -1, err: err}
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !os.SameFile(info, f.rootIdentity) {
		return clineGateCapture{exit: -1, err: fmt.Errorf("cleanup task root replaced")}
	}
	if !ok || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return clineGateCapture{exit: -1, err: fmt.Errorf("cleanup task identity unavailable")}
	}
	code := strings.Join([]string{
		"import os,stat,sys",
		"root,dev,ino,uid=sys.argv[1:]; parent,name=os.path.split(root)",
		"p=os.open(parent,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW); fd=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=p)",
		"identity=os.fstat(fd)",
		"if (identity.st_dev,identity.st_ino,identity.st_uid)!=(int(dev),int(ino),int(uid)):raise SystemExit('root authority changed')",
		"count=[0]",
		"def clear(fd,depth):",
		" if depth>32:raise RuntimeError('cleanup depth')",
		" os.fchmod(fd,os.fstat(fd).st_mode|0o700)",
		" with os.scandir(fd) as entries:",
		"  for entry in entries:",
		"   count[0]+=1",
		"   if count[0]>8192:raise RuntimeError('cleanup entry bound')",
		"   item=os.stat(entry.name,dir_fd=fd,follow_symlinks=False)",
		"   if item.st_uid!=int(uid):raise RuntimeError('foreign cleanup owner')",
		"   if stat.S_ISDIR(item.st_mode):",
		"    child=os.open(entry.name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)",
		"    try:",
		"     if not os.path.samestat(item,os.fstat(child)):raise RuntimeError('directory changed')",
		"     clear(child,depth+1)",
		"    finally:os.close(child)",
		"    os.rmdir(entry.name,dir_fd=fd)",
		"   else:os.unlink(entry.name,dir_fd=fd)",
		"clear(fd,0)",
		"if not os.path.samestat(identity,os.stat(name,dir_fd=p,follow_symlinks=False)):raise SystemExit('cleanup root replaced')",
		"os.close(fd);os.rmdir(name,dir_fd=p);os.close(p)",
	}, "\n")
	return captureClineGate(context.Background(), "/usr/bin/python3", "/", []string{"PATH=/usr/bin:/bin", "LANG=C.UTF-8"}, []string{"-I", "-c", code, f.root, strconv.FormatUint(st.Dev, 10), strconv.FormatUint(st.Ino, 10), strconv.FormatUint(uint64(st.Uid), 10)}, 4500*time.Millisecond)
}
func (f *clineGate) finish() {
	f.cancel()
	if f.t.Failed() && f.current != "" {
		f.stages[f.current] = map[string]string{"status": "failed", "reason": "see exact bounded command/source evidence"}
	}
	cleanup := f.cleanup()
	f.record("cleanup", "cleanup", "/", nil, cleanup)
	complete := !f.t.Failed() && cleanup.err == nil && cleanup.exit == 0 && !cleanup.overflow
	f.evidence["collection_complete"], f.evidence["cleanup_complete"] = complete, cleanup.err == nil && cleanup.exit == 0 && !cleanup.overflow
	f.evidence["cleanup_error"] = clineGateError(cleanup.err)
	f.evidence["commands"], f.evidence["stages"] = f.commands, f.stages
	f.evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
	hashes := map[string]string{}
	for name, body := range f.files {
		hashes[name] = clineGateSHA(body)
	}
	f.evidence["artifact_sha256"] = hashes
	body, err := json.MarshalIndent(f.evidence, "", "  ")
	if err != nil || len(body) > 1<<20 {
		f.t.Error("bounded evidence summary failed")
		return
	}
	f.keep("cline-project-evidence.json", append(body, '\n'))
	if f.output == "" {
		f.output = f.t.TempDir()
	}
	info, err := os.Lstat(f.output)
	resolved, re := filepath.EvalSymlinks(f.output)
	if err != nil || re != nil || resolved != f.output || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || f.outputIdentity != nil && !os.SameFile(info, f.outputIdentity) {
		f.t.Error("exact evidence directory missing/replaced")
		return
	}
	for name, body := range f.files {
		file, err := os.OpenFile(filepath.Join(f.output, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
		if err != nil {
			f.t.Error(err)
			return
		}
		_, we := file.Write(body)
		ce := file.Close()
		if we != nil || ce != nil {
			f.t.Error("evidence write/close failed")
			return
		}
	}
	if !complete {
		f.t.Error("Cline project gate or exact task cleanup incomplete")
	}
}
