//go:build linux

package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var vscodeProjectIdentity = regexp.MustCompile(`^[0-9a-f]{40}$`)
var vscodeProjectQualifications = []string{"project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call"}

func vscodeProjectGatewayResult(body []byte, commit, tree string) error {
	if len(body) == 0 || len(body) > 1<<20 || !vscodeProjectIdentity.MatchString(commit) || !vscodeProjectIdentity.MatchString(tree) {
		return fmt.Errorf("bounded exact-source gateway evidence required")
	}
	value, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return err
	}
	if value["evidence_kind"] != "vscode-trusted-project-frontend" || value["mode"] != "trusted-project-v1" || value["source_commit"] != commit || value["source_tree"] != tree || value["collection_complete"] != true || value["vendor_authentication_qualified"] != false || value["real_model_qualified"] != false || value["user_public_state"] != "prepared" || value["user_native_install_qualified"] != false {
		return fmt.Errorf("trusted gateway lifecycle/source/boundary evidence differs")
	}
	qualifications, ok := value["qualification"].(map[string]any)
	if !ok || len(qualifications) != len(vscodeProjectQualifications) {
		return fmt.Errorf("trusted gateway qualifications missing")
	}
	for _, key := range vscodeProjectQualifications {
		if qualifications[key] != true {
			return fmt.Errorf("required trusted gateway stage not passed: %s", key)
		}
	}
	cleanup, ok := value["frontend_cleanup"].(map[string]any)
	if !ok || cleanup["complete"] != true {
		return fmt.Errorf("trusted frontend cleanup unproved")
	}
	controller, ok := value["controller"].(map[string]any)
	if !ok || controller["controller_reaped"] != true || controller["exit"] != json.Number("0") || controller["error"] != nil {
		return fmt.Errorf("trusted controller process failed or did not reap")
	}
	elapsed, ok := value["elapsed_seconds"].(json.Number)
	if !ok {
		return fmt.Errorf("trusted original deadline evidence missing")
	}
	seconds, err := elapsed.Float64()
	if err != nil || seconds <= 0 || seconds > 900 {
		return fmt.Errorf("trusted original deadline exceeded")
	}
	return nil
}

// The test delegates privileged host ownership to the existing reviewed strict
// controller. No client, sudo, fake HOME or fabricated hosted eligibility is
// introduced by this wrapper; ordinary full-tree supervision contains Python.
func TestAgentpluginsVSCodeNativeProjectGatewayLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_VSCODE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in genuine hosted Ubuntu 22 trusted gateway lifecycle")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" || os.Getenv("ImageOS") != "ubuntu22" || os.Getuid() <= 0 || os.Geteuid() != os.Getuid() || os.Getgid() <= 0 || os.Getegid() != os.Getgid() {
		t.Fatal("actual original nonroot GitHub-hosted Ubuntu 22 amd64 identity required")
	}
	commit, tree := os.Getenv("EXPECTED_COMMIT"), os.Getenv("EXPECTED_TREE")
	if !vscodeProjectIdentity.MatchString(commit) || !vscodeProjectIdentity.MatchString(tree) {
		t.Fatal("exact clean checkout commit/tree must be supplied by the hosted guard")
	}
	source := RepoRoot(t)
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("exact workflow Go toolchain unavailable")
	}
	goPath, err = filepath.EvalSymlinks(goPath)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	output := filepath.Join(work, "evidence")
	if selected := os.Getenv("AGENTPLUGINS_VSCODE_PROJECT_OUTPUT"); selected != "" {
		if !filepath.IsAbs(selected) || filepath.Clean(selected) != selected {
			t.Fatal("explicit canonical trusted output required")
		}
		output = selected
	}
	env := []string{"PATH=" + filepath.Dir(goPath) + ":/usr/bin:/bin", "LANG=C.UTF-8", "HOME=" + work,
		"GITHUB_ACTIONS=" + os.Getenv("GITHUB_ACTIONS"), "RUNNER_ENVIRONMENT=" + os.Getenv("RUNNER_ENVIRONMENT"),
		"AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED=" + os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED"),
		"EXPECTED_COMMIT=" + commit, "EXPECTED_TREE=" + tree, "ImageOS=" + os.Getenv("ImageOS"), "ImageVersion=" + os.Getenv("ImageVersion")}
	ctx, cancel := context.WithTimeout(context.Background(), 930*time.Second)
	defer cancel()
	r, runErr := (process.OS{}).RunWithDescendantExitGrace(ctx, ports.Command{Argv: []string{"/usr/bin/python3", filepath.Join(source, "scripts", "research-vscode-project.py"), "--output", output}, Env: env, Dir: source, StdoutLimitBytes: 1 << 20}, 5*time.Second)
	if runErr != nil || ctx.Err() != nil || r.ExitCode != 0 || len(r.Stderr) != 0 {
		t.Fatalf("trusted project frontend/process/cleanup failed; inspect the two closed CI envelopes (exit %d, error %v)", r.ExitCode, runErr)
	}
	path := filepath.Join(output, "native", "vscode-project-research.json")
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil || canonical != path {
		t.Fatal("trusted gateway result alias refused")
	}
	before, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	identity, ok := before.Sys().(*syscall.Stat_t)
	if !ok || !before.Mode().IsRegular() || identity.Nlink != 1 || identity.Uid != uint32(os.Getuid()) {
		t.Fatal("trusted gateway result must be an ordinary owned single-link file")
	}
	body, err := readProjectNativeSnapshot(path, before, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if err := vscodeProjectGatewayResult(body, commit, tree); err != nil {
		t.Fatal(err)
	}
}

func TestVSCodeProjectGatewayEvidenceRejectsIncompleteAndWrongSource(t *testing.T) {
	commit, tree := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, kind := range []string{"valid", "wrong-source", "missing-stage", "fake-auth", "bad-cleanup", "deadline", "duplicate"} {
		value := map[string]any{"evidence_kind": "vscode-trusted-project-frontend", "mode": "trusted-project-v1", "source_commit": commit, "source_tree": tree, "collection_complete": true, "vendor_authentication_qualified": false, "real_model_qualified": false, "user_public_state": "prepared", "user_native_install_qualified": false, "frontend_cleanup": map[string]any{"complete": true}, "controller": map[string]any{"controller_reaped": true, "exit": 0, "error": nil}, "elapsed_seconds": 890}
		flags := map[string]bool{}
		for _, key := range vscodeProjectQualifications {
			flags[key] = true
		}
		value["qualification"] = flags
		switch kind {
		case "wrong-source":
			value["source_tree"] = commit
		case "missing-stage":
			flags["mcp-tools-call"] = false
		case "fake-auth":
			value["vendor_authentication_qualified"] = true
		case "bad-cleanup":
			value["frontend_cleanup"] = map[string]bool{"complete": false}
		case "deadline":
			value["elapsed_seconds"] = 901
		}
		body, _ := json.Marshal(value)
		if kind == "duplicate" {
			body = append([]byte(`{"mode":"untrusted",`), body[1:]...)
		}
		err := vscodeProjectGatewayResult(body, commit, tree)
		if (err == nil) != (kind == "valid") {
			t.Fatalf("%s gateway evidence: %v", kind, err)
		}
	}
}
