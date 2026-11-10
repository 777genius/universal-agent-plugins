package commands

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestWritePrivateDetailedPreservesOrderAndAllowlist(t *testing.T) {
	r := report.New("validate", "test")
	r.Root = "/private/input/secret"
	r.Findings = []report.Finding{{Code: "capability_missing", Layer: "normative", Severity: "warning"}}
	r.Components = []report.Component{{ID: "server", Type: "mcp", Status: report.Pass, Requirements: []string{"stdio"}}}
	r.Capabilities = &readiness.Capabilities{
		Commands: []string{"validate", "inspect"},
		Schemas:  []readiness.Schema{{ID: "standard", Digest: "digest"}},
	}
	r.Clients = []planner.ClientCompatibility{{ClientID: "codex", Limitations: []string{"static_only"}}}
	r.Error = &report.Error{Code: "input_unavailable", Action: "Review settings."}
	var out bytes.Buffer
	if err := writePrivateDetailed(&out, r); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"validate: readiness not_evaluated; conformance not_evaluated; host not_evaluated; compatibility not_evaluated; toolchain not_evaluated; runtime not_evaluated",
		"warning: capability_missing (normative)",
		"mcp server: pass; requirements stdio",
		"commands: validate, inspect",
		"schema: standard; digest",
		"target codex: static support only; static_only",
		"Review settings.",
	}, "\n") + "\n"
	if got := out.String(); got != want {
		t.Fatalf("detailed private output changed:\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(out.String(), r.Root) {
		t.Fatal("raw private root disclosed")
	}
}

type privateDetailedFailingWriter struct{}

func (privateDetailedFailingWriter) Write([]byte) (int, error) {
	return 0, errors.New("writer refused")
}

func TestWritePrivateDetailedPreservesHeaderFailureExit(t *testing.T) {
	err := writePrivateDetailed(privateDetailedFailingWriter{}, report.New("validate", "test"))
	if err == nil || exitx.Code(err) != 1 || !strings.Contains(err.Error(), "authoring output failed") {
		t.Fatalf("unexpected write failure: %v", err)
	}
}

func TestPrivateHumanStatusSummaryParity(t *testing.T) {
	r := report.New("validate", "test")
	r.Readiness.Status = report.Fail
	r.Toolchain.Status = report.Pass
	var detailed, dev bytes.Buffer
	if err := writePrivateDetailed(&detailed, r); err != nil {
		t.Fatal(err)
	}
	if err := writePrivateHuman(&dev, r); err != nil {
		t.Fatal(err)
	}
	firstLine := func(s string) string {
		return strings.SplitN(s, "\n", 2)[0]
	}
	if got, want := firstLine(detailed.String()), firstLine(dev.String()); got != want {
		t.Fatalf("private renderers disagree: detailed %q, dev %q", got, want)
	}
}

type failPrivateDetailedAfterHeader struct{ writes int }

func (w *failPrivateDetailedAfterHeader) Write(b []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, errors.New("capability writer refused")
	}
	return len(b), nil
}

func TestWritePrivateDetailedPreservesCapabilityWriterFailure(t *testing.T) {
	r := report.New("capabilities", "test")
	r.Capabilities = &readiness.Capabilities{Commands: []string{"validate"}}
	w := &failPrivateDetailedAfterHeader{}
	err := writePrivateDetailed(w, r)
	if err == nil || exitx.Code(err) != 1 || !strings.Contains(err.Error(), "capability writer refused") {
		t.Fatalf("unexpected capability write error: %v", err)
	}
}
