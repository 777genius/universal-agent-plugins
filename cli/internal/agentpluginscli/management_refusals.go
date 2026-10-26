package agentpluginscli

import (
	"net/http"
	"os"
	"runtime"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

// Only application-owned static guidance belongs here. Provider errors and
// subprocess output stay private; they are not safe public refusal messages.
func managementRefusal(code, message string) error {
	return &management.Refusal{Code: code, Message: message, Status: http.StatusUnprocessableEntity}
}

func invalidManagementRequest(message string) error {
	return &management.Refusal{Code: "INVALID_REQUEST", Message: message, Status: http.StatusBadRequest}
}

func managementExecutableReason(target management.StartupTarget, required bool) string {
	if target.Executable == "" {
		if required {
			return "Select an installed native client executable before installing into this profile"
		}
		return ""
	}
	info, err := os.Stat(target.Executable)
	if err != nil || !info.Mode().IsRegular() || runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
		return "The captured native client executable is unavailable; select its installed runtime again"
	}
	return ""
}

func managementTargetReason(target management.StartupTarget, required bool) string {
	if target.DetectionStatus == management.ClientNotDetected {
		return "This client is not detected; install or configure it before selecting it"
	}
	return managementExecutableReason(target, required)
}

func (b *ManagementBackend) selectedManagementClient(id string, installing bool) (string, error) {
	for _, target := range b.config.Targets {
		if target.Id != id {
			continue
		}
		if installing {
			if reason := managementTargetReason(target, b.app.Installer.RequiresClientExecutable(target.Client)); reason != "" {
				return "", managementRefusal("TARGET_UNAVAILABLE", reason)
			}
		}
		return target.Client, nil
	}
	return "", managementRefusal("TARGET_OUTSIDE_CONTEXT", "Select a target from the current captured environment")
}
