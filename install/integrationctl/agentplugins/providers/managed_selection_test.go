package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func TestManagedMCPSelectionRequiresExactArtifact(t *testing.T) {
	for _, client := range []domain.ClientID{domain.ClientCursor, domain.ClientClaude, domain.ClientCodex, domain.ClientChatGPT, domain.ClientKimi} {
		t.Run(string(client), func(t *testing.T) {
			root := t.TempDir()
			filename := "mcp.json"
			body := `{"mcpServers":{"z":{},"a":{}}}`
			if client == domain.ClientClaude || client == domain.ClientCodex || client == domain.ClientChatGPT {
				filename = ".mcp.json"
			}
			if client == domain.ClientClaude {
				body = `{"z":{},"a":{}}`
			}
			if client == domain.ClientKimi {
				filename = filepath.Join(".kimi-plugin", "plugin.json")
				body = `{"name":"demo","mcpServers":{"z":{},"a":{}}}`
			}
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filename)), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, filename), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			stager := testStager(Stager{})
			var mismatch *ports.VerificationError
			if err := stager.Verify(context.Background(), root, "observe"); !errors.As(err, &mismatch) {
				t.Fatal(err)
			}
			names, err := stager.ManagedMCPNames(context.Background(), client, root, mismatch.ActualDigest)
			if err != nil || !reflect.DeepEqual(names, []string{"a", "z"}) {
				t.Fatalf("names=%v err=%v", names, err)
			}
			if err := os.WriteFile(filepath.Join(root, filename), []byte(`{"mcpServers":{}}`), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := stager.ManagedMCPNames(context.Background(), client, root, mismatch.ActualDigest); err == nil {
				t.Fatal("unverified modified names trusted")
			}
		})
	}
}

func TestVerifiedAbsentMCPSelectionIsKnownEmpty(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plugin.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	stager := testStager(Stager{})
	var mismatch *ports.VerificationError
	if err := stager.Verify(context.Background(), root, "observe"); !errors.As(err, &mismatch) {
		t.Fatal(err)
	}
	names, err := stager.ManagedMCPNames(context.Background(), domain.ClientCursor, root, mismatch.ActualDigest)
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

func TestManifestSelectionWithoutServersMemberIsProvenEmpty(t *testing.T) {
	// The projected Kimi manifest declares "mcpServers" only when servers were
	// selected, so a verified skills-only manifest records an empty selection.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".kimi-plugin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".kimi-plugin", "plugin.json"),
		[]byte(`{"name":"demo","skills":"./skills/"}`), 0600); err != nil {
		t.Fatal(err)
	}
	stager := testStager(Stager{})
	var mismatch *ports.VerificationError
	if err := stager.Verify(context.Background(), root, "observe"); !errors.As(err, &mismatch) {
		t.Fatal(err)
	}
	names, err := stager.ManagedMCPNames(context.Background(), domain.ClientKimi, root, mismatch.ActualDigest)
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("names=%v err=%v", names, err)
	}
}

func TestDedicatedSelectionStillFailsClosedWithoutServersMember(t *testing.T) {
	// A dedicated selection file without its member is unrecognizable shape,
	// never a proven empty selection.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), []byte(`{"servers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	stager := testStager(Stager{})
	var mismatch *ports.VerificationError
	if err := stager.Verify(context.Background(), root, "observe"); !errors.As(err, &mismatch) {
		t.Fatal(err)
	}
	if _, err := stager.ManagedMCPNames(context.Background(), domain.ClientCursor, root, mismatch.ActualDigest); err == nil {
		t.Fatal("unrecognized nested selection shape trusted")
	}
}
