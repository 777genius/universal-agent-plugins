package claude

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type claudeMCPHealth uint8

const (
	claudeMCPHealthUnknown claudeMCPHealth = iota
	claudeMCPHealthConnected
	claudeMCPHealthUnready
)

type claudeMCPObservation struct {
	state     claudeMCPHealth
	transport string
}

func observeClaudeMCPHealth(body []byte, selector string) claudeMCPObservation {
	if len(body) > 1<<20 || strings.ContainsAny(string(body), "\x00\x1b") {
		return claudeMCPObservation{}
	}
	lines := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")
	if len(lines) < 4 || lines[0] != selector+":" || lines[1] != "  Scope: Dynamic config (from command line)" {
		return claudeMCPObservation{}
	}
	state := claudeMCPHealthUnknown
	statuses, types := 0, 0
	transport := ""
	for _, line := range lines[2:] {
		if strings.HasPrefix(line, "  Status: ") {
			statuses++
			switch line {
			case "  Status: ✔ Connected":
				state = claudeMCPHealthConnected
			case "  Status: ✘ Failed to connect", "  Status: ⏸ Pending approval (run `claude` to approve)", "  Status: ✘ Rejected (see disabledMcpjsonServers in settings)", "  Status: ⚠ Needs authentication":
				state = claudeMCPHealthUnready
			default:
				return claudeMCPObservation{}
			}
		}
		if strings.HasPrefix(line, "  Type: ") {
			types++
			transport = strings.TrimPrefix(line, "  Type: ")
			if line != "  Type: stdio" && line != "  Type: http" && line != "  Type: sse" {
				return claudeMCPObservation{}
			}
		}
	}
	// Claude 2.1.263 omits transport details for default stdio entries. Accept
	// only its complete three-line response, without inferring a transport.
	legacy := types == 0 && len(lines) == 4 && lines[3] == ""
	if statuses != 1 || types > 1 || types == 0 && !legacy {
		return claudeMCPObservation{}
	}
	return claudeMCPObservation{state: state, transport: transport}
}

func finishClaudeMCPActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	names := shared.SupportedMCPNames(request.Plan)
	if len(names) == 0 {
		return qualifyClaudeNativeSkillAuthentication(request.Plan, outcome), nil
	}
	unique, err := inspectClaudeMCPNamespace(ctx, env, request)
	if err != nil {
		return failedClaudeMCPProbe(outcome, "inspect the selected Claude Code MCP namespace", err)
	}
	if !unique {
		outcome.Activation = domain.ActivationPrepared
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Claude Code does not expose one unique enabled plugin named "+request.DeclaredName+"; review same-name plugin precedence before using its MCP servers")
		return outcome, nil
	}
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		return outcome, err
	}
	allLocal := true
	for _, name := range names {
		selector := "plugin:" + request.DeclaredName + ":" + name
		changed, err := grantClaudeMCPApproval(env, request, selector)
		observeClaudeSetupEffect(&outcome.NativeEffect, changed, err)
		if err != nil {
			return shared.FailedActivation(outcome, "retry exact Claude Code MCP approval", err)
		}
		command := probe.Command
		command.Argv = []string{command.Argv[0], "mcp", "get", selector}
		result, err := RunClaudeListCommand(ctx, env.Runner, command)
		if err != nil {
			return failedClaudeMCPProbe(outcome, "retry exact Claude Code MCP health", err)
		}
		if result.ExitCode != 0 {
			return shared.FailedActivation(outcome, "inspect the selected Claude Code MCP server", fmt.Errorf("claude Code MCP health command failed with exit %d", result.ExitCode))
		}
		observation := observeClaudeMCPHealth(result.Stdout, selector)
		allLocal = allLocal && claudeMCPIsLocal(ctx, request, name, observation)
		switch observation.state {
		case claudeMCPHealthConnected:
		case claudeMCPHealthUnready:
			outcome.Activation = domain.ActivationPrepared
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Claude Code has not connected the selected MCP server "+selector+"; review its existing rejection, policy or authentication")
		default:
			return shared.FailedActivation(outcome, "inspect exact Claude Code MCP health output", fmt.Errorf("claude Code MCP health output is not recognized"))
		}
	}
	return qualifyClaudeNativeAuthentication(request.Plan, outcome, allLocal), nil
}

func failedClaudeMCPProbe(outcome domain.ActivationOutcome, action string, err error) (domain.ActivationOutcome, error) {
	if domain.IsUnclosedClientProbe(err) {
		outcome.NativeEffect = domain.NativeEffectUncertain
	}
	return shared.FailedActivation(outcome, action, err)
}
