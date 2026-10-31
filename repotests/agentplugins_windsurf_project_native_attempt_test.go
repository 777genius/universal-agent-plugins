//go:build linux

package pluginkitairepo_test

import "testing"

func TestAgentpluginsDevinNativeProjectRuntimeAttempt(t *testing.T) {
	windsurfComponentFirstCapture(t)
}

// Keep the original startup capture separately selectable from component research.
func TestAgentpluginsDevinNativeProjectStartupAttempt(t *testing.T) {
	projectNativeCLIRuntimeAttempt(t, "windsurf")
}
