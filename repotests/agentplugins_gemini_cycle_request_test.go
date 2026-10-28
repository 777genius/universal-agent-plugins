//go:build linux

package pluginkitairepo_test

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

func geminiCycleDeclarations(value any, names []string) error {
	groups, ok := value.([]any)
	if !ok || len(groups) != 1 {
		return fmt.Errorf("Gemini cycle exact one declaration group")
	}
	group, ok := groups[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(group, "functionDeclarations", "") != nil {
		return fmt.Errorf("Gemini cycle declaration group fields")
	}
	declarations, ok := group["functionDeclarations"].([]any)
	if !ok || len(declarations) != len(names) {
		return fmt.Errorf("Gemini cycle exact zero/one/two declarations")
	}
	seen := map[string]bool{}
	for _, value := range declarations {
		declaration, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("Gemini cycle declaration object")
		}
		name, ok := declaration["name"].(string)
		known := false
		for _, want := range names {
			known = known || name == want
		}
		if !ok || !known || seen[name] || declaration["description"] != "Inspect only disposable fixture runtime facts." {
			return fmt.Errorf("Gemini cycle source-bound declaration name/description")
		}
		if err := geminiProjectDeclaration([]any{map[string]any{"functionDeclarations": []any{declaration}}}, name); err != nil {
			return err
		}
		seen[name] = true
	}
	return nil
}
func geminiCycleToolConfig(value any, names []string) error {
	config, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(config, "functionCallingConfig", "") != nil {
		return fmt.Errorf("Gemini cycle tool config")
	}
	row, ok := config["functionCallingConfig"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, "mode", "allowedFunctionNames") != nil || !geminiProjectEnum(row["mode"], "MODE_UNSPECIFIED AUTO ANY NONE VALIDATED") {
		return fmt.Errorf("Gemini cycle function config")
	}
	if raw, exists := row["allowedFunctionNames"]; exists {
		values, ok := raw.([]any)
		if !ok || len(values) != len(names) {
			return fmt.Errorf("Gemini cycle allowed function count")
		}
		got := make([]string, 0, len(values))
		for _, value := range values {
			text, ok := value.(string)
			if !ok {
				return fmt.Errorf("Gemini cycle function name type")
			}
			got = append(got, text)
		}
		want := append([]string{}, names...)
		sort.Strings(got)
		sort.Strings(want)
		if !reflect.DeepEqual(got, want) {
			return fmt.Errorf("Gemini cycle allowed function identity")
		}
	}
	return nil
}
func geminiCycleRequest(body []byte, q *geminiCyclePhase) ([]map[string]any, error) {
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "contents tools", "generationConfig systemInstruction safetySettings toolConfig cachedContent") != nil {
		return nil, fmt.Errorf("Gemini cycle request fields")
	}
	if err := geminiCycleDeclarations(m["tools"], q.tools); err != nil {
		return nil, err
	}
	parts, err := geminiProjectContents(m["contents"], q.tool, q.wireID, q.arguments())
	if err != nil {
		return nil, err
	}
	rows := m["contents"].([]any)
	envelope := map[string]any{}
	for key, item := range m {
		if key != "contents" {
			envelope[key] = item
		}
	}
	if len(q.requests) == 1 {
		q.initialContents = rows
		q.initialEnvelope = envelope
	} else {
		if !reflect.DeepEqual(envelope, q.initialEnvelope) || len(rows) != len(q.initialContents)+2 || !reflect.DeepEqual(rows[:len(q.initialContents)], q.initialContents) {
			return nil, fmt.Errorf("Gemini cycle request prefix/history drift")
		}
		for index, role := range []string{"model", "user"} {
			row, ok := rows[len(q.initialContents)+index].(map[string]any)
			if !ok || row["role"] != role || len(row["parts"].([]any)) != 1 {
				return nil, fmt.Errorf("Gemini cycle closed call/response rows")
			}
		}
	}
	if value, exists := m["generationConfig"]; exists {
		if err := geminiProjectGeneration(value); err != nil {
			return nil, err
		}
	}
	if value, exists := m["systemInstruction"]; exists {
		if _, err := geminiProjectContents([]any{value}, q.tool, q.wireID, q.arguments()); err != nil {
			return nil, err
		}
	}
	if value, exists := m["cachedContent"]; exists && !projectNativeRuntimeText(value, 1024) {
		return nil, fmt.Errorf("Gemini cycle cached content")
	}
	if value, exists := m["toolConfig"]; exists {
		if err := geminiCycleToolConfig(value, q.tools); err != nil {
			return nil, err
		}
	}
	if value, exists := m["safetySettings"]; exists {
		rows, ok := value.([]any)
		if !ok || len(rows) > 8 {
			return nil, fmt.Errorf("Gemini cycle safety settings")
		}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok || projectNativeRuntimeKeys(row, "category threshold", "") != nil || !geminiProjectEnum(row["category"], "HARM_CATEGORY_UNSPECIFIED HARM_CATEGORY_HARASSMENT HARM_CATEGORY_HATE_SPEECH HARM_CATEGORY_SEXUALLY_EXPLICIT HARM_CATEGORY_DANGEROUS_CONTENT HARM_CATEGORY_CIVIC_INTEGRITY") || !geminiProjectEnum(row["threshold"], "HARM_BLOCK_THRESHOLD_UNSPECIFIED BLOCK_LOW_AND_ABOVE BLOCK_MEDIUM_AND_ABOVE BLOCK_ONLY_HIGH BLOCK_NONE OFF") {
				return nil, fmt.Errorf("Gemini cycle exact safety fields")
			}
		}
	}
	return parts, nil
}
func geminiCycleHistory(parts []map[string]any, q *geminiCyclePhase, index int) (string, error) {
	calls, responses := 0, 0
	output := ""
	for _, part := range parts {
		if part["functionCall"] != nil {
			calls++
		}
		if value, ok := part["functionResponse"].(map[string]any); ok {
			responses++
			output = value["response"].(map[string]any)["output"].(string)
		}
	}
	if index == 0 {
		if calls != 0 || responses != 0 {
			return "", fmt.Errorf("Gemini cycle history before preselected call")
		}
		return "", nil
	}
	if q.operation == "none" || index != 1 || calls != 1 || responses != 1 {
		return "", fmt.Errorf("Gemini cycle exact history/call count")
	}
	prefix, suffix := "<untrusted_context>\n", "\n</untrusted_context>"
	if len(output) < len(prefix)+len(suffix) || output[:len(prefix)] != prefix || output[len(output)-len(suffix):] != suffix {
		return "", fmt.Errorf("Gemini cycle exact return wrapper")
	}
	inner := output[len(prefix) : len(output)-len(suffix)]
	if strings.Contains(inner, "<untrusted_context>") || strings.Contains(inner, "</untrusted_context>") {
		return "", fmt.Errorf("Gemini cycle duplicate return wrapper")
	}
	if _, err := projectNativeRuntimeJSON([]byte(inner)); err != nil {
		return "", err
	}
	return inner, nil
}
