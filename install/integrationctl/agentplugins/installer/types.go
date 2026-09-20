package installer

// Operation is the process-local lifecycle verb. Install, update, repair, and
// remove are published. Two Claude+Codex targets use the same verb via Request.Targets.
type Operation string

const (
	OpInstall Operation = "install"
	OpRemove  Operation = "remove"
	OpUpdate  Operation = "update"
	OpRepair  Operation = "repair"
)

// Outcome is the coarse public result. Mapping is not a bool.
type Outcome string

const (
	OutcomeUnchanged  Outcome = "unchanged"
	OutcomeCompleted  Outcome = "completed"
	OutcomeIncomplete Outcome = "incomplete"
	OutcomeRecovery   Outcome = "recovery_required"
	OutcomeConflict   Outcome = "conflict"
	OutcomeCancelled  Outcome = "cancelled" //nolint:misspell // Preserve the public outcome wire value.
)

// Request is copied by Prepare. Subsequent caller edits do not change the handle.
type Request struct {
	Operation   Operation
	PackageRoot string
	// SourceRoot is the stable absolute local source identity when PackageRoot
	// points at a host-owned sealed snapshot. Empty means PackageRoot itself.
	SourceRoot string
	// ExecutableFiles preserves a host-acquired snapshot's logical file modes.
	// Nil infers local package executables; a non-nil empty slice means none.
	// Supported for single-target requests only. Assessment still has to match
	// the resulting complete tree digest.
	ExecutableFiles    []string
	ClientID           string
	ClientConfigRoot   string
	ClientExecutable   string
	InstallationID     string
	OperationID        string
	Selector           string
	RequiredComponents []string
	// Assessment is a host decision for these exact local bytes. It is copied
	// and digest-checked by Prepare. Confirmation of the filesystem plan does
	// not create or alter this security decision.
	Assessment *Assessment
	// ExternalUninstalled is host attestation that the selected client's
	// native plugin was already removed, or was never activated. Confirmed
	// Apply does not invent this fact.
	ExternalUninstalled bool
	// Targets selects two clients in one operation. Empty means the single
	// ClientID fields. Group operations keep the same Operation verb.
	Targets []ClientTarget
	// KnownTargets carries host-observed facts for installed sibling bindings
	// that are not selected by this operation. The installer verifies BindingID
	// against owned state before using ConfigRoot or Executable; it never reads a
	// host sidecar or guesses a profile from HOME.
	KnownTargets []TargetFacts
}

// ClientTarget is one Claude or Codex selection in a group request.
type ClientTarget struct {
	ClientID, ClientConfigRoot, ClientExecutable string
	PackageRoot                                  string
	ExternalUninstalled                          bool
}

// TargetFacts is the request-scoped host view of one existing client binding.
// BindingID ties host-owned profile and executable paths to UAP-owned state.
type TargetFacts struct {
	ClientID   string
	BindingID  string
	ConfigRoot string
	Executable string
}

// Decision is host UI confirmation, outside mutation locks.
type Decision struct {
	Confirmed bool
}

// BindingFacts is the typed committed-binding view for host seams.
type BindingFacts struct {
	InstallationID, ClientID, BindingID, Scope string
	TargetPath, DataRoot, DataReceiptID        string
	OperationID, TreeDigest                    string
}

// Plan is an immutable copy for presentation. Operational paths are included
// because the embedding host already chose explicit roots.
type Plan struct {
	Operation            Operation
	SourceRoot           string
	TreeDigest           string
	DigestAlgorithm      string
	ClientID             string
	ConfigRoot           string
	TargetPath           string
	InstallationID       string
	BindingID            string
	HelperVersion        string
	HelperDigest         string
	RequiredMissing      []string
	NoChange             bool
	Targets              []PlanTarget
	Delivery             DeliveryPlan
	Client               ClientResult
	RequiresConfirmation bool
}

// DeliveryPlan is the provider's presentation snapshot, without mutation APIs.
// LocalActions may contain host paths and are for private human output only.
type DeliveryPlan struct {
	Status, PackageMode, InstallIntent, PhysicalArtifactID string
	Activation, Authentication, Policy, Verification       string
	Components                                             []ComponentDecision
	UserActions, LocalActions, Warnings                    []string
	Diagnostics                                            []PlanDiagnostic
}

type ComponentDecision struct{ Kind, Name, Support, Reason string }
type PlanDiagnostic struct{ Severity, Boundary, Code, Path, Item, Message string }

// PlanTarget is one client's prepared identity in a group handle.
type PlanTarget struct {
	ClientID, ConfigRoot, TargetPath, BindingID, TreeDigest string
	NoChange                                                bool
}

// Result is returned together with an error when part of the work already happened.
type Result struct {
	Operation            Operation
	InstallationID       string
	Outcome              Outcome
	Binding              BindingFacts
	ManualActions        []string
	Reason               string
	NoChange             bool
	Mutated              bool
	RequiresConfirmation bool
	DataRetained         bool
	Client               ClientResult
	Targets              []ClientResult
	NextActions          []NextAction
	// Recovery classifies observed receipts after Recover. Apply leaves it empty.
	Recovery RecoveryReport
}

// RecoveryReport is the §5.8 resolved/remaining/unknown receipt view.
// It is populated even when Recover returns an error.
type RecoveryReport struct {
	Resolved  []PendingReceipt
	Remaining []PendingReceipt
	Unknown   []PendingReceipt
}

// NextAction is a structured follow-up. It is not a bool and not a retry token.
type NextAction struct {
	Kind   string
	Reason string
}

// ClientResult is the public per-client lifecycle view. Mapping is not a bool.
type ClientResult struct {
	ClientID, BindingID, TreeDigest                                   string
	Materialization, Activation, Authentication, Policy, Verification string
	RequiredComponents                                                []string
}

// Assessment is a digest-bound content verdict. It is not a filesystem plan.
type Assessment struct {
	TreeDigest string
	Outcome    AssessmentOutcome
	Reason     string
}

// AssessmentOutcome is the host-visible scanner verdict.
type AssessmentOutcome string

const (
	AssessmentAllow       AssessmentOutcome = "allow"
	AssessmentBlock       AssessmentOutcome = "block"
	AssessmentUnavailable AssessmentOutcome = "unavailable"
)

// ProgressPhase is a coarse installer phase. Percent complete is not invented.
type ProgressPhase string

const (
	ProgressPrepare   ProgressPhase = "prepare"
	ProgressPreflight ProgressPhase = "preflight"
	ProgressStage     ProgressPhase = "stage"
	ProgressCommit    ProgressPhase = "commit"
	ProgressActivate  ProgressPhase = "activate"
	ProgressVerify    ProgressPhase = "verify"
	ProgressComplete  ProgressPhase = "complete"
)

// ProgressEvent is an observational checkpoint. The observer does not decide.
type ProgressEvent struct {
	Phase ProgressPhase
}

// Inspection is a read-only view of owned UAP state. Recovery facts are
// limited identities, not raw JSON and not an executable plan.
type Inspection struct {
	StateRoot     string
	Installations []InspectedInstallation
	Recovery      RecoveryObservation
}

// RecoveryObservation is the §5.8 read-only pending-transaction view.
type RecoveryObservation struct {
	Required bool
	Journals []PendingJournal
	Receipts []PendingReceipt
	Reason   string
}

// PendingJournal is one open directory-swap journal.
type PendingJournal struct {
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
	InstallationID string
	TreeDigest     string
	Bindings       []InspectedBinding
	DataRetained   bool
	DataRoots      []string
}

// InspectedBinding is a public subset of one client binding.
type InspectedBinding struct {
	ClientID, BindingID, Scope, TargetPath, DataRoot, TreeDigest string
	Materialization, Activation, Authentication, Verification    string
}

// ClientMetadata is read-only provider surface. Discover does not execute files.
type ClientMetadata struct {
	ClientID          string
	Scopes            []string
	ExecutablePresent bool
	ExecutablePath    string
	Bindings          []InspectedBinding
}
