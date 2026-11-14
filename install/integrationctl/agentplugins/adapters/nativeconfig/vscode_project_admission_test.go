package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVSCodeProjectCodecRefusesForeignAndModifiedMembers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	mustWrite(t, path, vscodeForeignDocument)
	req := Request{Paths: Paths{JSON: path}, Codec: CodecVSCode, Action: ActionAdd, Name: "foreign",
		Server: Server{Type: "stdio", Command: "TEST-never-executed"}}
	kernel := New()
	if _, err := kernel.Apply(req); !errors.Is(err, ErrCollision) {
		t.Fatalf("foreign numeric-env/disabled member adopted: %v", err)
	}
	req.Name = "managed"
	receipt, err := kernel.Apply(req)
	if err != nil {
		t.Fatal(err)
	}
	wrong := receipt
	wrong.Codec = CodecCopilot
	req.Action, req.Owned = ActionUpdate, &wrong
	if _, err := kernel.Apply(req); !errors.Is(err, ErrNotOwned) {
		t.Fatalf("Copilot receipt admitted to VS Code member: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(body), `"command":"TEST-never-executed"`, `"command":"TEST-never-executed","foreignMember":true`, 1)
	if changed == string(body) {
		t.Fatal("test did not modify the managed entry")
	}
	mustWrite(t, path, changed)
	req.Owned = &receipt
	for _, action := range []Action{ActionUpdate, ActionRemove} {
		req.Action = action
		if _, err := kernel.Apply(req); !errors.Is(err, ErrNotOwned) {
			t.Fatalf("changed complete entry accepted for %s: %v", action, err)
		}
		current, err := os.ReadFile(path)
		if err != nil || string(current) != changed {
			t.Fatalf("refusal changed native bytes: %v", err)
		}
	}
}

func TestVSCodeProjectCodecRejectsUnsupportedDialectAndAlternatePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	for _, server := range []Server{
		{Type: "remote", URL: "https://inert.test/mcp"},
		{Type: "remote", URL: "https://inert.test/mcp", RemoteTransport: "http"},
		{Type: "remote", URL: "https://inert.test/mcp", RemoteTransport: "sse", CWD: filepath.Dir(path)},
		{Type: "stdio", Command: "inert", RemoteTransport: "sse"},
		{Type: "stdio", Command: "inert", CodexAuth: &CodexAuthOptions{BearerTokenEnvVar: "TOKEN"}},
		{Type: "stdio", Command: "inert", OpenCodeV2: &OpenCodeV2Options{}},
	} {
		if _, err := DesiredReceipt(path, CodecVSCode, "managed", server, Placeholders{}); err == nil {
			t.Fatalf("unsupported VS Code dialect admitted: %+v", server)
		}
	}
	locks, err := WriterLockPaths(Paths{JSON: path}, CodecVSCode)
	if err != nil || len(locks) != 1 || locks[0] != path+".agentplugins.lock" {
		t.Fatalf("VS Code exact writer lock = %q, %v", locks, err)
	}
	paths := Paths{JSON: path, JSONC: path + "c"}
	if _, err := WriterLockPaths(paths, CodecVSCode); err == nil {
		t.Fatal("invented alternate VS Code config path obtained writer authority")
	}
	called := false
	kernel := NewWithLockAcquirer(func(Paths, Codec) (func() error, error) {
		called = true
		return func() error { return nil }, nil
	})
	req := Request{Paths: paths, Codec: CodecVSCode, Action: ActionAdd, Name: "managed", Server: Server{Type: "stdio", Command: "inert"}}
	if _, err := kernel.Apply(req); err == nil || called {
		t.Fatalf("alternate path reached mutation authority: %v, locked=%t", err, called)
	}
	if _, _, err := New().Inspect(paths, CodecVSCode, "managed", nil); err == nil {
		t.Fatal("alternate path admitted by read-only inspection")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused path created state: %v", err)
	}
}

func TestVSCodeProjectJSONCStillRejectsMalformedOrDuplicateMembers(t *testing.T) {
	for _, body := range []string{`{"servers":[]}`, `{"servers":{},"servers":{}}`, `{"servers":{"foreign":{"env":{"PORT":1,"PORT":2}}}}`} {
		path := filepath.Join(t.TempDir(), "mcp.json")
		mustWrite(t, path, body)
		if _, _, err := New().Inspect(Paths{JSON: path}, CodecVSCode, "managed", nil); !errors.Is(err, ErrMalformed) {
			t.Fatalf("malformed native JSONC admitted: %v", err)
		}
	}
}
