package copilot

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Folder trust is a remembered user choice, not plugin ownership. It therefore
// remains after removal; only a failed installation compensates a new grant.
func copilotTrustPath(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	root, err := copilotPlanRoot(client, plan)
	if err != nil {
		return "", err
	}
	profile := plan.TargetContext.UserConfigRoot
	for _, path := range []string{root, profile} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return "", fmt.Errorf("copilot folder trust requires canonical absolute paths")
		}
		if err := pathpolicy.RequireContainedChild(filepath.VolumeName(path)+string(filepath.Separator), path); err != nil {
			return "", err
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("copilot folder trust requires a known home directory")
	}
	if canonical, err := filepath.EvalSymlinks(home); err == nil {
		home = canonical
	}
	for _, protected := range []string{filepath.Clean(home), profile} {
		rel, err := filepath.Rel(root, protected)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel) {
			return "", fmt.Errorf("copilot folder trust refuses a home or profile directory or its ancestor")
		}
	}
	if root == filepath.VolumeName(root)+string(filepath.Separator) {
		return "", fmt.Errorf("copilot folder trust refuses a filesystem root")
	}
	path := filepath.Join(profile, "config.json")
	return path, pathpolicy.RequireContainedChild(profile, path)
}

func copilotTrustDocument(file nativeconfig.FileSnapshot, root string) ([]byte, bool, error) {
	if len(file.Body) > 1<<20 {
		return nil, false, fmt.Errorf("copilot trust document exceeds inspection limit")
	}
	if file.Exists && len(file.Body) == 0 {
		return nil, false, fmt.Errorf("copilot folder trust refuses an empty existing config")
	}
	return nativeconfig.CopilotTrustProject(file.Body, root)
}

func verifyCopilotTrust(env clients.Env, request domain.ActivationRequest, required bool) error {
	path, err := copilotTrustPath(request.Client, request.Plan)
	if err != nil {
		return err
	}
	file, err := env.NativeConfig.ReadExactFileBounded(path, 1<<20)
	if err != nil {
		return err
	}
	_, trusted, err := copilotTrustDocument(file, request.Plan.TargetContext.ProjectRoot)
	if err == nil && required && !trusted {
		return fmt.Errorf("copilot has not remembered the selected project folder; rerun installation to restore folder trust")
	}
	return err
}

func withCopilotTrust(ctx context.Context, env clients.Env, request domain.ActivationRequest, apply func() error) error {
	path, err := copilotTrustPath(request.Client, request.Plan)
	if err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	file, err := env.NativeConfig.BeginExactFile(path)
	if err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	next, trusted, err := copilotTrustDocument(file.Original(), request.Plan.TargetContext.ProjectRoot)
	componentsStarted := false
	if err == nil {
		err = ctx.Err()
	}
	if err == nil && !trusted {
		err = file.Apply(next)
	}
	if err == nil {
		componentsStarted = true
		err = apply()
	}
	if err == nil || nativeconfig.IsCommittedCleanup(err) && copilotComponentFailureEffect(err) != domain.NativeEffectUncertain {
		if closeErr := file.Close(); closeErr != nil {
			return &nativeconfig.CommittedCleanupError{Err: errors.Join(err, closeErr)}
		}
		return err
	}
	rollback := file.Rollback()
	closeErr := file.Close()
	if rollback != nil || file.Effect() != nativeconfig.FileUnchanged {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: errors.Join(err, rollback, closeErr)}
	}
	effect := domain.NativeEffectUnchanged
	if componentsStarted {
		effect = copilotComponentFailureEffect(err)
	}
	return &shared.NativeEffectError{Effect: effect, Err: errors.Join(err, closeErr)}
}

func copilotComponentFailureEffect(err error) domain.NativeEffectState {
	var observed *shared.NativeEffectError
	if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
		return domain.NativeEffectUnchanged
	}
	if observed == nil && nativeconfig.IsCommittedCleanup(err) {
		return domain.NativeEffectCommitted
	}
	return domain.NativeEffectUncertain
}
