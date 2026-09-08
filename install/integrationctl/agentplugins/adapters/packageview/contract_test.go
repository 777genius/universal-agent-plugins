package packageview

import (
	"context"
	"errors"
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
}
