package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestPluginServiceDoctorReadyNeedsBootstrapNeedsBuildAndBlocked(t *testing.T) {
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) {
		switch name {
		case "python", "python3", "node", "npm", "pnpm":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		if len(args) == 1 && args[0] == "--version" && (filepath.Base(name) == "python" || filepath.Base(name) == "python3") {
			return "Python 3.11.0", nil
		}
		if len(args) == 1 && args[0] == "--version" && filepath.Base(name) == "node" {
			return "v22.21.1", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})

	cases := []struct {
		name       string
		setup      func(t *testing.T, root string)
		wantReady  bool
		wantStatus string
	}{
		{
			name: "ready",
			setup: func(t *testing.T, root string) {
				writeDoctorFile(t, root, "plugin/plugin.yaml", minimalBootstrapManifest())
				writeDoctorFile(t, root, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
				writeDoctorFile(t, root, "package.json", `{"type":"module"}`)
				writeDoctorFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec node \"$ROOT/plugin/main.mjs\" \"$@\"\n")
				writeDoctorFile(t, root, filepath.Join("plugin", "main.mjs"), "console.log('ok')\n")
				writeDoctorFile(t, root, filepath.Join("node_modules", ".installed"), "ok")
				mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))
			},
			wantReady:  true,
			wantStatus: "Status: ready",
		},
		{
			name: "needs-bootstrap",
			setup: func(t *testing.T, root string) {
				writeDoctorFile(t, root, "plugin/plugin.yaml", minimalBootstrapManifest())
				writeDoctorFile(t, root, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
				writeDoctorFile(t, root, "requirements.txt", "requests==2.32.0\n")
				writeDoctorFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec python \"$ROOT/plugin/main.py\" \"$@\"\n")
				writeDoctorFile(t, root, filepath.Join("plugin", "main.py"), "print('ok')\n")
				mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))
			},
			wantReady:  false,
			wantStatus: "Status: needs_bootstrap",
		},
		{
			name: "needs-build",
			setup: func(t *testing.T, root string) {
				writeDoctorFile(t, root, "plugin/plugin.yaml", minimalBootstrapManifest())
				writeDoctorFile(t, root, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
				writeDoctorFile(t, root, "package.json", `{"scripts":{"build":"tsc -p tsconfig.json"}}`)
				writeDoctorFile(t, root, "tsconfig.json", "{}\n")
				writeDoctorFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec node \"$ROOT/dist/main.js\" \"$@\"\n")
				writeDoctorFile(t, root, filepath.Join("node_modules", ".installed"), "ok")
				mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))
			},
			wantReady:  false,
			wantStatus: "Status: needs_build",
		},
		{
			name: "blocked",
			setup: func(t *testing.T, root string) {
				writeDoctorFile(t, root, "plugin/plugin.yaml", minimalBootstrapManifest())
				writeDoctorFile(t, root, "plugin/launcher.yaml", "runtime: node\nentrypoint: ./bin/demo\n")
				writeDoctorFile(t, root, "package.json", `{"scripts":{"build":"tsc -p tsconfig.json"}}`)
				writeDoctorFile(t, root, "pnpm-lock.yaml", "lockfileVersion: '9.0'\n")
				writeDoctorFile(t, root, "tsconfig.json", "{}\n")
				writeDoctorFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec node \"$ROOT/dist/main.js\" \"$@\"\n")
				mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))
				runtimecheck.LookPath = func(name string) (string, error) {
					switch name {
					case "python", "python3", "node", "npm":
						return name, nil
					default:
						return "", exec.ErrNotFound
					}
				}
			},
			wantReady:  false,
			wantStatus: "Status: blocked",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			if tc.name != "blocked" {
				runtimecheck.LookPath = func(name string) (string, error) {
					switch name {
					case "python", "python3", "node", "npm", "pnpm":
						return name, nil
					default:
						return "", exec.ErrNotFound
					}
				}
			}
			var svc PluginService
			result, err := svc.Doctor(PluginDoctorOptions{Root: root})
			if err != nil {
				t.Fatal(err)
			}
			if result.Ready != tc.wantReady {
				t.Fatalf("ready = %v want %v", result.Ready, tc.wantReady)
			}
			output := strings.Join(result.Lines, "\n")
			for _, want := range []string{"Project:", tc.wantStatus, "Environment:", "Next:"} {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q:\n%s", want, output)
				}
			}
			if tc.name == "ready" || tc.name == "blocked" || tc.name == "needs-build" {
				if !strings.Contains(output, "Runtime requirement: Node.js 20+ installed on the machine running the plugin") {
					t.Fatalf("output missing node runtime requirement:\n%s", output)
				}
				if !strings.Contains(output, "  node: ok (node") {
					t.Fatalf("output missing node env-check:\n%s", output)
				}
			}
			if tc.name == "needs-bootstrap" {
				if !strings.Contains(output, "Runtime requirement: Python 3.10+ installed on the machine running the plugin") {
					t.Fatalf("output missing python runtime requirement:\n%s", output)
				}
				if !strings.Contains(output, "  python runtime: ok (python") {
					t.Fatalf("output missing python env-check:\n%s", output)
				}
			}
			if tc.name == "blocked" {
				if !strings.Contains(output, "  pnpm: missing from PATH") {
					t.Fatalf("output missing pnpm env-check:\n%s", output)
				}
				if !strings.Contains(output, "check PATH for non-interactive shells") {
					t.Fatalf("output missing PATH hint:\n%s", output)
				}
			}
		})
	}
}

func TestPluginServiceDoctorReportsGoToolchainWhenGoModIsPresent(t *testing.T) {
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) {
		switch name {
		case "go", "gofmt":
			return "/mock/bin/" + name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		if filepath.Base(name) == "go" && len(args) == 1 && args[0] == "version" {
			return "go version go1.25.1 darwin/arm64", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})

	root := t.TempDir()
	writeDoctorFile(t, root, "plugin/plugin.yaml", `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["codex-package"]
`)
	writeDoctorFile(t, root, "go.mod", "module example.com/demo\n\ngo 1.25.0\n")

	var svc PluginService
	result, err := svc.Doctor(PluginDoctorOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	output := strings.Join(result.Lines, "\n")
	for _, want := range []string{
		"Environment:",
		"  go: ok (/mock/bin/go; go version go1.25.1 darwin/arm64)",
		"  gofmt: ok (/mock/bin/gofmt)",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestPluginServiceDoctorPoetryManagerOwnedEnvIsReady(t *testing.T) {
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) {
		switch name {
		case "poetry":
			return name, nil
		default:
			return "", exec.ErrNotFound
		}
	}
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		base := filepath.Base(name)
		if base == "poetry" && len(args) == 3 && args[0] == "env" && args[1] == "info" && args[2] == "--path" {
			return filepath.Join(dir, "external-env"), nil
		}
		if len(args) == 1 && args[0] == "--version" && strings.Contains(filepath.ToSlash(name), "external-env") {
			return "Python 3.11.0", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})

	root := t.TempDir()
	writeDoctorFile(t, root, "plugin/plugin.yaml", minimalBootstrapManifest())
	writeDoctorFile(t, root, "plugin/launcher.yaml", "runtime: python\nentrypoint: ./bin/demo\n")
	writeDoctorFile(t, root, "pyproject.toml", "[tool.poetry]\nname='demo'\n")
	writeDoctorFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexec python \"$ROOT/plugin/main.py\" \"$@\"\n")
	writeDoctorFile(t, root, filepath.Join("plugin", "main.py"), "print('ok')\n")
	writeDoctorFile(t, root, filepath.Join("external-env", "bin", "python3"), "ok")
	mustChmodBootstrapExecutable(t, filepath.Join(root, "bin", "demo"))

	var svc PluginService
	result, err := svc.Doctor(PluginDoctorOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ready {
		t.Fatalf("expected ready result:\n%s", strings.Join(result.Lines, "\n"))
	}
	output := strings.Join(result.Lines, "\n")
	if !strings.Contains(output, "Status: ready") || !strings.Contains(output, "manager=poetry") {
		t.Fatalf("unexpected doctor output:\n%s", output)
	}
}

func writeDoctorFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	mode := os.FileMode(0o644)
	if strings.HasPrefix(rel, "bin/") {
		mode = 0o755
	}
	if err := os.WriteFile(full, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}
