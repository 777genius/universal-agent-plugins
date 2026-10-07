// Standalone native contract fixture, not an OpenCode host or product E2E.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var version = "1.18.33"
var mode = "ok"

func main() {
	executable, _ := os.Executable()
	cwd, _ := os.Getwd()
	input, _ := io.ReadAll(os.Stdin)
	body, _ := json.Marshal(map[string]any{"cwd": cwd, "env": os.Environ(), "stdin": string(input), "args": os.Args[1:]})
	_ = os.WriteFile(filepath.Join(filepath.Dir(executable), "observed.json"), body, 0600)
	switch mode {
	case "failed":
		os.Exit(17)
	case "slow":
		time.Sleep(30 * time.Second)
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
