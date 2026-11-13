//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
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
	if value["evidence_kind"] != "vscode-trusted-project-frontend" || value["mode"] != "trusted-project-v1" || value["source_commit"] != commit || value["source_tree"] != tree || value["collection_complete"] != true || value["vendor_authentication_qualified"] != false || value["real_model_qualified"] != false || value["user_public_state"] != "not-installed" || value["user_native_install_qualified"] != false {
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

const vscodeArtifactPrefix = "UAP_VSCODE_ARTIFACT_HANDOFF "

func vscodeArtifactObject(value any, fields string) (map[string]any, error) {
	row, ok := value.(map[string]any)
	names := strings.Fields(fields)
	if !ok || len(row) != len(names) {
		return nil, fmt.Errorf("closed artifact fields required")
	}
	for _, name := range names {
		if _, ok := row[name]; !ok {
			return nil, fmt.Errorf("artifact field missing")
		}
	}
	return row, nil
}
func vscodeArtifactIdentity(value any, archive bool) error {
	fields := "device inode mode uid gid"
	if archive {
		fields += " links size mtime_ns ctime_ns"
	}
	row, e := vscodeArtifactObject(value, fields)
	if e != nil {
		return e
	}
	numbers := map[string]int64{}
	for key, value := range row {
		number, ok := value.(json.Number)
		if !ok {
			return fmt.Errorf("artifact identity type differs")
		}
		n, e := number.Int64()
		if e != nil || n < 0 {
			return fmt.Errorf("artifact number refused")
		}
		numbers[key] = n
	}
	if numbers["inode"] <= 0 || numbers["uid"] != int64(os.Getuid()) || numbers["gid"] != int64(os.Getgid()) {
		return fmt.Errorf("artifact owner differs")
	}
	if archive {
		if numbers["mode"]&syscall.S_IFMT != syscall.S_IFREG || numbers["links"] != 1 || numbers["size"] <= 0 || numbers["size"] > 32<<20 {
			return fmt.Errorf("artifact ZIP identity differs")
		}
	} else if numbers["mode"]&syscall.S_IFMT != syscall.S_IFDIR {
		return fmt.Errorf("artifact directory differs")
	}
	return nil
}
func vscodeArtifactBinding(value any) error {
	if value == nil {
		return nil
	}
	binding, e := vscodeArtifactObject(value, "upload zip bytes sha256")
	if e != nil {
		return e
	}
	if e = vscodeArtifactIdentity(binding["upload"], false); e != nil {
		return e
	}
	if e = vscodeArtifactIdentity(binding["zip"], true); e != nil {
		return e
	}
	size, ok := binding["bytes"].(json.Number)
	zip := binding["zip"].(map[string]any)
	sha, shaOK := binding["sha256"].(string)
	if !ok || size != zip["size"] || !shaOK || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(sha) {
		return fmt.Errorf("artifact ZIP binding differs")
	}
	return nil
}
func vscodeProjectArtifactHandoff(body []byte, commit, tree string) ([]byte, error) {
	if len(body) == 0 || len(body) > 16<<10 || bytes.Count(body, []byte{'\n'}) != 1 || !bytes.HasSuffix(body, []byte{'\n'}) {
		return nil, fmt.Errorf("single bounded artifact frame required")
	}
	value, e := shared.DecodeStrictJSONObject(body)
	if e != nil {
		return nil, e
	}
	row, e := vscodeArtifactObject(value, "schema_version source_commit source_tree collection_complete output archives")
	if e != nil {
		return nil, e
	}
	_, status := row["collection_complete"].(bool)
	if row["schema_version"] != json.Number("1") || row["source_commit"] != commit || row["source_tree"] != tree || !vscodeProjectIdentity.MatchString(commit) || !vscodeProjectIdentity.MatchString(tree) || !status {
		return nil, fmt.Errorf("artifact handoff source differs")
	}
	output, e := vscodeArtifactObject(row["output"], "root frontend native")
	if e != nil {
		return nil, e
	}
	for _, identity := range output {
		if e = vscodeArtifactIdentity(identity, false); e != nil {
			return nil, e
		}
	}
	archives, e := vscodeArtifactObject(row["archives"], "frontend native")
	if e != nil {
		return nil, e
	}
	for _, binding := range archives {
		if e = vscodeArtifactBinding(binding); e != nil {
			return nil, e
		}
	}
	return bytes.TrimSuffix(body, []byte{'\n'}), nil
}

func vscodeProjectForwardHandoff(r ports.CommandResult, runErr, contextErr error, commit, tree string) ([]byte, error) {
	if runErr != nil || contextErr != nil || !r.CleanupComplete || (r.ExitCode != 0 && r.ExitCode != 2) || len(r.Stderr) != 0 || len(r.Stdout) >= 1<<20 {
		return nil, fmt.Errorf("artifact process/capture/cleanup unproved")
	}
	return vscodeProjectArtifactHandoff(r.Stdout, commit, tree)
}

// The test delegates privileged host ownership to the existing reviewed strict
// controller. No client, sudo, fake HOME or fabricated hosted eligibility is
// introduced by this wrapper; ordinary full-tree supervision contains Python.
func TestAgentpluginsVSCodeNativeProjectGatewayLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_VSCODE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in genuine hosted Ubuntu 24 trusted gateway lifecycle")
	}
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" || os.Getenv("ImageOS") != "ubuntu24" || os.Getuid() <= 0 || os.Geteuid() != os.Getuid() || os.Getgid() <= 0 || os.Getegid() != os.Getgid() {
		t.Fatal("actual original nonroot GitHub-hosted Ubuntu 24 amd64 identity required")
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
	handoff, err := vscodeProjectForwardHandoff(r, runErr, ctx.Err(), commit, tree)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(vscodeArtifactPrefix + string(handoff))
	if r.ExitCode != 0 {
		t.Fatal("trusted native collection failed; admitted negative handoff retained")
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
	for _, kind := range []string{"valid", "wrong-source", "missing-stage", "fake-auth", "bad-cleanup", "deadline", "duplicate", "stale-user-prepared"} {
		value := map[string]any{"evidence_kind": "vscode-trusted-project-frontend", "mode": "trusted-project-v1", "source_commit": commit, "source_tree": tree, "collection_complete": true, "vendor_authentication_qualified": false, "real_model_qualified": false, "user_public_state": "not-installed", "user_native_install_qualified": false, "frontend_cleanup": map[string]any{"complete": true}, "controller": map[string]any{"controller_reaped": true, "exit": 0, "error": nil}, "elapsed_seconds": 890}
		flags := map[string]bool{}
		for _, key := range vscodeProjectQualifications {
			flags[key] = true
		}
		value["qualification"] = flags
		switch kind {
		case "stale-user-prepared":
			value["user_public_state"] = "prepared"
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

func TestVSCodeProjectGatewayEvidenceArtifactForwarding(t *testing.T) {
	commit, tree := strings.Repeat("a", 40), strings.Repeat("b", 40)
	identity := map[string]any{"device": 1, "inode": 2, "mode": syscall.S_IFDIR | 0700, "uid": os.Getuid(), "gid": os.Getgid()}
	value := map[string]any{"schema_version": 1, "source_commit": commit, "source_tree": tree, "collection_complete": false, "output": map[string]any{"root": identity, "frontend": identity, "native": identity}, "archives": map[string]any{"frontend": nil, "native": nil}}
	body, e := json.Marshal(value)
	if e != nil {
		t.Fatal(e)
	}
	body = append(body, '\n')
	valid := ports.CommandResult{CleanupComplete: true, ExitCode: 2, Stdout: body}
	if _, e = vscodeProjectForwardHandoff(valid, nil, nil, commit, tree); e != nil {
		t.Fatal(e)
	}
	for _, kind := range []string{"cleanup", "capture", "cancel", "exit", "trailing", "duplicate", "wrong-source"} {
		t.Run(kind, func(t *testing.T) {
			r := valid
			var runErr, contextErr error
			switch kind {
			case "cleanup":
				r.CleanupComplete = false
			case "capture":
				runErr = fmt.Errorf("inert late capture error")
			case "cancel":
				contextErr = context.Canceled
			case "exit":
				r.ExitCode = 1
			case "trailing":
				r.Stdout = append(append([]byte{}, body...), 'x')
			case "duplicate":
				r.Stdout = append(append([]byte{}, body...), body...)
			case "wrong-source":
				r.Stdout = bytes.Replace(body, []byte(tree), []byte(commit), 1)
			}
			if _, e := vscodeProjectForwardHandoff(r, runErr, contextErr, commit, tree); e == nil {
				t.Fatal("unproved handoff forwarded")
			}
		})
	}
}
