package pluginkitairepo_test

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestOpenCodeLoaderSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE=1 to run real OpenCode loader smoke")
	}
	opencodeBin, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("opencode not in PATH")
	}

	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := filepath.Join(t.TempDir(), "opencode-basic")
	copyTree(t, filepath.Join(root, "examples", "plugins", "opencode-basic"), workDir)
	for _, rel := range []string{
		filepath.Join("targets", "opencode", "plugins", "custom-tool.js"),
		filepath.Join(".opencode", "plugins", "custom-tool.js"),
	} {
		if err := os.Remove(filepath.Join(workDir, rel)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove %s from smoke workspace: %v", rel, err)
		}
	}

	generateCmd := exec.Command(pluginKitAIBin, "generate", workDir)
	generateCmd.Dir = root
	generateCmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("generate temp opencode workspace: %v\n%s", err, out)
	}

	markerPath := filepath.Join(t.TempDir(), "opencode-plugin-marker.json")
	for attempt := 1; attempt <= 2; attempt++ {
		if err := runOpenCodeServeSmoke(workDir, opencodeBin, markerPath, "PLUGIN_KIT_AI_OPENCODE_SMOKE_MARKER"); err == nil {
			body, readErr := os.ReadFile(markerPath)
			if readErr != nil {
				t.Fatalf("read OpenCode smoke marker: %v", readErr)
			}
			if !strings.Contains(string(body), "directory") {
				t.Fatalf("unexpected OpenCode smoke marker:\n%s", body)
			}
			return
		} else if attempt < 2 {
			t.Logf("OpenCode loader smoke attempt %d timed out, retrying once", attempt)
		} else {
			t.Fatal(err)
		}
	}
}

func TestOpenCodeStandaloneToolsSmoke(t *testing.T) {
	if strings.TrimSpace(os.Getenv("PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE")) != "1" {
		t.Skip("set PLUGIN_KIT_AI_ENABLE_OPENCODE_SMOKE=1 to run real OpenCode standalone-tools smoke")
	}
	opencodeBin, err := exec.LookPath("opencode")
	if err != nil {
		t.Skip("opencode not in PATH")
	}

	root := RepoRoot(t)
	pluginKitAIBin := buildPluginKitAI(t)
	workDir := filepath.Join(t.TempDir(), "opencode-basic")
	copyTree(t, filepath.Join(root, "examples", "plugins", "opencode-basic"), workDir)
	for _, rel := range []string{
		filepath.Join("targets", "opencode", "plugins", "example.js"),
		filepath.Join("targets", "opencode", "plugins", "custom-tool.js"),
		filepath.Join(".opencode", "plugins", "example.js"),
		filepath.Join(".opencode", "plugins", "custom-tool.js"),
	} {
		if err := os.Remove(filepath.Join(workDir, rel)); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove %s from standalone-tools smoke workspace: %v", rel, err)
		}
	}

	generateCmd := exec.Command(pluginKitAIBin, "generate", workDir)
	generateCmd.Dir = root
	generateCmd.Env = append(os.Environ(), "GOWORK=off")
	if out, err := generateCmd.CombinedOutput(); err != nil {
		t.Fatalf("generate temp opencode workspace for standalone-tools smoke: %v\n%s", err, out)
	}

	markerPath := filepath.Join(t.TempDir(), "opencode-standalone-tool-marker.json")
	for attempt := 1; attempt <= 2; attempt++ {
		if err := runOpenCodeServeSmoke(workDir, opencodeBin, markerPath, "PLUGIN_KIT_AI_OPENCODE_TOOL_SMOKE_MARKER"); err == nil {
			body, readErr := os.ReadFile(markerPath)
			if readErr != nil {
				t.Fatalf("read OpenCode standalone-tools smoke marker: %v", readErr)
			}
			if !strings.Contains(string(body), "standalone-tool") || !strings.Contains(string(body), "echo.ts") {
				t.Fatalf("unexpected OpenCode standalone-tools smoke marker:\n%s", body)
			}
			return
		} else if attempt < 2 {
			t.Logf("OpenCode standalone-tools smoke attempt %d timed out, retrying once", attempt)
		} else {
			t.Fatal(err)
		}
	}
}

func runOpenCodeServeSmoke(workDir, opencodeBin, markerPath, markerEnv string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	port := pickFreePort()
	cmd := exec.CommandContext(ctx, opencodeBin, "serve")
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		markerEnv+"="+markerPath,
		"OPENCODE_SERVER_PASSWORD=plugin-kit-ai-smoke",
	)
	cmd.Args = append(cmd.Args, "--port", port)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	errCh := make(chan error, 1)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		errCh <- cmd.Wait()
	}()

	attachCmd := exec.CommandContext(ctx, opencodeBin, "attach", "http://127.0.0.1:"+port)
	attachCmd.Dir = workDir
	attachCmd.Env = append(os.Environ(),
		"OPENCODE_SERVER_PASSWORD=plugin-kit-ai-smoke",
	)

	deadline := time.Now().Add(15 * time.Second)
	attached := false
	for {
		if _, err := os.Stat(markerPath); err == nil {
			cancel()
			<-errCh
			return nil
		}
		if !attached && bytes.Contains(output.Bytes(), []byte("opencode server listening on")) {
			attached = true
			go func() {
				_, _ = attachCmd.CombinedOutput()
			}()
		}
		if time.Now().After(deadline) {
			cancel()
			err := <-errCh
			return fmt.Errorf("OpenCode serve smoke did not observe marker before timeout; err=%v\n%s", err, output.Bytes())
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func pickFreePort() string {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "0"
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return "0"
	}
	return strconv.Itoa(addr.Port)
}
