package platformexec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func marshalJSON(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(body, '\n'), nil
}

func mustJSON(value any) []byte {
	body, err := marshalJSON(value)
	if err != nil {
		panic(err)
	}
	return body
}

func jsonDocumentsEqual(left, right any) bool {
	lb, err := json.Marshal(left)
	if err != nil {
		return false
	}
	rb, err := json.Marshal(right)
	if err != nil {
		return false
	}
	return bytes.Equal(lb, rb)
}

func decodeJSONObject(body []byte, label string) (map[string]any, error) {
	if err := rejectDuplicateJSONKeys(body, label); err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("%s is invalid JSON: %w", label, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, nil
}

// rejectDuplicateJSONKeys rejects repeated object keys: json.Unmarshal silently
// keeps the last one while client parsers may keep the first, so a duplicate is
// a parser differential waiting to happen.
func rejectDuplicateJSONKeys(body []byte, label string) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	type frame struct {
		isObject  bool
		expectKey bool
		seen      map[string]bool
	}
	var stack []*frame
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s is invalid JSON: %w", label, err)
		}
		switch typed := token.(type) {
		case json.Delim:
			switch typed {
			case '{':
				if len(stack) > 0 {
					stack[len(stack)-1].expectKey = true
				}
				stack = append(stack, &frame{isObject: true, expectKey: true, seen: map[string]bool{}})
			case '[':
				if len(stack) > 0 {
					stack[len(stack)-1].expectKey = true
				}
				stack = append(stack, &frame{})
			case '}', ']':
				if len(stack) == 0 {
					return fmt.Errorf("%s is invalid JSON: unbalanced delimiters", label)
				}
				stack = stack[:len(stack)-1]
			}
		case string:
			if len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			if top.isObject && top.expectKey {
				if top.seen[typed] {
					return fmt.Errorf("%s is invalid JSON: duplicate object key %q", label, typed)
				}
				top.seen[typed] = true
				top.expectKey = false
				continue
			}
			if top.isObject {
				top.expectKey = true
			}
		default:
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				if top.isObject {
					top.expectKey = true
				}
			}
		}
	}
}

func jsonStringArray(values []any) []string {
	var out []string
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return out
}

func parseMarkdownFrontmatterDocument(body []byte, label string) (map[string]any, string, error) {
	src := strings.ReplaceAll(string(body), "\r\n", "\n")
	src = strings.ReplaceAll(src, "\r", "\n")
	src = strings.TrimPrefix(src, "\ufeff")
	if !strings.HasPrefix(src, "---\n") {
		return nil, "", fmt.Errorf("%s must start with YAML frontmatter", label)
	}
	rest := strings.TrimPrefix(src, "---\n")
	idx := strings.Index(rest, "\n---\n")
	if idx < 0 {
		if strings.HasSuffix(rest, "\n---") {
			idx = len(rest) - len("\n---")
		} else {
			return nil, "", fmt.Errorf("%s frontmatter terminator not found", label)
		}
	}
	frontmatter := map[string]any{}
	if err := yaml.Unmarshal([]byte(rest[:idx]), &frontmatter); err != nil {
		return nil, "", fmt.Errorf("parse %s frontmatter: %w", label, err)
	}
	bodyOffset := idx + len("\n---\n")
	if bodyOffset > len(rest) {
		bodyOffset = len(rest)
	}
	return frontmatter, strings.TrimSpace(rest[bodyOffset:]), nil
}

func mustYAML(value any) []byte {
	body, err := yaml.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}

func readYAMLDoc[T any](root string, rel string) (T, bool, error) {
	var out T
	if strings.TrimSpace(rel) == "" {
		return out, false, nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return out, false, err
	}
	if err := yaml.Unmarshal(body, &out); err != nil {
		return out, true, err
	}
	return out, true, nil
}
