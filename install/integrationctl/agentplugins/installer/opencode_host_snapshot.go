package installer

import "github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"

// OpenCodePreparedHost preserves the facade contract while sharing its pure,
// immutable native authority with the CLI's existing lifecycle consumer.
type OpenCodePreparedHost = opencodehost.NativePrepared

func newOpenCodePreparedHost(executable, root string, environment []string, evidence opencodehost.VersionEvidence, profile opencodehost.Profile, selections []opencodehost.Selection) *OpenCodePreparedHost {
	return opencodehost.NewNativePrepared(executable, root, environment, evidence, profile, selections)
}

func selectOpenCodeNative(profile opencodehost.Profile, skills bool, transports []string) ([]opencodehost.Selection, error) {
	return opencodehost.SelectNative(profile, skills, transports)
}
