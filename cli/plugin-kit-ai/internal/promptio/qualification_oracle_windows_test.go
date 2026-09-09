//go:build windows

package promptio

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func qualificationCloseFile(t *testing.T, f *os.File) {
	t.Helper()
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Errorf("fixture close: %v", err)
		}
	})
}

func qualificationRequireOps(t *testing.T, q *qualificationHandleTrace, want ...string) {
	t.Helper()
	if err := q.verdict(); err != nil {
		t.Fatal(err)
	}
	for _, op := range want {
		count := 0
		for _, e := range q.records[:q.count] {
			if e.operation == op {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("expected one %s, got %d", op, count)
		}
	}
}

// Mutate actual completed traces. Every single deleted/duplicated transition,
// every wrong generation and owned identity must fail the qualification verdict.
func qualificationMutations(t *testing.T, source *qualificationHandleTrace) {
	t.Helper()
	check := func(label string, records []qualificationHandleRecord, generations int, overflow bool) {
		t.Helper()
		q := &qualificationHandleTrace{count: len(records), generation: generations, overflow: overflow}
		copy(q.records[:], records)
		if err := q.verdict(); err == nil {
			t.Fatalf("accepted %s", label)
		}
	}
	records := source.records[:source.count]
	check("empty", nil, 0, false)
	check("missing entire registered request", nil, 1, false)
	check("unstarted registered generation", records, source.generation+1, false)
	check("overflow", records, source.generation, true)
	for i, event := range records {
		// Native character reads and repeated cancel attempts are legitimately optional.
		if event.operation == "native-read-complete" || (event.operation == "console-cancel" || event.operation == "thread-cancel") {
			continue
		}
		deleted := append([]qualificationHandleRecord{}, records[:i]...)
		deleted = append(deleted, records[i+1:]...)
		check("missing "+event.operation, deleted, source.generation, false)
		duplicate := append([]qualificationHandleRecord{}, records[:i]...)
		duplicate = append(duplicate, event)
		duplicate = append(duplicate, records[i:]...)
		check("duplicate "+event.operation, duplicate, source.generation, false)
		changed := append([]qualificationHandleRecord{}, records...)
		changed[i].operation = "unknown-transition"
		check("unknown transition", changed, source.generation, false)
		if event.operation == "worker-start" || event.operation == "thread-acquire" || event.operation == "thread-close" {
			changed = append([]qualificationHandleRecord{}, records...)
			changed[i].threadID = 0
			check("invalid worker TID", changed, source.generation, false)
		}
		for _, generation := range []int{0, source.generation + 1} {
			changed := append([]qualificationHandleRecord{}, records...)
			changed[i].generation = generation
			check("wrong generation "+event.operation, changed, source.generation, false)
		}
		if event.handle != 0 && event.err == nil && event.operation != "branch-file" {
			for _, h := range []windows.Handle{0, windows.InvalidHandle, event.handle + 1} {
				changed := append([]qualificationHandleRecord{}, records...)
				changed[i].handle = h
				check("wrong handle "+event.operation, changed, source.generation, false)
			}
		}
	}
	var noAttempts []qualificationHandleRecord
	for _, event := range records {
		if event.operation != "console-cancel" && event.operation != "thread-cancel" {
			noAttempts = append(noAttempts, event)
		}
	}
	if len(noAttempts) != len(records) {
		check("missing all native cancellation completions", noAttempts, source.generation, false)
	}
	for _, kind := range []string{"console", "thread"} {
		var filtered []qualificationHandleRecord
		for _, event := range records {
			if event.operation != kind+"-acquire" && event.operation != kind+"-close" {
				filtered = append(filtered, event)
			}
		}
		if len(filtered) != len(records) {
			check("missing "+kind+" pair", filtered, source.generation, false)
		}
	}
}

func TestQualificationOracleStreamBranches(t *testing.T) {
	for _, branch := range []string{"reader", "reader-error", "file", "pipe", "pre-cancel", "duplicate-failure"} {
		t.Run(branch, func(t *testing.T) {
			q := new(qualificationHandleTrace)
			ctx := q.context(context.Background())
			var r io.Reader = strings.NewReader("answer\n")
			var writer *os.File
			switch branch {
			case "reader-error":
				r = qualificationErrorReader{}
			case "file", "pre-cancel", "duplicate-failure":
				f, err := os.CreateTemp(t.TempDir(), "input")
				if err != nil {
					t.Fatal(err)
				}
				qualificationCloseFile(t, f)
				if _, err := f.WriteString("answer\n"); err != nil {
					t.Fatal(err)
				}
				if _, err := f.Seek(0, 0); err != nil {
					t.Fatal(err)
				}
				r = f
			case "pipe":
				f, w, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				qualificationCloseFile(t, f)
				qualificationCloseFile(t, w)
				writer = w
				if _, err := w.WriteString("answer\n"); err != nil {
					t.Fatal(err)
				}
				r = f
			}
			if branch == "pre-cancel" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if branch == "duplicate-failure" {
				ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps).duplicate = func(windows.Handle, windows.Handle, windows.Handle, *windows.Handle, uint32, bool, uint32) error {
					return windows.ERROR_ACCESS_DENIED
				}
			}
			line, err := ReadLine(ctx, r)
			switch branch {
			case "reader-error":
				if !errors.Is(err, io.ErrUnexpectedEOF) {
					t.Fatal(err)
				}
			case "pre-cancel":
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case "duplicate-failure":
				if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
					t.Fatal(err)
				}
			default:
				if err != nil || line != "answer" {
					t.Fatalf("%q %v", line, err)
				}
			}
			qualificationRequireOps(t, q, "request-start", "request-end")
			if f, ok := r.(*os.File); ok {
				var flags uint32
				if err := qualificationGetHandleInformation(windows.Handle(f.Fd()), &flags); err != nil {
					t.Fatalf("borrowed input closed: %v", err)
				}
			}
			if f, ok := r.(*os.File); ok {
				if writer != nil {
					if _, err := writer.WriteString("answer\n"); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := f.Seek(0, 0); err != nil {
						t.Fatal(err)
					}
				}
				if line, err := readLine(context.Background(), f); err != nil || line != "answer" {
					t.Fatalf("borrowed next-owner read: %q %v", line, err)
				}
			}
			qualificationMutations(t, q)
		})
	}
}

type qualificationErrorReader struct{}

func (qualificationErrorReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func TestQualificationOracleGenerationReuse(t *testing.T) {
	q := new(qualificationHandleTrace)
	for i := 0; i < 2; i++ {
		_, _ = ReadLine(q.context(context.Background()), strings.NewReader("ok\n"))
	}
	if err := q.verdict(); err != nil {
		t.Fatal(err)
	}
	// An event from another valid generation must also fail, not just out-of-range IDs.
	q.records[2].generation = 2
	if err := q.verdict(); err == nil {
		t.Fatal("accepted cross-generation event")
	}
	// A complete stream trace with deliberate numeric reuse in the next generation.
	q = new(qualificationHandleTrace)
	f, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	qualificationCloseFile(t, f)
	_, _ = ReadLine(q.context(context.Background()), f)
	n := q.count
	for _, e := range q.records[:n] {
		e.generation = 2
		q.records[q.count] = e
		q.count++
	}
	q.generation = 2
	if err := q.verdict(); err != nil {
		t.Fatalf("balanced numeric reuse: %v", err)
	}
	for i := n; i < q.count; i++ {
		if q.records[i].operation == "thread-close" {
			q.records[i].generation = 1
			break
		}
	}
	if err := q.verdict(); err == nil {
		t.Fatal("accepted close from old generation")
	}
}

func qualificationConsoleFixture(t *testing.T) *os.File {
	t.Helper()
	h, err := windows.CreateFile(windows.StringToUTF16Ptr("CONIN$"), windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatalf("unmet fixture condition: attached disposable console required: %v", err)
	}
	f := os.NewFile(uintptr(h), "oracle-console")
	qualificationCloseFile(t, f)
	return f
}

func qualificationConsoleBranches(t *testing.T, f *os.File) {
	t.Run("input-validation-failure", func(t *testing.T) {
		q := new(qualificationHandleTrace)
		qualificationConsoleInputClassification(t, windows.Handle(f.Fd()), q)
		qualificationRequireOps(t, q, "input-validate", "request-end")
		qualificationMutations(t, q)
	})
	t.Run("cancel-before-native-read", func(t *testing.T) { qualificationCancellationBeforeRead(t, f) })
	for _, branch := range []string{"success", "eof", "read-error", "open-failure", "post-open-failure", "mode-mismatch", "duplicate-failure"} {
		t.Run(branch, func(t *testing.T) {
			q := new(qualificationHandleTrace)
			ctx := q.context(context.Background())
			ops := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps)
			ops.read = func(context.Context, windows.Handle, uint32) (string, error) {
				if branch == "eof" {
					return "", io.EOF
				}
				if branch == "read-error" {
					return "", windows.ERROR_READ_FAULT
				}
				return "answer", nil
			}
			switch branch {
			case "open-failure":
				ops.open = func() (windows.Handle, error) { return windows.InvalidHandle, windows.ERROR_ACCESS_DENIED }
			case "post-open-failure", "mode-mismatch":
				calls := 0
				ops.mode = func(h windows.Handle, mode *uint32) error {
					calls++
					err := windows.GetConsoleMode(h, mode)
					if calls == 2 {
						if branch == "post-open-failure" {
							return windows.ERROR_INVALID_HANDLE
						}
						*mode ^= windows.ENABLE_LINE_INPUT
					}
					return err
				}
			case "duplicate-failure":
				ops.duplicate = func(windows.Handle, windows.Handle, windows.Handle, *windows.Handle, uint32, bool, uint32) error {
					return windows.ERROR_ACCESS_DENIED
				}
			}
			line, err := ReadLine(ctx, f)
			if branch == "success" {
				if line != "answer" || err != nil {
					t.Fatalf("%q %v", line, err)
				}
			} else if err == nil {
				t.Fatalf("%s returned success", branch)
			}
			qualificationRequireOps(t, q, "branch-console", "input-validate", "console-acquire", "request-end")
			qualificationMutations(t, q)
			var mode uint32
			if err := windows.GetConsoleMode(windows.Handle(f.Fd()), &mode); err != nil {
				t.Fatalf("borrowed console closed: %v", err)
			}
		})
	}
}

var qualificationHandleInformation = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")

const qualificationProtectFromClose = 2

func qualificationGetHandleInformation(h windows.Handle, flags *uint32) error {
	ok, _, err := qualificationHandleInformation.Call(uintptr(h), uintptr(unsafe.Pointer(flags)))
	if ok == 0 {
		return err
	}
	return nil
}
