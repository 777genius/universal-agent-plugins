package providers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/filetree"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type Stager struct {
	LauncherSource  *managedstdio.Source
	SnapshotBuilder packagesnapshot.Builder
	PluginDataRoot  string
	// Registry supplies the client adapters that project a sanitized tree.
	// It is injected by the composition root and never defaulted to "every client".
	Registry *clients.Registry
	// Paths is required. There is deliberately no default: a silently supplied
	// one would let a caller that forgot to wire it keep running with whatever
	// containment rules that default happened to carry.
	Paths ports.PathPolicy
}

var errPathPolicyRequired = errors.New("stager path policy is required")

func (stager Stager) requireDeps() error {
	if stager.Paths == nil {
		return errPathPolicyRequired
	}
	if stager.Registry == nil {
		return clients.ErrRegistryRequired
	}
	return nil
}

func (stager Stager) stagingLayout(id domain.ClientID) clients.StagingLayout {
	if layout, ok := clients.As[clients.StagingLayout](stager.Registry, id); ok {
		return layout
	}
	return shared.DefaultStagingLayout{}
}

func (stager Stager) launcher() clients.StdioLauncherDeliverer {
	if stager.LauncherSource == nil {
		return nil
	}
	return stager.LauncherSource
}

func (stager Stager) Discard(ctx context.Context, delivery domain.StagedDelivery) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := stager.requireDeps(); err != nil {
		return err
	}
	plan := domain.DeliveryPlan{ClientID: delivery.ClientID, TargetRoot: delivery.OwnedBase}
	stagingBase := filepath.Clean(stager.stagingLayout(delivery.ClientID).StagingBase(plan))
	if filepath.Dir(filepath.Clean(delivery.StagingPath)) != stagingBase ||
		!strings.HasPrefix(filepath.Base(delivery.StagingPath), ".agentplugins-staging-") {
		return fmt.Errorf("refuse unsafe staged delivery cleanup")
	}
	return stager.removeStaging(stagingBase, delivery.StagingPath)
}

func (stager Stager) Verify(ctx context.Context, root, expectedDigest string) error {
	if strings.TrimSpace(expectedDigest) == "" {
		return fmt.Errorf("expected artifact digest is required")
	}
	if err := rejectExcludedOwnershipMarkers(root); err != nil {
		kind := ports.VerificationIndeterminate
		var marker *excludedOwnershipMarkerError
		if errors.As(err, &marker) {
			kind = ports.VerificationExcludedMarker
		} else if errors.Is(err, os.ErrNotExist) {
			kind = ports.VerificationAbsent
		}
		return &ports.VerificationError{Kind: kind, Err: err}
	}
	artifact, err := stager.SnapshotBuilder.Build(ctx, root)
	if err != nil {
		kind := ports.VerificationIndeterminate
		if errors.Is(err, os.ErrNotExist) {
			kind = ports.VerificationAbsent
		}
		return &ports.VerificationError{Kind: kind, Err: err}
	}
	digest := artifact.Digest
	if closeErr := artifact.Close(); closeErr != nil {
		return &ports.VerificationError{Kind: ports.VerificationIndeterminate, Err: closeErr}
	}
	if digest != expectedDigest {
		return &ports.VerificationError{Kind: ports.VerificationDigestMismatch, ActualDigest: digest, Err: fmt.Errorf("artifact digest mismatch")}
	}
	return nil
}

type excludedOwnershipMarkerError struct{ name string }

func (err *excludedOwnershipMarkerError) Error() string {
	return fmt.Sprintf("managed artifact contains excluded ownership marker %q", err.name)
}

func rejectExcludedOwnershipMarkers(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if filepath.Clean(path) == filepath.Clean(root) {
			return nil
		}
		if entry.Name() == ".git" || entry.Name() == ".plugin-kit-ai.lock" {
			return &excludedOwnershipMarkerError{name: entry.Name()}
		}
		return nil
	})
}

func (stager Stager) Stage(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
) (delivery domain.StagedDelivery, err error) {
	return stager.stage(ctx, envelope, plan, operationID, hints, stager.pluginDataPath(plan))
}

// StageWithPluginData binds projections to the exact owned locator recorded by
// lifecycle state. It is an optional extension to ports.PackageStager so older
// non-stdio test doubles remain source-compatible.
func (stager Stager) StageWithPluginData(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
	pluginDataPath string,
) (delivery domain.StagedDelivery, err error) {
	if strings.TrimSpace(pluginDataPath) == "" || !filepath.IsAbs(pluginDataPath) {
		return domain.StagedDelivery{}, fmt.Errorf("owned PLUGIN_DATA path must be absolute")
	}
	return stager.stage(ctx, envelope, plan, operationID, hints, filepath.Clean(pluginDataPath))
}

func (stager Stager) stage(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	operationID string,
	hints domain.CompatibilityHints,
	pluginDataPath string,
) (delivery domain.StagedDelivery, err error) {
	layout, err := stager.preflightStage(ctx, envelope, plan, operationID)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	stagingBase, stagingPath, err := stager.createStagingDir(plan, layout, operationID)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	defer func() {
		if err != nil {
			_ = stager.removeStaging(stagingBase, stagingPath)
		}
	}()
	if err = stager.materializeStaging(stagingPath, envelope, plan); err != nil {
		return domain.StagedDelivery{}, err
	}
	projected, err := stager.project(ctx, stagingPath, envelope, plan, hints, pluginDataPath)
	if err != nil {
		return domain.StagedDelivery{}, err
	}
	return stager.assembleDelivery(ctx, envelope, plan, stagingPath, projected)
}

func (stager Stager) preflightStage(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, operationID string) (clients.StagingLayout, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := stager.requireDeps(); err != nil {
		return nil, err
	}
	if plan.Status == domain.PlanUnsupported {
		return nil, fmt.Errorf("cannot stage an unsupported delivery plan")
	}
	if strings.TrimSpace(envelope.SnapshotRoot) == "" {
		return nil, fmt.Errorf("package snapshot root is required")
	}
	if err := stager.Paths.ValidateLeafID(operationID); err != nil {
		return nil, fmt.Errorf("invalid staging operation id: %w", err)
	}
	layout := stager.stagingLayout(plan.ClientID)
	if err := stager.validatePlanPaths(plan, layout); err != nil {
		return nil, err
	}
	if err := validateReservedStdioEnvironment(envelope, plan); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(plan.TargetRoot, 0o700); err != nil {
		return nil, fmt.Errorf("create client target root: %w", err)
	}
	if err := stager.validatePlanPaths(plan, layout); err != nil {
		return nil, err
	}
	return layout, nil
}

func (stager Stager) createStagingDir(plan domain.DeliveryPlan, layout clients.StagingLayout, operationID string) (string, string, error) {
	suffix := sha256.Sum256([]byte(operationID))
	stagingBase := layout.StagingBase(plan)
	if err := os.MkdirAll(stagingBase, 0o700); err != nil {
		return "", "", fmt.Errorf("create client staging root: %w", err)
	}
	stagingPath := filepath.Join(stagingBase, ".agentplugins-staging-"+hex.EncodeToString(suffix[:8]))
	if err := stager.Paths.RequireContainedChild(stagingBase, stagingPath); err != nil {
		return "", "", fmt.Errorf("unsafe staging path: %w", err)
	}
	if _, statErr := os.Lstat(stagingPath); statErr == nil {
		return "", "", fmt.Errorf("staging path already exists")
	} else if !os.IsNotExist(statErr) {
		return "", "", fmt.Errorf("inspect staging path: %w", statErr)
	}
	return stagingBase, stagingPath, nil
}

func (stager Stager) materializeStaging(stagingPath string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	if err := filetree.CopyDir(envelope.SnapshotRoot, stagingPath); err != nil {
		return fmt.Errorf("copy package snapshot to staging: %w", err)
	}
	return sanitizePackage(stager.Paths, stagingPath, envelope, plan)
}

func (stager Stager) assembleDelivery(
	ctx context.Context,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	stagingPath string,
	projected []domain.NativeObjectOwnership,
) (domain.StagedDelivery, error) {
	artifact, err := stager.SnapshotBuilder.Build(ctx, stagingPath)
	if err != nil {
		return domain.StagedDelivery{}, fmt.Errorf("verify staged artifact: %w", err)
	}
	artifactDigest := artifact.Digest
	if closeErr := artifact.Close(); closeErr != nil {
		return domain.StagedDelivery{}, fmt.Errorf("clean staged verification snapshot: %w", closeErr)
	}
	objects := make([]domain.NativeObjectOwnership, 0, 1+len(projected))
	objects = append(objects, domain.NativeObjectOwnership{
		ObjectID:        "package:" + string(plan.ClientID) + ":" + plan.PhysicalArtifactID,
		Kind:            "managed_package_directory",
		LogicalName:     envelope.Manifest.Name,
		Path:            plan.ActivePath,
		ManagedDigest:   artifactDigest,
		ProtectionClass: "managed",
	})
	objects = append(objects, projected...)
	return domain.StagedDelivery{
		ClientID:       plan.ClientID,
		OwnedBase:      plan.TargetRoot,
		ActivePath:     plan.ActivePath,
		StagingPath:    stagingPath,
		ArtifactDigest: artifactDigest,
		NativeObjects:  objects,
	}, nil
}

func (stager Stager) project(
	ctx context.Context,
	stagingPath string,
	envelope domain.PackageEnvelope,
	plan domain.DeliveryPlan,
	hints domain.CompatibilityHints,
	pluginDataPath string,
) ([]domain.NativeObjectOwnership, error) {
	projector, ok := clients.As[clients.Projector](stager.Registry, plan.ClientID)
	if !ok {
		if domain.RequiresNativeProjector(plan.ClientID) {
			return nil, fmt.Errorf("client %q requires a native projector", plan.ClientID)
		}
		return nil, nil
	}
	return projector.Project(ctx, clients.ProjectionInput{
		StagingPath:    stagingPath,
		Envelope:       envelope,
		Plan:           plan,
		Hints:          hints,
		PluginDataPath: pluginDataPath,
		Launcher:       stager.launcher(),
	})
}

func validateReservedStdioEnvironment(envelope domain.PackageEnvelope, plan domain.DeliveryPlan) error {
	for _, name := range domain.SelectedMCPNames(plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		switch env := server.Decoded["env"].(type) {
		case map[string]any:
			if _, ok := env["PLUGIN_ROOT"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_ROOT", name)
			}
			if _, ok := env["PLUGIN_DATA"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_DATA", name)
			}
		case map[string]string:
			if _, ok := env["PLUGIN_ROOT"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_ROOT", name)
			}
			if _, ok := env["PLUGIN_DATA"]; ok {
				return fmt.Errorf("stdio MCP server %s defines reserved PLUGIN_DATA", name)
			}
		}
	}
	return nil
}

func (stager Stager) pluginDataPath(plan domain.DeliveryPlan) string {
	base := stager.PluginDataRoot
	if strings.TrimSpace(base) == "" {
		base = filepath.Join(plan.TargetAnchor, "plugin-data")
	}
	return filepath.Join(base, plan.PhysicalArtifactID)
}

func (stager Stager) validatePlanPaths(plan domain.DeliveryPlan, layout clients.StagingLayout) error {
	if strings.TrimSpace(plan.TargetAnchor) == "" || strings.TrimSpace(plan.TargetRoot) == "" || strings.TrimSpace(plan.ActivePath) == "" {
		return fmt.Errorf("delivery plan target paths are incomplete")
	}
	if err := stager.Paths.RequireContainedChild(plan.TargetAnchor, plan.TargetRoot); err != nil {
		return fmt.Errorf("unsafe delivery target root: %w", err)
	}
	if err := layout.ValidateTargetLayout(plan); err != nil {
		return err
	}
	if filepath.Dir(filepath.Clean(plan.ActivePath)) != filepath.Clean(plan.TargetRoot) {
		return fmt.Errorf("delivery active path must be a direct child of target root")
	}
	if err := stager.Paths.RequireContainedChild(plan.TargetRoot, plan.ActivePath); err != nil {
		return fmt.Errorf("unsafe delivery active path: %w", err)
	}
	return nil
}

func (stager Stager) removeStaging(base, path string) error {
	if err := stager.Paths.RequireContainedChild(base, path); err != nil {
		return err
	}
	return os.RemoveAll(path)
}
