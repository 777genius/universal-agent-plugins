package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli"
)

const (
	machinePolicyTestMode   = "UAP_TEST_MACHINE_POLICY_ENVIRONMENT"
	machinePolicyTestRoot   = "UAP_TEST_MACHINE_POLICY_ROOT"
	machinePolicyPathMarker = "UAP_TEST_MACHINE_POLICY_PATH="
)

func TestEnterprisePolicyMachineEnvironment(t *testing.T) {
	if mode := os.Getenv(machinePolicyTestMode); mode != "" {
		checkMachinePolicyEnvironmentChild(t, mode)
		return
	}
	want, err := machinePolicyPath()
	if err != nil {
		t.Fatalf("normal machine policy lookup: %v", err)
	}
	for _, mode := range []string{"absent", "spoofed"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			// A fresh process avoids Windows known-folder caches hiding environment dependence.
			cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestEnterprisePolicyMachineEnvironment$", "-test.count=1")
			cmd.Env = machinePolicyChildEnvironment(t, root, mode)
			cmd.Dir = filepath.Join(root, "project")
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("%s environment child failed: %v\n%s", mode, err, output)
			}
			var paths []string
			for _, line := range strings.Split(string(output), "\n") {
				if path, ok := strings.CutPrefix(strings.TrimSuffix(line, "\r"), machinePolicyPathMarker); ok {
					paths = append(paths, path)
				}
			}
			if len(paths) != 1 || paths[0] != want {
				t.Fatalf("%s environment machine paths = %q, want exactly %q\n%s", mode, paths, want, output)
			}
		})
	}
}

func machinePolicyChildEnvironment(t *testing.T, root, mode string) []string {
	t.Helper()
	environment := []string{machinePolicyTestMode + "=" + mode, machinePolicyTestRoot + "=" + root}
	for _, key := range []string{"PATH", "SystemRoot", "WINDIR", "COMSPEC", "PATHEXT"} {
		if value, present := os.LookupEnv(key); present {
			environment = append(environment, key+"="+value)
		}
	}
	for key, leaf := range map[string]string{
		"HOME": "home", "USERPROFILE": "home", "AGENTPLUGINS_HOME": "data",
		"XDG_CONFIG_HOME": "config", "XDG_DATA_HOME": "data", "XDG_CACHE_HOME": "cache",
		"XDG_STATE_HOME": "state", "TMPDIR": "tmp",
	} {
		path := filepath.Join(root, leaf)
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
		environment = append(environment, key+"="+path)
	}
	if err := os.MkdirAll(filepath.Join(root, "project"), 0o700); err != nil {
		t.Fatal(err)
	}
	if mode == "spoofed" {
		for _, key := range []string{"ProgramData", "ALLUSERSPROFILE", "SystemDrive"} {
			path := filepath.Join(root, "spoof-"+key)
			if err := os.MkdirAll(path, 0o700); err != nil {
				t.Fatal(err)
			}
			environment = append(environment, key+"="+path)
		}
	}
	return environment
}

func checkMachinePolicyEnvironmentChild(t *testing.T, mode string) {
	t.Helper()
	root := os.Getenv(machinePolicyTestRoot)
	if !filepath.IsAbs(root) || (mode != "absent" && mode != "spoofed") {
		t.Fatalf("invalid isolated child setup: root=%q mode=%q", root, mode)
	}
	for _, key := range []string{"ProgramData", "ALLUSERSPROFILE", "SystemDrive"} {
		value, present := os.LookupEnv(key)
		if mode == "absent" {
			if present {
				t.Fatalf("%s unexpectedly inherited: %q", key, value)
			}
			continue
		}
		if want := filepath.Join(root, "spoof-"+key); !present || value != want {
			t.Fatalf("%s = %q (present=%t), want isolated spoof %q", key, value, present, want)
		}
		if info, err := os.Stat(value); err != nil || !info.IsDir() {
			t.Fatalf("%s spoof must name an existing directory: %v", key, err)
		}
	}
	machine, err := machinePolicyPath()
	if err != nil || !filepath.IsAbs(machine) {
		t.Fatalf("%s environment machine policy = %q: %v", mode, machine, err)
	}
	app, err := configureEnterprisePolicy(agentpluginscli.App{}, filepath.Join(root, "home"), filepath.Join(root, "project"))
	if err != nil {
		t.Fatalf("%s environment policy composition: %v", mode, err)
	}
	if app.PolicyFiles == nil || len(app.PolicyFiles.Paths) == 0 {
		t.Fatal("composition omitted the machine policy layer")
	}
	if layer := app.PolicyFiles.Paths[0]; layer.Layer != "machine" || layer.Path != machine {
		t.Fatalf("first policy layer = %+v, want machine path %q", layer, machine)
	}
	if _, err := fmt.Fprintln(os.Stdout, machinePolicyPathMarker+machine); err != nil {
		t.Fatal(err)
	}
}
