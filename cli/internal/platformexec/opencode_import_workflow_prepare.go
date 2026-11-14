package platformexec

func initializeOpenCodeImportState() opencodeImportedState {
	return newOpenCodeImportedState()
}

func runOpenCodeImportScopes(state *opencodeImportedState, root string, seed ImportSeed) error {
	if err := importOpenCodeUserScope(state, seed); err != nil {
		return err
	}
	if err := importOpenCodeProjectScope(state, root); err != nil {
		return err
	}
	return nil
}

func finalizeOpenCodeImport(state opencodeImportedState, seed ImportSeed) (ImportResult, error) {
	if err := requireOpenCodeImportedInput(state); err != nil {
		return ImportResult{}, err
	}
	return buildOpenCodeImportResult(state, seed)
}
