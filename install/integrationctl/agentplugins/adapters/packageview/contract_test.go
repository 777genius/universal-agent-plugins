package packageview

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func TestCapturedFramingExistingGolden(t *testing.T) {
	entries := []packagedigest.CapturedEntry{
		{Path: "link", Kind: "symlink", Target: "a"},
		{Path: "bin/x", Kind: "file", Executable: true, Content: []byte("x")},
		{Path: "a-prefix", Kind: "file", Content: []byte("prefix")},
		{Path: "bin", Kind: "directory"},
		{Path: "a", Kind: "file"},
	}
	got, e := packagedigest.DigestCaptured(context.Background(), entries)
	if e != nil || got != "sha256:3f80c5a7d3a7e9a4446f2e49f6de414645643bd5d2ac63192df2686035b59f33" {
		t.Fatalf("existing framing golden: %s, %v", got, e)
	}
	// Pure framing has no path capability and does not reorder caller-owned input.
	if entries[0].Path != "link" {
		t.Fatal("mutated caller input")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e = packagedigest.DigestCaptured(ctx, entries)
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	for _, bad := range [][]packagedigest.CapturedEntry{
		{{Path: "a/b", Kind: "file"}},
		{{Path: "a", Kind: "file"}, {Path: "a/b", Kind: "file"}},
		{{Path: "A", Kind: "file"}, {Path: "a", Kind: "file"}},
		{{Path: "a", Kind: "symlink", Target: "missing"}},
		{{Path: "CON.txt", Kind: "file"}},
		{{Path: "a", Kind: "file", Content: []byte("version https://git-lfs.github.com/spec/v1\n")}},
		{{Path: "a", Kind: "device"}},
	} {
		if _, e := packagedigest.DigestCaptured(context.Background(), bad); !errors.Is(e, packagedigest.ErrCapturedPolicy) {
			t.Fatalf("accepted invalid representation %+v", bad)
		}
	}
}

func TestLimitsAndSafeErrors(t *testing.T) {
	if _, e := (Limits{Entries: -1}).bounded(); e == nil {
		t.Fatal("negative limit accepted")
	}
	if _, e := (Limits{FileBytes: 65 << 20}).bounded(); e == nil {
		t.Fatal("unbounded override accepted")
	}
	v, e := (Limits{}).bounded()
	if e != nil || v.Entries != 10000 || v.Depth != 64 || v.DocumentBytes != 16<<20 {
		t.Fatal("profile drift")
	}
	var l Lease
	if e := l.Close(); e != nil {
		t.Fatal(e)
	}
	if e := l.Close(); e != nil {
		t.Fatal("repeated zero-value Close", e)
	}
}

// Remapping must retain the observed typed error, including wrapped cancellation,
// and must not invent cancellation in place of an already observed edit.
func TestAcquisitionErrorPreservesCause(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		original := &Error{Code: "canceled", cancellation: cause}
		got := acquisitionError(original, "source_changed")
		if got != original || !errors.Is(got, cause) {
			t.Fatalf("lost cancellation: %v", got)
		}
	}
	for _, code := range []string{"source_changed", "path_limit", "filesystem_unavailable"} {
		original := fail(code)
		if got := acquisitionError(original, "scratch_unavailable"); got != original {
			t.Fatalf("lost observed %s: %v", code, got)
		}
	}
	var safe *Error
	if got := acquisitionError(errors.New("private host detail"), "source_changed"); !errors.As(got, &safe) || safe.Code != "source_changed" {
		t.Fatalf("lost sanitized fallback: %v", got)
	}
}

// GeneratedStaging is the only seam that can relax Darwin's read-only-mount
// requirement. It must be impossible to build one without an already open,
// live directory handle: a caller holding only a path string (every ordinary
// validate/inspect/test request) can never obtain one, on any platform.
func TestGeneratedStagingRequiresLiveHandle(t *testing.T) {
	if _, e := NewGeneratedStaging(nil); e == nil {
		t.Fatal("nil handle accepted")
	}
	var zero GeneratedStaging
	if zero.present() {
		t.Fatal("zero value reports present")
	}
	if zero.matches(nil) {
		t.Fatal("zero value matches nil")
	}
	dir, e := os.OpenRoot(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	g, e := NewGeneratedStaging(dir)
	if e != nil {
		t.Fatal(e)
	}
	if !g.present() {
		t.Fatal("live handle did not produce a present proof")
	}
	info, e := dir.Stat(".")
	if e != nil {
		t.Fatal(e)
	}
	if !g.matches(info) {
		t.Fatal("proof does not match the directory it was built from")
	}
	other, e := os.OpenRoot(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	otherInfo, e := other.Stat(".")
	if e != nil {
		t.Fatal(e)
	}
	if g.matches(otherInfo) {
		t.Fatal("proof matched an unrelated directory")
	}
	if e := dir.Close(); e != nil {
		t.Fatal(e)
	}
	// Close does not invalidate the already captured identity snapshot.
	if !g.present() || !g.matches(info) {
		t.Fatal("proof invalidated by closing the source handle")
	}
}
