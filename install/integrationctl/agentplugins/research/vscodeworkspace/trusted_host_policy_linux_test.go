//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

const inertStockPolicy = `# inert public stock syntax
abi <abi/4.0>,
include <tunables/global>
profile vscode /usr/share/code{/bin,}/code flags=(unconfined) {
 userns,
 include if exists <local/code>
}
`

func TestHostPolicyClosedGrammar(t *testing.T) {
	if !stockPolicyGrammar([]byte(inertStockPolicy)) {
		t.Fatal("known syntax refused")
	}
	for _, body := range []string{
		strings.ReplaceAll(inertStockPolicy, "userns,", "userns,\n capability sys_admin,"),
		strings.ReplaceAll(inertStockPolicy, "<local/code>", "<local/private>"),
		strings.ReplaceAll(inertStockPolicy, "profile vscode", "profile other"),
		strings.ReplaceAll(inertStockPolicy, "(unconfined)", "(complain)"),
		inertStockPolicy + "#include <private>\n", inertStockPolicy + "# include <private>\n",
		inertStockPolicy + "userns,\n", strings.ReplaceAll(inertStockPolicy, "userns,", ""),
	} {
		if stockPolicyGrammar([]byte(body)) {
			t.Fatal("unknown policy admitted")
		}
	}
}
func TestHostPolicyLoadedMembershipIsExactBoundedAndPrivate(t *testing.T) {
	for _, mode := range []string{"unconfined", "enforce", "complain", "kill"} {
		got, err := loadedVSCodeMode([]byte("private-other (enforce)\nvscode (" + mode + ")\nvscode-other (enforce)\n"))
		if err != nil || got != mode {
			t.Fatal(got, err)
		}
	}
	got, err := loadedVSCodeMode([]byte("vscode-other (unconfined)\nprivate-other (enforce)\n"))
	if err != nil || got != "absent" {
		t.Fatal("prefix collision", got, err)
	}
	for _, body := range []string{"vscode (unknown)\n", "vscode (unconfined)\nvscode (enforce)\n", "vscode\n", strings.Repeat("x", 4097), strings.Repeat("x\n", (256<<10)/2+1)} {
		if _, err := loadedVSCodeMode([]byte(body)); err == nil {
			t.Fatal("unknown/oversized membership admitted")
		}
	}
}
func TestHostPolicySysctlDecimalBounds(t *testing.T) {
	for _, body := range []string{"0\n", "1\n"} {
		if _, err := policySysctl([]byte(body), 1); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := policySysctl([]byte("2147483647\n"), 2147483647); err != nil || n != 2147483647 {
		t.Fatal(n, err)
	}
	for _, body := range []string{"2\n", "-1\n", "01\n", " 1\n", "1\n\n", "true", strings.Repeat("1", 33)} {
		if _, err := policySysctl([]byte(body), 1); err == nil {
			t.Fatal("bad sysctl admitted")
		}
	}
}
func TestHostPolicyOrdinaryCaptureAndNofollowRace(t *testing.T) {
	root := t.TempDir()
	wanted := filepath.Join(root, "wanted")
	foreign := filepath.Join(root, "foreign")
	for _, path := range []string{wanted, foreign} {
		if err := os.WriteFile(path, []byte("same-size"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	uid := uint32(os.Getuid())
	body, file, _, err := policyRead(wanted, 32, false, uid)
	if err != nil || !bytes.Equal(body, []byte("same-size")) {
		t.Fatal(err)
	}
	_ = file.Close()
	for _, kind := range []string{"foreign", "symlink", "fifo", "same-size-change"} {
		t.Run(kind, func(t *testing.T) {
			if err := os.Remove(wanted); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(wanted, []byte("same-size"), 0600); err != nil {
				t.Fatal(err)
			}
			open := func(path string, flags int, mode uint32) (int, error) {
				if flags&unix.O_NONBLOCK == 0 || flags&unix.O_NOFOLLOW == 0 || flags&unix.O_CLOEXEC == 0 {
					t.Fatal("unsafe open flags")
				}
				switch kind {
				case "foreign":
					return unix.Open(foreign, flags, mode)
				case "symlink":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(foreign, path); err != nil {
						t.Fatal(err)
					}
				case "fifo":
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := unix.Mkfifo(path, 0600); err != nil {
						t.Fatal(err)
					}
				case "same-size-change":
					if err := os.WriteFile(path, []byte("new-bytes"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				return unix.Open(path, flags, mode)
			}
			start := time.Now()
			if _, file, _, err := policyReadUsing(wanted, 32, false, uid, open); err == nil {
				_ = file.Close()
				t.Fatal("changed opened object admitted")
			}
			if time.Since(start) > time.Second {
				t.Fatal("nonblocking guard stalled")
			}
		})
	}
}
func TestHostPolicyZeroSizeVirtualIsStillBounded(t *testing.T) {
	body, file, _, err := policyRead("/proc/self/status", 64<<10, true, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	if len(body) == 0 {
		t.Fatal("virtual body missing")
	}
	if _, file, _, err := policyRead("/proc/self/status", 1, true, uint32(os.Getuid())); err == nil {
		_ = file.Close()
		t.Fatal("virtual overflow admitted")
	}
	if _, file, _, err := policyRead("/proc/self/status", 64<<10, false, uint32(os.Getuid())); err == nil {
		_ = file.Close()
		t.Fatal("ordinary size equality weakened")
	}
}
func TestHostPolicyRefusalPreservesOriginalNativeFailure(t *testing.T) {
	native := errors.New("inert native startup failed")
	for _, changed := range []bool{false, true} {
		c := testController(t, nil)
		before := emptyPolicySnapshot()
		after := emptyPolicySnapshot()
		if changed {
			after.Grammar = "refused"
		}
		m := &hostPolicyMeasurement{before: before, reason: "file-capture-refused"}
		err := m.finish(c, after, native)
		if !errors.Is(err, native) {
			t.Fatal("native failure lost", err)
		}
		var receipt hostPolicyReceipt
		if err := json.Unmarshal(c.files[hostPolicyArtifact], &receipt); err != nil {
			t.Fatal(err)
		}
		if receipt.Status != "refused" || receipt.NativeQualified || receipt.LoadedEqualsDisk {
			t.Fatal("measurement promoted native outcome", receipt)
		}
		if bytes.Contains(c.files[hostPolicyArtifact], []byte(native.Error())) {
			t.Fatal("free-form native failure copied")
		}
	}
}
func TestHostPolicyHeldSameSizeReplacementRefuses(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "held")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	_, file, id, err := policyRead(path, 32, false, uint32(os.Getuid()))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("replaced"), 0600); err != nil {
		t.Fatal(err)
	}
	m := &hostPolicyMeasurement{before: emptyPolicySnapshot(), reason: "none", held: []policyHeld{{file, path, id}}}
	c := testController(t, nil)
	if err := m.finish(c, emptyPolicySnapshot(), nil); err == nil || !strings.Contains(err.Error(), "held-identity-changed") {
		t.Fatal("same-size replacement admitted", err)
	}
}
