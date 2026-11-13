package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

type rpcClient struct {
	input      io.WriteCloser
	output     *bufio.Reader
	transcript []json.RawMessage
	total      int
	facts      map[string]any
}

func (c *rpcClient) send(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = c.input.Write(append(body, '\n'))
	return err
}
func (c *rpcClient) call(id int, method string, params any) (map[string]any, error) {
	if err := c.send(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	line, err := c.output.ReadSlice('\n')
	c.total += len(line)
	if err != nil || len(line) > 65536 || c.total > 1<<20 {
		return nil, errors.New("protocol stream bound")
	}
	value, err := shared.DecodeStrictJSONObject(line)
	_, hasResult := value["result"]
	_, hasError := value["error"]
	if err != nil || len(value) != 3 || value["jsonrpc"] != "2.0" || value["id"] != json.Number(strconv.Itoa(id)) || hasResult == hasError {
		return nil, errors.New("protocol response identity")
	}
	c.transcript = append(c.transcript, append([]byte(nil), line...))
	return value, nil
}
func exchange(c *rpcClient, t transport, operation string) error {
	initial, err := c.call(1, "initialize", map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "uap-no-model-e2e", "version": "1"}})
	if err != nil {
		return err
	}
	result, ok := initial["result"].(map[string]any)
	if !ok || result["protocolVersion"] != "2025-06-18" {
		return errors.New("initialize refused")
	}
	info, ok := result["serverInfo"].(map[string]any)
	if !ok || info["name"] != "agentplugins-native-probe" || info["version"] != "1.0.0" {
		return errors.New("server identity refused")
	}
	if err = c.send(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		return err
	}
	tools, err := c.call(2, "tools/list", map[string]any{})
	if err != nil {
		return err
	}
	listed, ok := tools["result"].(map[string]any)
	if !ok {
		return errors.New("tools result refused")
	}
	rows, ok := listed["tools"].([]any)
	if !ok || len(rows) != 1 {
		return errors.New("tools inventory refused")
	}
	tool, ok := rows[0].(map[string]any)
	if !ok || tool["name"] != "inspect_runtime" {
		return errors.New("tool identity refused")
	}
	nonce := t.Env["UAP_TEST_NONCE"]
	response, err := c.call(3, "tools/call", map[string]any{"name": "inspect_runtime", "arguments": map[string]any{"nonce": nonce, "operation": operation, "marker": "kept-" + nonce}})
	if err != nil {
		return err
	}
	c.facts, err = checkFacts(response, t, operation)
	if err != nil {
		return err
	}
	response, err = c.call(4, "tools/call", map[string]any{"name": "inspect_runtime", "arguments": map[string]any{"nonce": "wrong-" + nonce}})
	if err != nil {
		return err
	}
	negative, ok := response["error"].(map[string]any)
	if !ok || negative["code"] != json.Number("-32602") {
		return errors.New("negative nonce refused")
	}
	return nil
}
func checkFacts(response map[string]any, t transport, operation string) (map[string]any, error) {
	result, ok := response["result"].(map[string]any)
	if !ok || result["isError"] != false {
		return nil, errors.New("tool result refused")
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		return nil, errors.New("tool content refused")
	}
	row, ok := content[0].(map[string]any)
	if !ok || row["type"] != "text" {
		return nil, errors.New("tool row refused")
	}
	text, ok := row["text"].(string)
	if !ok {
		return nil, errors.New("tool facts refused")
	}
	facts, err := shared.DecodeStrictJSONObject([]byte(text))
	if err != nil {
		return nil, err
	}
	for key, expected := range map[string]string{"nonce": t.Env["UAP_TEST_NONCE"], "revision": t.Env["UAP_TEST_REVISION"], "marker": "kept-" + t.Env["UAP_TEST_NONCE"], "plugin_root": t.Env["PLUGIN_ROOT"], "plugin_data": t.Env["PLUGIN_DATA"], "cwd": t.CWD, "operation": operation} {
		if expected == "" || facts[key] != expected {
			return nil, errors.New("tool identity facts differ")
		}
	}
	return facts, nil
}

type boundedBuffer struct{ buffer bytes.Buffer }

func (b *boundedBuffer) Len() int { return b.buffer.Len() }

func (b *boundedBuffer) Write(value []byte) (int, error) {
	if b.Len()+len(value) > 32768 {
		return 0, errors.New("protocol stderr bound")
	}
	return b.buffer.Write(value)
}
func protocolEnvironment(root, action string, t transport) ([]string, error) {
	env := environment(root, strings.HasPrefix(action, "cold-"))
	for key, value := range t.Env {
		if strings.ContainsAny(key+value, "\x00\r\n") || strings.Contains(key, "=") {
			return nil, errors.New("transport environment refused")
		}
		env = append(env, key+"="+value)
	}
	return env, nil
}
func protocol(root, action string) error {
	t, err := selected(root, action)
	if err != nil {
		return err
	}
	env, err := protocolEnvironment(root, action, t)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, t.Command, t.Args...)
	cmd.Dir, cmd.Env = t.CWD, env
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr boundedBuffer
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	finished := false
	defer func() {
		if !finished {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	client := rpcClient{input: input, output: bufio.NewReaderSize(output, 65537)}
	operation := "read"
	if strings.HasSuffix(action, "-create") {
		operation = "write"
	}
	if err = exchange(&client, t, operation); err != nil {
		return err
	}
	if err = input.Close(); err != nil {
		return err
	}
	tail, err := io.ReadAll(io.LimitReader(client.output, 1))
	if err != nil || len(tail) != 0 {
		return errors.New("extra protocol output")
	}
	err, finished = cmd.Wait(), true
	if err != nil || stderr.Len() != 0 {
		return errors.New("protocol exit or stderr refused")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"caller": "direct-installed-stdio-client", "transcript": client.transcript, "facts": client.facts, "native_cline_tools_call": false})
}
