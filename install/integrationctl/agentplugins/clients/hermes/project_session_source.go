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
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

	"gopkg.in/yaml.v3"
)

const projectSessionMCPKind = "hermes_project_session_mcp"

// Held preparer: not registered as an Adapter capability until native runtime
// and selected-session admission pass. No native process or writer runs here.
func prepareProjectSession(ctx context.Context, req clients.SessionRequest, owner domain.NativeObjectOwnership) (clients.SessionPlan, error) {
	if err := verifyProjectSessionRoot(ctx, req.ProjectRoot, owner); err != nil {
		return clients.SessionPlan{}, err
	}
	if req.Client.ClientID != domain.ClientHermes || !cleanRoot(req.Client.ExecutablePath) || !cleanRoot(req.Client.ConfigRoot) {
		return clients.SessionPlan{}, fmt.Errorf("hermes selected client unavailable")
	}
	root := projectSessionRoot(req.ProjectRoot)
	if req.Client.ConfigRoot == root || strings.HasPrefix(root, req.Client.ConfigRoot+string(filepath.Separator)) ||
		strings.HasPrefix(req.Client.ConfigRoot, root+string(filepath.Separator)) {
		return clients.SessionPlan{}, fmt.Errorf("hermes session cannot borrow original User root")
	}
	names, err := sessionBindingNames(req, owner)
	if err != nil {
		return clients.SessionPlan{}, err
	}
	body, err := projectSessionConfig(ctx, req.ProjectRoot, owner, names)
	if err != nil {
		return clients.SessionPlan{}, err
	}

	snapshot := nativeconfig.NewWithFileIO(sessionConfigSnapshot{path: filepath.Join(root, "config.yaml"), body: body})
	for _, binding := range req.Bindings {
		for _, object := range binding.NativeObjects {
			if object.Kind != projectSessionMCPKind {
				continue
			}
			receipt := nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: nativeconfig.CodecHermes, Name: object.LogicalName, Digest: object.ManagedDigest}
			present, owned, err := snapshot.Inspect(nativeconfig.Paths{JSON: object.Path}, nativeconfig.CodecHermes, object.LogicalName, &receipt)
			if err != nil || !present || !owned {
				return clients.SessionPlan{}, fmt.Errorf("hermes selected MCP receipt changed")
			}
		}
	}
	var args []string
	if req.Inspect {
		args = []string{"mcp", "list"}
	}
	return clients.SessionPlan{ClientID: domain.ClientHermes, Executable: req.Client.ExecutablePath, Arguments: args, Directory: req.ProjectRoot, Environment: map[string]string{"HERMES_HOME": root}, Servers: names, ConfigDigest: fmt.Sprintf("%x", sha256.Sum256(body))}, nil
}

func sessionBindingNames(req clients.SessionRequest, owner domain.NativeObjectOwnership) ([]string, error) {
	selected := map[string]bool{}
	for _, binding := range req.Bindings {
		if binding.ClientID != string(domain.ClientHermes) || binding.Scope != string(domain.ScopeProject) || binding.NativeProfileRoot != filepath.Join(req.ProjectRoot, ".hermes") || binding.NativeActivationAttempt != "" {
			return nil, fmt.Errorf("hermes binding authority differs")
		}
		rootFound := false
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
				selected[object.LogicalName] = true
			case projectSkillKind, "managed_package_directory":
				// The normal installed-package verifier owns these separate receipts.
			default:
				return nil, fmt.Errorf("hermes binding contains foreign ownership")
			}
		}
		if !rootFound {
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
	entries, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	namesInRoot, readErr := entries.Readdirnames(4)
	closeErr := entries.Close()
	if readErr != nil && !errors.Is(readErr, io.EOF) || closeErr != nil || len(namesInRoot) > 3 {
		return nil, fmt.Errorf("hermes session root contains unknown entries")
	}
	for _, name := range namesInRoot {
		if name != ".uap-owner" && name != "config.yaml" && name != "config.yaml.agentplugins.lock" {
			return nil, fmt.Errorf("hermes session root contains unknown entries")
		}
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
	if plan.ClientID != domain.ClientHermes || len(plan.Environment) != 1 || plan.Environment["HERMES_HOME"] != projectSessionRoot(plan.Directory) {
		return fmt.Errorf("hermes session overrides differ")
	}
	body, err := projectSessionConfig(ctx, plan.Directory, owner, plan.Servers)
	if err != nil {
		return err
	}
	if plan.ConfigDigest != fmt.Sprintf("%x", sha256.Sum256(body)) {
		return fmt.Errorf("hermes session configuration changed")
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
