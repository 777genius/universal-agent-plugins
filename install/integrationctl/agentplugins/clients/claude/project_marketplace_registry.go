package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func inspectClaudeProjectRegistry(ctx context.Context, body []byte, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	registered, err := claudeProjectMarketplaceRegistered(client.ConfigRoot, plan.ActivePath, plan.PhysicalArtifactID)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if registered && managed == nil {
		return clients.RegistryCollision, nil
	}
	observed := inspectClaudeMarketplacePlugin(body, plan.DeclaredName, plan.PhysicalArtifactID, plan.ActivePath, plan.TargetContext.ProjectRoot, client.ConfigRoot, "")
	if observed.cache != "" && managed != nil {
		if !registered {
			return clients.RegistryIndeterminate, fmt.Errorf("the Claude native plugin lost its exact marketplace source")
		}
		if err := verifyClaudeProjectBindings(client.ConfigRoot, plan.DeclaredName, plan.PhysicalArtifactID, plan.TargetContext.ProjectRoot, observed.cache); err != nil {
			return clients.RegistryIndeterminate, err
		}
		if err := verifyClaudeProjectCache(ctx, client.ConfigRoot, plan.DeclaredName, plan.PhysicalArtifactID, plan.TargetContext.ProjectRoot, observed.cache, managed.NativeObjects, false); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	if observed.sourceMissing {
		if managed == nil || !registered {
			return clients.RegistryIndeterminate, fmt.Errorf("the unavailable Claude source has no owned native registration")
		}
		if err := requireMissingClaudeProjectSource(plan.ActivePath); err != nil {
			return clients.RegistryIndeterminate, err
		}
		return clients.RegistryExpected, nil
	}
	return claudeProjectRegistryFinding(body, client, plan, managed), nil
}

// The listing is scoped to the selected project. Check the native global
// installation index as well before a scoped command can affect a marketplace
// the user independently reused from another project or scope.
func verifyClaudeProjectBindings(configRoot, name, physical, project, cache string) error {
	document, err := readClaudeJSONObject(configRoot, filepath.Join("plugins", "installed_plugins.json"))
	if os.IsNotExist(err) && cache == "" {
		return nil
	}
	if err != nil {
		return err
	}
	plugins, ok := document["plugins"].(map[string]any)
	if !ok || document["version"] != json.Number("2") {
		return fmt.Errorf("the Claude native installation index is not recognized")
	}
	marketplace := ProjectMarketplaceName(physical, project)
	found := false
	for id, value := range plugins {
		if !strings.HasSuffix(id, "@"+marketplace) {
			continue
		}
		rows, ok := value.([]any)
		if !ok || id != name+"@"+marketplace || len(rows) > 1 {
			return fmt.Errorf("the Claude project marketplace has another native binding")
		}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok || cache == "" || row["scope"] != "project" || row["projectPath"] != project || row["installPath"] != cache {
				return fmt.Errorf("the Claude native plugin has another project, scope, or cache binding")
			}
			found = true
		}
	}
	if found != (cache != "") {
		return fmt.Errorf("the Claude native listing differs from its installation index")
	}
	return nil
}

// This registry is written by Claude's own scoped marketplace command. Reading
// its exact directory source prevents adopting or removing a same-name source
// that the user replaced independently.
func claudeProjectMarketplaceRegistered(configRoot, activePath, physical string) (bool, error) {
	document, err := readClaudeJSONObject(configRoot, filepath.Join("plugins", "known_marketplaces.json"))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	project := filepath.Dir(filepath.Dir(filepath.Dir(activePath)))
	value, present := document[ProjectMarketplaceName(physical, project)]
	if !present {
		return false, nil
	}
	entry, ok := value.(map[string]any)
	if !ok {
		return false, fmt.Errorf("the Claude marketplace registration is not an object")
	}
	source, ok := entry["source"].(map[string]any)
	if !ok || source["source"] != "directory" || source["path"] != activePath || entry["installLocation"] != activePath {
		return false, fmt.Errorf("the Claude marketplace registration no longer points at the owned package")
	}
	return true, nil
}

func readClaudeJSONObject(root, relative string) (map[string]any, error) {
	path := filepath.Join(root, relative)
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nil, err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, fmt.Errorf("the Claude registry must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return nil, fmt.Errorf("the Claude registry changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(file, (1<<20)+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("the Claude registry read exceeded its bound")
	}
	return shared.DecodeStrictJSONObject(body)
}

func observeClaudeMarketplacePlugin(body []byte, name, physical, active, project, user, version string) projectListState {
	return inspectClaudeMarketplacePlugin(body, name, physical, active, project, user, version).state
}

type claudeMarketplaceObservation struct {
	state         projectListState
	cache         string
	sourceMissing bool
}

func inspectClaudeMarketplacePlugin(body []byte, name, physical, active, project, user, version string) claudeMarketplaceObservation {
	entries, ok := decodeProjectList(body)
	if !ok || len(body) > 1<<20 || name == "" || physical == "" || !exactAbsolute(active) || !exactAbsolute(project) || !exactAbsolute(user) {
		return claudeMarketplaceObservation{}
	}
	marketplace := ProjectMarketplaceName(physical, project)
	expected := name + "@" + marketplace
	result := claudeMarketplaceObservation{state: projectListAbsent}
	seen := map[string]bool{}
	for _, value := range entries {
		entry, ok := value.(map[string]any)
		if !ok {
			return claudeMarketplaceObservation{}
		}
		id, ok := entry["id"].(string)
		if !ok || id == "" {
			return claudeMarketplaceObservation{}
		}
		if id == "(suppressed)@skills-dir" {
			if _, valid := projectSuppression(entry); !valid || seen[id] {
				return claudeMarketplaceObservation{}
			}
			seen[id] = true
			continue
		}
		// Claude lists registrations from every project, marking entries from
		// other workspaces disabled. Only our exact generated id claims this
		// installation; unrelated project registrations must remain untouched.
		rowProject := project
		if path, present := entry["projectPath"].(string); present {
			rowProject = path
		}
		row, valid := parseProjectListRow(entry, rowProject)
		if !valid {
			return claudeMarketplaceObservation{}
		}
		identity := id + "\x00" + row.scope + "\x00" + row.path
		if seen[identity] {
			return claudeMarketplaceObservation{}
		}
		seen[identity] = true
		if id != expected {
			continue
		}
		if result.state != projectListAbsent {
			return claudeMarketplaceObservation{}
		}
		result = claudeMarketplaceCandidate(entry, row, projectListQuery{name: name, active: active, project: project, user: user}, marketplace, version)
	}
	return result
}

func claudeMarketplaceCandidate(entry map[string]any, row projectListRow, q projectListQuery, marketplace, version string) claudeMarketplaceObservation {
	observedVersion, valid := entry["version"].(string)
	if !valid || pathpolicy.ValidateLeafID(observedVersion) != nil {
		return claudeMarketplaceObservation{}
	}
	cache := filepath.Join(q.user, "plugins", "cache", marketplace, q.name, observedVersion)
	if row.scope != "project" || entry["projectPath"] != q.project || row.path != cache {
		return claudeMarketplaceObservation{state: projectListForeign}
	}
	if version != "" && observedVersion != version {
		return claudeMarketplaceObservation{}
	}
	if row.enabled && claudeMarketplaceSourceMissing(entry, marketplace) {
		return claudeMarketplaceObservation{state: projectListAbsent, cache: cache, sourceMissing: true}
	}
	enabled, valid := claudeMarketplaceFolderFields(entry, q.active, version, row.enabled)
	if !valid {
		return claudeMarketplaceObservation{}
	}
	for _, key := range []string{"errors", "notes"} {
		if values, present := entry[key]; present && len(values.([]any)) > 0 {
			return claudeMarketplaceObservation{}
		}
	}
	state := projectListDisabled
	if enabled {
		state = projectListEnabled
	}
	return claudeMarketplaceObservation{state: state, cache: cache}
}

func claudeMarketplaceFolderFields(entry map[string]any, active, version string, enabled bool) (bool, bool) {
	folder, hasFolder := entry["readFromFolder"]
	folderVersion, hasVersion := entry["folderVersion"]
	projectEnabled, hasEnabled := entry["projectEnabled"]
	// Claude 2.1.263 copies the package into its cache. Both this schema and
	// the newer read-through schema require the same independent cache digest.
	if !hasFolder && !hasVersion && !hasEnabled {
		return enabled, true
	}
	text, versionOK := folderVersion.(string)
	flag, enabledOK := projectEnabled.(bool)
	if !hasFolder || !hasVersion || !hasEnabled || folder != filepath.Join(active, claudeProjectBundleDirectory) || !versionOK || pathpolicy.ValidateLeafID(text) != nil || !enabledOK || version != "" && text != version {
		return false, false
	}
	return enabled && flag, true
}
