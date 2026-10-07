package hostprep

import "github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"

// Snapshot preserves the preparation API through the shared immutable authority.
type Snapshot = opencodehost.NativePrepared

var ErrTransportUnsupported = opencodehost.ErrNativeTransportUnsupported

func newSnapshot(executable, root string, environment []string, evidence opencodehost.VersionEvidence, profile opencodehost.Profile, selections []opencodehost.Selection) *Snapshot {
	return opencodehost.NewNativePrepared(executable, root, environment, evidence, profile, selections)
}

func SelectNative(profile opencodehost.Profile, skills bool, transports []string) ([]opencodehost.Selection, error) {
	return opencodehost.SelectNative(profile, skills, transports)
}
