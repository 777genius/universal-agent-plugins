// agentplugins_native_probe is a credential-free MCP fixture, never a product server.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type request struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type facts struct {
	Nonce    string   `json:"nonce"`
	Revision string   `json:"revision"`
	CWD      string   `json:"cwd"`
	Argv     []string `json:"argv"`
	Root     string   `json:"plugin_root"`
	Data     string   `json:"plugin_data"`
	Literal  string   `json:"literal"`
	Once     string   `json:"once"`
	Marker   string   `json:"marker,omitempty"`
	PID      int      `json:"pid"`
	Session  string   `json:"session"`
}

var session = fmt.Sprintf("%d-%d", os.Getpid(), time.Now().UnixNano())

func contained(path string) (string, error) {
	root, err := filepath.EvalSymlinks(os.Getenv("UAP_TEST_ROOT"))
	if err != nil {
		return "", err
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("fixture path must be absolute")
	}
	// Resolve existing ancestors before creating anything. Reject symlinks escaping root.
	parent := filepath.Clean(path)
	var suffix []string
	for {
		_, err = os.Lstat(parent)
		if err == nil {
			break
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		suffix = append(suffix, filepath.Base(parent))
		next := filepath.Dir(parent)
		if next == parent {
			return "", errors.New("no existing ancestor")
		}
		parent = next
	}
	parent, err = filepath.EvalSymlinks(parent)
	if err != nil {
		return "", err
	}
	for i := len(suffix) - 1; i >= 0; i-- {
		parent = filepath.Join(parent, suffix[i])
	}
	rel, err := filepath.Rel(root, parent)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("fixture path escapes test root")
	}
	return parent, nil
}
func event(method string, f *facts) error {
	path, err := contained(os.Getenv("UAP_TEST_EVENTS"))
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return err
	}
	if stat.Size() > 2<<20 {
		return errors.New("fixture event limit")
	}
	return json.NewEncoder(file).Encode(map[string]any{"method": method, "pid": os.Getpid(), "session": session, "nonce": os.Getenv("UAP_TEST_NONCE"), "timestamp": time.Now().UTC().Format(time.RFC3339Nano), "facts": f})
}
func inspect(params json.RawMessage) (any, error) {
	var p struct {
		Name      string `json:"name"`
		Arguments struct {
			Nonce     string `json:"nonce"`
			Operation string `json:"operation"`
			Marker    string `json:"marker"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, err
	}
	if p.Name != "inspect_runtime" || p.Arguments.Nonce == "" || p.Arguments.Nonce != os.Getenv("UAP_TEST_NONCE") {
		return nil, errors.New("unknown tool or nonce mismatch")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	f := facts{Nonce: p.Arguments.Nonce, Revision: os.Getenv("UAP_TEST_REVISION"), CWD: cwd, Argv: os.Args[1:], Root: os.Getenv("PLUGIN_ROOT"), Data: os.Getenv("PLUGIN_DATA"), Literal: os.Getenv("UAP_TEST_LITERAL"), Once: os.Getenv("UAP_TEST_ONCE"), PID: os.Getpid(), Session: session}
	if p.Arguments.Operation != "" {
		if p.Arguments.Operation != "write" && p.Arguments.Operation != "read" {
			return nil, errors.New("unknown marker operation")
		}
		path, err := contained(filepath.Join(f.Data, "native-marker.txt"))
		if err != nil {
			return nil, err
		}
		if p.Arguments.Operation == "write" {
			if len(p.Arguments.Marker) > 256 {
				return nil, errors.New("marker too long")
			}
			if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return nil, err
			}
			if err = os.WriteFile(path, []byte(p.Arguments.Marker), 0600); err != nil {
				return nil, err
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if len(b) > 256 {
			return nil, errors.New("marker too long")
		}
		f.Marker = string(b)
	}
	if err := event("tools/call", &f); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(f)
	return map[string]any{"content": []any{map[string]any{"type": "text", "text": string(b)}}, "isError": false}, nil
}
func handle(r request) (any, error) {
	switch r.Method {
	case "initialize":
		var p struct {
			Version string `json:"protocolVersion"`
		}
		if err := json.Unmarshal(r.Params, &p); err != nil {
			return nil, err
		}
		switch p.Version {
		case "2024-11-05", "2025-03-26", "2025-06-18", "2025-11-25":
		default:
			p.Version = "2025-11-25"
		}
		if err := event(r.Method, nil); err != nil {
			return nil, err
		}
		return map[string]any{"protocolVersion": p.Version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "agentplugins-native-probe", "version": "1.0.0"}}, nil
	case "tools/list":
		if err := event(r.Method, nil); err != nil {
			return nil, err
		}
		return map[string]any{"tools": []any{map[string]any{"name": "inspect_runtime", "description": "Inspect only disposable fixture runtime facts.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]string{"type": "string"}, "operation": map[string]string{"type": "string"}, "marker": map[string]string{"type": "string"}}, "required": []string{"nonce"}, "additionalProperties": false}}}}, nil
	case "tools/call":
		return inspect(r.Params)
	case "ping":
		return map[string]any{}, nil
	default:
		return nil, errors.New("method not found")
	}
}
func main() {
	if len(os.Args) == 2 && os.Args[1] == "--fail-startup" {
		if err := event("startup-failed", nil); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(43)
		}
		fmt.Fprintln(os.Stderr, "intentional fixture startup failure")
		os.Exit(42)
	}

	scan := bufio.NewScanner(io.LimitReader(os.Stdin, 8<<20))
	scan.Buffer(make([]byte, 4096), 1<<20)
	out := json.NewEncoder(os.Stdout)
	for scan.Scan() {
		var r request
		if err := json.Unmarshal(scan.Bytes(), &r); err != nil {
			_ = out.Encode(map[string]any{"jsonrpc": "2.0", "id": nil, "error": map[string]any{"code": -32700, "message": "invalid JSON"}})
			continue
		}
		if len(r.ID) == 0 {
			continue
		}
		result, err := handle(r)
		response := map[string]any{"jsonrpc": "2.0", "id": r.ID}
		if err != nil {
			response["error"] = map[string]any{"code": -32602, "message": err.Error()}
		} else {
			response["result"] = result
		}
		if out.Encode(response) != nil {
			return
		}
	}
	if scan.Err() != nil {
		fmt.Fprintln(os.Stderr, "fixture input limit or read error")
	}
}
