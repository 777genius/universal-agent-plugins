//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"time"
)

// Source: google-gemini/gemini-cli@6006c14a4decfe4b891c6263ad6ad5992e404b5b,
// packages/core/src/tools/tools.ts blob ab2870984aba77e77fc5bacb25009cd47698582e.
const geminiProjectWaitDescription = "Set to true to wait for all previously requested tools in this turn to complete before starting. Set to false (or omit) to run in parallel. Use true whe."
const geminiProjectModel = "gemini-2.5-flash"

func geminiProjectContents(value any, tool, id string, args map[string]any) ([]map[string]any, error) {
	rows, ok := value.([]any)
	if !ok || len(rows) == 0 || len(rows) > 128 {
		return nil, fmt.Errorf("Gemini contents")
	}
	var parts []map[string]any
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "parts", "role") != nil {
			return nil, fmt.Errorf("Gemini content fields")
		}
		if role, exists := row["role"]; exists && role != "user" && role != "model" {
			return nil, fmt.Errorf("Gemini content role")
		}
		items, ok := row["parts"].([]any)
		if !ok || len(items) == 0 || len(items) > 256 {
			return nil, fmt.Errorf("Gemini parts")
		}
		for _, value := range items {
			part, ok := value.(map[string]any)
			if !ok || len(part) != 1 {
				return nil, fmt.Errorf("Gemini part union")
			}
			if text, ok := part["text"]; ok {
				if !projectNativeRuntimeText(text, projectNativeRuntimeBodyLimit) {
					return nil, fmt.Errorf("Gemini text part")
				}
			} else if value, ok := part["functionCall"]; ok {
				call, ok := value.(map[string]any)
				if !ok || projectNativeRuntimeKeys(call, "id name args", "") != nil || call["id"] != id || call["name"] != tool || !reflect.DeepEqual(call["args"], args) {
					return nil, fmt.Errorf("Gemini function call identity/arguments")
				}
			} else if value, ok := part["functionResponse"]; ok {
				response, ok := value.(map[string]any)
				if !ok || projectNativeRuntimeKeys(response, "id name response", "") != nil || response["id"] != id || response["name"] != tool {
					return nil, fmt.Errorf("Gemini function response identity")
				}
				output, ok := response["response"].(map[string]any)
				if !ok || projectNativeRuntimeKeys(output, "output", "") != nil || !projectNativeRuntimeText(output["output"], projectNativeRuntimeBodyLimit) {
					return nil, fmt.Errorf("Gemini function output")
				}
			} else {
				return nil, fmt.Errorf("unadmitted Gemini part kind")
			}
			parts = append(parts, part)
		}
	}
	return parts, nil
}
func geminiProjectGeneration(value any) error {
	m, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "", "temperature topP topK candidateCount maxOutputTokens stopSequences seed presencePenalty frequencyPenalty responseMimeType thinkingConfig") != nil {
		return fmt.Errorf("Gemini generation fields")
	}
	for key, value := range m {
		switch key {
		case "temperature", "topP", "topK", "candidateCount", "maxOutputTokens", "seed", "presencePenalty", "frequencyPenalty":
			bounds := map[string][2]float64{"temperature": {0, 2}, "topP": {0, 1}, "topK": {0, 2048}, "candidateCount": {1, 8}, "maxOutputTokens": {1, 1 << 20}, "seed": {-(1 << 31), 1 << 31}, "presencePenalty": {-2, 2}, "frequencyPenalty": {-2, 2}}
			b := bounds[key]
			integer := key == "topK" || key == "candidateCount" || key == "maxOutputTokens" || key == "seed"
			if !projectNativeRuntimeNumber(value, b[0], b[1], integer) {
				return fmt.Errorf("Gemini generation number")
			}
		case "responseMimeType":
			if value != "text/plain" && value != "application/json" {
				return fmt.Errorf("Gemini response MIME")
			}
		case "stopSequences":
			rows, ok := value.([]any)
			if !ok || len(rows) > 16 {
				return fmt.Errorf("Gemini stop sequences")
			}
			for _, row := range rows {
				if !projectNativeRuntimeText(row, 256) {
					return fmt.Errorf("Gemini stop string")
				}
			}
		case "thinkingConfig":
			row, ok := value.(map[string]any)
			if !ok || projectNativeRuntimeKeys(row, "", "includeThoughts thinkingBudget thinkingLevel") != nil {
				return fmt.Errorf("Gemini thinking fields")
			}
			for key, v := range row {
				switch key {
				case "includeThoughts":
					if _, ok := v.(bool); !ok {
						return fmt.Errorf("Gemini thinking bool")
					}
				case "thinkingBudget":
					if !projectNativeRuntimeNumber(v, -1, 32768, true) {
						return fmt.Errorf("Gemini thinking budget")
					}
				case "thinkingLevel":
					if v != "THINKING_LEVEL_UNSPECIFIED" && v != "LOW" && v != "HIGH" {
						return fmt.Errorf("Gemini thinking level")
					}
				}
			}
		}
	}
	return nil
}
func geminiProjectDeclaration(value any, tool string) error {
	rows, ok := value.([]any)
	if !ok || len(rows) != 1 {
		return fmt.Errorf("Gemini exact tool group")
	}
	group, ok := rows[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(group, "functionDeclarations", "") != nil {
		return fmt.Errorf("Gemini tool group")
	}
	declarations, ok := group["functionDeclarations"].([]any)
	if !ok || len(declarations) != 1 {
		return fmt.Errorf("Gemini selected tool missing/foreign declaration")
	}
	declaration, ok := declarations[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(declaration, "name description parametersJsonSchema", "") != nil || declaration["name"] != tool || !projectNativeRuntimeText(declaration["description"], 16384) {
		return fmt.Errorf("Gemini selected function declaration")
	}
	schema, ok := declaration["parametersJsonSchema"].(map[string]any)
	if !ok {
		return fmt.Errorf("Gemini parameter schema")
	}
	properties, ok := schema["properties"].(map[string]any)
	if !ok || !reflect.DeepEqual(properties["wait_for_previous"], map[string]any{"type": "boolean", "description": geminiProjectWaitDescription}) {
		return fmt.Errorf("Gemini native wait_for_previous schema differs")
	}
	copySchema := map[string]any{}
	for key, v := range schema {
		copySchema[key] = v
	}
	copyProperties := map[string]any{}
	for key, v := range properties {
		if key != "wait_for_previous" {
			copyProperties[key] = v
		}
	}
	copySchema["properties"] = copyProperties
	return projectNativeRuntimeSchema(copySchema)
}

func geminiProjectRequest(body []byte, tool, id string, args map[string]any, count bool) ([]map[string]any, error) {
	value, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Gemini request object")
	}
	optional := "generationConfig systemInstruction safetySettings tools toolConfig cachedContent"
	if count {
		optional = ""
	}
	if err := projectNativeRuntimeKeys(m, "contents", optional); err != nil {
		return nil, err
	}
	parts, err := geminiProjectContents(m["contents"], tool, id, args)
	if err != nil {
		return nil, err
	}
	if count {
		return parts, nil
	}
	if err := geminiProjectDeclaration(m["tools"], tool); err != nil {
		return nil, err
	}
	if config, exists := m["generationConfig"]; exists {
		if err := geminiProjectGeneration(config); err != nil {
			return nil, err
		}
	}
	if value, exists := m["systemInstruction"]; exists {
		if _, err := geminiProjectContents([]any{value}, tool, id, args); err != nil {
			return nil, err
		}
	}
	if value, exists := m["cachedContent"]; exists && !projectNativeRuntimeText(value, 1024) {
		return nil, fmt.Errorf("Gemini cached content")
	}
	if value, exists := m["toolConfig"]; exists {
		m, ok := value.(map[string]any)
		if !ok || projectNativeRuntimeKeys(m, "functionCallingConfig", "") != nil {
			return nil, fmt.Errorf("Gemini tool config")
		}
		row, ok := m["functionCallingConfig"].(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "mode", "allowedFunctionNames") != nil || (row["mode"] != "MODE_UNSPECIFIED" && row["mode"] != "AUTO" && row["mode"] != "ANY" && row["mode"] != "NONE" && row["mode"] != "VALIDATED") {
			return nil, fmt.Errorf("Gemini function config")
		}
		if names, exists := row["allowedFunctionNames"]; exists && !reflect.DeepEqual(names, []any{tool}) {
			return nil, fmt.Errorf("Gemini allowed tool names")
		}
	}
	if value, exists := m["safetySettings"]; exists {
		rows, ok := value.([]any)
		if !ok || len(rows) > 8 {
			return nil, fmt.Errorf("Gemini safety settings")
		}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok || projectNativeRuntimeKeys(row, "category threshold", "") != nil || !projectNativeRuntimeText(row["category"], 128) || !projectNativeRuntimeText(row["threshold"], 128) {
				return nil, fmt.Errorf("Gemini safety setting")
			}
			if !geminiProjectEnum(row["category"], "HARM_CATEGORY_UNSPECIFIED HARM_CATEGORY_HARASSMENT HARM_CATEGORY_HATE_SPEECH HARM_CATEGORY_SEXUALLY_EXPLICIT HARM_CATEGORY_DANGEROUS_CONTENT HARM_CATEGORY_CIVIC_INTEGRITY") || !geminiProjectEnum(row["threshold"], "HARM_BLOCK_THRESHOLD_UNSPECIFIED BLOCK_LOW_AND_ABOVE BLOCK_MEDIUM_AND_ABOVE BLOCK_ONLY_HIGH BLOCK_NONE OFF") {
				return nil, fmt.Errorf("Gemini safety enum")
			}
		}
	}
	return parts, nil
}

func geminiProjectEnum(value any, allowed string) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	for _, name := range strings.Fields(allowed) {
		if text == name {
			return true
		}
	}
	return false
}

type geminiProjectProvider struct {
	mu                                     sync.Mutex
	server                                 *httptest.Server
	tool, id, key, nonce, marker, sentinel string
	requests                               [][]byte
	routes                                 []string
	streams                                int
	err                                    error
	returned                               bool
	validate                               func(string) error
}

func newGeminiProjectProvider(nonce, key string, validate func(string) error) *geminiProjectProvider {
	tool := "mcp_uapgeminia_inspect_runtime"
	return &geminiProjectProvider{tool: tool, id: tool + "__uap_" + nonce + "_attempt", nonce: nonce, key: key, marker: "retained-" + nonce, sentinel: "UAP_DONE_" + nonce + "_attempt", validate: validate}
}
func (p *geminiProjectProvider) start() error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p.server = &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: time.Second}}
	p.server.Start()
	return nil
}
func (p *geminiProjectProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fail := func(err error) {
		if p.err == nil {
			p.err = err
		}
		http.Error(w, "scripted Gemini provider rejected request", 400)
	}
	count := r.URL.RequestURI() == "/v1beta/models/"+geminiProjectModel+":countTokens"
	if len(p.requests) >= 8 || r.Method != "POST" || (!count && r.URL.RequestURI() != "/v1beta/models/"+geminiProjectModel+":streamGenerateContent?alt=sse") || r.Host != strings.TrimPrefix(p.server.URL, "http://") || r.Header.Get("x-goog-api-key") != p.key || r.Header.Get("Authorization") != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail(fmt.Errorf("Gemini provider route/host/key/request bound"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, projectNativeRuntimeBodyLimit+1))
	if err != nil || len(body) > projectNativeRuntimeBodyLimit {
		fail(fmt.Errorf("Gemini provider body bound"))
		return
	}
	p.requests = append(p.requests, bytes.ReplaceAll(body, []byte(p.key), []byte("[redacted]")))
	p.routes = append(p.routes, r.URL.RequestURI())
	args := map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
	parts, err := geminiProjectRequest(body, p.tool, p.id, args, count)
	if err != nil {
		fail(err)
		return
	}
	if count {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"totalTokens":1}`)
		return
	}
	p.streams++
	if p.streams == 1 {
		for _, part := range parts {
			if part["functionCall"] != nil || part["functionResponse"] != nil {
				fail(fmt.Errorf("Gemini tool history before call"))
				return
			}
		}
		p.candidate(w, map[string]any{"functionCall": map[string]any{"id": p.id, "name": p.tool, "args": args}})
		return
	}
	if p.streams != 2 {
		fail(fmt.Errorf("Gemini extra model turn"))
		return
	}
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
	if calls != 1 || responses != 1 || !strings.HasPrefix(output, "<untrusted_context>\n") || !strings.HasSuffix(output, "\n</untrusted_context>") {
		fail(fmt.Errorf("Gemini returned MCP wrapper/call count differs"))
		return
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(output, "<untrusted_context>\n"), "\n</untrusted_context>")
	if strings.Contains(inner, "<untrusted_context>") || strings.Contains(inner, "</untrusted_context>") {
		fail(fmt.Errorf("Gemini repeated untrusted wrapper"))
		return
	}
	if err := p.validate(inner); err != nil {
		fail(err)
		return
	}
	p.returned = true
	p.candidate(w, map[string]any{"text": p.sentinel})
}
func (p *geminiProjectProvider) candidate(w http.ResponseWriter, part map[string]any) {
	w.Header().Set("Content-Type", "text/event-stream")
	body, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{part}}, "finishReason": "STOP"}}, "usageMetadata": map[string]int{"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}})
	if _, err := fmt.Fprintf(w, "data: %s\n\n", body); err != nil && p.err == nil {
		p.err = fmt.Errorf("Gemini response write failed")
	}
}
