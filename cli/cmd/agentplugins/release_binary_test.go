package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseBuiltBinaryHasNoConformanceEnvironmentOverride(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the exact production command")
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate release binary test")
	}
	moduleRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
	binary := filepath.Join(t.TempDir(), "agentplugins")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	goExecutable := filepath.Join(runtime.GOROOT(), "bin", "go")
	build := exec.Command(goExecutable, "build", "-trimpath", "-ldflags=-s -w -X main.version=release-binary-test", "-o", binary, "./cmd/agentplugins")
	build.Dir = moduleRoot
	build.Env = replaceEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0", "GOWORK": "off", "GOTOOLCHAIN": "local", "GOFLAGS": "-buildvcs=false -p=1",
	})
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build exact production command: %v\n%s", err, output)
	}

	body, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte(defaultDirectoryOrigin)) {
		t.Fatal("binary inspection did not find the known production Directory origin")
	}
	for _, variable := range forbiddenProductionDirectoryVariables {
		if bytes.Contains(body, []byte(variable)) {
			t.Fatalf("release-built binary contains forbidden conformance environment hook %q", variable)
		}
	}

	home := filepath.Join(t.TempDir(), "must-not-exist")
	malicious := map[string]string{"AGENTPLUGINS_HOME": home}
	for _, variable := range forbiddenProductionDirectoryVariables {
		malicious[variable] = filepath.Join(t.TempDir(), "caller-controlled")
	}
	command := exec.Command(binary, "version")
	command.Env = replaceEnvironment(os.Environ(), malicious)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("release-built binary interpreted caller conformance data: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "agentplugins release-binary-test" {
		t.Fatalf("release-built binary output = %q", output)
	}
	if _, err := os.Lstat(home); !os.IsNotExist(err) {
		t.Fatalf("release-built binary mutated AGENTPLUGINS_HOME while rejecting caller conformance data: %v", err)
	}
}

func replaceEnvironment(base []string, replacements map[string]string) []string {
	result := make([]string, 0, len(base)+len(replacements))
	for _, entry := range base {
		name, _, _ := strings.Cut(entry, "=")
		if _, replaced := replacements[name]; !replaced {
			result = append(result, entry)
		}
	}
	for name, value := range replacements {
		result = append(result, name+"="+value)
	}
	return result
}
