package installer

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Inspection is a read-only view of owned UAP state. Recovery facts are
// limited identities, not raw JSON and not an executable plan.
type Inspection struct {
	StateRoot           string
	Installations       []InspectedInstallation
	Recovery            RecoveryObservation
	EnvironmentReceipts []domain.EnvironmentReceipt
}

// RecoveryObservation is the §5.8 read-only pending-transaction view.
type RecoveryObservation struct {
	// StateDigest binds all persisted bindings, receipts and installation facts.
	StateDigest   string
	Required      bool
	Journals      []PendingJournal
	NativeIntents []PendingNativeIntent
	Receipts      []PendingReceipt
	Reason        string
}

// PendingJournal is one open directory-swap journal.
type PendingJournal struct {
	EnvironmentID                                                     string
	OperationID, Digest, BindingID, InstallationID, TargetPath, Phase string
}

// PendingReceipt is an unfinished state receipt, including state_committed
// after the matching journal was already removed.
type PendingReceipt struct {
	OperationID, BindingID, InstallationID, TargetPath, Phase string
	JournalPresent                                            bool
}

// InspectedInstallation is a public subset of one UAP installation.
type InspectedInstallation struct {
	SourceKey      string
	InstallationID string
	TreeDigest     string
	Bindings       []InspectedBinding
	DataRetained   bool
	DataRoots      []string
}

// InspectedBinding is a public subset of one client binding.
type InspectedBinding struct {
	// ManagedPaths are observed ownership locations, never mutation authority.
	ManagedPaths                                                 []string `json:",omitempty"`
	Policy                                                       string
	LifecycleConverged                                           bool
	ClientID, BindingID, Scope, TargetPath, DataRoot, TreeDigest string
	Materialization, Activation, Authentication, Verification    string
	Revision                                                     string
	Owner                                                        *domain.EnvironmentReceipt
	ProjectionDigest, ProjectionStatus                           string
}

// ClientMetadata is read-only provider surface. Discover does not execute files.
type ClientMetadata struct {
	Capabilities      domain.ClientCapabilities
	ClientID          string
	Scopes            []string
	ExecutablePresent bool
	ExecutablePath    string
	Bindings          []InspectedBinding
}
