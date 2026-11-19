package kimi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var kimiWorkspaceID = regexp.MustCompile(`^(wd_[a-z0-9._-]+_[0-9a-f]{12}|wm_[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$`)

func (api *kimiSkillAPI) catalog(request domain.ActivationRequest, expected map[string]kimiSkillMetadata) error {
	meta, err := api.call(http.MethodGet, "/meta", nil)
	if err != nil {
		return err
	}
	if meta["server_version"] != request.Client.Version || meta["dangerous_bypass_auth"] != false || meta["backend"] != "v2" || !kimiText(meta["server_id"], 256, false) {
		return fmt.Errorf("kimi native server identity differs")
	}
	workspace, err := api.workspace(request)
	if err != nil {
		return err
	}
	data, err := api.call(http.MethodGet, "/workspaces/"+url.PathEscape(workspace)+"/skills", nil)
	if err != nil {
		return err
	}
	return kimiCatalogMatches(data, expected)
}

func (api *kimiSkillAPI) workspace(request domain.ActivationRequest) (string, error) {
	data, err := api.call(http.MethodGet, "/workspaces", nil)
	if err != nil {
		return "", err
	}
	rows, ok := data["items"].([]any)
	if !ok || len(data) != 1 || len(rows) > 512 {
		return "", fmt.Errorf("kimi workspace list differs")
	}
	root := request.Plan.TargetContext.ProjectRoot
	selected, err := kimiFindWorkspace(rows, request.Client.Version, root)
	if err != nil {
		return "", err
	}
	if selected != "" {
		return selected, nil
	}
	// This function is reachable only during acting completion. VerifyOnly
	// never starts the web process or reaches native workspace registration.
	if request.VerifyOnly {
		return "", fmt.Errorf("kimi workspace registration requires confirmed completion")
	}
	body := map[string]any{"root": root}
	if request.Client.Version == "2.5.2" {
		if request.NativeAttempt.BindingID == "" {
			return "", fmt.Errorf("kimi workspace registration requires a stable binding identity")
		}
		body["idempotency_key"] = request.NativeAttempt.BindingID
	}
	// Registration was requested; an invalid response cannot prove that its
	// native workspace mutation did not occur.
	api.registration = domain.NativeEffectUncertain
	row, err := api.call(http.MethodPost, "/workspaces", body)
	if err != nil || !kimiWorkspaceMatches(row, request.Client.Version, root) {
		return "", fmt.Errorf("kimi registered workspace differs")
	}
	api.registration = domain.NativeEffectCommitted
	id, _ := row["id"].(string)
	return id, nil
}

func kimiFindWorkspace(rows []any, version, root string) (string, error) {
	selected, seen := "", map[string]bool{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return "", fmt.Errorf("kimi workspace row differs")
		}
		id, _ := row["id"].(string)
		path, _ := row["root"].(string)
		if !kimiWorkspaceID.MatchString(id) || seen[id] || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return "", fmt.Errorf("kimi workspace identity differs")
		}
		seen[id] = true
		if strings.EqualFold(path, root) {
			if path != root || selected != "" || !kimiWorkspaceMatches(row, version, root) {
				return "", fmt.Errorf("kimi selected workspace is ambiguous")
			}
			selected = id
		}
	}
	return selected, nil
}

func kimiWorkspaceMatches(row map[string]any, version, root string) bool {
	id, _ := row["id"].(string)
	if row["root"] != root || len(id) > 256 || !kimiWorkspaceID.MatchString(id) || !kimiText(row["name"], 4096, false) || !kimiWorkspaceTimes(row) {
		return false
	}
	if version == "2.3.2" {
		return strings.HasPrefix(id, "wd_") && len(row) == 6
	}
	dirs, ok := row["additional_dirs"].([]any)
	return version == "2.5.2" && len(row) == 8 && ok && len(dirs) == 0 && (row["kind"] == "implicit" || row["kind"] == "managed")
}

func kimiWorkspaceTimes(row map[string]any) bool {
	count, ok := row["session_count"].(json.Number)
	n, err := count.Int64()
	if !ok || err != nil || n < 0 {
		return false
	}
	for _, key := range []string{"created_at", "last_opened_at"} {
		stamp, ok := row[key].(string)
		if !ok {
			return false
		}
		if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
			return false
		}
	}
	return true
}

func kimiCatalogMatches(data map[string]any, expected map[string]kimiSkillMetadata) error {
	rows, ok := data["skills"].([]any)
	if !ok || len(data) != 1 || len(rows) > 512 {
		return fmt.Errorf("kimi Skills catalog differs")
	}
	seen, found := map[string]bool{}, map[string]bool{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || !kimiCatalogRowShape(row) {
			return fmt.Errorf("kimi Skill row differs")
		}
		if row["source"] != "project" && row["source"] != "user" && row["source"] != "extra" && row["source"] != "builtin" {
			return fmt.Errorf("kimi Skill source differs")
		}
		name := row["name"].(string)
		key := shared.FoldJSONKey(name)
		if seen[key] {
			return fmt.Errorf("kimi Skill name is duplicated")
		}
		seen[key] = true
		if !kimiCatalogRowFields(row) {
			return fmt.Errorf("kimi Skill row has unqualified fields")
		}
		if selected, exists := expected[name]; exists {
			if row["path"] != selected.path || row["description"] != selected.description || row["source"] != "project" || len(row) != 4 {
				return fmt.Errorf("kimi selected Skill origin or metadata differs")
			}
			found[name] = true
		}
	}
	if len(found) != len(expected) {
		return fmt.Errorf("kimi selected Skill is absent from native discovery")
	}
	return nil
}

func kimiCatalogRowShape(row map[string]any) bool {
	return kimiText(row["name"], 256, false) && kimiText(row["description"], 8192, true) && kimiText(row["path"], 4096, false) && kimiText(row["source"], 32, false)
}

func kimiCatalogRowFields(row map[string]any) bool {
	for key := range row {
		if key != "name" && key != "description" && key != "path" && key != "source" && key != "type" && key != "disable_model_invocation" && key != "scopes" {
			return false
		}
	}
	return true
}
