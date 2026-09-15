// Package bootstrap installs locked dependencies for exact generated standard templates.
package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Error struct {
	Code  string
	Cause error
}

func (e *Error) Error() string {
	if e.Cause == nil {
		return e.Code
	}
	return e.Code + ": " + e.Cause.Error()
}
func (e *Error) Unwrap() error { return e.Cause }

type capturedFile struct {
	bytes  []byte
	info   fs.FileInfo
	handle *os.File
}

type capturedDirectory struct {
	info   fs.FileInfo
	handle *os.Root
}

type planLease struct {
	once  sync.Once
	root  *os.Root
	files []capturedFile
	dirs  []capturedDirectory
	err   error
}

func (l *planLease) close() error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		for _, file := range l.files {
			l.err = errors.Join(l.err, file.handle.Close())
		}
		for _, dir := range l.dirs {
			l.err = errors.Join(l.err, dir.handle.Close())
		}
		if l.root != nil {
			l.err = errors.Join(l.err, l.root.Close())
		}
	})
	return l.err
}

type Plan struct {
	Runtime  string
	Manager  string
	Command  []string
	rootPath string
	rootInfo fs.FileInfo
	files    map[string]capturedFile
	dirs     map[string]capturedDirectory
	lease    *planLease
}

func (p Plan) Close() error { return p.lease.close() }

// Runner is injectable so tests execute only purpose-built disposable fixtures.
type Runner func(context.Context, string, []string, string, []string) error
type Service struct{ Runner Runner }

var competingPaths = []string{"npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "pyproject.toml", "uv.lock", "poetry.lock", "Pipfile.lock", "go.mod", "go.sum"}

func (s Service) Plan(root string, p project.Result) (Plan, error) {
	fail := func(code string) (Plan, error) { return Plan{}, &Error{Code: code} }
	rootPath, rootInfo, rootHandle, err := captureRoot(root)
	if err != nil {
		return fail("bootstrap_template_unrecognized")
	}
	lease := &planLease{root: rootHandle}
	success := false
	defer func() {
		if !success {
			_ = lease.close()
		}
	}()
	if p.Facts.Package == nil || p.Facts.Package.Manifest.Name == "" {
		return fail("bootstrap_template_unrecognized")
	}
	pkg := p.Facts.Package
	if !pkg.MCP.Present || len(pkg.MCP.Servers) != 1 || len(pkg.MCP.InvalidServer) != 0 {
		return fail("bootstrap_template_unrecognized")
	}
	server, ok := pkg.MCP.Servers[pkg.Manifest.Name]
	if !ok || server.Type != "stdio" || server.Decoded["command"] != "node" {
		return fail("bootstrap_template_unrecognized")
	}
	args, ok := server.Decoded["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "${PLUGIN_ROOT}/src/server.mjs" {
		return fail("bootstrap_template_unrecognized")
	}
	for _, ambiguous := range competingPaths {
		if _, err := os.Lstat(filepath.Join(root, ambiguous)); err == nil || !errors.Is(err, os.ErrNotExist) {
			return fail("bootstrap_layout_ambiguous")
		}
	}
	files := make(map[string]capturedFile, 5)
	dirs := make(map[string]capturedDirectory, 1)
	dirs["src"], err = captureDirectory(root, "src")
	if err != nil {
		return fail("bootstrap_template_unrecognized")
	}
	lease.dirs = append(lease.dirs, dirs["src"])
	for _, name := range []string{"plugin.json", "mcp.json", "package.json", "package-lock.json", "src/server.mjs"} {
		captured, err := captureRegular(root, name)
		if err != nil {
			if name == "package.json" || name == "package-lock.json" {
				return fail("bootstrap_lock_required")
			}
			return fail("bootstrap_template_unrecognized")
		}
		files[name] = captured
		lease.files = append(lease.files, captured)
	}
	if current, err := captureDirectory(root, "src"); err != nil || !os.SameFile(dirs["src"].info, current.info) {
		return fail("bootstrap_source_changed")
	} else {
		_ = current.handle.Close()
	}
	// Bind the decoded standard facts to the exact core bytes captured here.
	if !bytes.Equal(files["plugin.json"].bytes, p.Input.Plugin.Bytes) || !bytes.Equal(files["mcp.json"].bytes, p.Input.MCP.Bytes) {
		return fail("bootstrap_source_changed")
	}
	if !scaffold.MatchesGeneratedNodePackage(pkg.Manifest.Name, files["package.json"].bytes, files["package-lock.json"].bytes) {
		return fail("bootstrap_template_unrecognized")
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fail("bootstrap_destination_exists")
	}
	success = true
	return Plan{Runtime: "node", Manager: "npm", Command: []string{"npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"}, rootPath: rootPath, rootInfo: rootInfo, files: files, dirs: dirs, lease: lease}, nil
}

func captureRoot(root string) (string, fs.FileInfo, *os.Root, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", nil, nil, err
	}
	abs = filepath.Clean(abs)
	info, err := os.Lstat(abs)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, nil, errors.New("root is not a real directory")
	}
	held, err := os.OpenRoot(abs)
	if err != nil {
		return "", nil, nil, err
	}
	opened, openErr := held.Stat(".")
	byName, nameErr := os.Lstat(abs)
	if openErr != nil || nameErr != nil || !os.SameFile(info, opened) || !os.SameFile(info, byName) || byName.Mode()&os.ModeSymlink != 0 {
		_ = held.Close()
		return "", nil, nil, errors.New("root changed while opening")
	}
	return abs, info, held, nil
}

func captureDirectory(root, relative string) (capturedDirectory, error) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	before, err := os.Lstat(path)
	if err != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
		return capturedDirectory{}, errors.New("unavailable real directory")
	}
	held, err := os.OpenRoot(path)
	if err != nil {
		return capturedDirectory{}, err
	}
	opened, err := held.Stat(".")
	byName, nameErr := os.Lstat(path)
	if err != nil || nameErr != nil || !os.SameFile(before, opened) || !os.SameFile(before, byName) || byName.Mode()&os.ModeSymlink != 0 {
		_ = held.Close()
		return capturedDirectory{}, errors.New("directory changed while opening")
	}
	return capturedDirectory{info: before, handle: held}, nil
}

func captureRegular(root, relative string) (capturedFile, error) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	before, err := os.Lstat(path)
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() < 0 || before.Size() > 16<<20 {
		return capturedFile{}, errors.New("unavailable regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return capturedFile{}, err
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		_ = f.Close()
		return capturedFile{}, errors.New("file changed while opening")
	}
	body, err := io.ReadAll(io.LimitReader(f, 16<<20+1))
	if err != nil || len(body) > 16<<20 {
		_ = f.Close()
		return capturedFile{}, errors.New("file read failed or exceeded limit")
	}
	after, statErr := f.Stat()
	byName, nameErr := os.Lstat(path)
	if statErr != nil || nameErr != nil || !os.SameFile(before, after) || !os.SameFile(before, byName) || after.Size() != int64(len(body)) {
		_ = f.Close()
		return capturedFile{}, errors.New("file changed while reading")
	}
	return capturedFile{bytes: append([]byte(nil), body...), info: before, handle: f}, nil
}

func validPlan(plan Plan) bool {
	return plan.Runtime == "node" && plan.Manager == "npm" && len(plan.Command) == 5 && plan.Command[0] == "npm" && plan.Command[1] == "ci" && plan.Command[2] == "--ignore-scripts" && plan.Command[3] == "--no-audit" && plan.Command[4] == "--no-fund" && plan.rootPath != "" && plan.rootInfo != nil && len(plan.files) == 5 && len(plan.dirs) == 1 && plan.lease != nil && plan.lease.root != nil
}

func verifyPlan(root string, held *os.Root, plan Plan) error {
	rootPath, byName, observedRoot, err := captureRoot(root)
	if observedRoot != nil {
		defer observedRoot.Close()
	}
	opened, openErr := held.Stat(".")
	if err != nil || openErr != nil || plan.rootPath != rootPath || !os.SameFile(plan.rootInfo, byName) || !os.SameFile(plan.rootInfo, opened) {
		return &Error{Code: "bootstrap_source_changed", Cause: errors.Join(err, openErr)}
	}
	for name, expected := range plan.dirs {
		current, captureErr := captureDirectory(root, name)
		if current.handle != nil {
			defer current.handle.Close()
		}
		expectedOpen, expectedErr := expected.handle.Stat(".")
		if captureErr != nil || expectedErr != nil || !os.SameFile(expected.info, expectedOpen) || !os.SameFile(expected.info, current.info) {
			return &Error{Code: "bootstrap_source_changed", Cause: captureErr}
		}
	}
	for name, expected := range plan.files {
		current, captureErr := captureRegular(root, name)
		if current.handle != nil {
			defer current.handle.Close()
		}
		expectedOpen, expectedErr := expected.handle.Stat()
		if captureErr != nil || expectedErr != nil || !os.SameFile(expected.info, expectedOpen) || !os.SameFile(expected.info, current.info) || !bytes.Equal(expected.bytes, current.bytes) {
			return &Error{Code: "bootstrap_source_changed", Cause: captureErr}
		}
	}
	for name, expected := range plan.dirs {
		current, captureErr := captureDirectory(root, name)
		if current.handle != nil {
			defer current.handle.Close()
		}
		if captureErr != nil || !os.SameFile(expected.info, current.info) {
			return &Error{Code: "bootstrap_source_changed", Cause: captureErr}
		}
	}
	finalPath, finalInfo, finalRoot, finalErr := captureRoot(root)
	if finalRoot != nil {
		defer finalRoot.Close()
	}
	if finalErr != nil || finalPath != plan.rootPath || !os.SameFile(plan.rootInfo, finalInfo) {
		return &Error{Code: "bootstrap_source_changed", Cause: finalErr}
	}
	for _, name := range append(append([]string(nil), competingPaths...), "node_modules") {
		if _, statErr := held.Lstat(name); statErr == nil || !errors.Is(statErr, os.ErrNotExist) {
			if name == "node_modules" {
				return &Error{Code: "bootstrap_destination_exists", Cause: statErr}
			}
			return &Error{Code: "bootstrap_layout_ambiguous", Cause: statErr}
		}
	}
	return nil
}

func (s Service) Apply(ctx context.Context, root string, plan Plan) (committed bool, err error) {
	if !validPlan(plan) {
		return false, &Error{Code: "bootstrap_plan_invalid"}
	}
	held := plan.lease.root
	defer func() { err = errors.Join(err, plan.Close()) }()
	if err = verifyPlan(root, held, plan); err != nil {
		return false, err
	}
	stageName := fmt.Sprintf(".agentplugins-bootstrap-%016x", rand.Uint64())
	if err = held.Mkdir(stageName, 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
	}
	stageInfo, err := held.Lstat(stageName)
	if err != nil {
		return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
	}
	stage, err := held.OpenRoot(stageName)
	if err != nil {
		return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
	}
	defer func() {
		cleanup := errors.Join(clearOwned(stage), stage.Close())
		if now, statErr := held.Lstat(stageName); statErr == nil && os.SameFile(stageInfo, now) {
			cleanup = errors.Join(cleanup, held.Remove(stageName))
		} else {
			cleanup = errors.Join(cleanup, fmt.Errorf("staging ownership changed: %w", errorOr(statErr, fs.ErrInvalid)))
		}
		if cleanup != nil {
			err = errors.Join(err, &Error{Code: "bootstrap_cleanup_failed", Cause: cleanup})
		}
	}()
	if err = stage.Mkdir("project", 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
	}
	work, err := stage.OpenRoot("project")
	if err != nil {
		return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
	}
	defer func() { err = errors.Join(err, work.Close()) }()
	for _, name := range []string{"package.json", "package-lock.json"} {
		if err = writeFile(work, name, plan.files[name].bytes); err != nil {
			return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
		}
	}
	for _, name := range []string{"home", "cache", "tmp"} {
		if err = stage.Mkdir(name, 0700); err != nil {
			return false, &Error{Code: "bootstrap_stage_failed", Cause: err}
		}
	}
	if err = verifyPlan(root, held, plan); err != nil { // immediately before launch
		return false, err
	}
	stagePath := filepath.Join(root, stageName)
	workPath := filepath.Join(stagePath, "project")
	run := s.Runner
	if run == nil {
		run = runCommand
	}
	env := cleanEnvironment(stagePath, filepath.Join(stagePath, "home"), filepath.Join(stagePath, "cache"), filepath.Join(stagePath, "tmp"))
	if runErr := run(ctx, plan.Command[0], plan.Command[1:], workPath, env); runErr != nil {
		return false, &Error{Code: "bootstrap_process_failed", Cause: errors.Join(runErr, ctx.Err())}
	}
	modulesInfo, statErr := work.Lstat("node_modules")
	if statErr != nil || !modulesInfo.IsDir() || modulesInfo.Mode()&os.ModeSymlink != 0 {
		return false, &Error{Code: "bootstrap_process_failed", Cause: statErr}
	}
	modules, openErr := work.OpenRoot("node_modules")
	if openErr != nil {
		return false, &Error{Code: "bootstrap_process_failed", Cause: openErr}
	}
	defer func() { err = errors.Join(err, modules.Close()) }()
	openedModules, openStatErr := modules.Stat(".")
	if openStatErr != nil || !os.SameFile(modulesInfo, openedModules) {
		return false, &Error{Code: "bootstrap_process_failed", Cause: openStatErr}
	}
	if err = verifyPlan(root, held, plan); err != nil { // immediately before commit
		return false, err
	}
	from, fromErr := work.Open(".")
	to, toErr := held.Open(".")
	if fromErr != nil || toErr != nil {
		if from != nil {
			_ = from.Close()
		}
		if to != nil {
			_ = to.Close()
		}
		return false, &Error{Code: "bootstrap_commit_failed", Cause: errors.Join(fromErr, toErr)}
	}
	renameErr := scaffold.RenameExclusive(from, "node_modules", to, "node_modules")
	closeErr := errors.Join(from.Close(), to.Close())
	if renameErr != nil || closeErr != nil {
		return false, &Error{Code: "bootstrap_commit_failed", Cause: errors.Join(renameErr, closeErr)}
	}
	return true, nil
}

func writeFile(root *os.Root, name string, body []byte) error {
	f, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := f.Write(body)
	return errors.Join(writeErr, f.Close())
}

func clearOwned(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := dir.Readdirnames(64)
	if errors.Is(readErr, io.EOF) {
		readErr = nil
	}
	err = errors.Join(readErr, dir.Close())
	if err != nil {
		return err
	}
	if len(names) >= 64 {
		return errors.New("staging cleanup exceeded entry bound")
	}
	for _, name := range names {
		err = errors.Join(err, root.RemoveAll(name))
	}
	return err
}

func errorOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

func cleanEnvironment(stage, home, cache, tmp string) []string {
	env := []string{"HOME=" + home, "USERPROFILE=" + home, "NPM_CONFIG_CACHE=" + cache,
		"NPM_CONFIG_USERCONFIG=" + filepath.Join(stage, "user-npmrc"), "NPM_CONFIG_GLOBALCONFIG=" + filepath.Join(stage, "global-npmrc"),
		"NPM_CONFIG_PREFIX=" + filepath.Join(stage, "prefix"), "NPM_CONFIG_UPDATE_NOTIFIER=false", "NPM_CONFIG_AUDIT=false", "NPM_CONFIG_FUND=false",
		"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp}
	for _, key := range []string{"PATH", "PATHEXT", "SYSTEMROOT", "WINDIR", "COMSPEC"} {
		if value := os.Getenv(key); value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func runCommand(ctx context.Context, name string, args []string, dir string, env []string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	_, err = (processadapter.OS{}).Run(ctx, ports.Command{Argv: append([]string{path}, args...), Dir: dir, Env: env, StdoutLimitBytes: 64 << 10})
	return err
}
