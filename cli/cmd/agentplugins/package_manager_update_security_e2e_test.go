//go:build uappm_e2e

package main

import "testing"

func pmInstallUpdateAllSecurityBatchPreflight(t *testing.T) {
	runComposedUpdateAllSecurityPreflight(t, false)
}

func pmInstallUpdateAllSecurityMixedSavedPreflight(t *testing.T) {
	runComposedUpdateAllSecurityPreflight(t, true)
}
