package platformexec

import (
	"bytes"
	"encoding/json"
	"errors"
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
	var stack jsonKeyStack
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s is invalid JSON: %w", label, err)
		}
		if err := stack.observe(token); err != nil {
			return fmt.Errorf("%s is invalid JSON: %w", label, err)
		}
	}
}

type jsonKeyFrame struct {
	isObject  bool
	expectKey bool
	seen      map[string]bool
}

// jsonKeyStack walks decoded tokens frame by frame so each step stays small.
type jsonKeyStack []*jsonKeyFrame

func (s *jsonKeyStack) observe(token json.Token) error {
	switch typed := token.(type) {
	case json.Delim:
		return s.observeDelim(typed)
	case string:
		return s.observeString(typed)
	default:
		s.markValue()
		return nil
	}
}

func (s *jsonKeyStack) observeDelim(delim json.Delim) error {
	switch delim {
	case '{':
		s.push(&jsonKeyFrame{isObject: true, expectKey: true, seen: map[string]bool{}})
	case '[':
		s.push(&jsonKeyFrame{})
	case '}', ']':
		if len(*s) == 0 {
			return errors.New("unbalanced delimiters")
		}
		*s = (*s)[:len(*s)-1]
	}
	return nil
}

func (s *jsonKeyStack) push(frame *jsonKeyFrame) {
	if n := len(*s); n > 0 {
		(*s)[n-1].expectKey = true
	}
	*s = append(*s, frame)
}

func (s *jsonKeyStack) observeString(value string) error {
	if len(*s) == 0 {
		return nil
	}
	top := (*s)[len(*s)-1]
	if !top.isObject {
		return nil
	}
	if !top.expectKey {
		top.expectKey = true
		return nil
	}
	if top.seen[value] {
		return fmt.Errorf("duplicate object key %q", value)
	}
	top.seen[value] = true
	top.expectKey = false
	return nil
}

func (s *jsonKeyStack) markValue() {
	if n := len(*s); n > 0 && (*s)[n-1].isObject {
		(*s)[n-1].expectKey = true
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
