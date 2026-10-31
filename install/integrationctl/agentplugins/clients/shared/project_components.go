package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectComponents names one adapter's already-selected native file layout.
// It supplies ownership mechanics, not client discovery or settings policy.
type ProjectComponents struct {
	ProjectRoot, SkillsRoot string
	ConfigPaths             nativeconfig.Paths
	Codec                   nativeconfig.Codec
	SkillKind, MCPKind      string
}

func (layout ProjectComponents) validate() error {
	if !cleanComponentPath(layout.ProjectRoot) || layout.SkillKind == "" || layout.MCPKind == "" || layout.SkillKind == layout.MCPKind {
		return fmt.Errorf("invalid project component layout")
	}
	paths := []string{layout.SkillsRoot, layout.ConfigPaths.JSON}
	if layout.ConfigPaths.JSONC != "" {
		paths = append(paths, layout.ConfigPaths.JSONC)
	}
	for _, path := range paths {
		if !cleanComponentPath(path) {
			return fmt.Errorf("project component path must be clean and absolute")
		}
		if err := pathpolicy.RequireContainedChild(layout.ProjectRoot, path); err != nil {
			return err
		}
	}
	return nil
}

func cleanComponentPath(path string) bool {
	return utf8.ValidString(path) && filepath.IsAbs(path) && filepath.Clean(path) == path && filepath.Dir(path) != path && strings.TrimSpace(path) == path && strings.IndexFunc(path, unicode.IsControl) < 0
}

func (layout ProjectComponents) ValidateObjects(objects []domain.NativeObjectOwnership) error {
	if err := layout.validate(); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, object := range objects {
		if object.LogicalName == "" {
			return fmt.Errorf("project component name must be non-empty")
		}
		// Skills are directory leafs; MCP server names are JSON object keys.
		// Never impose filesystem leaf syntax on an MCP selector.
		if object.Kind == layout.SkillKind {
			if err := pathpolicy.ValidateLeafID(object.LogicalName); err != nil {
				return err
			}
		}
		if object.ObjectID != object.Kind+":"+object.LogicalName || seen[object.ObjectID] || object.ManagedDigest == "" {
			return fmt.Errorf("invalid or duplicate project component ownership")
		}
		seen[object.ObjectID] = true
		if err := layout.validateObjectPath(object); err != nil {
			return err
		}
	}
	return nil
}

func (layout ProjectComponents) validateObjectPath(object domain.NativeObjectOwnership) error {
	switch object.Kind {
	case layout.SkillKind:
		if object.Path != filepath.Join(layout.SkillsRoot, object.LogicalName) {
			return fmt.Errorf("project skill path differs from selected layout")
		}
		if err := componentRelative(object.SourceRelative); err != nil {
			return err
		}
	case layout.MCPKind:
		if object.Path != layout.ConfigPaths.JSON && (layout.ConfigPaths.JSONC == "" || object.Path != layout.ConfigPaths.JSONC) {
			return fmt.Errorf("project MCP path differs from selected layout")
		}
	default:
		return fmt.Errorf("unknown project component ownership kind %q", object.Kind)
	}
	if !cleanComponentPath(object.Path) {
		return fmt.Errorf("invalid project native object path")
	}
	if err := pathpolicy.RequireContainedChild(layout.ProjectRoot, object.Path); err != nil {
		return err
	}
	return nil
}

func componentRelative(path string) error {
	if path == "" || strings.ContainsAny(path, "\\:") || strings.IndexFunc(path, unicode.IsControl) >= 0 {
		return fmt.Errorf("invalid portable skill source directory")
	}
	for _, part := range strings.Split(path, "/") {
		if err := pathpolicy.ValidatePortablePathSegment(part); err != nil {
			return fmt.Errorf("invalid portable skill source segment: %w", err)
		}
	}
	return nil
}

func (layout ProjectComponents) SkillObject(name, sourceRelative, stagingRoot string) (domain.NativeObjectOwnership, error) {
	object := domain.NativeObjectOwnership{ObjectID: layout.SkillKind + ":" + name, Kind: layout.SkillKind, LogicalName: name,
		Path: filepath.Join(layout.SkillsRoot, name), SourceRelative: sourceRelative, ProtectionClass: "managed"}
	if err := layout.packagePath(stagingRoot); err != nil {
		return object, err
	}
	if err := componentRelative(sourceRelative); err != nil {
		return object, err
	}
	source := filepath.Join(stagingRoot, filepath.FromSlash(sourceRelative))
	if err := pathpolicy.RequireContainedChild(stagingRoot, source); err != nil {
		return object, err
	}
	digest, err := DigestSkillDirectory(source)
	if err != nil {
		return object, err
	}
	object.ManagedDigest = digest
	return object, layout.ValidateObjects([]domain.NativeObjectOwnership{object})
}

func (layout ProjectComponents) packagePath(root string) error {
	if !cleanComponentPath(root) {
		return fmt.Errorf("project package root must be clean and absolute")
	}
	if err := pathpolicy.RequireContainedChild(layout.ProjectRoot, root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("project package root must be a real directory")
	}
	return nil
}

func (layout ProjectComponents) MCPObject(name string, server nativeconfig.Server, resolvedPath string) (domain.NativeObjectOwnership, error) {
	receipt, err := nativeconfig.DesiredReceipt(resolvedPath, layout.Codec, name, server, nativeconfig.Placeholders{})
	object := domain.NativeObjectOwnership{ObjectID: layout.MCPKind + ":" + name, Kind: layout.MCPKind, LogicalName: name,
		Path: resolvedPath, ManagedDigest: receipt.Digest, ProtectionClass: "managed"}
	if err != nil {
		return object, err
	}
	return object, layout.ValidateObjects([]domain.NativeObjectOwnership{object})
}

func (layout ProjectComponents) receipt(object domain.NativeObjectOwnership) *nativeconfig.Receipt {
	return &nativeconfig.Receipt{Version: "1", Path: object.Path, Codec: layout.Codec, Name: object.LogicalName, Digest: object.ManagedDigest}
}

func verifyComponentSkill(object domain.NativeObjectOwnership, allowMissing bool) error {
	info, err := os.Lstat(object.Path)
	if os.IsNotExist(err) && allowMissing {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("managed project skill is not a real directory")
	}
	digest, err := DigestSkillDirectory(object.Path)
	if err != nil {
		return err
	}
	if digest != object.ManagedDigest {
		return fmt.Errorf("managed project skill %q changed outside agentplugins", object.LogicalName)
	}
	return nil
}

func (layout ProjectComponents) Verify(objects []domain.NativeObjectOwnership, allowMissing bool, kernel nativeconfig.Kernel) error {
	if err := layout.ValidateObjects(objects); err != nil {
		return err
	}
	if err := kernel.RequireFileIO(); err != nil {
		return err
	}
	for _, object := range objects {
		if object.Kind == layout.SkillKind {
			if err := verifyComponentSkill(object, allowMissing); err != nil {
				return err
			}
			continue
		}
		present, owned, err := kernel.Inspect(layout.ConfigPaths, layout.Codec, object.LogicalName, layout.receipt(object))
		if err != nil {
			return err
		}
		if !present && allowMissing {
			continue
		}
		if !present || !owned {
			return fmt.Errorf("managed project MCP server %q changed outside agentplugins", object.LogicalName)
		}
	}
	return nil
}

func (layout ProjectComponents) Inspect(components []domain.ComponentDecision, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	prior := []domain.NativeObjectOwnership{}
	if managed != nil {
		prior = managed.NativeObjects
	}
	if err := layout.Verify(prior, true, kernel); err != nil {
		return clients.RegistryIndeterminate, err
	}
	ownership := ObjectMap(prior)
	finding := clients.RegistryClear
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		present, owned, err := layout.inspectComponent(component, ownership, kernel)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if present && !owned {
			return clients.RegistryCollision, nil
		}
		if present {
			finding = clients.RegistryExpected
		}
	}
	return finding, nil
}

func (layout ProjectComponents) inspectComponent(component domain.ComponentDecision, ownership map[string]domain.NativeObjectOwnership, kernel nativeconfig.Kernel) (bool, bool, error) {
	if component.Name == "" {
		return false, false, fmt.Errorf("project component name must be non-empty")
	}
	if component.Kind == domain.ComponentSkill {
		if err := pathpolicy.ValidateLeafID(component.Name); err != nil {
			return false, false, err
		}
		_, err := os.Lstat(filepath.Join(layout.SkillsRoot, component.Name))
		if err != nil && !os.IsNotExist(err) {
			return false, false, err
		}
		_, owned := ownership[layout.SkillKind+":"+component.Name]
		return err == nil, owned, nil
	}
	if component.Kind == domain.ComponentMCPServer {
		var receipt *nativeconfig.Receipt
		if object, ok := ownership[layout.MCPKind+":"+component.Name]; ok {
			receipt = layout.receipt(object)
		}
		return kernel.Inspect(layout.ConfigPaths, layout.Codec, component.Name, receipt)
	}
	return false, false, fmt.Errorf("unsupported native project component kind %q", component.Kind)
}
