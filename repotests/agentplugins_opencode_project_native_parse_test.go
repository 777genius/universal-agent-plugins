package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	if _, err = decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("JSON has trailing bytes")
	}
	return value, nil
}

type openCodeProjectSkill struct{ Name, Location, Content string }

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
func openCodeProjectCheckConfig(object map[string]any, urls map[string]string, absent []string) error {
	mcp, ok := object["mcp"].(map[string]any)
	if !ok {
		return fmt.Errorf("native config has no MCP object")
	}
	for name, url := range urls {
		server, ok := mcp[name].(map[string]any)
		if !ok || server["type"] != "remote" || server["url"] != url || server["enabled"] == false {
			return fmt.Errorf("native MCP source %q differs", name)
		}
	}
	for _, name := range absent {
		if _, ok := mcp[name]; ok {
			return fmt.Errorf("removed MCP key %q remains", name)
		}
	}
	return nil
}
