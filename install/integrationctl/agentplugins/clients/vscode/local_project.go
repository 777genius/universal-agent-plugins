package vscode

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodelocalhooks"
)

const declaredPluginRoot = "/TEST-declared-plugin"
const declaredPluginData = "/TEST-declared-data"

func localHash(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func resolvedSpecs(c LocalConfig, plugin, data string) ([]vscodelocalhooks.Spec, error) {
	specs := cloneSpecs(c.HookSpecs)
	// NewReplacer processes the authored input once: tokens appearing inside a
	// trusted physical root are not recursively interpreted.
	replace := strings.NewReplacer("${PLUGIN_ROOT}", plugin, "${PLUGIN_DATA}", data)
	for i := range specs {
		if strings.Contains(specs[i].Executable, "${") {
			return nil, fmt.Errorf("local runtime executable must be fixed")
		}
		for j, arg := range specs[i].Args {
			remainder := strings.NewReplacer("${PLUGIN_ROOT}", "", "${PLUGIN_DATA}", "").Replace(arg)
			if strings.Contains(remainder, "${") {
				return nil, fmt.Errorf("local undeclared argument token")
			}
			specs[i].Args[j] = replace.Replace(arg)
		}
	}
	return specs, nil
}

func renderSpecs(c LocalConfig, plugin, data string) ([]byte, error) {
	specs, err := resolvedSpecs(c, plugin, data)
	if err != nil {
		return nil, err
	}
	return vscodelocalhooks.Render(c.TargetShell, specs)
}

func readLocalHook(root string) ([]byte, error) {
	anchor, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer func() { _ = anchor.Close() }()
	path := filepath.FromSlash(vscodelocalhooks.PluginPath)
	info, err := anchor.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > vscodelocalhooks.MaxDocumentBytes {
		return nil, fmt.Errorf("local hook must be a bounded regular file")
	}
	file, err := anchor.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	body, err := io.ReadAll(io.LimitReader(file, vscodelocalhooks.MaxDocumentBytes+1))
	if len(body) > vscodelocalhooks.MaxDocumentBytes {
		return nil, fmt.Errorf("local hook exceeds limit")
	}
	return body, err
}

func (a *LocalAdapter) verifyDeclaredHooks(envelope domain.PackageEnvelope) error {
	body, err := readLocalHook(envelope.SnapshotRoot)
	if os.IsNotExist(err) && !a.config.NativeStop && a.config.DeclaredHookDigest == "" {
		return nil
	}
	if err != nil {
		return err
	}
	if localHash(body) != a.config.DeclaredHookDigest {
		return fmt.Errorf("local declared hook digest differs from canonical source")
	}
	if !a.config.NativeStop {
		return nil
	}
	specs, err := resolvedSpecs(a.config, declaredPluginRoot, declaredPluginData)
	if err != nil {
		return err
	}
	normalized := strings.NewReplacer("${PLUGIN_ROOT}", declaredPluginRoot, "${PLUGIN_DATA}", declaredPluginData).Replace(string(body))
	return vscodelocalhooks.VerifyOwned([]byte(normalized), a.config.TargetShell, specs)
}

func (a *LocalAdapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	facts, err := a.validateProjection(in.Envelope, in.Plan)
	if err != nil {
		return nil, err
	}
	if err := a.verifyDeclaredHooks(in.Envelope); err != nil {
		return nil, err
	}
	if facts.NativeStop && (!cleanLocalPath(in.Plan.ActivePath) || !cleanLocalPath(in.PluginDataPath)) {
		return nil, fmt.Errorf("local projection requires fixed active/data roots")
	}
	anchor, err := os.OpenRoot(in.StagingPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = anchor.Close() }()
	if err := removeUnselectedSkills(anchor, in.Envelope, facts.Skills); err != nil {
		return nil, err
	}
	if !facts.NativeStop {
		if err := anchor.Remove(filepath.FromSlash(vscodelocalhooks.PluginPath)); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	} else if err := a.writeProjectedHooks(anchor, in); err != nil {
		return nil, err
	}
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	if err := projectLocalMCP(in, facts.MCPServers); err != nil {
		return nil, err
	}
	return localProjectionObjects(facts), nil
}

// Projection describes requested contributions, not retained ownership. The
// existing lifecycle carries previous native objects across unchanged effects;
// an empty route cannot manufacture selector authority from profile values.
func localProjectionObjects(facts domain.LocalDeliveryFacts) []domain.NativeObjectOwnership {
	if !facts.NativeStop && len(facts.MCPServers) == 0 && len(facts.Skills) == 0 {
		return nil
	}
	return []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
}

func removeUnselectedSkills(anchor *os.Root, envelope domain.PackageEnvelope, names []string) error {
	for name := range envelope.Skills {
		if slices.Contains(names, name) {
			continue
		}
		if !selectedNames([]string{name}) {
			return fmt.Errorf("local skill identity is unsafe")
		}
		if err := anchor.RemoveAll(filepath.Join("skills", name)); err != nil {
			return err
		}
	}
	return nil
}

func (a *LocalAdapter) writeProjectedHooks(anchor *os.Root, in clients.ProjectionInput) error {
	body, err := readLocalHook(in.StagingPath)
	if err != nil {
		return err
	}
	if localHash(body) != a.config.DeclaredHookDigest {
		return fmt.Errorf("local staged canonical hook differs")
	}
	rendered, err := renderSpecs(a.config, in.Plan.ActivePath, in.PluginDataPath)
	if err != nil {
		return err
	}
	file, err := anchor.OpenFile(filepath.FromSlash(vscodelocalhooks.PluginPath), os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(rendered)
	return errorsJoinClose(writeErr, file.Close())
}

func (a *LocalAdapter) validateProjection(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (domain.LocalDeliveryFacts, error) {
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return domain.LocalDeliveryFacts{}, err
	}
	facts, err := recordedLocalFacts(plan.SelectedDelivery)
	if err != nil {
		return facts, err
	}
	if facts.SettingsPath != a.config.ProfileSettingsPath || facts.Tuple != a.config.QualifiedTuple || facts.NativeStop != a.config.NativeStop || !slices.Equal(facts.MCPServers, a.config.MCPServers) || !slices.Equal(facts.Skills, a.config.Skills) {
		return facts, fmt.Errorf("local projection differs from frozen constructor")
	}
	return facts, nil
}

func (a *LocalAdapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, data string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	facts, err := a.validateProjection(envelope, plan)
	if err != nil {
		return nil, err
	}
	if root != plan.ActivePath {
		return nil, fmt.Errorf("local active projection root differs")
	}
	if facts.NativeStop {
		body, err := readLocalHook(root)
		if err != nil {
			return nil, err
		}
		specs, err := resolvedSpecs(a.config, root, data)
		if err != nil {
			return nil, err
		}
		if err := vscodelocalhooks.VerifyOwned(body, a.config.TargetShell, specs); err != nil {
			return nil, err
		}
	}
	return localProjectionObjects(facts), nil
}
