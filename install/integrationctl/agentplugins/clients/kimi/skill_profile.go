package kimi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var errKimiAdditionalDirectories = errors.New("kimi additional project-directory policy is unqualified")

type kimiProfileSnapshot struct {
	files      map[string]shared.NativeFileSnapshot
	trust      string
	token      shared.NativeFileSnapshot
	executable os.FileInfo
	profile    os.FileInfo
}

func kimiSkillProfile(ctx context.Context, env clients.Env, request domain.ActivationRequest) (kimiProfileSnapshot, ports.Command, error) {
	var snapshot kimiProfileSnapshot
	var command ports.Command
	if err := env.NativeConfig.RequireBoundedRead(); err != nil {
		return snapshot, command, err
	}
	if _, err := kimiPlanRoot(request.Client, request.Plan); err != nil {
		return snapshot, command, err
	}
	profile, project := request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot
	if !shared.HasListingCLI(request.Client.ExecutablePath) || env.Snapshots == nil {
		return snapshot, command, fmt.Errorf("kimi selected executable or snapshot authority is unavailable")
	}
	for _, path := range []string{env.UserHome, profile, project} {
		if path == "" {
			return snapshot, command, fmt.Errorf("kimi original profile/project authority is unavailable")
		}
		canonical, err := clients.CanonicalDirectory(path, os.Lstat, filepath.EvalSymlinks)
		if err != nil || canonical != path {
			return snapshot, command, fmt.Errorf("kimi original profile/project authority differs")
		}
	}
	profileEnv, err := New().VersionProbeEnvironment(profile)
	if err != nil {
		return snapshot, command, err
	}
	snapshot.files, err = kimiProtectedFiles(env, request)
	if err != nil {
		return snapshot, command, err
	}
	snapshot.trust, snapshot.executable, snapshot.profile, err = kimiProfileIdentities(ctx, env, request)
	if err != nil {
		return snapshot, command, err
	}
	snapshot.token, err = env.NativeConfig.ReadExactFileBounded(filepath.Join(profile, "server.token"), 43)
	if err != nil {
		return snapshot, command, err
	}
	command = ports.Command{Argv: []string{request.Client.ExecutablePath, "web", "--no-open", "--host", "127.0.0.1", "--port", "0"}, Dir: project, StdoutLimitBytes: 1 << 20}
	command.Env, err = kimiSkillEnvironment(env.UserHome, profileEnv, os.Environ())
	return snapshot, command, err
}

func kimiProfileIdentities(ctx context.Context, env clients.Env, request domain.ActivationRequest) (string, os.FileInfo, os.FileInfo, error) {
	trust, err := env.Snapshots.Digest(ctx, filepath.Join(request.Client.ConfigRoot, "workspace-trust"))
	if errors.Is(err, os.ErrNotExist) {
		trust, err = "absent", nil
	}
	if err != nil {
		return "", nil, nil, err
	}
	executable, err := os.Stat(request.Client.ExecutablePath)
	if err != nil {
		return "", nil, nil, err
	}
	profile, err := os.Lstat(request.Client.ConfigRoot)
	if os.IsNotExist(err) {
		err = nil
	}
	return trust, executable, profile, err
}

func kimiProtectedFiles(env clients.Env, request domain.ActivationRequest) (map[string]shared.NativeFileSnapshot, error) {
	profile, project := request.Client.ConfigRoot, request.Plan.TargetContext.ProjectRoot
	files := map[string]shared.NativeFileSnapshot{}
	paths := []string{filepath.Join(profile, "config.toml"), filepath.Join(profile, "mcp.json"), registryPath(profile), filepath.Join(project, ".mcp.json"), filepath.Join(project, ".kimi-code", "mcp.json"), filepath.Join(project, ".kimi-code", "local.toml")}
	for _, path := range paths {
		file, err := env.NativeConfig.ReadExactFileBounded(path, 1<<20)
		if err != nil {
			return nil, err
		}
		files[path] = file
	}
	// 2.5's create route loads additional directories from local.toml. A
	// nonempty unqualified document must not be overridden with an empty list.
	if request.Client.Version == "2.5.2" && strings.TrimSpace(string(files[paths[len(paths)-1]].Body)) != "" {
		return nil, errKimiAdditionalDirectories
	}
	return files, nil
}

func (before kimiProfileSnapshot) same(after kimiProfileSnapshot) bool {
	if !reflect.DeepEqual(before.files, after.files) || before.trust != after.trust || before.executable == nil || after.executable == nil || !os.SameFile(before.executable, after.executable) || before.executable.Size() != after.executable.Size() || !before.executable.ModTime().Equal(after.executable.ModTime()) {
		return false
	}
	if before.profile == nil {
		return after.profile != nil && after.profile.IsDir() && after.profile.Mode()&os.ModeSymlink == 0
	}
	if after.profile == nil || !after.profile.IsDir() || !os.SameFile(before.profile, after.profile) {
		return false
	}
	if before.token.Exists && !reflect.DeepEqual(before.token, after.token) {
		return false
	}
	if before.profile.Mode() == after.profile.Mode() {
		return true
	}
	// Native writePrivateFile hardens the selected parent only while creating
	// its first token. Permit exactly that reduction, with original inode kept.
	return !before.token.Exists && after.profile.Mode() == os.ModeDir|0o700 &&
		after.profile.Mode() & ^before.profile.Mode() == 0
}

func kimiSkillEnvironment(home string, profile, ambient []string) ([]string, error) {
	result := append([]string{}, profile...)
	result = append(result, "HOME="+home, "USERPROFILE="+home, "NO_COLOR=1", "TERM=dumb")
	// Preserve only native process basics. Loader, proxy, provider credentials,
	// trust overrides, selection flags and alternative profile controls are not
	// authority for this explicit, inert catalog request.
	keep := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "TEMP": true, "TMP": true, "TMPDIR": true, "LANG": true, "LC_ALL": true}
	seen := map[string]bool{}
	for _, entry := range ambient {
		key, _, valid := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if !keep[key] {
			continue
		}
		if !valid || seen[key] || strings.ContainsRune(entry, 0) {
			return nil, fmt.Errorf("kimi native process environment is ambiguous")
		}
		seen[key] = true
		result = append(result, entry)
	}
	return result, nil
}

func kimiIssuedToken(env clients.Env, profile, expected string) (shared.NativeFileSnapshot, error) {
	// Official privateFiles.ts blob701c08b3fa9ab08c83c5e8b4fca5a1417209f969
	// checks POSIX mode only outside Windows; these bits do not represent ACLs.
	file, err := env.NativeConfig.ReadExactFileBounded(filepath.Join(profile, "server.token"), 43)
	if err != nil || !file.Exists || (runtime.GOOS != "windows" && file.Mode&0o077 != 0) || len(expected) != 43 || string(file.Body) != expected {
		return shared.NativeFileSnapshot{}, fmt.Errorf("kimi native-issued private bearer differs")
	}
	return file, nil
}
