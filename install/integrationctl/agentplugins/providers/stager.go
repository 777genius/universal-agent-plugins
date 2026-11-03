package providers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	icdomain "github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
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
	// Installed verification reads fresh bytes and does not depend on temporary
	// snapshot creation, sealing, or cleanup.
	digest, err := stager.SnapshotBuilder.Digest(ctx, root)
	if err != nil {
		kind := ports.VerificationIndeterminate
		if errors.Is(err, os.ErrNotExist) {
			kind = ports.VerificationAbsent
		}
		return &ports.VerificationError{Kind: kind, Err: err}
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
		if entry.Name() == ".git" || icdomain.IsWorkspaceLockFile(entry.Name()) {
			return &excludedOwnershipMarkerError{name: entry.Name()}
		}
		return nil
	})
}
