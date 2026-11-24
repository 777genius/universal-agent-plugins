//go:build linux

package process

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func readonlyRuntimeGroupCensus(t *testing.T) (readonlyRuntimeCensus, [4]readonlyRuntimeRoot) {
	t.Helper()
	census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias)}
	roots := [4]readonlyRuntimeRoot{}
	for i, visible := range readonlyRuntimePaths() {
		path := "/usr"
		if i != 0 {
			path = filepath.Join(path, filepath.Base(visible))
		}
		facts := readonlyObjectFacts{Device: 66305, Inode: uint64(100 + i), Links: 2, Mode: unix.S_IFDIR | 0755}
		roots[i] = readonlyRuntimeRoot{visible: visible, object: readonlyOriginalObject{path: path, facts: facts, directory: true}}
		if err := census.record(path, facts, ""); err != nil {
			t.Fatal("inert original root capture refused", err)
		}
	}
	return census, roots
}

func readonlyRuntimeObservedGroupFacts(group uint32) readonlyObjectFacts {
	// Match the known diagnostic fields. The original timestamp seconds were not
	// archived; their zero values here are inert and are not native evidence.
	facts := readonlyObjectFacts{Device: 66305, Inode: 2047, Mode: unix.S_IFREG | 02755,
		Links: 1, Size: 72184, GID: group}
	facts.Ctime.Nsec = 273560716
	return facts
}

func TestReadonlyRuntimeMembersRetainOriginalGroups(t *testing.T) {
	for _, group := range []uint32{42, 1234, 65534} {
		for _, kind := range []string{"file", "directory", "link"} {
			t.Run(strconv.FormatUint(uint64(group), 10)+"/"+kind, func(t *testing.T) {
				census, roots := readonlyRuntimeGroupCensus(t)
				facts, link := readonlyRuntimeObservedGroupFacts(group), ""
				switch kind {
				case "directory":
					facts.Mode, facts.Size = unix.S_IFDIR|02755, 4096
				case "link":
					facts.Mode, link = unix.S_IFLNK|0777, "../lib/original"
					facts.Size = int64(len(link))
				}
				if err := census.record("/usr/bin/original", facts, link); err != nil {
					t.Fatal("root-owned original member group was refused", err)
				}
				if got := census.rows["/usr/bin/original"]; got.facts != facts || got.link != link {
					t.Fatal("parent commitment erased original group or metadata", got, facts)
				}
				if err := errors.Join(census.requireRoots(t.Context(), roots), census.requireAliases(t.Context())); err != nil {
					t.Fatal("original member lost fixed root or hardlink closure", err)
				}
				if _, err := readonlyRuntimeFacts(facts, 0, 0); !errors.Is(err, errReadonlyRuntimeMetadata) ||
					!strings.Contains(err.Error(), "predicates=0x10") {
					t.Fatal("explicit fixed-group guard was relaxed", err)
				}
			})
		}
	}
}

func TestReadonlyRuntimeGroupOnlyChangeRemainsInOriginalProof(t *testing.T) {
	first, _ := readonlyRuntimeGroupCensus(t)
	facts := readonlyRuntimeObservedGroupFacts(42)
	if err := first.record("/usr/bin/original", facts, ""); err != nil {
		t.Fatal(err)
	}
	original, err := digestReadonlyRuntime(t.Context(), first.rows)
	if err != nil {
		t.Fatal(err)
	}
	view := readonlyPreparedView{policy: readonlyPolicy{RuntimeProof: original}}
	later, _ := readonlyRuntimeGroupCensus(t)
	facts.GID++ // No timestamp, mode, path or other metadata difference can mask this control.
	if err := later.record("/usr/bin/original", facts, ""); err != nil {
		t.Fatal(err)
	}
	changed, err := digestReadonlyRuntime(t.Context(), later.rows)
	if err != nil || original.Count != 5 || changed.Count != original.Count {
		t.Fatal("original bounded census membership changed", original, changed, err)
	}
	if changed == view.policy.RuntimeProof {
		t.Fatal("GID-only change matched the first retained runtime proof")
	}
	if first.rows["/usr/bin/original"].facts.GID != 42 {
		t.Fatal("later capture changed the original census image")
	}
}

func TestReadonlyRuntimeFixedRootRejectsGroupDespiteMatchingCapturedRow(t *testing.T) {
	census, roots := readonlyRuntimeGroupCensus(t)
	roots[0].object.facts.GID = 42
	row := census.rows["/usr"]
	row.facts.GID = 42
	census.rows["/usr"] = row
	if census.rows["/usr"].facts != roots[0].object.facts {
		t.Fatal("strict root control lacks matching original row")
	}
	err := census.requireRoots(t.Context(), roots)
	if !errors.Is(err, errReadonlyRuntimeMetadata) || !strings.Contains(err.Error(), "predicates=0x10") ||
		!strings.Contains(err.Error(), "readonly runtime visible root is outside the captured closure") {
		t.Fatal("fixed root adopted its captured nonzero group", err)
	}
}

func TestReadonlyRuntimeChildTraversalRetainsOriginalGroupAndBorrow(t *testing.T) {
	base, original, census, _ := readonlyRuntimeFixture(t)
	path := filepath.Join(base, "bin", "native")
	if err := os.WriteFile(path, []byte("original group fixture body"), 0600); err != nil {
		t.Fatal(err)
	}
	before, err := readonlyNamedFacts(original, "bin")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	fileFacts, err := readonlyFacts(info)
	if err != nil {
		t.Fatal(err)
	}
	census.gid = before.GID ^ 1 // Per-member group must not be selected from the fixed-root expectation.
	census.mount, err = readonlyMountID(original, "", unix.AT_EMPTY_PATH)
	if err != nil {
		t.Fatal(err)
	}
	if entries, err := original.ReadDir(1); err != nil || len(entries) != 1 {
		t.Fatal("borrowed iterator setup differs", err)
	}
	if err := census.child(t.Context(), original, "bin", "/usr/bin", before, 1); err != nil {
		t.Fatal("real original child traversal refused its captured group", err)
	}
	if len(census.rows) != 2 || census.rows["/usr/bin"].facts != before || census.rows["/usr/bin/native"].facts != fileFacts {
		t.Fatal("real traversal changed original directory or file metadata", census.rows)
	}
	if rest, err := original.ReadDir(-1); err != nil || len(rest) != 2 {
		t.Fatal("child traversal closed or consumed the original borrow", err)
	}
	after, err := readonlyNamedFacts(original, "bin")
	if err != nil || after != before {
		t.Fatal("child traversal changed its original parent", err)
	}
	if body, err := os.ReadFile(path); err != nil || string(body) != "original group fixture body" {
		t.Fatal("child traversal changed original file bytes", err)
	}
}

func TestReadonlyRuntimeMemberGroupCannotAdmitUnsafeOwnerOrMode(t *testing.T) {
	for _, fault := range []string{"uid", "group-write", "other-write", "special"} {
		t.Run(fault, func(t *testing.T) {
			census, _ := readonlyRuntimeGroupCensus(t)
			facts := readonlyRuntimeObservedGroupFacts(42)
			switch fault {
			case "uid":
				facts.UID = 1
			case "group-write":
				facts.Mode |= 0020
			case "other-write":
				facts.Mode |= 0002
			case "special":
				facts.Mode = unix.S_IFIFO | 0600
			}
			err := census.record("/usr/bin/original", facts, "")
			if err == nil || len(census.rows) != 4 || len(census.aliases) != 0 {
				t.Fatal("nonzero group admitted unsafe original metadata", err)
			}
			if fault == "uid" {
				if !errors.Is(err, errReadonlyRuntimeMetadata) || !strings.Contains(err.Error(), "predicates=0x08") {
					t.Fatal("member owner refusal lost its original predicate", err)
				}
			} else if !errors.Is(err, errReadonlyRuntimeUnsafe) || !strings.Contains(err.Error(), "unsafe_predicates=") ||
				!strings.Contains(err.Error(), `member="bin/original"`) {
				t.Fatal("member refused for an unrelated metadata condition", err)
			}
		})
	}
}

func TestReadonlyRuntimeMemberGroupCannotChangeFixedPolicyRoles(t *testing.T) {
	for _, role := range []string{"root", "native", "primitive"} {
		t.Run(role, func(t *testing.T) {
			policy := readonlyPolicyFixture()
			policy.Executable.Path, policy.Argv[0] = "/usr/bin/grok", "/usr/bin/grok"
			policy.Executable.Facts.UID, policy.Executable.Facts.GID = 0, 0
			if err := policy.validate(); err != nil {
				t.Fatal("original fixed-role control refused", err)
			}
			switch role {
			case "root":
				policy.Runtime[0].Facts.GID = 42
			case "native":
				policy.Executable.Facts.GID = 42
			case "primitive":
				policy.Primitive.GID = 42
			}
			if err := policy.validate(); err == nil || !strings.Contains(err.Error(), "readonly original type, mode or ownership differs") {
				t.Fatal("member group policy escaped into a fixed role", err)
			}
		})
	}
}

func TestReadonlyRuntimeRetainedSetIDMetadataCannotAdmitSelectedExecutable(t *testing.T) {
	before := readonlyRuntimeObservedGroupFacts(65534)
	before.UID = 65534
	retained, err := readonlyRuntimeFacts(before, before.UID, before.GID)
	if err != nil || retained != before {
		t.Fatal("runtime set-id metadata lost original matching owners", retained, err)
	}
	if err := admitReadonlyFacts(retained, false, before.UID, before.GID); err == nil {
		t.Fatal("runtime set-id metadata relaxed selected executable admission")
	}
	plain := retained
	plain.Mode &^= 07000
	if err := admitReadonlyFacts(plain, false, before.UID, before.GID); err != nil {
		t.Fatal("selected executable mode-only positive control refused", err)
	}
}

func TestReadonlyRuntimeUnsafeMemberDiagnosticRetainsOriginalRow(t *testing.T) {
	for _, test := range []struct {
		name       string
		predicates uint8
	}{
		{"writable-file", 2}, {"writable-directory", 2}, {"fifo", 1}, {"writable-fifo", 3}, {"linked-symlink", 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			base, original, census, _ := readonlyRuntimeFixture(t)
			path := filepath.Join(base, "unsafe")
			createReadonlyRuntimeUnsafeMember(t, path, test.name)
			before, err := readonlyNamedFacts(original, "unsafe")
			if err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(base)
			if err != nil {
				t.Fatal(err)
			}
			if first, readErr := original.ReadDir(1); readErr != nil || len(first) != 1 {
				t.Fatal("original iterator control failed", readErr)
			}
			census.mount, err = readonlyMountID(original, "", unix.AT_EMPTY_PATH)
			if err != nil {
				t.Fatal(err)
			}
			err = census.entry(t.Context(), original, "/usr", "unsafe", 0)
			if !errors.Is(err, errReadonlyRuntimeUnsafe) || errors.Is(err, errReadonlyRuntimeMetadata) {
				t.Fatal("original member refused for an unrelated predicate", err)
			}
			for _, want := range []string{fmt.Sprintf("unsafe_predicates=0x%02x kind=%#o mode=%#o links=%d", test.predicates, before.Mode&unix.S_IFMT, before.Mode, before.Links),
				`member="unsafe"`, "member_bytes=6 ", "member_truncated=false"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("original refusal lacks %q: %s", want, err)
				}
			}
			if len(err.Error()) > readonlyRuntimeDiagnosticLimit || len(census.rows) != 0 || len(census.aliases) != 0 {
				t.Fatal("member diagnostic expanded its bound or admitted original evidence", err)
			}
			requireReadonlyRuntimeOriginalSurvives(t, path, original, len(entries), before)
		})
	}
}

func createReadonlyRuntimeUnsafeMember(t *testing.T, path, kind string) {
	t.Helper()
	var err error
	switch kind {
	case "writable-file":
		err = os.WriteFile(path, []byte("original native body"), 0600)
	case "writable-directory":
		err = os.Mkdir(path, 0700)
	case "fifo", "writable-fifo":
		err = unix.Mkfifo(path, 0600)
	case "linked-symlink":
		err = os.Symlink("unfollowed-target", path)
		if err == nil {
			err = os.Link(path, path+"-alias")
		}
	default:
		t.Fatal("unknown unsafe member fixture", kind)
	}
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(kind, "writable-") {
		if err := os.Chmod(path, 0770); err != nil {
			t.Fatal(err)
		}
	}
}
