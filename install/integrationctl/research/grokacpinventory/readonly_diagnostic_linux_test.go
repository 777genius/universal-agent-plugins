//go:build linux

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestReadonlySupervisedCapturePreservesOnlyActualExchangeGuard(t *testing.T) {
	r := inertRequest()
	var body map[string]any
	if err := json.Unmarshal(inertResult(t, r), &body); err != nil {
		t.Fatal(err)
	}
	body["mode"] = readonlyMode
	body["readonly"] = map[string]any{"authority": map[string]any{}, "stage": "runtime",
		"control_complete": false, "original": nil, "control_marker": nil}
	valid, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	valid = append(valid, '\n')
	for _, mode := range []string{readonlyMode, "", sessionMode, lockMode, cliMode} {
		r.Mode = mode
		for _, item := range []struct {
			name, code, supervisor string
			frame                  []byte
		}{
			{"partial", "collector-frame-refused", "PRIVATE supervisor", []byte("PRIVATE partial")},
			{"duplicate-key", "json-key-refused", "PRIVATE supervisor", []byte("{\"schema_version\":1,\"schema_version\":1}\n")},
			{"identity", "result-identity-refused", "PRIVATE supervisor", bytes.Replace(valid, []byte(`"scope":"a"`), []byte(`"scope":"b"`), 1)},
			{"unknown-decoder", "strict-cleanup-unproved", "PRIVATE supervisor", []byte("{\"PRIVATE\":}\n")},
			{"runner-only", "strict-cleanup-unproved", "result-identity-refused", valid},
		} {
			t.Run(mode+"/"+item.name, func(t *testing.T) {
				called := false
				raw, refusal := supervisedCapture(t.Context(), ports.Command{}, r, 1,
					func(_ context.Context, _ ports.Command, capture func(io.Writer, io.Reader) error) error {
						called = true
						return errors.Join(capture(io.Discard, bytes.NewReader(item.frame)), errors.New(item.supervisor))
					})
				want, diagnostic := "strict-cleanup-unproved", "grok-acp-controller-refused"
				if mode == readonlyMode {
					diagnostic += ":" + item.code
					if item.code != want {
						want = item.code + "\n" + want
					}
				}
				if !called || raw != nil || refusal == nil || refusal.Error() != want {
					t.Fatal("capture lost its failure or released raw", refusal)
				}
				if got := controllerRefusal(readonlyDiagnostic(mode, refusal)); got != diagnostic {
					t.Fatal("controller lost the closed guard or emitted private text", got)
				}
			})
		}
	}
}

func TestReadonlyDiagnosticPreservesActualInputRefusal(t *testing.T) {
	path, root := readonlyTestRoot(t)
	foreign := filepath.Join(t.TempDir(), "PRIVATE-foreign")
	if err := os.WriteFile(foreign, []byte("PRIVATE bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(foreign, filepath.Join(path, "alias")); err != nil {
		t.Fatal(err)
	}
	_, _, refusal := readonlyInput(t.Context(), root, os.Getuid(), os.Getgid())
	if refusal == nil || refusal.Error() != "readonly-input-authority" {
		t.Fatal("actual alias input did not reach the expected guard", refusal)
	}
	diagnostic := readonlyDiagnostic(readonlyMode, errors.Join(refusal, errors.New("PRIVATE cleanup path")))
	if !errors.Is(diagnostic, refusal) || controllerRefusal(diagnostic) != "grok-acp-controller-refused:readonly-input-authority" {
		t.Fatal("original refusal lost its error or diagnostic", diagnostic)
	}
	if body, err := os.ReadFile(foreign); err != nil || string(body) != "PRIVATE bytes" {
		t.Fatal("diagnostic changed foreign bytes", err)
	}
	if _, err := os.Lstat(filepath.Join(path, "alias")); err != nil {
		t.Fatal("diagnostic cleaned unproved input", err)
	}
}

func TestReadonlyDiagnosticNeverEmitsUnknownErrorText(t *testing.T) {
	for _, text := range []string{"PRIVATE/path", "", "readonly-input-authority PRIVATE", "readonly-input-authority readonly-input-proof", "readonly-input-authority\n"} {
		refusal := errors.New(text)
		got := readonlyDiagnostic(readonlyMode, refusal)
		if !errors.Is(got, refusal) || controllerRefusal(got) != "grok-acp-controller-refused:readonly-helper-unclassified" {
			t.Fatal("unrecognized exception entered the diagnostic", got)
		}
	}
	refusal := errors.New("held-file-changed")
	joined := fmt.Errorf("PRIVATE wrapper: %w", errors.Join(refusal, errors.New("PRIVATE second error")))
	if got := controllerRefusal(readonlyDiagnostic(readonlyMode, joined)); got != "grok-acp-controller-refused:held-file-changed" {
		t.Fatal("nested fixed guard lost", got)
	}
}

func TestReadonlyDiagnosticLeavesOlderModesAndSuccessUnchanged(t *testing.T) {
	for _, mode := range []string{"", cliMode, lockMode, sessionMode} {
		refusal := errors.New("readonly-input-authority")
		if got := readonlyDiagnostic(mode, refusal); !errors.Is(got, refusal) || controllerRefusal(got) != "grok-acp-controller-refused" {
			t.Fatal("older mode changed", mode, got)
		}
	}
	if readonlyDiagnostic(readonlyMode, nil) != nil {
		t.Fatal("success became a failure")
	}
	seen := map[string]bool{}
	for _, code := range strings.Fields(readonlyDiagnosticCodes) {
		if seen[code] || strings.ContainsAny(code, "\r\n:\t ") {
			t.Fatal("ambiguous diagnostic enum", code)
		}
		seen[code] = true
	}
}

func TestReadonlyPrimitiveRefusalClassifiesActualMissingExecutable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PRIVATE-missing-bwrap")
	held, cause := systemFile(t.Context(), path)
	if held.file != nil || !errors.Is(cause, os.ErrNotExist) {
		t.Fatal("actual missing executable did not retain its filesystem error", cause)
	}
	refusal := readonlyDiagnostic(readonlyMode, readonlyPrimitiveRefusal(cause))
	if !errors.Is(refusal, cause) || !errors.Is(refusal, os.ErrNotExist) {
		t.Fatal("missing executable lost its original error", refusal)
	}
	if got := controllerRefusal(refusal); got != "grok-acp-controller-refused:readonly-primitive-missing" {
		t.Fatal("missing executable diagnostic is not closed", got)
	}
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("diagnostic changed the missing input", err)
	}
}

func TestReadonlyPrimitiveRefusalPreservesAuthorityAndPrivateErrors(t *testing.T) {
	permission := &os.PathError{Op: "open", Path: "/PRIVATE/tool", Err: syscall.EACCES}
	for _, item := range []struct {
		name, code string
		cause      error
	}{
		{"permission", "readonly-primitive-permission", fmt.Errorf("PRIVATE wrapper: %w", permission)},
		{"operation-not-permitted", "readonly-primitive-permission", &os.PathError{Op: "open", Path: "/PRIVATE/tool", Err: syscall.EPERM}},
		{"unknown", "readonly-primitive-refused", errors.New("PRIVATE filesystem failure")},
		{"not-directory", "readonly-primitive-refused", &os.PathError{Op: "lstat", Path: "/PRIVATE/tool", Err: syscall.ENOTDIR}},
		{"canceled", "readonly-primitive-refused", context.Canceled},
		{"deadline", "readonly-primitive-refused", context.DeadlineExceeded},
		{"existing-authority", "file-authority-refused", errors.Join(errors.New("file-authority-refused"), permission)},
		{"existing-ancestor", "system-ancestor-refused", errors.New("system-ancestor-refused")},
	} {
		t.Run(item.name, func(t *testing.T) {
			refusal := readonlyDiagnostic(readonlyMode, readonlyPrimitiveRefusal(item.cause))
			if !errors.Is(refusal, item.cause) {
				t.Fatal("primitive classification lost the original error", refusal)
			}
			if got := controllerRefusal(refusal); got != "grok-acp-controller-refused:"+item.code {
				t.Fatal("primitive classification changed a guard or disclosed private text", got)
			}
		})
	}
}
