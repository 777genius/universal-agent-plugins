package project

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReadRootSpelling(t *testing.T) {
	t.Chdir(t.TempDir())
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".", "demo", "./demo", `.\demo`, `demo\..\demo`, "junction/../demo", "missing/../demo"} {
		want := name
		if runtime.GOOS == "windows" {
			want = cwd + `\` + name
		}
		if got, err := readRoot(name); err != nil || got != want {
			t.Fatalf("%q: got %q, %v; want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"", `C:\demo`, `C:demo`, `C:`, `\demo`, "/demo", `\\server\share`, `\\?\C:\demo`, `\\.\pipe\fixture`, `demo:stream`} {
		if got, err := readRoot(name); err != nil || got != name {
			t.Fatalf("changed namespace %q to %q: %v", name, got, err)
		}
	}
}

func TestRelativeReadRootTraversalOrder(t *testing.T) {
	writableNative(t)
	parent, scratch := t.TempDir(), t.TempDir()
	put(t, parent, "demo/plugin.json", core)
	put(t, parent, "regular", "inert")
	t.Chdir(parent)
	s := Service{Scratch: scratch}
	for _, name := range []string{"demo", "./demo", "demo/../demo"} {
		p, err := s.Read(context.Background(), name)
		if err != nil || p.Facts.Package == nil || p.Input.Identity.Digest == "" {
			t.Fatalf("ordinary relative root %q: %v", name, err)
		}
	}
	for _, name := range []string{"missing/../demo", "regular/../demo"} {
		if _, err := s.Read(context.Background(), name); err == nil {
			t.Fatalf("erased traversal evidence: %q", name)
		}
	}
	t.Chdir(filepath.Join(parent, "demo"))
	if p, err := s.Read(context.Background(), "."); err != nil || p.Facts.Package == nil {
		t.Fatalf("dot root: %v", err)
	}
	empty(t, scratch)
}
