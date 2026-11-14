//go:build darwin && arm64 && nativequalification

package nativeconfig

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Identical bodies compile with frozen old Begin and candidate public Plain.
// Legacy selection is a real runtime comparison, never missing-API compile RED.
func plainTestEntry(t *testing.T, kernel Kernel) func(string) (*ExactFile, error) {
	t.Helper()
	if os.Getenv("TEST_PLAIN_ENTRY") == "legacy" {
		return kernel.BeginExactFile
	}
	candidate, ok := any(kernel).(interface {
		BeginPlainExactFile(string) (*ExactFile, error)
	})
	plainTestRequire(t, ok, "candidate entry unavailable; select legacy for frozen source")
	return candidate.BeginPlainExactFile
}
func plainTestBegin(t *testing.T, path string) *ExactFile {
	t.Helper()
	file, err := plainTestEntry(t, New())(path)
	plainTestRequire(t, err == nil, "begin: %v", err)
	t.Cleanup(func() { closeExactFile(t, file) })
	return file
}
func plainTestRequire(t *testing.T, good bool, format string, args ...any) {
	t.Helper()
	if !good {
		t.Fatalf(format, args...)
	}
}
func plainTestMust(t *testing.T, err error) { t.Helper(); plainTestRequire(t, err == nil, "%v", err) }
func plainTestAllowed(path string) bool {
	path = filepath.Clean(path)
	return filepath.IsAbs(path) && (strings.HasPrefix(path, "/private/tmp/TEST-darwin-backend-r7.") || strings.HasPrefix(path, "/tmp/TEST-darwin-backend-r7."))
}
func plainTestRoot(t *testing.T) string {
	t.Helper()
	plainTestRequire(t, plainTestAllowed(os.Getenv("TMPDIR")), "requires a NEW TEST TMPDIR")
	return t.TempDir()
}

type plainTestWitness struct {
	Dev                          int64
	Ino                          uint64
	Mode, UID, GID, Flags, Links uint32
	Size                         int64
	Mtime, Ctime                 [2]int64
	ACL                          string
	ACLCount                     int64
	Inherited                    bool
	Attrs                        map[string]string
}

// Independent C executable uses direct libc, not this backend's Go reader.
func plainTestQuery(t *testing.T, path string, fd *os.File) plainTestWitness {
	t.Helper()
	helper := os.Getenv("TEST_PLAIN_WITNESS")
	plainTestRequire(t, plainTestAllowed(helper), "missing independent C witness under NEW TEST root")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, helper, path)
	if fd != nil {
		command = exec.CommandContext(ctx, helper, "--fd", "3")
		command.ExtraFiles = []*os.File{fd}
	}
	output, err := command.CombinedOutput()
	plainTestRequire(t, err == nil, "actual witness failed for %s: %v %s", path, err, output)
	var witness plainTestWitness
	plainTestMust(t, json.Unmarshal(output, &witness))
	t.Logf("witness %s: %s", path, output)
	return witness
}
func plainTestMetadata(w plainTestWitness) plainTestWitness {
	w.Dev, w.Ino, w.Size, w.Links = 0, 0, 0, 0
	w.Mtime, w.Ctime = [2]int64{}, [2]int64{}
	return w
}
func plainTestBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	plainTestRequire(t, err == nil && bytes.Equal(got, want), "readback %s differs: %v", path, err)
}

// Observe an ACTUAL sibling name and retain its independently opened fd. No
// hooks or skipped missing witnesses. Query after publication is not a proof
// of pre-publication race timing; that independent native witness stays pending.
func plainTestWatch(t *testing.T, root, target string) (<-chan *os.File, func()) {
	result, done := make(chan *os.File, 1), make(chan struct{})
	var once sync.Once
	stop := func() { once.Do(func() { close(done) }) }
	t.Cleanup(func() {
		stop()
		if f := <-result; f != nil {
			plainTestMust(t, f.Close())
		}
	})
	go func() {
		defer close(result)
		for {
			entries, _ := os.ReadDir(root)
			for _, entry := range entries {
				if !strings.HasPrefix(entry.Name(), ".plain-exact-") && !strings.HasPrefix(entry.Name(), target+".tmp-") {
					continue
				}
				fd, err := unix.Open(filepath.Join(root, entry.Name()), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
				if err == nil {
					result <- os.NewFile(uintptr(fd), "independent-stage")
					return
				}
			}
			select {
			case <-done:
				return
			default:
			}
			time.Sleep(time.Millisecond)
		}
	}()
	return result, stop
}
func plainTestStage(t *testing.T, result <-chan *os.File, stop func(), path string, body []byte) plainTestWitness {
	t.Helper()
	stop()
	fd := <-result
	plainTestRequire(t, fd != nil, "actual sibling witness missing; native qualification incomplete")
	defer func() { plainTestMust(t, fd.Close()) }()
	before := plainTestQuery(t, path+" actual sibling fd", fd)
	_, err := fd.Seek(0, io.SeekStart)
	plainTestMust(t, err)
	actual, err := io.ReadAll(fd)
	plainTestRequire(t, err == nil && bytes.Equal(actual, body), "actual sibling bytes differ: %v", err)
	after := plainTestQuery(t, path+" actual sibling fd after bytes", fd)
	plainTestRequire(t, reflect.DeepEqual(before, after), "actual sibling drifted during independent read")
	return after
}
func plainTestApplyWitness(t *testing.T, file *ExactFile, path string, body []byte) plainTestWitness {
	stages, stop := plainTestWatch(t, filepath.Dir(path), filepath.Base(path))
	err := file.Apply(body)
	t.Logf("Apply result before independent stage assertions: err=%v effect=%s", err, file.Effect())
	stage := plainTestStage(t, stages, stop, path, body)
	plainTestRequire(t, err == nil && file.Effect() == FileCommitted, "positive unqualified: %v %s", err, file.Effect())
	plainTestBytes(t, path, body)
	output := plainTestQuery(t, path, nil)
	plainTestRequire(t, stage.Ino == output.Ino && reflect.DeepEqual(plainTestMetadata(stage), plainTestMetadata(output)), "actual stage/output bytes or metadata identity")
	return output
}
func plainTestRestoreWitness(t *testing.T, file *ExactFile, path string, body []byte, before, output plainTestWitness, exists bool) {
	var stages <-chan *os.File
	var stop func()
	if exists {
		stages, stop = plainTestWatch(t, filepath.Dir(path), filepath.Base(path))
	}
	err := file.Rollback()
	plainTestRequire(t, err == nil && file.Effect() == FileUnchanged, "untouched rollback: %v %s", err, file.Effect())
	if !exists {
		_, err := os.Lstat(path)
		plainTestRequire(t, os.IsNotExist(err), "absent rollback: %v", err)
		return
	}
	stage := plainTestStage(t, stages, stop, path, body)
	plainTestBytes(t, path, body)
	restored := plainTestQuery(t, path, nil)
	plainTestRequire(t, restored.Ino != output.Ino && stage.Ino == restored.Ino, "restoration must bind its actual new identity")
	plainTestRequire(t, reflect.DeepEqual(plainTestMetadata(before), plainTestMetadata(restored)) && reflect.DeepEqual(plainTestMetadata(stage), plainTestMetadata(restored)), "restoration-stage/output metadata differ from original")
}
func TestPlainExactFileDarwinPositive(t *testing.T) {
	for _, exists := range []bool{true, false} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			path := filepath.Join(plainTestRoot(t), "target")
			original := bytes.Repeat([]byte("original\n"), 1<<20)
			var before plainTestWitness
			if exists {
				plainTestMust(t, os.WriteFile(path, original, 0640))
				before = plainTestQuery(t, path, nil)
			}
			file := plainTestBegin(t, path)
			snapshot := file.Original()
			plainTestRequire(t, snapshot.Exists == exists && (!exists || bytes.Equal(snapshot.Body, original)), "locked original snapshot")
			output := plainTestApplyWitness(t, file, path, bytes.Repeat([]byte("changed\n"), 1<<20))
			if exists {
				plainTestRequire(t, reflect.DeepEqual(plainTestMetadata(before), plainTestMetadata(output)), "original/output mode/owner/security/provenance changed")
			}
			plainTestRestoreWitness(t, file, path, original, before, output, exists)
		})
	}
}
func plainTestDrift(t *testing.T, path, kind string) {
	t.Helper()
	before := plainTestQuery(t, path, nil)
	var err error
	switch kind {
	case "chmod":
		err = os.Chmod(path, 0644)
	case "inode":
		body, readErr := os.ReadFile(path)
		plainTestMust(t, readErr)
		err = os.WriteFile(path+"-foreign", body, os.FileMode(before.Mode&0777))
		if err == nil {
			err = os.Rename(path+"-foreign", path)
		}
	case "provenance":
		err = unix.Setxattr(path, "com.apple.provenance", []byte("TEST-foreign-provenance"), 0)
	}
	plainTestRequire(t, err == nil, "%s actual drift witness unavailable: %v", kind, err)
	after := plainTestQuery(t, path, nil)
	plainTestRequire(t, !reflect.DeepEqual(before, after), "actual drift did not occur")
	if kind == "provenance" {
		plainTestRequire(t, !reflect.DeepEqual(before.Attrs, after.Attrs), "actual opaque provenance did not change")
	}
}
func plainTestDriftCase(t *testing.T, exists, applied bool, kind string) {
	path := filepath.Join(plainTestRoot(t), "target")
	body := []byte("original")
	if exists {
		plainTestMust(t, os.WriteFile(path, body, 0600))
	}
	file := plainTestBegin(t, path)
	if applied {
		body = []byte("changed")
		plainTestMust(t, file.Apply(body))
	}
	plainTestDrift(t, path, kind)
	foreign := plainTestQuery(t, path, nil)
	if !applied {
		plainTestRequire(t, file.Apply([]byte("changed")) != nil, "overwrote same-byte foreign state")
	}
	err := file.Rollback()
	plainTestRequire(t, err != nil && file.Effect() == FileUncertain, "foreign output adopted: %v %s", err, file.Effect())
	plainTestBytes(t, path, body)
	plainTestRequire(t, reflect.DeepEqual(foreign, plainTestQuery(t, path, nil)), "foreign identity/metadata mutated")
}

// RED: old byte CAS overwrites same-byte chmod/inode/provenance drift.
func TestPlainExactFileDarwinBeforeApplyDrift(t *testing.T) {
	for _, kind := range []string{"chmod", "inode", "provenance"} {
		t.Run(kind, func(t *testing.T) { plainTestDriftCase(t, true, false, kind) })
	}
}

// RED: old rollback restores/removes same-byte foreign existing/new output.
func TestPlainExactFileDarwinAfterApplyDrift(t *testing.T) {
	for _, exists := range []bool{true, false} {
		for _, kind := range []string{"chmod", "inode", "provenance"} {
			t.Run(fmt.Sprint(exists)+"/"+kind, func(t *testing.T) { plainTestDriftCase(t, exists, true, kind) })
		}
	}
}
func plainTestControlWitness(kind string, w plainTestWitness) bool {
	acl, _ := hex.DecodeString(w.ACL)
	switch kind {
	case "other-xattr":
		return w.Attrs[hex.EncodeToString([]byte("user.TEST"))] != ""
	case "compression":
		value, found := w.Attrs[hex.EncodeToString([]byte("com.apple.decmpfs"))]
		return found && len(value) >= 32 && w.Flags&unix.UF_COMPRESSED != 0
	case "inherited-acl":
		return len(acl) > 44 && w.Inherited && w.ACLCount > 0
	case "present-empty-acl":
		return len(acl) > 44 && w.ACLCount == 0
	case "flags":
		return w.Flags != 0
	case "hardlink":
		return w.Links > 1
	case "owner":
		return int64(w.UID) != int64(os.Geteuid())
	case "special-mode":
		return w.Mode&07000 != 0
	case "readonly":
		return w.Mode&0200 == 0
	}
	return false
}

// Every control requires actual independently decoded metadata. Missing actual
// compression/inherited/present-empty ACL/owner witnesses fail, never skip.
func TestPlainExactFileDarwinRefusalControls(t *testing.T) {
	root := os.Getenv("TEST_PLAIN_CONTROL_ROOT")
	plainTestRequire(t, plainTestAllowed(root), "missing independent NEW TEST control root")
	for _, kind := range []string{"other-xattr", "compression", "inherited-acl", "present-empty-acl", "flags", "hardlink", "owner", "special-mode", "readonly"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(root, kind, "target")
			before := plainTestQuery(t, path, nil)
			plainTestRequire(t, plainTestControlWitness(kind, before), "independent actual %s witness missing", kind)
			body, err := os.ReadFile(path)
			plainTestMust(t, err)
			file := plainTestBegin(t, path)
			plainTestRequire(t, file.Apply(append(bytes.Clone(body), '\n')) != nil, "forbidden metadata accepted")
			plainTestBytes(t, path, body)
			plainTestRequire(t, reflect.DeepEqual(before, plainTestQuery(t, path, nil)), "refusal mutated control metadata")
		})
	}
}
func TestPlainExactFileDarwinNoopAndLeafLink(t *testing.T) {
	root := plainTestRoot(t)
	path := filepath.Join(root, "target")
	body := []byte("unchanged")
	plainTestMust(t, os.WriteFile(path, body, 0600))
	before := plainTestQuery(t, path, nil)
	planning := plainTestBegin(t, path)
	plainTestRequire(t, bytes.Equal(planning.Original().Body, body), "unchanged planning")
	plainTestMust(t, planning.Close())
	plainTestRequire(t, reflect.DeepEqual(before, plainTestQuery(t, path, nil)), "unchanged planning mutated file")
	file := plainTestBegin(t, path)
	err := file.Apply(body)
	plainTestRequire(t, err == nil && file.Effect() == FileUnchanged, "noop: %v %s", err, file.Effect())
	plainTestMust(t, file.Rollback())
	plainTestRequire(t, reflect.DeepEqual(before, plainTestQuery(t, path, nil)), "noop replaced original")
	plainTestMust(t, file.Close())
	plainTestMust(t, os.Symlink(path, path+"-link"))
	linked, err := plainTestEntry(t, New())(path + "-link")
	if linked != nil {
		plainTestMust(t, linked.Close())
	}
	plainTestRequire(t, err != nil, "leaf link admitted")
	plainTestBytes(t, path, body)
}
func TestPlainExactFileDarwinCustomIO(t *testing.T) {
	path := filepath.Join(plainTestRoot(t), "target")
	body := []byte("unchanged")
	plainTestMust(t, os.WriteFile(path, body, 0600))
	before := plainTestQuery(t, path, nil)
	file, err := plainTestEntry(t, NewWithFileIO(osFiles{}))(path) // real OS IO without default authority
	plainTestMust(t, err)
	defer closeExactFile(t, file)
	plainTestRequire(t, file.Apply([]byte("changed")) != nil, "custom IO authorized mutation")
	plainTestBytes(t, path, body)
	plainTestRequire(t, reflect.DeepEqual(before, plainTestQuery(t, path, nil)), "custom refusal mutated metadata")
}
func plainTestProvenanceOnly(w plainTestWitness) bool {
	value, present := w.Attrs[hex.EncodeToString([]byte("com.apple.provenance"))]
	return len(w.Attrs) == 0 || (len(w.Attrs) == 1 && present && len(value) <= 512)
}
func TestPlainExactFileDarwinNaturalStageMismatch(t *testing.T) {
	root := os.Getenv("TEST_PLAIN_CONTROL_ROOT")
	plainTestRequire(t, plainTestAllowed(root), "missing NEW TEST natural mismatch fixture")
	path := filepath.Join(root, "natural-provenance-mismatch", "target")
	before := plainTestQuery(t, path, nil)
	body, err := os.ReadFile(path)
	plainTestMust(t, err)
	file := plainTestBegin(t, path)
	desired := bytes.Repeat([]byte("changed\n"), 1<<20)
	stages, stop := plainTestWatch(t, filepath.Dir(path), "target")
	err = file.Apply(desired)
	t.Logf("Apply mismatch result before independent stage assertions: err=%v effect=%s", err, file.Effect())
	stage := plainTestStage(t, stages, stop, path, desired)
	plainTestRequire(t, plainTestProvenanceOnly(before) && plainTestProvenanceOnly(stage) && !reflect.DeepEqual(before.Attrs, stage.Attrs), "actual natural provenance mismatch witness missing")
	a, b := plainTestMetadata(before), plainTestMetadata(stage)
	a.Attrs, b.Attrs = nil, nil
	plainTestRequire(t, reflect.DeepEqual(a, b), "mismatch control has unrelated metadata differences")
	plainTestRequire(t, err != nil, "natural mismatching stage published")
	plainTestBytes(t, path, body)
	plainTestRequire(t, reflect.DeepEqual(before, plainTestQuery(t, path, nil)), "mismatch refusal mutated original")
}
