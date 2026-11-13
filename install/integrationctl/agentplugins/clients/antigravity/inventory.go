package antigravity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var errInventoryUnavailable = errors.New("antigravity selected User inventory is unavailable")
var errInventoryConfigAbsent = errors.New("antigravity existing qualified config is required for native observation")

var _ clients.ReadOnlyVerifier = (*Adapter)(nil)

func (*Adapter) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backend string) bool {
	if client.ClientID != domain.ClientAntigravity || plan.ClientID != domain.ClientAntigravity || selectedNativePluginRoot(plan) != nil {
		return false
	}
	// Existing Project file activation stays unchanged; this classifier is User-only.
	return inventoryHome(client, plan, backend) != ""
}

func inventoryHome(client domain.DetectedClient, plan domain.DeliveryPlan, backend string) string {
	root := client.NativeConfigRoot
	if plan.Scope != domain.ScopeUser || !cleanAbsolute(root) || client.ConfigRoot != root || plan.NativeRegistryRoot != root ||
		!cleanAbsolute(backend) || backend != client.ExecutablePath || backend != plan.NativeRegistryExecutable ||
		filepath.Base(root) != "config" || filepath.Base(filepath.Dir(root)) != ".gemini" {
		return ""
	}
	return filepath.Dir(filepath.Dir(root))
}

// These records attest portable incarnation/bytes/mode, not UID, nlink or ACL ownership.
type inventoryFile struct {
	root      *os.Root
	file      *os.File
	parent    string
	name      string
	rootNamed os.FileInfo
	rootHeld  os.FileInfo
	named     os.FileInfo
	held      os.FileInfo
	digest    string
	limit     int64
}

func sameInventoryFile(left, right os.FileInfo) bool {
	return os.SameFile(left, right) && left.Mode() == right.Mode() && left.Size() == right.Size() &&
		left.ModTime().Equal(right.ModTime())
}

func inventoryDigest(ctx context.Context, file io.ReadSeeker, limit int64) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	hash, count := sha256.New(), int64(0)
	buffer := make([]byte, 32768)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := file.Read(buffer[:min(int64(len(buffer)), limit-count+1)])
		count += int64(n)
		if count > limit {
			return "", errInventoryUnavailable
		}
		_, _ = hash.Write(buffer[:n])
		if cause := ctx.Err(); cause != nil {
			return "", cause
		}
		if errors.Is(err, io.EOF) {
			return hex.EncodeToString(hash.Sum(nil)), nil
		}
		if err != nil {
			return "", err
		}
	}
}

func openInventoryFile(ctx context.Context, path string, limit int64) (record *inventoryFile, resultErr error) {
	parent, name := filepath.Dir(path), filepath.Base(path)
	canonical, err := filepath.EvalSymlinks(parent)
	if err != nil || canonical != parent {
		return nil, errInventoryUnavailable
	}
	root, err := os.OpenRoot(parent)
	if err != nil {
		return nil, err
	}
	record = &inventoryFile{root: root, parent: parent, name: name, limit: limit}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, record.close())
			record = nil
		}
	}()
	if record.rootNamed, err = os.Lstat(parent); err != nil {
		return record, err
	}
	if record.rootHeld, err = root.Stat("."); err != nil || !record.rootNamed.IsDir() || !record.rootHeld.IsDir() ||
		!os.SameFile(record.rootNamed, record.rootHeld) {
		return record, errInventoryUnavailable
	}
	if record.named, err = root.Lstat(name); err != nil {
		return record, err
	}
	if !record.named.Mode().IsRegular() || record.named.Size() < 0 || record.named.Size() > limit {
		return record, errInventoryUnavailable
	}
	if record.file, err = atomicfile.OpenRootReadFile(root, name); err != nil {
		return record, err
	}
	if record.held, err = record.file.Stat(); err != nil || !os.SameFile(record.named, record.held) {
		return record, errInventoryUnavailable
	}
	if record.digest, err = inventoryDigest(ctx, record.file, limit); err != nil {
		return record, err
	}
	err = record.verify(ctx)
	return record, err
}

func (record *inventoryFile) close() error {
	var err error
	if record.file != nil {
		err = record.file.Close()
	}
	if record.root != nil {
		err = errors.Join(err, record.root.Close())
	}
	return err
}

func (record *inventoryFile) identity() error {
	canonical, err := filepath.EvalSymlinks(record.parent)
	if err != nil || canonical != record.parent {
		return errInventoryUnavailable
	}
	namedRoot, err := os.Lstat(record.parent)
	if err != nil {
		return err
	}
	heldRoot, err := record.root.Stat(".")
	if err != nil || !sameInventoryFile(record.rootNamed, namedRoot) || !sameInventoryFile(record.rootHeld, heldRoot) {
		return errInventoryUnavailable
	}
	named, err := record.root.Lstat(record.name)
	if err != nil {
		return err
	}
	held, err := record.file.Stat()
	if err != nil || !sameInventoryFile(record.named, named) || !sameInventoryFile(record.held, held) {
		return errInventoryUnavailable
	}
	return nil
}

func (record *inventoryFile) verify(ctx context.Context) error {
	if err := record.identity(); err != nil {
		return err
	}
	digest, err := inventoryDigest(ctx, record.file, record.limit)
	if err != nil || digest != record.digest {
		return errInventoryUnavailable
	}
	return record.identity()
}

func inventoryConfig(ctx context.Context, record *inventoryFile, selector string) (*bool, error) {
	var expected *bool
	if _, err := record.file.Seek(0, io.SeekStart); err != nil {
		return nil, errInventoryUnavailable
	}
	body, err := io.ReadAll(io.LimitReader(record.file, record.limit+1))
	if err != nil || int64(len(body)) > record.limit {
		return nil, errInventoryUnavailable
	}
	document, err := shared.DecodeStrictJSONObject(body)
	if err != nil || len(document) != 1 {
		return nil, errInventoryConfigAbsent
	}
	plugins, ok := document["plugins"].(map[string]any)
	if !ok || len(plugins) > 128 {
		return nil, errInventoryConfigAbsent
	}
	for name, value := range plugins {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 1 || !nativePluginID.MatchString(name) {
			return nil, errInventoryConfigAbsent
		}
		enabled, ok := row["enabled"].(bool)
		if !ok {
			return nil, errInventoryConfigAbsent
		}
		if name == selector {
			expected = &enabled
		}
	}
	if expected == nil {
		return nil, errInventoryConfigAbsent // Unmeasured absent-override/bootstrap state.
	}
	return expected, record.verify(ctx)
}

func inventoryCommand(ctx context.Context, env clients.Env, home, dir string, argv []string) ([]byte, error) {
	result, err := env.Runner.Run(ctx, legacyports.Command{Argv: argv, Dir: dir,
		Env: []string{"HOME=" + home, "USERPROFILE=" + home}, StdoutLimitBytes: 1 << 20})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil || !result.CleanupComplete || result.NoChildStarted || result.ExitCode != 0 ||
		len(result.Stderr) != 0 || len(result.Stdout) >= 1<<20 {
		return nil, errInventoryUnavailable
	}
	return result.Stdout, nil
}

// The caller verifies the receipt-bound package before activation. Rejoin the
// existing filesystem namespace before each new native command: a later foreign
// manifest or aliased target cannot borrow the selected package's ownership.
func verifyInventoryNamespace(ctx context.Context, req domain.ActivationRequest) error {
	if req.Plan.DeclaredName != req.DeclaredName {
		return errInventoryUnavailable
	}
	canonical, err := filepath.EvalSymlinks(req.Plan.ActivePath)
	if err != nil || canonical != req.Plan.ActivePath {
		return errInventoryUnavailable
	}
	finding, err := New().InspectPreparedRegistry(req.Plan, req.DeclaredName, true)
	if err != nil || finding != clients.RegistryExpected {
		return errInventoryUnavailable
	}
	return ctx.Err()
}

func observeUserInventory(ctx context.Context, env clients.Env, req domain.ActivationRequest) (enabled, present, attempted bool, resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	home := inventoryHome(req.Client, req.Plan, req.BackendExecutable)
	if home == "" || env.Runner == nil {
		return false, false, false, errInventoryUnavailable
	}
	config, err := openInventoryFile(ctx, filepath.Join(req.Plan.NativeRegistryRoot, "config.json"), 16384)
	if errors.Is(err, os.ErrNotExist) {
		return false, false, false, errInventoryConfigAbsent
	}
	if err != nil {
		return false, false, false, errInventoryUnavailable
	}
	var binary *inventoryFile
	defer func() {
		if resultErr == nil && binary != nil {
			resultErr = errors.Join(config.identity(), binary.identity())
		}
		if binary != nil {
			resultErr = errors.Join(resultErr, binary.close())
		}
		resultErr = errors.Join(resultErr, config.close(), ctx.Err())
	}()
	expected, err := inventoryConfig(ctx, config, filepath.Base(req.Plan.ActivePath))
	if err != nil {
		return false, false, false, err
	}
	binary, err = openInventoryFile(ctx, req.BackendExecutable, 512<<20)
	if err != nil {
		return false, false, false, errInventoryUnavailable
	}
	for index, argv := range [][]string{{req.BackendExecutable, "--version"}, {req.BackendExecutable, "plugin", "list", "--json"}} {
		if err = errors.Join(verifyInventoryNamespace(ctx, req), config.verify(ctx), binary.verify(ctx), config.identity(), binary.identity(), ctx.Err()); err != nil {
			return false, false, attempted, errInventoryUnavailable
		}
		attempted = true
		body, commandErr := inventoryCommand(ctx, env, home, req.Plan.NativeRegistryRoot, argv)
		if commandErr != nil {
			return false, false, true, commandErr
		}
		if err = errors.Join(verifyInventoryNamespace(ctx, req), config.verify(ctx), binary.verify(ctx), config.identity(), binary.identity(), ctx.Err()); err != nil {
			return false, false, true, errInventoryUnavailable
		}
		if index == 0 {
			if !bytes.Equal(body, []byte("1.5.3\n")) {
				return false, false, true, errInventoryUnavailable
			}
		} else {
			enabled, present, err = inventoryReadback(body, req.Plan, *expected)
			return enabled, present, true, err
		}
	}
	return false, false, attempted, errInventoryUnavailable
}

func inventoryNames(value any) ([]string, error) {
	values, ok := value.([]any)
	if !ok || len(values) > 128 {
		return nil, errInventoryUnavailable
	}
	names, seen := make([]string, 0, len(values)), map[string]bool{}
	for _, value := range values {
		name, ok := value.(string)
		if !ok || name == "" || seen[name] {
			return nil, errInventoryUnavailable
		}
		names, seen[name] = append(names, name), true
	}
	sort.Strings(names)
	return names, nil
}

func inventoryComponents(components []domain.ComponentDecision) (map[domain.ComponentKind][]string, error) {
	expected := map[domain.ComponentKind][]string{domain.ComponentSkill: {}, domain.ComponentMCPServer: {}}
	if len(components) > 256 {
		return nil, errInventoryUnavailable
	}
	for _, component := range components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		names, known := expected[component.Kind]
		if !known || component.Name == "" || len(names) >= 128 || slices.Contains(names, component.Name) {
			return nil, errInventoryUnavailable
		}
		expected[component.Kind] = append(names, component.Name)
	}
	for _, names := range expected {
		sort.Strings(names)
	}
	return expected, nil
}

func inventorySelectedRow(rows []any, active string) (map[string]any, error) {
	var selected map[string]any
	seenNames, seenPaths := map[string]bool{}, map[string]bool{}
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok || len(row) != 6 {
			return nil, errInventoryUnavailable
		}
		name, nameOK := row["name"].(string)
		path, pathOK := row["path"].(string)
		version, versionOK := row["version"].(string)
		_, enabledOK := row["enabled"].(bool)
		_, skillErr := inventoryNames(row["skills"])
		_, serverErr := inventoryNames(row["mcpServers"])
		if !nameOK || !pathOK || !inventoryPhysicalRow(name, path) || !versionOK || version == "" || !enabledOK || skillErr != nil || serverErr != nil {
			return nil, errInventoryUnavailable
		}
		foldedName, foldedPath := strings.ToLower(name), strings.ToLower(path)
		if seenNames[foldedName] || seenPaths[foldedPath] {
			return nil, errInventoryUnavailable
		}
		seenNames[foldedName], seenPaths[foldedPath] = true, true
		if strings.EqualFold(name, filepath.Base(active)) || strings.EqualFold(path, active) {
			if selected != nil || name != filepath.Base(active) || path != active {
				return nil, errInventoryUnavailable
			}
			selected = row
		}
	}
	return selected, nil
}

// The six-field schema matches all21 retained811 native list captures across three OSes.
func selectedInventory(body []byte, plan domain.DeliveryPlan) (bool, bool, error) {
	if len(body) == 0 || len(body) >= 1<<20 || !utf8.Valid(body) {
		return false, false, errInventoryUnavailable
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	value, err := shared.DecodeUniqueJSONValue(decoder)
	if err != nil {
		return false, false, errInventoryUnavailable
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return false, false, errInventoryUnavailable
	}
	rows, ok := value.([]any)
	if !ok || len(rows) > 4096 || plan.DeclaredVersion == "" || !cleanAbsolute(plan.ActivePath) {
		return false, false, errInventoryUnavailable
	}
	expected, err := inventoryComponents(plan.Components)
	if err != nil {
		return false, false, err
	}
	selected, err := inventorySelectedRow(rows, plan.ActivePath)
	if err != nil {
		return false, false, err
	}
	if selected == nil {
		return false, false, nil
	}
	enabled, enabledOK := selected["enabled"].(bool)
	version, versionOK := selected["version"].(string)
	skills, skillErr := inventoryNames(selected["skills"])
	servers, serverErr := inventoryNames(selected["mcpServers"])
	if !enabledOK || !versionOK || version != plan.DeclaredVersion || skillErr != nil || serverErr != nil ||
		!slices.Equal(skills, expected[domain.ComponentSkill]) || !slices.Equal(servers, expected[domain.ComponentMCPServer]) {
		return false, false, errInventoryUnavailable
	}
	return enabled, true, nil
}

func inventoryReadback(body []byte, plan domain.DeliveryPlan, expected bool) (bool, bool, error) {
	enabled, present, err := selectedInventory(body, plan)
	if err == nil && present && enabled != expected {
		err = errInventoryUnavailable
	}
	return enabled, present, err
}

func inventoryPhysicalRow(name, path string) bool {
	return name != "" && cleanAbsolute(path) && filepath.Base(path) == name
}
