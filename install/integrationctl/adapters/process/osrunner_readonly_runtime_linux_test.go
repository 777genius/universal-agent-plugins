//go:build linux

package process

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func readonlyRuntimeFixture(t *testing.T) (string, *os.File, readonlyRuntimeCensus, [4]readonlyRuntimeRoot) {
	t.Helper()
	base := t.TempDir()
	for _, name := range []string{"bin", "lib", "lib64"} {
		if err := os.Mkdir(filepath.Join(base, name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	roots := [4]readonlyRuntimeRoot{}
	for i, visible := range readonlyRuntimePaths() {
		path, logical := base, "/usr"
		if i != 0 {
			path, logical = filepath.Join(base, filepath.Base(visible)), filepath.Join("/usr", filepath.Base(visible))
		}
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = file.Close() })
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		facts, err := readonlyFacts(info)
		if err != nil {
			t.Fatal(err)
		}
		roots[i] = readonlyRuntimeRoot{visible: visible, object: readonlyOriginalObject{path: logical, file: file, facts: facts, directory: true}}
	}
	census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias), uid: uint32(os.Getuid()), gid: uint32(os.Getgid())}
	return base, roots[0].object.file, census, roots
}

func observeReadonlyRuntimeFixture(t *testing.T, census *readonlyRuntimeCensus, roots *[4]readonlyRuntimeRoot) error {
	t.Helper()
	for i := range roots {
		info, err := roots[i].object.file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		roots[i].object.facts, err = readonlyFacts(info)
		if err != nil {
			t.Fatal(err)
		}
	}
	iterator, err := openReadonlyDirectory(roots[0].object.file, ".")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = iterator.Close() })
	census.mount, err = readonlyMountID(iterator, "", unix.AT_EMPTY_PATH)
	if err != nil {
		t.Fatal(err)
	}
	return census.directory(t.Context(), iterator, "/usr", roots[0].object.facts, 0)
}

func TestReadonlyRuntimeCensusJoinsRealInternalAliasesWithoutConsumingBorrow(t *testing.T) {
	base, original, census, roots := readonlyRuntimeFixture(t)
	if err := os.WriteFile(filepath.Join(base, "bin", "native"), []byte("native"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("native", filepath.Join(base, "bin", "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(filepath.Join(base, "bin", "native"), filepath.Join(base, "lib", "native")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/lib/native", filepath.Join(base, "lib64", "alias")); err != nil {
		t.Fatal(err)
	}
	if first, err := original.ReadDir(1); err != nil || len(first) != 1 {
		t.Fatal("original iterator setup differs", err)
	}
	if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots)); err != nil {
		t.Fatal("complete original runtime aliases were refused", err)
	}
	proof, err := digestReadonlyRuntime(t.Context(), census.rows)
	if err != nil || proof.Count != 8 || len(proof.SHA256) != 64 || census.rows["/usr/lib64/alias"].link != "/lib/native" {
		t.Fatal("runtime census omitted original alias facts", proof, err)
	}
	if rest, err := original.ReadDir(-1); err != nil || len(rest) != 2 {
		t.Fatal("runtime census consumed the borrowed iterator", err)
	}
}

func TestReadonlyRuntimeClosureRefusesExternalHardlinkAndSpecialMember(t *testing.T) {
	for _, fault := range []string{"external-hardlink", "special"} {
		t.Run(fault, func(t *testing.T) {
			base, original, census, roots := readonlyRuntimeFixture(t)
			leaf := filepath.Join(base, "bin", "native")
			if err := os.WriteFile(leaf, []byte("native"), 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch fault {
			case "external-hardlink":
				err = os.Link(leaf, filepath.Join(t.TempDir(), "outside"))
			case "special":
				err = unix.Mkfifo(filepath.Join(base, "lib", "fifo"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			err = observeReadonlyRuntimeFixture(t, &census, &roots)
			if err == nil {
				err = errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots))
			}
			if err == nil {
				t.Fatal("incomplete or special runtime closure was admitted", fault)
			}
			if _, err := original.Stat(); err != nil {
				t.Fatal("runtime refusal closed a borrowed root", err)
			}
		})
	}
}

func TestReadonlyRuntimeAliasesAreCapturedDataAndChangesRemainVisible(t *testing.T) {
	for _, target := range []string{"/etc/alternatives/awk", "missing", "alias"} {
		t.Run(target, func(t *testing.T) {
			base, _, census, roots := readonlyRuntimeFixture(t)
			path := filepath.Join(base, "bin", "alias")
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
				t.Fatal(err)
			}
			if err := errors.Join(census.requireAliases(t.Context()), census.requireRoots(t.Context(), roots)); err != nil {
				t.Fatal("runtime alias target was followed", err)
			}
			before, err := digestReadonlyRuntime(t.Context(), census.rows)
			if err != nil || census.rows["/usr/bin/alias"].link != target {
				t.Fatal("original runtime alias text was omitted", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("changed", path); err != nil {
				t.Fatal(err)
			}
			census.rows = make(map[string]readonlyRuntimeRow)
			if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
				t.Fatal(err)
			}
			after, err := digestReadonlyRuntime(t.Context(), census.rows)
			if err != nil || before == after {
				t.Fatal("changed original runtime alias was ignored", err)
			}
		})
	}
}

func TestReadonlyRuntimeOwnerRetentionAndCancellationAreExplicit(t *testing.T) {
	before := readonlyObjectFacts{Device: 1, Inode: 2, Links: 1, Mode: unix.S_IFREG | 04755, UID: 65534, GID: 65534, Size: 4}
	retained, err := readonlyRuntimeFacts(before, 65534, 65534)
	if err != nil || retained != before {
		t.Fatal("original host runtime metadata differs", retained, err)
	}
	if _, err := readonlyRuntimeFacts(before, 1000, 1000); err == nil {
		t.Fatal("overflow was treated as an arbitrary owner")
	}
	retained.UID, retained.GID = 0, 0
	if err := admitReadonlyFacts(retained, false, 0, 0); err == nil {
		t.Fatal("runtime metadata rules relaxed selected executable admission")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := censusReadonlyRuntime(ctx, [4]readonlyRuntimeRoot{}, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("runtime census reached descriptors after cancellation", err)
	}
}

func TestReadonlyRuntimeOriginalPackageGroupSurvivesCensus(t *testing.T) {
	// The original hosted refusal was root-owned /usr/bin/chage, group 42.
	original := readonlyObjectFacts{Device: 66305, Inode: 2047, Links: 1,
		Mode: unix.S_IFREG | 02755, GID: 42, Size: 72184}
	original.Ctime.Nsec = 273560716
	census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias)}
	if err := census.record("/usr/bin/chage", original, ""); err != nil {
		t.Fatal("original root-owned package group was refused", err)
	}
	if census.rows["/usr/bin/chage"].facts != original {
		t.Fatal("original package group was normalized or omitted")
	}
	before, err := digestReadonlyRuntime(t.Context(), census.rows)
	if err != nil {
		t.Fatal(err)
	}
	changed := original
	changed.GID++
	census.rows = make(map[string]readonlyRuntimeRow)
	census.aliases = make(map[[2]uint64]readonlyRuntimeAlias)
	if err := census.record("/usr/bin/chage", changed, ""); err != nil {
		t.Fatal(err)
	}
	after, err := digestReadonlyRuntime(t.Context(), census.rows)
	if err != nil || before == after {
		t.Fatal("different original host groups collapsed into one proof", before, after, err)
	}
}

func TestReadonlyRuntimePackageGroupsKeepOwnerAndWriteGuards(t *testing.T) {
	for _, fault := range []string{"owner", "group-write", "other-write"} {
		t.Run(fault, func(t *testing.T) {
			facts := readonlyObjectFacts{Device: 1, Inode: 2, Links: 1, Mode: unix.S_IFREG | 02755, GID: 42}
			switch fault {
			case "owner":
				facts.UID = 1
			case "group-write":
				facts.Mode |= 0020
			case "other-write":
				facts.Mode |= 0002
			}
			census := readonlyRuntimeCensus{rows: make(map[string]readonlyRuntimeRow), aliases: make(map[[2]uint64]readonlyRuntimeAlias)}
			if err := census.record("/usr/bin/chage", facts, ""); err == nil || len(census.rows) != 0 || len(census.aliases) != 0 {
				t.Fatal("package group admitted unsafe runtime metadata", err)
			}
		})
	}
}

func TestReadonlyRuntimeFourRootsKeepCapturedGroups(t *testing.T) {
	_, _, census, roots := readonlyRuntimeFixture(t)
	if err := observeReadonlyRuntimeFixture(t, &census, &roots); err != nil {
		t.Fatal(err)
	}
	if err := census.requireRoots(t.Context(), roots); err != nil {
		t.Fatal(err)
	}
	for index, root := range roots {
		changed := roots
		changed[index].object.facts.GID ^= 1
		if err := census.requireRoots(t.Context(), changed); !errors.Is(err, errReadonlyRuntimeMetadata) {
			t.Fatal("fixed runtime root lost its captured group", root.visible, err)
		}
	}
}

func TestReadonlyRuntimeMetadataDiagnosticRetainsFirstCapturedRow(t *testing.T) {
	for _, kind := range []string{"root", "root-group", "file", "directory"} {
		t.Run(kind, func(t *testing.T) {
			base, original, census, roots := readonlyRuntimeFixture(t)
			name, path, relative := "", base, "."
			predicates := uint8(8)
			switch kind {
			case "root":
				census.uid ^= 1
			case "root-group":
				census.gid ^= 1
				predicates = 16
			case "file":
				name, relative = "native", "native"
				path = filepath.Join(base, name)
				if err := os.WriteFile(path, []byte("original native body"), 0600); err != nil {
					t.Fatal(err)
				}
				census.uid ^= 1
			case "directory":
				name, relative = "bin", "bin"
				path = filepath.Join(base, name)
				census.uid ^= 1
			}
			info, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			before, err := readonlyFacts(info)
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
			if kind == "root" || kind == "root-group" {
				err = observeReadonlyRuntimeFixture(t, &census, &roots)
			} else {
				err = census.entry(t.Context(), original, "/usr", name, 0)
			}
			requireReadonlyRuntimeDiagnostic(t, err, before, census.uid, census.gid, predicates, relative)
			if len(census.rows) != 0 || len(census.aliases) != 0 {
				t.Fatal("rejected original became census evidence")
			}
			requireReadonlyRuntimeOriginalSurvives(t, path, original, len(entries), before)
		})
	}
}

func requireReadonlyRuntimeOriginalSurvives(t *testing.T, path string, original *os.File, entries int, before readonlyObjectFacts) {
	t.Helper()
	if remaining, err := original.ReadDir(-1); err != nil || len(remaining) != entries-1 {
		t.Fatal("refusal closed or consumed the borrowed root", err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	after, err := readonlyFacts(info)
	if err != nil || after != before {
		t.Fatal("refusal changed or replaced the original row", before, after, err)
	}
	if before.Mode&unix.S_IFMT == unix.S_IFREG {
		body, readErr := os.ReadFile(path)
		if readErr != nil || string(body) != "original native body" {
			t.Fatal("refusal changed the original file", readErr)
		}
	}
}

func requireReadonlyRuntimeDiagnostic(t *testing.T, err error, facts readonlyObjectFacts, uid, gid uint32, predicates uint8, relative string) {
	t.Helper()
	if !errors.Is(err, errReadonlyRuntimeMetadata) {
		t.Fatal("original metadata refusal was lost", err)
	}
	text := err.Error()
	wants := []string{
		fmt.Sprintf("readonly runtime ownership or metadata differs: predicates=0x%02x ", predicates),
		fmt.Sprintf("dev=%d ino=%d mode=%#o links=%d size=%d ", facts.Device, facts.Inode, facts.Mode, facts.Links, facts.Size),
		fmt.Sprintf("uid=%d expected_uid=%d gid=%d expected_gid=%d ", facts.UID, uid, facts.GID, gid),
		fmt.Sprintf("mtime_nsec=%d ctime_nsec=%d; ", facts.Mtime.Nsec, facts.Ctime.Nsec),
		"member=" + strconv.QuoteToASCII(relative), "member_truncated=false",
	}
	for _, want := range wants {
		if !strings.Contains(text, want) {
			t.Fatalf("original refusal lacks %q: %s", want, text)
		}
	}
	if len(text) > readonlyRuntimeDiagnosticLimit {
		t.Fatal("original refusal diagnostic exceeded its bound")
	}
}

func TestReadonlyRuntimeMetadataDiagnosticKeepsEveryOriginalPredicate(t *testing.T) {
	original := readonlyObjectFacts{Device: 11, Inode: 22, Links: 1, Mode: unix.S_IFREG | 0700, UID: 123, GID: 456, Size: 7}
	for _, test := range []struct {
		name       string
		predicates uint8
		mutate     func(*readonlyObjectFacts)
	}{
		{"inode", 1, func(f *readonlyObjectFacts) { f.Inode = 0 }},
		{"links", 2, func(f *readonlyObjectFacts) { f.Links = 0 }},
		{"size", 4, func(f *readonlyObjectFacts) { f.Size = -1 }},
		{"uid", 8, func(f *readonlyObjectFacts) { f.UID++ }},
		{"gid", 16, func(f *readonlyObjectFacts) { f.GID++ }},
		{"mtime-negative", 32, func(f *readonlyObjectFacts) { f.Mtime.Nsec = -1 }},
		{"mtime-upper", 32, func(f *readonlyObjectFacts) { f.Mtime.Nsec = 1e9 }},
		{"ctime-negative", 64, func(f *readonlyObjectFacts) { f.Ctime.Nsec = -1 }},
		{"ctime-upper", 64, func(f *readonlyObjectFacts) { f.Ctime.Nsec = 1e9 }},
		{"all", 127, func(f *readonlyObjectFacts) {
			f.Inode, f.Links, f.Size, f.UID, f.GID = 0, 0, -1, 1, 2
			f.Mtime.Nsec, f.Ctime.Nsec = -1, 1e9
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := original
			test.mutate(&facts)
			result, err := readonlyRuntimeFacts(facts, original.UID, original.GID)
			if result != (readonlyObjectFacts{}) {
				t.Fatal("rejected facts were normalized into authority", result)
			}
			err = readonlyRuntimeMemberError("/usr/bin/original", err)
			requireReadonlyRuntimeDiagnostic(t, err, facts, original.UID, original.GID, test.predicates, "bin/original")
		})
	}
	upper := original
	upper.Mtime.Nsec, upper.Ctime.Nsec = 999999999, 999999999
	for _, facts := range []readonlyObjectFacts{original, upper} {
		result, err := readonlyRuntimeFacts(facts, original.UID, original.GID)
		if err != nil || result != facts {
			t.Fatal("valid original host metadata changed", result, err)
		}
	}
	facts := original
	facts.Mode |= 0022
	_, err := readonlyRuntimeFacts(facts, original.UID, original.GID)
	if err == nil || err.Error() != "readonly runtime contains an unsafe or special member" || errors.Is(err, errReadonlyRuntimeMetadata) {
		t.Fatal("later original mode refusal changed", err)
	}
	requireReadonlyRuntimeSameError(t, readonlyRuntimeMemberError("/usr/bin/original", err), err)
}

func TestReadonlyRuntimeMemberDiagnosticUsesOriginalBoundedBytes(t *testing.T) {
	facts := readonlyObjectFacts{Device: ^uint64(0), Inode: ^uint64(0), Links: ^uint64(0),
		Mode: ^uint32(0), UID: ^uint32(0), GID: ^uint32(0), Size: -1}
	facts.Mtime.Nsec, facts.Ctime.Nsec = -1, 1e9
	_, originalErr := readonlyRuntimeFacts(facts, 0, 0)
	if !errors.Is(originalErr, errReadonlyRuntimeMetadata) {
		t.Fatal("original real guard did not refuse", originalErr)
	}
	for _, path := range []string{"/usr", "/usr/bin/tool", "/usr/lib/tab\tquote\"backslash\\", "/usr/" + strings.Repeat("a", 4000),
		"/usr/bin/" + strings.Repeat("a", 123) + "é-tail", "/usr/" + strings.Repeat("\x01", 128) + "tail"} {
		t.Run(strconv.Itoa(len(path)), func(t *testing.T) {
			err := readonlyRuntimeMemberError(path, originalErr)
			text := err.Error()
			if !errors.Is(err, originalErr) || len(text) > readonlyRuntimeDiagnosticLimit {
				t.Fatal("original refusal or byte bound changed", err)
			}
			for _, char := range text {
				if char < 32 || char > 126 {
					t.Fatalf("diagnostic contains unescaped rune %U", char)
				}
			}
			relative := "."
			if path != "/usr" {
				relative = path[len("/usr/"):]
			}
			preview := relative[:min(len(relative), 128)]
			digest := sha256.Sum256([]byte("agentplugins-readonly-runtime-member-v1\x00" + relative))
			for _, want := range []string{"member=" + strconv.QuoteToASCII(preview), fmt.Sprintf("member_bytes=%d ", len(relative)),
				fmt.Sprintf("member_sha256=%x ", digest), fmt.Sprintf("member_truncated=%t", len(relative) > 128)} {
				if !strings.Contains(text, want) {
					t.Fatalf("original full member projection lacks %q: %s", want, text)
				}
			}
			if strings.HasSuffix(path, "é-tail") && !strings.Contains(text, `\xc3`) {
				t.Fatal("split UTF-8 byte was not safely escaped", text)
			}
		})
	}
	for _, path := range []string{"/usr/", "/usr/../etc/passwd", "/etc/passwd", "usr/bin/tool", "/usr/x\n", "/usr/x\r", "/usr/x\x00", "/usr/x\xff", "/usr/" + strings.Repeat("a", 4096)} {
		requireReadonlyRuntimeSameError(t, readonlyRuntimeMemberError(path, originalErr), originalErr)
	}
	other := errors.New("readonly runtime ownership or metadata differs")
	requireReadonlyRuntimeSameError(t, readonlyRuntimeMemberError("/usr/bin/tool", other), other)
	requireReadonlyRuntimeSameError(t, readonlyRuntimeMemberError("/usr", nil), nil)
}

func requireReadonlyRuntimeSameError(t *testing.T, actual, expected error) {
	t.Helper()
	// Exact value identity matters here: errors.Is would permit a new wrapper.
	if reflect.ValueOf(actual) != reflect.ValueOf(expected) {
		t.Fatalf("projection replaced the original error: %T %v; want %T %v", actual, actual, expected, expected)
	}
}
