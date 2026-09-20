package codexconfig

import (
	"strings"
	"testing"
)

func TestDecodeImportedConfigRejectsMalformedFieldShapes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "model_not_string",
			body: "model = true\n",
			want: `Codex config field "model" must be a string`,
		},
		{
			name: "notify_not_array",
			body: "notify = \"./bin/demo notify\"\n",
			want: `Codex config field "notify" must be an array of non-empty strings`,
		},
		{
			name: "notify_mixed_array",
			body: "notify = [\"./bin/demo\", 7]\n",
			want: `Codex config field "notify" must contain non-empty strings`,
		},
		{
			name: "notify_empty_string",
			body: "notify = [\"./bin/demo\", \"\"]\n",
			want: `Codex config field "notify" must contain non-empty strings`,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeImportedConfig([]byte(tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("DecodeImportedConfig error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestDecodeImportedConfigPreservesCanonicalFieldsAndExtra(t *testing.T) {
	t.Parallel()
	body := []byte("model = \"gpt-5.4-mini\"\nnotify = [\"./bin/demo\", \"notify\", \"extra\"]\napproval_policy = \"never\"\n[ui]\nverbose = true\n")

	data, err := DecodeImportedConfig(body)
	if err != nil {
		t.Fatal(err)
	}
	if data.Model != "gpt-5.4-mini" {
		t.Fatalf("model = %q", data.Model)
	}
	if len(data.Notify) != 3 || data.Notify[0] != "./bin/demo" || data.Notify[2] != "extra" {
		t.Fatalf("notify = %#v", data.Notify)
	}
	if got := strings.TrimSpace(data.Extra["approval_policy"].(string)); got != "never" {
		t.Fatalf("approval_policy = %q", got)
	}
	ui, ok := data.Extra["ui"].(map[string]any)
	if !ok || ui == nil || ui["verbose"] != true {
		t.Fatalf("extra = %#v", data.Extra)
	}
}
