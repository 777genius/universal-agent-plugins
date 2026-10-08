// Standalone native contract fixture, not an OpenCode host or product E2E.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var version = "1.18.33"
var mode = "ok"

func main() {
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()
	input, _ := io.ReadAll(os.Stdin)
	observed := map[string]any{"cwd": cwd, "env": os.Environ(), "stdin": string(input), "args": os.Args[1:], "pid": os.Getpid()}
	if strings.HasPrefix(mode, "isolated-") {
		roots := map[string]string{}
		var failures []string
		for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR", "TMPDIR", "TMP", "TEMP", "USERPROFILE", "APPDATA", "LOCALAPPDATA"} {
			root := os.Getenv(key)
			if root == "" {
				continue
			}
			roots[key] = root
			if !filepath.IsAbs(root) {
				failures = append(failures, key+": root is not absolute")
				continue
			}
			if info, err := os.Stat(root); err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
				failures = append(failures, key+": root is not private")
			}
			if err := os.WriteFile(filepath.Join(root, "native-probe-marker"), []byte("private"), 0600); err != nil {
				failures = append(failures, key+": "+err.Error())
			}
		}
		var config []byte
		configPath := os.Getenv("OPENCODE_CONFIG")
		if !filepath.IsAbs(configPath) {
			failures = append(failures, "config: path is not absolute")
		} else {
			var err error
			config, err = os.ReadFile(configPath)
			if err != nil {
				failures = append(failures, "config: "+err.Error())
			}
		}
		observed["roots"], observed["config"], observed["failures"] = roots, string(config), failures
		mode = strings.TrimPrefix(mode, "isolated-")
	}
	body, _ := json.Marshal(observed)
	observation := filepath.Join(filepath.Dir(executable), "observed.json")
	if err := os.WriteFile(observation+".tmp", body, 0600); err != nil {
		os.Exit(18)
	}
	if err := os.Rename(observation+".tmp", observation); err != nil {
		os.Exit(19)
	}
	switch mode {
	case "failed":
		os.Exit(17)
	case "slow":
		time.Sleep(60 * time.Second)
	case "cold":
		time.Sleep(11 * time.Second)
	case "retarget":
		selected := filepath.Join(filepath.Dir(executable), "selected")
		_ = os.Remove(selected)
		_ = os.Symlink(filepath.Join(filepath.Dir(executable), "replacement"), selected)
	case "malformed":
		fmt.Println("1.18.33\n2.0.21")
		return
	case "loose":
		fmt.Println("OpenCode CLI version 1.18")
		return
	case "stderr":
		fmt.Fprintln(os.Stderr, version)
		return
	case "limit":
		fmt.Fprint(os.Stderr, strings.Repeat("x", 4096))
	case "prefix":
		fmt.Print("  opencode v" + version + "\n")
		return
	}
	fmt.Println(version)
}
