// Command clinesession runs only the disposable Cline session acceptance actions.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var sourceCommit, sourceTree, nativeSHA, installerSHA, probeSHA, fixtureSHA, gitBinary, gitSHA, goBinary, goSHA string

const name = "uap-cline-session"
const serverName = "uap_session_probe"

type request struct {
	Schema   int    `json:"schema_version"`
	Root     string `json:"root"`
	Action   string `json:"action"`
	Commit   string `json:"source_commit"`
	Tree     string `json:"source_tree"`
	Deadline int64  `json:"deadline_unix_ns"`
}
type transport struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
	CWD     string            `json:"cwd"`
}
type entry struct {
	Transport   transport `json:"transport"`
	Disabled    bool      `json:"disabled"`
	AutoApprove []string  `json:"autoApprove"`
}
type runner func(context.Context, ports.Command, time.Duration) (ports.CommandResult, error)

func suffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
func decode(body []byte) (request, error) {
	var r request
	seen, err := shared.DecodeStrictJSONObject(body)
	if err != nil || len(seen) != 6 {
		return r, errors.New("request object refused")
	}
	for key := range seen {
		if !map[string]bool{"schema_version": true, "root": true, "action": true, "source_commit": true, "source_tree": true, "deadline_unix_ns": true}[key] {
			return r, errors.New("request field refused")
		}
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return r, err
	}
	if r.Schema != 1 || r.Commit != sourceCommit || r.Tree != sourceTree || len(r.Commit) != 40 || len(r.Tree) != 40 || !filepath.IsAbs(r.Root) || filepath.Clean(r.Root) != r.Root || !strings.HasPrefix(filepath.Base(r.Root), "uap-cline-session-") {
		return r, errors.New("request identity refused")
	}
	return r, nil
}
func unchanged(a, b os.FileInfo) bool {
	return os.SameFile(a, b) && a.Mode() == b.Mode() && a.Size() == b.Size() && a.ModTime().Equal(b.ModTime())
}
func heldRead(path, root string, limit int64) (body []byte, resultErr error) {
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return nil, err
	}
	directory, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	before, err := directory.Lstat(relative)
	if err != nil || !before.Mode().IsRegular() || before.Size() < 0 || before.Size() > limit {
		return nil, errors.New("ordinary file refused")
	}
	file, err := atomicfile.OpenRootReadFile(directory, relative)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	held, err := file.Stat()
	if err != nil || !unchanged(before, held) {
		return nil, errors.New("opened file differs")
	}
	body, err = io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(body)) != before.Size() {
		return nil, errors.New("file byte bound")
	}
	after, err := file.Stat()
	linked, linkErr := directory.Lstat(relative)
	if err != nil || linkErr != nil || !unchanged(held, after) || !unchanged(held, linked) {
		return nil, errors.New("file changed")
	}
	return body, nil
}
func pin(path, root, expected string) error {
	body, err := heldRead(path, root, 256<<20)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(body)
	if len(expected) != 64 || hex.EncodeToString(sum[:]) != expected {
		return errors.New("runtime hash differs")
	}
	return nil
}
func environment(root string, cold bool) []string {
	home := filepath.Join(root, "home")
	cache := filepath.Join(root, "cache")
	if cold {
		home, cache = filepath.Join(root, "cold-home"), filepath.Join(root, "cold-cache")
	}
	result := []string{"HOME=" + home, "USERPROFILE=" + home, "PATH=" + filepath.Join(root, "vendor", "package", "bin") + string(os.PathListSeparator) + filepath.Dir(gitBinary) + string(os.PathListSeparator) + filepath.Dir(goBinary), "AGENTPLUGINS_HOME=" + cache, "CLINE_NO_AUTO_UPDATE=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.hooksPath", "GIT_CONFIG_VALUE_0=" + os.DevNull, "NO_COLOR=1", "TERM=dumb", "LANG=C.UTF-8", "GOWORK=off", "GOTOOLCHAIN=local", "GOCACHE=" + filepath.Join(root, "go-cache"), "GOPATH=" + filepath.Join(root, "go-path"), "APPDATA=" + filepath.Join(home, "appdata"), "LOCALAPPDATA=" + filepath.Join(home, "localappdata")}
	for _, key := range []string{"CONFIG", "DATA", "CACHE", "STATE"} {
		result = append(result, "XDG_"+key+"_HOME="+filepath.Join(home, "xdg-"+strings.ToLower(key)))
	}
	for _, key := range []string{"SystemRoot", "SystemDrive", "WINDIR"} {
		if value := os.Getenv(key); runtime.GOOS == "windows" && value != "" {
			result = append(result, key+"="+value)
		}
	}
	for _, key := range []string{"TEMP", "TMP", "TMPDIR"} {
		result = append(result, key+"="+filepath.Join(root, "tmp"))
	}
	return result
}
func paths(root string) (string, string, string) {
	return filepath.Join(root, "bin", "agentplugins"+suffix()), filepath.Join(root, "vendor", "package", "bin", "cline"+suffix()), filepath.Join(root, "bin", "probe"+suffix())
}
func fixedArgs(root string) map[string][]string {
	return map[string][]string{
		"install":        {"add", filepath.Join(root, "source"), "--yes", "--format", "json", "--no-save"},
		"repeat-install": {"add", filepath.Join(root, "source"), "--yes", "--format", "json", "--no-save"},
		"update":         {"update", filepath.Join(root, "source"), "--yes", "--format", "json"},
		"repair":         {"repair", name, "--yes", "--format", "json"}, "remove": {"remove", name, "--yes", "--format", "json"},
		"declined-start": {"run", "--inspect"}, "install-yes-is-not-start-consent": {"run", "--inspect", "--yes"},
		"preflight-list": {"list", "--format", "json"}, "preflight-doctor": {"doctor", "--format", "json"}, "preview": {"run", "--dry-run", "--format", "json"},
		"session-inspect": {"run", "--inspect", "--confirm-client-start"}, "missing-package-refusal": {"run", "--inspect", "--confirm-client-start"}, "removed-package-refusal": {"run", "--inspect", "--confirm-client-start"},
		"saved-add":           {"add", "https://github.com/777genius/universal-agent-plugins/tree/" + sourceCommit + "/repotests/testdata/agentplugins_native_probe", "--yes", "--format", "json"},
		"cold-locked-install": {"install", "--locked", "--yes", "--format", "json"}, "cold-repeat": {"install", "--locked", "--yes", "--format", "json"}, "cold-inspect": {"run", "--inspect", "--confirm-client-start"},
	}
}
func command(r request) (ports.Command, error) {
	uap, native, _ := paths(r.Root)
	project := filepath.Join(r.Root, "a")
	cmd := ports.Command{Env: environment(r.Root, strings.HasPrefix(r.Action, "cold-")), Dir: project, StdoutLimitBytes: 1 << 20}
	switch r.Action {
	case "native-version":
		cmd.Argv = []string{native, "--version"}
		return cmd, nil
	case "project-init", "saved-init", "cold-init":
		cmd.Argv = []string{gitBinary, "-c", "core.hooksPath=" + os.DevNull, "init", "--quiet"}
		if r.Action != "project-init" {
			cmd.Dir = filepath.Join(r.Root, strings.TrimSuffix(r.Action, "-init"))
		}
		return cmd, nil
	case "ordinary-native-user-source":
		cmd.Argv = []string{native, "--auto-approve", "false", "config", "mcp", "--json"}
		cmd.Dir = filepath.Join(r.Root, "b")
		return cmd, nil
	case "cold-build":
		t, err := selected(r.Root, "cold-session")
		if err != nil {
			return cmd, err
		}
		cmd.Argv = []string{goBinary, "build", "-o", filepath.Join(r.Root, "cold-compiled-probe"+suffix()), filepath.Join(t.CWD, "main.go")}
		return cmd, nil
	case "direct-create", "direct-session", "cold-create", "cold-session":
		self, err := os.Executable()
		if err != nil {
			return cmd, err
		}
		cmd.Argv = []string{self, "--protocol", r.Root, r.Action}
		return cmd, nil
	}
	args, ok := fixedArgs(r.Root)[r.Action]
	if !ok {
		return cmd, errors.New("unknown fixed action")
	}
	if r.Action == "saved-add" {
		project = filepath.Join(r.Root, "saved")
	}
	if strings.HasPrefix(r.Action, "cold-") {
		project = filepath.Join(r.Root, "cold")
	}
	cmd.Dir = project
	cmd.Argv = append([]string{uap}, args...)
	cmd.Argv = append(cmd.Argv, "--target", "cline", "--scope", "project", "--project-root", project)
	return cmd, nil
}
func verifyPins(root string) error {
	uap, native, probe := paths(root)
	for _, item := range [][3]string{{uap, root, installerSHA}, {native, root, nativeSHA}, {probe, root, probeSHA}, {gitBinary, filepath.Dir(gitBinary), gitSHA}, {goBinary, filepath.Dir(goBinary), goSHA}} {
		if err := pin(item[0], item[1], item[2]); err != nil {
			return err
		}
	}
	return nil
}
func settled(result ports.CommandResult, executionErr, contextErr error) error {
	if executionErr != nil || contextErr != nil || !result.CleanupComplete || result.NoChildStarted || len(result.Stdout) > 1<<20 || len(result.Stderr) > 32768 {
		return errors.Join(errors.New("command settlement refused"), executionErr, contextErr)
	}
	return nil
}
func run(input io.Reader, output io.Writer, invoke runner) error {
	reader := bufio.NewReaderSize(input, 16385)
	body, err := reader.ReadSlice('\n')
	if err != nil || len(body) > 16384 {
		return errors.New("request frame bound")
	}
	req, err := decode(body)
	if err != nil {
		return err
	}
	cmd, err := command(req)
	if err != nil {
		return err
	}
	remaining := time.Until(time.Unix(0, req.Deadline))
	if remaining <= 0 || remaining > 125*time.Second {
		return errors.New("original deadline refused")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, remaining)
	defer cancel()
	go func() { var extra [1]byte; _, _ = reader.Read(extra[:]); cancel() }()
	before, err := os.Lstat(req.Root)
	canonical, canonicalErr := filepath.EvalSymlinks(req.Root)
	if err != nil || canonicalErr != nil || canonical != req.Root || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return errors.New("root refused")
	}
	if err = verifyPins(req.Root); err != nil {
		return err
	}
	result, err := invoke(ctx, cmd, 5*time.Second)
	if err = settled(result, err, ctx.Err()); err != nil {
		return err
	}
	after, err := os.Lstat(req.Root)
	if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() {
		return errors.New("root changed")
	}
	if err = verifyPins(req.Root); err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	return json.NewEncoder(output).Encode(map[string]any{"schema_version": 1, "source_commit": sourceCommit, "source_tree": sourceTree, "action": req.Action, "cleanup_complete": true, "exit": result.ExitCode, "stdout": result.Stdout, "stderr": result.Stderr,
		"runtime_sha256": map[string]string{"native": nativeSHA, "installer": installerSHA, "probe": probeSHA, "git": gitSHA, "go": goSHA, "fixture_source": fixtureSHA}})
}
func selected(root, action string) (transport, error) {
	project := filepath.Join(root, "a")
	selectedName := serverName
	if strings.HasPrefix(action, "cold-") {
		project = filepath.Join(root, "cold")
		selectedName = "uap_public_probe"
	}
	var config struct {
		Servers map[string]entry `json:"mcpServers"`
	}
	var zero transport
	body, err := heldRead(filepath.Join(project, ".uap", "clients", "cline", "mcp.json"), root, 1<<20)
	if err != nil {
		return zero, err
	}
	value, err := shared.DecodeStrictJSONObject(body)
	if err != nil {
		return zero, err
	}
	rows, _ := value["mcpServers"].(map[string]any)
	row, _ := rows[selectedName].(map[string]any)
	if disabled, present := row["disabled"]; present && disabled != false {
		return zero, errors.New("selected disablement refused")
	}
	if approved, present := row["autoApprove"]; present {
		items, valid := approved.([]any)
		if !valid || len(items) != 0 {
			return zero, errors.New("selected auto-approval refused")
		}
	}
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&config); err != nil {
		return zero, err
	}
	item, ok := config.Servers[selectedName]
	if !ok || len(config.Servers) != 1 || item.Disabled || len(item.AutoApprove) != 0 || item.Transport.Type != "stdio" || item.Transport.Env == nil {
		return zero, errors.New("selected transport refused")
	}
	return validateSelected(root, project, action, item.Transport)
}
func validateSelected(root, project, action string, t transport) (transport, error) {
	var zero transport

	managed := filepath.Join(project, ".uap", "managed", "clients", "cline")
	if err := fixtureEnvironment(root, action, t.Env); err != nil {
		return zero, err
	}
	if !filepath.IsAbs(t.Command) || !filepath.IsAbs(t.CWD) || pathpolicy.RequireContainedChild(managed, t.Command) != nil || pathpolicy.RequireContainedChild(managed, t.CWD) != nil || t.Env["UAP_TEST_ROOT"] != root || t.Env["UAP_TEST_EVENTS"] != filepath.Join(root, "mcp-events.jsonl") || len(t.Args) > 16 {
		return zero, errors.New("selected helper authority refused")
	}
	if err := pin(t.Command, root, installerSHA); err != nil {
		return zero, err
	}
	active := t.CWD
	data := t.Env["PLUGIN_DATA"]
	if t.Command != filepath.Join(active, managedstdio.RelativeDirectory, managedstdio.ExecutableName) || t.Env["PLUGIN_ROOT"] != active || data != filepath.Join(project, ".uap", "plugin-data", filepath.Base(active)) || pathpolicy.RequireContainedChild(filepath.Join(project, ".uap", "plugin-data"), data) != nil {
		return zero, errors.New("exact helper binding refused")
	}
	child := "./bin/probe" + suffix()
	childArgs := []string{}
	childPath, childSHA := filepath.Join(active, "bin", "probe"+suffix()), probeSHA
	if strings.HasPrefix(action, "cold-") {
		child = "go"
		childArgs = []string{"run", filepath.Join(active, "main.go")}
		childPath, childSHA = filepath.Join(active, "main.go"), fixtureSHA
	}
	if err := pin(childPath, root, childSHA); err != nil {
		return zero, err
	}
	expected := managedstdio.Arguments(active, data, active, pathcontract.Plugin, child, childArgs)
	if strings.Join(t.Args, "\x00") != strings.Join(expected, "\x00") {
		return zero, errors.New("managed arguments refused")
	}
	return t, nil
}
func fixtureEnvironment(root, action string, env map[string]string) error {
	for key := range env {
		if !map[string]bool{"PLUGIN_ROOT": true, "PLUGIN_DATA": true, "UAP_TEST_ROOT": true, "UAP_TEST_NONCE": true, "UAP_TEST_REVISION": true, "UAP_TEST_EVENTS": true, "UAP_TEST_REPORT_OPERATION": true, "GOTOOLCHAIN": true, "GOWORK": true}[key] {
			return errors.New("unknown selected environment")
		}
	}
	if strings.HasPrefix(action, "cold-") {
		nonce, err := heldRead(filepath.Join(root, "nonce.txt"), root, 32)
		if err != nil || len(nonce) != 32 || env["GOTOOLCHAIN"] != "local" || env["GOWORK"] != "off" {
			return errors.New("cold fixture environment refused")
		}
		env["UAP_TEST_ROOT"], env["UAP_TEST_NONCE"] = root, string(nonce)
		env["UAP_TEST_REVISION"], env["UAP_TEST_EVENTS"] = "cold", filepath.Join(root, "mcp-events.jsonl")
	}
	env["UAP_TEST_REPORT_OPERATION"] = "1"
	return nil
}

func main() {
	var err error
	if len(os.Args) == 4 && os.Args[1] == "--protocol" && map[string]bool{"direct-create": true, "direct-session": true, "cold-create": true, "cold-session": true}[os.Args[3]] {
		err = protocol(os.Args[2], os.Args[3])
	} else if len(os.Args) == 1 {
		err = run(os.Stdin, os.Stdout, (process.OS{}).RunWithDescendantExitGrace)
	} else {
		err = errors.New("fixed mode refused")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cline fixed action refused: %.512s\n", err.Error())
		os.Exit(1)
	}
}
