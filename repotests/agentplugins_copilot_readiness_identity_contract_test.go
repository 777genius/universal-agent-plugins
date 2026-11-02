//go:build linux

package pluginkitairepo_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestCopilotReadinessSourceIDRequiresHeldLocalSourceFraming(t *testing.T) {
	const root = "/held/package"
	// Fixed independently reconstructed SHA256(path + NUL + NUL), first12bytes.
	held := domain.SourceBinding{SourceBindingID: "src_aaaa387ef526663a339f2be5", RequestedSource: root, CanonicalSource: root}
	if !copilotReadinessLocalSource(held, root) {
		t.Fatal("independently held local source identity refused")
	}
	for _, mutate := range []func(*domain.SourceBinding){
		func(v *domain.SourceBinding) { v.SourceBindingID = "" }, func(v *domain.SourceBinding) { v.SourceBindingID = "src_4d4682536698889720380f6d" },
		func(v *domain.SourceBinding) { v.RequestedSource = "/held/other" }, func(v *domain.SourceBinding) { v.CanonicalSource = "/held/other" },
		func(v *domain.SourceBinding) { v.Repository = "other/repository" }, func(v *domain.SourceBinding) { v.PackageSubpath = "other" },
		func(v *domain.SourceBinding) { v.ResolvedRevision = "other" }, func(v *domain.SourceBinding) { v.SourceKind = "archive" },
		func(v *domain.SourceBinding) { v.SourceURL = "https://example.invalid/" }, func(v *domain.SourceBinding) { v.ArchiveSHA256 = "foreign" },
		func(v *domain.SourceBinding) { v.StableSourceKey = "foreign" }, func(v *domain.SourceBinding) { v.Publisher = "foreign" },
	} {
		changed := held
		mutate(&changed)
		if copilotReadinessLocalSource(changed, root) {
			t.Fatal("foreign source identity rebound to held package")
		}
	}
}
func TestCopilotReadinessInstallationClientAndDataIDsRemainDerivedFromHeldInstallation(t *testing.T) {
	const installation = "450e2516-6d65-4ea3-8c4f-d0adda9c2698"
	const project = "/held/project"
	binding := domain.ClientBinding{PhysicalArtifact: "native-proof-ac613ed30810", TargetLocator: project + "/.uap/managed/clients/copilot/native-proof-ac613ed30810", ClientBindingID: "client_e11a201539d1742a27a88399", DataReceiptID: "data_4c8ad55a543d213ce962f6c9"}
	data := domain.DataReceipt{DataReceiptID: binding.DataReceiptID, PhysicalBackend: binding.PhysicalArtifact, Locator: project + "/.uap/plugin-data/native-proof-ac613ed30810", Scope: "project", OwnershipDigest: "sha256:72c9f294ac6e9ce2c61f87c32346401404ca48e850054abddbd4ba7fd3ce90ee"}
	if !copilotReadinessBindingIDs(installation, binding, data, project) {
		t.Fatal("independently reconstructed held installation identifiers refused")
	}
	for _, id := range []string{"", "foreign", "550e8400-e29b-41d4-a716-446655440000"} {
		if copilotReadinessBindingIDs(id, binding, data, project) {
			t.Fatal("foreign public installation adopted")
		}
	}
	for _, fault := range []string{"physical", "active", "client", "coherent-data", "locator", "scope", "ownership"} {
		b, d := binding, data
		switch fault {
		case "physical":
			b.PhysicalArtifact, d.PhysicalBackend = "native-proof-000000000000", "native-proof-000000000000"
		case "active":
			b.TargetLocator = "/foreign/managed"
		case "client":
			b.ClientBindingID = "client_000000000000000000000000"
		case "coherent-data":
			b.DataReceiptID, d.DataReceiptID = "data_000000000000000000000000", "data_000000000000000000000000"
		case "locator":
			d.Locator = "/foreign/data"
		case "scope":
			d.Scope = "user"
		case "ownership":
			d.OwnershipDigest = "sha256:foreign"
		}
		if copilotReadinessBindingIDs(installation, b, d, project) {
			t.Fatal("foreign derived client/data authority admitted", fault)
		}
	}
}
