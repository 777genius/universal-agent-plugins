//go:build go1.25

// An owned transport fixture; it never installs extensions or starts clients.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "manage" || os.Args[2] != "discover" {
		os.Exit(2)
	}
	mode := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if mode == "hang" {
		signal.Ignore(syscall.SIGTERM)
		if err := writePID(); err != nil {
			os.Exit(3)
		}
		time.Sleep(time.Minute)
		return
	}
	if mode == "exit" {
		fmt.Fprintln(os.Stderr, "private native error must not escape")
		os.Exit(7)
	}
	if mode == "malformed" {
		fmt.Println("private not-json output")
		return
	}
	if mode == "oversized" {
		fmt.Println(strings.Repeat("x", 70_000))
		return
	}
	profile := map[string]any{"id": "gemini", "client": "gemini", "configRoot": os.Getenv("GEMINI_CLI_HOME"), "detectionStatus": "detected"}
	target := map[string]any{"client": "gemini", "label": "Gemini CLI", "available": true, "detectionStatus": "detected", "profile": profile}
	if mode == "relative" {
		profile["configRoot"] = "unsafe-relative-path"
	}
	if mode == "absent-profile" {
		delete(target, "profile")
	}
	if mode == "status-mismatch" {
		profile["detectionStatus"] = "not_detected"
	}
	if mode == "environment" {
		for _, key := range []string{"OPENAI_API_KEY", "ANTHROPIC_AUTH_TOKEN", "NODE_OPTIONS", "HTTP_PROXY", "HTTPS_PROXY", "UAP_SDK_TEST_MODE"} {
			if os.Getenv(key) != "" {
				target["label"] = "unexpected inherited credential or launch option"
			}
		}
	}
	targets := []any{target}
	if mode == "duplicate" {
		targets = append(targets, target)
	}
	version := 1
	if mode == "version" {
		version = 2
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"apiVersion": version, "engineVersion": "fixture", "targets": targets})
	if mode == "trailing" {
		fmt.Println("private trailing output")
	}
}

func writePID() error {
	root, err := os.OpenRoot(os.Getenv("HOME"))
	if err != nil {
		return err
	}
	writeErr := root.WriteFile("pid", []byte(strconv.Itoa(os.Getpid())), 0600)
	closeErr := root.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
