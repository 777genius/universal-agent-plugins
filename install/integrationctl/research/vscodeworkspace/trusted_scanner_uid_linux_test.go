//go:build linux

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/securityscan"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type scannerNoNetwork struct{ calls int }

func (s *scannerNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	s.calls++
	return nil, errors.New("inert contract prohibits network")
}

func scannerUIDContract(t *testing.T) {
	uid, ue := strconv.Atoi(os.Getenv("UAP_VSCODE_SCANNER_UID"))
	gid, ge := strconv.Atoi(os.Getenv("UAP_VSCODE_SCANNER_GID"))
	if ue != nil || ge != nil || uid <= 0 || gid <= 0 || os.Getuid() != uid || os.Geteuid() != uid || os.Getgid() != gid || os.Getegid() != gid {
		t.Fatal("contract did not drop to exact original UID/GID")
	}
	groups, err := os.Getgroups()
	if err != nil || len(groups) != 0 {
		t.Fatal("contract supplementary groups remain")
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"} {
		if !bytes.Contains(status, []byte(key+":\t0000000000000000\n")) {
			t.Fatal("contract capability drop differs", key)
		}
	}
	if !bytes.Contains(status, []byte("NoNewPrivs:\t1\n")) {
		t.Fatal("contract NNP missing")
	}
	base := os.Getenv("UAP_VSCODE_SCANNER_BASE")
	if !sealedRootOperand(base) {
		t.Fatal("contract root operand differs")
	}
	root, state := filepath.Join(base, "trusted"), filepath.Join(base, "trusted", "installer-state")
	security := filepath.Join(state, "security")
	lintai := filepath.Join(security, "lintai")
	version, platform := filepath.Join(lintai, "0.1.3"), filepath.Join(lintai, "0.1.3", "linux-amd64-musl")
	binary := filepath.Join(platform, "lintai")
	assessments := filepath.Join(security, "assessments")
	external, err := os.MkdirTemp("/tmp", "uap-vscode-scanner-decoy-")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(external); err != nil {
			t.Error(err)
		}
	}()
	stateDecoy, assessmentDecoy := filepath.Join(state, "inert-decoy"), filepath.Join(assessments, "inert-decoy")
	for _, path := range []string{stateDecoy, assessmentDecoy} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	denied := func(label string, err error) {
		t.Helper()
		if !errors.Is(err, unix.EACCES) && !errors.Is(err, unix.EPERM) && !errors.Is(err, unix.EBUSY) {
			t.Fatalf("unprivileged %s was not denied before mutation: %v", label, err)
		}
	}
	for _, path := range []string{base, root, state, security, lintai, version, platform, binary, assessments} {
		denied("rename "+path, os.Rename(path, path+"-must-not-exist"))
		decoy := external
		switch path {
		case security:
			decoy = stateDecoy
		case lintai, version, platform, binary:
			decoy = assessmentDecoy
		case assessments:
			decoy = lintai
		}
		if err := scannerExchangeGeometry(path, decoy); err != nil {
			t.Fatal("denial contract exchange operands are not valid", err)
		}
		denied("exchange "+path, unix.Renameat2(unix.AT_FDCWD, path, unix.AT_FDCWD, decoy, unix.RENAME_EXCHANGE))
		if path != state && path != assessments {
			denied("chmod "+path, os.Chmod(path, 0700))
			denied("unlink/rmdir "+path, os.Remove(path))
		}
	}
	file, openErr := os.OpenFile(binary, os.O_WRONLY, 0)
	if file != nil {
		_ = file.Close()
	}
	denied("scanner write", openErr)
	denied("scanner unmount", unix.Unmount(security, unix.UMOUNT_NOFOLLOW))
	for _, dir := range []string{state, assessments} {
		if err := os.Chmod(dir, 0700); err != nil {
			t.Fatal("native owner cannot apply ordinary private mode", err)
		}
	}
	ctx := context.Background()
	unlock, err := (processlock.Lock{Path: filepath.Join(state, "mutation.lock")}).Acquire(ctx)
	if err != nil {
		t.Fatal("real mutation lock cannot use private state", err)
	}
	defer func() {
		if err := unlock(); err != nil {
			t.Error(err)
		}
	}()
	store := statev2.Store{Path: filepath.Join(state, "state-v2.json")}
	if err := store.Save(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}); err != nil {
		t.Fatal("real state writer cannot use private state", err)
	}
	cache := securityscan.FileCache{Root: assessments}
	for range 2 {
		if err := cache.Store(strings.Repeat("a", 64), domain.SecurityAssessment{}); err != nil {
			t.Fatal("real assessment temporary/atomic writer failed", err)
		}
	}
	temporary, err := os.CreateTemp(state, "inert-state-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if err := temporary.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(temporary.Name()); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []string{"a", "b"} {
		file, err := os.OpenFile(filepath.Join(root, scope+"-events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal("precreated event log append refused", err)
		}
		_, err = file.WriteString("expressly inert contract event\n")
		if closeErr := file.Close(); err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
	}
	transport := &scannerNoNetwork{}
	client := &http.Client{Transport: transport}
	t.Setenv("PATH", filepath.Join(base, "executables")+":/usr/bin:/bin")
	_, err = (securityscan.ReleaseScanner{Root: filepath.Join(state, "missing-cache"), HTTPClient: client, GOOS: "linux", GOARCH: "amd64"}).Scan(ctx, state)
	if err == nil || transport.calls != 1 {
		t.Fatal("PATH alone unexpectedly selected scanner cache", err, transport.calls)
	}
	output, err := (securityscan.ReleaseScanner{Root: lintai, HTTPClient: client, GOOS: "linux", GOARCH: "amd64"}).Scan(ctx, state)
	if err != nil || len(output) != 0 || transport.calls != 1 {
		t.Fatal("real cache route did not run only inert ELF without network", err, transport.calls)
	}
}

// RENAME_EXCHANGE rejects ancestor pairs or differing parent mounts before
// permission checks. Such errors cannot demonstrate the intended protection.
func scannerExchangeGeometry(source, target string) error {
	if !filepath.IsAbs(source) || filepath.Clean(source) != source || !filepath.IsAbs(target) || filepath.Clean(target) != target || source == target || trustedContained(source, target) || trustedContained(target, source) {
		return fmt.Errorf("exchange operands must be canonical and disjoint")
	}
	for _, path := range []string{source, target} {
		parent, err := trustedParent(path)
		if err != nil {
			return err
		}
		if err := parent.Close(); err != nil {
			return err
		}
		info, err := anchoredInfo(path)
		if err != nil || (!info.IsDir() && !regular(info, 128<<20)) {
			return fmt.Errorf("ordinary exchange operand required")
		}
	}
	left, err := scannerMountID(filepath.Dir(source))
	right, rightErr := scannerMountID(filepath.Dir(target))
	if err != nil || rightErr != nil || left != right {
		return fmt.Errorf("exchange operand parents must share one mount")
	}
	return nil
}
