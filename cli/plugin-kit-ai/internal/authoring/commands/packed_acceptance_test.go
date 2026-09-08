//go:build packedci

package commands_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type packedInputs struct {
	Repo      string
	Projects  []packedProject
	Snapshots []struct{ Root string }
}

func TestPackedGeneratedPackagesReachExistingInstallerPlanner(t *testing.T) {
	config := os.Getenv("UAP_PACKED_INSTALLER_CONFIG")
	if config == "" {
		for _, key := range []string{"UAP_PACKED_INSTALLER_CONFIG_SHA256", "UAP_PACKED_INSTALLER_COMMIT", "UAP_PACKED_INSTALLER_NODE", "UAP_PACKED_INSTALLER_OUTPUT"} {
			if os.Getenv(key) != "" {
				t.Fatalf("partial opt-in: %s without config", key)
			}
		}
		t.Fatal("packedci requires terminal packed-native input")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Fatal("packed bridge requires Linux amd64")
	}
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	script := filepath.Join(repo, "npm/agentplugins/scripts/packed-installer-bridge.js")
	node, digest, commit, output := os.Getenv("UAP_PACKED_INSTALLER_NODE"), os.Getenv("UAP_PACKED_INSTALLER_CONFIG_SHA256"), os.Getenv("UAP_PACKED_INSTALLER_COMMIT"), os.Getenv("UAP_PACKED_INSTALLER_OUTPUT")
	if !filepath.IsAbs(node) || !filepath.IsAbs(config) || !filepath.IsAbs(output) || filepath.Clean(output) != output || len(digest) != 64 || len(commit) != 40 {
		t.Fatal("complete absolute opt-in and identity pins required")
	}
	git := exec.Command("/usr/bin/git", "rev-parse", "HEAD")
	git.Dir = repo
	head, err := git.Output()
	if err != nil || strings.TrimSpace(string(head)) != commit {
		t.Fatalf("planner checkout is not intended commit: %v", err)
	}
	git = exec.Command("/usr/bin/git", "status", "--porcelain=v1", "--untracked-files=all")
	git.Dir = repo
	status, err := git.Output()
	if err != nil || len(status) != 0 {
		t.Fatalf("packed acceptance requires clean integrated checkout: %v\n%s", err, status)
	}
	verify := func() []byte {
		t.Helper()
		cmd := exec.Command(node, script, "verify", config, digest, commit)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		b, err := cmd.Output()
		if err != nil {
			t.Fatalf("sealed native intake: %v\n%s", err, stderr.String())
		}
		return b
	}
	before := verify()
	var inputs packedInputs
	if err := json.Unmarshal(before, &inputs); err != nil {
		t.Fatal(err)
	}
	if inputs.Repo != repo || len(inputs.Projects) != 10 {
		t.Fatal("wrong source checkout or incomplete project matrix")
	}
	wantProjects := map[string]bool{}
	for _, product := range []string{"agentplugins", "plugin-kit-ai"} {
		for _, lane := range []string{"skill", "mcp-remote", "mcp-stdio", "hybrid-remote", "hybrid-stdio"} {
			wantProjects[product+"/"+lane] = true
		}
	}
	seenSources := map[string]bool{}
	for _, p := range inputs.Projects {
		key := p.Product + "/" + p.Lane
		if !wantProjects[key] || seenSources[p.Source] {
			t.Fatal("unexpected or duplicate packed project")
		}
		delete(wantProjects, key)
		seenSources[p.Source] = true
	}
	for _, s := range inputs.Snapshots {
		if within(s.Root, output) || within(output, s.Root) {
			t.Fatal("bridge output overlaps input")
		}
	}
	if within(repo, output) || within(filepath.Dir(config), output) {
		t.Fatal("keep bridge output outside checkout and config directory")
	}
	var plans []map[string]any
	for _, p := range inputs.Projects {
		t.Run(p.Product+"/"+p.Lane, func(t *testing.T) { plans = append(plans, packedPlanner(t, p)...) })
	}
	if !bytes.Equal(before, verify()) {
		t.Fatal("native inputs changed during planning")
	}
	if t.Failed() {
		return
	}
	if len(plans) != 30 {
		t.Fatal("exactly thirty packed plans required")
	}
	tuples := map[string]bool{}
	for _, plan := range plans {
		key := fmt.Sprintf("%s/%s/%s", plan["product"], plan["lane"], plan["target"])
		if tuples[key] {
			t.Fatal("duplicate packed plan tuple")
		}
		tuples[key] = true
	}
	record := map[string]any{"kind": "packed-generated-existing-injected-installer-planner", "commit": commit, "config_sha256": digest, "inputs": json.RawMessage(before), "plans": plans, "release_eligible": false, "platform_acceptance": false, "attested": false}
	b, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	// Exclusive terminal evidence only after all 30 plans and preservation checks.
	if resolved, err := filepath.EvalSymlinks(filepath.Dir(output)); err != nil || resolved != filepath.Dir(output) {
		t.Fatal("unsafe output parent")
	}
	f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.Write(append(b, '\n'))
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("write evidence: %v %v", err, closeErr)
	}
}
