package clients

import (
	"reflect"
	"testing"
)

func TestSessionChildEnvironmentRemovesWindowsCaseAliases(t *testing.T) {
	plan := SessionPlan{Environment: map[string]string{
		"CLINE_MCP_SETTINGS_PATH": "/project/mcp.json",
		"CLINE_NO_AUTO_UPDATE": "1",
	}}
	base := []string{
		"HOME=/original", "cline_mcp_settings_path=/foreign",
		"ClInE_No_AuTo_UpDaTe=0", "KEEP=original",
	}
	for _, test := range []struct {
		name            string
		caseInsensitive bool
		want            []string
	}{
		{
			name: "Windows", caseInsensitive: true,
			want: []string{"HOME=/original", "KEEP=original",
				"CLINE_MCP_SETTINGS_PATH=/project/mcp.json", "CLINE_NO_AUTO_UPDATE=1"},
		},
		{
			name: "POSIX", caseInsensitive: false,
			want: []string{"HOME=/original", "cline_mcp_settings_path=/foreign",
				"ClInE_No_AuTo_UpDaTe=0", "KEEP=original",
				"CLINE_MCP_SETTINGS_PATH=/project/mcp.json", "CLINE_NO_AUTO_UPDATE=1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := plan.childEnvironment(base, test.caseInsensitive); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("environment override differs: got %q, want %q", got, test.want)
			}
		})
	}
	if base[1] != "cline_mcp_settings_path=/foreign" || len(plan.Environment) != 2 {
		t.Fatal("session environment inputs mutated")
	}
}
