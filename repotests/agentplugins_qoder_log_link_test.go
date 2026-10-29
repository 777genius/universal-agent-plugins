//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"unicode"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type qoderLogLinkSnapshot struct {
	Record, TargetRel           string
	Profile, Logs, Leaf, Target os.FileInfo
}

type qoderLogEdge struct {
	parent, file *os.File
	name         string
	info         os.FileInfo
}

func qoderLogRefused(category string) error {
	return fmt.Errorf("Qoder original log link refused: %s", category)
}

func qoderLogOpenAt(parent *os.File, name string, directory bool) (*os.File, error) {
	flags := unix.O_PATH | unix.O_NOFOLLOW | unix.O_CLOEXEC
	if directory {
		flags |= unix.O_DIRECTORY
	}
	fd, err := unix.Openat(int(parent.Fd()), name, flags, 0)
	if err != nil {
		return nil, qoderLogRefused("no-follow-open")
	}
	return os.NewFile(uintptr(fd), "qoder-held-node"), nil
}

func qoderLogOpenRoot(path string) (*os.File, error) {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || len(parts) > 64 || len(path) > 4096 {
		return nil, qoderLogRefused("profile-path")
	}
	fd, err := unix.Open("/", unix.O_PATH|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, qoderLogRefused("root-open")
	}
	current := os.NewFile(uintptr(fd), "qoder-held-root")
	for _, part := range parts {
		next, openErr := qoderLogOpenAt(current, part, true)
		closeErr := current.Close()
		if openErr != nil || closeErr != nil {
			if next != nil {
				if err := next.Close(); err != nil {
					return nil, qoderLogRefused("ancestor-close")
				}
			}
			return nil, qoderLogRefused("ancestor-open-or-close")
		}
		current = next
	}
	return current, nil
}

func qoderLogOwned(info os.FileInfo, directory bool) bool {
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != uint32(os.Getuid()) || identity.Gid != uint32(os.Getgid()) {
		return false
	}
	if directory {
		return info.IsDir()
	}
	return info.IsDir() || (info.Mode().IsRegular() && identity.Nlink == 1 && info.Size() >= 0)
}

func qoderLogTargetParts(body []byte) ([]string, error) {
	text := string(body)
	parts := strings.Split(text, string(filepath.Separator))
	if !utf8.Valid(body) || text == "" || text == "." || filepath.IsAbs(text) || filepath.VolumeName(text) != "" || (len(text) >= 2 && text[1] == ':') || filepath.Clean(text) != text || strings.Contains(text, "\\") || len(parts) > 30 {
		return nil, qoderLogRefused("target-path")
	}
	for _, character := range text {
		if unicode.IsControl(character) {
			return nil, qoderLogRefused("target-control")
		}
	}
	for _, part := range parts {
		if part == ".." || part == "" {
			return nil, qoderLogRefused("target-escape")
		}
	}
	return parts, nil
}

func qoderLogTarget(logs *os.File, parts []string, edges *[]qoderLogEdge) (os.FileInfo, error) {
	parent := logs
	var target os.FileInfo
	for index, part := range parts {
		directory := index < len(parts)-1
		file, err := qoderLogOpenAt(parent, part, directory)
		if err != nil {
			return nil, err
		}
		info, statErr := file.Stat()
		*edges = append(*edges, qoderLogEdge{parent: parent, file: file, name: part, info: info})
		if statErr != nil || !qoderLogOwned(info, directory) {
			return nil, qoderLogRefused("target-type-or-owner")
		}
		parent, target = file, info
	}
	return target, nil
}

func qoderLogCheckEdges(edges []qoderLogEdge) error {
	for _, edge := range edges {
		held, err := edge.file.Stat()
		if err != nil || !projectNativeRuntimeStableStat(edge.info.Sys(), held.Sys()) {
			return qoderLogRefused("held-identity-changed")
		}
		linked, err := qoderLogOpenAt(edge.parent, edge.name, false)
		if err != nil {
			return err
		}
		info, statErr := linked.Stat()
		closeErr := linked.Close()
		if statErr != nil || closeErr != nil || !projectNativeRuntimeStableStat(edge.info.Sys(), info.Sys()) {
			return qoderLogRefused("linked-identity-or-close")
		}
	}
	return nil
}

func qoderReadOriginalLogLink(root string, observed os.FileInfo) (snapshot qoderLogLinkSnapshot, resultErr error) {
	profile, err := qoderLogOpenRoot(root)
	if err != nil {
		return snapshot, err
	}
	edges := make([]qoderLogEdge, 0, 32)
	defer func() {
		for index := len(edges) - 1; index >= 0; index-- {
			if err := edges[index].file.Close(); err != nil {
				resultErr = qoderLogRefused("descriptor-close")
			}
		}
		if err := profile.Close(); err != nil {
			resultErr = qoderLogRefused("profile-close")
		}
		if resultErr != nil {
			snapshot = qoderLogLinkSnapshot{}
		}
	}()
	snapshot.Profile, err = profile.Stat()
	if err != nil || !qoderLogOwned(snapshot.Profile, true) {
		return snapshot, qoderLogRefused("profile-owner")
	}
	logs, err := qoderLogOpenAt(profile, "logs", true)
	if err != nil {
		return snapshot, err
	}
	snapshot.Logs, err = logs.Stat()
	edges = append(edges, qoderLogEdge{parent: profile, file: logs, name: "logs", info: snapshot.Logs})
	if err != nil || !qoderLogOwned(snapshot.Logs, true) {
		return snapshot, qoderLogRefused("logs-owner")
	}
	return qoderReadLogLeaf(root, logs, observed, snapshot, &edges)
}

func qoderReadLogLeaf(root string, logs *os.File, observed os.FileInfo, snapshot qoderLogLinkSnapshot, edges *[]qoderLogEdge) (qoderLogLinkSnapshot, error) {
	leaf, err := qoderLogOpenAt(logs, "latest", false)
	if err != nil {
		return snapshot, err
	}
	info, err := leaf.Stat()
	*edges = append(*edges, qoderLogEdge{parent: logs, file: leaf, name: "latest", info: info})
	if err != nil || !projectNativeRuntimeStableStat(observed.Sys(), info.Sys()) {
		return snapshot, qoderLogRefused("leaf-identity")
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || info.Mode() != os.ModeSymlink|0777 || identity.Nlink != 1 || identity.Uid != uint32(os.Getuid()) || identity.Gid != uint32(os.Getgid()) || info.Size() <= 0 || info.Size() > 4096 {
		return snapshot, qoderLogRefused("leaf-type-owner-or-size")
	}
	buffer := make([]byte, 4097)
	count, err := unix.Readlinkat(int(logs.Fd()), "latest", buffer)
	if err != nil || count <= 0 || count > 4096 || int64(count) != info.Size() {
		return snapshot, qoderLogRefused("link-read-size")
	}
	parts, err := qoderLogTargetParts(buffer[:count])
	if err != nil {
		return snapshot, err
	}
	snapshot.Target, err = qoderLogTarget(logs, parts, edges)
	if err != nil {
		return snapshot, err
	}
	if err := qoderLogCheckEdges(*edges); err != nil {
		return snapshot, err
	}
	linkedRoot, err := os.Lstat(root)
	if err != nil || projectNativeRuntimeParents(filepath.Join(root, "leaf")) != nil || !projectNativeRuntimeStableStat(snapshot.Profile.Sys(), linkedRoot.Sys()) {
		return snapshot, qoderLogRefused("profile-identity")
	}
	sum := sha256.Sum256(buffer[:count])
	snapshot.Leaf, snapshot.TargetRel = info, filepath.Join("logs", filepath.Join(parts...))
	snapshot.Record = fmt.Sprintf("symlink:%s:target:%s:%d:%s", qoderLogMetadata(info), qoderLogMetadata(snapshot.Target), count, hex.EncodeToString(sum[:]))
	return snapshot, nil
}

func qoderLogMetadata(info os.FileInfo) string {
	stat := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%d:%d:%o:%d:%d:%d:%d:%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Mode, stat.Nlink, stat.Uid, stat.Gid, stat.Size, stat.Mtim.Sec, stat.Mtim.Nsec, stat.Ctim.Sec, stat.Ctim.Nsec)
}

func qoderVerifyOriginalLogClosure(root string, before qoderLogLinkSnapshot, inventory map[string]string) error {
	kind := "file:"
	if before.Target.IsDir() {
		kind = "directory:"
	}
	if !strings.HasPrefix(inventory[before.TargetRel], kind) {
		return qoderLogRefused("target-not-in-complete-inventory")
	}
	after, err := qoderReadOriginalLogLink(root, before.Leaf)
	if err != nil {
		return err
	}
	if before.Record != after.Record || before.TargetRel != after.TargetRel || !projectNativeRuntimeStableStat(before.Profile.Sys(), after.Profile.Sys()) || !projectNativeRuntimeStableStat(before.Logs.Sys(), after.Logs.Sys()) || !projectNativeRuntimeStableStat(before.Target.Sys(), after.Target.Sys()) {
		return qoderLogRefused("snapshot-changed-during-inventory")
	}
	return nil
}

func qoderLogFixture(t *testing.T, target string) (string, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".qoder")
	if err := os.MkdirAll(filepath.Join(root, "logs", "run-private-target"), 0700); err != nil {
		t.Fatal(err)
	}
	projectNativeRuntimeWrite(t, filepath.Join(root, "logs", "run-private-target", "log"), []byte("private log contents\n"), 0600)
	link := filepath.Join(root, "logs", "latest")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	return root, link
}

func TestQoderOriginalLogLinkIsMetadataInsideCompleteFrozenProfile(t *testing.T) {
	for _, target := range []string{"run-private-target", "run-private-target/log"} {
		t.Run(target, func(t *testing.T) {
			root, link := qoderLogFixture(t, target)
			leaf, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			before, err := projectNativeCLIQoderProfile(root)
			if err != nil {
				t.Fatal(err)
			}
			record := before[filepath.Join("logs", "latest")]
			if !strings.HasPrefix(record, "symlink:") || strings.Contains(record, "run-private-target") || strings.Contains(record, "private log") || !strings.HasPrefix(before[filepath.Join("logs", "run-private-target", "log")], "file:") {
				t.Fatal("link followed, target omitted or private link text captured")
			}
			after, err := projectNativeCLIQoderProfile(root)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("unchanged complete original profile did not preserve exact log link")
			}
			final, err := os.Lstat(link)
			if err != nil || !projectNativeRuntimeStableStat(leaf.Sys(), final.Sys()) {
				t.Fatal("inventory changed the original symlink identity")
			}
		})
	}
}

func TestQoderOriginalLogLinkCannotGrantAliasOrForeignAuthority(t *testing.T) {
	for _, target := range []string{"/foreign", "C:foreign", "../outside", ".", "run-private-target/../run-private-target", "latest", "missing", "run-private-target\n", strings.Repeat("x", 4095)} {
		t.Run(fmt.Sprint(len(target), target[:min(len(target), 12)]), func(t *testing.T) {
			root, _ := qoderLogFixture(t, target)
			if _, err := projectNativeCLIQoderProfile(root); err == nil {
				t.Fatal("unsafe log target admitted")
			} else if strings.Contains(err.Error(), target) && len(target) > 12 {
				t.Fatal("private target escaped fixed refusal")
			}
		})
	}
}

func TestQoderOriginalLogLinkRejectsOtherLinksAliasesAndNonordinaryTargets(t *testing.T) {
	for _, kind := range []string{"other-link", "link-hardlink", "target-hardlink", "target-alias", "ancestor-alias", "fifo"} {
		t.Run(kind, func(t *testing.T) {
			root, link := qoderLogFixture(t, "run-private-target")
			qoderLogUnsafeFixture(t, root, link, kind)
			if _, err := projectNativeCLIQoderProfile(root); err == nil {
				t.Fatal("nonordinary original profile admitted", kind)
			}
		})
	}
}

func qoderLogUnsafeFixture(t *testing.T, root, link, kind string) {
	t.Helper()
	target := filepath.Join(root, "logs", "run-private-target")
	switch kind {
	case "other-link":
		if err := os.Symlink(target, filepath.Join(root, "other")); err != nil {
			t.Fatal(err)
		}
	case "link-hardlink":
		if err := os.Link(link, filepath.Join(t.TempDir(), "outside")); err != nil {
			t.Fatal(err)
		}
	case "target-hardlink":
		if err := os.Link(filepath.Join(target, "log"), filepath.Join(t.TempDir(), "outside")); err != nil {
			t.Fatal(err)
		}
	default:
		if err := os.Rename(target, filepath.Join(t.TempDir(), "outside")); err != nil {
			t.Fatal(err)
		}
		if kind == "fifo" {
			if err := syscall.Mkfifo(target, 0600); err != nil {
				t.Fatal(err)
			}
			return
		}
		if err := os.Symlink(t.TempDir(), target); err != nil {
			t.Fatal(err)
		}
		if kind == "ancestor-alias" {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("run-private-target/child", link); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestQoderOriginalLogLinkNeverExemptsNativeLogMutationOrRotation(t *testing.T) {
	for _, kind := range []string{"bytes", "mode", "empty-directory", "new-log", "rotation", "same-target-rotation"} {
		t.Run(kind, func(t *testing.T) {
			root, link := qoderLogFixture(t, "run-private-target")
			before, err := projectNativeCLIQoderProfile(root)
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			snapshot, err := qoderReadOriginalLogLink(root, info)
			if err != nil {
				t.Fatal(err)
			}
			qoderLogMutateFixture(t, root, link, kind)
			after, err := projectNativeCLIQoderProfile(root)
			if err == nil && reflect.DeepEqual(before, after) {
				t.Fatal("native log mutation became a profile exception")
			}
			if kind == "rotation" || kind == "same-target-rotation" {
				if err := qoderVerifyOriginalLogClosure(root, snapshot, before); err == nil {
					t.Fatal("original held link identity was adopted after rotation")
				}
			}
		})
	}
}

func qoderLogMutateFixture(t *testing.T, root, link, kind string) {
	t.Helper()
	path := filepath.Join(root, "logs", "run-private-target", "log")
	var err error
	switch kind {
	case "bytes":
		err = os.WriteFile(path, []byte("changed private contents\n"), 0600)
	case "mode":
		err = os.Chmod(path, 0640)
	case "empty-directory":
		err = os.Mkdir(filepath.Join(root, "logs", "empty"), 0700)
	case "new-log":
		err = os.WriteFile(filepath.Join(root, "logs", "new"), []byte("new private log\n"), 0600)
	case "rotation":
		if err = os.Mkdir(filepath.Join(root, "logs", "next-run"), 0700); err == nil {
			err = os.Remove(link)
		}
		if err == nil {
			err = os.Symlink("next-run", link)
		}
	case "same-target-rotation":
		if err = os.Remove(link); err == nil {
			err = os.Symlink("run-private-target", link)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestQoderOriginalLogLinkRequiresTargetInTheOrdinaryInventory(t *testing.T) {
	root, link := qoderLogFixture(t, "run-private-target")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := qoderReadOriginalLogLink(root, info)
	if err != nil {
		t.Fatal(err)
	}
	if err := qoderVerifyOriginalLogClosure(root, snapshot, map[string]string{}); err == nil {
		t.Fatal("opaque log target without its ordinary closure admitted")
	}
	body, err := os.ReadFile(filepath.Join(root, "logs", "run-private-target", "log"))
	if err != nil || !bytes.Equal(body, []byte("private log contents\n")) {
		t.Fatal("link inventory changed private target contents")
	}
}
