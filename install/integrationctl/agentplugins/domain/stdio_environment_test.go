package domain

import "testing"

func TestReservedStdioEnvKeyPlatformRules(t *testing.T) {
	for _, tc := range []struct {
		name string
		env any
		windows bool
		want string
	}{
		{"none", nil, true, ""},
		{"canonical-any", map[string]any{"PLUGIN_ROOT": "bad"}, false, "PLUGIN_ROOT"},
		{"canonical-string", map[string]string{"PLUGIN_DATA": "bad"}, true, "PLUGIN_DATA"},
		{"windows-root-any", map[string]any{"plugin_root": "bad"}, true, "PLUGIN_ROOT"},
		{"windows-data-string", map[string]string{"Plugin_Data": "bad"}, true, "PLUGIN_DATA"},
		{"unix-root-distinct", map[string]any{"plugin_root": "allowed"}, false, ""},
		{"unix-data-distinct", map[string]string{"Plugin_Data": "allowed"}, false, ""},
		{"both-deterministic", map[string]string{"plugin_data": "bad", "Plugin_ROOT": "bad"}, true, "PLUGIN_ROOT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ReservedStdioEnvKey(tc.env, tc.windows); got != tc.want {
				t.Fatalf("reserved key = %q, want %q", got, tc.want)
			}
		})
	}
}
