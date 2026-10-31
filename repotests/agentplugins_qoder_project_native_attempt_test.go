//go:build linux

package pluginkitairepo_test

import "testing"

func TestAgentpluginsQoderNativeProjectRuntimeAttempt(t *testing.T) {
	projectNativeCLIRuntimeAttempt(t, "qoder")
}
