//go:build linux

package pluginkitairepo_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const kiroBoundaryArchive = "83dd96e00a75937958096da0436703562bc8c841bef66bde00a4caa5494d8be8"
const kiroBoundaryLimit = 128 << 10

type kiroBoundaryIdentity struct {
	Source        map[string]string `json:"source"`
	ArchiveSHA    string            `json:"archive_sha256"`
	ArchiveBytes  int64             `json:"archive_bytes"`
	ArchiveMember string            `json:"archive_member"`
	Binary        string            `json:"binary"`
	BinarySHA     string            `json:"binary_sha256"`
	Helper        string            `json:"helper"`
	HelperSHA     string            `json:"helper_sha256"`
	RuntimeFiles  map[string]string `json:"runtime_files"`
}

type kiroBoundaryExchange struct {
	input, output bytes.Buffer
	phase, stop   string
	lines         int
}

func kiroBoundarySHA(body []byte) string {
	digest := sha256.Sum256(body)
	return hex.EncodeToString(digest[:])
}

func kiroBoundaryNow() int64 {
	var value unix.Timespec
	if unix.ClockGettime(unix.CLOCK_MONOTONIC, &value) != nil {
		return 0
	}
	return value.Nano()
}

func kiroBoundaryRemaining(origin, now int64) (time.Duration, error) {
	if origin <= 0 || now < origin || now-origin >= int64(120*time.Second) {
		return 0, fmt.Errorf("original Kiro work deadline exhausted or invalid")
	}
	return 120*time.Second - time.Duration(now-origin), nil
}

func kiroBoundaryUnit(unit, membership string, uid, gid, expectedUID, expectedGID int) error {
	if !regexp.MustCompile(`^uap-kiro-acp-boundary-[0-9a-f]{32}\.service$`).MatchString(unit) || membership != "0::/system.slice/"+unit+"\n" || uid <= 0 || gid <= 0 || uid != expectedUID || gid != expectedGID {
		return fmt.Errorf("actual original ordinary Kiro delegation differs")
	}
	return nil
}

func (e *kiroBoundaryExchange) send(writer io.Writer, id int, method string, params map[string]any) error {
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil || e.input.Len()+len(body)+1 > 4096 {
		return fmt.Errorf("fixed request bound")
	}
	body = append(body, '\n')
	n, err := writer.Write(body)
	e.input.Write(body[:n])
	if err != nil {
		return err
	}
	if n != len(body) {
		return io.ErrShortWrite
	}
	return nil
}

func (e *kiroBoundaryExchange) response(scanner *bufio.Scanner, id string) (map[string]any, error) {
	for scanner.Scan() {
		e.lines++
		if e.lines > 128 || e.output.Len() > kiroBoundaryLimit {
			return nil, fmt.Errorf("observed stdout bound exceeded")
		}
		value, err := projectNativeRuntimeJSON(scanner.Bytes())
		row, ok := value.(map[string]any)
		if err != nil || !ok || row["jsonrpc"] != "2.0" {
			return nil, fmt.Errorf("malformed ACP record")
		}
		for key := range row {
			if key != "jsonrpc" && key != "id" && key != "method" && key != "params" && key != "result" && key != "error" {
				return nil, fmt.Errorf("unknown ACP envelope field")
			}
		}
		if method, exists := row["method"]; exists {
			name, valid := method.(string)
			_, hasResult := row["result"]
			_, hasError := row["error"]
			if !valid || name == "" || hasResult || hasError {
				return nil, fmt.Errorf("invalid ACP server message")
			}
			if _, requested := row["id"]; requested {
				e.stop = e.phase + "-server-request-unanswered"
				return nil, nil
			}
			continue
		}
		if row["id"] != json.Number(id) || row["params"] != nil {
			return nil, fmt.Errorf("unexpected response identity")
		}
		result, hasResult := row["result"]
		failure, hasFailure := row["error"]
		if hasResult == hasFailure {
			return nil, fmt.Errorf("one response result or error required")
		}
		if hasFailure {
			detail, valid := failure.(map[string]any)
			code, number := detail["code"].(json.Number)
			_, integerError := strconv.ParseInt(string(code), 10, 64)
			message, text := detail["message"].(string)
			if !valid || !number || integerError != nil || !text || message == "" {
				return nil, fmt.Errorf("invalid server error envelope")
			}
			e.stop = e.phase + "-server-error"
			return nil, nil
		}
		object, valid := result.(map[string]any)
		if !valid {
			return nil, fmt.Errorf("object response required")
		}
		return object, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("ACP observed prefix: %w", err)
	}
	return nil, io.ErrUnexpectedEOF
}

// This diagnostic owns its reader, not VerifyACP's settlement contract. It
// records exactly the prefix it reads, including any scanner read-ahead. It
// never claims that bytes after its deliberately stopped exchange are absent.
func (e *kiroBoundaryExchange) run(stdin io.Writer, stdout io.Reader, cwd string) error {
	scanner := bufio.NewScanner(io.TeeReader(io.LimitReader(stdout, kiroBoundaryLimit+1), &e.output))
	scanner.Buffer(make([]byte, 4096), 64<<10)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		if atEOF && len(data) > 0 && !bytes.ContainsRune(data, '\n') {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return bufio.ScanLines(data, atEOF)
	})
	e.phase = "initialize"
	params := map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}, "clientInfo": map[string]any{"name": "agentplugins", "version": "1"}}
	if err := e.send(stdin, 0, "initialize", params); err != nil {
		return err
	}
	initial, err := e.response(scanner, "0")
	if err != nil || e.stop != "" {
		return err
	}
	if initial["protocolVersion"] != json.Number("1") {
		e.stop = "initialize-result-unrecognized"
		return nil
	}
	e.phase = "session-new"
	if err := e.send(stdin, 1, "session/new", map[string]any{"cwd": cwd, "mcpServers": []any{}}); err != nil {
		return err
	}
	_, err = e.response(scanner, "1")
	if err == nil && e.stop == "" {
		e.stop = "session-new-result-observed"
	}
	return err
}

func kiroBoundaryReadIdentity(t *testing.T) kiroBoundaryIdentity {
	t.Helper()
	path := os.Getenv("UAP_KIRO_BOUNDARY_IDENTITY")
	body, err := projectNativeRuntimeRead(path, 512<<10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = projectNativeRuntimeJSON(body); err != nil {
		t.Fatal(err)
	}
	var identity kiroBoundaryIdentity
	if json.Unmarshal(body, &identity) != nil || identity.ArchiveSHA != kiroBoundaryArchive || identity.ArchiveBytes != 137418453 || identity.ArchiveMember != "kirocli/bin/kiro-cli" || len(identity.Source) != 2 || len(identity.RuntimeFiles) == 0 || len(identity.RuntimeFiles) > 8192 {
		t.Fatal("official archive-derived source identity required")
	}
	for _, key := range []string{"commit", "tree"} {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(identity.Source[key]) {
			t.Fatal("exact source identity required")
		}
	}
	inputs := filepath.Dir(path)
	executable, err := os.Executable()
	if err != nil || identity.Helper != executable || identity.Helper != filepath.Join(inputs, "boundary.test") || identity.Binary != filepath.Join(inputs, "vendor", filepath.FromSlash(identity.ArchiveMember)) || identity.RuntimeFiles[identity.Binary] != identity.BinarySHA {
		t.Fatal("exact helper and archive member paths required")
	}
	return identity
}

func kiroBoundaryKeepInputs(t *testing.T, identity kiroBoundaryIdentity) func() {
	t.Helper()
	files := map[string]string{identity.Helper: identity.HelperSHA}
	identityPath := os.Getenv("UAP_KIRO_BOUNDARY_IDENTITY")
	identityDigest, err := projectNativeRuntimeDigest(identityPath)
	if err != nil {
		t.Fatal(err)
	}
	files[identityPath] = identityDigest
	for path, digest := range identity.RuntimeFiles {
		if !strings.HasPrefix(path, filepath.Dir(identity.Helper)+"/vendor/") {
			t.Fatal("archive member outside immutable inputs")
		}
		files[path] = digest
	}
	held := map[string]os.FileInfo{}
	for path := range files {
		info, err := os.Lstat(path)
		var fs syscall.Statfs_t
		if err != nil || syscall.Statfs(path, &fs) != nil || fs.Flags&unix.ST_RDONLY == 0 {
			t.Fatal("read-only mounted input required")
		}
		held[path] = info
	}
	return func() {
		for path, expected := range files {
			digest, err := projectNativeRuntimeDigest(path)
			current, statErr := os.Lstat(path)
			if err != nil || statErr != nil || digest != expected || !projectNativeRuntimeStableStat(current.Sys(), held[path].Sys()) {
				t.Fatal("immutable input custody changed")
			}
		}
	}
}

func kiroBoundaryProfile(t *testing.T, root, label, binary string) (string, []string) {
	t.Helper()
	profile := filepath.Join(root, label)
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	env := make([]string, 0, 11)
	env = append(env, "PATH="+filepath.Dir(binary)+":/usr/bin:/bin", "LANG=C.UTF-8", "TERM=dumb", "NO_COLOR=1")
	for key, name := range map[string]string{"HOME": "home", "XDG_CONFIG_HOME": "config", "XDG_CACHE_HOME": "cache", "XDG_DATA_HOME": "data", "XDG_STATE_HOME": "state", "XDG_RUNTIME_DIR": "xdg-runtime", "TMPDIR": "tmp"} {
		path := filepath.Join(profile, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		env = append(env, key+"="+path)
	}
	cwd := filepath.Join(profile, "cwd")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	return cwd, env
}

func TestAgentpluginsKiroACPBoundaryCapture(t *testing.T) {
	if os.Getenv("UAP_KIRO_BOUNDARY_CAPTURE") != "1" {
		t.Skip("separate hosted diagnostic opt-in required; no native qualification")
	}
	if runtime.GOARCH != "amd64" || os.Getenv("GITHUB_ACTIONS") != "true" || os.Getenv("RUNNER_ENVIRONMENT") != "github-hosted" || os.Getenv("AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED") != "1" {
		t.Fatal("actual opted-in GitHub-hosted Linux amd64 required")
	}
	unit := os.Getenv("UAP_KIRO_BOUNDARY_UNIT")
	uid, e1 := strconv.Atoi(os.Getenv("UAP_KIRO_BOUNDARY_UID"))
	gid, e2 := strconv.Atoi(os.Getenv("UAP_KIRO_BOUNDARY_GID"))
	membership, e3 := os.ReadFile("/proc/self/cgroup")
	if e1 != nil || e2 != nil || e3 != nil || os.Geteuid() != os.Getuid() || os.Getegid() != os.Getgid() || kiroBoundaryUnit(unit, string(membership), os.Getuid(), os.Getgid(), uid, gid) != nil {
		t.Fatal("actual original ordinary delegation required")
	}
	status, err := os.ReadFile("/proc/self/status")
	if err != nil || !bytes.Contains(status, []byte("NoNewPrivs:\t1\n")) {
		t.Fatal("NoNewPrivileges required")
	}
	cgroup := filepath.Join("/sys/fs/cgroup/system.slice", unit)
	info, err := os.Lstat(cgroup)
	var fs syscall.Statfs_t
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Sys().(*syscall.Stat_t).Uid != uint32(uid) || syscall.Statfs(cgroup, &fs) != nil || uint64(fs.Type) != 0x63677270 {
		t.Fatal("actual delegated cgroup2 required")
	}
	origin, err := strconv.ParseInt(os.Getenv("UAP_KIRO_BOUNDARY_ORIGIN_NS"), 10, 64)
	remaining, clockErr := kiroBoundaryRemaining(origin, kiroBoundaryNow())
	if err != nil || clockErr != nil {
		t.Fatal("one original work deadline required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), remaining)
	defer cancel()
	identity := kiroBoundaryReadIdentity(t)
	unchanged := kiroBoundaryKeepInputs(t, identity)
	unchanged()
	root := os.Getenv("UAP_KIRO_BOUNDARY_RUNTIME")
	if root != filepath.Join(filepath.Dir(filepath.Dir(identity.Helper)), "runtime") || projectNativeRuntimeParents(filepath.Join(root, "leaf")) != nil {
		t.Fatal("exact owned runtime directory required")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("fresh empty runtime required")
	}
	exchange := &kiroBoundaryExchange{}
	report := map[string]any{"diagnostic_only": true, "capture_complete": false, "identity": identity,
		"qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false},
		"stdout_scope":  "read-prefix-through-exchange-stop", "stdout_complete": false, "stderr_scope": "unavailable-from-duplex-api",
		"delegation": map[string]any{"unit": unit, "uid": uid, "gid": gid, "membership": string(membership), "no_new_privileges": true, "origin_ns": origin}}
	defer func() {
		report["stdin"], report["stdout"] = append([]byte{}, exchange.input.Bytes()...), append([]byte{}, exchange.output.Bytes()...)
		report["stdin_sha256"], report["stdout_sha256"] = kiroBoundarySHA(exchange.input.Bytes()), kiroBoundarySHA(exchange.output.Bytes())
		report["phase"], report["stop_reason"], report["observed_records"] = exchange.phase, exchange.stop, exchange.lines
		report["capture_complete"] = !t.Failed()
		body, marshalErr := json.Marshal(report)
		if marshalErr != nil || len(body) > 768<<10 {
			t.Error("bounded helper receipt serialization failed")
			return
		}
		fmt.Printf("UAP_KIRO_ACP_BOUNDARY=%s\n", body)
	}()
	if err = (process.OS{}).DuplexCapability(); err != nil {
		report["runner_error"] = err.Error()
		t.Fatal("unchanged OS duplex capability unavailable")
	}
	report["duplex_capability"] = true
	versionCWD, versionEnv := kiroBoundaryProfile(t, root, "version", identity.Binary)
	versionCtx, versionCancel := context.WithTimeout(ctx, 10*time.Second)
	version, versionErr := (process.OS{}).Run(versionCtx, ports.Command{Argv: []string{identity.Binary, "--version"}, Env: versionEnv, Dir: versionCWD, StdoutLimitBytes: 4096})
	versionCancel()
	report["version"] = map[string]any{"exit": version.ExitCode, "stdout": version.Stdout, "stderr": version.Stderr, "error": fmt.Sprint(versionErr)}
	unchanged()
	if versionErr != nil || version.ExitCode != 0 || string(version.Stdout) != "kiro-cli 3.0.0\n" || len(version.Stderr) != 0 {
		t.Fatal("exact native version or finite cleanup failed")
	}
	cwd, env := kiroBoundaryProfile(t, root, "acp", identity.Binary)
	argv := []string{identity.Binary, "acp", "--agent-engine", "v3", "--auth-method", "cli"}
	report["argv"], report["cwd"], report["environment"] = argv, cwd, env
	bounded, stop := context.WithTimeout(ctx, 40*time.Second)
	err = (process.OS{}).RunDuplexWithPlannedShutdown(bounded, ports.Command{Argv: argv, Env: env, Dir: cwd}, func(stdin io.Writer, stdout io.Reader) error {
		return exchange.run(stdin, stdout, cwd)
	})
	deadlineErr := bounded.Err()
	stop()
	unchanged()
	if err != nil || deadlineErr != nil || ctx.Err() != nil || exchange.stop == "" {
		report["runner_error"], report["deadline_error"] = fmt.Sprint(err), fmt.Sprint(deadlineErr)
		t.Fatal("ACP diagnostic transport, deadline or planned cleanup failed")
	}
	report["planned_shutdown_verified"] = true
}

func TestKiroBoundaryInertExchange(t *testing.T) {
	initial := "{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{\"protocolVersion\":1}}\n"
	for _, item := range []struct {
		name, stdout, stop string
		requests           int
		failure            bool
	}{
		{"initialize-error", "{\"jsonrpc\":\"2.0\",\"id\":0,\"error\":{\"code\":-1,\"message\":\"unclassified native error\"}}\n", "initialize-server-error", 1, false},
		{"session-error", initial + "{\"jsonrpc\":\"2.0\",\"id\":1,\"error\":{\"code\":-2,\"message\":\"boundary\"}}\n", "session-new-server-error", 2, false},
		{"session-result", initial + "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"sessionId\":\"inert\"}}\n", "session-new-result-observed", 2, false},
		{"server-request", "{\"jsonrpc\":\"2.0\",\"id\":7,\"method\":\"session/request_permission\",\"params\":{}}\n", "initialize-server-request-unanswered", 1, false},
		{"unknown-protocol", strings.Replace(initial, "Version\":1", "Version\":2", 1), "initialize-result-unrecognized", 1, false},
		{"duplicate", "{\"jsonrpc\":\"2.0\",\"id\":0,\"id\":0,\"result\":{}}\n", "", 1, true},
		{"nonfinite", "{\"jsonrpc\":\"2.0\",\"id\":0,\"result\":{\"x\":1e999}}\n", "", 1, true},
		{"wrong-id", strings.Replace(initial, "\"id\":0", "\"id\":false", 1), "", 1, true},
		{"early-eof", "", "", 1, true},
		{"unterminated-json", strings.TrimSuffix(initial, "\n"), "", 1, true},
		{"oversized", strings.Repeat("x", 64<<10) + "\n", "", 1, true},
	} {
		t.Run(item.name, func(t *testing.T) {
			exchange := &kiroBoundaryExchange{}
			var sent bytes.Buffer
			err := exchange.run(&sent, strings.NewReader(item.stdout), "/owned/empty")
			if (err != nil) != item.failure || exchange.stop != item.stop || bytes.Count(sent.Bytes(), []byte("\n")) != item.requests || !bytes.Equal(sent.Bytes(), exchange.input.Bytes()) {
				t.Fatalf("unexpected diagnostic boundary: %v, %s, %s", err, exchange.stop, sent.Bytes())
			}
			if bytes.Contains(sent.Bytes(), []byte("authenticate")) || bytes.Contains(sent.Bytes(), []byte("session/prompt")) || bytes.Contains(sent.Bytes(), []byte("tools/call")) {
				t.Fatal("diagnostic emitted unauthorized protocol")
			}
			if item.requests == 2 && !bytes.Contains(sent.Bytes(), []byte(`"mcpServers":[]`)) {
				t.Fatal("only empty MCP session allowed")
			}
		})
	}
}

func TestKiroBoundaryInertContainmentIdentity(t *testing.T) {
	unit := "uap-kiro-acp-boundary-" + strings.Repeat("a", 32) + ".service"
	if kiroBoundaryUnit(unit, "0::/system.slice/"+unit+"\n", 1001, 1001, 1001, 1001) != nil {
		t.Fatal("exact ordinary membership refused")
	}
	for _, membership := range []string{"0::/\n", "0::/system.slice/other.service\n", "0::/system.slice/" + unit + "/child\n"} {
		if kiroBoundaryUnit(unit, membership, 1001, 1001, 1001, 1001) == nil {
			t.Fatal("detached membership admitted")
		}
	}
	if kiroBoundaryUnit(unit, "0::/system.slice/"+unit+"\n", 0, 0, 0, 0) == nil || kiroBoundaryUnit(unit, "0::/system.slice/"+unit+"\n", 1002, 1001, 1001, 1001) == nil {
		t.Fatal("root or changed UID admitted")
	}
	for _, pair := range [][2]int64{{0, 1}, {2, 1}, {1, 1 + int64(120*time.Second)}} {
		if _, err := kiroBoundaryRemaining(pair[0], pair[1]); err == nil {
			t.Fatal("invalid or renewed deadline admitted")
		}
	}
}
