// Package bootstrap installs locked dependencies for exact generated standard templates.
package bootstrap

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/scaffold"
)

type Error struct{ Code string }

func (e *Error) Error() string { return e.Code }

type Plan struct {
	Runtime string
	Manager string
	Command []string
}

// Runner is injectable so tests execute only purpose-built disposable fixtures.
type Runner func(context.Context, string, []string, string, []string) error

type Service struct{ Runner Runner }

func (s Service) Plan(root string, p project.Result) (Plan, error) {
	fail := func(code string) (Plan, error) { return Plan{}, &Error{Code: code} }
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
	for _, ambiguous := range []string{"npm-shrinkwrap.json", "pnpm-lock.yaml", "yarn.lock", "bun.lock", "bun.lockb", "pyproject.toml", "uv.lock", "poetry.lock", "Pipfile.lock", "go.mod", "go.sum"} {
		if _, err := os.Lstat(filepath.Join(root, ambiguous)); err == nil || !errors.Is(err, os.ErrNotExist) {
			return fail("bootstrap_layout_ambiguous")
		}
	}
	packageJSON, err := readRegular(root, "package.json")
	if err != nil {
		return fail("bootstrap_lock_required")
	}
	packageLock, err := readRegular(root, "package-lock.json")
	if err != nil {
		return fail("bootstrap_lock_required")
	}
	if !scaffold.MatchesGeneratedNodePackage(pkg.Manifest.Name, packageJSON, packageLock) {
		return fail("bootstrap_template_unrecognized")
	}
	if _, err := readRegular(root, "src/server.mjs"); err != nil {
		return fail("bootstrap_template_unrecognized")
	}
	if _, err := os.Lstat(filepath.Join(root, "node_modules")); err == nil || !errors.Is(err, os.ErrNotExist) {
		return fail("bootstrap_destination_exists")
	}
	return Plan{Runtime: "node", Manager: "npm", Command: []string{"npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund"}}, nil
}

func readRegular(root, relative string) ([]byte, error) {
	path := filepath.Join(root, filepath.FromSlash(relative))
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16<<20 {
		return nil, errors.New("unavailable regular file")
	}
	return os.ReadFile(path)
}

func (s Service) Apply(ctx context.Context, root string, plan Plan) (committed bool, err error) {
	if plan.Runtime != "node" || plan.Manager != "npm" || len(plan.Command) != 5 ||
		plan.Command[0] != "npm" || plan.Command[1] != "ci" || plan.Command[2] != "--ignore-scripts" ||
		plan.Command[3] != "--no-audit" || plan.Command[4] != "--no-fund" {
		return false, &Error{Code: "bootstrap_plan_invalid"}
	}
	stage, err := os.MkdirTemp(root, ".agentplugins-bootstrap-")
	if err != nil {
		return false, &Error{Code: "bootstrap_stage_failed"}
	}
	defer func() {
		if cleanup := os.RemoveAll(stage); cleanup != nil && err == nil {
			err = &Error{Code: "bootstrap_cleanup_failed"}
		}
	}()
	work := filepath.Join(stage, "project")
	if err = os.Mkdir(work, 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed"}
	}
	for _, name := range []string{"package.json", "package-lock.json"} {
		if err = copyFile(filepath.Join(root, name), filepath.Join(work, name)); err != nil {
			return false, &Error{Code: "bootstrap_stage_failed"}
		}
	}
	home := filepath.Join(stage, "home")
	cache := filepath.Join(stage, "cache")
	tmp := filepath.Join(stage, "tmp")
	if err = os.Mkdir(home, 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed"}
	}
	if err = os.Mkdir(cache, 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed"}
	}
	if err = os.Mkdir(tmp, 0700); err != nil {
		return false, &Error{Code: "bootstrap_stage_failed"}
	}
	run := s.Runner
	if run == nil {
		run = runCommand
	}
	env := cleanEnvironment(stage, home, cache, tmp)
	if err = run(ctx, plan.Command[0], plan.Command[1:], work, env); err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		return false, &Error{Code: "bootstrap_process_failed"}
	}
	modules := filepath.Join(work, "node_modules")
	if info, statErr := os.Lstat(modules); statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, &Error{Code: "bootstrap_process_failed"}
	}
	if err = os.Rename(modules, filepath.Join(root, "node_modules")); err != nil {
		return false, &Error{Code: "bootstrap_commit_failed"}
	}
	return true, nil
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	return errors.Join(copyErr, closeErr)
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
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	return cmd.Run()
}
