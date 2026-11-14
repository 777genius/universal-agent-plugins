package hermes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"

	"gopkg.in/yaml.v3"
)

const projectSessionMCPKind = "hermes_project_session_mcp"

// Planning reads exact owned sources and the selected bundle without starting
// a native process. The publisher retains both parent and root identities.
func prepareProjectSession(ctx context.Context, req clients.SessionRequest, owner domain.NativeObjectOwnership) (clients.SessionPlan, error) {
	if err := ctx.Err(); err != nil {
		return clients.SessionPlan{}, err
	}
	if req.Client.ClientID != domain.ClientHermes || !cleanRoot(req.Client.ExecutablePath) || !cleanRoot(req.Client.ConfigRoot) {
		return clients.SessionPlan{}, fmt.Errorf("hermes selected client unavailable")
	}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, TargetContext: domain.TargetContext{ProjectRoot: req.ProjectRoot}}
	if err := New().CheckPlanPrecondition(clients.PlanInput{}, &plan); err != nil {
		return clients.SessionPlan{}, err
	}
	if err := hermesProjectAuthority(req.ProjectRoot, req.Client.ConfigRoot, os.Environ()); err != nil {
		return clients.SessionPlan{}, err
	}
	names, err := sessionBindingNames(req, owner)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	bundle, err := qualifyHermesBundle(req.Client.ExecutablePath)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	body, err := projectSessionConfig(ctx, req.ProjectRoot, owner, names)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	if err := verifySessionReceipts(req, body); err != nil {
		return clients.SessionPlan{}, err
	}
	var args []string
	if req.Inspect {
		args = []string{"config", "get", "mcp_servers", "--json"}
	}
	return clients.SessionPlan{ClientID: domain.ClientHermes, Executable: bundle.executable, Arguments: args,
		Directory: req.ProjectRoot, Environment: map[string]string{"HERMES_HOME": projectSessionRoot(req.ProjectRoot)}, Servers: names,
		ConfigDigest: hermesSessionDigest(body, bundle)}, ctx.Err()
}

func verifySessionReceipts(req clients.SessionRequest, body []byte) error {
	snapshot := nativeconfig.NewWithFileIO(sessionConfigSnapshot{path: filepath.Join(projectSessionRoot(req.ProjectRoot), "config.yaml"), body: body})
	for _, binding := range req.Bindings {
		for _, object := range binding.NativeObjects {
			if object.Kind != projectSessionMCPKind {
				continue
			}
			receipt := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecHermes, Name: object.LogicalName, Digest: object.ManagedDigest}
			present, owned, err := snapshot.Inspect(nativeconfig.Paths{JSON: object.Path}, nativeconfig.CodecHermes, object.LogicalName, &receipt)
			if err != nil || !present || !owned {
				return fmt.Errorf("hermes selected MCP receipt changed")
			}
		}
	}
	return validateSessionLaunchers(req, body)
}

func sessionBindingNames(req clients.SessionRequest, owner domain.NativeObjectOwnership) ([]string, error) {
	selected := map[string]bool{}
	for _, binding := range req.Bindings {
		if err := validateSessionBindingAuthority(req.ProjectRoot, binding); err != nil {
			return nil, err
		}
		rootFound, mcpFound := false, false
		for _, object := range binding.NativeObjects {
			switch object.Kind {
			case projectSessionRootKind:
				if rootFound || object != owner {
					return nil, fmt.Errorf("hermes root receipt differs")
				}
				rootFound = true
			case projectSessionMCPKind:
				if !validSessionMCPObject(object, req.ProjectRoot) || selected[object.LogicalName] {
					return nil, fmt.Errorf("hermes MCP ownership differs")
				}
				mcpFound = true
				selected[object.LogicalName] = true
			case projectSkillKind, "managed_package_directory":
				// The normal installed-package verifier owns these separate receipts.
			default:
				return nil, fmt.Errorf("hermes binding contains foreign ownership")
			}
		}
		if mcpFound && !rootFound {
			return nil, fmt.Errorf("hermes binding has no session root receipt")
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("hermes session has no owned servers")
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

func validateSessionBindingAuthority(project string, binding domain.ClientBinding) error {
	if binding.Scope != string(domain.ScopeProject) || binding.NativeActivationAttempt != "" {
		return fmt.Errorf("hermes binding authority differs")
	}
	return New().ValidateBindingProfile(filepath.Join(project, ".hermes"), binding)
}

func projectSessionConfig(ctx context.Context, project string, owner domain.NativeObjectOwnership, names []string) (result []byte, resultErr error) {
	if err := verifyProjectSessionRoot(ctx, project, owner); err != nil {
		return nil, err
	}
	path := projectSessionRoot(project)
	before, err := noAliasDirectory(path)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	held, err := root.Stat(".")
	if err != nil || !os.SameFile(before, held) {
		return nil, fmt.Errorf("hermes session root changed before open")
	}
	if err := verifySessionBootstrap(ctx, root, held); err != nil {
		return nil, err
	}
	body, err := readSessionMember(root, "config.yaml", 1<<20)
	if err != nil {
		return nil, err
	}
	if err := exactProjectSessionConfig(body, names); err != nil {
		return nil, err
	}
	after, err := noAliasDirectory(path)
	if err != nil || !os.SameFile(before, after) {
		return nil, fmt.Errorf("hermes session root changed during config read")
	}
	return body, ctx.Err()
}

func exactProjectSessionConfig(body []byte, names []string) error {
	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&doc); err != nil {
		return fmt.Errorf("hermes session YAML is malformed")
	}
	var trailing yaml.Node
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return fmt.Errorf("hermes session YAML has trailing document")
	}
	if len(doc.Content) != 1 {
		return fmt.Errorf("hermes session YAML must have one document")
	}
	mapping := doc.Content[0]
	if mapping.Kind != yaml.MappingNode || len(mapping.Content) != 2 || mapping.Content[0].Value != "mcp_servers" {
		return fmt.Errorf("hermes session contains unowned settings")
	}
	servers := mapping.Content[1]
	if servers.Kind != yaml.MappingNode || len(servers.Content) != 2*len(names) {
		return fmt.Errorf("hermes session contains unowned servers")
	}
	wanted := map[string]bool{}
	for _, name := range names {
		if wanted[name] {
			return fmt.Errorf("hermes session names duplicate")
		}
		wanted[name] = true
	}
	for index := 0; index < len(servers.Content); index += 2 {
		key, value := servers.Content[index], servers.Content[index+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || !wanted[key.Value] || value.Kind != yaml.MappingNode {
			return fmt.Errorf("hermes session server identity differs")
		}
		delete(wanted, key.Value)
	}
	return sessionPlainYAML(&doc, 0)
}

func sessionPlainYAML(node *yaml.Node, depth int) error {
	if depth > 32 {
		return fmt.Errorf("hermes session YAML nesting exceeded bound")
	}
	if node.Alias != nil || node.Anchor != "" || node.Kind == yaml.AliasNode {
		return fmt.Errorf("hermes session YAML aliases refused")
	}
	keys := map[string]bool{}
	for index, child := range node.Content {
		if node.Kind == yaml.MappingNode && index%2 == 0 {
			if child.Kind != yaml.ScalarNode || child.Tag != "!!str" || keys[child.Value] || child.Value == "<<" {
				return fmt.Errorf("hermes session duplicate or merged keys refused")
			}
			keys[child.Value] = true
		}
		if err := sessionPlainYAML(child, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func verifyProjectSessionSource(ctx context.Context, plan clients.SessionPlan, owner domain.NativeObjectOwnership) error {
	inspect := []string{"config", "get", "mcp_servers", "--json"}
	if plan.ClientID != domain.ClientHermes || len(plan.Environment) != 1 || plan.Environment["HERMES_HOME"] != projectSessionRoot(plan.Directory) ||
		len(plan.Servers) == 0 || len(plan.Arguments) != 0 && !slices.Equal(plan.Arguments, inspect) {
		return fmt.Errorf("hermes session overrides differ")
	}
	if err := hermesSessionSignals(projectSessionRoot(plan.Directory), os.Environ()); err != nil {
		return err
	}
	bundle, err := qualifyHermesBundle(plan.Executable)
	if err != nil || bundle.executable != plan.Executable {
		return fmt.Errorf("hermes selected bundle changed")
	}
	body, err := projectSessionConfig(ctx, plan.Directory, owner, plan.Servers)
	if err != nil {
		return err
	}
	if plan.ConfigDigest != hermesSessionDigest(body, bundle) {
		return fmt.Errorf("hermes session configuration changed")
	}
	return ctx.Err()
}

func hermesSessionDigest(body []byte, bundle hermesBundle) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%s:%x", bundle.digest, sha256.Sum256(body)))))
}

func validateSessionLaunchers(req clients.SessionRequest, body []byte) error {
	var document struct {
		Servers map[string]map[string]any `yaml:"mcp_servers"`
	}
	if err := yaml.Unmarshal(body, &document); err != nil {
		return err
	}
	for _, binding := range req.Bindings {
		for _, object := range binding.NativeObjects {
			if object.Kind != projectSessionMCPKind {
				continue
			}
			entry := document.Servers[object.LogicalName]
			if disabled, present := entry["disabled"]; present && disabled != false {
				return fmt.Errorf("hermes owned MCP entry was disabled")
			}
			if command, stdio := entry["command"]; stdio && command != filepath.Join(binding.TargetLocator, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
				return fmt.Errorf("hermes selected stdio must use its owned managed launcher")
			}
		}
	}
	return nil
}

// Freeze one bounded source for the existing codec's receipt verification; no
// parser duplication or second pathname read can adopt a successive revision.
type sessionConfigSnapshot struct {
	path string
	body []byte
}

func (snapshot sessionConfigSnapshot) ReadNoFollow(path string) ([]byte, os.FileMode, bool, error) {
	if path != snapshot.path {
		return nil, 0, false, fmt.Errorf("hermes snapshot path differs")
	}
	return append([]byte(nil), snapshot.body...), 0o600, true, nil
}

func (sessionConfigSnapshot) WriteAtomic(string, []byte, os.FileMode) error {
	return fmt.Errorf("hermes session snapshot is read-only")
}

func (sessionConfigSnapshot) RemoveNoFollow(string) error {
	return fmt.Errorf("hermes session snapshot is read-only")
}

func validSessionMCPObject(object domain.NativeObjectOwnership, project string) bool {
	return !object.UserModified && object.ManagedDigest != "" && object.ProtectionClass == "managed" &&
		object.Path == filepath.Join(projectSessionRoot(project), "config.yaml") && object.LogicalName != "" &&
		object.ObjectID == projectSessionMCPKind+":"+object.LogicalName
}

// The pinned bootstrap creates only this finite catalog. Backups remain
// historical bytes: a valid receipt-owned primary is required before every
// child, and native effective-source readback cannot adopt a fallback.
func verifySessionBootstrap(ctx context.Context, root *os.Root, held os.FileInfo) (resultErr error) {
	members := map[string]os.FileInfo{".": held}
	for _, name := range []string{".", "cron", "sessions", "logs", "logs/curator", "memories", "pairing", "hooks", "image_cache", "audio_cache", "skills", "backups", "backups/config"} {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := inspectBootstrapDirectory(root, name, members); err != nil {
			return err
		}
	}
	var soul *os.File
	defer func() {
		if soul != nil {
			resultErr = errors.Join(resultErr, soul.Close())
		}
	}()
	for name, info := range members {
		if info.IsDir() {
			continue
		}
		file, err := inspectBootstrapFile(root, name, info)
		if err != nil {
			return err
		}
		if file != nil {
			soul = file
		}
	}
	for name, before := range members {
		after, err := root.Lstat(name)
		if err != nil || !sameSessionMember(before, after) {
			return fmt.Errorf("hermes bootstrap membership changed")
		}
	}
	if soul != nil {
		body, err := io.ReadAll(io.LimitReader(soul, 668))
		after, statErr := soul.Stat()
		named, nameErr := root.Lstat("SOUL.md")
		if err != nil || statErr != nil || nameErr != nil || !sameSessionMember(members["SOUL.md"], after) ||
			!sameSessionMember(after, named) || len(body) != 667 || fmt.Sprintf("%x", sha256.Sum256(body)) != "36c1f5a2e92cd1d018311eaf4c8f1e8886672eae78212e033c681d0e3d5d506f" {
			return fmt.Errorf("hermes bootstrap SOUL differs from pinned default")
		}
	}
	return ctx.Err()
}

func inspectBootstrapDirectory(root *os.Root, name string, members map[string]os.FileInfo) (resultErr error) {
	before, present := members[name]
	if !present {
		return nil
	}
	directory, err := root.OpenRoot(filepath.FromSlash(name))
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, directory.Close()) }()
	file, err := directory.Open(".")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	held, err := file.Stat()
	if err != nil || !sameSessionMember(before, held) {
		return fmt.Errorf("hermes bootstrap directory changed before enumeration")
	}
	if err := validateSessionOwnedDirectory(file, held); err != nil {
		return err
	}
	if name == "." {
		if err := validateSessionDirectoryHandle(file, held); err != nil {
			return err
		}
	}
	limit := map[string]int{".": 14, "logs": 3, "backups": 1, "backups/config": 5}[name]
	names, err := file.Readdirnames(limit + 1)
	if err != nil && !errors.Is(err, io.EOF) || len(names) > limit {
		return fmt.Errorf("hermes bootstrap membership exceeds the fixed catalog")
	}
	for _, child := range names {
		relative := filepath.ToSlash(filepath.Join(name, child))
		info, err := directory.Lstat(child)
		if err != nil || !bootstrapMemberMetadata(relative, info) {
			return fmt.Errorf("hermes bootstrap contains an unknown or unsafe member")
		}
		members[relative] = info
	}
	after, err := root.Lstat(filepath.FromSlash(name))
	if err != nil || !sameSessionMember(before, after) {
		return fmt.Errorf("hermes bootstrap directory changed during enumeration")
	}
	return nil
}

func bootstrapMemberMetadata(name string, info os.FileInfo) bool {
	mode, limit, directory := os.FileMode(0o600), int64(1<<20), false
	switch name {
	case "cron", "sessions", "logs", "logs/curator", "memories", "pairing", "hooks", "image_cache", "audio_cache", "skills":
		mode, directory = 0o700, true
	case "backups", "backups/config":
		mode, directory = 0o755, true
	case ".uap-owner":
		limit = 4096
	case "config.yaml":
	case "config.yaml.agentplugins.lock":
		limit = 0
	case "SOUL.md":
		limit = 667
	case "logs/agent.log", "logs/errors.log":
		mode = 0o644
	default:
		stamp, ok := strings.CutPrefix(name, "backups/config/config.yaml.good.")
		if _, err := time.Parse("20060102-150405", stamp); !ok || len(stamp) != 15 || err != nil {
			return false
		}
	}
	if runtime.GOOS != "windows" {
		expected := mode
		if directory {
			expected |= os.ModeDir
		}
		if info.Mode() != expected {
			return false
		}
	}
	return directory && info.IsDir() && info.Mode()&os.ModeSymlink == 0 ||
		!directory && info.Mode().IsRegular() && info.Size() >= 0 && info.Size() <= limit
}

func inspectBootstrapFile(root *os.Root, name string, before os.FileInfo) (result *os.File, resultErr error) {
	file, err := openSessionFile(root, filepath.FromSlash(name))
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			resultErr = errors.Join(resultErr, file.Close())
		}
	}()
	held, err := file.Stat()
	if err != nil || !sameSessionMember(before, held) {
		return nil, fmt.Errorf("hermes bootstrap file changed before inspection")
	}
	if err := validateSessionOwnedHandle(file, held); err != nil {
		return nil, err
	}
	named, err := root.Lstat(filepath.FromSlash(name))
	if err != nil || !sameSessionMember(held, named) {
		return nil, fmt.Errorf("hermes bootstrap file changed during inspection")
	}
	if name == "SOUL.md" {
		return file, nil
	}
	return nil, nil
}
