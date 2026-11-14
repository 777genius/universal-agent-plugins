package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

type envelope struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

type toolsCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

func main() {
	in := bufio.NewReader(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for {
		body, err := readMCPMessage(in)
		if err != nil {
			if err == io.EOF {
				return
			}
			writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				Error: map[string]any{
					"code":    -32700,
					"message": err.Error(),
				},
			})
			return
		}
		var msg envelope
		if err := json.Unmarshal(body, &msg); err != nil {
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				Error: map[string]any{
					"code":    -32700,
					"message": err.Error(),
				},
			})
			continue
		}
		switch msg.Method {
		case "initialize":
			_ = writeMarker("initialize", "", "")
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
					"serverInfo": map[string]any{
						"name":    "cursor-cli-mcp-smoke",
						"version": "0.1.0",
					},
				},
			})
		case "notifications/initialized", "initialized":
			continue
		case "ping":
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result:  map[string]any{},
			})
		case "tools/list":
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]any{
					"tools": []map[string]any{{
						"name":        "release_checks",
						"description": "Records a deterministic Cursor CLI MCP smoke marker.",
						"inputSchema": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"token": map[string]any{
									"type": "string",
								},
							},
							"required":             []string{"token"},
							"additionalProperties": false,
						},
					}},
				},
			})
		case "tools/call":
			var params toolsCallParams
			if err := json.Unmarshal(msg.Params, &params); err != nil {
				_ = writeMCPMessage(out, envelope{
					JSONRPC: "2.0",
					ID:      msg.ID,
					Error: map[string]any{
						"code":    -32602,
						"message": err.Error(),
					},
				})
				continue
			}
			token := strings.TrimSpace(fmt.Sprint(params.Arguments["token"]))
			if token == "" {
				token = "MISSING"
			}
			_ = writeMarker("tools/call", "release_checks", token)
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Result: map[string]any{
					"content": []map[string]any{{
						"type": "text",
						"text": "CURSOR_MCP_TOOL_OK " + token,
					}},
					"isError": false,
				},
			})
		default:
			if msg.ID == nil {
				continue
			}
			_ = writeMCPMessage(out, envelope{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: map[string]any{
					"code":    -32601,
					"message": "method not found",
				},
			})
		}
	}
}

func readMCPMessage(r *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			if _, err := fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &contentLength); err != nil {
				return nil, err
			}
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, err
	}
	return body, nil
}

func writeMCPMessage(w io.Writer, value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	if _, err := w.Write(body); err != nil {
		return err
	}
	if flusher, ok := w.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}

func writeMarker(event, tool, token string) error {
	path := strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_MCP_SMOKE_MARKER"))
	if path == "" {
		path = strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_CURSOR_MCP_MARKER"))
	}
	if path == "" {
		return nil
	}
	body, err := json.MarshalIndent(map[string]any{
		"event":     event,
		"tool":      tool,
		"token":     token,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
	}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}
