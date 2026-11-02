package usecase

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Readiness changes the selected plan, never source bytes or portable validity.
// Existing installations are retained atomically on local unavailability.
func (service Service) preflightComponents(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, preserveExisting bool, priorSelections ...map[string]bool) error {
	skipped, losesDelivered, err := service.preflightSelectedStdio(envelope, plan, preserveExisting, priorSelections)
	if err != nil {
		return err
	}
	if skipped {
		plan.Warnings = append(plan.Warnings, "components_skipped_local_readiness")
		if losesDelivered {
			return fmt.Errorf("local component readiness changed; retaining the entire installed package: %s", readinessMessages(*plan))
		}
	}
	if err := plan.SelectedDelivery.ValidatePlan(*plan, envelope.TreeDigest); err != nil {
		return err
	}
	return rejectEmptySupportedComponents(plan)
}

func (service Service) preflightSelectedStdio(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, preserveExisting bool, priorSelections []map[string]bool) (skipped, losesDelivered bool, err error) {
	probe := &stdioHelperProbe{}
	for _, name := range domain.SelectedMCPNames(*plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		failure, preflightErr := service.preflightOneStdio(envelope, plan, server, probe)
		if preflightErr != nil {
			return false, false, preflightErr
		}
		if failure == nil {
			continue
		}
		skipped = true
		if preserveExisting && (len(priorSelections) == 0 || priorSelections[0][name]) {
			losesDelivered = true
		}
		markStdioReadinessSkip(plan, name, failure)
	}
	return skipped, losesDelivered, nil
}

type stdioHelperProbe struct {
	checked bool
	err     error
}

func (service Service) preflightOneStdio(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, server domain.MCPServer, probe *stdioHelperProbe) (*domain.ComponentReadinessError, error) {
	var failure *domain.ComponentReadinessError
	if plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, plan.Scope).UsesManagedStdioLauncher {
		var err error
		failure, err = service.checkedManagedStdioHelper(envelope.SnapshotRoot, probe)
		if err != nil {
			return nil, err
		}
	}
	if failure != nil {
		return failure, nil
	}
	return service.evaluateStdioReadiness(envelope, plan, server)
}

func (service Service) checkedManagedStdioHelper(snapshotRoot string, probe *stdioHelperProbe) (*domain.ComponentReadinessError, error) {
	if !probe.checked {
		if checker, ok := service.Stager.(ports.ManagedStdioPreflighter); ok {
			probe.err = checker.PreflightManagedStdio(snapshotRoot)
		}
		probe.checked = true
	}
	var failure *domain.ComponentReadinessError
	if probe.err != nil && !errors.As(probe.err, &failure) {
		return nil, probe.err
	}
	return failure, nil
}

func (service Service) evaluateStdioReadiness(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, server domain.MCPServer) (*domain.ComponentReadinessError, error) {
	dataRoot, err := service.preflightStdioDataRoot(plan, server)
	if err != nil {
		return nil, err
	}
	var failure *domain.ComponentReadinessError
	readinessErr := stdioReadiness(envelope.SnapshotRoot, server.Decoded, dataRoot)
	if readinessErr != nil && !errors.As(readinessErr, &failure) {
		return nil, readinessErr
	}
	if failure != nil {
		return failure, nil
	}
	return service.preflightPluginDataCWD(envelope, plan, server)
}

func (service Service) preflightStdioDataRoot(plan *domain.DeliveryPlan, server domain.MCPServer) (string, error) {
	if !strings.Contains(authoredPATH(server.Decoded["env"]), "${PLUGIN_DATA}") {
		return "", nil
	}
	checker, ok := service.PluginData.(ports.DataPathPreflighter)
	if !ok {
		return "", nil
	}
	dataRoot, _, err := checker.PreflightDataPath(plan.PhysicalArtifactID)
	return dataRoot, err
}

func (service Service) preflightPluginDataCWD(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, server domain.MCPServer) (*domain.ComponentReadinessError, error) {
	cwd, _ := server.Decoded["cwd"].(string)
	parsed, _ := pathcontract.ParseCWD(cwd)
	if parsed.Anchor != pathcontract.Data {
		return nil, nil
	}
	checker, ok := service.PluginData.(ports.DataPathPreflighter)
	if !ok {
		return &domain.ComponentReadinessError{Code: "stdio_data_unavailable", Message: "readonly PLUGIN_DATA readiness is unavailable"}, nil
	}
	root, exists, err := checker.PreflightDataPath(plan.PhysicalArtifactID)
	if err != nil {
		return nil, err
	}
	if !exists && strings.Trim(parsed.Relative, "./") == "" && !strings.Contains(parsed.Relative, "..") {
		// EnsureData creates the owned root, but never authored subdirectories.
		return nil, nil
	}
	return inspectPluginDataCWD(cwd, envelope.SnapshotRoot, root)
}

func inspectPluginDataCWD(cwd, snapshotRoot, root string) (*domain.ComponentReadinessError, error) {
	parsed, parseErr := pathcontract.ExpandCWD(cwd, snapshotRoot, root)
	if parseErr != nil {
		return nil, parseErr
	}
	resolved := pathcontract.Resolve(root, parsed.Relative)
	if unexpectedPathIO(resolved.Err) {
		return nil, resolved.Err
	}
	if resolved.State != pathcontract.Resolved {
		return &domain.ComponentReadinessError{Code: "stdio_data_cwd_unavailable", Message: fmt.Sprintf("PLUGIN_DATA cwd is unavailable: %v", resolved.Err)}, nil
	}
	info, err := os.Stat(resolved.Path)
	if err != nil || !info.IsDir() {
		if unexpectedPathIO(err) {
			return nil, err
		}
		return &domain.ComponentReadinessError{Code: "stdio_data_cwd_unavailable", Message: "PLUGIN_DATA cwd is not a directory"}, nil
	}
	return nil, nil
}

func markStdioReadinessSkip(plan *domain.DeliveryPlan, name string, failure *domain.ComponentReadinessError) {
	for i := range plan.Components {
		c := &plan.Components[i]
		if c.Kind == domain.ComponentMCPServer && c.Name == name {
			c.Support = domain.SupportUnsupported
			c.Reason = failure.Code
		}
	}
	plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer, Item: name, Code: failure.Code, Message: failure.Message})
}

func rejectEmptySupportedComponents(plan *domain.DeliveryPlan) error {
	if facts, ok := plan.SelectedDelivery.LocalFacts(); ok && facts.NativeStop {
		return nil
	}
	supported := false
	for _, c := range plan.Components {
		if c.Support != domain.SupportUnsupported {
			supported = true
		}
	}
	if supported || len(plan.Components) == 0 {
		return nil
	}
	plan.Status = domain.PlanUnsupported
	plan.Activation = domain.ActivationFailed
	plan.Warnings = append(plan.Warnings, "no_supported_components")
	return fmt.Errorf("no_supported_components: no components can be delivered; %s", readinessMessages(*plan))
}
