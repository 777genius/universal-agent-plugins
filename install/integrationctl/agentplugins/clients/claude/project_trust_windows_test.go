package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

func TestProjectClaudeWindowsTrustUsesNativeSlashProjectKey(t *testing.T) {
	request := newClaudeMarketplaceRequest(t)
	project, err := claudeTrustableProject(request)
	if err != nil || project != request.Plan.TargetContext.ProjectRoot || !strings.Contains(project, `\`) {
		t.Fatalf("native Windows project key=%q: %v", project, err)
	}
	body, err := renderClaudeProjectTrust([]byte(`{}`), project)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(request.Client.ConfigRoot, ".claude.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if trusted, err := claudeProjectTrustRecorded(clients.Env{NativeConfig: nativeconfig.New()}, request); err != nil || !trusted {
		t.Fatalf("native slash trust key was not observed: trusted=%v err=%v", trusted, err)
	}
	for _, path := range []string{`C:\Selected\Project`, `c:\Selected\Project`, `\\?\C:\Selected\Project`} {
		key := filepath.ToSlash(path)
		body, err := renderClaudeProjectTrust([]byte(`{}`), path)
		var profile struct {
			Projects map[string]struct {
				Trusted bool `json:"hasTrustDialogAccepted"`
			} `json:"projects"`
		}
		if err != nil || json.Unmarshal(body, &profile) != nil || len(profile.Projects) != 1 || !profile.Projects[key].Trusted {
			t.Fatalf("native key %q was not preserved: %s; %v", key, body, err)
		}
		if _, exists := profile.Projects[path]; exists {
			t.Fatalf("native backslash alias was written for %q", path)
		}
	}
}
