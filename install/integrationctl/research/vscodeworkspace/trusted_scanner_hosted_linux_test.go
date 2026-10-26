//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// This gate runs in the existing genuine hosted contract step BEFORE vendor
// execution. Only nongenuine/local runs skip it; missing hosted privileges fail.
func TestTrustedScannerHostedUIDContract(t *testing.T) {
	for key, want := range map[string]string{"GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"} {
		if os.Getenv(key) != want {
			t.Skip("hosted UID/mount contract not executed in this local environment")
		}
	}
	if !commitPattern.MatchString(os.Getenv("EXPECTED_COMMIT")) {
		t.Fatal("hosted contract exact source admission missing")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	switch os.Getenv("UAP_VSCODE_SCANNER_CONTRACT") {
	case "uid":
		scannerUIDContract(t)
	case "root":
		if os.Getuid() != 0 || os.Geteuid() != 0 {
			t.Fatal("hosted root contract did not receive sudo authority")
		}
		scannerRootContract(t, executable)
	case "":
		if os.Getuid() <= 0 {
			t.Fatal("hosted contract must start as the original ordinary user")
		}
		args := []string{"-n", "--preserve-env=GITHUB_ACTIONS,RUNNER_ENVIRONMENT,AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED,EXPECTED_COMMIT,UAP_VSCODE_SCANNER_CONTRACT", executable, "-test.run=^TestTrustedScannerHostedUIDContract$", "-test.count=1"}
		env := scannerContractEnv("root")
		scannerContractCommand(t, "/usr/bin/sudo", args, env)
	default:
		t.Fatal("unknown hosted scanner contract role")
	}
}

func scannerContractEnv(role string) []string {
	env := make([]string, 0, 7)
	env = append(env, "PATH=/usr/bin:/bin", "LANG=C.UTF-8", "UAP_VSCODE_SCANNER_CONTRACT="+role)
	for _, key := range []string{"GITHUB_ACTIONS", "RUNNER_ENVIRONMENT", "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED", "EXPECTED_COMMIT"} {
		env = append(env, key+"="+os.Getenv(key))
	}
	return env
}

func scannerContractCommand(t *testing.T, binary string, args, env []string) {
	t.Helper()
	if err := trustedSystem(binary); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	out, stderr := &liveBuffer{limit: controlLimit, cancel: cancel}, &liveBuffer{limit: controlLimit, cancel: cancel}
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env, cmd.Dir = env, "/"
	cmd.Stdout, cmd.Stderr, cmd.WaitDelay = out, stderr, 100*time.Millisecond
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatal("hosted inert contract capture/deadline failed", ctx.Err())
	}
	if err != nil {
		t.Fatalf("mandatory hosted inert filesystem contract failed: %v stdout=%q stderr=%q", err, out.bytes(), stderr.bytes())
	}
}

func scannerRootContract(t *testing.T, executable string) {
	uid, gid, err := callerIDs()
	if err != nil {
		t.Fatal(err)
	}
	base, err := os.MkdirTemp("/tmp", "uap-vscode-sealed-")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(base, 0755); err != nil {
		t.Fatal(err)
	}
	c := &controller{work: context.Background(), request: sealedRequest{Base: base, Input: inputRequest{UID: uid, GID: gid, Trusted: &trustedInput{ScannerArchiveSHA256: trustedScannerArchive}}}}
	c.trustedState = &trustedLifecycle{Root: filepath.Join(base, "trusted")}
	defer func() {
		if err := c.closeTrustedScanner(); err != nil {
			t.Error(err)
			return
		}
		if err := cleanSealed(base); err != nil {
			t.Error(err)
		}
		if _, err := os.Lstat(base); !os.IsNotExist(err) {
			t.Error("hosted inert contract root survived cleanup")
		}
	}()
	for _, dir := range []string{c.trustedState.Root, filepath.Join(c.trustedState.Root, "installer-state")} {
		if err := c.trustedDirectory(dir); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(base, "executables"), 0755); err != nil {
		t.Fatal(err)
	}
	// /usr/bin/true is an expressly inert ELF, never a vendor or fake report.
	body, err := readOrdinary("/usr/bin/true", 128<<20)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	c.request.Input.Trusted.ScannerSHA256 = hex.EncodeToString(sum[:])
	if err := os.WriteFile(filepath.Join(base, "executables", "lintai"), body, 0555); err != nil {
		t.Fatal(err)
	}
	if err := c.prepareTrustedScanner(); err != nil {
		t.Fatal("mandatory hosted cache/mount admission failed", err)
	}
	if err := c.trustedDirectory(c.trustedState.Root); err != nil {
		t.Fatal("idempotent controller check failed", err)
	}
	if err := c.trustedWrite(c.scannerCache.binary, body, 0555); err == nil {
		t.Fatal("controller allowed a sealed scanner rewrite")
	}
	env := append(scannerContractEnv("uid"), "UAP_VSCODE_SCANNER_BASE="+base, "UAP_VSCODE_SCANNER_UID="+strconv.Itoa(uid), "UAP_VSCODE_SCANNER_GID="+strconv.Itoa(gid))
	args := []string{"--reuid=" + strconv.Itoa(uid), "--regid=" + strconv.Itoa(gid), "--clear-groups", "--bounding-set=-all", "--inh-caps=-all", "--ambient-caps=-all", "--no-new-privs", executable, "-test.run=^TestTrustedScannerHostedUIDContract$", "-test.count=1"}
	scannerContractCommand(t, "/usr/bin/setpriv", args, env)
	if err := c.scannerCache.verify(); err != nil {
		t.Fatal("native owner changed pinned cache authority", err)
	}
	want := c.scannerCache.mount
	c.scannerCache.mount++
	if err := c.closeTrustedScanner(); err == nil {
		t.Fatal("foreign mount identity granted unmount authority")
	}
	// A refused release retains the original records, so normal exact cleanup
	// remains possible after restoring only this deliberately altered test field.
	c.scannerCache.mount = want
	if got, err := scannerMountID(c.scannerCache.security); err != nil || got != want {
		t.Fatalf("failed release detached the task mount: %d (%v)", got, err)
	}
}
