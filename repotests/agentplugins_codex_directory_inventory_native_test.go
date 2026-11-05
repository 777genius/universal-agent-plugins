package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/codex"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func directorySiteCodexInventory(t *testing.T, f *nativeFixture, codexBin string, binding domain.ClientBinding, present bool, label string) {
	t.Helper()
	pluginID := directorySitePlugin + "@" + shared.ManagedMarketplaceName(binding.PhysicalArtifact)
	result, err := nativeCommand(t, f, codexBin, label+"-list", "plugin", "list", "--json")
	if err != nil {
		t.Fatalf("Codex plugin list: %v %s", err, result)
	}
	var listing struct {
		Installed []struct {
			ID        string `json:"pluginId"`
			Installed bool   `json:"installed"`
			Enabled   bool   `json:"enabled"`
			Source    struct {
				Path string `json:"path"`
			} `json:"source"`
		} `json:"installed"`
	}
	if err := json.Unmarshal(result, &listing); err != nil {
		t.Fatal(err)
	}
	marketplace, ok := strings.CutPrefix(pluginID, directorySitePlugin+"@")
	if !ok || marketplace == "" || listing.Installed == nil {
		t.Fatalf("Codex Directory native listing is not authoritative: %s", result)
	}
	status := codex.PluginStatusFromList(result, directorySitePlugin, marketplace)
	if status == codex.StatusUnknown ||
		(present && status != codex.StatusInstalled) || (!present && status != codex.StatusAbsent) {
		t.Fatalf("Codex Directory native listing status %v, want present=%v: %s", status, present, result)
	}
	found := 0
	for _, entry := range listing.Installed {
		if entry.ID == pluginID {
			if !entry.Enabled || !entry.Installed || entry.Source.Path != binding.TargetLocator {
				t.Fatalf("Codex Directory source/path mismatch: %+v", entry)
			}
			found++
		}
	}
	if (found == 1) != present || found > 1 {
		t.Fatalf("Codex plugin presence %d, want %t: %s", found, present, result)
	}
	r := startNativeRPC(t, f, codexBin, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_directory_fixture", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var skills struct {
		Data []struct {
			CWD    string            `json:"cwd"`
			Errors []json.RawMessage `json:"errors"`
			Skills []struct {
				Name     string `json:"name"`
				PluginID string `json:"pluginId"`
				Path     string `json:"path"`
				Enabled  bool   `json:"enabled"`
			} `json:"skills"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(r.must(t, "skills/list", map[string]any{"cwds": []string{f.Project}, "forceReload": true}), &skills); err != nil {
		t.Fatal(err)
	}
	if skills.NextCursor != nil || len(skills.Data) != 1 ||
		!openCodeSamePath(skills.Data[0].CWD, f.Project) || len(skills.Data[0].Errors) != 0 || skills.Data[0].Skills == nil {
		t.Fatalf("Codex Directory skills/list missing selected cwd or contained errors: %+v", skills)
	}
	found = 0
	for _, group := range skills.Data {
		for _, skill := range group.Skills {
			if skill.PluginID != pluginID {
				continue
			}
			if skill.Name != directorySitePlugin+":"+directorySiteSkill || !skill.Enabled ||
				!strings.HasSuffix(filepath.ToSlash(skill.Path), "/skills/"+directorySiteSkill+"/SKILL.md") ||
				!strings.Contains(filepath.ToSlash(skill.Path), "/plugins/cache/") {
				t.Fatalf("Codex Directory Skill name/path mismatch: %+v", skill)
			}
			nativeRequireContained(t, f.Root, skill.Path)
			if _, err := os.Stat(skill.Path); err != nil {
				t.Fatal(err)
			}
			found++
		}
	}
	if (found == 1) != present || found > 1 {
		t.Fatalf("Codex Directory native skills count=%d, want present=%t", found, present)
	}
}

