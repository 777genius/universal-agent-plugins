package platformexec

func (opencodeAdapter) Import(root string, seed ImportSeed) (ImportResult, error) {
	return importOpenCodePackage(root, seed)
}
