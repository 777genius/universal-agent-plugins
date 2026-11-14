package nativeconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClineCWDReceiptUpdateDriftAndRemove(t *testing.T) {
	for _, historicalRemove := range []bool{false, true} {
		t.Run(map[bool]string{false: "update", true: "historical-remove"}[historicalRemove], func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "cline.json")
			mustWrite(t, p, `{"mcpServers":{"foreign":{"transport":{"type":"sse","url":"https://foreign.test"}}}}`)
			req := Request{Paths: Paths{JSON: p}, Codec: CodecCline, Action: ActionAdd, Name: "owned", Server: Server{Type: "stdio", Command: "node", Args: []string{"relative.js"}}}
			old, err := New().Apply(req)
			if err != nil {
				t.Fatal(err)
			}
			if historicalRemove {
				req.Action = ActionRemove
				req.Owned = &old
				if _, err := New().Apply(req); err != nil {
					t.Fatal(err)
				}
				return
			}
			req.Action = ActionUpdate
			req.Owned = &old
			req.Server.CWD = "${PLUGIN_DATA}/cache"
			req.Placeholders = Placeholders{DataRoot: "/owned/data"}
			updated, err := New().Apply(req)
			if err != nil {
				t.Fatal(err)
			}
			if old.Digest == updated.Digest {
				t.Fatal("cwd missing from ownership digest")
			}
			var doc map[string]any
			mustReadJSON(t, p, &doc)
			transport := doc["mcpServers"].(map[string]any)["owned"].(map[string]any)["transport"].(map[string]any)
			if transport["cwd"] != "/owned/data/cache" || transport["args"].([]any)[0] != "relative.js" {
				t.Fatalf("projection: %#v", transport)
			}
			intact, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			tampered := strings.Replace(string(intact), "/owned/data/cache", "/user/change", 1)
			mustWrite(t, p, tampered)
			req.Owned = &updated
			if _, err := New().Apply(req); !errors.Is(err, ErrNotOwned) {
				t.Fatalf("cwd drift accepted: %v", err)
			}
			after, _ := os.ReadFile(p)
			if string(after) != tampered {
				t.Fatal("drift mutated config")
			}
			mustWrite(t, p, string(intact))
			req.Action = ActionRemove
			if _, err := New().Apply(req); err != nil {
				t.Fatal(err)
			}
			mustReadJSON(t, p, &doc)
			servers := doc["mcpServers"].(map[string]any)
			if _, ok := servers["owned"]; ok || servers["foreign"] == nil {
				t.Fatalf("remove changed foreign: %#v", servers)
			}
		})
	}
}

func TestResolvedStdioProjectionDoesNotExpandAgain(t *testing.T) {
	for _, codec := range []Codec{CodecCline, CodecOpenCode, CodecGemini, CodecWindsurf} {
		t.Run(string(codec), func(t *testing.T) {
			root := "/owned/${PLUGIN_DATA}/${PLUGIN_ROOT}"
			server := Server{Type: "stdio", Command: "node", Args: []string{root + "/run.js", "${UNKNOWN}"}, Env: map[string]string{"ROOT": root, "UNKNOWN": "${HOME}"}, StdioValuesResolved: true}
			if codec != CodecWindsurf {
				server.CWD = root
			}
			projected, err := projectServer(codec, server, Placeholders{PackageRoot: "/wrong-root", DataRoot: "/wrong-data"})
			if err != nil {
				t.Fatal(err)
			}
			if codec == CodecCline {
				projected = projected["transport"].(map[string]any)
			}
			args := projected["args"]
			env := projected["env"]
			if codec == CodecOpenCode {
				args = projected["command"].([]string)[1:]
				env = projected["environment"]
			}
			if args.([]string)[0] != root+"/run.js" || args.([]string)[1] != "${UNKNOWN}" || env.(map[string]string)["ROOT"] != root || env.(map[string]string)["UNKNOWN"] != "${HOME}" {
				t.Fatalf("second expansion: %#v", projected)
			}
			if codec != CodecWindsurf && projected["cwd"] != root {
				t.Fatalf("cwd expanded: %#v", projected)
			}
		})
	}
}

func TestResolvedCWDStillExpandsRawStdioValues(t *testing.T) {
	root := "/owned/${PLUGIN_DATA}/${PLUGIN_ROOT}"
	projected, err := projectServer(CodecGemini, Server{Type: "stdio", Command: "node", CWD: root, CWDResolved: true, Args: []string{"${PLUGIN_ROOT}/run.js"}, Env: map[string]string{"DATA": "${PLUGIN_DATA}"}}, Placeholders{PackageRoot: root, DataRoot: "/data"})
	if err != nil {
		t.Fatal(err)
	}
	if projected["cwd"] != root || projected["args"].([]string)[0] != root+"/run.js" || projected["env"].(map[string]string)["DATA"] != "/data" {
		t.Fatalf("wrong raw/resolved boundary: %#v", projected)
	}
}
