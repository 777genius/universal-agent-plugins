package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// PrepareOpenCodeVersionProbeEnvironment populates an empty, private directory
// owned by the caller. It never reads or inherits the user's environment.
func PrepareOpenCodeVersionProbeEnvironment(root string) ([]string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return nil, fmt.Errorf("OpenCode version probe requires a clean absolute private directory")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("OpenCode version probe requires an ordinary private directory")
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil || canonical != root {
		return nil, fmt.Errorf("OpenCode version probe directory contains an alias")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		return nil, fmt.Errorf("OpenCode version probe directory must be empty")
	}
	var env []string
	for _, item := range []struct{ key, directory string }{
		{"HOME", "home"}, {"XDG_CONFIG_HOME", "config"}, {"XDG_CACHE_HOME", "cache"},
		{"XDG_DATA_HOME", "data"}, {"XDG_STATE_HOME", "state"},
		{"XDG_RUNTIME_DIR", "runtime"}, {"TMPDIR", "tmp"},
	} {
		path := filepath.Join(root, item.directory)
		if err := os.Mkdir(path, 0700); err != nil {
			return nil, err
		}
		env = append(env, item.key+"="+path)
	}
	return finishOpenCodeVersionEnvironment(root, env)
}

func finishOpenCodeVersionEnvironment(root string, env []string) ([]string, error) {
	tmp := filepath.Join(root, "tmp")
	env = append(env, "TMP="+tmp, "TEMP="+tmp)
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+filepath.Join(root, "home"),
			"APPDATA="+filepath.Join(root, "config"), "LOCALAPPDATA="+filepath.Join(root, "data"))
	}
	config := filepath.Join(root, "opencode.json")
	file, err := os.OpenFile(config, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	_, writeErr := file.WriteString("{}\n")
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return append(env, "OPENCODE_CONFIG="+config,
		"OPENCODE_CONFIG_CONTENT={}", "OPENCODE_CLI_CONFIG_CONTENT={}",
		"OPENCODE_DISABLE_PROJECT_CONFIG=1", "OPENCODE_DISABLE_MODELS_FETCH=1",
		"OPENCODE_DISABLE_AUTOUPDATE=1"), nil
}
