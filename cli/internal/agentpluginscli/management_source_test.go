package agentpluginscli

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementAddRejectsLocalArchivesRegardlessOfFilesystemType(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("existing.zip", []byte("not an acquired package"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("directory.zip", 0700); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"missing.zip",
		"existing.zip",
		"directory.zip",
		"existing.zip#sha256=" + strings.Repeat("a", 64),
		"directory.zip#sha256=" + strings.Repeat("a", 64),
	} {
		t.Run(source, func(t *testing.T) {
			// Ref provides a deterministic next refusal if a local source gets
			// past admission. No resolver or network dependencies are provided.
			input := management.ChangeRequest{Source: source, Ref: "main"}
			_, _, _, err := (&ManagementBackend{}).managementAdd(t.Context(), App{}, input, nil, environmentSnapshot{})
			var refusal *management.Refusal
			if !errors.As(err, &refusal) || refusal.Code != "SOURCE_NOT_SUPPORTED" {
				t.Fatalf("local archive source %q reached resolution admission: %v", source, err)
			}
		})
	}
}

func TestManagementAddPreservesRemoteArchiveAndGitGrammar(t *testing.T) {
	for _, source := range []string{
		"github:owner/plugins.zip",
		"github:owner/plugins.zip@" + strings.Repeat("a", 40),
		"owner/plugins.zip@" + strings.Repeat("a", 40),
		"git+https://example.test/plugins.zip.git",
		"https://example.test/plugins.zip",
	} {
		t.Run(source, func(t *testing.T) {
			// A separate Ref is deliberately invalid, so an admitted remote
			// source stops before any acquisition without constructing fixtures.
			input := management.ChangeRequest{Source: source, Ref: "main"}
			_, _, _, err := (&ManagementBackend{}).managementAdd(t.Context(), App{}, input, nil, environmentSnapshot{})
			var refusal *management.Refusal
			if !errors.As(err, &refusal) || refusal.Code != "INVALID_REQUEST" {
				t.Fatalf("remote source %q was rejected as local: %v", source, err)
			}
		})
	}
}
