package statev2

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStoreRoundTripAndDuplicateDeclaredNames(t *testing.T) {
	t.Parallel()
	store := Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}
	state := domain.StateFileV2{
		SchemaVersion: domain.StateSchemaVersion,
		Installations: []domain.Installation{
			validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001"),
			validInstallation("00000000-0000-4000-8000-000000000002", "src_two", "demo-000000000002"),
		},
	}
	if err := store.Save(state); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Installations) != 2 || loaded.Installations[0].DeclaredName != "demo" || loaded.Installations[1].DeclaredName != "demo" {
		t.Fatalf("installations = %+v", loaded.Installations)
	}
	info, err := os.Stat(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o", info.Mode().Perm())
	}
}

func TestChatGPTStateUsesCurrentSchemaAndOldReadersFailClosed(t *testing.T) {
	t.Parallel()
	store := Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{
		validChatGPTInstallation(),
	}}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		t.Fatal(err)
	}
	if header.SchemaVersion != domain.StateSchemaVersion || header.SchemaVersion == domain.PreviousStateSchemaVersion {
		t.Fatalf("ChatGPT state was not gated behind current schema: %s", body)
	}
	for _, required := range []string{`"catalog_evidence"`, `"app_present"`, `"app_bindings"`} {
		if !bytes.Contains(body, []byte(required)) {
			t.Fatalf("v3 ChatGPT state omitted %s: %s", required, body)
		}
	}
}

func TestStoreReadsLegacyV2LosslesslyWithoutMutatingUntilSave(t *testing.T) {
	t.Parallel()
	store := Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}
	legacy := domain.StateFileV2{SchemaVersion: domain.LegacyStateSchemaVersion, Installations: []domain.Installation{
		validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001"),
	}}
	body, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(store.Path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != domain.StateSchemaVersion || len(loaded.Installations) != 1 || loaded.Installations[0].Source.TreeDigest != "sha256:tree" {
		t.Fatalf("legacy state migration = %+v", loaded)
	}
	afterRead, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(afterRead, body) {
		t.Fatal("read-only legacy state load rewrote the state file")
	}
	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	afterSave, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(afterSave, []byte(`"schema_version": 4`)) {
		t.Fatalf("first explicit save did not persist current state: %s", afterSave)
	}
}

func TestStoreRejectsV3FieldsDisguisedAsLegacyV2(t *testing.T) {
	t.Parallel()
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{validChatGPTInstallation()}}
	body, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	body = bytes.Replace(body, []byte(`"schema_version":4`), []byte(`"schema_version":2`), 1)
	path := filepath.Join(t.TempDir(), "state-v2.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{Path: path}).Load(); err == nil || (!strings.Contains(err.Error(), "app_present") && !strings.Contains(err.Error(), "catalog_evidence")) {
		t.Fatalf("v3 fields under a v2 header were not rejected strictly: %v", err)
	}
}

func TestStoreFailsClosedOnFutureOrUnknownState(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"future":  `{"schema_version":5,"installations":[]}`,
		"unknown": `{"schema_version":4,"installations":[],"future":true}`,
	} {
		name, body := name, body
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state-v2.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := (Store{Path: path}).Load(); err == nil {
				t.Fatal("invalid future state accepted")
			}
		})
	}
}

func TestStoreRejectsDuplicateInstallationIdentity(t *testing.T) {
	t.Parallel()
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation, installation}}
	err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
	if err == nil || !strings.Contains(err.Error(), "duplicate installation_id") {
		t.Fatalf("save error = %v", err)
	}
}

func TestStoreAcceptsOfficialPackageWithoutPortableSchemaURI(t *testing.T) {
	t.Parallel()
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	installation.Package.FormatID = domain.FormatIDOpenAIPlugin
	installation.Package.SchemaURI = ""
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	if err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state); err != nil {
		t.Fatalf("official package state rejected: %v", err)
	}
}

func TestStoreRejectsDuplicateReceiptOperationIDsAcrossClients(t *testing.T) {
	t.Parallel()
	installations := []domain.Installation{
		validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001"),
		validInstallation("00000000-0000-4000-8000-000000000002", "src_two", "demo-000000000002"),
	}
	for installationIndex := range installations {
		for key, client := range installations[installationIndex].Clients {
			client.Receipts = []domain.MutationReceipt{{
				OperationID: "duplicate-op", Sequence: 1, MutationType: "directory_swap",
				ClientBindingID: client.ClientBindingID, Phase: "committed",
			}}
			installations[installationIndex].Clients[key] = client
		}
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: installations}
	err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
	if err == nil || !strings.Contains(err.Error(), "duplicate receipt operation_id") {
		t.Fatalf("save error = %v", err)
	}
}

func TestStoreRejectsUnknownReceiptPhase(t *testing.T) {
	t.Parallel()
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	for key, client := range installation.Clients {
		client.Receipts = []domain.MutationReceipt{{
			OperationID: "tampered-op", Sequence: 1, MutationType: "directory_swap",
			ClientBindingID: client.ClientBindingID, Phase: "tampered",
		}}
		installation.Clients[key] = client
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
	if err == nil || !strings.Contains(err.Error(), "invalid phase") {
		t.Fatalf("save error = %v", err)
	}
}

func TestStoreValidatesStandaloneTransactionReceipts(t *testing.T) {
	t.Parallel()
	base := domain.MutationReceipt{
		OperationID: "standalone-op", Sequence: 1, MutationType: "directory_remove",
		ClientBindingID: "removed-binding", Phase: domain.ReceiptPhaseCommitted,
	}
	for _, test := range []struct {
		name    string
		mutate  func(*domain.MutationReceipt)
		wantErr string
	}{
		{name: "valid committed"},
		{name: "valid state committed", mutate: func(receipt *domain.MutationReceipt) {
			receipt.Phase = domain.ReceiptPhaseStateCommitted
		}},
		{name: "invalid phase", mutate: func(receipt *domain.MutationReceipt) {
			receipt.Phase = "tampered"
		}, wantErr: "invalid phase"},
		{name: "missing mutation type", mutate: func(receipt *domain.MutationReceipt) {
			receipt.MutationType = ""
		}, wantErr: "incomplete"},
	} {
		t.Run(test.name, func(t *testing.T) {
			receipt := base
			if test.mutate != nil {
				test.mutate(&receipt)
			}
			state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, TransactionReceipts: []domain.MutationReceipt{receipt}}
			err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
			if test.wantErr == "" && err != nil {
				t.Fatalf("valid receipt rejected: %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("save error = %v; want %q", err, test.wantErr)
			}
		})
	}
}

func TestStoreRejectsReceiptOperationIDSharedWithStandaloneReceipt(t *testing.T) {
	t.Parallel()
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	var bindingID string
	for key, client := range installation.Clients {
		bindingID = client.ClientBindingID
		client.Receipts = []domain.MutationReceipt{{OperationID: "duplicate-op", Sequence: 1, MutationType: "directory_swap", ClientBindingID: bindingID, Phase: domain.ReceiptPhaseCommitted}}
		installation.Clients[key] = client
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}, TransactionReceipts: []domain.MutationReceipt{{OperationID: "duplicate-op", Sequence: 2, MutationType: "directory_remove", ClientBindingID: bindingID, Phase: domain.ReceiptPhaseCommitted}}}
	err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
	if err == nil || !strings.Contains(err.Error(), "duplicate receipt operation_id") {
		t.Fatalf("save error = %v", err)
	}
}

func TestStoreRejectsUnknownLifecycleState(t *testing.T) {
	t.Parallel()
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	for key, client := range installation.Clients {
		client.Activation = domain.ActivationState("future_activation_state")
		installation.Clients[key] = client
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}}
	err := (Store{Path: filepath.Join(t.TempDir(), "state-v2.json")}).Save(state)
	if err == nil || !strings.Contains(err.Error(), "unknown lifecycle state") {
		t.Fatalf("save error = %v", err)
	}
}

func TestValidateRequiresFullDirectoryResolvedRevisionsButAllowsAbsentDirectLocalRevision(t *testing.T) {
	directoryInstallation := func() domain.Installation {
		installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
		installation.OriginMode = domain.OriginModeDirectory
		installation.Directory = &domain.DirectoryOrigin{ProductID: "demo", DistributionID: "owner/demo", DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: 1}
		installation.Source.ResolvedRevision = strings.Repeat("a", 40)
		for key, client := range installation.Clients {
			client.PackageRevision = &domain.ClientPackageRevision{ResolvedRevision: strings.Repeat("a", 40), TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest", DistributionID: "owner/demo", ReleaseSequence: 1}
			installation.Clients[key] = client
		}
		return installation
	}
	for _, test := range []struct {
		name   string
		mutate func(*domain.Installation)
	}{
		{name: "missing installation revision", mutate: func(installation *domain.Installation) { installation.Source.ResolvedRevision = "" }},
		{name: "malformed installation revision", mutate: func(installation *domain.Installation) {
			installation.Source.ResolvedRevision = strings.Repeat("A", 40)
		}},
		{name: "missing client revision", mutate: func(installation *domain.Installation) {
			for key, client := range installation.Clients {
				client.PackageRevision.ResolvedRevision = ""
				installation.Clients[key] = client
			}
		}},
		{name: "malformed client revision", mutate: func(installation *domain.Installation) {
			for key, client := range installation.Clients {
				client.PackageRevision.ResolvedRevision = "not-a-full-sha"
				installation.Clients[key] = client
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			installation := directoryInstallation()
			test.mutate(&installation)
			err := Validate(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{installation}})
			if err == nil || !strings.Contains(err.Error(), "resolved_revision") {
				t.Fatalf("invalid Directory revision accepted: %v", err)
			}
		})
	}
	direct := validInstallation("00000000-0000-4000-8000-000000000001", "src_one", "demo-000000000001")
	direct.OriginMode = domain.OriginModeDirect
	direct.Source.ResolvedRevision = ""
	if err := Validate(domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{direct}}); err != nil {
		t.Fatalf("direct local state with absent Git revision was rejected: %v", err)
	}
}

func validInstallation(installationID, sourceID, physicalID string) domain.Installation {
	clientID := domain.ComputeClientBindingID(installationID, "codex", "user", "test-home")
	return domain.Installation{
		InstallationID: installationID,
		DeclaredName:   "demo",
		Source: domain.SourceBinding{
			SourceBindingID:  sourceID,
			RequestedSource:  "demo",
			CanonicalSource:  "https://example.com/demo",
			ResolvedRevision: "abc123",
			TreeDigest:       "sha256:tree",
		},
		Package: domain.PackageBinding{
			LoaderKind:     domain.LoaderKindAgentPlugins,
			FormatID:       domain.FormatIDAgentPluginsV1,
			SchemaURI:      domain.PluginSchemaV1,
			DeclaredName:   "demo",
			ManifestDigest: "sha256:manifest",
		},
		Clients: map[string]domain.ClientBinding{
			clientID: {
				ClientBindingID:  clientID,
				ClientID:         "codex",
				Scope:            "user",
				TargetLocator:    "test-home",
				PhysicalArtifact: physicalID,
				Materialization:  domain.MaterializationMaterialized,
				Activation:       domain.ActivationPrepared,
				Authentication:   domain.AuthenticationNotRequired,
				Policy:           domain.PolicyAllowed,
				Verification:     domain.VerificationPackageValid,
			},
		},
	}
}

func validChatGPTInstallation() domain.Installation {
	installation := validInstallation("00000000-0000-4000-8000-000000000001", "src_chatgpt", "demo-chatgpt")
	installation.Package.Inventory = domain.ComponentInventory{
		MCPPresent: true, MCPEnabled: true, MCPServers: []string{"demo"},
		AppPresent: true, AppBindings: []string{"demo"},
	}
	for key, client := range installation.Clients {
		client.ClientID = "chatgpt"
		client.PackageRevision = &domain.ClientPackageRevision{
			Version: "1.0.0", ResolvedRevision: "abc123", TreeDigest: "sha256:tree", ManifestDigest: "sha256:manifest",
			CatalogEvidence: &domain.CatalogEvidence{
				SchemaVersion: 2, CatalogVersion: "0.2.0", Repository: "777genius/universal-agent-plugins",
				Revision: strings.Repeat("a", 40), Digest: "sha256:catalog", MinimumCLIVersion: "0.1.6",
				Compatibility: map[string]domain.CatalogCompatibility{"chatgpt": {
					Package: "projected", AppBinding: &domain.CatalogAppBinding{
						AppKey: "demo", ID: "connector_demo", MCPServer: "demo", MCPURL: "https://example.test/mcp",
						RuntimeEvidence: "tests/e2e/results/chatgpt-demo.json", RuntimeEvidenceRevision: strings.Repeat("b", 40),
					},
				}},
			},
		}
		installation.Clients[key] = client
	}
	return installation
}
