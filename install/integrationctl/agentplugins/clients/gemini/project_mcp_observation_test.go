package gemini

import (
	"strings"
	"testing"
)

func TestGeminiMCPConnectedRequiresExactStdioIdentity(t *testing.T) {
	row := "✓ audit: /owned/package/server.py --literal (stdio) - Connected"
	expected := map[string]string{"audit": row}
	good := "Configured MCP servers:\n\n" + row + "\n"
	if !geminiMCPConnected([]byte(good), expected) {
		t.Fatal("exact native initialized server refused")
	}
	for _, bad := range []string{strings.Replace(good, "Connected", "Disabled", 1), strings.Replace(good, "/owned/", "/foreign/", 1), good + row + "\n", strings.Replace(good, "(stdio)", "(http)", 1)} {
		if geminiMCPConnected([]byte(bad), expected) {
			t.Fatalf("qualified uncertain MCP output %q", bad)
		}
	}
}
