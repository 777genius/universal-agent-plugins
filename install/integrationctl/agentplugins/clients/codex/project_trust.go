package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/codextoml"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Trust remains a persistent user preference after plugin removal. Only a
// confirmed installation may grant it; native verification stays read-only.
func grantCodexProjectTrust(env clients.Env, request domain.ActivationRequest) error {
	project, err := codexProjectPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if request.VerifyOnly {
		return nil
	}
	if request.Delivery.ArtifactDigest == "" {
		return fmt.Errorf("codex project trust requires a verified delivery")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(home); resolveErr == nil {
		home = resolved
	}
	if project == home || filepath.Dir(project) == project {
		return fmt.Errorf("codex project trust cannot grant a home or filesystem root")
	}
	profile := request.Plan.TargetContext.UserConfigRoot
	if err := validateProfile(profile, ""); err != nil {
		return err
	}
	if err := os.MkdirAll(profile, 0700); err != nil {
		return err
	}
	_, err = shared.UpdateProjectTrustFileWithInitial(env.NativeConfig, filepath.Join(profile, "config.toml"), nil, func(body []byte) ([]byte, error) { return codextoml.TrustProject(body, project) })
	return err
}

func pendingCodexProjectTrust(outcome domain.ActivationOutcome, err error) (domain.ActivationOutcome, error) {
	outcome.LocalActions = append(outcome.LocalActions, "Codex exact project trust could not be persisted: "+err.Error())
	return outcome, nil
}
