//go:build linux

package pluginkitairepo_test

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type geminiCompleteArchiveFaultFile struct {
	*os.File
	problem              error
	writeFailure, closed bool
}

func (file *geminiCompleteArchiveFaultFile) Write(body []byte) (int, error) {
	if file.writeFailure {
		return 0, file.problem
	}
	return file.File.Write(body)
}

func (file *geminiCompleteArchiveFaultFile) Close() error {
	file.closed = true
	err := file.File.Close()
	if !file.writeFailure {
		return errors.Join(err, file.problem)
	}
	return err
}

func TestGeminiCycleCompleteRealArchiveWriteCloseAndExposeFailures(t *testing.T) {
	problem := errors.New("inert owned archive failure")
	for _, kind := range []string{"valid", "write", "close", "before-expose", "final-collision", "partial-collision"} {
		t.Run(kind, func(t *testing.T) {
			output := t.TempDir()
			partial, final := filepath.Join(output, ".research.zip.partial"), filepath.Join(output, "research.zip")
			files := map[string][]byte{"inert-note.txt": []byte("inert archive mechanics; no completion assertion\n")}
			if kind == "final-collision" || kind == "partial-collision" {
				path := final
				if kind == "partial-collision" {
					path = partial
				}
				if err := os.WriteFile(path, []byte("caller-owned"), 0600); err != nil {
					t.Fatal(err)
				}
				err := geminiCycleWriteArchive(output, files, nil)
				body, readErr := os.ReadFile(path)
				if err == nil || readErr != nil || string(body) != "caller-owned" {
					t.Fatal("archive replaced an existing output", err, readErr)
				}
				if kind == "final-collision" {
					if _, err := os.Lstat(partial); !os.IsNotExist(err) {
						t.Fatal("failed expose retained partial", err)
					}
				}
				return
			}
			var err error
			if kind == "write" || kind == "close" {
				file, openErr := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if openErr != nil {
					t.Fatal(openErr)
				}
				fault := &geminiCompleteArchiveFaultFile{File: file, problem: problem, writeFailure: kind == "write"}
				err = geminiCycleWriteOpenedArchive(output, partial, fault, files, nil)
				if !fault.closed || !errors.Is(err, problem) {
					t.Fatal("real owned write/close failure disappeared", err)
				}
			} else {
				check := func() error { return nil }
				if kind == "before-expose" {
					check = func() error { return problem }
				}
				err = geminiCycleWriteArchive(output, files, check)
			}
			if kind == "valid" {
				if err != nil {
					t.Fatal(err)
				}
				actual := geminiCompleteContractReadArchive(t, final)
				if !bytes.Equal(actual["inert-note.txt"], files["inert-note.txt"]) {
					t.Fatal("real closed archive changed bytes")
				}
			} else {
				if err == nil {
					t.Fatal("archive error was ignored")
				}
				if _, err := os.Lstat(final); !os.IsNotExist(err) {
					t.Fatal("archive failure exposed final ZIP", err)
				}
			}
			if _, err := os.Lstat(partial); !os.IsNotExist(err) {
				t.Fatal("owned partial survived completion/failure", err)
			}
		})
	}
}

func geminiCompleteContractReadArchive(t *testing.T, path string) map[string][]byte {
	t.Helper()
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := archive.Close(); err != nil {
			t.Error(err)
		}
	}()
	files := map[string][]byte{}
	for _, entry := range archive.File {
		stream, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, (1<<20)+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || len(body) > 1<<20 || files[entry.Name] != nil {
			t.Fatal("real ZIP body/CRC/duplicate refusal", readErr, closeErr)
		}
		files[entry.Name] = body
	}
	return files
}

func TestGeminiCycleCompleteLateArchiveFailureCannotPass(t *testing.T) {
	if mode := os.Getenv("UAP_GEMINI_ARCHIVE_CHILD"); mode != "" {
		geminiCompleteContractArchiveChild(t, mode)
		return
	}
	for _, mode := range []string{"test-failure", "expired-work", "expired-total"} {
		t.Run(mode, func(t *testing.T) {
			output := t.TempDir()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestGeminiCycleCompleteLateArchiveFailureCannotPass$", "-test.count=1")
			command.Env = []string{"PATH=/usr/bin:/bin", "UAP_GEMINI_ARCHIVE_CHILD=" + mode, "UAP_GEMINI_ARCHIVE_OUTPUT=" + output}
			log, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || ctx.Err() != nil || !bytes.Contains(log, []byte("FAIL: TestGeminiCycleCompleteLateArchiveFailureCannotPass")) {
				t.Fatalf("late archive failure did not fail its actual Go child: %v: %s", err, log)
			}
			files := geminiCompleteContractReadArchive(t, filepath.Join(output, "research.zip"))
			result := geminiCompleteContractDecode(t, files["runtime-attempt.json"])
			if len(files) != 57 || result["runtime_attempt_complete"] != false || result["test_failed"] != true || result["capture_mode"] != "gemini-complete-cycle-first-capture" || result["cycle_capture_review_required"] != true || result["vendor_authentication_qualified"] != false || result["real_model_qualified"] != false {
				t.Fatal("late failure serialized an untruthful completion pair")
			}
			for _, value := range result["qualification"].(map[string]any) {
				if value != false {
					t.Fatal("late failure retained qualification")
				}
			}
			if mode != "test-failure" && result["cycle_observed_complete"] != false {
				t.Fatal("expired origin retained complete observation")
			}
			identity := result["source_identity"].(map[string]any)
			input := geminiCompleteContractEncode(t, map[string]any{"files": files, "source": map[string]any{"source_commit": identity["installer_base_commit"], "source_tree": identity["installer_tree"]}})
			const code = `import base64,json,sys
import project_native_runtime as runtime
value=json.load(sys.stdin)
bodies={k:base64.b64decode(v,validate=True) for k,v in value["files"].items()}
r=runtime.attempt_members(bodies,value["source"],"gemini")
assert r["test_failed"] is True and r["runtime_attempt_complete"] is False
assert all(v is False for v in r["qualification"].values())
print("truthful failed synthetic archive admitted by preserved reader")`
			if body := geminiCompleteContractPython(t, code, input); !strings.Contains(string(body), "truthful failed synthetic archive") {
				t.Fatal("preserved failed reader did not confirm late archive")
			}
		})
	}
}

func geminiCompleteContractArchiveChild(t *testing.T, mode string) {
	t.Helper()
	files, source := geminiCompleteSyntheticFiles(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &geminiProjectAttempt{ctx: ctx, started: time.Now(), files: files, evidence: geminiCompleteContractDecode(t, files["runtime-attempt.json"]), output: os.Getenv("UAP_GEMINI_ARCHIVE_OUTPUT"), secret: "inert-unused-archive-key"}
	c := &geminiProjectCycle{f: f, observed: true, stage: "complete-cycle", public: append([]string{}, geminiCyclePublic...), listings: append([]string{}, geminiCycleListings...), natives: append([]string{}, geminiCycleNative...)}
	switch mode {
	case "test-failure":
		t.Error("expected inert late test failure")
	case "expired-work":
		cancel()
	case "expired-total":
		f.started = time.Now().Add(-181 * time.Second)
	default:
		t.Fatal("unknown inert archive child mode")
	}
	c.archive(t, source)
}
