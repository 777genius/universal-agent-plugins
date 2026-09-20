package app

func resolvePublicationBaseContext(rootInput, targetInput, destInput, unsupportedMessage, missingDestMessage string) (publicationContext, error) {
	root := normalizePublicationRootInput(rootInput)
	target, err := validatePublicationTargetInput(targetInput, unsupportedMessage)
	if err != nil {
		return publicationContext{}, err
	}
	dest, err := validatePublicationDestInput(destInput, missingDestMessage)
	if err != nil {
		return publicationContext{}, err
	}
	return discoverPublicationBaseContext(root, target, dest)
}
