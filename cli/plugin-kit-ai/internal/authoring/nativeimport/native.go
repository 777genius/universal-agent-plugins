// Package nativeimport converts one explicit native source into a portable,
// standard package plan without discovery, execution, or client mutation.
package nativeimport

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/jsonmaint"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const MaxSourceBytes = 4 << 20

type Error struct{ Code string }

func (e *Error) Error() string { return "native import: " + e.Code }
func fail(code string) error   { return &Error{Code: code} }

type Issue struct {
	Code   string `json:"code"`
	ItemID string `json:"item_id"`
}

type Plan struct {
	Package             scaffold.Plan
	SourceSHA256        string
	SafeServers         int
	SkippedServers      []Issue
	UnsupportedTopLevel []Issue
	source              string
	sourceBytes         []byte
	sourceInfo          os.FileInfo
}

var bareCommand = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func Build(ctx context.Context, source, from, name, description string) (Plan, error) {
	if ctx == nil || from != "claude" || source == "" {
		return Plan{}, fail("arguments_invalid")
	}
	abs, err := filepath.Abs(source)
	if err != nil || filepath.Clean(abs) != abs || len(abs) > 4096 {
		return Plan{}, fail("source_invalid")
	}
	info, body, err := capture(ctx, abs)
	if err != nil {
		return Plan{}, err
	}
	decoded, err := jsonmaint.Decode(body)
	if err != nil {
		return Plan{}, fail(codeOf(err, "source_malformed"))
	}
	root, ok := decoded.(map[string]any)
	if !ok {
		return Plan{}, fail("source_top_level_invalid")
	}
	result := Plan{source: abs, sourceBytes: body, sourceInfo: info, SourceSHA256: hash(body)}
	keys := sortedKeys(root)
	for _, key := range keys {
		if key != "mcpServers" {
			result.UnsupportedTopLevel = append(result.UnsupportedTopLevel, issue("unsupported_top_level_field", key))
		}
	}
	rawServers, present := root["mcpServers"]
	servers, ok := rawServers.(map[string]any)
	if !present || !ok {
		return Plan{}, fail("mcp_servers_invalid")
	}
	portable := map[string]any{}
	for _, serverName := range sortedKeys(servers) {
		server, ok := servers[serverName].(map[string]any)
		converted, safe := convertServer(server)
		if !ok || !safe || !safeIdentity(serverName) {
			result.SkippedServers = append(result.SkippedServers, issue("server_skipped", serverName))
			continue
		}
		portable[serverName] = converted
	}
	result.SafeServers = len(portable)
	plugin := map[string]any{"$schema": domain.PluginSchemaV1, "description": description, "name": name, "version": "0.1.0"}
	mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": portable}
	pluginJSON, err := canonical(plugin)
	if err != nil {
		return Plan{}, fail("identity_invalid")
	}
	mcpJSON, err := canonical(mcp)
	if err != nil {
		return Plan{}, fail("source_malformed")
	}
	readme := []byte("# " + name + "\n\n" + description + "\n\nImported from an explicit Claude MCP configuration. Review and test the portable stdio servers before use.\n")
	if !safeText(name, 64) || !safeText(description, 1024) || strings.ContainsAny(name, "\r\n") {
		return Plan{}, fail("identity_invalid")
	}
	result.Package, err = scaffold.NewPlan([]scaffold.File{{Path: "plugin.json", Bytes: pluginJSON, Mode: 0644}, {Path: "mcp.json", Bytes: mcpJSON, Mode: 0644}, {Path: "README.md", Bytes: readme, Mode: 0644}})
	if err != nil {
		return Plan{}, fail("identity_invalid")
	}
	return result, nil
}

func Apply(ctx context.Context, p Plan, output string, validate scaffold.Validate) (scaffold.Result, error) {
	if p.SafeServers == 0 {
		return scaffold.Result{}, fail("no_safe_servers")
	}
	if !filepath.IsAbs(output) || filepath.Clean(output) != output || output == p.source {
		return scaffold.Result{}, fail("output_invalid")
	}
	info, body, err := capture(ctx, p.source)
	if err != nil || !os.SameFile(info, p.sourceInfo) || !bytes.Equal(body, p.sourceBytes) {
		return scaffold.Result{}, fail("source_changed")
	}
	return scaffold.Apply(ctx, p.Package, scaffold.ApplyOptions{Destination: output, SourceRoots: []string{p.source}, Validate: validate})
}

func convertServer(server map[string]any) (map[string]any, bool) {
	if server == nil {
		return nil, false
	}
	for key := range server {
		if key != "command" && key != "args" {
			return nil, false
		}
	}
	command, ok := server["command"].(string)
	if !ok || !bareCommand.MatchString(command) || secretLike(command) {
		return nil, false
	}
	result := map[string]any{"type": "stdio", "command": command}
	if raw, exists := server["args"]; exists {
		values, ok := raw.([]any)
		if !ok || len(values) > 128 {
			return nil, false
		}
		args := make([]string, len(values))
		for i, raw := range values {
			arg, ok := raw.(string)
			if !ok || !portableArg(arg) {
				return nil, false
			}
			args[i] = arg
		}
		result["args"] = args
	}
	return result, true
}

func portableArg(value string) bool {
	if len(value) > 4096 || !utf8.ValidString(value) || strings.ContainsRune(value, 0) || filepath.IsAbs(value) || filepath.VolumeName(value) != "" || strings.HasPrefix(value, `\\`) || secretLike(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\t' {
			return false
		}
	}
	u, err := url.Parse(value)
	return err != nil || u.Scheme == "" || u.User == nil && u.RawQuery == "" && u.Fragment == ""
}

func secretLike(value string) bool {
	lower := strings.ToLower(value)
	for _, marker := range []string{"secret", "token", "password", "passwd", "credential", "authorization", "bearer", "api_key", "apikey", "ghp_", "github_pat", "sk-", "akia"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func safeIdentity(value string) bool {
	return safeText(value, 128) && !secretLike(value) && !strings.ContainsAny(value, `/\\`)
}
func safeText(value string, max int) bool {
	if strings.TrimSpace(value) == "" || len(value) > max*utf8.UTFMax || utf8.RuneCountInString(value) > max || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}

func capture(ctx context.Context, path string) (os.FileInfo, []byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > MaxSourceBytes {
		return nil, nil, fail("source_unavailable")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fail("source_unavailable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) || !opened.Mode().IsRegular() {
		return nil, nil, fail("source_changed")
	}
	body, err := io.ReadAll(io.LimitReader(f, MaxSourceBytes+1))
	if err != nil || len(body) > MaxSourceBytes {
		return nil, nil, fail("source_oversize")
	}
	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fail("source_changed")
	}
	again, err := io.ReadAll(io.LimitReader(f, MaxSourceBytes+1))
	after, statErr := f.Stat()
	byName, nameErr := os.Lstat(path)
	if err != nil || statErr != nil || nameErr != nil || !bytes.Equal(body, again) || !os.SameFile(before, after) || !os.SameFile(before, byName) || after.Size() != int64(len(body)) {
		return nil, nil, fail("source_changed")
	}
	return before, body, nil
}

func issue(code, value string) Issue {
	return Issue{Code: code, ItemID: "sha256:" + hash([]byte(value))}
}
func hash(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
func canonical(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	return append(b, '\n'), err
}
func codeOf(err error, fallback string) string {
	var e *jsonmaint.Error
	if errors.As(err, &e) {
		return e.Code
	}
	return fallback
}

var _ = fmt.Sprintf
