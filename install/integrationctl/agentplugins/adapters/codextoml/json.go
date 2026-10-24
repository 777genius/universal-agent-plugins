package codextoml

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

func decodeTOMLJSON(body []byte) (map[string]any, error) {
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("codex MCP entry has trailing JSON")
	}
	converted, err := tomlJSONValue(result)
	if err != nil {
		return nil, err
	}
	result, _ = converted.(map[string]any)
	return result, nil
}

func tomlJSONValue(value any) (any, error) {
	switch typed := value.(type) {
	case json.Number:
		if strings.ContainsAny(string(typed), ".eE") {
			return strconv.ParseFloat(string(typed), 64)
		}
		return strconv.ParseInt(string(typed), 10, 64)
	case map[string]any:
		for key, child := range typed {
			converted, err := tomlJSONValue(child)
			if err != nil {
				return nil, err
			}
			typed[key] = converted
		}
	case []any:
		for i, child := range typed {
			converted, err := tomlJSONValue(child)
			if err != nil {
				return nil, err
			}
			typed[i] = converted
		}
	}
	return value, nil
}
