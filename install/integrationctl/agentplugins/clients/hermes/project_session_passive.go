package hermes

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
)

// A session environment is private and transient. An explicitly empty base
// must never acquire ambient values; each public adapter call owns its copy.
func sessionEnvironment(base []string) []string {
	if base == nil {
		base = os.Environ()
	}
	result := make([]string, len(base))
	copy(result, base)
	return result
}

// An explicit Linux inspection uses the ordinary OS temporary namespace only
// when the caller supplied no temporary or scratch selector. This allocates
// nothing and grants no ownership or cleanup authority over that namespace.
func sessionEnvironmentOverrides(project string, inspect bool, base []string) map[string]string {
	result := map[string]string{"HERMES_HOME": projectSessionRoot(project)}
	if !inspect {
		return result
	}
	// Explicit inspection must not populate caller-owned Python bytecode caches.
	result["PYTHONDONTWRITEBYTECODE"] = "1"
	if runtime.GOOS != "linux" {
		return result
	}
	for _, item := range base {
		key, _, _ := strings.Cut(item, "=")
		switch key {
		case "TMPDIR", "TMP", "TEMP", "HERMES_SCRATCH_DIR":
			return result
		}
	}
	result["TMPDIR"] = "/tmp"
	return result
}

// Derive the exact child selection from the original base. Caller selections,
// including explicit blanks and duplicates, never acquire a fallback override.
func sessionPlanEnvironment(plan clients.SessionPlan, base []string) ([]string, error) {
	inspect := len(plan.Arguments) != 0
	if !maps.Equal(plan.Environment, sessionEnvironmentOverrides(plan.Directory, inspect, base)) {
		return nil, fmt.Errorf("hermes session overrides differ")
	}
	result := plan.ChildEnvironment(base)
	if inspect {
		if _, err := sessionPassiveEnvironment(result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Keep both the named parent and the selected directory open. Terminal-dot
// acquisition refuses a FIFO substituted between Lstat and OpenRoot.
// filepath.Join deliberately cannot construct this suffix: it removes the dot.
const sessionDirectoryTerminal = string(os.PathSeparator) + "."

type sessionHeldDirectory struct {
	path, identity, parentIdentity string
	root, parent                   *os.Root
	file, parentFile               *os.File
	info, parentInfo               os.FileInfo
}

func openSessionDirectory(path string) (result *sessionHeldDirectory, resultErr error) {
	directory := &sessionHeldDirectory{path: path}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, directory.close())
		}
	}()
	var err error
	directory.parentInfo, err = noAliasDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	directory.info, err = noAliasDirectory(path)
	if err != nil {
		return nil, err
	}
	directory.parent, err = os.OpenRoot(filepath.Dir(path) + sessionDirectoryTerminal)
	if err != nil {
		return nil, err
	}
	directory.parentFile, err = directory.parent.Open(".")
	if err != nil {
		return nil, err
	}
	directory.parentIdentity, err = sessionDirectoryIdentity(directory.parentFile, directory.parentInfo)
	if err != nil {
		return nil, err
	}
	if err := directory.openChild(); err != nil {
		return nil, err
	}
	return directory, nil
}

func (directory *sessionHeldDirectory) openChild() error {
	var err error
	directory.root, err = directory.parent.OpenRoot(filepath.Base(directory.path) + sessionDirectoryTerminal)
	if err != nil {
		return err
	}
	directory.file, err = directory.root.Open(".")
	if err != nil {
		return err
	}
	directory.identity, err = sessionDirectoryIdentity(directory.file, directory.info)
	if err != nil {
		return err
	}
	return directory.rejoin()
}

func (directory *sessionHeldDirectory) child(name string) (result *sessionHeldDirectory, resultErr error) {
	if err := directory.rejoin(); err != nil {
		return nil, err
	}
	child := &sessionHeldDirectory{path: filepath.Join(directory.path, name), parentInfo: directory.info, parentIdentity: directory.identity}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, child.close())
		}
	}()
	var err error
	child.info, err = directory.root.Lstat(name)
	if err != nil || !child.info.IsDir() || child.info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("hermes retained child directory is not ordinary")
	}
	child.parent, err = directory.root.OpenRoot(".")
	if err != nil {
		return nil, err
	}
	child.parentFile, err = child.parent.Open(".")
	if err != nil {
		return nil, err
	}
	if err := child.openChild(); err != nil {
		return nil, err
	}
	return child, nil
}

func sessionDirectoryIdentity(file *os.File, before os.FileInfo) (string, error) {
	held, err := file.Stat()
	if err != nil || !held.IsDir() || !os.SameFile(before, held) || before.Mode() != held.Mode() {
		return "", fmt.Errorf("hermes retained directory changed")
	}
	return sessionPhysicalIdentity(file, held)
}

func (directory *sessionHeldDirectory) rejoin() error {
	for _, member := range []struct {
		path, identity string
		file           *os.File
		info           os.FileInfo
	}{
		{filepath.Dir(directory.path), directory.parentIdentity, directory.parentFile, directory.parentInfo},
		{directory.path, directory.identity, directory.file, directory.info},
	} {
		named, err := noAliasDirectory(member.path)
		if err != nil || !os.SameFile(member.info, named) || member.info.Mode() != named.Mode() {
			return fmt.Errorf("hermes original directory or parent changed")
		}
		identity, err := sessionDirectoryIdentity(member.file, named)
		if err != nil || identity != member.identity {
			return fmt.Errorf("hermes original directory handle changed")
		}
	}
	named, err := directory.parent.Lstat(filepath.Base(directory.path))
	if err != nil || !os.SameFile(directory.info, named) || directory.info.Mode() != named.Mode() {
		return fmt.Errorf("hermes retained parent no longer names selected directory")
	}
	return nil
}

func (directory *sessionHeldDirectory) close() error {
	var result error
	for _, file := range []*os.File{directory.file, directory.parentFile} {
		if file != nil {
			result = errors.Join(result, file.Close())
		}
	}
	for _, root := range []*os.Root{directory.root, directory.parent} {
		if root != nil {
			result = errors.Join(result, root.Close())
		}
	}
	return result
}

type sessionMissingMember struct {
	directory *sessionHeldDirectory
	name      string
}

type sessionPassiveState struct {
	directories []*sessionHeldDirectory
	missing     []sessionMissingMember
	selectors   []string
	install     string
}

func (state *sessionPassiveState) close() error {
	var result error
	for _, directory := range state.directories {
		result = errors.Join(result, directory.close())
	}
	return result
}

func (state *sessionPassiveState) rejoin(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, directory := range state.directories {
		if err := directory.rejoin(); err != nil {
			return err
		}
	}
	for _, missing := range state.missing {
		if _, err := missing.directory.root.Lstat(missing.name); !os.IsNotExist(err) {
			return fmt.Errorf("hermes passive startup absence changed")
		}
	}
	for _, directory := range state.directories {
		if err := directory.rejoin(); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func captureSessionPassive(ctx context.Context, bundle hermesBundle, environment []string) (result *sessionPassiveState, resultErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	state := &sessionPassiveState{}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, state.close())
		}
	}()
	values, err := sessionPassiveEnvironment(environment)
	if err != nil {
		return nil, err
	}
	code := filepath.Join(bundle.root, "hermes-agent")
	for _, character := range code {
		if character > 127 {
			return nil, fmt.Errorf("hermes passive install identity requires an ASCII code path")
		}
	}
	state.install = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.ToLower(code))))[:12]
	directory, err := openSessionDirectory(code)
	if err != nil {
		return nil, err
	}
	state.directories = append(state.directories, directory)
	if !os.SameFile(bundle.originalRoot, directory.parentInfo) {
		return nil, fmt.Errorf("hermes original selected bundle root changed")
	}
	for _, name := range []string{".git", ".env", "install-stamp.json", ".hermes-update-zip-swap"} {
		state.missing = append(state.missing, sessionMissingMember{directory, name})
	}
	for _, key := range sessionPassiveKeys() {
		state.selectors = append(state.selectors, key, values[key])
	}
	if err := state.captureHostAbsence(ctx, values); err != nil {
		return nil, err
	}
	if err := state.rejoin(ctx); err != nil {
		return nil, err
	}
	return state, nil
}

func sessionPassiveKeys() []string {
	return []string{"HOME", "USERPROFILE", "TMPDIR", "TMP", "TEMP", "HERMES_SCRATCH_DIR", "XDG_STATE_HOME", "HERMES_INSTALL_ROOT", "HERMES_GATEWAY_LOCK_DIR"}
}

func sessionPassiveEnvironment(environment []string) (map[string]string, error) {
	values, seen := map[string]string{}, map[string]bool{}
	for _, item := range environment {
		key, value, _ := strings.Cut(item, "=")
		if runtime.GOOS == "windows" {
			key = strings.ToUpper(key)
		}
		for _, selected := range sessionPassiveKeys() {
			if selected == key {
				if seen[key] || strings.ContainsRune(value, 0) {
					return nil, fmt.Errorf("hermes passive environment is ambiguous")
				}
				values[key], seen[key] = value, true
			}
		}
	}
	if values["HERMES_INSTALL_ROOT"] != "" || values["HERMES_GATEWAY_LOCK_DIR"] != "" {
		return nil, fmt.Errorf("hermes passive startup has a different recovery authority")
	}
	for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
		// Python str.strip includes U+001C..001F in addition to Unicode White_Space.
		value := strings.Trim(values[key], "\t\n\v\f\r\x1c\x1d\x1e\x1f \u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000")
		if value != "" && value != values["HERMES_SCRATCH_DIR"] {
			return values, nil
		}
	}
	return nil, fmt.Errorf("hermes passive startup could activate scratch pruning")
}

func (state *sessionPassiveState) captureHostAbsence(ctx context.Context, values map[string]string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := values["XDG_STATE_HOME"]
	if !filepath.IsAbs(path) {
		home := values["HOME"]
		if runtime.GOOS == "windows" {
			home = values["USERPROFILE"]
		}
		if !cleanRoot(home) {
			return fmt.Errorf("hermes passive host state home is unavailable")
		}
		path = filepath.Join(home, ".local", "state")
	}
	directory, err := openSessionDirectory(path)
	if err != nil {
		return err
	}
	state.directories = append(state.directories, directory)
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := directory.root.Lstat("hermes"); os.IsNotExist(err) {
		state.missing = append(state.missing, sessionMissingMember{directory, "hermes"})
		return nil
	} else if err != nil {
		return err
	}
	child, err := directory.child("hermes")
	if err != nil {
		return err
	}
	state.directories = append(state.directories, child)
	if child.parentIdentity != directory.identity {
		return fmt.Errorf("hermes passive host state parent changed")
	}
	state.missing = append(state.missing, sessionMissingMember{child, "gateway-locks"})
	return nil
}
