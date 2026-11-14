package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestSkillSourceRecheckDoesNotOpenFIFOOrLegacyAlias(t *testing.T) {
	for _, kind := range []string{"fifo", "hardlink"} {
		t.Run(kind, func(t *testing.T) {
			root, p, gate := skillFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			ops := applyOps{rename: renameExclusive, write: func(ctx context.Context, r *os.Root, files []File) error {
				if e := writeTree(ctx, r, files); e != nil {
					return e
				}
				core := filepath.Join(root, "plugin.json")
				if e := os.Remove(core); e != nil {
					return e
				}
				if kind == "fifo" {
					return syscall.Mkfifo(core, 0600)
				}
				if e := os.Mkdir(filepath.Join(root, "plugin"), 0700); e != nil {
					return e
				}
				canonical := filepath.Join(root, "plugin/plugin.yaml")
				if e := os.WriteFile(canonical, []byte("opaque legacy"), 0000); e != nil {
					return e
				}
				return os.Link(canonical, core)
			}}
			r, e := applySkill(ctx, p, root, gate, sharedSkillValidation("new-skill"), ops)
			if e == nil || r.Committed || ctx.Err() != nil {
				t.Fatalf("unsafe or blocking recheck: %v %+v", e, r)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Name() != "plugin.json" && entry.Name() != "plugin" {
					t.Fatalf("unexpected residue: %s", entry.Name())
				}
			}
		})
	}
}
