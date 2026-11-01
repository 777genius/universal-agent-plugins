package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"
)

// Refuse duplicate/case-ambiguous keys, excessive depth and trailing documents.
func openCodeProjectJSON(body []byte) (any, error) {
	if !utf8.Valid(body) {
		return nil, fmt.Errorf("invalid UTF-8 JSON")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var read func(int) (any, error)
	read = func(depth int) (any, error) {
		if depth > 64 {
			return nil, fmt.Errorf("JSON nesting exceeds limit")
		}
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return token, nil
		}
		switch delimiter {
		case '{':
			object := map[string]any{}
			seen := map[string]bool{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, fmt.Errorf("non-string key")
				}
				fold := strings.ToLower(name)
				if seen[fold] {
					return nil, fmt.Errorf("duplicate or ambiguous JSON key")
				}
				seen[fold] = true
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				object[name] = value
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, fmt.Errorf("invalid object end")
			}
			return object, nil
		case '[':
			array := []any{}
			for decoder.More() {
				value, err := read(depth + 1)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, fmt.Errorf("invalid array end")
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter")
		}
	}
	value, err := read(0)
	if err != nil {
		return nil, err
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("JSON has trailing bytes")
	}
	return value, nil
}

type openCodeProjectSkill struct{ Name, Location, Content string }

// Public progress is a closed source-derived contract. Native and Git stderr
// never use this allowance; repeated removal has its separate exact refusal.
func openCodeProjectProgress(label string, body []byte) bool {
	resolve := "Resolving and validating one Agent Plugin package for every selected target...\n"
	var expected string
	switch label {
	case "add-user":
		expected = resolve + "Installing uap-opencode-user-control for 1 selected clients...\n"
	case "add-A", "add-B", "add-same-user", "add-same-A", "add-json":
		expected = resolve + "Installing uap-commands-fixture for 1 selected clients...\n"
	case "repeat-add-A":
		// Existing same-scope binding dispatches runAddLoaded before the group
		// Installing line. The real public-source fixture measures this branch.
		expected = resolve + "Applying transactional client package...\n"
	case "unchanged-update-A", "update-A":
		expected = "Resolving and validating one updated Agent Plugin package for every selected target...\n"
	case "intact-repair-A", "missing-repair-A":
		expected = "Resolving and validating each unique exact installed package revision once...\n"
	case "remove-A", "remove-json":
		expected = ""
	default:
		return false
	}
	return string(body) == expected
}

func openCodeProjectSkills(body []byte) (map[string]openCodeProjectSkill, error) {
	value, err := openCodeProjectJSON(body)
	if err != nil {
		return nil, err
	}
	rows, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("native skill listing is not an array")
	}
	found := map[string]openCodeProjectSkill{}
	for _, row := range rows {
		object, ok := row.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("native skill row is not an object")
		}
		name, a := object["name"].(string)
		location, b := object["location"].(string)
		content, c := object["content"].(string)
		if !a || !b || !c || name == "" || location == "" || content == "" || found[name].Name != "" {
			return nil, fmt.Errorf("malformed or duplicate native skill row")
		}
		found[name] = openCodeProjectSkill{Name: name, Location: location, Content: content}
	}
	return found, nil
}
func openCodeProjectSamePath(actual, expected string) bool {
	if !filepath.IsAbs(actual) || filepath.Clean(actual) != actual || strings.ContainsRune(actual, 0) {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(actual, expected)
	}
	return actual == expected
}
func openCodeProjectSkillContent(body []byte) (string, error) {
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", fmt.Errorf("fixture frontmatter missing")
	}
	end := strings.Index(text[4:], "\n---\n")
	if end < 0 {
		return "", fmt.Errorf("fixture frontmatter end missing")
	}
	return strings.TrimSpace(text[4+end+5:]), nil
}
func openCodeProjectCheckSkills(found map[string]openCodeProjectSkill, expected map[string]openCodeProjectSkill, forbidden []string) error {
	for name, want := range expected {
		got, ok := found[name]
		if !ok || !openCodeProjectSamePath(got.Location, want.Location) || strings.TrimSpace(got.Content) != strings.TrimSpace(want.Content) {
			return fmt.Errorf("native skill %q does not match exact selected path/body", name)
		}
	}
	for _, name := range forbidden {
		if _, ok := found[name]; ok {
			return fmt.Errorf("native lists withheld/removed fixture skill %q", name)
		}
	}
	return nil
}
// OpenCode V2's public "debug config" returns an ordered source inventory,
// not the flattened V1 effective config object. Bind every source to the
// independently selected fixture paths before computing the expected
// lower-to-higher-priority MCP namespace. This proves source attribution,
// not MCP connection, model use or native plugin activation.
func openCodeProjectV2SourceConfig(body []byte, userDir, projectDir string, allowInline, requireProject bool) (map[string]any, error) {
	value, err := openCodeProjectJSON(body)
	if err != nil {
		return nil, err
	}
	rows, ok := value.([]any)
	if !ok || len(rows) < 1 || len(rows) > 12 {
		return nil, fmt.Errorf("V2 native source inventory is not a bounded array")
	}
	allowed := map[string]bool{}
	for _, base := range []string{userDir, projectDir} {
		for _, name := range []string{"opencode.json", "opencode.jsonc", ".opencode", ""} {
			allowed[filepath.Join(base, name)] = true
		}
	}
	seen := map[string]bool{}
	servers := map[string]any{}
	userDocument, projectDocument, inlineDocument := false, false, false
	for _, row := range rows {
		entry, ok := row.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native source entry is malformed")
		}
		kind, ok := entry["type"].(string)
		if !ok || (kind != "directory" && kind != "document") {
			return nil, fmt.Errorf("V2 native source type is unknown")
		}
		path, hasPath := entry["path"].(string)
		if raw, present := entry["path"]; present && !hasPath || hasPath && (!filepath.IsAbs(path) || filepath.Clean(path) != path || !allowed[path]) {
			_ = raw
			return nil, fmt.Errorf("V2 native source path escaped expected config directories")
		}
		if kind == "directory" {
			if !hasPath || len(entry) != 2 {
				return nil, fmt.Errorf("V2 native directory entry has unexpected members")
			}
			if seen[path] {
				return nil, fmt.Errorf("duplicate native config source")
			}
			seen[path] = true
			continue
		}
		if len(entry) != 3 && (len(entry) != 2 || hasPath) {
			return nil, fmt.Errorf("V2 native config document has unexpected members")
		}
		info, ok := entry["info"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native config document has no bounded info")
		}
		if !hasPath {
			if !allowInline || inlineDocument {
				return nil, fmt.Errorf("unrequested V2 inline config document")
			}
			inlineDocument = true
		} else {
			if seen[path] {
				return nil, fmt.Errorf("duplicate V2 native config document")
			}
			seen[path] = true
			if path == filepath.Join(userDir, "opencode.jsonc") || path == filepath.Join(userDir, "opencode.json") {
				userDocument = true
			}
			if path == filepath.Join(projectDir, "opencode.jsonc") || path == filepath.Join(projectDir, "opencode.json") {
				projectDocument = true
			}
		}
		mcp, exists := info["mcp"]
		if !exists {
			continue
		}
		config, ok := mcp.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native source MCP field is malformed")
		}
		for key, raw := range config {
			if key == "servers" {
				continue
			}
			// Timeout and other MCP policy members are valid in V2, but
			// only a concrete servers object can establish native source.
			if key != "timeout" {
				return nil, fmt.Errorf("V2 native source has a flat MCP member")
			}
			_ = raw
		}
		serverSet, exists := config["servers"]
		if !exists {
			continue
		}
		mapping, ok := serverSet.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native source servers member is malformed")
		}
		for name, source := range mapping {
			if _, ok := source.(map[string]any); !ok {
				return nil, fmt.Errorf("V2 native source server is malformed")
			}
			servers[name] = source
		}
	}
	if !userDocument || requireProject && !projectDocument || allowInline && !inlineDocument {
		return nil, fmt.Errorf("V2 native expected config sources were not observed")
	}
	return map[string]any{"mcp": map[string]any{"servers": servers}}, nil
}

func openCodeProjectConfig(body []byte) (map[string]any, error) {
	value, err := openCodeProjectJSON(body)
	if err != nil {
		return nil, err
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("native config is not an object")
	}
	return object, nil
}
// Read the actual V1/V2 MCP shape without treating a flattened test
// projection as native evidence. V2 declares servers under mcp.servers.
func openCodeProjectServers(object map[string]any) (map[string]any, error) {
	mcp, ok := object["mcp"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("native config has no MCP object")
	}
	switch os.Getenv("AGENTPLUGINS_OPENCODE_VERSION") {
	case "", "1.18.34":
		if _, exists := mcp["servers"]; exists {
			return nil, fmt.Errorf("V1 native MCP unexpectedly uses V2 server structure")
		}
		return mcp, nil
	case "2.2.5":
		servers, ok := mcp["servers"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("V2 native MCP has no servers object")
		}
		return servers, nil
	default:
		return nil, fmt.Errorf("unsupported native OpenCode version in project proof")
	}
}
func openCodeProjectCheckConfig(object map[string]any, urls map[string]string, absent []string) error {
	mcp, err := openCodeProjectServers(object)
	if err != nil {
		return err
	}
	for name, url := range urls {
		server, ok := mcp[name].(map[string]any)
		if !ok || server["type"] != "remote" || server["url"] != url {
			return fmt.Errorf("native MCP source %q differs", name)
		}
		if enabled, present := server["enabled"]; present {
			flag, valid := enabled.(bool)
			if !valid || !flag {
				return fmt.Errorf("native MCP enabled flag %q malformed or false", name)
			}
		}
		if disabled, present := server["disabled"]; present {
			flag, valid := disabled.(bool)
			if !valid || flag {
				return fmt.Errorf("native MCP disabled flag %q malformed or true", name)
			}
		}
	}
	for _, name := range absent {
		if _, ok := mcp[name]; ok {
			return fmt.Errorf("removed MCP key %q remains", name)
		}
	}
	return nil
}

// Early public remove errors have no JSON envelope: prepare returns before the
// renderer and main emits one exact fixed stderr line. This is not a fallback
// parser for failed native JSON.
func openCodeProjectAbsentRemoval(result projectNativeCaptureResult) bool {
	if result.Exit != 1 || result.ErrorKind != "exit" || result.Error == "" || len(result.Stdout) != 0 || result.Deadline || result.StdoutLimit || result.StderrLimit {
		return false
	}
	first := "agentplugins: installation \"" + commandsPluginName + "\" was not found\n"
	second := "agentplugins: plugin is not installed for target \"opencode\" in project scope; no target was changed\n"
	return string(result.Stderr) == first || string(result.Stderr) == second
}
