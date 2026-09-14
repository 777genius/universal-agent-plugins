// Package mcpruntime runs one explicitly selected standard MCP server in an
// operation-owned copy of the package. It has no legacy manifest dependency.
package mcpruntime

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const (
	maxFrame        = 1 << 20
	maxFixture      = 1 << 20
	protocolVersion = "2025-06-18"
)

type Options struct {
	SourceRoot, Scratch string
	Project             project.Result
	Server, Tool        string
	Fixture             string
	AllowNetwork        bool
	Deadline            time.Duration
	Projects            project.Service
}

// Evidence intentionally contains no argv, URL, headers, environment, fixture,
// response body, temporary path, or server/tool name.
type Evidence struct {
	Transport  string
	Initialize bool
	ListTools  bool
	ToolCall   bool
	ToolCount  int
	Cleanup    bool
}

type Error struct{ Code string }

func (e *Error) Error() string { return "MCP runtime: " + e.Code }
func fail(code string) error   { return &Error{Code: code} }

func Run(ctx context.Context, o Options) (ev Evidence, err error) {
	if o.Project.Facts.Package == nil || o.Project.Facts.Package.FormatID != domain.FormatIDAgentPluginsV1 {
		return ev, fail("runtime_package_invalid")
	}
	server, ok := o.Project.Facts.Package.MCP.Servers[o.Server]
	if !ok {
		return ev, fail("runtime_server_unknown")
	}
	if (o.Tool == "") != (o.Fixture == "") {
		return ev, fail("runtime_tool_fixture_pair_required")
	}
	if o.Deadline <= 0 || o.Deadline > time.Minute {
		return ev, fail("runtime_deadline_invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, o.Deadline)
	defer cancel()
	root, data, cleanup, err := privateCopy(ctx, o)
	if err != nil {
		return ev, err
	}
	defer func() {
		if cleanup() != nil {
			ev.Cleanup = false
			err = errors.Join(fail("runtime_cleanup_failed"), err)
		} else {
			ev.Cleanup = true
		}
	}()
	var arguments map[string]any
	if o.Fixture != "" {
		arguments, err = fixture(root, o.Fixture)
		if err != nil {
			return ev, err
		}
	}
	switch server.Type {
	case "stdio":
		ev.Transport = "stdio"
		err = runStdio(ctx, root, data, server, o.Tool, arguments, &ev)
	case "streamable-http":
		ev.Transport = "streamable_http"
		if !o.AllowNetwork {
			return ev, fail("runtime_network_opt_in_required")
		}
		err = runHTTP(ctx, server, o.Tool, arguments, &ev)
	default:
		err = fail("runtime_transport_unsupported")
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ev, fail("runtime_deadline_exceeded")
	}
	return ev, err
}

func privateCopy(ctx context.Context, o Options) (string, string, func() error, error) {
	base, err := os.MkdirTemp(o.Scratch, "author-mcp-")
	if err != nil {
		return "", "", nil, fail("runtime_sandbox_unavailable")
	}
	owned := true
	cleanup := func() error {
		if !owned {
			return nil
		}
		owned = false
		return os.RemoveAll(base)
	}
	bad := func(e error) (string, string, func() error, error) {
		ce := cleanup()
		return "", "", func() error { return ce }, e
	}
	if err := os.Chmod(base, 0700); err != nil {
		return bad(fail("runtime_sandbox_unavailable"))
	}
	runtimeRoot, data := filepath.Join(base, "package"), filepath.Join(base, "data")
	if err := os.Mkdir(runtimeRoot, 0700); err != nil {
		return bad(fail("runtime_sandbox_unavailable"))
	}
	for _, d := range []string{data, filepath.Join(data, "home"), filepath.Join(data, "tmp"), filepath.Join(data, "config"), filepath.Join(data, "cache")} {
		if err := os.Mkdir(d, 0700); err != nil {
			return bad(fail("runtime_sandbox_unavailable"))
		}
	}
	var observations []struct {
		path, kind     string
		target         string
		exec, captured bool
		size           int64
	}
	for _, v := range o.Project.Input.Inventory {
		observations = append(observations, struct {
			path, kind     string
			target         string
			exec, captured bool
			size           int64
		}{v.Path, v.Kind, v.Target, v.Executable, v.Captured, v.Size})
	}
	sort.Slice(observations, func(i, j int) bool { return observations[i].path < observations[j].path })
	source, err := os.OpenRoot(o.SourceRoot)
	if err != nil {
		return bad(fail("runtime_source_changed"))
	}
	defer source.Close()
	for _, v := range observations {
		if err := ctx.Err(); err != nil {
			return bad(err)
		}
		if v.path == ".git" || v.path == ".plugin-kit-ai.lock" && v.kind != "directory" {
			continue
		}
		if !safePackagePath(v.path) {
			return bad(fail("runtime_package_not_copyable"))
		}
		if !v.captured {
			return bad(fail("runtime_package_not_copyable"))
		}
		to := filepath.Join(runtimeRoot, filepath.FromSlash(v.path))
		if v.kind == "directory" {
			if err := os.MkdirAll(to, 0700); err != nil {
				return bad(fail("runtime_sandbox_unavailable"))
			}
			continue
		}
		if v.kind == "symlink" {
			if !safeLink(v.path, v.target) || os.Symlink(filepath.FromSlash(v.target), to) != nil {
				return bad(fail("runtime_package_not_copyable"))
			}
			continue
		}
		if v.kind != "file" {
			return bad(fail("runtime_package_not_copyable"))
		}
		from, err := source.Open(filepath.FromSlash(v.path))
		if err != nil {
			return bad(fail("runtime_source_changed"))
		}
		info, statErr := from.Stat()
		body, readErr := io.ReadAll(io.LimitReader(from, v.size+1))
		closeErr := from.Close()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() != v.size || int64(len(body)) != v.size || readErr != nil || closeErr != nil {
			return bad(fail("runtime_source_changed"))
		}
		mode := os.FileMode(0600)
		if v.exec {
			mode = 0700
		}
		if err := os.WriteFile(to, body, mode); err != nil {
			return bad(fail("runtime_sandbox_unavailable"))
		}
	}
	copyProject, err := o.Projects.Read(ctx, runtimeRoot)
	if err != nil || copyProject.Input.Identity.TreeDigest == "" || copyProject.Input.Identity.TreeDigest != o.Project.Input.Identity.TreeDigest {
		return bad(fail("runtime_source_changed"))
	}
	return runtimeRoot, data, cleanup, nil
}

func safePackagePath(name string) bool {
	return name != "" && !strings.Contains(name, `\`) && !path.IsAbs(name) && path.Clean(name) == name && name != "." && !strings.HasPrefix(name, "../")
}

func safeLink(name, target string) bool {
	if target == "" || strings.Contains(target, `\`) || path.IsAbs(target) {
		return false
	}
	resolved := path.Clean(path.Join(path.Dir(name), target))
	return safePackagePath(resolved)
}

func fixture(root, name string) (map[string]any, error) {
	if name == "" || strings.Contains(name, `\`) || path.IsAbs(name) || path.Clean(name) != name || name == "." || strings.HasPrefix(name, "../") {
		return nil, fail("runtime_fixture_outside_package")
	}
	name = filepath.FromSlash(name)
	f, err := os.OpenRoot(root)
	if err != nil {
		return nil, fail("runtime_fixture_unavailable")
	}
	defer f.Close()
	r, err := f.Open(name)
	if err != nil {
		return nil, fail("runtime_fixture_unavailable")
	}
	defer r.Close()
	limited := io.LimitReader(r, maxFixture+1)
	body, err := io.ReadAll(limited)
	if err != nil || len(body) > maxFixture {
		return nil, fail("runtime_fixture_invalid")
	}
	var value map[string]any
	if conformance.RejectDuplicateJSONKeys(body) != nil {
		return nil, fail("runtime_fixture_invalid")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	if err := d.Decode(&value); err != nil || value == nil || d.Decode(&struct{}{}) != io.EOF {
		return nil, fail("runtime_fixture_invalid")
	}
	return value, nil
}

func runStdio(ctx context.Context, root, data string, server domain.MCPServer, tool string, arguments map[string]any, ev *Evidence) error {
	command, _ := server.Decoded["command"].(string)
	if command != "node" {
		return fail("runtime_stdio_requires_node")
	}
	node, err := exec.LookPath("node")
	if err != nil {
		return fail("runtime_node_unavailable")
	}
	args, err := stringSlice(server.Decoded["args"])
	if err != nil || len(args) == 0 || !(strings.HasPrefix(args[0], "./") || strings.HasPrefix(args[0], "${PLUGIN_ROOT}/")) {
		return fail("runtime_stdio_config_invalid")
	}
	for i := range args {
		args[i] = expand(args[i], root, data)
	}
	script, err := contained(args[0], root)
	if err != nil {
		return fail("runtime_command_outside_package")
	}
	info, err := os.Lstat(script)
	if err != nil || !info.Mode().IsRegular() {
		return fail("runtime_command_unavailable")
	}
	args[0] = script
	cwd := root
	if authored, _ := server.Decoded["cwd"].(string); authored != "" {
		cwd, err = contained(expand(authored, root, data), root, data)
		if err != nil {
			return fail("runtime_cwd_outside_sandbox")
		}
		info, err := os.Lstat(cwd)
		if err != nil || !info.IsDir() {
			return fail("runtime_cwd_unavailable")
		}
	}
	if err := (processadapter.OS{}).DuplexCapability(); err != nil {
		return fail("runtime_process_containment_unavailable")
	}
	env, err := restrictedEnv(root, data, node, server.Decoded["env"])
	if err != nil {
		return fail("runtime_stdio_config_invalid")
	}
	commandLine := append([]string{node}, args...)
	err = (processadapter.OS{}).RunDuplexWithPlannedShutdown(ctx, ports.Command{Argv: commandLine, Env: env, Dir: cwd}, func(stdin io.Writer, stdout io.Reader) error {
		client := &stdioClient{in: bufio.NewReaderSize(stdout, 64<<10), out: stdin}
		initialized, err := client.call(1, "initialize", map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agentplugins-author", "version": "1"}})
		if err != nil || !validInitialize(initialized) {
			if err == nil {
				err = fail("runtime_protocol_invalid")
			}
			return err
		}
		ev.Initialize = true
		if err := client.notify("notifications/initialized", map[string]any{}); err != nil {
			return err
		}
		result, err := client.call(2, "tools/list", map[string]any{})
		if err != nil {
			return err
		}
		ev.ListTools = true
		var listed struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		}
		if json.Unmarshal(result, &listed) != nil {
			return fail("runtime_protocol_invalid")
		}
		ev.ToolCount = len(listed.Tools)
		if tool != "" {
			if !hasTool(listed.Tools, tool) {
				return fail("runtime_tool_unknown")
			}
			result, err = client.call(3, "tools/call", map[string]any{"name": tool, "arguments": arguments})
			if err != nil {
				return err
			}
			var called struct {
				Content []json.RawMessage `json:"content"`
				IsError bool              `json:"isError"`
			}
			if json.Unmarshal(result, &called) != nil || called.IsError || called.Content == nil {
				return fail("runtime_tool_failed")
			}
			ev.ToolCall = true
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

type stdioClient struct {
	in  *bufio.Reader
	out io.Writer
}
type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   json.RawMessage `json:"error"`
}

func (c *stdioClient) notify(method string, params any) error {
	return json.NewEncoder(c.out).Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}
func (c *stdioClient) call(id int, method string, params any) (json.RawMessage, error) {
	if err := json.NewEncoder(c.out).Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
		return nil, fail("runtime_protocol_io_failed")
	}
	for {
		line, err := readLine(c.in)
		if err != nil {
			return nil, fail("runtime_protocol_io_failed")
		}
		var r response
		if json.Unmarshal(line, &r) != nil {
			return nil, fail("runtime_protocol_invalid")
		}
		var got int
		if len(r.ID) == 0 || json.Unmarshal(r.ID, &got) != nil || got != id {
			continue
		}
		if r.JSONRPC != "2.0" {
			return nil, fail("runtime_protocol_invalid")
		}
		if len(r.Error) != 0 && string(r.Error) != "null" {
			return nil, fail("runtime_protocol_error")
		}
		if len(r.Result) == 0 {
			return nil, fail("runtime_protocol_invalid")
		}
		return r.Result, nil
	}
}

func readLine(reader *bufio.Reader) ([]byte, error) {
	var line []byte
	for {
		part, err := reader.ReadSlice('\n')
		if len(line)+len(part) > maxFrame {
			return nil, fail("runtime_protocol_io_failed")
		}
		line = append(line, part...)
		if err == nil {
			return line, nil
		}
		if !errors.Is(err, bufio.ErrBufferFull) {
			return nil, err
		}
	}
}

func runHTTP(ctx context.Context, server domain.MCPServer, tool string, arguments map[string]any, ev *Evidence) error {
	raw, _ := server.Decoded["url"].(string)
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Hostname() == "" {
		return fail("runtime_http_config_invalid")
	}
	headers, err := stringMap(server.Decoded["headers"])
	if err != nil {
		return fail("runtime_http_config_invalid")
	}
	for name := range headers {
		switch http.CanonicalHeaderKey(name) {
		case "Accept", "Content-Type", "Mcp-Session-Id":
			return fail("runtime_http_config_invalid")
		}
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	session := ""
	call := func(id int, method string, params any, notification bool) (json.RawMessage, error) {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if notification {
			body, _ = json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
		if err != nil {
			return nil, fail("runtime_http_request_failed")
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if session != "" {
			req.Header.Set("Mcp-Session-Id", session)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fail("runtime_http_request_failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fail("runtime_http_status_failed")
		}
		if value := resp.Header.Get("Mcp-Session-Id"); value != "" {
			session = value
		}
		if notification && resp.StatusCode == http.StatusAccepted {
			return nil, nil
		}
		payload, err := readHTTPPayload(resp, id)
		if err != nil {
			return nil, err
		}
		var r response
		var responseID int
		if json.Unmarshal(payload, &r) != nil || r.JSONRPC != "2.0" || json.Unmarshal(r.ID, &responseID) != nil || responseID != id || len(r.Error) != 0 && string(r.Error) != "null" || len(r.Result) == 0 {
			return nil, fail("runtime_protocol_invalid")
		}
		return r.Result, nil
	}
	initialized, err := call(1, "initialize", map[string]any{"protocolVersion": protocolVersion, "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agentplugins-author", "version": "1"}}, false)
	if err != nil || !validInitialize(initialized) {
		if err == nil {
			err = fail("runtime_protocol_invalid")
		}
		return err
	}
	ev.Initialize = true
	if _, err := call(0, "notifications/initialized", map[string]any{}, true); err != nil {
		return err
	}
	result, err := call(2, "tools/list", map[string]any{}, false)
	if err != nil {
		return err
	}
	ev.ListTools = true
	var listed struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if json.Unmarshal(result, &listed) != nil {
		return fail("runtime_protocol_invalid")
	}
	ev.ToolCount = len(listed.Tools)
	if tool != "" {
		if !hasTool(listed.Tools, tool) {
			return fail("runtime_tool_unknown")
		}
		result, err = call(3, "tools/call", map[string]any{"name": tool, "arguments": arguments}, false)
		if err != nil {
			return err
		}
		var called struct {
			Content []json.RawMessage `json:"content"`
			IsError bool              `json:"isError"`
		}
		if json.Unmarshal(result, &called) != nil || called.IsError || called.Content == nil {
			return fail("runtime_tool_failed")
		}
		ev.ToolCall = true
	}
	return nil
}

func validInitialize(result json.RawMessage) bool {
	var initialized struct {
		ProtocolVersion string                     `json:"protocolVersion"`
		Capabilities    map[string]json.RawMessage `json:"capabilities"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	return json.Unmarshal(result, &initialized) == nil && initialized.ProtocolVersion == protocolVersion && initialized.Capabilities != nil && initialized.ServerInfo.Name != "" && initialized.ServerInfo.Version != ""
}

func hasTool(tools []struct {
	Name string `json:"name"`
}, name string) bool {
	for _, tool := range tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func readHTTPPayload(resp *http.Response, id int) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFrame+1))
	if err != nil || len(body) > maxFrame {
		return nil, fail("runtime_protocol_io_failed")
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if strings.Contains(contentType, "text/event-stream") {
		var data []string
		flush := func() []byte {
			if len(data) == 0 {
				return nil
			}
			payload := []byte(strings.Join(data, "\n"))
			data = nil
			var candidate response
			var responseID int
			if json.Unmarshal(payload, &candidate) == nil && candidate.JSONRPC == "2.0" && json.Unmarshal(candidate.ID, &responseID) == nil && responseID == id {
				return payload
			}
			return nil
		}
		for _, raw := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
			line := strings.TrimSuffix(raw, "\r")
			if line == "" {
				if payload := flush(); payload != nil {
					return payload, nil
				}
				continue
			}
			if strings.HasPrefix(line, "data:") {
				value := strings.TrimPrefix(line, "data:")
				if strings.HasPrefix(value, " ") {
					value = value[1:]
				}
				data = append(data, value)
			}
		}
		if payload := flush(); payload != nil {
			return payload, nil
		}
		return nil, fail("runtime_protocol_invalid")
	}
	if !strings.Contains(contentType, "application/json") {
		return nil, fail("runtime_protocol_invalid")
	}
	return body, nil
}

func stringSlice(v any) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	values, ok := v.([]any)
	if !ok {
		return nil, errors.New("invalid")
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i], ok = value.(string)
		if !ok {
			return nil, errors.New("invalid")
		}
	}
	return out, nil
}
func stringMap(v any) (map[string]string, error) {
	out := map[string]string{}
	if v == nil {
		return out, nil
	}
	values, ok := v.(map[string]any)
	if !ok {
		return nil, errors.New("invalid")
	}
	for k, value := range values {
		s, ok := value.(string)
		if !ok {
			return nil, errors.New("invalid")
		}
		out[k] = s
	}
	return out, nil
}
func expand(value, root, data string) string {
	return strings.NewReplacer("${PLUGIN_ROOT}", root, "${PLUGIN_DATA}", data).Replace(value)
}
func contained(value string, bases ...string) (string, error) {
	root := bases[0]
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, filepath.FromSlash(value))
	}
	clean := filepath.Clean(value)
	for _, base := range bases {
		if rel, err := filepath.Rel(base, clean); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return clean, nil
		}
	}
	return "", errors.New("outside")
}
func restrictedEnv(root, data, runtimeExecutable string, authored any) ([]string, error) {
	env := []string{"HOME=" + filepath.Join(data, "home"), "TMPDIR=" + filepath.Join(data, "tmp"), "XDG_CONFIG_HOME=" + filepath.Join(data, "config"), "XDG_CACHE_HOME=" + filepath.Join(data, "cache"), "PLUGIN_ROOT=" + root, "PLUGIN_DATA=" + data, "PATH=" + filepath.Dir(runtimeExecutable)}
	values, err := stringMap(authored)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		upper := strings.ToUpper(k)
		if k == "" || strings.ContainsRune(k, '=') || upper == "HOME" || upper == "PATH" || upper == "TMP" || upper == "TEMP" || upper == "TMPDIR" || upper == "PLUGIN_ROOT" || upper == "PLUGIN_DATA" || strings.HasPrefix(upper, "XDG_") {
			return nil, errors.New("reserved runtime environment name")
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+expand(values[k], root, data))
	}
	return env, nil
}
