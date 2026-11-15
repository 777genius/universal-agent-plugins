package nativeconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tailscale/hujson"
)

func TestCopilotTrustProjectPreservesJSONCAndPermissions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	body := []byte("// original profile\n{\n" +
		`  "trustedFolders": ["/foreign", /* retained folder comment */],` + "\n" +
		`  "allowedTools": ["read"], "deniedTools": ["shell"],` + "\n" +
		`  "foreign": {"opaque": 12345678901234567890, /* retained value */ "enabled": false,},` + "\n}\n")
	before := bytes.Clone(body)
	next, already, err := CopilotTrustProject(body, root)
	if err != nil || already {
		t.Fatalf("append trust: already=%v err=%v", already, err)
	}
	if !bytes.Equal(body, before) {
		t.Fatal("patch mutated caller's original bytes")
	}
	for _, comment := range []string{"// original profile", "/* retained folder comment */", "/* retained value */", "12345678901234567890"} {
		if !strings.Contains(string(next), comment) {
			t.Fatalf("lost original comment or exact value %q: %s", comment, next)
		}
	}
	want := copilotTrustDocument(t, body)
	want["trustedFolders"] = []any{"/foreign", root}
	if got := copilotTrustDocument(t, next); !reflect.DeepEqual(got, want) {
		t.Fatalf("changed unrelated configuration: got=%v want=%v", got, want)
	}
	again, already, err := CopilotTrustProject(next, root)
	if err != nil || !already || !bytes.Equal(again, next) {
		t.Fatalf("repeat changed configuration: already=%v err=%v", already, err)
	}
	again[0] ^= 1
	if bytes.Equal(again, next) {
		t.Fatal("idempotent result aliases input")
	}
}

func TestCopilotTrustProjectCreatesOnlyExactTrust(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project with spaces")
	for _, body := range [][]byte{nil, []byte("{}"), []byte(`{"trustedFolders": []}`), []byte(`{"trustedFolders": ["/"]}`)} {
		next, already, err := CopilotTrustProject(body, root)
		if err != nil || already {
			t.Fatalf("exact trust append: already=%v err=%v", already, err)
		}
		got := copilotTrustDocument(t, next)["trustedFolders"].([]any)
		if got[len(got)-1] != root {
			t.Fatalf("selected project was broadened: %v", got)
		}
		if len(body) == 0 && len(copilotTrustDocument(t, next)) != 1 {
			t.Fatal("new profile added unrelated state")
		}
	}
}

func TestCopilotTrustProjectRejectsMalformedState(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	for _, body := range []string{
		" ", "null", "[]", "true", "{", "{} {}",
		`{"trustedFolders": null}`, `{"trustedFolders": "/foreign"}`,
		`{"trustedFolders": {}}`, `{"trustedFolders": [false]}`,
		`{"trustedFolders": [1]}`, `{"trustedFolders": [[]]}`,
		`{"trustedFolders": [], "trustedFolders": []}`,
		`{"foreign": {"duplicate": 1, "duplicate": 2}}`,
	} {
		t.Run(body, func(t *testing.T) {
			next, already, err := CopilotTrustProject([]byte(body), root)
			if !errors.Is(err, ErrMalformed) || already || next != nil {
				t.Fatalf("malformed config accepted: next=%s already=%v err=%v", next, already, err)
			}
		})
	}
	// Finding the exact root cannot short-circuit validation of later entries.
	body, err := json.Marshal(map[string]any{"trustedFolders": []any{root, nil}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := CopilotTrustProject(body, root); !errors.Is(err, ErrMalformed) {
		t.Fatal("exact root concealed invalid later array entry")
	}
}

func TestCopilotTrustProjectRejectsBroadOrUncleanRoot(t *testing.T) {
	root := t.TempDir()
	for _, project := range []string{"", "relative", filepath.VolumeName(root) + string(filepath.Separator), root + string(filepath.Separator) + ".."} {
		if _, _, err := CopilotTrustProject([]byte("{}"), project); err == nil {
			t.Fatalf("unsafe project accepted: %q", project)
		}
	}
}

func copilotTrustDocument(t *testing.T, body []byte) map[string]any {
	t.Helper()
	standard, err := hujson.Standardize(body)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(standard))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}
