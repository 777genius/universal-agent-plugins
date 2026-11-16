package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func completeCodexUserDiscovery(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if len(request.Plan.Components) == 0 {
		return outcome, nil
	}
	if !codexNativeDiscoverySupported(request) {
		return pendingCodexUserDiscovery(outcome, fmt.Errorf("selected remote or unsupported components require separate native verification"))
	}
	runner, ok := env.Runner.(ports.CompletionCommandRunner)
	if !ok {
		return pendingCodexUserDiscovery(outcome, fmt.Errorf("native interactive runner is unavailable"))
	}
	command, err := codexCommand(request.Client.ConfigRoot, request.BackendExecutable, os.Environ(), "app-server")
	if err != nil {
		return pendingCodexUserDiscovery(outcome, err)
	}
	bounded, cancel := context.WithTimeout(ctx, codexProjectDiscoveryTimeout)
	defer cancel()
	version, err := codexNativeUserVersion(bounded, env, request)
	if err != nil {
		return pendingCodexUserDiscovery(outcome, err)
	}
	command.Dir = request.Delivery.ActivePath
	err = runner.RunDuplexToCompletion(bounded, command, func(stdin io.Writer, stdout io.Reader) error {
		return exchangeCodexUserDiscovery(stdin, stdout, request, version)
	})
	if err != nil {
		return pendingCodexUserDiscovery(outcome, err)
	}
	if err := bounded.Err(); err != nil {
		return pendingCodexUserDiscovery(outcome, err)
	}
	outcome.Authentication = codexNativeAuthentication(request.Plan)
	return outcome, nil
}

func pendingCodexUserDiscovery(outcome domain.ActivationOutcome, err error) (domain.ActivationOutcome, error) {
	outcome.Activation = domain.ActivationPrepared
	err = shared.CompletionProbeFailure(err)
	if domain.IsUnclosedClientProbe(err) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, err
	}
	outcome.LocalActions = append(outcome.LocalActions, "Codex registered the plugin but native component readiness is unconfirmed: "+codexDiscoveryReason(err))
	return outcome, nil
}

// codexDiscoveryReason keeps the human next action on one readable line. The
// runner joins every cleanup cause, so a timeout otherwise prints a multi-line
// chain of repeated deadline errors and kill signals.
func codexDiscoveryReason(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("Codex did not finish native discovery within %s; start a new Codex session and check the plugin's skills and MCP servers", codexProjectDiscoveryTimeout)
	}
	return strings.Join(strings.Fields(err.Error()), " ")
}

func exchangeCodexUserDiscovery(stdin io.Writer, stdout io.Reader, request domain.ActivationRequest, version string) error {
	reader := bufio.NewScanner(io.LimitReader(stdout, (4<<20)+1))
	reader.Buffer(make([]byte, 4096), 2<<20)
	writer := json.NewEncoder(stdin)
	nativeAgent, err := initializeCodexDiscovery(reader, writer, request.Client.ConfigRoot)
	if err != nil {
		return err
	}
	if err := writer.Encode(map[string]any{"id": 3, "method": "skills/list", "params": map[string]any{"cwds": []string{request.Delivery.ActivePath}, "forceReload": true}}); err != nil {
		return err
	}
	listing, err := codexDiscoveryResponse(reader, "3")
	if err != nil {
		return err
	}
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	pluginID := request.DeclaredName + "@" + marketplace
	if err := verifyCodexUserSkills(listing, request, marketplace, pluginID, version); err != nil {
		return err
	}
	return verifyCodexNativeMCP(reader, writer, request.Plan, pluginID, nativeAgent)
}

func verifyCodexUserSkills(listing map[string]any, request domain.ActivationRequest, marketplace, pluginID, version string) error {
	groups, ok := listing["data"].([]any)
	if !ok || len(groups) != 1 {
		return fmt.Errorf("codex returned no selected user plugin skill inventory")
	}
	group, ok := groups[0].(map[string]any)
	if !ok || !codexDiscoveryPathMatches(group["cwd"], request.Delivery.ActivePath) {
		return fmt.Errorf("codex returned another user plugin context")
	}
	skillErrors, ok := group["errors"].([]any)
	if !ok || len(skillErrors) != 0 {
		return fmt.Errorf("codex reported user plugin skill errors")
	}
	skills, ok := group["skills"].([]any)
	if !ok {
		return fmt.Errorf("codex returned no user skill collection")
	}
	wanted := map[string]string{}
	for _, component := range request.Plan.Components {
		if component.Kind != domain.ComponentSkill || component.Support == domain.SupportUnsupported {
			continue
		}
		wanted[request.DeclaredName+":"+component.Name] = filepath.Join(request.Client.ConfigRoot, "plugins", "cache", marketplace, request.DeclaredName, version, "skills", component.Name, "SKILL.md")
	}
	return verifyCodexSkillEntries(skills, wanted, "user", pluginID)
}
