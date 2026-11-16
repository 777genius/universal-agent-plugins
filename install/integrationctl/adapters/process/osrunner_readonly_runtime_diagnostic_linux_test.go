//go:build linux

package process

import (
	"errors"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestReadonlyRuntimeUnsafeDiagnosticRetainsRefusals(t *testing.T) {
	for _, test := range []struct {
		name   string
		mode   uint32
		links  uint64
		bitmap string
	}{
		{"special", unix.S_IFIFO | 0600, 1, "0x01"},
		{"writable", unix.S_IFREG | 0664, 1, "0x02"},
		{"linked-symlink", unix.S_IFLNK | 0777, 2, "0x04"},
		{"special-writable", unix.S_IFSOCK | 0666, 1, "0x03"},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := readonlyObjectFacts{Inode: 1, Links: test.links, Mode: test.mode}
			_, err := readonlyRuntimeFacts(facts, 0, 0)
			if !errors.Is(err, errReadonlyRuntimeUnsafe) || !strings.Contains(err.Error(), "unsafe_predicates="+test.bitmap) {
				t.Fatalf("missing exact refusal class: %v", err)
			}
			projected := readonlyRuntimeMemberError("/usr/test-fixture/unsafe", err)
			if !errors.Is(projected, errReadonlyRuntimeUnsafe) || !strings.Contains(projected.Error(), `member="test-fixture/unsafe"`) || len(projected.Error()) > readonlyRuntimeDiagnosticLimit {
				t.Fatalf("missing bounded member authority: %v", projected)
			}
		})
	}
	for _, mode := range []uint32{unix.S_IFREG | 0644, unix.S_IFDIR | 0755, unix.S_IFLNK | 0777} {
		if _, err := readonlyRuntimeFacts(readonlyObjectFacts{Inode: 1, Links: 1, Mode: mode}, 0, 0); err != nil {
			t.Fatalf("valid metadata refused: %v", err)
		}
	}
}
