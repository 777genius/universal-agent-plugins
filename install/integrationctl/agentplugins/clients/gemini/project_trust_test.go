package gemini

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tailscale/hujson"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func TestGeminiProjectTrustPreservesForeignRulesAndRejectsDeny(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(parent, "selected")
	foreign := filepath.Join(parent, "foreign")
	raw := "{ /* owner comment */ " + strconvQuote(foreign) + ": \"TRUST_FOLDER\" }"
	next, err := geminiProjectTrustMapping([]byte(raw), project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(next), "/* owner comment */") {
		t.Fatal("lost owner comment")
	}
	standard, err := hujson.Standardize(next)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := shared.DecodeStrictJSONObject(standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(mapping) != 2 || mapping[foreign] != "TRUST_FOLDER" || mapping[project] != "TRUST_FOLDER" {
		t.Fatalf("changed unrelated rules: %s", next)
	}
	again, err := geminiProjectTrustMapping(next, project)
	if err != nil || string(again) != string(next) {
		t.Fatal("repeat changed trust")
	}
	for _, body := range []string{
		"{" + strconvQuote(project) + ":\"DO_NOT_TRUST\"}",
		"{" + strconvQuote(parent) + ":\"DO_NOT_TRUST\"}",
		"{" + strconvQuote(project) + ":\"DO_NOT_TRUST\"," + strconvQuote(project) + ":\"TRUST_FOLDER\"}",
		"{" + strconvQuote(foreign) + ":\"unknown\"}",
		"{" + strconvQuote(foreign) + ":\"TRUST_FOLDER\",}", "[]",
	} {
		if _, err := geminiProjectTrustMapping([]byte(body), project); err == nil {
			t.Fatalf("accepted unsafe trust %s", body)
		}
	}
}

func strconvQuote(value string) string { body, _ := json.Marshal(value); return string(body) }

func TestGeminiTrustIgnoresUnrelatedStaleHistoricalRoots(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(parent, "selected")
	blocker := filepath.Join(parent, "deleted-volume")
	if err := os.WriteFile(blocker, []byte("unrelated history now has a file ancestor"), 0600); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(blocker, "historical-project")
	for _, level := range []string{"TRUST_FOLDER", "TRUST_PARENT", "DO_NOT_TRUST"} {
		body := []byte("{" + strconvQuote(stale) + ":" + strconvQuote(level) + "}")
		next, err := geminiProjectTrustMapping(body, project)
		if err != nil {
			t.Fatalf("unrelated stale %s blocked grant: %v", level, err)
		}
		standard, err := hujson.Standardize(next)
		if err != nil {
			t.Fatal(err)
		}
		mapping, err := shared.DecodeStrictJSONObject(standard)
		if err != nil {
			t.Fatal(err)
		}
		if mapping[stale] != level || mapping[project] != "TRUST_FOLDER" {
			t.Fatalf("history changed: %s", next)
		}
	}
}

func TestGeminiTrustPreservesRelevantDenyThroughAlias(t *testing.T) {
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "alias")
	physical := filepath.Join(parent, "real")
	if err := os.Mkdir(physical, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(physical, alias); err != nil {
		t.Skip(err)
	}
	body := []byte("{" + strconvQuote(alias) + ":\"DO_NOT_TRUST\"}")
	if _, err := geminiProjectTrustMapping(body, filepath.Join(physical, "selected")); err == nil {
		t.Fatal("overrode explicit deny through live alias")
	}
}
