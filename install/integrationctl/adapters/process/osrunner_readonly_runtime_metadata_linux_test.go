//go:build linux && grokreadonlynative

package process

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

var runtimeMetadataOutput = flag.String("runtime-metadata-output", "", "exclusive output for nonqualifying /usr metadata diagnostic")

// This report never supplies RuntimeProof. It reads no regular-file contents or
// link targets and does not normalize any discovered member.
type runtimeMetadataReport struct {
	MetadataOnly  bool     `json:"metadata_only"`
	Qualification bool     `json:"qualification"`
	Complete      bool     `json:"complete"`
	Truncated     bool     `json:"truncated"`
	Visited       int      `json:"visited"`
	Refusals      int      `json:"refusals"`
	Rows          []string `json:"rows"`
	Error         string   `json:"error,omitempty"`
}

type runtimeMetadataWalker struct {
	validator readonlyRuntimeCensus
	report    runtimeMetadataReport
}

func (walk *runtimeMetadataWalker) observe(path string, facts readonlyObjectFacts) error {
	walk.report.Visited++
	if walk.report.Visited > readonlyRuntimeEntryLimit {
		return errors.New("diagnostic runtime entry bound exceeded")
	}
	if _, err := walk.validator.memberFacts(path, facts); err != nil {
		walk.report.Refusals++
		if len(walk.report.Rows) < 64 {
			walk.report.Rows = append(walk.report.Rows, readonlyRuntimeMemberError(path, err).Error())
		} else {
			walk.report.Truncated = true
		}
	}
	return nil
}

func (walk *runtimeMetadataWalker) directory(ctx context.Context, file *os.File, path string, before readonlyObjectFacts, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 32 {
		return errors.New("diagnostic runtime depth exceeded")
	}
	if err := requireReadonlyHeld(file, before); err != nil {
		return err
	}
	if err := walk.observe(path, before); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries, err := file.ReadDir(32)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		for _, entry := range entries {
			if err := walk.entry(ctx, file, path, entry.Name(), depth); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			break
		}
		if len(entries) == 0 {
			return errors.New("diagnostic directory read made no progress")
		}
	}
	return errors.Join(requireReadonlyHeld(file, before), requireReadonlyMount(file, "", unix.AT_EMPTY_PATH, walk.validator.mount), ctx.Err())
}

func (walk *runtimeMetadataWalker) entry(ctx context.Context, parent *os.File, path, name string, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if name == "" || name == "." || name == ".." || !utf8.ValidString(name) || strings.ContainsAny(name, "/\x00\r\n") {
		return errors.New("diagnostic member name differs")
	}
	path = filepath.Join(path, name)
	if !readonlyAbsolute(path) || !readonlyWithin(path, "/usr") {
		return errors.New("diagnostic member escaped fixed runtime closure")
	}
	if err := requireReadonlyMount(parent, name, 0, walk.validator.mount); err != nil {
		return err
	}
	before, err := readonlyNamedFacts(parent, name)
	if err != nil {
		return err
	}
	if before.Mode&unix.S_IFMT == unix.S_IFDIR {
		child, openErr := openReadonlyDirectory(parent, name)
		if openErr != nil {
			return openErr
		}
		visitErr := walk.directory(ctx, child, path, before, depth+1)
		err = errors.Join(visitErr, child.Close())
	} else {
		err = walk.observe(path, before)
	}
	if err != nil {
		return err
	}
	return errors.Join(requireReadonlyNamed(parent, name, before), requireReadonlyMount(parent, name, 0, walk.validator.mount))
}

func (report *runtimeMetadataReport) encode() ([]byte, error) {
	for {
		body, err := json.Marshal(report)
		if err != nil {
			return nil, err
		}
		if len(body)+1 <= 64<<10 {
			return append(body, '\n'), nil
		}
		if len(report.Rows) == 0 {
			return nil, errors.New("diagnostic report exceeds bound")
		}
		report.Rows = report.Rows[:len(report.Rows)-1]
		report.Truncated = true
	}
}

// finish is shared by the fixed-root diagnostic and its small bound controls.
// An incomplete traversal never supplies a complete metadata observation.
func (report *runtimeMetadataReport) finish(err error) {
	report.Complete = err == nil
	if err != nil {
		report.Error = fmt.Sprintf("metadata traversal incomplete: %.1024s", err)
	}
}

func TestReadonlyRuntimeMetadataDiagnostic(t *testing.T) {
	output := *runtimeMetadataOutput
	if !filepath.IsAbs(output) || filepath.Clean(output) != output {
		t.Fatal("original absolute diagnostic output required")
	}
	walk := runtimeMetadataWalker{validator: readonlyRuntimeCensus{uid: 0, gid: 0}, report: runtimeMetadataReport{MetadataOnly: true}}
	defer func() {
		body, err := walk.report.encode()
		if err != nil {
			t.Error(err)
			return
		}
		file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Error(err)
			return
		}
		n, writeErr := file.Write(body)
		if err := errors.Join(writeErr, file.Close()); err != nil || n != len(body) {
			t.Error("diagnostic output incomplete", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	root, err := os.Open("/")
	if err != nil {
		walk.report.Error = "open fixed root"
		t.Fatal(err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	}()
	file, err := openReadonlyDirectory(root, "usr")
	if err != nil {
		walk.report.Error = "open fixed runtime"
		t.Fatal(err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error(err)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		walk.report.Error = "stat fixed runtime"
		t.Fatal(err)
	}
	before, err := readonlyFacts(info)
	if err != nil {
		walk.report.Error = "decode fixed runtime metadata"
		t.Fatal(err)
	}
	walk.validator.mount, err = readonlyMountID(file, "", unix.AT_EMPTY_PATH)
	if err == nil {
		err = requireReadonlyMount(root, "usr", 0, walk.validator.mount)
	}
	if err == nil {
		err = walk.directory(ctx, file, "/usr", before, 0)
	}
	if err == nil {
		err = requireReadonlyNamed(root, "usr", before)
	}
	walk.report.finish(err)
	if err != nil {
		t.Fatal("nonqualifying metadata diagnostic incomplete", err)
	}
	t.Logf("metadata diagnostic only: visited=%d refusals=%d truncated=%t; not runtime qualification", walk.report.Visited, walk.report.Refusals, walk.report.Truncated)
}

func TestReadonlyRuntimeMetadataDiagnosticControls(t *testing.T) {
	base, file, census, _ := readonlyRuntimeFixture(t)
	directory := filepath.Join(base, "writable")
	if err := os.Mkdir(directory, 0777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(directory, 0777); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "child")
	if err := os.WriteFile(path, []byte("PRIVATE fixture bytes must never enter metadata"), 0666); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0666); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "unvisited"), []byte("EXTERNAL private fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "external")); err != nil {
		t.Fatal(err)
	}
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	facts, err := readonlyFacts(info)
	if err != nil {
		t.Fatal(err)
	}
	census.mount, err = readonlyMountID(file, "", unix.AT_EMPTY_PATH)
	if err != nil {
		t.Fatal(err)
	}
	walk := runtimeMetadataWalker{validator: census, report: runtimeMetadataReport{MetadataOnly: true}}
	if err := walk.directory(t.Context(), file, "/usr", facts, 0); err != nil {
		t.Fatal(err)
	}
	body, err := walk.report.encode()
	if err != nil {
		t.Fatal(err)
	}
	if walk.report.Refusals != 2 || !strings.Contains(string(body), "writable/child") || strings.Contains(string(body), "PRIVATE") || strings.Contains(string(body), "EXTERNAL") || strings.Contains(string(body), "unvisited") {
		t.Fatal("metadata traversal lost nested refusal or followed contents/link", string(body))
	}
	for range 128 {
		walk.report.Rows = append(walk.report.Rows, strings.Repeat("bounded metadata ", 128))
	}
	body, err = walk.report.encode()
	if err != nil || len(body) > 64<<10 || !walk.report.Truncated || walk.report.Qualification {
		t.Fatal("diagnostic bounds granted authority", len(body), err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := walk.directory(ctx, file, "/usr", facts, 0); !errors.Is(err, context.Canceled) {
		t.Fatal("diagnostic ignored cancellation", err)
	}
}

// Seed only the counter: each case creates at most one directory and one leaf,
// never a large runtime tree or an observation of the host's /usr contents.
func TestReadonlyRuntimeMetadataDiagnosticVisitBound(t *testing.T) {
	for _, test := range []struct {
		name     string
		seed     int
		nested   bool
		leaf     bool
		complete bool
	}{
		{name: "exact limit", seed: readonlyRuntimeEntryLimit - 1, complete: true},
		{name: "root overflow", seed: readonlyRuntimeEntryLimit},
		{name: "leaf overflow", seed: readonlyRuntimeEntryLimit - 1, leaf: true},
		{name: "nested overflow", seed: readonlyRuntimeEntryLimit - 2, nested: true, leaf: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			base := t.TempDir()
			parent := base
			if test.nested {
				parent = filepath.Join(base, "nested")
				if err := os.Mkdir(parent, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if test.leaf {
				if err := os.WriteFile(filepath.Join(parent, "leaf"), []byte("contents must not be read"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			file, err := os.Open(base)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := file.Close(); err != nil {
					t.Error(err)
				}
			})
			info, err := file.Stat()
			if err != nil {
				t.Fatal(err)
			}
			facts, err := readonlyFacts(info)
			if err != nil {
				t.Fatal(err)
			}
			census := readonlyRuntimeCensus{uid: uint32(os.Getuid()), gid: uint32(os.Getgid())}
			census.mount, err = readonlyMountID(file, "", unix.AT_EMPTY_PATH)
			if err != nil {
				t.Fatal(err)
			}
			walk := runtimeMetadataWalker{validator: census, report: runtimeMetadataReport{MetadataOnly: true, Visited: test.seed}}
			err = walk.directory(t.Context(), file, "/usr", facts, 0)
			if test.complete && err != nil || !test.complete && (err == nil || !strings.Contains(err.Error(), "diagnostic runtime entry bound exceeded")) {
				t.Fatalf("entry-bound propagation: complete=%t, err=%v", test.complete, err)
			}
			walk.report.finish(err)
			wanted := readonlyRuntimeEntryLimit
			if !test.complete {
				wanted++
			}
			if walk.report.Visited != wanted || walk.report.Complete != test.complete || walk.report.Qualification || !walk.report.MetadataOnly {
				t.Fatalf("entry-bound report differs: %+v", walk.report)
			}
			body, err := walk.report.encode()
			if err != nil {
				t.Fatal(err)
			}
			var reported runtimeMetadataReport
			if err := json.Unmarshal(body, &reported); err != nil {
				t.Fatal(err)
			}
			if reported.Complete && reported.Visited > readonlyRuntimeEntryLimit || reported.Complete != test.complete || reported.Qualification || !test.complete && reported.Error == "" {
				t.Fatalf("encoded bound failure granted completeness: %s", body)
			}
		})
	}
}
