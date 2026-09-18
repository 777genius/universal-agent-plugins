package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
)

// DecodeUniqueJSONValue decodes one JSON value and rejects a document whose
// object keys collide under Unicode case folding. Clients read these documents
// to decide identity and ownership, so a key that differs from another only by
// case is ambiguous evidence and must not be resolved silently by last-write.
//
// The decoder must have UseNumber set: numeric tokens are range-checked so an
// unused extension member cannot carry a value no consumer can represent.
func DecodeUniqueJSONValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	delimiter, isDelimiter := token.(json.Delim)
	if !isDelimiter {
		if number, ok := token.(json.Number); ok {
			if err := validateJSONNumber(number); err != nil {
				return nil, err
			}
		}
		return token, nil
	}
	switch delimiter {
	case '{':
		return decodeUniqueJSONObject(decoder)
	case '[':
		return decodeJSONArray(decoder)
	default:
		return nil, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
}

func decodeUniqueJSONObject(decoder *json.Decoder) (any, error) {
	object := make(map[string]any)
	foldedKeys := make(map[string]struct{})
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		if keyErr != nil {
			return nil, keyErr
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("JSON object key is not a string")
		}
		folded := FoldJSONKey(key)
		if _, duplicate := foldedKeys[folded]; duplicate {
			return nil, fmt.Errorf("duplicate or case-ambiguous JSON object key %q", key)
		}
		foldedKeys[folded] = struct{}{}
		value, valueErr := DecodeUniqueJSONValue(decoder)
		if valueErr != nil {
			return nil, valueErr
		}
		object[key] = value
	}
	closing, closingErr := decoder.Token()
	if closingErr != nil || closing != json.Delim('}') {
		return nil, fmt.Errorf("invalid JSON object")
	}
	return object, nil
}

func decodeJSONArray(decoder *json.Decoder) (any, error) {
	var array []any
	for decoder.More() {
		value, valueErr := DecodeUniqueJSONValue(decoder)
		if valueErr != nil {
			return nil, valueErr
		}
		array = append(array, value)
	}
	closing, closingErr := decoder.Token()
	if closingErr != nil || closing != json.Delim(']') {
		return nil, fmt.Errorf("invalid JSON array")
	}
	return array, nil
}

// validateJSONNumber keeps every numeric token, including unused extension
// members, inside the finite IEEE-754 range before anything can trust it.
func validateJSONNumber(number json.Number) error {
	if _, err := strconv.ParseFloat(number.String(), 64); err != nil {
		return fmt.Errorf("JSON number %q is outside the finite numeric range", number)
	}
	return nil
}

// FoldJSONKey produces a stable representative for each unicode.SimpleFold
// equivalence class, matching strings.EqualFold without pairwise comparisons.
func FoldJSONKey(key string) string {
	var folded strings.Builder
	folded.Grow(len(key))
	for _, current := range key {
		representative := current
		for next := unicode.SimpleFold(current); next != current; next = unicode.SimpleFold(next) {
			if next < representative {
				representative = next
			}
		}
		folded.WriteRune(representative)
	}
	return folded.String()
}

// DecodeStrictJSONObject decodes a whole document into an object, rejecting
// trailing data and a non-object root on top of the uniqueness contract.
func DecodeStrictJSONObject(body []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(body)))
	decoder.UseNumber()
	value, err := DecodeUniqueJSONValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("JSON document has trailing data")
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("JSON document must be an object")
	}
	return object, nil
}

// ReadJSONManifestName reads the declared name of a plugin manifest.
func ReadJSONManifestName(path string) (string, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value, err := DecodeStrictJSONObject(body)
	if err != nil {
		return "", err
	}
	name, ok := value["name"].(string)
	if !ok || strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("manifest %s has no valid name", path)
	}
	return name, nil
}

// CloneObject deep-copies a decoded JSON object so a projection can rewrite it
// without mutating the envelope every other client also projects from.
func CloneObject(source map[string]any) map[string]any {
	clone := make(map[string]any, len(source))
	for key, value := range source {
		clone[key] = cloneJSONValue(value)
	}
	return clone
}

func cloneJSONValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return CloneObject(typed)
	case []any:
		clone := make([]any, len(typed))
		for index, item := range typed {
			clone[index] = cloneJSONValue(item)
		}
		return clone
	case map[string]string:
		clone := make(map[string]string, len(typed))
		for key, item := range typed {
			clone[key] = item
		}
		return clone
	case []string:
		return append([]string(nil), typed...)
	case json.RawMessage:
		return append(json.RawMessage(nil), typed...)
	default:
		return value
	}
}

// WriteJSON writes an indented JSON document atomically.
func WriteJSON(path string, value any) error {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	return atomicfile.Write(path, body, 0o644)
}
