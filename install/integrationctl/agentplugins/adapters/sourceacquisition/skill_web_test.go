package sourceacquisition

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestWebSkillCompleteTreeAndPinnedRestore(t *testing.T) {
	const prefix = "/publisher/.well-known/agent-skills/"
	skill := "---\nname: demo\ndescription: Test skill\n---\nInert content.\n"
	indexRequests := 0
	changed := false
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Accept-Encoding") != "identity" {
			t.Error("credentials or content transformation")
		}
		switch r.URL.Path {
		case prefix + "index.json":
			indexRequests++
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","description":"demo","files":["references/guide.md","SKILL.md","scripts/tool.sh"]}]}`)
		case prefix + "demo/SKILL.md":
			_, _ = fmt.Fprint(w, skill)
		case prefix + "demo/references/guide.md":
			if changed {
				_, _ = fmt.Fprint(w, "changed")
			} else {
				_, _ = fmt.Fprint(w, "Full guide")
			}
		case prefix + "demo/scripts/tool.sh":
			_, _ = fmt.Fprint(w, "#!/bin/sh\nexit 0\n")
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	a := Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
	snapshot, facts, err := a.ResolveHTTPSkill(context.Background(), server.URL+"/publisher/", "demo")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(snapshot) }()
	if facts.URL != server.URL+"/publisher" || facts.IndexURL != server.URL+prefix+"index.json" || facts.RawTreeDigest != snapshot.TreeDigest || len(facts.Files) != 3 {
		t.Fatalf("unexpected facts %+v", facts)
	}
	for relative, want := range map[string]string{"SKILL.md": skill, "references/guide.md": "Full guide", "scripts/tool.sh": "#!/bin/sh\nexit 0\n"} {
		body, err := os.ReadFile(filepath.Join(snapshot.Root, filepath.FromSlash(relative)))
		if err != nil || string(body) != want {
			t.Fatalf("file %s: %q %v", relative, body, err)
		}
	}
	if len(snapshot.ExecutableFiles) != 0 {
		t.Fatal("HTTP files must not gain executable modes")
	}
	originalFiles := append([]domain.WebSkillFile(nil), facts.Files...)
	restored, err := a.AcquireHTTPSkillVerified(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = packagedigest.Remove(restored) }()
	if restored.TreeDigest != snapshot.TreeDigest || indexRequests != 1 || !reflect.DeepEqual(originalFiles, facts.Files) {
		t.Fatal("restore rediscovered or changed immutable facts")
	}
	changed = true
	if _, err := a.AcquireHTTPSkillVerified(context.Background(), facts); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed bytes accepted: %v", err)
	}
	if indexRequests != 1 {
		t.Fatal("restore fetched index")
	}
	facts.RawTreeDigest = "sha256:" + strings.Repeat("a", 64)
	changed = false
	if _, err := a.AcquireHTTPSkillVerified(context.Background(), facts); err == nil || !strings.Contains(err.Error(), "tree digest") {
		t.Fatalf("changed tree pin accepted: %v", err)
	}
}

func TestWebSkillFallbackOnlyFor404(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusForbidden, http.StatusInternalServerError, http.StatusTemporaryRedirect, http.StatusOK} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			fallback := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/prefix/.well-known/agent-skills/index.json":
					w.Header().Set("Location", "/other")
					w.WriteHeader(status)
					_, _ = fmt.Fprint(w, "malformed")
				case "/prefix/.well-known/skills/index.json":
					fallback++
					_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md"]}]}`)
				case "/prefix/.well-known/skills/demo/SKILL.md":
					_, _ = fmt.Fprint(w, "complete")
				default:
					t.Errorf("out of scope request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			a := Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			snapshot, _, err := a.ResolveHTTPSkill(context.Background(), server.URL+"/prefix", "demo")
			if status == http.StatusNotFound {
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = packagedigest.Remove(snapshot) }()
				if fallback != 1 {
					t.Fatal("missing fallback")
				}
			} else if err == nil || fallback != 0 {
				t.Fatalf("unexpected fallback=%d err=%v", fallback, err)
			}
		})
	}
}

func TestWebSkillMalformedOrUnsafeIndex(t *testing.T) {
	cases := map[string]string{
		"malformed":           "{",
		"null skills":         `{"skills":null}`,
		"duplicate key":       `{"skills":[],"skills":[{"name":"demo","files":["SKILL.md"]}]}`,
		"duplicate name":      `{"skills":[{"name":"demo","files":["SKILL.md"]},{"name":"demo","files":["SKILL.md"]}]}`,
		"missing skill":       `{"skills":[{"name":"other","files":["SKILL.md"]}]}`,
		"versioned collision": `{"$schema":"https://schemas.agentskills.io/discovery/0.2.0/schema.json","skills":[{"name":"demo","files":["SKILL.md"]}]}`,
		"version collision":   `{"version":"0.2.0","skills":[{"name":"demo","files":["SKILL.md"]}]}`,
		"unsupported archive": `{"skills":[{"name":"demo","type":"application/zip","url":"demo.zip"}]}`,
		"missing root":        `{"skills":[{"name":"demo","files":["refs/doc.md"]}]}`,
		"traversal":           `{"skills":[{"name":"demo","files":["SKILL.md","../escape"]}]}`,
		"absolute":            `{"skills":[{"name":"demo","files":["SKILL.md","/escape"]}]}`,
		"encoded traversal":   `{"skills":[{"name":"demo","files":["SKILL.md","%2e%2e/escape"]}]}`,
		"duplicate file":      `{"skills":[{"name":"demo","files":["SKILL.md","SKILL.md"]}]}`,
		"file dir collision":  `{"skills":[{"name":"demo","files":["SKILL.md","refs","refs/one"]}]}`,
	}
	for name, index := range cases {
		t.Run(name, func(t *testing.T) {
			files := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "index.json") {
					_, _ = fmt.Fprint(w, index)
				} else {
					files++
					_, _ = fmt.Fprint(w, "unexpected")
				}
			}))
			defer server.Close()
			a := Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			if snapshot, _, err := a.ResolveHTTPSkill(context.Background(), server.URL, "demo"); err == nil {
				_ = packagedigest.Remove(snapshot)
				t.Fatal("invalid index accepted")
			}
			if files != 0 {
				t.Fatalf("invalid index caused %d file requests", files)
			}
		})
	}
}

func TestWebSkillLimitsAndRedirects(t *testing.T) {
	for _, scenario := range []string{"index bound", "file bound", "encoded body", "file redirect"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "index.json") {
					if scenario == "index bound" {
						w.Header().Set("Content-Length", fmt.Sprint(webSkillIndexBytes+1))
						return
					}
					_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md"]}]}`)
					return
				}
				switch scenario {
				case "file bound":
					w.Header().Set("Content-Length", fmt.Sprint(webSkillFileBytes+1))
				case "encoded body":
					w.Header().Set("Content-Encoding", "gzip")
					_, _ = fmt.Fprint(w, "bad")
				case "file redirect":
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
				default:
					t.Fatal("unexpected file request")
				}
			}))
			defer server.Close()
			a := Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}
			if snapshot, _, err := a.ResolveHTTPSkill(context.Background(), server.URL, "demo"); err == nil {
				_ = packagedigest.Remove(snapshot)
				t.Fatal("unsafe response accepted")
			}
		})
	}
}

func TestWebSkillFallbackForPositiveHTMLOnly(t *testing.T) {
	for _, scenario := range []struct {
		name, contentType, body string
		fallback                bool
	}{
		{"html mime", "text/html; charset=utf-8", "<body>App</body>", true},
		{"html body", "text/plain", "<!DOCTYPE html><html>App</html>", true},
		{"malformed json", "application/json", "{broken", false},
		{"unsupported json", "application/json", `{"skills":[{"name":"demo","url":"SKILL.md"}]}`, false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			probes := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/prefix/.well-known/agent-skills/index.json":
					w.Header().Set("Content-Type", scenario.contentType)
					_, _ = fmt.Fprint(w, scenario.body)
				case "/prefix/.well-known/skills/index.json":
					probes++
					_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md"]}]}`)
				case "/prefix/.well-known/skills/demo/SKILL.md":
					_, _ = fmt.Fprint(w, "inert")
				default:
					t.Errorf("unexpected request %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			snapshot, _, err := (Acquirer{TempRoot: t.TempDir(), HTTPClient: server.Client()}).ResolveHTTPSkill(context.Background(), server.URL+"/prefix", "demo")
			if scenario.fallback {
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = packagedigest.Remove(snapshot) }()
				if probes != 1 {
					t.Fatal("HTML fallback missing")
				}
			} else if err == nil || probes != 0 {
				t.Fatalf("unexpected fallback=%d err=%v", probes, err)
			}
		})
	}
}

// cancelAfterFirstRead cancels once the client has read the first body bytes,
// so cancellation lands inside the body read rather than inside Do.
type cancelAfterFirstRead struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (body cancelAfterFirstRead) Read(buffer []byte) (int, error) {
	n, err := body.ReadCloser.Read(buffer)
	body.cancel()
	return n, err
}

type cancelOnFileBodyTransport struct {
	base   http.RoundTripper
	path   string
	cancel context.CancelFunc
}

func (transport cancelOnFileBodyTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := transport.base.RoundTrip(request)
	if err == nil && request.URL.Path == transport.path {
		response.Body = cancelAfterFirstRead{ReadCloser: response.Body, cancel: transport.cancel}
	}
	return response, err
}

func TestWebSkillCancellationDuringFileBodyLeavesNoTree(t *testing.T) {
	const prefix = "/publisher/.well-known/agent-skills/"
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case prefix + "index.json":
			_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md","references/slow.md"]}]}`)
		case prefix + "demo/SKILL.md":
			_, _ = fmt.Fprint(w, "inert")
		case prefix + "demo/references/slow.md":
			_, _ = fmt.Fprint(w, "partial")
			w.(http.Flusher).Flush()
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	temporary := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &http.Client{Transport: cancelOnFileBodyTransport{base: server.Client().Transport, path: prefix + "demo/references/slow.md", cancel: cancel}}
	_, _, err := (Acquirer{TempRoot: temporary, HTTPClient: client}).ResolveHTTPSkill(ctx, server.URL+"/publisher", "demo")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Skill file body cancellation = %v", err)
	}
	if entries, err := os.ReadDir(temporary); err != nil || len(entries) != 0 {
		t.Fatalf("canceled Skill download left workspace: %v %v", entries, err)
	}
}

func TestWebSkillRefusesUntrustedCertificateByDefault(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"skills":[{"name":"demo","files":["SKILL.md"]}]}`)
	}))
	defer server.Close()
	snapshot, _, err := (Acquirer{TempRoot: t.TempDir()}).ResolveHTTPSkill(context.Background(), server.URL, "demo")
	if err == nil || !strings.Contains(err.Error(), "TLS") {
		if err == nil {
			_ = packagedigest.Remove(snapshot)
		}
		t.Fatalf("self-signed publisher certificate was not refused by TLS verification: %v", err)
	}
}
