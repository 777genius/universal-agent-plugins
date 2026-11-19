package pluginkitairepo_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func openCodeCollisionNativeArgs() []string {
	return []string{"--print-logs", "--log-level", "DEBUG", "run", "--model", "uap-fixture/fixture", "--format", "json", "Call the fixture inspect_runtime tool once if available, then finish."}
}

func openCodeCollisionRedactor(root, client, provider, mcp string) *strings.Replacer {
	pairs := []string{root, "<fixture-root>", client, "<native-binary>", provider, "<fixture-provider>", mcp, "<fixture-mcp>", "disposable-not-a-secret", "<fixture-key>"}
	var replacements []string
	for i := 0; i < len(pairs); i += 2 {
		encoded, _ := json.Marshal(pairs[i])
		replacements = append(replacements, string(encoded[1:len(encoded)-1]), pairs[i+1], pairs[i], pairs[i+1])
	}
	return strings.NewReplacer(replacements...)
}

// This bounds retained diagnostics after CombinedOutput, not live process output.
// Replace complete known fixture values before cutting either retained edge.
func openCodeCollisionDiagnostic(body []byte, redact *strings.Replacer) ([]byte, bool) {
	safe := redact.Replace(string(body))
	if len(safe) <= openCodeProjectLimit {
		return []byte(safe), false
	}
	const omitted = "\n<diagnostic-middle-omitted>\n"
	edge := (openCodeProjectLimit - len(omitted)) / 2
	return []byte(safe[:edge] + omitted + safe[len(safe)-(openCodeProjectLimit-len(omitted)-edge):]), true
}

// A selected-file snapshot is not an effective native config observation.
// Diagnostic read refusal does not change the original native test assertions.
func openCodeCollisionSelectedConfig(path string) ([]byte, string) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, "absent"
	}
	if err != nil {
		return nil, "read_refused"
	}
	body, err := readProjectNativeSnapshot(path, info, 1<<20)
	if err != nil {
		return nil, "read_refused"
	}
	return body, "captured"
}

type openCodeCollisionHTTPFact struct {
	Method string `json:"method"`
	Stage  string `json:"stage"`
	Status int    `json:"status"`
}

type openCodeCollisionHTTPFacts struct {
	Rows    []openCodeCollisionHTTPFact `json:"rows"`
	Omitted int                         `json:"omitted"`
}

// Only fixed method/stage/status facts: no URL query, headers, body or raw path.
// Caller holds the existing fixture mutex; the original 405 remains unchanged.
func (facts *openCodeCollisionHTTPFacts) record(method, path string) {
	if len(facts.Rows) >= 64 {
		facts.Omitted++
		return
	}
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodOptions, http.MethodPut, http.MethodPatch:
	default:
		method = "other"
	}
	stage := "other"
	switch path {
	case "/server-0":
		stage = "install"
	case "/server-0/B":
		stage = "version-update"
	case "/server-0/C":
		stage = "same-version-refresh"
	case "/server-1":
		stage = "secondary-server"
	}
	facts.Rows = append(facts.Rows, openCodeCollisionHTTPFact{Method: method, Stage: stage, Status: http.StatusMethodNotAllowed})
}

func TestOpenCodeCollisionDiagnosticsAreBoundedAndPrivate(t *testing.T) {
	t.Run("same invocation with supported global log flags", func(t *testing.T) {
		want := []string{"--print-logs", "--log-level", "DEBUG", "run", "--model", "uap-fixture/fixture", "--format", "json", "Call the fixture inspect_runtime tool once if available, then finish."}
		if !reflect.DeepEqual(openCodeCollisionNativeArgs(), want) {
			t.Fatal("native invocation changed")
		}
	})
	t.Run("redact before both retained edges", func(t *testing.T) {
		secret := "CANARY-" + strings.Repeat("secret", 100)
		redactor := strings.NewReplacer(secret, "<fixture-key>")
		body := "start:" + strings.Repeat("x", openCodeProjectLimit/2-32) + secret + strings.Repeat("y", openCodeProjectLimit) + secret + ":end"
		safe, truncated := openCodeCollisionDiagnostic([]byte(body), redactor)
		if !truncated || len(safe) != openCodeProjectLimit || !strings.HasPrefix(string(safe), "start:") || !strings.HasSuffix(string(safe), ":end") || strings.Contains(string(safe), "CANARY") || strings.Contains(string(safe), "secret") {
			t.Fatal("diagnostic bound/redaction/edges changed")
		}
		safe, truncated = openCodeCollisionDiagnostic([]byte(secret), redactor)
		if truncated || string(safe) != "<fixture-key>" {
			t.Fatal("short diagnostic not redacted")
		}
	})
	t.Run("known fixture values in plain and JSON text", func(t *testing.T) {
		values := []string{`C:\owned\fixture`, `C:\native\opencode.exe`, "http://127.0.0.1:1234", "http://127.0.0.1:5678", "disposable-not-a-secret"}
		encoded, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		safe, _ := openCodeCollisionDiagnostic(append(encoded, []byte(strings.Join(values, " "))...), openCodeCollisionRedactor(values[0], values[1], values[2], values[3]))
		for _, value := range values {
			quoted, _ := json.Marshal(value)
			if strings.Contains(string(safe), value) || strings.Contains(string(safe), string(quoted[1:len(quoted)-1])) {
				t.Fatal("known fixture value retained")
			}
		}
	})
	t.Run("fixed HTTP facts and bounded rows", func(t *testing.T) {
		var facts openCodeCollisionHTTPFacts
		facts.record(http.MethodGet, "/server-0/C")
		facts.record("PRIVATE-CANARY", "/private-canary?token=secret")
		for range 70 {
			facts.record(http.MethodHead, "/server-0/B")
		}
		body, err := json.Marshal(facts)
		if err != nil || len(facts.Rows) != 64 || facts.Omitted != 8 || facts.Rows[0] != (openCodeCollisionHTTPFact{Method: "GET", Stage: "same-version-refresh", Status: 405}) || facts.Rows[1].Method != "other" || facts.Rows[1].Stage != "other" || strings.Contains(string(body), "CANARY") || strings.Contains(string(body), "secret") || strings.Contains(string(body), "private") {
			t.Fatal("HTTP diagnostic leaked input or lost fixed facts/cap")
		}
	})
	t.Run("selected file captured without accepting unsafe leaves", func(t *testing.T) {
		root := t.TempDir()
		path := filepath.Join(root, "opencode.json")
		want := []byte(`{"mcp":{"api/server":{"url":"http://fixture/server-0/C"}}}`)
		if err := os.WriteFile(path, want, 0o600); err != nil {
			t.Fatal(err)
		}
		body, status := openCodeCollisionSelectedConfig(path)
		if status != "captured" || string(body) != string(want) {
			t.Fatal("selected config not captured exactly")
		}
		if body, status = openCodeCollisionSelectedConfig(filepath.Join(root, "absent")); status != "absent" || len(body) != 0 {
			t.Fatal("absent selected config")
		}
		if body, status = openCodeCollisionSelectedConfig(root); status != "read_refused" || len(body) != 0 {
			t.Fatal("directory selected config accepted")
		}
		if err := os.WriteFile(path, []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		if body, status = openCodeCollisionSelectedConfig(path); status != "read_refused" || len(body) != 0 {
			t.Fatal("oversized selected config accepted")
		}
	})
}

func TestOpenCodeCollisionSelectedConfigRefusesSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "target.json")
	if err := os.WriteFile(target, []byte(`{"mcp":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "opencode.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if body, status := openCodeCollisionSelectedConfig(link); status != "read_refused" || len(body) != 0 {
		t.Fatal("symlink selected config accepted")
	}
}
