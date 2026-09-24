package codex

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const codexCLIName = "Codex CLI"

var (
	_ clients.Lifecycle          = (*Adapter)(nil)
	_ clients.AutomaticActivator = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier   = (*Adapter)(nil)
)

// AutomaticallyActivates reports that Activate will drive the Codex CLI.
func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return shared.HasClientCLI(env, request.BackendExecutable)
}

// VerifierAvailable reports that an exact Codex listing can be observed.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return shared.HasListingCLI(backendExecutable)
}

// Activate registers the managed marketplace and plugin through the Codex CLI,
// or leaves a manual follow-up when that CLI is missing.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, "install the prepared plugin in Codex Plugins, then verify it appears in Plugins > Personal")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("in Codex, install %s from %s, then verify it appears in Plugins > Personal", request.DeclaredName, request.Delivery.ActivePath))
		return outcome, nil
	}
	profileRoot := request.Client.ConfigRoot
	if profileRoot == "" {
		profileRoot = request.Plan.NativeRegistryRoot
	}
	if err := validateProfile(profileRoot, request.Plan.NativeRegistryRoot); err != nil {
		return domain.ActivationOutcome{}, err
	}
	request.Client.ConfigRoot = profileRoot
	return completeCodexActivation(ctx, env, request, outcome)
}

func completeCodexActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	if request.VerifyOnly {
		err = verifyCodexPlugin(ctx, env, request)
	} else {
		err = activateCodexPlugin(ctx, env, request)
	}
	if err == nil {
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	}
	if errors.Is(err, ErrListContractUnknown) {
		return manualCodexVerification(outcome, request), nil
	}
	next := fmt.Sprintf("verify with `%s plugin list --json`", request.BackendExecutable)
	if !request.VerifyOnly {
		next = fmt.Sprintf("run Codex activation again for the prepared package at %s, then verify with `%s plugin list --json`", request.Delivery.ActivePath, request.BackendExecutable)
	}
	return shared.FailedActivation(outcome, next, err)
}

func activateCodexPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	if err := registerCodexMarketplace(ctx, env, request, marketplace); err != nil {
		return err
	}
	pluginSpec := request.DeclaredName + "@" + marketplace
	if _, err := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "add", pluginSpec, "--json"); err != nil {
		if !request.Replacing {
			_, _ = runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace, "--json")
		}
		return fmt.Errorf("activate Codex plugin: %w", err)
	}
	return verifyCodexPlugin(ctx, env, request)
}

func registerCodexMarketplace(ctx context.Context, env clients.Env, request domain.ActivationRequest, marketplace string) error {
	if request.Replacing {
		if _, err := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "marketplace", "update", marketplace, "--json"); err != nil {
			if _, fallbackErr := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); fallbackErr != nil {
				return fmt.Errorf("refresh Codex marketplace: %w; fallback registration: %w", err, fallbackErr)
			}
		}
		return nil
	}
	if _, err := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); err != nil {
		return fmt.Errorf("register Codex marketplace: %w", err)
	}
	return nil
}

func verifyCodexPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	pluginSpec := request.DeclaredName + "@" + marketplace
	var listed []byte
	for attempt := range 3 {
		result, err := runCodex(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugin", "list", "--json")
		if err == nil {
			listed = result.Stdout
			break
		}
		if attempt == 2 || !removedCodexBackupPath(err) {
			return fmt.Errorf("verify Codex plugin listing: %w", err)
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 50 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("verify Codex plugin listing: %w", ctx.Err())
		case <-timer.C:
		}
	}
	switch PluginStatusFromList(listed, request.DeclaredName, marketplace) {
	case StatusInstalled:
		return nil
	case StatusAbsent:
		return fmt.Errorf("%w: verify Codex plugin listing: %s is not installed and enabled", shared.ErrRecognizedNegativeEvidence, pluginSpec)
	default:
		return fmt.Errorf("%w: verify Codex plugin listing", ErrListContractUnknown)
	}
}

// Codex may briefly retain an executable path inside its just-removed plugin
// backup while a marketplace replacement settles. Only retry this exact
// read-only observation; a missing active executable must still fail closed.
func removedCodexBackupPath(err error) bool {
	if !errors.Is(err, os.ErrNotExist) {
		return false
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	for path := filepath.Clean(pathErr.Path); path != "."; path = filepath.Dir(path) {
		if strings.HasPrefix(filepath.Base(path), "plugin-backup-") {
			return true
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
	}
	return false
}

func manualCodexVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the managed plugin is installed and enabled in Codex"}
	outcome.LocalActions = []string{fmt.Sprintf("the `%s plugin list --json` output contract was not recognized; inspect %s manually", request.BackendExecutable, request.DeclaredName)}
	return outcome
}
