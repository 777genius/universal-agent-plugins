package agentpluginscli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func repairIntentPin(kind string, pinned bool) environment.LockedPackage {
	pin := environment.LockedPackage{Alias: "demo", DigestAlgorithm: domain.TreeDigestAlgorithm,
		TreeDigest: "sha256:" + strings.Repeat("d", 64), ManifestDigest: "sha256:" + strings.Repeat("e", 64)}
	if kind == "git" {
		selector := environment.Selector{Kind: environment.SelectorRef, Value: "main"}
		if pinned {
			selector = environment.Selector{Kind: environment.SelectorCommit, Value: strings.Repeat("a", 40)}
		}
		pin.Source.Git = &environment.LockedGitSource{Repository: "acme/demo", Commit: strings.Repeat("a", 40), Requested: selector}
	} else {
		pin.Source.Archive = &environment.LockedArchiveSource{URL: "https://packages.example/demo.zip", SHA256: strings.Repeat("b", 64)}
		if pinned {
			pin.Source.Archive.RequestedSHA256 = pin.Source.Archive.SHA256
		}
	}
	pin.SourceKey = mustSourceKey(pin.Requirement())
	return pin
}

func TestEnterprisePolicyRepairIntentRequiresMatchingSavedAuthority(t *testing.T) {
	for _, kind := range []string{"git", "archive"} {
		for _, evidence := range []string{"pinned", "moving", "missing", "renamed-alias", "different-source", "different-tree", "different-revision"} {
			t.Run(kind+"/"+evidence, func(t *testing.T) {
				recorded := repairIntentPin(kind, true)
				original := repairIntentPin(kind, evidence != "moving")
				switch evidence {
				case "renamed-alias":
					original.Alias = "another"
				case "different-source":
					original.SourceKey = "another-source"
				case "different-tree":
					original.TreeDigest = "sha256:" + strings.Repeat("f", 64)
				case "different-revision":
					if original.Source.Git != nil {
						original.Source.Git.Commit = strings.Repeat("c", 40)
					} else {
						original.Source.Archive.SHA256 = strings.Repeat("c", 64)
					}
				}
				locked := environment.Lock{Packages: []environment.LockedPackage{original}}
				if evidence == "missing" {
					locked.Packages = nil
				}
				originalIntent, recordedIntent := original.Requirement(), recorded.Requirement()
				result := repairRequestedIntent(recorded, locked)
				request := lockedPolicyRequest(result, domain.ScopeUser)
				pinned := request.ArchivePinned
				if kind == "git" {
					pinned = request.GitPinned
				}
				if pinned == nil || *pinned != (evidence == "pinned") {
					t.Fatalf("repair borrowed an unproven original pin: %+v", result)
				}
				if result.SourceKey != recorded.SourceKey || result.Revision() != recorded.Revision() || result.TreeDigest != recorded.TreeDigest || result.ManifestDigest != recorded.ManifestDigest {
					t.Fatal("repair replaced immutable recorded acquisition identity")
				}
				if !reflect.DeepEqual(originalIntent, original.Requirement()) || !reflect.DeepEqual(recordedIntent, recorded.Requirement()) {
					t.Fatal("repair mutated shared original intent")
				}
			})
		}
	}
}

func TestEnterprisePolicyRepairInspectsColdDirectoryWithoutInstallEligibility(t *testing.T) {
	f, pin, acquirer := coldDependencyCleanupFixture(t, false)
	pin = dependencyCleanupRegistryPin(pin)
	pin.ExecutableFiles = []string{}
	directory := &fixedDirectoryClient{err: domain.ErrDirectoryIneligible}
	f.app.DirectoryClient = directory
	f.app = f.app.withEnterprisePolicyGate(sourcePolicyApp(t, "schema_version: 2\nsources:\n  allowed_kinds: [directory]\n").InstallationPolicy)
	app, cleanup, err := dependencyPreviewApp(f.app, &options{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	action := dependencyAction{Alias: pin.Alias, ClientID: "codex", Action: "repair", Package: &pin}
	if err := checkEnterpriseRepairClosure(t.Context(), app, environment.Lock{Packages: []environment.LockedPackage{pin}}, action); err != nil {
		t.Fatalf("policy inspection required new-install Directory eligibility: %v", err)
	}
	if directory.calls != 0 || acquirer.calls != 1 {
		t.Fatalf("policy changed exact recorded repair acquisition: directory=%d source=%d", directory.calls, acquirer.calls)
	}
}
