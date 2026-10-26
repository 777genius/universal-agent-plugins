//go:build linux

package pluginkitairepo_test

import "testing"

func TestAgentpluginsGrokNativeProjectRuntimeAttempt(t *testing.T) {
	projectNativeCLIRuntimeAttempt(t, "grok")
}
