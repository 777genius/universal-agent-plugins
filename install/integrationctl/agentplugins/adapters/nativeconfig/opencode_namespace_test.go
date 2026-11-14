package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenCodeNamespaceNameSemantics(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{"api/server", "api server", true},
		{"foo", "foo_bar", true},
		{"foo_bar", "foo", true},
		{"foo", "foobar", false},
		{"foo-bar", "foo_bar", false},
		{"🤖", "__", true},
		{"é", "_", true},
		{"ab", "abc_def", false},
	} {
		if got := openCodeNamesMayCollide(tc.a, tc.b); got != tc.want {
			t.Errorf("collision(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
	if got := openCodeToolNamePart("🤖"); got != "__" {
		t.Fatalf("UTF-16 surrogate pair normalized to %q", got)
	}
	if reason := openCodeCollisionReason("api/server", "api server"); reason != "server names normalize identically" {
		t.Fatalf("normalization reason = %q", reason)
	}
	if reason := openCodeCollisionReason("foo", "foo_bar"); reason != "one normalized server name contains the other followed by _" {
		t.Fatalf("prefix reason = %q", reason)
	}
}

func TestOpenCodeNamespaceProposedServersConflictWithEachOther(t *testing.T) {
	root := t.TempDir()
	paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	var collision *OpenCodeNamespaceConflict
	if err := New().CheckOpenCodeNamespace(paths, []string{"api server", "api/server"}, nil); !errors.As(err, &collision) || collision.Reason == "" {
		t.Fatalf("proposed server conflict = %v", err)
	}
	if _, err := os.Stat(paths.JSON); !os.IsNotExist(err) {
		t.Fatalf("read-only check created config: %v", err)
	}
}

func TestOpenCodeNamespacePreflightAndApplyRejectWithoutWriting(t *testing.T) {
	for _, tc := range []struct{ ext, foreign, proposed string }{
		{"json", "api/server", "api server"},
		{"jsonc", "api/server", "api server"},
		{"json", "api/server", "api/server"},
	} {
		t.Run(tc.ext+"/"+tc.proposed, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			path := filepath.Join(root, "opencode."+tc.ext)
			original := `{"theme":"dark","mcp":{"` + tc.foreign + `":{"type":"remote","url":"https://foreign.test"}}}`
			if tc.ext == "jsonc" {
				original = "// preserved\n" + original
			}
			mustWrite(t, path, original)
			kernel := New()
			var collision *OpenCodeNamespaceConflict
			if err := kernel.CheckOpenCodeNamespace(paths, []string{tc.proposed}, nil); !errors.As(err, &collision) || collision.Proposed != tc.proposed || collision.Existing != tc.foreign {
				t.Fatalf("preflight conflict = %v", err)
			}
			_, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: tc.proposed, Server: Server{Type: "remote", URL: "https://example.test"}})
			if !errors.As(err, &collision) && !errors.Is(err, ErrCollision) {
				t.Fatalf("final guard conflict = %v", err)
			}
			if body := mustRead(t, path); body != original {
				t.Fatalf("rejected write changed config: %s", body)
			}
		})
	}
}

func TestOpenCodeNamespaceBatchRemovalAndLateRace(t *testing.T) {
	root := t.TempDir()
	paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	kernel := New()
	old, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "api/server", Server: Server{Type: "remote", URL: "https://example.test/old"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := kernel.CheckOpenCodeNamespace(paths, []string{"api server"}, []string{"api/server"}); err != nil {
		t.Fatalf("planned removal still conflicted: %v", err)
	}
	_, err = kernel.ApplyBatch([]Request{
		{Paths: paths, Codec: CodecOpenCode, Action: ActionRemove, Name: "api/server", Owned: &old},
		{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "api server", Server: Server{Type: "remote", URL: "https://example.test/new"}},
	})
	if err != nil {
		t.Fatalf("atomic rename failed: %v", err)
	}
	if err := kernel.CheckOpenCodeNamespace(paths, []string{"unrelated"}, nil); err != nil {
		t.Fatal(err)
	}
	// A non-cooperating client changes the file after the read-only check.
	before := mustRead(t, paths.JSON)
	foreign := strings.Replace(before, `"mcp": {`, `"mcp": {"unrelated_extra":{"type":"remote","url":"https://foreign.test"},`, 1)
	if foreign == before {
		foreign = strings.Replace(before, `"mcp":{`, `"mcp":{"unrelated_extra":{"type":"remote","url":"https://foreign.test"},`, 1)
	}
	if foreign == before {
		t.Fatalf("fixture has unexpected formatting: %s", before)
	}
	mustWrite(t, paths.JSON, foreign)
	_, err = kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "unrelated", Server: Server{Type: "remote", URL: "https://example.test"}})
	var collision *OpenCodeNamespaceConflict
	if !errors.As(err, &collision) || collision.Existing != "unrelated_extra" {
		t.Fatalf("late conflict = %v", err)
	}
	if got := mustRead(t, paths.JSON); got != foreign {
		t.Fatal("late rejection changed config")
	}
}

func TestOpenCodeNamespaceDisabledFlatServersAndV2(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       error
	}{
		{"disabled", `{"mcp":{"api/server":{"type":"remote","url":"https://foreign.test","enabled":false}}}`, nil},
		{"flat named servers", `{"mcp":{"servers":{"type":"remote","url":"https://foreign.test"}}}`, nil},
		{"nested servers", `{"mcp":{"servers":{"foreign":{"type":"remote","url":"https://foreign.test"}}}}`, ErrOpenCodeV2Namespace},
		{"empty nested servers", `{"mcp":{"servers":{}}}`, ErrOpenCodeV2Namespace},
		{"invalid enabled", `{"mcp":{"foreign":{"type":"remote","url":"https://foreign.test","enabled":"no"}}}`, ErrMalformed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			mustWrite(t, paths.JSON, tc.body)
			err := New().CheckOpenCodeNamespace(paths, []string{"api server"}, nil)
			if tc.want == nil && err != nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("preflight = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				_, applyErr := New().Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "api server", Server: Server{Type: "remote", URL: "https://example.test"}})
				if !errors.Is(applyErr, tc.want) || mustRead(t, paths.JSON) != tc.body {
					t.Fatalf("final guard = %v; bytes changed = %v", applyErr, mustRead(t, paths.JSON) != tc.body)
				}
			}
		})
	}
}

func TestOpenCodeNamespaceForeignConflictDoesNotBlockRemoval(t *testing.T) {
	root := t.TempDir()
	paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
	kernel := New()
	owned, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionAdd, Name: "owned", Server: Server{Type: "remote", URL: "https://example.test"}})
	if err != nil {
		t.Fatal(err)
	}
	body := mustRead(t, paths.JSON)
	body = strings.Replace(body, `"owned":`, `"api/server":{"type":"remote","url":"https://foreign.test"},"api server":{"type":"remote","url":"https://foreign.test"},"owned":`, 1)
	if !strings.Contains(body, `"api/server"`) {
		t.Fatalf("fixture has unexpected formatting: %s", body)
	}
	mustWrite(t, paths.JSON, body)
	if _, err := kernel.Apply(Request{Paths: paths, Codec: CodecOpenCode, Action: ActionRemove, Name: "owned", Owned: &owned}); err != nil {
		t.Fatalf("unrelated foreign collision blocked removal: %v", err)
	}
}

func TestOpenCodeNamespacePreflightRejectsAmbiguousMalformedAndSymlinkConfig(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, Paths)
		want  error
	}{
		{"both variants", func(t *testing.T, p Paths) { mustWrite(t, p.JSON, `{}`); mustWrite(t, p.JSONC, `{}`) }, ErrAmbiguousConfig},
		{"malformed", func(t *testing.T, p Paths) { mustWrite(t, p.JSON, `{"mcp":`) }, ErrMalformed},
		{"duplicate key", func(t *testing.T, p Paths) { mustWrite(t, p.JSON, `{"mcp":{"a":{},"a":{}}}`) }, ErrMalformed},
		{"symlink", func(t *testing.T, p Paths) {
			other := filepath.Join(filepath.Dir(p.JSON), "foreign.json")
			mustWrite(t, other, `{}`)
			if err := os.Symlink(other, p.JSON); err != nil {
				t.Fatal(err)
			}
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			paths := Paths{JSON: filepath.Join(root, "opencode.json"), JSONC: filepath.Join(root, "opencode.jsonc")}
			tc.setup(t, paths)
			if err := New().CheckOpenCodeNamespace(paths, []string{"new"}, nil); err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("expected fail-closed %v; got %v", tc.want, err)
			}
		})
	}
}
