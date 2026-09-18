package domain

// PlanRequest carries everything the planner needs for one delivery decision.
// Detected is the surface map used for backends a client shares with another
// logical client; a nil map means the planner falls back to the one its
// composition root configured.
type PlanRequest struct {
	Envelope           PackageEnvelope
	Client             DetectedClient
	Scope              InstallScope
	PhysicalArtifactID string
	InstallIntent      InstallIntent
	Detected           map[ClientID]DetectedClient
}
