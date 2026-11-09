package nativeconfig

import (
	"bytes"
	"runtime"
  "errors"
  "os"
  "path/filepath"
  "strings"
  "testing"
)

func TestHermesYAMLOwnedMCPAddUpdateRemovePreservesForeignConfig(t *testing.T) {
 t.Parallel()
 root := t.TempDir()
 path := filepath.Join(root, "config.yaml")
 original := "# Keep this comment and provider credentials\nmodel: demo\nsecrets:\n  api_key: keep-private\nmcp_servers:\n  foreign:\n    command: echo\n    args: [foreign]\n"
 if err := os.WriteFile(path, []byte(original), 0o600); err != nil { t.Fatal(err) }
 kernel := New()
 req := Request{Paths: Paths{JSON: path}, Codec: CodecHermes, Action: ActionAdd,
  Name: "owned", Server: Server{Type: "stdio", Command: "node", Args: []string{"${PLUGIN_ROOT}/entry.mjs"}},
  Placeholders: Placeholders{PackageRoot: filepath.Join(root, "managed")}}
 receipt, err := kernel.Apply(req)
 if err != nil { t.Fatal(err) }
 present, owned, err := kernel.Inspect(req.Paths, req.Codec, req.Name, &receipt)
 if err != nil || !present || !owned { t.Fatalf("installed entry not owned: %t %t %v", present, owned, err) }
 changed, err := os.ReadFile(path)
 if err != nil || !strings.Contains(string(changed), "# Keep this comment") ||
   !strings.Contains(string(changed), "api_key: keep-private") || !strings.Contains(string(changed), "foreign:") {
  t.Fatalf("Hermes YAML touched foreign fields: %s, %v", changed, err)
 }
 if _, err := kernel.Apply(req); !errors.Is(err, ErrCollision) { t.Fatalf("add must refuse identity collision: %v", err) }
 req.Action, req.Owned = ActionUpdate, &receipt
 req.Server = Server{Type: "remote", URL: "https://example.test/mcp"}
 updated, err := kernel.Apply(req)
 if err != nil { t.Fatal(err) }
 if _, owned, err := kernel.Inspect(req.Paths, req.Codec, req.Name, &receipt); err != nil || owned {
  t.Fatalf("old receipt must not own updated entry: %t %v", owned, err)
 }
 req.Action, req.Owned = ActionRemove, &updated
 if _, err := kernel.Apply(req); err != nil { t.Fatal(err) }
 after, err := os.ReadFile(path)
 if err != nil || strings.Contains(string(after), "owned:") ||
  !strings.Contains(string(after), "api_key: keep-private") || !strings.Contains(string(after), "foreign:") {
  t.Fatalf("exact removal touched foreign config: %s, %v", after, err)
 }
 info, err := os.Lstat(path)
 if err != nil || info.Mode().Perm() != 0o600 { t.Fatalf("mode changed: %v %+v", err, info) }
}

func TestHermesYAMLRefusesForeignEditBeforeRemove(t *testing.T) {
 t.Parallel()
 path := filepath.Join(t.TempDir(), "config.yaml")
 kernel := New()
 req := Request{Paths: Paths{JSON: path}, Codec: CodecHermes, Action: ActionAdd,
  Name: "owned", Server: Server{Type: "stdio", Command: "node"}}
 receipt, err := kernel.Apply(req)
 if err != nil { t.Fatal(err) }
 if err := os.WriteFile(path, []byte("mcp_servers:\n  owned:\n    command: foreign\n"), 0o600); err != nil { t.Fatal(err) }
 req.Action, req.Owned = ActionRemove, &receipt
 if _, err := kernel.Apply(req); !errors.Is(err, ErrNotOwned) { t.Fatalf("foreign edit must prevent remove: %v", err) }
 body, err := os.ReadFile(path)
 if err != nil || !strings.Contains(string(body), "command: foreign") { t.Fatalf("foreign edit overwritten: %s %v", body, err) }
}

func TestHermesYAMLRejectsMalformedAndAmbiguousConfig(t *testing.T) {
 t.Parallel()
 root := t.TempDir()
 path := filepath.Join(root, "config.yaml")
 kernel := New()
 req := Request{Paths: Paths{JSON: path}, Codec: CodecHermes, Action: ActionAdd,
  Name: "owned", Server: Server{Type: "stdio", Command: "node"}}
 for _, contents := range []string{
  "mcp_servers: [invalid]\n",
  "mcp_servers:\n  duplicate: {command: one}\n  duplicate: {command: two}\n",
  "mcp_servers:\n  original: &first {command: node}\n  other: *first\n",
  "mcp_servers:\n  invalid: !!python/object/apply:os.system [echo]\n",
 } {
  if err := os.WriteFile(path, []byte(contents), 0o600); err != nil { t.Fatal(err) }
  if _, err := kernel.Apply(req); err == nil { t.Fatalf("ambiguous YAML accepted: %q", contents) }
  body, err := os.ReadFile(path)
  if err != nil || string(body) != contents { t.Fatalf("YAML changed after refusal: %v", err) }
 }
 if err := os.Remove(path); err != nil { t.Fatal(err) }
 if err := os.Symlink(filepath.Join(root, "other.yaml"), path); err == nil {
  if _, err := kernel.Apply(req); err == nil { t.Fatal("symlink must not be followed") }
 }
 if _, err := kernel.Apply(Request{Paths: Paths{JSON: filepath.Join(root, "wrong.yaml")},
  Codec: CodecHermes, Name: "other", Action: ActionAdd,
  Server: Server{Type: "stdio", Command: "node"}}); err == nil {
  t.Fatal("Hermes YAML writes require exact config.yaml")
 }
}


func TestHermesRefusesExistingWorldReadableConfig(t *testing.T) {
	if runtime.GOOS == "windows" { t.Skip("Windows security uses ACLs rather than POSIX mode bits") }
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := []byte("model: demo\n")
	if err := os.WriteFile(path, body, 0o644); err != nil { t.Fatal(err) }
	kernel := New()
	req := Request{Paths: Paths{JSON:path}, Codec:CodecHermes, Action:ActionAdd,
		Name:"managed", Server:Server{Type:"stdio",Command:"node"}}
	if _, err := kernel.Apply(req); err == nil {
		t.Fatal("Hermes must not write secrets into an existing 0644 config")
	}
	now, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(now, body) {
		t.Fatalf("unsafe file changed on refusal: %q %v", now, err)
	}
}
