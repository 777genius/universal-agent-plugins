//go:build windows && (amd64 || arm64)

package packageview

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A bounded native diagnostic separates trusted alias resolution, protected
// acquisition and GUID identity without reading source data. Only fresh fixture
// paths are logged. Failure at any required stage remains a failed test.
func TestWindowsScratchAliasResolutionStages(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "target")
	if e := os.Mkdir(target, 0700); e != nil {
		t.Fatal(e)
	}
	alias := filepath.Join(parent, "junction")
	nativeJunction(t, alias, target)
	chain := filepath.Join(parent, "chain")
	nativeJunction(t, chain, alias)
	for _, path := range []string{alias, chain} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			info, e := os.Lstat(path)
			if e != nil {
				t.Fatal("Lstat stage:", e)
			}
			link, e := os.Readlink(path)
			t.Logf("Lstat mode=%v; Readlink=%q err=%v", info.Mode(), link, e)
			if e != nil {
				t.Fatal("Readlink stage:", e)
			}
			eval, evalErr := filepath.EvalSymlinks(path)
			t.Logf("stdlib EvalSymlinks=%q err=%v", eval, evalErr)
			before := winRecordCount()
			if evalErr == nil {
				old, oldErr := openSource(eval, GeneratedStaging{})
				t.Logf("stdlib result openSource err=%v", oldErr)
				if old != nil {
					if e := old.close(); e != nil {
						t.Fatal(e)
					}
				}
			}
			resolved, e := winResolveScratch(path)
			t.Logf("trusted scratch resolution=%q err=%v", resolved, e)
			if e != nil {
				t.Fatal("scratch resolution stage:", e)
			}
			var stack [64]byte
			t.Logf("resolved acquisition pid=%d goroutine=%s", os.Getpid(), strings.Fields(string(stack[:runtime.Stack(stack[:], false)]))[1])
			scratch, e := openTrustedScratchWithMetadataStage(resolved, nil)
			t.Logf("resolved trusted scratch acquisition err=%v", e)
			if e != nil {
				t.Fatal("protected acquisition stage:", e)
			}
			defer scratch.close()
			source, e := openSource(target, GeneratedStaging{})
			if e != nil {
				t.Fatal("target acquisition stage:", e)
			}
			defer source.close()
			sv, sp, se := winDirectoryIdentity(source.anchor)
			tv, tp, te := winDirectoryIdentity(scratch.anchor)
			t.Logf("GUID identity source=(%q,%q,%v) scratch=(%q,%q,%v)", sv, sp, se, tv, tp, te)
			if se != nil || te != nil || !strings.EqualFold(sv, tv) || !strings.EqualFold(sp, tp) {
				t.Fatal("physical identity stage did not retain the actual alias target")
			}
			var safe *Error
			if e := winScratchDisjoint(source.anchor, scratch.anchor); !errors.As(e, &safe) || safe.Code != "scratch_overlaps_source" {
				t.Fatal("overlap stage:", e)
			}
			if e := scratch.close(); e != nil {
				t.Fatal(e)
			}
			if e := source.close(); e != nil {
				t.Fatal(e)
			}
			if winRecordCount() != before {
				t.Fatal("stage diagnostic leaked ancestry")
			}
		})
	}
}
