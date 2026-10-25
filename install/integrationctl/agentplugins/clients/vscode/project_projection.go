package vscode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Projector             = (*ProjectAdapter)(nil)
	_ clients.ActiveNativeProjector = (*ProjectAdapter)(nil)
)

func (*ProjectAdapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateWorkspaceProjection(in.StagingPath, in.Plan, false); err != nil {
		return nil, err
	}
	// This is an internal portable staging document. The native destination is
	// independently encoded by CodecVSCode into .vscode/mcp.json's servers.
	if err := shared.ProjectMCPServers(shared.MCPProjection{Root: in.StagingPath, Envelope: in.Envelope,
		Names: shared.SupportedMCPNames(in.Plan), Dialect: shared.MCPDialectCursor, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath}); err != nil {
		return nil, err
	}
	if err := projectWorkspaceStdio(in); err != nil {
		return nil, err
	}
	return workspaceObjects(in.StagingPath, in.Envelope, in.Plan)
}

func (*ProjectAdapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return nil, err
	}
	if err := validateWorkspaceProjection(root, plan, true); err != nil {
		return nil, err
	}
	if err := validateWorkspaceStdio(root, envelope, plan, dataPath); err != nil {
		return nil, err
	}
	return workspaceObjects(root, envelope, plan)
}

func validateWorkspaceProjection(root string, plan domain.DeliveryPlan, active bool) error {
	if _, err := workspacePlanRoot(domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return err
	}
	if active && root != plan.ActivePath || !active && (filepath.Dir(root) != plan.TargetRoot || !strings.HasPrefix(filepath.Base(root), ".agentplugins-staging-")) {
		return fmt.Errorf("VS Code package path differs from selected staging/active workspace")
	}
	if err := pathpolicy.RequireContainedChild(plan.TargetContext.ProjectRoot, root); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("VS Code package must be an ordinary workspace directory")
	}
	return nil
}
