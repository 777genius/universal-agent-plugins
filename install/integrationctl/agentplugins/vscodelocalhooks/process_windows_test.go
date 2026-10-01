package vscodelocalhooks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	hooks "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

type windowsRecording struct {
	Args        []string
	CWD         string
	Home        string
	UserProfile string
}

// Independent native .exe receiver: observes the actual process argv and writes
// UTF-8 itself, outside PowerShell's legacy stdout transcoding. No executor mock.
func TestTESTWindowsArgvRecorder(t *testing.T) {
	if os.Getenv("U2_TEST_WINDOWS_RECORDER") != "1" {
		return
	}
	cwd, err := os.Getwd()
	if err != nil {
		os.Exit(3)
	}
	got := windowsRecording{Args: os.Args[3:], CWD: cwd, Home: os.Getenv("HOME"), UserProfile: os.Getenv("USERPROFILE")}
	data, err := json.Marshal(got)
	if err != nil {
		os.Exit(4)
	}
	// O_EXCL makes a repeated/unexpected process effect observable.
	file, err := os.OpenFile(os.Getenv("U2_TEST_RECORD"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		os.Exit(5)
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		os.Exit(6)
	}
	if _, err := os.Stdout.WriteString("TEST-native-exe-argv-recorded\n"); err != nil {
		os.Exit(7)
	}
	os.Exit(0)
}

// Red: expansion, injection, Unicode/argv corruption, wrong platform field,
// substituted pwsh7 or missing PS5.1. Required native execution never skips.
func TestTESTWindowsPowerShell51Process(t *testing.T) {
	requireNativeHost(t)
	root := t.TempDir()
	home, profile := filepath.Join(root, "TEST-HOME"), filepath.Join(root, "TEST-USERPROFILE")
	for _, dir := range []string{home, profile} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	// SystemRoot is the sole ambient host fact admitted into the fresh env.
	systemRoot := os.Getenv("SystemRoot")
	target := hooks.Target{Shell: hooks.WindowsPowerShell51, SystemRoot: systemRoot, ComSpec: filepath.Join(systemRoot, "System32", "cmd.exe")}
	ps := filepath.Join(systemRoot, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	recorder := filepath.Join(root, "TEST ü 中文 recorder's $cash `ticks` %TEST_LITERAL% & [brackets].exe")
	copyTestExecutable(t, recorder)
	record, sentinel := filepath.Join(root, "TEST-args.json"), filepath.Join(root, "TEST-INJECTION-MUST-NOT-EXIST")
	env := []string{
		"SystemRoot=" + systemRoot, "ComSpec=" + target.ComSpec, "HOME=" + home, "USERPROFILE=" + profile,
		"APPDATA=" + home, "LOCALAPPDATA=" + profile, "TEMP=" + root, "TMP=" + root, "PATH=C:\\TEST-no-PATH",
		"POWERSHELL_UPDATECHECK=Off", "U2_TEST_WINDOWS_RECORDER=1", "U2_TEST_RECORD=" + record,
		"TEST_LITERAL=TEST-expansion-would-be-a-defect",
	}
	// The same exact system executable is probed separately so the public rendered
	// invocation remains completely unchanged. No wrapper prefix/suffix or encoding
	// overrides are added to the vendor argv.
	if _, err := hooks.RenderArgv(target, recorder, nil); err != nil {
		t.Fatal("required trusted system snapshot invalid", err)
	}
	version := probeWindowsPowerShell(t, ps, home, env)
	values := []string{
		"spaces in literal", "apostrophe's value", "unicode-ü-中文-😀", "NFC-é", "NFD-e\u0301",
		"$TEST_LITERAL", "${HOME}", "back`tick", "%TEST_LITERAL%", "a&b", "[a-z]*?", ";semicolon",
		`back\slash`, "--looks-like-a-flag", home,
		"$(New-Item -ItemType File -Path '" + sentinel + "')",
		"'; New-Item -ItemType File -Path '" + sentinel + "'; '",
		"`$env:HOME", "!TEST_LITERAL!",
	}
	args := append([]string{"-test.run=^TestTESTWindowsArgvRecorder$", "--"}, values...)
	spec := hooks.Spec{Event: hooks.Stop, Executable: recorder, Args: args, TimeoutSeconds: 5}
	body, err := hooks.Render(target, []hooks.Spec{spec})
	if err != nil {
		t.Fatal(err)
	}
	command := nativePlatformCommand(t, body, "windows")
	if err := hooks.VerifyOwned(body, target, []hooks.Spec{spec}); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runWindowsPowerShell(t, ps, command, home, env)
	if code != 0 || len(stderr) != 0 || !bytes.Equal(bytes.TrimSpace(stdout), []byte("TEST-native-exe-argv-recorded")) {
		t.Fatalf("native process failed: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertWindowsRecording(t, record, home, profile, values, version)
	assertWindowsFileAbsent(t, sentinel)
	t.Logf("TEST rendered native file: %s", body)
	testWindowsRefusalsBeforeProcess(t, target, recorder, root, ps, home, env)
	testWindowsSmartQuotes(t, target, recorder, root, ps, home, env)
}

func assertWindowsRecording(t *testing.T, record, home, profile string, values []string, version []byte) {
	t.Helper()
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal("native .exe did not record argv", err)
	}
	if !utf8.Valid(data) {
		t.Fatal("native recorder file is not UTF-8", data)
	}
	var got windowsRecording
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := windowsRecording{Args: values, CWD: home, Home: home, UserProfile: profile}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("native argv/env/cwd changed: got %#v want %#v", got, want)
	}
	t.Logf("TEST native record encoding=UTF-8 args/env/cwd=%s version=%s", data, version)
}

// Independent runtime facts, from the actual system process, never inferred
// from its filename or from renderer implementation strings.
func probeWindowsPowerShell(t *testing.T, ps, home string, env []string) []byte {
	t.Helper()
	phaseFile := filepath.Join(t.TempDir(), "TEST-ps51-phases.txt")
	// Only this separate metadata probe has markers. Each .NET append is
	// synchronous, finite and terminating on failure; nothing enters stdout.
	command := strings.Join([]string{
		`$ErrorActionPreference='Stop'; $clock=[System.Diagnostics.Stopwatch]::StartNew()`,
		`$phaseFile='` + strings.ReplaceAll(phaseFile, "'", "''") + `'`,
		`function Mark([string]$phase) { [System.IO.File]::AppendAllText($phaseFile, ($phase+' elapsedMs='+$clock.ElapsedMilliseconds+' pid='+$PID+[Environment]::NewLine), [System.Text.UTF8Encoding]::new($false)) }`,
		`Mark 'script-entry'`,
		`$metadata=[ordered]@{Major=$PSVersionTable.PSVersion.Major;Minor=$PSVersionTable.PSVersion.Minor;Edition=$PSVersionTable.PSEdition}`,
		`Mark 'version-edition'`,
		`Mark 'get-process-before'; $metadata['ProcessPath']=(Get-Process -Id $PID).Path; Mark 'get-process-after'`,
		`$metadata['ConsoleCodePage']=[Console]::OutputEncoding.CodePage; $metadata['NativeInputCodePage']=$OutputEncoding.CodePage; Mark 'encodings'`,
		`Mark 'convert-json-before'; $json=$metadata | ConvertTo-Json -Compress; Mark 'convert-json-after'; $json`,
	}, "; ")
	stdout, stderr, code := runObservedWindowsPowerShell(t, ps, command, home, env, phaseFile)
	var got struct {
		Major, Minor                         int
		Edition, ProcessPath                 string
		ConsoleCodePage, NativeInputCodePage int
	}
	if code != 0 || len(stderr) != 0 {
		t.Fatalf("PS5.1 version probe failed: code=%d stderr=%q", code, stderr)
	}
	if err := json.Unmarshal(bytes.TrimPrefix(stdout, []byte{0xef, 0xbb, 0xbf}), &got); err != nil {
		t.Fatal(err)
	}
	if got.Major != 5 || got.Minor != 1 || got.Edition != "Desktop" || !equalWindowsPath(got.ProcessPath, ps) {
		t.Fatalf("actual system Windows PowerShell 5.1 required: %s", stdout)
	}
	return stdout
}

func equalWindowsPath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// Exactly hookExecutor.ts's explicit spawn branch: no cmd intermediate and no
// pwsh search/fallback. A deadline bounds execution; WaitDelay bounds open pipes.
func runWindowsPowerShell(t *testing.T, ps, command, home string, env []string) ([]byte, []byte, int) {
	t.Helper()
	return runObservedWindowsPowerShell(t, ps, command, home, env, "")
}

func runObservedWindowsPowerShell(t *testing.T, ps, command, home string, env []string, phaseFile string) ([]byte, []byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, ps, "-ExecutionPolicy", "Bypass", "-NoProfile", "-NoLogo", "-Command", command)
	child.Dir, child.Env, child.WaitDelay = home, env, time.Second
	runStarted := time.Now()
	// On timeout, terminate only this owned fixture process tree before waiting.
	// This is test cleanup, not a claim about the vendor's timeout semantics.
	child.Cancel = func() error {
		cleanupStarted := time.Now()
		t.Logf("TEST PS cancel entry pid=%d elapsed=%s context=%v", child.Process.Pid, time.Since(runStarted), ctx.Err())
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cleanupCancel()
		taskkill := filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(ps))), "taskkill.exe")
		cleanup := exec.CommandContext(cleanupCtx, taskkill, "/PID", strconv.Itoa(child.Process.Pid), "/T", "/F")
		cleanup.Dir, cleanup.Env, cleanup.WaitDelay = home, env, time.Second
		output, cleanupErr := cleanup.CombinedOutput()
		t.Logf("TEST owned taskkill complete pid=%d elapsed=%s error=%v output=%q", child.Process.Pid, time.Since(cleanupStarted), cleanupErr, output)
		if cleanupErr != nil {
			t.Logf("TEST tree cleanup failed: %v output=%q", cleanupErr, output)
			return child.Process.Kill()
		}
		return nil
	}
	var stdout, stderr bytes.Buffer
	child.Stdout, child.Stderr = &stdout, &stderr
	started := time.Now()
	t.Log("TEST PS Start begin")
	err := child.Start()
	t.Logf("TEST PS Start end elapsed=%s error=%v", time.Since(started), err)
	if err == nil {
		waiting := time.Now()
		t.Logf("TEST PS Wait begin pid=%d", child.Process.Pid)
		err = child.Wait()
		t.Logf("TEST PS Wait end pid=%d elapsed=%s error=%v context=%v", child.Process.Pid, time.Since(waiting), err, ctx.Err())
	}
	code := -1
	if child.ProcessState != nil {
		code = child.ProcessState.ExitCode()
	}
	t.Logf("TEST system PS argv=%q code=%d stdoutBytes=%x stderrBytes=%x", child.Args, code, stdout.Bytes(), stderr.Bytes())
	if phaseFile != "" {
		logWindowsProbePhases(t, phaseFile)
	}
	if ctx.Err() != nil {
		t.Fatal("native PowerShell deadline", ctx.Err())
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatal("required system PS5.1 unavailable or pipes not closed", err)
	}
	return stdout.Bytes(), stderr.Bytes(), code
}

func logWindowsProbePhases(t *testing.T, phaseFile string) {
	t.Helper()
	// The script can append only seven fixed records. Read only its own fresh
	// file, after Wait/owned cleanup and before any timeout fatal or TempDir cleanup.
	file, err := os.Open(phaseFile)
	if err != nil {
		t.Logf("TEST PS final phases unavailable: %v", err)
		t.Error("required metadata phase evidence unavailable")
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			t.Error("close metadata phase evidence", err)
		}
	}()
	var data [1024]byte
	n, err := file.Read(data[:])
	t.Logf("TEST PS final phases bytes=%d readError=%v content=%q", n, err, data[:n])
	if err != nil || n == len(data) {
		t.Error("metadata phase evidence unreadable or exceeds bound", err)
	}
}

func testWindowsRefusalsBeforeProcess(t *testing.T, target hooks.Target, recorder, root, ps, home string, env []string) {
	t.Helper()
	for i, value := range []string{"", `double"quote`, `trailing\`, "tab\t", "control\x01", "delete\x7f", "nul\x00", "line\n", "return\r"} {
		t.Run("refused-"+string(rune('A'+i)), func(t *testing.T) {
			record := filepath.Join(root, "TEST-refused-"+string(rune('A'+i))+".json")
			caseEnv := append(append([]string{}, env...), "U2_TEST_RECORD="+record)
			args := []string{"-test.run=^TestTESTWindowsArgvRecorder$", "--", value}
			command, argvErr := hooks.RenderArgv(target, recorder, args)
			body, renderErr := hooks.Render(target, []hooks.Spec{{Event: hooks.Stop, Executable: recorder, Args: args, TimeoutSeconds: 5}})
			// If validation regresses, actually execute the returned artifact in the
			// isolated TEST fixture, making unexpected process effects observable.
			if len(body) != 0 {
				runWindowsPowerShell(t, ps, nativePlatformCommand(t, body, "windows"), home, caseEnv)
			}
			wantErr := hooks.ErrUnsupported
			if value == "nul\x00" || value == "line\n" || value == "return\r" {
				wantErr = hooks.ErrInvalid
			}
			if command != "" || len(body) != 0 || !errors.Is(argvErr, wantErr) || !errors.Is(renderErr, wantErr) {
				t.Errorf("refused form produced command/artifact or wrong errors: argv=%v render=%v", argvErr, renderErr)
			}
			assertWindowsFileAbsent(t, record)
		})
	}
}

func assertWindowsFileAbsent(t *testing.T, name string) {
	t.Helper()
	if _, err := os.Lstat(name); !os.IsNotExist(err) {
		t.Fatal("unexpected TEST process effect or unverifiable absence", name, err)
	}
}

// Frozen pre-fix commands are negative controls, never a supported renderer.
// Actual PS5.1 must expose a sentinel effect or argv/path failure; logs retain
// the native observation. The repaired public API must refuse with zero effects.
func testWindowsSmartQuotes(t *testing.T, target hooks.Target, recorder, root, ps, home string, env []string) {
	t.Helper()
	for _, quote := range []rune{'\u2018', '\u2019', '\u201a', '\u201b'} {
		t.Run("smart-"+strconv.FormatInt(int64(quote), 16), func(t *testing.T) {
			sentinel := filepath.Join(root, "TEST-smart-"+strconv.FormatInt(int64(quote), 16)+"-MUST-NOT-EXIST")
			value := "safe" + string(quote) + "; New-Item -ItemType File -Path '" + strings.ReplaceAll(sentinel, "'", "''") + "'; #"
			path := filepath.Join(root, "TEST-reader"+string(quote)+"s.exe")
			copyTestExecutable(t, path)
			for i, spec := range []hooks.Spec{
				{Event: hooks.Stop, Executable: recorder, Args: []string{"-test.run=^TestTESTWindowsArgvRecorder$", "--", value}, TimeoutSeconds: 5},
				{Event: hooks.Stop, Executable: path, Args: []string{"-test.run=^TestTESTWindowsArgvRecorder$", "--"}, TimeoutSeconds: 5},
			} {
				controlRecord := sentinel + "-control-" + strconv.Itoa(i) + ".json"
				controlEnv := append(append([]string{}, env...), "U2_TEST_RECORD="+controlRecord)
				parts := make([]string, 1, 1+len(spec.Args))
				parts[0] = spec.Executable
				parts = append(parts, spec.Args...)
				for j := range parts {
					parts[j] = "'" + strings.ReplaceAll(parts[j], "'", "''") + "'"
				}
				_, _, code := runWindowsPowerShell(t, ps, "& "+strings.Join(parts, " "), home, controlEnv)
				assertWindowsSmartControl(t, controlRecord, sentinel, spec.Args[2:], code)
				if err := os.Remove(sentinel); err != nil && !os.IsNotExist(err) {
					t.Fatal(err)
				}
				assertWindowsSmartRefusal(t, target, spec, sentinel, ps, home, env)
			}
		})
	}
}

func assertWindowsSmartControl(t *testing.T, record, sentinel string, wantArgs []string, code int) {
	t.Helper()
	data, err := os.ReadFile(record)
	var got windowsRecording
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err == nil {
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
	}
	_, sentinelErr := os.Lstat(sentinel)
	if sentinelErr != nil && !os.IsNotExist(sentinelErr) {
		t.Fatal(sentinelErr)
	}
	if code == 0 && err == nil && reflect.DeepEqual(got.Args, wantArgs) && os.IsNotExist(sentinelErr) {
		t.Fatal("pre-fix native control did not expose smart-quote failure")
	}
	t.Logf("TEST pre-fix native control code=%d record=%s recordError=%v sentinelExists=%t", code, data, err, sentinelErr == nil)
}

func assertWindowsSmartRefusal(t *testing.T, target hooks.Target, spec hooks.Spec, sentinel, ps, home string, env []string) {
	t.Helper()
	record := sentinel + "-refused.json"
	caseEnv := append(append([]string{}, env...), "U2_TEST_RECORD="+record)
	command, argvErr := hooks.RenderArgv(target, spec.Executable, spec.Args)
	body, renderErr := hooks.Render(target, []hooks.Spec{spec})
	verifyErr := hooks.VerifyOwned([]byte(authored), target, []hooks.Spec{spec})
	// A refusal regression runs the artifact so actual receiver/sentinel effects
	// cannot be hidden by checking only the returned error.
	if len(body) != 0 {
		runWindowsPowerShell(t, ps, nativePlatformCommand(t, body, "windows"), home, caseEnv)
	}
	if command != "" || len(body) != 0 {
		t.Error("smart quote produced executable output")
	}
	for _, err := range []error{argvErr, renderErr, verifyErr} {
		if !errors.Is(err, hooks.ErrUnsupported) {
			t.Errorf("smart quote accepted or wrong classification: %v", err)
		} else if strings.Contains(err.Error(), "TEST-") || strings.Contains(err.Error(), "reader") {
			t.Error("refusal leaked supplied literal")
		}
	}
	assertWindowsFileAbsent(t, record)
	assertWindowsFileAbsent(t, sentinel)
}
