//go:build linux

package mcpruntime

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSandboxPathMapsOnlyPrivateRuntimeRoots(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "package")
	data := filepath.Join(base, "data")
	for _, dir := range []string{root, data, filepath.Join(root, "nested"), filepath.Join(data, "tmp")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		host, want string
	}{
		{root, containedPackageRoot},
		{filepath.Join(root, "nested"), containedPackageRoot + "/nested"},
		{filepath.Join(data, "tmp"), containedDataRoot + "/tmp"},
	} {
		got, err := sandboxPath(tc.host, root, data)
		if err != nil || got != tc.want {
			t.Fatalf("sandboxPath(%q)=%q, %v; want %q", tc.host, got, err, tc.want)
		}
	}
	for _, outside := range []string{base, filepath.Join(base, "package-sibling"), filepath.Join(base, "data-sibling")} {
		if got, err := sandboxPath(outside, root, data); err == nil {
			t.Fatalf("outside path mapped into sandbox: %q -> %q", outside, got)
		}
	}
}

func TestSystemRuntimeAndExecutableChecksFailClosed(t *testing.T) {
	for _, allowed := range []string{"/usr/bin/node", "/bin/node", "/lib/runtime", "/lib64/runtime"} {
		if !withinSystemRuntime(allowed) {
			t.Fatalf("system runtime path rejected: %q", allowed)
		}
	}
	for _, rejected := range []string{"/tmp/node", "/usr-local/node", "/usrmalicious/node", "/etc/node"} {
		if withinSystemRuntime(rejected) {
			t.Fatalf("non-runtime path accepted: %q", rejected)
		}
	}

	dir := t.TempDir()
	executable := filepath.Join(dir, "executable")
	if err := os.WriteFile(executable, nil, 0700); err != nil {
		t.Fatal(err)
	}
	if got, err := systemExecutable("fixture", executable); err != nil || got != executable {
		t.Fatalf("regular executable rejected: %q %v", got, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(executable, link); err == nil {
		if _, err := systemExecutable("fixture", link); err == nil {
			t.Fatal("symlink executable accepted")
		}
	}
	nonExecutable := filepath.Join(dir, "non-executable")
	if err := os.WriteFile(nonExecutable, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := systemExecutable("fixture", nonExecutable); err == nil {
		t.Fatal("non-executable file accepted")
	}
}
