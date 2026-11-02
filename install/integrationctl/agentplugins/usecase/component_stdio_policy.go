package usecase

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func stdioReadiness(root string, decoded map[string]any, dataRoots ...string) error {
	if err := rejectReservedStdioEnv(decoded); err != nil {
		return err
	}
	if err := stdioCommandReadiness(root, decoded, dataRoots...); err != nil {
		return err
	}
	return stdioCWDReadiness(root, decoded, dataRoots...)
}

func stdioRuntimeUnavailable(message string) *domain.ComponentReadinessError {
	return &domain.ComponentReadinessError{Code: "stdio_runtime_unavailable", Message: message}
}

func rejectReservedStdioEnv(decoded map[string]any) error {
	if reserved := domain.ReservedStdioEnvKey(decoded["env"], runtime.GOOS == "windows"); reserved != "" {
		return &domain.ComponentReadinessError{Code: "stdio_reserved_environment", Message: "stdio environment defines reserved " + reserved}
	}
	return nil
}

func stdioCommandReadiness(root string, decoded map[string]any, dataRoots ...string) error {
	command, _ := decoded["command"].(string)
	if strings.ContainsAny(command, "/\\") || strings.HasPrefix(command, ".") {
		return stdioBundledCommandReadiness(root, command)
	}
	if err := lookupStdioCommand(root, command, decoded["env"], dataRoots...); err != nil {
		if unexpectedPathIO(err) {
			return err
		}
		return stdioRuntimeUnavailable(fmt.Sprintf("requires executable %q on PATH; install it explicitly (agentplugins never installs runtimes)", command))
	}
	return nil
}

func stdioBundledCommandReadiness(root, command string) error {
	relative, err := pathcontract.ParseCommand(command)
	if err != nil {
		return stdioRuntimeUnavailable(err.Error())
	}
	resolved := pathcontract.Resolve(root, relative)
	if unexpectedPathIO(resolved.Err) {
		return resolved.Err
	}
	if resolved.State != pathcontract.Resolved {
		return stdioRuntimeUnavailable(fmt.Sprintf("bundled command %q is unavailable: %v", command, resolved.Err))
	}
	info, err := os.Stat(resolved.Path)
	if unexpectedPathIO(err) {
		return err
	}
	if err != nil || info.IsDir() || (runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0) {
		return stdioRuntimeUnavailable(fmt.Sprintf("bundled command %q is missing or non-executable", command))
	}
	return nil
}

func stdioCWDReadiness(root string, decoded map[string]any, dataRoots ...string) error {
	cwd, _ := decoded["cwd"].(string)
	dataRoot := ""
	if len(dataRoots) > 0 {
		dataRoot = dataRoots[0]
	}
	path, err := pathcontract.ExpandCWD(cwd, root, dataRoot)
	if err != nil {
		return stdioRuntimeUnavailable(err.Error())
	}
	if path.Anchor != pathcontract.Plugin {
		return nil
	}
	resolved := pathcontract.Resolve(root, path.Relative)
	if unexpectedPathIO(resolved.Err) {
		return resolved.Err
	}
	if resolved.State != pathcontract.Resolved {
		return stdioRuntimeUnavailable(fmt.Sprintf("cwd %q is unavailable: %v", cwd, resolved.Err))
	}
	info, err := os.Stat(resolved.Path)
	if unexpectedPathIO(err) {
		return err
	}
	if err != nil || !info.IsDir() {
		return stdioRuntimeUnavailable(fmt.Sprintf("cwd %q is not a directory", cwd))
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
