// This process is a transport/lifecycle fixture. It never installs a plugin.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "manage" || os.Args[2] != "serve" {
		os.Exit(2)
	}
	mode := os.Getenv("UAP_SDK_TEST_MODE")
	if earlyFixture(mode) {
		return
	}
	var config map[string]any
	if err := json.NewDecoder(os.Stdin).Decode(&config); err != nil {
		os.Exit(3)
	}
	if mode == "timeout" {
		_, _ = io.Copy(io.Discard, os.Stdin)
		return
	}
	if mode == "stubborn" {
		signal.Ignore(syscall.SIGTERM)
	}
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	origin, token, context := "http://"+listener.Addr().String(), strings.Repeat("a", 64), "captured-fixture-context"
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token || r.Header.Get("Origin") != origin {
			w.WriteHeader(403)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(fixtureSession(mode, context))
	})
	server := http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() { _ = server.Serve(listener) }()
	writeBootstrap(mode, origin, token, context)
	_, _ = io.Copy(io.Discard, os.Stdin)
	if mode == "stubborn" {
		select {}
	}
	_ = server.Close()
}
func writeBootstrap(mode, origin, token, context string) {
	version := 1
	if mode == "version" {
		version = 2
	}
	bootstrapURL := origin
	if mode == "remote" {
		bootstrapURL = "http://example.com:1234"
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"apiVersion": version, "url": bootstrapURL, "origin": bootstrapURL, "token": token, "engineVersion": "fixture", "contextId": context})
}

func earlyFixture(mode string) bool {
	switch mode {
	case "exit":
		fmt.Fprintln(os.Stderr, "private stderr: "+strings.Repeat("a", 64))
		os.Exit(7)
	case "malformed":
		fmt.Println("this is not JSON")
	case "oversized":
		fmt.Println(strings.Repeat("x", 70000))
	default:
		return false
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
	return true
}
func fixtureSession(mode, context string) map[string]any {
	if mode == "mismatch" {
		context = "wrong-context"
	}
	capabilities := []any{map[string]any{"action": "install", "supported": true}}
	names := map[string]bool{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		names[name] = true
	}
	for _, name := range []string{"HTTP_PROXY", "http_proxy", "HTTPS_PROXY", "https_proxy", "NO_PROXY", "no_proxy"} {
		capabilities = append(capabilities, map[string]any{"action": "environment:" + name, "supported": names[name]})
	}
	session := map[string]any{"apiVersion": 1, "engineVersion": "fixture", "environmentId": "fixture-environment", "contextId": context, "scope": "user", "targets": []any{map[string]any{"id": "environment", "client": "fixture", "label": "Fixture environment", "scope": "user", "displayPath": os.Getenv("AGENTPLUGINS_HOME"), "available": true}}, "capabilities": capabilities}
	switch mode {
	case "environment-missing":
		delete(session, "environmentId")
	case "environment-empty":
		session["environmentId"] = ""
	case "environment-number":
		session["environmentId"] = 123
	}
	return session
}
