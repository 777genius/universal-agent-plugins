package pi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
)

// piTrustSurfaces are the .pi directories whose entries must all be owned.
var piTrustSurfaces = []string{"extensions", "skills", "prompts", "themes"}

// piTrustBlocker mirrors the hook's refusals only to explain them; the hook
// (piTrustHookSource) remains the sole runtime authority. An empty result
// means the hook answers yes for this recorded project.
func piTrustBlocker(kernel nativeconfig.Kernel, user, project string) (string, error) {
	_, _, projectsPath := piTrustHookPaths(user)
	snapshot, err := kernel.ReadExactFile(projectsPath)
	if err != nil {
		return "", err
	}
	document, err := decodePiTrust(snapshot)
	if err != nil {
		return "", err
	}
	if blocker, err := piTrustPolicyBlocker(user, project); blocker != "" || err != nil {
		return blocker, err
	}
	config := filepath.Join(project, ".pi")
	if piTrustKind(config) != "directory" {
		return ".pi is not a real directory", nil
	}
	owned, conflict := piTrustLiveNames(config, document.Projects[project])
	if conflict != "" {
		return conflict, nil
	}
	if blocker, err := piTrustConfigBlocker(config, owned); blocker != "" || err != nil {
		return blocker, err
	}
	if blocker, err := piTrustMCPBlocker(filepath.Join(config, "mcp.json"), owned["mcp_servers"]); blocker != "" || err != nil {
		return blocker, err
	}
	return piTrustAgentsSkillsBlocker(project), nil
}

// piTrustLiveNames unions recorded name digests of installations whose
// managed package directory still exists, as the hook does.
func piTrustLiveNames(config string, owners map[string]piTrustOwner) (map[string]map[string]string, string) {
	owned := map[string]map[string]string{"mcp_servers": {}}
	for _, surface := range piTrustSurfaces {
		owned[surface] = map[string]string{}
	}
	for artifact, owner := range owners {
		if piTrustKind(filepath.Join(config, "extensions", artifact)) != "directory" {
			continue
		}
		for surface, names := range map[string]map[string]string{"extensions": owner.Extensions, "skills": owner.Skills, "mcp_servers": owner.MCPServers} {
			for name, digest := range names {
				if recorded, ok := owned[surface][name]; ok && recorded != digest {
					return nil, fmt.Sprintf("two UAP installations record different content for %s %q", surface, name)
				}
				owned[surface][name] = digest
			}
		}
	}
	return owned, ""
}

func piTrustConfigBlocker(config string, owned map[string]map[string]string) (string, error) {
	locks, err := nativeconfig.WriterLockPaths(nativeconfig.Paths{JSON: filepath.Join(config, "mcp.json")}, nativeconfig.CodecMCPServers)
	if err != nil {
		return "", err
	}
	allowed := map[string]bool{"mcp.json": true}
	for _, path := range append(locks, piTrustSurfaces...) {
		allowed[filepath.Base(path)] = true
	}
	entries, err := os.ReadDir(config)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !allowed[entry.Name()] {
			return fmt.Sprintf(".pi/%s is not managed by UAP", entry.Name()), nil
		}
	}
	for _, surface := range piTrustSurfaces {
		directory := filepath.Join(config, surface)
		if found := piTrustKind(directory); found == "absent" {
			continue
		} else if found != "directory" {
			return fmt.Sprintf(".pi/%s is not a real directory", surface), nil
		}
		names, err := os.ReadDir(directory)
		if err != nil {
			return "", err
		}
		for _, name := range names {
			recorded, ok := owned[surface][name.Name()]
			if !ok {
				return fmt.Sprintf(".pi/%s/%s is not managed by UAP", surface, name.Name()), nil
			}
			if digest, err := piTrustTreeDigest(filepath.Join(directory, name.Name())); err != nil || digest != recorded {
				return fmt.Sprintf(".pi/%s/%s changed after UAP recorded its content", surface, name.Name()), nil //nolint:nilerr // A refusal is an explanation.
			}
		}
	}
	return "", nil
}

func piTrustMCPBlocker(path string, owned map[string]string) (string, error) {
	switch piTrustKind(path) {
	case "absent":
		return "", nil
	case "file":
	default:
		return ".pi/mcp.json is not a regular file", nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var document map[string]json.RawMessage
	if !piTrustDecodes(body, &document) || document == nil {
		return ".pi/mcp.json is not a JSON object", nil
	}
	for key := range document {
		if key != "mcpServers" {
			return fmt.Sprintf(".pi/mcp.json key %q is not managed by UAP", key), nil
		}
	}
	servers, err := piTrustMCPServers(path)
	if err != nil {
		return err.Error(), nil //nolint:nilerr // A refusal is an explanation.
	}
	for name, entry := range servers {
		recorded, ok := owned[name]
		if !ok {
			return fmt.Sprintf(".pi/mcp.json server %q is not managed by UAP", name), nil
		}
		if digest, err := piTrustValueDigest(entry); err != nil || digest != recorded {
			return fmt.Sprintf(".pi/mcp.json server %q changed after UAP recorded it", name), nil //nolint:nilerr // A refusal is an explanation.
		}
	}
	return "", nil
}

// Pi loads .agents/skills from the project and its ancestors once trusted,
// except the user's own ~/.agents/skills.
func piTrustAgentsSkillsBlocker(project string) string {
	home := os.Getenv("HOME")
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	if resolved, err := filepath.EvalSymlinks(home); err == nil {
		home = resolved
	}
	user := piTrustComparable(filepath.Join(home, ".agents", "skills"))
	for directory := project; ; directory = filepath.Dir(directory) {
		skills := filepath.Join(directory, ".agents", "skills")
		if piTrustComparable(skills) != user && piTrustKind(skills) != "absent" {
			return skills + " exposes project Skills"
		}
		if filepath.Dir(directory) == directory {
			return ""
		}
	}
}

// piTrustPolicyBlocker reports decisions Pi owns before the hook is asked.
func piTrustPolicyBlocker(user, project string) (string, error) {
	var settings struct {
		DefaultProjectTrust any `json:"defaultProjectTrust"`
	}
	if body, err := os.ReadFile(filepath.Join(user, "settings.json")); err == nil {
		if !piTrustDecodes(body, &settings) {
			return "Pi's settings.json is unreadable", nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if settings.DefaultProjectTrust == "always" || settings.DefaultProjectTrust == "never" {
		return fmt.Sprintf("Pi's defaultProjectTrust setting is %q", settings.DefaultProjectTrust), nil
	}
	saved := map[string]json.RawMessage{}
	if body, err := os.ReadFile(filepath.Join(user, "trust.json")); err == nil {
		if !piTrustDecodes(body, &saved) {
			return "Pi's trust.json is unreadable", nil
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}
	for directory := project; ; directory = filepath.Dir(directory) {
		for path, decision := range saved {
			if (string(decision) == "true" || string(decision) == "false") && piTrustComparable(path) == piTrustComparable(directory) {
				return "Pi keeps your saved trust decision for " + path, nil
			}
		}
		if filepath.Dir(directory) == directory {
			return "", nil
		}
	}
}

// piTrustDecodes reads Pi's JSON documents as Pi does, after an optional
// BOM. An undecodable document is an explanation, not an operation failure.
func piTrustDecodes(body []byte, target any) bool {
	return json.Unmarshal(bytes.TrimPrefix(body, []byte("\xef\xbb\xbf")), target) == nil
}

// piTrustComparable matches Pi's path keys: resolved, and case-folded on
// Windows only.
func piTrustComparable(path string) string {
	if absolute, err := filepath.Abs(path); err == nil {
		path = absolute
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func piTrustKind(path string) string {
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return "absent"
	case err != nil:
		return "error"
	case info.IsDir():
		return "directory"
	case info.Mode().IsRegular():
		return "file"
	default:
		return "other"
	}
}
