//go:build linux

package pluginkitairepo_test

import "testing"

func TestAgentpluginsKiroNativeProjectRuntimeAttempt(t *testing.T) {
	projectNativeCLIRuntimeAttempt(t, "kiro")
}
