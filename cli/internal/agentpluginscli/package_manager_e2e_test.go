//go:build uappm_e2e

package agentpluginscli

import (
	_ "embed"
	"encoding/json"
	"regexp"
	"testing"
)

// This catalog is also read by scripts/package-manager-e2e.py. Keeping the
// required case IDs in one file prevents an empty or renamed selection from
// becoming a successful portable qualification.
//
//go:embed testdata/package_manager_e2e_cases.json
var packageManagerCaseCatalog []byte

func TestPackageManagerE2E(t *testing.T) {
	groups := map[string]map[string]func(*testing.T){
		"archive":     packageManagerArchiveCases(),
		"registry":    packageManagerRegistryCases(),
		"agreement":   packageManagerAgreementCases(),
		"transport":   packageManagerTransportCases(),
		"evolution":   packageManagerEvolutionCases(),
		"adversarial": packageManagerAdversarialCases(),
	}
	var catalog map[string][]string
	if err := json.Unmarshal(packageManagerCaseCatalog, &catalog); err != nil {
		t.Fatal(err)
	}
	if len(groups) != len(catalog) {
		t.Fatalf("case catalog groups differ: registered=%d catalog=%d", len(groups), len(catalog))
	}
	identifier := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	for group, cases := range groups {
		ids, exists := catalog[group]
		if !exists || len(ids) != len(cases) || len(ids) == 0 {
			t.Fatalf("case catalog differs for %s: registered=%d catalog=%v", group, len(cases), ids)
		}
		seen := map[string]bool{}
		for _, id := range ids {
			if !identifier.MatchString(id) || seen[id] || cases[id] == nil {
				t.Fatalf("invalid, duplicate or unregistered required case %s/%s", group, id)
			}
			seen[id] = true
		}
	}
	// The runner selects the explicit archive-safe groups with Go's -run filter.
	// Registration above is checked in full even when only a subset executes.
	for _, group := range []string{"archive", "registry", "agreement", "transport", "evolution", "adversarial"} {
		t.Run(group, func(t *testing.T) {
			for _, id := range catalog[group] {
				t.Run(id, groups[group][id])
			}
		})
	}
}

func packageManagerAgreementCases() map[string]func(*testing.T) {
	return map[string]func(*testing.T){
		"manifest-rejections":            TestDependencyManifestRejectsUnknownConflictingAndMissingIntent,
		"manifest-locations":             TestDependencyManifestErrorLocations,
		"version-intent-preservation":    TestVersionSelectionManifestForms,
		"transfer-replacement-noop":      TestDependencyTransferReplacementAndNoop,
		"transfer-confirmation-recheck":  TestDependencyTransferConfirmationRecheck,
		"transfer-publication-rollback":  TestDependencyTransferPublicationRollback,
		"transfer-mutation-guards":       TestDependencyTransferImportMutationGuards,
		"lock-external-edits":            TestDependencyLockPreservesExternalEdits,
		"lock-store-publication-recheck": TestDependencyLockRechecksAfterStorePublication,
		"lock-cold-store-hydration":      TestDependencyLockUnchangedHydratesMissingStore,
		"lock-read-only-no-publication":  TestDependencyLockReadOnlyResolutionDoesNotPublish,
		"lock-empty-existing-refusal":    TestDependencyLockRejectsEmptyExistingLock,
		"absent-empty-distinct":          TestSameEnvironmentFileDistinguishesAbsentAndEmpty,
		"direct-source-promotion":        TestDirectSavedAddPromotesRetainedIntentWithoutAcquisition,
		"direct-source-selector-refusal": TestDirectSavedPromotionRejectsSelectorOrAliasChanges,
		"occupied-declaration-key":       TestDirectSavedPromotionRejectsOccupiedDeclarationKey,
	}
}
