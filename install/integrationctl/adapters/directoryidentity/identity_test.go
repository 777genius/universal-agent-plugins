package directoryidentity

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Regression: stripping leading UUID bytes conflates length-distinct volumes;
// accepting zero/oversized lengths invents identity. No legacy UUID decoder exists.
func TestLinuxUUIDIndependentVectors(t *testing.T) {
	cases := []struct{ name, raw, want string }{
		{"one-byte", "0101000000000000000000000000000000", "01"},
		{"leading-zero-meaningful", "0200010000000000000000000000000000", "0001"},
		{"sixteen-byte", "1000112233445566778899aabbccddeeff", "00112233445566778899aabbccddeeff"},
		{"unused-bytes-do-not-grant", "0100010000000000000000000000000000", ""},
		{"zero-length", "0001000000000000000000000000000000", ""},
		{"seventeen-length", "1101000000000000000000000000000000", ""},
		{"short-record", "0101", ""},
		{"oversized-record", "01010000000000000000000000000000000000", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := hex.DecodeString(c.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeLinuxUUID(raw)
			if c.want == "" {
				if err == nil {
					t.Fatalf("invalid independent vector accepted: %q", got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

// Regression: a C-aligned UUID offset, missing returned bit, or trusting packed
// length past the buffer yields a fabricated volume. No prior Darwin decoder exists.
func TestDarwinPackedIndependentVectors(t *testing.T) {
	cases := []struct {
		name, raw, want string
		unsupported     bool
	}{
		{"packed-uuid", "28000000000000800000048000000000000000000000000000112233445566778899aabbccddeeff", "00112233445566778899aabbccddeeff", false},
		{"missing-bit", "180000000000008000000080000000000000000000000000", "", true},
		{"absent-uuid-oversized-length", "280000000000008000000080000000000000000000000000", "", false},
		{"zero-uuid", "28000000000000800000048000000000000000000000000000000000000000000000000000000000", "", false},
		{"short-packed-uuid", "27000000000000800000048000000000000000000000000000112233445566778899aabbccddeeff", "", false},
		{"length-past-buffer", "29000000000000800000048000000000000000000000000000112233445566778899aabbccddeeff", "", false},
		{"unexpected-common", "28000000010000800000048000000000000000000000000000112233445566778899aabbccddeeff", "", false},
		{"unexpected-volume", "28000000000000800100048000000000000000000000000000112233445566778899aabbccddeeff", "", false},
		{"unexpected-directory", "28000000000000800000048001000000000000000000000000112233445566778899aabbccddeeff", "", false},
		{"truncated-header", "18000000", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			raw, err := hex.DecodeString(c.raw)
			if err != nil {
				t.Fatal(err)
			}
			got, err := decodeDarwinUUID(raw)
			if c.want != "" {
				if err != nil || got != c.want {
					t.Fatalf("got %q, %v", got, err)
				}
				return
			}
			if err == nil || errors.Is(err, ErrUnsupported) != c.unsupported {
				t.Fatalf("got %q, %v; unsupported=%v", got, err, c.unsupported)
			}
		})
	}
}

// Regression: returning a diagnostic DTO slice enables post-construction token
// mutation; neutral journals previously had no immutable structural authority.
func TestNeutralAuthorityImmutableAndZero(t *testing.T) {
	f := Facts{Version: 1, CanonicalRoot: "/profile", Ancestry: []Entry{
		{CanonicalPath: "/", Scheme: "linux-fsuuid-inode-v1", VolumeID: "0001", ObjectID: "1"},
		{CanonicalPath: "/profile", Scheme: "linux-fsuuid-inode-v1", VolumeID: "0001", ObjectID: "2"},
	}}
	a, err := NewAuthority(f)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	f.Ancestry[0].ObjectID = "99"
	copyFacts := a.Facts()
	copyFacts.Ancestry[1].VolumeID = "ff"
	copyFacts.Ancestry = append(copyFacts.Ancestry, Entry{})
	later, err := json.Marshal(a)
	if err != nil || !bytes.Equal(encoded, later) {
		t.Fatalf("mutation changed authority: %s, %v", later, err)
	}
	var roundtrip Authority
	if err := json.Unmarshal(encoded, &roundtrip); err != nil || !a.Equal(roundtrip) {
		t.Fatalf("roundtrip: %v", err)
	}
	var zero Authority
	if !zero.IsZero() || zero.Equal(a) {
		t.Fatal("zero equality")
	}
	if _, err := json.Marshal(zero); err == nil {
		t.Fatal("zero encoded a grant")
	}
	if err := Revalidate(context.Background(), zero); err == nil {
		t.Fatal("zero revalidated")
	}
}

// Regression: cancellation still traverses directories, or a missing profile is
// created. Existing reservation tests cannot exercise this standalone fd API.
func TestCaptureCancellationBootstrapAndBounds(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Capture(ctx, root); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	missing := filepath.Join(root, "absent", "profile")

	expectedMissing := ErrBootstrapRequired
	if platformScheme() == "unsupported" {
		expectedMissing = ErrUnsupported
	}
	if _, err := Capture(context.Background(), missing); !errors.Is(err, expectedMissing) {
		t.Fatalf("missing: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(root, "absent")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("bootstrap created directory: %v", err)
	}
	for _, bad := range []string{"relative", root + "/../profile", root + "\n", "/" + strings.Repeat("a", 4096)} {
		if _, err := Capture(context.Background(), bad); err == nil {
			t.Fatalf("bad input accepted: %q", bad)
		}
	}
	if _, err := ancestryPaths("/" + strings.Repeat("a/", 255) + "a"); err == nil {
		t.Fatal("257 ancestry entries accepted")
	}
}

func fixtureDirectory(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "parent", "profile")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "marker"), []byte("same bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func captureQualified(t *testing.T, root string) (Authority, bool) {
	t.Helper()
	a, err := Capture(context.Background(), root)
	if errors.Is(err, ErrUnsupported) {
		t.Logf("QUALIFICATION=UNSUPPORTED; root=%s; cause=%v; identity-dependent assertions NOT_RUN", root, err)
		if !a.IsZero() {
			t.Fatal("unsupported capture returned authority")
		}
		return Authority{}, false
	}
	if err != nil {
		t.Fatalf("capture failed (not unsupported): %v", err)
	}
	return a, true
}

// Regression: a spelling/content-only proof accepts a new directory at the same
// path. Legacy dirswap tests protect publication receipts, not profile ancestry.
func TestSamePathRecreation(t *testing.T) {
	root := fixtureDirectory(t)
	a, qualified := captureQualified(t, root)
	if !qualified {
		t.Skip("NOT_RUN: actual namespace UUID ioctl unsupported; this is not a replacement PASS")
	}
	if err := os.Rename(root, root+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "marker"), []byte("same bytes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(context.Background(), a); !errors.Is(err, ErrChanged) {
		t.Fatalf("same-path recreation: %v", err)
	}
	t.Log("QUALIFICATION=SUPPORTED; same-byte replacement denied")
}

// Regression: comparing only the leaf inode misses a replaced ancestor with the
// original child moved under it. No existing test carries complete profile ancestry.
func TestAncestorReplacementPreservingChild(t *testing.T) {
	root := fixtureDirectory(t)
	a, qualified := captureQualified(t, root)
	if !qualified {
		t.Skip("NOT_RUN: actual namespace UUID ioctl unsupported; this is not a replacement PASS")
	}
	parent := filepath.Dir(root)
	if err := os.Rename(parent, parent+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(parent+"-old", "profile"), root); err != nil {
		t.Fatal(err)
	}
	fresh, err := Capture(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	oldEntries, newEntries := a.Facts().Ancestry, fresh.Facts().Ancestry
	if oldEntries[len(oldEntries)-1] != newEntries[len(newEntries)-1] {
		t.Fatal("fixture failed to preserve child")
	}
	if err := Revalidate(context.Background(), a); !errors.Is(err, ErrChanged) {
		t.Fatalf("ancestor substitution: %v", err)
	}
	t.Log("QUALIFICATION=SUPPORTED; substituted ancestor denied with unchanged child")
}

// Regression: persistence uses live-process descriptors or boot/PID tokens; a
// fresh process must revalidate stored ordinary facts. There was no profile token.
func TestPersistedProcess(t *testing.T) {
	if stored := os.Getenv("P1_TEST_AUTHORITY"); stored != "" {
		runPersistedChild(t, stored)
		return
	}
	root := fixtureDirectory(t)
	a, qualified := captureQualified(t, root)
	var b []byte
	var err error
	if qualified {
		b, err = json.Marshal(a)
	} else {
		b, err = json.Marshal(struct {
			Root    string `json:"root"`
			Outcome string `json:"outcome"`
		}{root, "unsupported"})
	}
	if err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(filepath.Dir(root), "authority.json")
	if err := os.WriteFile(token, b, 0o600); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestPersistedProcess$", "-test.v")
	cmd.Env = []string{"P1_TEST_AUTHORITY=" + token, "HOME=" + filepath.Dir(root), "TMPDIR=" + filepath.Dir(root)}
	out, err := cmd.CombinedOutput()
	marker := "FRESH_PROCESS_REVALIDATED"
	if !qualified {
		marker = "FRESH_PROCESS_UNSUPPORTED_NO_GRANT"
	}
	if err != nil || !bytes.Contains(out, []byte(marker)) {
		t.Fatalf("subprocess: %v\n%s", err, out)
	}
	t.Logf("persisted=%s\n%s", b, out)
}

func runPersistedChild(t *testing.T, stored string) {
	t.Helper()
	b, err := os.ReadFile(stored)
	if err != nil {
		t.Fatal(err)
	}
	var outcome struct {
		Root    string `json:"root"`
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(b, &outcome); err != nil {
		t.Fatal(err)
	}
	if outcome.Outcome == "unsupported" {
		a, err := Capture(context.Background(), outcome.Root)
		if !errors.Is(err, ErrUnsupported) || !a.IsZero() {
			t.Fatalf("fresh process unsupported: %v", err)
		}
		t.Logf("FRESH_PROCESS_UNSUPPORTED_NO_GRANT: %v", err)
		return
	}
	var a Authority
	if err := json.Unmarshal(b, &a); err != nil {
		t.Fatal(err)
	}
	if err := Revalidate(context.Background(), a); err != nil {
		t.Fatalf("fresh process: %v", err)
	}
	t.Log("FRESH_PROCESS_REVALIDATED")
}
