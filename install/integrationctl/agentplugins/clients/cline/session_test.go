package cline

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSessionRequiresInstalledClientBeforeFileAccess(t *testing.T) {
	for _, client := range []domain.DetectedClient{{}, {ClientID: domain.ClientCline, ExecutablePath: "relative", Status: domain.DetectionDetected}} {
		req := clients.SessionRequest{Client: client, Bindings: []domain.ClientBinding{{Scope: "project"}}}
		if _, err := New().PlanSession(context.Background(), clients.Env{}, req); err == nil {
			t.Fatal("unavailable client accepted")
		}
	}
}

// An unrelated binding must never trigger a native MCP config read.
// Nil NativeConfig makes an attempted unauthorized read fail immediately.
func TestSessionRejectsForeignBindingBeforeNativeConfigRead(t *testing.T) {
	root := t.TempDir()
	req := clients.SessionRequest{
		Client: domain.DetectedClient{
			ClientID: domain.ClientCline, Status: domain.DetectionDetected,
			ExecutablePath: filepath.Join(root, "cline"),
			Surfaces:       []domain.ClientSurface{{ID: "cline_cli", Detected: true}},
		},
		ProjectRoot: root,
		Bindings:    []domain.ClientBinding{{Scope: "user"}},
	}
	if _, err := New().PlanSession(context.Background(), clients.Env{}, req); err == nil || !strings.Contains(err.Error(), "Project scope") {
		t.Fatalf("foreign binding was not refused before file access: %v", err)
	}
}

type sessionReadIO struct {
	nativeconfig.FileIO
	reads      int
	beforeRead func(int)
}

func (files *sessionReadIO) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	files.reads++
	if files.beforeRead != nil {
		files.beforeRead(files.reads)
	}
	file, err := nativeconfig.New().ReadExactFile(path)
	return file.Body, file.Mode, file.Exists, err
}

func ownedSessionFixture(t *testing.T) (clients.SessionRequest, string) {
	t.Helper()
	root := t.TempDir()
	layout := clineProjectComponents(root)
	server := nativeconfig.Server{Type: "stdio", Command: filepath.Join(root, "owned-probe"), CWD: root}
	receipt, err := nativeconfig.New().Apply(nativeconfig.Request{
		Paths: layout.ConfigPaths, Codec: layout.Codec, Action: nativeconfig.ActionAdd, Name: "owned", Server: server,
	})
	if err != nil {
		t.Fatal(err)
	}
	object, err := layout.MCPObject("owned", server, receipt.Path)
	if err != nil || object.ManagedDigest != receipt.Digest {
		t.Fatalf("real native config receipt differs: %v", err)
	}
	return clients.SessionRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientCline, Status: domain.DetectionDetected,
			ExecutablePath: filepath.Join(root, "cline"), Surfaces: []domain.ClientSurface{{ID: "cline_cli", Detected: true}}},
		ProjectRoot: root,
		Bindings: []domain.ClientBinding{{ClientID: "cline", Scope: "project", NativeProfileRoot: root, PhysicalArtifact: "owned",
			TargetLocator: filepath.Join(root, ".uap", "managed", "clients", "cline", "owned"), NativeObjects: []domain.NativeObjectOwnership{object}}},
	}, receipt.Path
}

func TestSessionRejectsLaterForeignBindingBeforeNativeConfigRead(t *testing.T) {
	req, _ := ownedSessionFixture(t)
	req.Bindings = append(req.Bindings, domain.ClientBinding{Scope: "user"})
	files := &sessionReadIO{}
	_, err := New().PlanSession(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, req)
	if err == nil || !strings.Contains(err.Error(), "Project scope") || files.reads != 0 {
		t.Fatalf("later foreign binding reached config reads: reads=%d, err=%v", files.reads, err)
	}
}

func TestSessionRejectsConfigChangeBetweenReceiptAndSnapshot(t *testing.T) {
	req, path := ownedSessionFixture(t)
	if _, err := New().PlanSession(context.Background(), clients.Env{NativeConfig: nativeconfig.New()}, req); err != nil {
		t.Fatalf("intact owned source refused: %v", err)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	foreign := bytes.Replace(original, []byte("owned-probe"), []byte("other-probe"), 1)
	if bytes.Equal(original, foreign) {
		t.Fatal("mutation did not change the owned command")
	}
	if err := exactSessionServers(foreign, []string{"owned"}); err != nil {
		t.Fatalf("mutation must preserve the allowed session shape: %v", err)
	}
	mutations := 0
	var changed os.FileInfo
	files := &sessionReadIO{beforeRead: func(read int) {
		if read != 2 {
			return
		}
		if err := os.WriteFile(path, foreign, 0o600); err != nil {
			t.Fatal(err)
		}
		changed, err = os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		mutations++
	}}
	_, err = New().PlanSession(context.Background(), clients.Env{NativeConfig: nativeconfig.NewWithFileIO(files)}, req)
	if err == nil || mutations != 1 {
		t.Fatalf("changed owned source accepted: mutations=%d, reads=%d, err=%v", mutations, files.reads, err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, foreign) {
		t.Fatalf("refusal changed the competing config bytes: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || !os.SameFile(changed, info) || changed.Mode() != info.Mode() || !changed.ModTime().Equal(info.ModTime()) {
		t.Fatalf("refusal changed the competing config identity or metadata: %v", err)
	}
}

func TestSessionWholeFileRefusesForeignDisabledAndMalformedMembers(t *testing.T) {
	// Use platform-native absolute paths, including Windows drive/share paths.
	// Relative launchers must never be resolved through the host PATH or cwd.
	executable, err := json.Marshal(filepath.Join(t.TempDir(), "managed", "probe"))
	if err != nil {
		t.Fatal(err)
	}
	directory, err := json.Marshal(filepath.Join(t.TempDir(), "project"))
	if err != nil {
		t.Fatal(err)
	}
	nativePaths := func(body string) string {
		body = strings.ReplaceAll(body, `"/managed/probe"`, string(executable))
		return strings.ReplaceAll(body, `"/project"`, string(directory))
	}
	for _, body := range []string{
		`{"mcpServers":{}}`,
		`{"mcpServers":{"owned":{},"foreign":{}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":true}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":"false"}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":["read_file"]}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":["*"]}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":true}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":null}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"autoApprove":{}}}}`,
		`{"mcpServers":{"owned":{}} ,"extra":{}}`,
		`{"mcpServers":{"owned":{},"owned":{}}}`,
		`{"mcpServers":{"owned":null}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"sse","url":"http://localhost"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"http","url":"http://localhost"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":""}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"managed/probe","cwd":"/project"}}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"relative/project"}}}}`,
	} {
		if err := exactSessionServers([]byte(nativePaths(body)), []string{"owned"}); err == nil {
			t.Fatal("unsafe session source accepted", body)
		}
	}
	for _, body := range []string{
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false}}}`,
		`{"mcpServers":{"owned":{"transport":{"type":"stdio","command":"/managed/probe","cwd":"/project"},"disabled":false,"autoApprove":[]}}}`,
	} {
		if err := exactSessionServers([]byte(nativePaths(body)), []string{"owned"}); err != nil {
			t.Fatal(err)
		}
	}
}
