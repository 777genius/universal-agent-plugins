// Command antigravityinventory supervises one fixed disposable-profile CLI action.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var sourceCommit, sourceTree, nativeSHA string
var requestFields = map[string]bool{"binary": true, "binary_sha256": true, "root": true, "selector": true, "action": true, "source_commit": true, "source_tree": true, "deadline_unix_ns": true}

var safeName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

type request struct {
	Binary    string `json:"binary"`
	BinarySHA string `json:"binary_sha256"`
	Root      string `json:"root"`
	Selector  string `json:"selector"`
	Action    string `json:"action"`
	Commit    string `json:"source_commit"`
	Tree      string `json:"source_tree"`
	Deadline  int64  `json:"deadline_unix_ns"`
}
type runner func(context.Context, ports.Command, time.Duration) (ports.CommandResult, error)

func decode(body []byte) (request, error) {
	var req request
	// A flat string/integer object admits no duplicate keys, unknown fields or aliases.
	dec := json.NewDecoder(strings.NewReader(string(body)))
	token, err := dec.Token()
	if err != nil || token != json.Delim('{') {
		return req, errors.New("request object refused")
	}
	seen := map[string]bool{}
	for dec.More() {
		key, keyErr := dec.Token()
		name, ok := key.(string)
		if keyErr != nil || !ok || seen[name] || !requestFields[name] {
			return req, errors.New("duplicate request field")
		}
		seen[name] = true
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return req, err
		}
	}
	if _, err = dec.Token(); err != nil {
		return req, err
	}
	if _, err = dec.Token(); !errors.Is(err, io.EOF) {
		return req, errors.New("request trailing bytes")
	}
	dec = json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&req); err != nil {
		return req, err
	}
	if len(seen) != 8 {
		return req, errors.New("request field count")
	}
	return req, validateRequest(req)
}

func validateRequest(req request) error {
	if req.Commit != sourceCommit || req.Tree != sourceTree || len(req.Commit) != 40 || len(req.Tree) != 40 ||
		!safeName.MatchString(req.Selector) || req.Selector == "." || req.Selector == ".." ||
		!filepath.IsAbs(req.Binary) || filepath.Clean(req.Binary) != req.Binary || !filepath.IsAbs(req.Root) || filepath.Clean(req.Root) != req.Root ||
		!strings.HasPrefix(filepath.Base(req.Root), "uap-agy-direct-") || len(req.BinarySHA) != 64 || req.BinarySHA != nativeSHA ||
		(req.Action != "list" && req.Action != "enable" && req.Action != "disable" && req.Action != "version") {
		return errors.New("closed request identity refused")
	}
	if _, err := hex.DecodeString(req.BinarySHA); err != nil {
		return err
	}
	return nil
}

func fingerprint(path string, directory bool) (*os.File, os.FileInfo, string, error) {
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || before.IsDir() != directory || (!directory && !before.Mode().IsRegular()) {
		return nil, nil, "", errors.New("held object refused")
	}
	held, err := os.Open(path)
	if err != nil {
		return nil, nil, "", err
	}
	info, err := held.Stat()
	if err != nil || !os.SameFile(before, info) || info.Mode() != before.Mode() {
		_ = held.Close()
		return nil, nil, "", errors.New("held identity differs")
	}
	digest := ""
	if !directory {
		if info.Size() <= 0 || info.Size() > 512<<20 {
			_ = held.Close()
			return nil, nil, "", errors.New("binary bound")
		}
		hash := sha256.New()
		n, readErr := io.Copy(hash, io.LimitReader(held, (512<<20)+1))
		if readErr != nil || n != info.Size() {
			_ = held.Close()
			return nil, nil, "", errors.New("binary capture refused")
		}
		digest = hex.EncodeToString(hash.Sum(nil))
	}
	return held, info, digest, nil
}

func verify(path string, held *os.File, original os.FileInfo) error {
	linked, err := os.Lstat(path)
	if err != nil {
		return err
	}
	current, err := held.Stat()
	if err != nil || !os.SameFile(linked, original) || !os.SameFile(current, original) || linked.Mode() != original.Mode() ||
		(!original.IsDir() && (linked.Size() != original.Size() || !linked.ModTime().Equal(original.ModTime()))) {
		return errors.New("held authority changed")
	}
	return nil
}

func closeCapture(file *os.File, body []byte, resultErr error) ([]byte, error) {
	resultErr = errors.Join(resultErr, file.Close())
	if resultErr != nil {
		body = nil
	}
	return body, resultErr
}

func fixedArgv(req request) []string {
	argv := []string{req.Binary, "plugin", req.Action, req.Selector}
	if req.Action == "list" {
		argv = []string{req.Binary, "plugin", "list", "--json"}
	}
	if req.Action == "version" {
		argv = []string{req.Binary, "--version"}
	}
	return argv
}

func execute(ctx context.Context, req request, run runner) (body []byte, resultErr error) {
	binary, binaryInfo, digest, err := fingerprint(req.Binary, false)
	if err != nil {
		return nil, err
	}
	defer func() { body, resultErr = closeCapture(binary, body, resultErr) }()
	root, rootInfo, _, err := fingerprint(req.Root, true)
	if err != nil {
		return nil, err
	}
	defer func() { body, resultErr = closeCapture(root, body, resultErr) }()
	if digest != req.BinarySHA {
		return nil, errors.New("binary pin differs")
	}
	argv := fixedArgv(req)
	home := filepath.Join(req.Root, "home")
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "APPDATA=" + filepath.Join(home, "AppData", "Roaming"), "LOCALAPPDATA=" + filepath.Join(home, "AppData", "Local"),
		"XDG_CONFIG_HOME=" + filepath.Join(req.Root, "config"), "XDG_CACHE_HOME=" + filepath.Join(req.Root, "cache"), "TMP=" + filepath.Join(req.Root, "tmp"), "TEMP=" + filepath.Join(req.Root, "tmp"), "TMPDIR=" + filepath.Join(req.Root, "tmp")}
	for _, key := range []string{"PATH", "SystemRoot", "LANG"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	result, err := run(ctx, ports.Command{Argv: argv, Env: env, Dir: filepath.Join(req.Root, "project"), StdoutLimitBytes: 1 << 20}, 5*time.Second)
	if err != nil || ctx.Err() != nil || !result.CleanupComplete || result.NoChildStarted || result.ExitCode != 0 || len(result.Stdout) >= 1<<20 || len(result.Stderr) != 0 {
		return nil, errors.New("command capture or cleanup refused")
	}
	if err = verify(req.Binary, binary, binaryInfo); err != nil {
		return nil, err
	}
	if err = verify(req.Root, root, rootInfo); err != nil {
		return nil, err
	}
	// Hash the held executable again after descendants settle; no pathname reopening.
	if _, err = binary.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, io.LimitReader(binary, (512<<20)+1)); err != nil || hex.EncodeToString(hash.Sum(nil)) != digest {
		return nil, errors.New("binary runtime pin changed")
	}
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	outputHash := sha256.Sum256(result.Stdout)
	return json.Marshal(map[string]any{"source_commit": req.Commit, "source_tree": req.Tree, "root": req.Root, "selector": req.Selector, "action": req.Action,
		"binary_sha256": digest, "cleanup_complete": true, "stdout_base64": base64.StdEncoding.EncodeToString(result.Stdout), "stdout_bytes": len(result.Stdout), "stdout_sha256": hex.EncodeToString(outputHash[:])})
}

func run(control io.Reader, output io.Writer, nativeRun runner) error {
	reader := bufio.NewReaderSize(control, 16385)
	body, err := reader.ReadSlice('\n')
	if err != nil || len(body) > 16384 {
		return errors.New("request frame bound")
	}
	req, err := decode(body)
	if err != nil {
		return err
	}
	remaining := time.Until(time.Unix(0, req.Deadline))
	if remaining <= 0 || remaining > 30*time.Second {
		return errors.New("original deadline refused")
	}
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, remaining)
	defer cancel()
	go func() { var extra [1]byte; _, _ = reader.Read(extra[:]); cancel() }()
	result, err := execute(ctx, req, nativeRun)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(result))
	return err
}
func main() {
	if err := run(os.Stdin, os.Stdout, (process.OS{}).RunWithDescendantExitGrace); err != nil {
		fmt.Fprintln(os.Stderr, "antigravity command refused")
		os.Exit(1)
	}
}
