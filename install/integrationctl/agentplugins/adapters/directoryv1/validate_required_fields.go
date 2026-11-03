package directoryv1

import (
	"encoding/json"
	"fmt"
	"strings"
)

func requireRootFields(body []byte, fields ...string) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(body, &root); err != nil {
		return err
	}
	return requiredMap(root, fields...)
}

func requiredMap(value map[string]json.RawMessage, fields ...string) error {
	if value == nil {
		return fmt.Errorf("expected object")
	}
	for _, f := range fields {
		raw, ok := value[f]
		if !ok {
			return fmt.Errorf("missing required field %q", f)
		}
		if string(raw) == "null" {
			return fmt.Errorf("required field %q cannot be null", f)
		}
	}
	return nil
}
func isJSONNull(value json.RawMessage) bool { return strings.TrimSpace(string(value)) == "null" }
