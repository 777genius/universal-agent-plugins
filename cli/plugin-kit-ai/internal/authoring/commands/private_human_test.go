package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
)

func TestWritePrivateHumanRendersMaintenanceDetails(t *testing.T) {
	r := report.New("normalize", "test")
	r.JSONDocument = &report.JSONDocument{Path: "plugin.json", BeforeSHA256: "before", AfterSHA256: "after", Changed: true}
	r.NativeImport = &report.NativeImport{Client: "claude", SourceSHA256: "source", SafeServers: 2, SkippedServers: []report.ImportIssue{{Code: "server_skipped"}}, UnsupportedTopLevel: []report.ImportIssue{{Code: "unsupported_top_level_field"}}}
	r.Committed = true
	r.Paths = []string{"plugin.json", "skills/demo/SKILL.md", "../private"}
	var out bytes.Buffer
	if err := writePrivateHuman(&out, r); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"json document: plugin.json; changed true; before before; after after",
		"native import: client claude; source source; safe servers 2; skipped servers 1; unsupported top-level fields 1",
		"committed: true",
		"affected path: plugin.json",
		"affected path: skills/demo/SKILL.md",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("output missing %q: %s", want, text)
		}
	}
	if strings.Contains(text, "../private") {
		t.Fatalf("non-local path disclosed: %s", text)
	}
}
