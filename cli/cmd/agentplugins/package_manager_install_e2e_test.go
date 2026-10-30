//go:build uappm_e2e

package main

import (
	_ "embed"
	"encoding/json"
	"testing"
)

//go:embed testdata/package_manager_install_cases.json
var pmInstallCaseInventory []byte

func TestPackageManagerInstallE2E(t *testing.T) {
	cases := map[string]func(*testing.T){
		"no-save-first-repeat-remove":    pmInstallNoSaveLifecycle,
		"selected-target-ownership":      pmInstallSelectedTargetOwnership,
		"component-set-replacement":      pmInstallComponentReplacement,
		"foreign-skill-collision":        pmInstallForeignSkillCollision,
		"foreign-disabled-mcp-collision": pmInstallForeignDisabledMCPCollision,
		"malformed-native-config":        pmInstallMalformedNativeConfig,
		"drift-recorded-pin-repair":      pmInstallDriftRecordedPinRepair,
		"preview-and-decline":            pmInstallPreviewAndDecline,
		"rich-cold-restore-isolation":    pmInstallRichColdRestoreIsolation,
	}
	var ids []string
	if err := json.Unmarshal(pmInstallCaseInventory, &ids); err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 || len(ids) != len(cases) {
		t.Fatalf("installation inventory/case mismatch: %d / %d", len(ids), len(cases))
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] || cases[id] == nil {
			t.Fatalf("duplicate, empty or unimplemented installation case %q", id)
		}
		seen[id] = true
	}
	for _, id := range ids {
		t.Run(id, cases[id])
	}
}
