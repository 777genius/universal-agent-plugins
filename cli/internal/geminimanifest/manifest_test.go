package geminimanifest

import (
	"strings"
	"testing"
)

func TestDecodeImportedExtensionRejectsMalformedFieldShapes(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "excludeTools wrong shape",
			body: `{"excludeTools":"run_shell_command(rm -rf)"}`,
			want: `Gemini extension field "excludeTools" must be an array of strings`,
		},
		{
			name: "settings wrong item shape",
			body: `{"settings":["release-profile"]}`,
			want: `Gemini extension field "settings" must contain JSON objects`,
		},
		{
			name: "settings missing required fields",
			body: `{"settings":[{"name":"release-profile"}]}`,
			want: `Gemini extension field "settings" contains an invalid object`,
		},
		{
			name: "settings invalid env var",
			body: `{"settings":[{"name":"release-profile","description":"profile","envVar":"BAD-VAR","sensitive":false}]}`,
			want: `settings envVar "BAD-VAR" must be a valid environment variable name`,
		},
		{
			name: "themes wrong shape",
			body: `{"themes":{"name":"release-dawn"}}`,
			want: `Gemini extension field "themes" must be an array of JSON objects`,
		},
		{
			name: "themes missing name",
			body: `{"themes":[{"background":{"primary":"#fff9f2"}}]}`,
			want: `themes objects require non-empty string name`,
		},
		{
			name: "themes missing tokens",
			body: `{"themes":[{"name":"release-dawn"}]}`,
			want: `themes objects require at least one theme token besides name`,
		},
		{
			name: "plan wrong shape",
			body: `{"plan":"bad"}`,
			want: `Gemini extension field "plan" must be a JSON object`,
		},
		{
			name: "mcpServers wrong shape",
			body: `{"mcpServers":["demo"]}`,
			want: `Gemini extension field "mcpServers" must be a JSON object`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeImportedExtension([]byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeImportedExtension error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestDecodeImportedExtensionPreservesCanonicalFieldsAndExtra(t *testing.T) {
	body := []byte(`{
	  "name":"demo",
	  "version":"0.2.0",
	  "description":"gemini demo",
	  "contextFileName":"TEAM.md",
	  "excludeTools":["run_shell_command(rm -rf)"],
	  "migratedTo":"https://github.com/example/gemini-demo-v2",
	  "plan":{"directory":".gemini/plans","retentionDays":7},
	  "settings":[{"name":"release-profile","description":"profile","envVar":"RELEASE_PROFILE","sensitive":false}],
	  "themes":[{"name":"release-dawn","background":{"primary":"#fff9f2"}}],
	  "mcpServers":{"demo":{"command":"demo","args":["serve"]}},
	  "x_galleryTopic":"gemini-cli-extension"
	}`)

	data, err := DecodeImportedExtension(body)
	if err != nil {
		t.Fatal(err)
	}
	if data.Name != "demo" || data.Version != "0.2.0" || data.Description != "gemini demo" {
		t.Fatalf("decoded identity = %+v", data)
	}
	if data.Meta.ContextFileName != "TEAM.md" || data.Meta.MigratedTo != "https://github.com/example/gemini-demo-v2" || data.Meta.PlanDirectory != ".gemini/plans" {
		t.Fatalf("decoded meta = %+v", data.Meta)
	}
	if len(data.Meta.ExcludeTools) != 1 || data.Meta.ExcludeTools[0] != "run_shell_command(rm -rf)" {
		t.Fatalf("decoded excludeTools = %+v", data.Meta.ExcludeTools)
	}
	if len(data.Settings) != 1 || len(data.Themes) != 1 || len(data.MCPServers) != 1 {
		t.Fatalf("decoded structured fields = %+v", data)
	}
	if _, ok := data.Extra["x_galleryTopic"]; !ok {
		t.Fatalf("expected extra x_galleryTopic, got %+v", data.Extra)
	}
	plan, ok := data.Extra["plan"].(map[string]any)
	if !ok || plan["retentionDays"] != float64(7) {
		t.Fatalf("expected plan extra retentionDays, got %+v", data.Extra["plan"])
	}
}
