package packagedigest

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCapturedLFSScanLinesParity(t *testing.T) {
	const prefix = "version https://git-lfs.github.com/spec/v1"
	for _, tc := range []struct {
		name, body string
		reject     bool
	}{
		{"LF", prefix + "\n", true}, {"CRLF", prefix + "\r\n", true},
		{"doubleCRLF", prefix + "\r\r\n", true}, {"doubleCREOF", prefix + "\r\r", true},
		{"singleCREOF", prefix + "\r", true}, {"bareEOF", prefix, true}, {"39bytes", prefix[:39], false},
		{"tripleCRLF", prefix + "\r\r\r\n", false}, {"tripleCREOF", prefix + "\r\r\r", false},
		{"laterLine", "other\n" + prefix + "\n", false},
		{"1024", prefix + "\n" + strings.Repeat("x", 1024-len(prefix)-1), true},
		{"1025", prefix + "\n" + strings.Repeat("x", 1025-len(prefix)-1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			write(t, filepath.Join(root, "pointer"), []byte(tc.body), 0600)
			digest, err := DigestCaptured(context.Background(), []CapturedEntry{{Path: "pointer", Kind: "file", Content: []byte(tc.body)}})
			snap, oldErr := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
			if oldErr == nil {
				t.Cleanup(func() {
					if err := Remove(snap); err != nil {
						t.Error(err)
					}
				})
			}
			if tc.reject {
				if !errors.Is(err, ErrCapturedPolicy) || digest != "" || oldErr == nil || !strings.Contains(oldErr.Error(), "Git LFS pointer is unsupported") {
					t.Fatalf("captured %q %v; installer %v", digest, err, oldErr)
				}
			} else if err != nil || oldErr != nil || digest != snap.TreeDigest {
				t.Fatalf("captured %q %v; installer %+v %v", digest, err, snap, oldErr)
			}
		})
	}
}
