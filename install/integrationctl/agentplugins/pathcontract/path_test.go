package pathcontract

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTraversalOrder(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"bin", "data", "real/deep"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin/server"), []byte("inert"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"", "data/..", "bin/../bin/server"} {
		if r := Resolve(root, rel); r.State != Resolved {
			t.Fatalf("%s: %+v", rel, r)
		}
	}
	if err := os.Symlink(filepath.Join(root, "real/deep"), filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	r := Resolve(root, "link/..")
	want, _ := filepath.EvalSymlinks(filepath.Join(root, "real"))
	if r.State != Resolved || r.Path != want {
		t.Fatalf("symlink parent: %+v want %s", r, want)
	}
	for _, rel := range []string{"missing/../bin/server", "bin/absent"} {
		r := Resolve(root, rel)
		if r.State != Missing || r.Path != "" {
			t.Fatalf("missing %s: %+v", rel, r)
		}
	}
	for _, rel := range []string{"../outside", "missing/../../outside"} {
		if r := Resolve(root, rel); r.State != Invalid {
			t.Fatalf("escape %s: %+v", rel, r)
		}
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "outside")); err != nil {
		t.Fatal(err)
	}
	if r := Resolve(root, "outside/missing"); r.State != Invalid {
		t.Fatalf("outside missing: %+v", r)
	}
	if err := os.Symlink("loop", filepath.Join(root, "loop")); err != nil {
		t.Fatal(err)
	}
	if r := Resolve(root, "loop"); r.State != Unavailable || r.Path != "" {
		t.Fatalf("loop: %+v", r)
	}
}
func TestParseCWDPreservesAnchorAndSuffix(t *testing.T) {
	for _, value := range []string{"./", "./data/..", "${PLUGIN_ROOT}/link/..", "${PLUGIN_DATA}/../outside"} {
		p, e := ParseCWD(value)
		if e != nil {
			t.Fatal(e)
		}
		if value == "${PLUGIN_DATA}/../outside" && (p.Anchor != Data || p.Relative != "../outside") {
			t.Fatalf("%+v", p)
		}
	}
}

func TestResolveAcceptsAlternateRootAliasAndRejectsDanglingOutside(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(filepath.Join(root, "inside"), 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink(filepath.Join(alias, "inside"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	if result := Resolve(root, "link"); result.State != Resolved {
		t.Fatalf("same root alias: %+v", result)
	}
	if err := os.Symlink(filepath.Join(base, "absent", "outside"), filepath.Join(root, "dangling")); err != nil {
		t.Fatal(err)
	}
	if result := Resolve(root, "dangling/missing"); result.State != Invalid {
		t.Fatalf("outside missing target: %+v", result)
	}
}

func TestResolveRejectsDanglingOutsideSymlinkChain(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "second")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink("second", filepath.Join(root, "first")); err != nil {
		t.Fatal(err)
	}
	if r := Resolve(root, "first"); r.State != Invalid || r.Path != "" {
		t.Fatalf("outside dangling chain: %+v", r)
	}
}
