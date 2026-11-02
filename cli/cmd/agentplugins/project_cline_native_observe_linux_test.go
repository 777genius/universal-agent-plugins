//go:build linux

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
)

func clineGateJSON(body []byte) (any, error) {
	if len(body) > clineGateStream || !utf8.Valid(body) {
		return nil, fmt.Errorf("bounded UTF-8 native JSON required")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(d)
	if err != nil {
		return nil, err
	}
	if _, err = d.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("native JSON trailing data")
	}
	return value, nil
}
func clineGateObject(t *testing.T, body []byte) map[string]any {
	t.Helper()
	value, err := clineGateJSON(body)
	object, ok := value.(map[string]any)
	if err != nil || !ok {
		t.Fatalf("whole public CLI JSON object required: %v", err)
	}
	return object
}
func clineGateRows(body []byte) ([]map[string]any, error) {
	value, err := clineGateJSON(body)
	if err != nil {
		return nil, err
	}
	rows, ok := value.([]any)
	if !ok || len(rows) > 128 {
		return nil, fmt.Errorf("bounded native array required")
	}
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m, ok := row.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("native row object required")
		}
		result = append(result, m)
	}
	return result, nil
}
func clineGateText(value any) (string, bool) {
	s, ok := value.(string)
	return s, ok && len(s) <= 8192 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}
func clineGateMCP(body []byte, path string, expected []string) error {
	rows, err := clineGateRows(body)
	if err != nil {
		return err
	}
	if len(rows) != len(expected) {
		return fmt.Errorf("native MCP inventory differs from exact selected enabled file")
	}
	counts := map[string]int{}
	for _, row := range rows {
		name, a := clineGateText(row["name"])
		actual, b := clineGateText(row["path"])
		if len(row) != 4 || !a || !b || actual != path || filepath.Clean(actual) != actual || !filepath.IsAbs(actual) || row["disabled"] != false || row["transportType"] != "streamableHttp" {
			return fmt.Errorf("native MCP enabled/path/transport row differs")
		}
		counts[name]++
	}
	for _, name := range expected {
		if counts[name] != 1 {
			return fmt.Errorf("missing/duplicate selected MCP member")
		}
	}
	return nil
}

type clineGateSkill struct{ path, name, marker string }

func clineGateSkills(body []byte, expected []clineGateSkill, forbidden []string) error {
	rows, err := clineGateRows(body)
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, row := range rows {
		path, a := clineGateText(row["path"])
		name, b := clineGateText(row["name"])
		description, c := clineGateText(row["description"])
		instructions, d := clineGateText(row["instructions"])
		front, e := row["frontmatter"].(map[string]any)
		if !a || !b || !c || !d || !e || !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return fmt.Errorf("native skill path/shape differs")
		}
		for _, blocked := range forbidden {
			if path == blocked || strings.HasPrefix(path, blocked+string(filepath.Separator)) {
				return fmt.Errorf("sibling/removed skill exposed")
			}
		}
		for _, want := range expected {
			if path == want.path {
				if name != want.name || description != want.marker || !strings.Contains(instructions, want.marker) || front["disable-model-invocation"] != true {
					return fmt.Errorf("native skill marker/invocation differs")
				}
				counts[path]++
			}
		}
	}
	for _, want := range expected {
		if counts[want.path] != 1 {
			return fmt.Errorf("missing/duplicate selected skill path")
		}
	}
	return nil
}
func (f *clineGate) userMCP() string {
	return filepath.Join(f.home, ".cline", "data", "settings", "cline_mcp_settings.json")
}
func (f *clineGate) projectMCP(project string) string {
	return filepath.Join(project, ".uap", "clients", "cline", "mcp.json")
}
func (f *clineGate) skill(project, name string) string {
	if project == "" {
		project = f.home
	}
	return filepath.Join(project, ".cline", "skills", name, "SKILL.md")
}
func (f *clineGate) ownedRoots() []string {
	return []string{filepath.Join(f.home, ".cline", "skills"), f.userMCP(), filepath.Join(f.cache, "state-v2.json"),
		filepath.Join(f.A, ".cline", "skills"), f.projectMCP(f.A), filepath.Join(f.A, ".uap", "state-v2.json"),
		filepath.Join(f.B, ".cline", "skills"), f.projectMCP(f.B), filepath.Join(f.B, ".uap", "state-v2.json")}
}
func (f *clineGate) snapshot(roots ...string) map[string]string {
	f.t.Helper()
	result := map[string]string{}
	entries, total := 0, int64(0)
	for _, root := range roots {
		if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
			f.t.Fatal("snapshot aliases outside exact source")
		} else if err != nil && !os.IsNotExist(err) {
			f.t.Fatal(err)
		}
		if root != f.root && !strings.HasPrefix(root, f.root+string(filepath.Separator)) {
			f.t.Fatal("snapshot outside fixture authority")
		}
		err := filepath.WalkDir(root, func(path string, _ os.DirEntry, walkErr error) error {
			if os.IsNotExist(walkErr) && path == root {
				result[path] = "absent"
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			entries++
			if entries > 2048 {
				return fmt.Errorf("snapshot entry bound")
			}
			info, err := os.Lstat(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok || !info.Mode().IsRegular() || st.Nlink != 1 || info.Size() > 1<<20 || total+info.Size() > 8<<20 {
				return fmt.Errorf("snapshot type/byte bound")
			}
			file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
			if err != nil {
				return err
			}
			opened, err := file.Stat()
			if err != nil || !os.SameFile(info, opened) {
				_ = file.Close()
				return fmt.Errorf("snapshot opened identity differs")
			}
			body, re := io.ReadAll(io.LimitReader(file, (1<<20)+1))
			after, se := file.Stat()
			ce := file.Close()
			current, pe := os.Lstat(path)
			if re != nil || se != nil || ce != nil || pe != nil || int64(len(body)) != info.Size() || !os.SameFile(info, after) || !os.SameFile(info, current) || !after.ModTime().Equal(info.ModTime()) || !current.ModTime().Equal(info.ModTime()) {
				return fmt.Errorf("snapshot bytes/identity changed")
			}
			total += int64(len(body))
			result[path] = fmt.Sprintf("%o:%d:%s", info.Mode(), len(body), clineGateSHA(body))
			return nil
		})
		if err != nil {
			f.t.Fatal(err)
		}
	}
	return result
}
func reflectClineArgs(args []string) bool {
	return len(args) == 1 && args[0] == "--version" || len(args) == 3 && args[0] == "config" && (args[1] == "mcp" || args[1] == "skills") && args[2] == "--json"
}
func (f *clineGate) native(label, cwd, override string, args ...string) []byte {
	f.t.Helper()
	if cwd != f.user && cwd != f.A && cwd != f.B {
		f.t.Fatal("unknown native cwd")
	}
	if !reflectClineArgs(args) {
		f.t.Fatal("unreviewed Cline command")
	}
	f.admit(label, "native")
	before := f.snapshot(f.ownedRoots()...)
	if err := clineGateBinaryPin(f.binary); err != nil {
		f.t.Fatal(err)
	}
	argv := append([]string{"--auto-approve", "false", "--cwd", cwd}, args...)
	r := captureClineGate(f.ctx, f.binary, cwd, f.env(override), argv, 30*time.Second)
	f.record(label, "native", cwd, append([]string{f.binary}, argv...), r)
	f.requireSnapshot(before, f.ownedRoots()...)
	receipt := f.commands[len(f.commands)-1]
	receipt["task_home"], receipt["mcp_settings_override"], receipt["automatic_update_disabled"] = f.home, "", true
	if override != "" {
		receipt["mcp_settings_override"] = f.projectMCP(override)
	}
	receipt["owned_before"], receipt["owned_after"] = before, f.snapshot(f.ownedRoots()...)
	if r.err != nil || r.exit != 0 || r.overflow || len(r.stderr) != 0 {
		f.t.Fatalf("native stage %s failed; see exact bounded streams", label)
	}
	return r.out
}
func (f *clineGate) checkMCP(label, cwd, override, path string, names ...string) {
	body := f.native(label, cwd, override, "config", "mcp", "--json")
	if err := clineGateMCP(body, path, names); err != nil {
		f.t.Fatalf("%s: %v", label, err)
	}
}
func (f *clineGate) checkSkills(label, project, marker string, removed bool) {
	want := []clineGateSkill{{path: f.skill("", "uap-cline-user"), name: "uap-cline-user", marker: "USER"}}
	forbidden := []string{filepath.Join(f.B, ".cline", "skills")}
	name := "uap-cline-a"
	if project == f.B {
		forbidden = []string{filepath.Join(f.A, ".cline", "skills")}
		name = "uap-cline-b"
	}
	path := f.skill(project, name)
	if removed {
		forbidden = append(forbidden, path)
	} else {
		want = append(want, clineGateSkill{path: path, name: name, marker: marker})
	}
	if err := clineGateSkills(f.native(label, project, "", "config", "skills", "--json"), want, forbidden); err != nil {
		f.t.Fatalf("%s: %v", label, err)
	}
}
