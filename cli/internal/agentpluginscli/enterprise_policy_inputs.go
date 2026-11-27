package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"

func withEnterprisePins(input usecase.AddInput, loaded loadedPackage) usecase.AddInput {
	input.GitPinned, input.ArchivePinned = requestedPolicyPins(input.Envelope.Source)
	input.OriginMode = loaded.origin
	if input.DirectoryResolution == nil {
		input.DirectoryResolution = cloneDirectoryOrigin(loaded.directory)
	}
	return input
}

func withEnterpriseBindingPins(input usecase.BindingChangeInput) usecase.BindingChangeInput {
	input.GitPinned, input.ArchivePinned = requestedPolicyPins(input.Envelope.Source)
	return input
}
