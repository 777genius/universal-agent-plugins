package agentpluginscli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"strings"
	"syscall"
	"testing"
)

func TestUnavailableDiagnosticUsesTypedFactsWithoutPublishingErrors(t *testing.T) {
	private := "opaque-provider-secret-438275"
	for name, cause := range map[string]error{
		"unknown provider":        errors.New(private),
		"path and permission":     &fs.PathError{Op: "mkdir", Path: "/private/" + private, Err: fs.ErrPermission},
		"URL credentials":         &url.Error{Op: "Get", URL: "https://user:" + private + "@example.test/" + private, Err: errors.New("response " + private)},
		"unlabelled network body": &net.OpError{Op: "dial", Net: "tcp", Err: errors.New(private)},
		"terminal effects":        errors.New("\x1b[2J\r\n" + private),
	} {
		t.Run(name, func(t *testing.T) {
			reports := reportByAgent([]agentItem{failedItem("pdf", "codex", cause)})
			for _, unavailable := range []installDiagnostic{{}, {WriteErr: cause}} {
				var output bytes.Buffer
				if err := renderAgentAttention(&output, reports, "", ""); err != nil {
					t.Fatal(err)
				}
				if err := renderInstallDiagnostic(&output, unavailable, reports); err != nil {
					t.Fatal(err)
				}
				text := output.String()
				if strings.Contains(text, private) || strings.Contains(text, "example.test") || strings.ContainsAny(text, "\x1b\r") {
					t.Fatalf("unknown detail reached terminal: %q", text)
				}
				if !strings.Contains(text, reports[0].Problems[0].Failure.Summary) || !strings.Contains(text, "Failure codes: "+string(failureKindOf(cause))) {
					t.Fatalf("safe classified cause was lost: %q", text)
				}
				if (unavailable.WriteErr != nil) != strings.Contains(text, "could not be saved") {
					t.Fatalf("disabled recording was confused with failed write: %q", text)
				}
				if strings.Contains(text, "hand the technical log") || strings.Contains(text, "Technical details: ") {
					t.Fatalf("unavailable diagnostics advertised a nonexistent file: %q", text)
				}
			}
		})
	}
}

func TestDiagnosticPointerAndSuccessKeepEstablishedOutput(t *testing.T) {
	reports := reportByAgent([]agentItem{failedItem("pdf", "codex", errors.New("private"))})
	var output bytes.Buffer
	if err := renderInstallDiagnostic(&output, installDiagnostic{Path: "/logs/install.jsonl"}, reports); err != nil {
		t.Fatal(err)
	}
	if output.String() != "Technical details: /logs/install.jsonl\n" {
		t.Fatalf("saved pointer changed: %q", output.String())
	}
	output.Reset()
	clean := reportByAgent([]agentItem{{Alias: "pdf", ClientID: "codex", Status: "completed"}})
	if err := renderInstallDiagnostic(&output, installDiagnostic{WriteErr: errors.New("private")}, clean); err != nil || output.Len() != 0 {
		t.Fatalf("successful install gained failure output: %q %v", output.String(), err)
	}
}

func TestDiagnosticWriteFailureExplainsOnlyKnownTypedConditions(t *testing.T) {
	for _, tc := range []struct {
		cause error
		words string
	}{
		{fs.ErrPermission, "cannot write to the log folder"},
		{syscall.ENOSPC, "not enough free disk space"},
		{syscall.EROFS, "log folder is read-only"},
		{errors.New("permission denied disk full read-only"), "Follow the per-agent results"},
	} {
		cause := fmt.Errorf("opaque-write-failure: %w", tc.cause)
		message := diagnosticUnavailableMessage(installDiagnostic{WriteErr: cause})
		if !strings.Contains(message, tc.words) || strings.Contains(message, "opaque-write-failure") {
			t.Fatalf("write failure was not classified safely: %q", message)
		}
	}
}

func TestDiagnosticCodesDeduplicateInObservedAgentOrder(t *testing.T) {
	reports := reportByAgent([]agentItem{
		failedItem("first", "codex", fs.ErrPermission),
		failedItem("second", "claude", fmt.Errorf("private: %w", fs.ErrPermission)),
		failedItem("third", "gemini", errors.New("private")),
	})
	var output bytes.Buffer
	if err := renderInstallDiagnostic(&output, installDiagnostic{}, reports); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Failure codes: PERMISSION_DENIED, FAILED\n") || strings.Count(output.String(), "PERMISSION_DENIED") != 1 {
		t.Fatalf("codes lost observed order or duplicated: %q", output.String())
	}
	for _, diagnostic := range []installDiagnostic{{}, {WriteErr: fs.ErrPermission}, {Path: "/logs/install.jsonl"}} {
		if err := renderInstallDiagnostic(targetNoticeWriter{err: io.ErrClosedPipe}, diagnostic, reports); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("diagnostic output error was lost: %v", err)
		}
		if err := renderInstallDiagnostic(diagnosticShortWriter{}, diagnostic, reports); !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("short diagnostic output was accepted: %v", err)
		}
	}
}

type diagnosticShortWriter struct{}

func (diagnosticShortWriter) Write([]byte) (int, error) { return 0, nil }
