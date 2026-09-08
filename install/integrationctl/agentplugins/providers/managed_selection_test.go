package providers

import (
	"context"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestManagedMCPSelectionRequiresExactArtifact(t *testing.T) {
	for _, client := range []domain.ClientID{domain.ClientCursor, domain.ClientClaude, domain.ClientCodex, domain.ClientChatGPT} {
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
			if err := os.WriteFile(filepath.Join(root, filename), []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			stager := Stager{}
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
	stager := Stager{}
	var mismatch *ports.VerificationError
	if err := stager.Verify(context.Background(), root, "observe"); !errors.As(err, &mismatch) {
		t.Fatal(err)
	}
	names, err := stager.ManagedMCPNames(context.Background(), domain.ClientCursor, root, mismatch.ActualDigest)
	if err != nil || names == nil || len(names) != 0 {
		t.Fatalf("names=%v err=%v", names, err)
	}
}
