//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
)

func cursorApprovalDirectory(home, project string) string {
	key := regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(project, "-")
	key = regexp.MustCompile(`^-+|-+$`).ReplaceAllString(key, "")
	return filepath.Join(home, ".cursor", "projects", key)
}

// The pinned JSONC/Zod parser emits stdio fields in schema order. Its record
// parser retains the fixed native config's sorted environment keys.
func cursorApprovalRecipe(home, project string, config []byte) (string, []byte, error) {
	for _, path := range []string{home, project} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == string(filepath.Separator) {
			return "", nil, fmt.Errorf("Cursor approval root differs")
		}
	}
	value, err := projectNativeRuntimeJSON(config)
	object, ok := value.(map[string]any)
	if err != nil || !ok || projectNativeRuntimeKeys(object, "mcpServers", "") != nil {
		return "", nil, fmt.Errorf("Cursor approval config differs")
	}
	servers, ok := object["mcpServers"].(map[string]any)
	server, serverOK := servers["uapcursora"].(map[string]any)
	if !ok || len(servers) != 1 || !serverOK || projectNativeRuntimeKeys(server, "command args env", "") != nil {
		return "", nil, fmt.Errorf("Cursor approval selected server differs")
	}
	var parsed struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
	}
	serverRaw, err := json.Marshal(server)
	if err != nil || json.Unmarshal(serverRaw, &parsed) != nil || parsed.Command == "" || len(parsed.Args) != 8 || len(parsed.Env) != 6 {
		return "", nil, fmt.Errorf("Cursor approval stdio shape differs")
	}
	for _, key := range []string{"PLUGIN_DATA", "PLUGIN_ROOT", "UAP_TEST_EVENTS", "UAP_TEST_NONCE", "UAP_TEST_REVISION", "UAP_TEST_ROOT"} {
		if parsed.Env[key] == "" {
			return "", nil, fmt.Errorf("Cursor approval environment differs")
		}
	}
	projection := struct {
		Path   string `json:"path"`
		Server any    `json:"server"`
	}{project, parsed}
	raw, err := json.Marshal(projection)
	if err != nil || bytes.Contains(raw, []byte("${")) || bytes.Contains(raw, []byte(`\u`)) {
		return "", nil, fmt.Errorf("Cursor approval fixed encoding differs")
	}
	for _, b := range raw {
		if b > 127 {
			return "", nil, fmt.Errorf("Cursor approval non-ASCII recipe")
		}
	}
	sum := sha256.Sum256(raw)
	body, err := json.MarshalIndent([]string{fmt.Sprintf("uapcursora-%x", sum[:8])}, "", "  ")
	return filepath.Join(cursorApprovalDirectory(home, project), "mcp-approvals.json"), body, err
}

func (f *cursorProjectAttempt) acceptApproval() error {
	config, ok := f.evidence["installed_config"].(string)
	if !ok {
		return fmt.Errorf("Cursor approval needs held installed config")
	}
	path, expected, err := cursorApprovalRecipe(f.base.Home, f.base.Project, []byte(config))
	if err != nil {
		return err
	}
	body, err := projectNativeRuntimeRead(path, 4096)
	info, statErr := os.Lstat(path)
	if err != nil || statErr != nil || !bytes.Equal(body, expected) || info.Mode() != 0644 {
		return fmt.Errorf("Cursor native approval bytes/mode differ")
	}
	identity, ok := info.Sys().(*syscall.Stat_t)
	if !ok || identity.Uid != uint32(os.Getuid()) || identity.Gid != uint32(os.Getgid()) || identity.Nlink != 1 {
		return fmt.Errorf("Cursor native approval ownership differs")
	}
	if err := f.hold(path); err != nil {
		return err
	}
	held := f.held[path]
	if held.digest != fmt.Sprintf("%x", sha256.Sum256(expected)) || !os.SameFile(info, held.info) || !projectNativeRuntimeStableStat(info.Sys(), held.info.Sys()) {
		return fmt.Errorf("Cursor approval changed before hold")
	}
	f.approvalParents = map[string]os.FileInfo{}
	for parent := filepath.Dir(path); parent != f.base.Home; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil || !info.IsDir() || info.Mode() != os.ModeDir|0755 {
			return fmt.Errorf("Cursor approval parent differs")
		}
		identity, ok := info.Sys().(*syscall.Stat_t)
		if !ok || identity.Uid != uint32(os.Getuid()) || identity.Gid != uint32(os.Getgid()) {
			return fmt.Errorf("Cursor approval parent ownership differs")
		}
		f.approvalParents[parent] = info
	}
	found := false
	for i, absent := range f.absent {
		if absent == path {
			f.absent = append(f.absent[:i], f.absent[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Cursor approval phase was not absent")
	}
	events, err := projectNativeRuntimeRead(f.base.Events, 1<<20)
	if (!os.IsNotExist(err) && err != nil) || len(events) != 0 {
		return fmt.Errorf("Cursor approval unexpectedly launched the probe")
	}
	f.evidence["native_approval"] = map[string]any{"path": path, "body": string(body), "pre_tools_probe_bytes": 0, "pre_tools_probe_sha256": fmt.Sprintf("%x", sha256.Sum256(nil))}
	return nil
}

func (f *cursorProjectAttempt) checkApprovalParents() error {
	for path, before := range f.approvalParents {
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
			return fmt.Errorf("Cursor approval parent identity changed")
		}
		a, okA := before.Sys().(*syscall.Stat_t)
		b, okB := after.Sys().(*syscall.Stat_t)
		if !okA || !okB || a.Uid != b.Uid || a.Gid != b.Gid {
			return fmt.Errorf("Cursor approval parent owner changed")
		}
	}
	return nil
}
