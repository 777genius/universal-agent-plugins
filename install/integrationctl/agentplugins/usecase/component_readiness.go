package usecase

import (
	"errors"
	"fmt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

type dataPathPreflighter interface {
	PreflightDataPath(string) (string, bool, error)
}

type managedStdioPreflighter interface{ PreflightManagedStdio(string) error }

// Readiness changes the selected plan, never source bytes or portable validity.
// Existing installations are retained atomically on local unavailability.
func (service Service) preflightComponents(envelope domain.PackageEnvelope, plan *domain.DeliveryPlan, preserveExisting bool, priorSelections ...map[string]bool) error {
	var helperErr error
	helperChecked, skipped, losesDelivered := false, false, false
	for _, name := range domain.SelectedMCPNames(*plan) {
		server := envelope.MCP.Servers[name]
		if server.Type != "stdio" {
			continue
		}
		var failure *domain.ComponentReadinessError
		if plan.ClientID == domain.ClientWindsurf || plan.ClientID == domain.ClientClaude {
			if !helperChecked {
				if checker, ok := service.Stager.(managedStdioPreflighter); ok {
					helperErr = checker.PreflightManagedStdio(envelope.SnapshotRoot)
				}
				helperChecked = true
			}
			if helperErr != nil && !errors.As(helperErr, &failure) {
				return helperErr
			}
		}
		if failure == nil {
			dataRoot := ""
			if strings.Contains(authoredPATH(server.Decoded["env"]), "${PLUGIN_DATA}") {
				if checker, ok := service.PluginData.(dataPathPreflighter); ok {
					var err error
					dataRoot, _, err = checker.PreflightDataPath(plan.PhysicalArtifactID)
					if err != nil {
						return err
					}
				}
			}
			readinessErr := stdioReadiness(envelope.SnapshotRoot, server.Decoded, dataRoot)
			if readinessErr != nil && !errors.As(readinessErr, &failure) {
				return readinessErr
			}
			if failure == nil {
				cwd, _ := server.Decoded["cwd"].(string)
				parsed, _ := pathcontract.ParseCWD(cwd)
				if parsed.Anchor == pathcontract.Data {
					checker, ok := service.PluginData.(dataPathPreflighter)
					if !ok {
						failure = &domain.ComponentReadinessError{Code: "stdio_data_unavailable", Message: "readonly PLUGIN_DATA readiness is unavailable"}
					} else {
						root, exists, err := checker.PreflightDataPath(plan.PhysicalArtifactID)
						if err != nil {
							return err
						}
						if !exists && strings.Trim(parsed.Relative, "./") == "" && !strings.Contains(parsed.Relative, "..") {
							// EnsureData creates the owned root, but never authored subdirectories.
						} else {
							parsed, parseErr := pathcontract.ExpandCWD(cwd, envelope.SnapshotRoot, root)
							if parseErr != nil {
								return parseErr
							}
							resolved := pathcontract.Resolve(root, parsed.Relative)
							if unexpectedPathIO(resolved.Err) {
								return resolved.Err
							}
							if resolved.State != pathcontract.Resolved {
								failure = &domain.ComponentReadinessError{Code: "stdio_data_cwd_unavailable", Message: fmt.Sprintf("PLUGIN_DATA cwd is unavailable: %v", resolved.Err)}
							} else if info, err := os.Stat(resolved.Path); err != nil || !info.IsDir() {
								if unexpectedPathIO(err) {
									return err
								}
								failure = &domain.ComponentReadinessError{Code: "stdio_data_cwd_unavailable", Message: "PLUGIN_DATA cwd is not a directory"}
							}
						}
					}
				}
			}
		}
		if failure == nil {
			continue
		}
		skipped = true
		if preserveExisting && (len(priorSelections) == 0 || priorSelections[0][name]) {
			losesDelivered = true
		}
		for i := range plan.Components {
			c := &plan.Components[i]
			if c.Kind == domain.ComponentMCPServer && c.Name == name {
				c.Support = domain.SupportUnsupported
				c.Reason = failure.Code
			}
		}
		plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer, Item: name, Code: failure.Code, Message: failure.Message})
	}
	if skipped {
		plan.Warnings = append(plan.Warnings, "components_skipped_local_readiness")
		if losesDelivered {
			return fmt.Errorf("local component readiness changed; retaining the entire installed package: %s", readinessMessages(*plan))
		}
	}
	supported := false
	for _, c := range plan.Components {
		if c.Support != domain.SupportUnsupported {
			supported = true
		}
	}
	if !supported && len(plan.Components) > 0 {
		plan.Status = domain.PlanUnsupported
		plan.Activation = domain.ActivationFailed
		plan.Warnings = append(plan.Warnings, "no_supported_components")
		messages := []string{}
		for _, d := range plan.Diagnostics {
			if d.Boundary == domain.BoundaryMCPServer {
				messages = append(messages, d.Item+": "+d.Message)
			}
		}
		return fmt.Errorf("no_supported_components: no components can be delivered; %s", strings.Join(messages, "; "))
	}
	return nil
}

func stdioReadiness(root string, decoded map[string]any, dataRoots ...string) error {
	fail := func(message string) *domain.ComponentReadinessError {
		return &domain.ComponentReadinessError{Code: "stdio_runtime_unavailable", Message: message}
	}
	for _, key := range []string{"PLUGIN_ROOT", "PLUGIN_DATA"} {
		reserved := false
		switch env := decoded["env"].(type) {
		case map[string]any:
			_, reserved = env[key]
		case map[string]string:
			_, reserved = env[key]
		}
		if reserved {
			return &domain.ComponentReadinessError{Code: "stdio_reserved_environment", Message: "stdio environment defines reserved " + key}
		}
	}
	command, _ := decoded["command"].(string)
	if strings.ContainsAny(command, "/\\") || strings.HasPrefix(command, ".") {
		relative, err := pathcontract.ParseCommand(command)
		if err != nil {
			return fail(err.Error())
		}
		resolved := pathcontract.Resolve(root, relative)
		if unexpectedPathIO(resolved.Err) {
			return resolved.Err
		}
		if resolved.State != pathcontract.Resolved {
			return fail(fmt.Sprintf("bundled command %q is unavailable: %v", command, resolved.Err))
		}
		info, err := os.Stat(resolved.Path)
		if unexpectedPathIO(err) {
			return err
		}
		if err != nil || info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
			return fail(fmt.Sprintf("bundled command %q is missing or non-executable", command))
		}
	} else if err := lookupStdioCommand(root, command, decoded["env"], dataRoots...); err != nil {
		if unexpectedPathIO(err) {
			return err
		}
		return fail(fmt.Sprintf("requires executable %q on PATH; install it explicitly (agentplugins never installs runtimes)", command))
	}
	cwd, _ := decoded["cwd"].(string)
	dataRoot := ""
	if len(dataRoots) > 0 {
		dataRoot = dataRoots[0]
	}
	path, err := pathcontract.ExpandCWD(cwd, root, dataRoot)
	if err != nil {
		return fail(err.Error())
	}
	if path.Anchor == pathcontract.Plugin {
		resolved := pathcontract.Resolve(root, path.Relative)
		if unexpectedPathIO(resolved.Err) {
			return resolved.Err
		}
		if resolved.State != pathcontract.Resolved {
			return fail(fmt.Sprintf("cwd %q is unavailable: %v", cwd, resolved.Err))
		}
		info, err := os.Stat(resolved.Path)
		if unexpectedPathIO(err) {
			return err
		}
		if err != nil || !info.IsDir() {
			return fail(fmt.Sprintf("cwd %q is not a directory", cwd))
		}
	}
	return nil
}

// Native clients inherit the authored env. Observe an explicit PATH without
// changing the installer process environment or executing the command.
func lookupStdioCommand(root, command string, environment any, dataRoots ...string) error {
	value, explicit := "", false
	switch env := environment.(type) {
	case map[string]any:
		value, explicit = env["PATH"].(string)
	case map[string]string:
		value, explicit = env["PATH"]
	}
	if !explicit {
		_, err := exec.LookPath(command)
		return err
	}
	dataRoot := ""
	if len(dataRoots) > 0 {
		dataRoot = dataRoots[0]
	}
	value = strings.NewReplacer("${PLUGIN_ROOT}", root, "${PLUGIN_DATA}", dataRoot).Replace(value)
	suffixes := []string{""}
	if runtime.GOOS == "windows" {
		suffixes = append(suffixes, filepath.SplitList(strings.ReplaceAll(os.Getenv("PATHEXT"), ";", string(os.PathListSeparator)))...)
	}
	for _, dir := range filepath.SplitList(value) {
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, suffix := range suffixes {
			info, err := os.Stat(filepath.Join(dir, command+suffix))
			if unexpectedPathIO(err) {
				return err
			}
			if err == nil && info.Mode().IsRegular() && (runtime.GOOS == "windows" || info.Mode().Perm()&0111 != 0) {
				return nil
			}
		}
	}
	return exec.ErrNotFound
}

func authoredPATH(environment any) string {
	switch env := environment.(type) {
	case map[string]any:
		value, _ := env["PATH"].(string)
		return value
	case map[string]string:
		return env["PATH"]
	}
	return ""
}

// Existing confirmation authorizes desired removals; expose them before staging.
func describeMCPRemovals(plan *domain.DeliveryPlan, prior *domain.ClientBinding) {
	if prior == nil {
		return
	}
	selected := map[string]bool{}
	for _, name := range domain.SelectedMCPNames(*plan) {
		selected[name] = true
	}
	for _, object := range prior.NativeObjects {
		if !strings.Contains(object.Kind, "mcp") || object.LogicalName == "" || selected[object.LogicalName] {
			continue
		}
		alreadyDescribed := false
		for _, d := range plan.Diagnostics {
			if d.Item == object.LogicalName && d.Code == "managed_component_removal_planned" {
				alreadyDescribed = true
				break
			}
		}
		if alreadyDescribed {
			continue
		}
		plan.Diagnostics = append(plan.Diagnostics, domain.Diagnostic{Severity: domain.SeverityWarning, Boundary: domain.BoundaryMCPServer, Item: object.LogicalName, Code: "managed_component_removal_planned", Message: "confirmed update will remove this previously managed MCP entry"})
	}
}

func readinessMessages(plan domain.DeliveryPlan) string {
	messages := []string{}
	for _, d := range plan.Diagnostics {
		if d.Boundary == domain.BoundaryMCPServer {
			messages = append(messages, d.Item+": "+d.Message)
		}
	}
	return strings.Join(messages, "; ")
}

func unexpectedPathIO(err error) bool {
	if err == nil || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ELOOP) || errors.Is(err, syscall.ENOTDIR) {
		return false
	}
	var pathErr *os.PathError
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EIO) || errors.As(err, &pathErr)
}
