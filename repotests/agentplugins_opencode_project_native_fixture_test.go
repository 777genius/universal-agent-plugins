package pluginkitairepo_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const openCodeProjectLimit = 128 << 10
const openCodeProjectEnvelopeLimit = 32 << 20

type openCodePassiveRecorder struct {
	server   *http.Server
	listener net.Listener
	accepted atomic.Int64
	requests atomic.Int64
	mu       sync.Mutex
	samples  []string
	done     chan struct{}
}

func newOpenCodePassiveRecorder(t *testing.T) *openCodePassiveRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	r := &openCodePassiveRecorder{listener: listener, done: make(chan struct{})}
	r.server = &http.Server{ReadHeaderTimeout: time.Second, ReadTimeout: time.Second, WriteTimeout: time.Second, IdleTimeout: time.Second,
		ConnState: func(c net.Conn, s http.ConnState) {
			if s == http.StateNew {
				r.accepted.Add(1)
				_ = c.SetDeadline(time.Now().Add(time.Second))
			}
		},
		Handler: http.HandlerFunc(func(w http.ResponseWriter, q *http.Request) {
			r.requests.Add(1)
			r.mu.Lock()
			if len(r.samples) < 8 {
				sample := q.Method + " " + q.URL.EscapedPath()
				if len(sample) > 256 {
					sample = sample[:256]
				}
				r.samples = append(r.samples, sample)
			}
			r.mu.Unlock()
			w.WriteHeader(http.StatusServiceUnavailable)
		})}
	go func() { defer close(r.done); _ = r.server.Serve(listener) }()
	return r
}
func (r *openCodePassiveRecorder) counts() [2]int64 {
	return [2]int64{r.accepted.Load(), r.requests.Load()}
}
func (r *openCodePassiveRecorder) url(marker string) string {
	return "http://" + r.listener.Addr().String() + "/" + marker
}
func (r *openCodePassiveRecorder) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := r.server.Shutdown(ctx)
	if err != nil {
		_ = r.server.Close()
	}
	select {
	case <-r.done:
		return err
	case <-time.After(time.Second):
		return fmt.Errorf("passive recorder Serve did not stop")
	}
}

// Every direct child has a predeclared label/class. The complete table is 43
// commands: 4 Git setup, 14 public UAP and 25 native read observations.
func openCodeProjectCommandTable() map[string]string {
	table := map[string]string{}
	for _, label := range []string{"A", "B", "user", "json"} {
		table["git-"+label] = "git"
	}
	table["version"] = "native"
	for _, label := range []string{"add-user", "add-A", "add-B", "repeat-add-A", "unchanged-update-A", "intact-repair-A", "update-A", "missing-repair-A", "remove-A", "repeat-remove-A", "add-same-user", "add-same-A", "add-json", "remove-json"} {
		table[label] = "uap"
	}
	for _, stage := range []string{"initial-A", "initial-B", "stable-A", "updated-A", "repaired-A", "removed-A", "same-user", "same-A", "same-B", "json-installed", "json-removed"} {
		table[stage+"-config"] = "native"
		table[stage+"-skill"] = "native"
	}
	table["inline-A-config"] = "native"
	table["restart-A-config"] = "native"
	return table
}

type openCodeProjectFixture struct {
	root, work, home, user, A, B, short, sourceA, sourceB, sourceUser, sourceControl, sourceJSON, state, client, installer, git string
	ctx                                                                                                                         context.Context
	cancel                                                                                                                      context.CancelFunc
	recorder                                                                                                                    *openCodePassiveRecorder
	files                                                                                                                       map[string][]byte
	bytes                                                                                                                       int
	records                                                                                                                     []map[string]any
	table, used                                                                                                                 map[string]string
	stages                                                                                                                      map[string]nativeStage
	evidence                                                                                                                    map[string]any
	failed                                                                                                                      bool
}

func newOpenCodeProjectFixture(t *testing.T) *openCodeProjectFixture {
	t.Helper()
	nativeRequireDisposable(t)
	root, err := os.MkdirTemp("", "uap-opencode-project-work-")
	if err != nil {
		t.Fatal(err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	f := &openCodeProjectFixture{root: root, work: filepath.Join(root, "work"), files: map[string][]byte{}, table: openCodeProjectCommandTable(), used: map[string]string{}, stages: map[string]nativeStage{}}
	f.ctx, f.cancel = context.WithTimeout(context.Background(), 6*time.Minute)
	f.home = filepath.Join(f.work, "home")
	f.user = filepath.Join(f.work, "user-cwd")
	f.A = filepath.Join(f.work, "A")
	f.B = filepath.Join(f.work, "B")
	f.short = filepath.Join(f.work, "json")
	f.state = filepath.Join(f.work, "installer-state")
	f.sourceA = filepath.Join(f.work, "source-A")
	f.sourceB = filepath.Join(f.work, "source-B")
	f.sourceUser = filepath.Join(f.work, "source-user")
	f.sourceControl = filepath.Join(f.work, "source-control")
	f.sourceJSON = filepath.Join(f.work, "source-json")
	for _, p := range []string{f.home, f.user, f.A, f.B, f.short, f.state, filepath.Join(f.work, "tmp"), filepath.Join(f.work, "xdg-config"), filepath.Join(f.work, "xdg-data"), filepath.Join(f.work, "xdg-cache"), filepath.Join(f.work, "xdg-state")} {
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.client = openCodeNativeBinary(t, "AGENTPLUGINS_OPENCODE_BIN")
	f.installer = openCodeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	f.git, err = exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	f.git, err = filepath.Abs(f.git)
	if err != nil {
		t.Fatal(err)
	}
	f.recorder = newOpenCodePassiveRecorder(t)
	f.evidence = map[string]any{"schema_version": 1, "client": "opencode", "started_utc": time.Now().UTC().Format(time.RFC3339Nano), "native_discovery_qualified": false, "native_mcp_initialization_qualified": false, "native_model_use_qualified": false, "activation": "prepared", "network": "not isolated; passive loopback recorder measures accepted TCP/HTTP only; vendor bootstrap may download/cache dependencies", "fixture_root": root}
	for _, name := range []string{"positive_project_discovery", "unchanged_lifecycle", "update", "missing_repair", "remove", "same_name_precedence", "inline_precedence", "root_json", "foreign_preservation"} {
		f.stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	for _, name := range []string{"native_mcp_initialization", "model_use", "authentication", "trust", "ide", "default_plugins", "enterprise_ancestor_sources"} {
		f.stages[name] = nativeStage{Status: "not_evaluated", Reason: "outside fixed debug observation contract"}
	}
	t.Logf("OpenCode project working authority: %s; only admitted envelope is uploaded", root)
	return f
}
func (f *openCodeProjectFixture) env() []string {
	return append(nativePlatformEnvironment(f.work, f.home, filepath.Dir(f.client), filepath.Dir(f.git)), "AGENTPLUGINS_HOME="+f.state, "OPENCODE_PURE=1", "OPENCODE_DISABLE_AUTOUPDATE=1", "OPENCODE_DISABLE_MODELS_FETCH=1", "OPENCODE_DISABLE_DEFAULT_PLUGINS=1", "OPENCODE_DISABLE_LSP_DOWNLOAD=1", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(f.home, ".gitconfig"), "GIT_TERMINAL_PROMPT=0")
}
func (f *openCodeProjectFixture) keep(t *testing.T, name string, body []byte) {
	t.Helper()
	if filepath.Base(name) != name || f.files[name] != nil || len(f.files) >= 127 || f.bytes+len(body) > openCodeProjectEnvelopeLimit-(1<<20) {
		f.failed = true
		t.Fatal("project evidence cumulative admission refused")
	}
	f.files[name] = append([]byte(nil), body...)
	f.bytes += len(body)
}
func (f *openCodeProjectFixture) command(t *testing.T, class, label, exe, cwd string, env, args []string) projectNativeCaptureResult {
	t.Helper()
	if f.table[label] != class || f.used[label] != "" || len(f.used) >= 60 {
		f.failed = true
		t.Fatal("command not in finite project table or repeated")
	}
	f.used[label] = class
	if class == "native" && !reflect.DeepEqual(args, []string{"--version"}) && !reflect.DeepEqual(args, []string{"debug", "config"}) && !reflect.DeepEqual(args, []string{"debug", "skill"}) {
		f.failed = true
		t.Fatal("unreviewed native argv")
	}
	timeout := 10 * time.Second
	if class == "uap" {
		timeout = 30 * time.Second
	}
	before := f.recorder.counts()
	result := captureProjectNative(f.ctx, exe, cwd, env, args, openCodeProjectLimit, timeout)
	after := f.recorder.counts()
	outName, errName := label+".stdout.log", label+".stderr.log"
	f.keep(t, outName, result.Stdout)
	f.keep(t, errName, result.Stderr)
	digest := func(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
	f.records = append(f.records, map[string]any{"class": class, "label": label, "argv": append([]string{exe}, args...), "cwd": cwd, "exit": result.Exit, "error": result.Error, "error_kind": result.ErrorKind, "deadline": result.Deadline, "stdout_limit": result.StdoutLimit, "stderr_limit": result.StderrLimit, "elapsed_ms": result.ElapsedMS, "stdout_file": outName, "stderr_file": errName, "stdout_bytes": len(result.Stdout), "stderr_bytes": len(result.Stderr), "stdout_sha256": digest(result.Stdout), "stderr_sha256": digest(result.Stderr), "recorder_before": before, "recorder_after": after})
	if before != [2]int64{} || after != [2]int64{} || projectNativeCaptureAdmission(result, class == "uap" && label == "repeat-remove-A") != nil {
		f.failed = true
		t.Fatal("child deadline/output limit or unexpected MCP connection; see bounded evidence")
	}
	return result
}
func (f *openCodeProjectFixture) setup(t *testing.T) {
	t.Helper()
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	f.evidence["source_identity"] = identity
	pinnedVersion := os.Getenv("AGENTPLUGINS_OPENCODE_VERSION")
	if (pinnedVersion != "1.18.34" && pinnedVersion != "2.2.5") || nativeSHA(t, f.client) != os.Getenv("AGENTPLUGINS_OPENCODE_SHA256") {
		t.Fatal("missing/exact OpenCode V1/V2 pin mismatch")
	}
	f.evidence["client_sha256"] = nativeSHA(t, f.client)
	f.evidence["installer_sha256"] = nativeSHA(t, f.installer)
	f.evidence["scanner"] = nativeProvisionScanner(t, &nativeFixture{Root: f.work})
	for _, item := range []struct{ label, root string }{{"A", f.A}, {"B", f.B}, {"user", f.user}, {"json", f.short}} {
		r := f.command(t, "git", "git-"+item.label, f.git, item.root, f.env(), []string{"-c", "core.hooksPath=/dev/null", "init", "--quiet"})
		if r.Exit != 0 || len(bytesTrim(r.Stderr)) != 0 {
			t.Fatal("fresh Git boundary failed")
		}
		info, err := os.Lstat(filepath.Join(item.root, ".git"))
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("expected fresh Git boundary missing")
		}
	}
	r := f.command(t, "native", "version", f.client, f.user, f.env(), []string{"--version"})
	expectedOutput := pinnedVersion
	if pinnedVersion == "2.2.5" {
		// The pinned V2 native CLI prints its product prefix; V1 prints only
		// the semver. Both are exact byte-for-byte native observations.
		expectedOutput = "opencode v2.2.5"
	}
	if r.Exit != 0 || len(bytesTrim(r.Stderr)) != 0 || string(bytesTrim(r.Stdout)) != expectedOutput {
		t.Fatal("native version is not exact pinned V1/V2 output")
	}
	f.evidence["client_version_measured"] = string(bytesTrim(r.Stdout))
	for _, item := range []struct{ root, marker string }{{f.sourceA, "A-one"}, {f.sourceB, "B-one"}, {f.sourceUser, "USER-same"}, {f.sourceJSON, "JSON-one"}} {
		f.source(t, item.root, item.marker)
	}
	nativeWrite(t, filepath.Join(f.sourceControl, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-opencode-user-control","version":"1.0.0"}`), 0600)
	nativeWrite(t, filepath.Join(f.sourceControl, "skills", "uap-user-control", "SKILL.md"), []byte("---\nname: uap-user-control\ndescription: Inert user control\n---\nUAP-USER-CONTROL\n"), 0600)
	f.writeMCP(t, f.sourceControl, "uap-user-control", "USER-control")
	for _, root := range []string{f.A, f.B} {
		nativeWrite(t, filepath.Join(root, "opencode.jsonc"), []byte(openCodeJSONCComment+`{"mcp":{},"default_agent":"uap-inert-project"}`), 0600)
		nativeWrite(t, filepath.Join(root, ".uap", "data", "caller-owned", "keep.txt"), []byte("caller-owned persistent data\n"), 0600)
	}
	nativeWrite(t, filepath.Join(f.short, "opencode.json"), []byte(`{"mcp":{},"default_agent":"uap-inert-json"}`), 0600)
	nativeWrite(t, filepath.Join(f.work, "xdg-config", "opencode", "opencode.jsonc"), []byte(openCodeJSONCComment+`{"mcp":{},"default_agent":"uap-inert-user"}`), 0600)
}
func bytesTrim(body []byte) []byte { return []byte(strings.TrimSpace(string(body))) }
func (f *openCodeProjectFixture) source(t *testing.T, root, marker string) {
	t.Helper()
	copyTree(t, commandsFixtureSource(t), root)
	path := filepath.Join(root, "skills", commandsGuideSkill, "SKILL.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, path, append(body, []byte("\nUAP-OPENCODE-"+marker+"\n")...), 0600)
	f.writeMCP(t, root, "scope-marker", marker)
}
func (f *openCodeProjectFixture) writeMCP(t *testing.T, root, key, marker string) {
	t.Helper()
	body, err := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{key: map[string]any{"type": "streamable-http", "url": f.recorder.url(marker)}}})
	if err != nil {
		t.Fatal(err)
	}
	nativeWrite(t, filepath.Join(root, "mcp.json"), body, 0600)
}
func (f *openCodeProjectFixture) run(t *testing.T, label, project string, args ...string) map[string]any {
	t.Helper()
	cwd := f.user
	if project != "" {
		cwd = project
		args = append(args, "--scope", "project", "--project-root", project)
	}
	args = append(args, "--target", "opencode", "--format", "json")
	r := f.command(t, "uap", label, f.installer, cwd, f.env(), args)
	if label == "repeat-remove-A" {
		if !openCodeProjectAbsentRemoval(r) {
			t.Fatal("repeat removal did not give exact source-established absence refusal")
		}
		f.evidence["repeat_remove_contract"] = "empty stdout; exact fixed stderr; pure ExitError exit1; unchanged owned state/files"
		return nil
	}
	if !openCodeProjectProgress(label, r.Stderr) {
		t.Fatal("public UAP emitted unexpected progress bytes; see evidence")
	}
	result, err := openCodeProjectConfig(r.Stdout)
	if err != nil {
		t.Fatalf("public UAP whole JSON: %v", err)
	}
	if r.Exit != 0 || result["result"] != "success" {
		t.Fatal("public UAP operation failed; see exact bounded evidence")
	}
	if args[0] != "remove" {
		if project != "" {
			result, err = openCodeProjectAssertionEnvelope(result)
			if err != nil {
				t.Fatalf("%s project result: %v", label, err)
			}
			codexNativeProjectPrepared(t, result)
		} else {
			openCodeAssertCompleted(t, label, result)
		}
	}
	return result
}
func (f *openCodeProjectFixture) binding(t *testing.T, project string) domain.ClientBinding {
	t.Helper()
	s, err := (statev2.Store{Path: filepath.Join(project, ".uap", "state-v2.json"), ProjectRoot: project}).Load()
	if err != nil || len(s.Installations) != 1 || len(s.Installations[0].Clients) != 1 {
		t.Fatalf("project installed state: %v", err)
	}
	for _, b := range s.Installations[0].Clients {
		if b.ClientID != "opencode" || b.Scope != "project" || b.NativeProfileRoot != project || b.Activation != domain.ActivationPrepared || b.Verification != domain.VerificationInstalled {
			t.Fatal("project receipt lost exact Prepared authority")
		}
		return b
	}
	t.Fatal("no project receipt")
	return domain.ClientBinding{}
}
func (f *openCodeProjectFixture) config(t *testing.T, label, project string, extra []string) map[string]any {
	t.Helper()
	env := append(f.env(), extra...)
	r := f.command(t, "native", label+"-config", f.client, project, env, []string{"debug", "config"})
	if r.Exit != 0 || len(bytesTrim(r.Stderr)) != 0 {
		t.Fatal("native config failed or warned; see exact evidence")
	}
	object, err := openCodeProjectConfig(r.Stdout)
	if os.Getenv("AGENTPLUGINS_OPENCODE_VERSION") == "2.2.5" {
		object, err = openCodeProjectV2SourceConfig(
			r.Stdout, filepath.Join(f.work, "xdg-config", "opencode"),
			project, len(extra) > 0, project != f.user)
	}
	if err != nil {
		t.Fatal(err)
	}
	return object
}
func (f *openCodeProjectFixture) skills(t *testing.T, label, project string, precedence bool) map[string]openCodeProjectSkill {
	t.Helper()
	r := f.command(t, "native", label+"-skill", f.client, project, f.env(), []string{"debug", "skill"})
	if r.Exit != 0 {
		t.Fatal("native skill command failed; see exact evidence")
	}
	if len(bytesTrim(r.Stderr)) != 0 {
		if !precedence {
			t.Fatal("positive skill observation warned")
		}
		f.evidence[label+"_warning"] = string(r.Stderr)
		f.stages["same_name_precedence"] = nativeStage{Status: "blocked", Reason: "unrecognized secondary warning retained verbatim; no warning/precedence qualification"}
	}
	found, err := openCodeProjectSkills(r.Stdout)
	if err != nil {
		t.Fatal(err)
	}
	return found
}
func (f *openCodeProjectFixture) expected(t *testing.T, project, source, command string) map[string]openCodeProjectSkill {
	t.Helper()
	skills := map[string]openCodeProjectSkill{}
	root := filepath.Join(project, ".opencode", "skills")
	if project == f.user {
		root = filepath.Join(f.work, "xdg-config", "opencode", "skills")
	}
	guide, err := os.ReadFile(filepath.Join(source, "skills", commandsGuideSkill, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	content, err := openCodeProjectSkillContent(guide)
	if err != nil {
		t.Fatal(err)
	}
	skills[commandsGuideSkill] = openCodeProjectSkill{Name: commandsGuideSkill, Location: filepath.Join(root, commandsGuideSkill, "SKILL.md"), Content: content}
	content, err = openCodeProjectSkillContent([]byte(commandsSkillBytes[command]))
	if err != nil {
		t.Fatal(err)
	}
	name := commandsSkill(command)
	skills[name] = openCodeProjectSkill{Name: name, Location: filepath.Join(root, name, "SKILL.md"), Content: content}
	skills["uap-user-control"] = openCodeProjectSkill{Name: "uap-user-control", Location: filepath.Join(f.work, "xdg-config", "opencode", "skills", "uap-user-control", "SKILL.md"), Content: "UAP-USER-CONTROL"}
	return skills
}
func (f *openCodeProjectFixture) observe(t *testing.T, label, project, source, marker, command string, removed bool) {
	t.Helper()
	urls := map[string]string{"uap-user-control": f.recorder.url("USER-control")}
	absent := []string{}
	expected := map[string]openCodeProjectSkill{"uap-user-control": {Name: "uap-user-control", Location: filepath.Join(f.work, "xdg-config", "opencode", "skills", "uap-user-control", "SKILL.md"), Content: "UAP-USER-CONTROL"}}
	forbidden := []string{commandsGuideSkill, commandsSkill("echo"), commandsSkill("wizard")}
	if !removed {
		urls["scope-marker"] = f.recorder.url(marker)
		expected = f.expected(t, project, source, command)
		forbidden = []string{commandsSkill(map[string]string{"echo": "wizard", "wizard": "echo"}[command])}
	} else {
		absent = append(absent, "scope-marker")
	}
	if err := openCodeProjectCheckConfig(f.config(t, label, project, nil), urls, absent); err != nil {
		t.Fatal(err)
	}
	if err := openCodeProjectCheckSkills(f.skills(t, label, project, false), expected, forbidden); err != nil {
		t.Fatal(err)
	}
}
func (f *openCodeProjectFixture) snapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	result := map[string]string{}
	total := int64(0)
	entries := 0
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			entries++
			if entries > 1024 || len(result) > 512 {
				return fmt.Errorf("snapshot entry cap")
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !info.Mode().IsRegular() || info.Size() > 1<<20 || total+info.Size() > 4<<20 {
				return fmt.Errorf("snapshot refuses type/size")
			}
			body, err := readProjectNativeSnapshot(path, info, min(int64(1<<20), (4<<20)-total))
			if err != nil {
				return err
			}
			total += int64(len(body))
			sum := sha256.Sum256(body)
			result[path] = fmt.Sprintf("%o:%d:%x", info.Mode(), len(body), sum)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}
func (f *openCodeProjectFixture) finish(t *testing.T) {
	t.Helper()
	f.cancel()
	closed := make(chan error, 1)
	go func() { closed <- f.recorder.close() }()
	select {
	case err := <-closed:
		if err != nil {
			f.failed = true
			t.Error(err)
		}
	case <-time.After(5 * time.Second):
		f.failed = true
		t.Error("passive recorder cleanup exceeded independent deadline")
	}
	if f.recorder.counts() != [2]int64{} {
		f.failed = true
		f.stages["unexpected_mcp_connection"] = nativeStage{Status: "failed", Reason: "passive endpoint observed accepted TCP/HTTP; no handshake qualification"}
		t.Error("late passive recorder connection/request observed")
	}
	f.evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
	f.evidence["success"] = !t.Failed() && !f.failed
	if t.Failed() || f.failed {
		f.evidence["native_discovery_qualified"] = false
	}
	f.evidence["commands"] = f.records
	f.evidence["command_table"] = f.table
	f.evidence["used_command_count"] = len(f.used)
	f.evidence["recorder_counts"] = f.recorder.counts()
	f.recorder.mu.Lock()
	f.evidence["recorder_samples"] = append([]string(nil), f.recorder.samples...)
	f.recorder.mu.Unlock()
	f.evidence["stages"] = f.stages
	f.evidence["cleanup"] = "passive listener closed with independent five-second deadline; working authority retained for disposable runner deletion; no recursive cleanup in fixture"
	body, err := json.MarshalIndent(f.evidence, "", "  ")
	if err != nil || len(body) > 1<<20 {
		f.files = map[string][]byte{}
		body = []byte(`{"success":false,"native_discovery_qualified":false,"error":"summary admission refused"}`)
		f.failed = true
	}
	f.files["opencode-project-evidence.json"] = append(body, '\n')
	names := make([]string, 0, len(f.files))
	size := 0
	for name, body := range f.files {
		names = append(names, name)
		size += len(body)
	}
	sort.Strings(names)
	if len(names) > 128 || size > openCodeProjectEnvelopeLimit {
		f.files = map[string][]byte{"opencode-project-evidence.json": []byte(`{"success":false,"native_discovery_qualified":false,"error":"aggregate envelope refused"}`)}
		names = []string{"opencode-project-evidence.json"}
		t.Error("aggregate project envelope refused")
	}
	pending, err := os.MkdirTemp(f.root, "pending-")
	if err != nil {
		t.Error(err)
		return
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(pending, name), f.files[name], 0600); err != nil {
			t.Error(err)
			return
		}
	}
	// Atomic directory publication leaves working sources/cache/raw trees nested
	// and invisible to the wrapper's flat evidence collector.
	admitted, err := os.MkdirTemp(filepath.Dir(f.root), "uap-opencode-project-admitted-")
	if err != nil {
		t.Error(err)
		return
	}
	if err = os.Remove(admitted); err == nil {
		err = os.Rename(pending, admitted)
	}
	if err != nil {
		t.Error(err)
		return
	}
	t.Logf("OpenCode project admitted envelope: %s", admitted)
}
