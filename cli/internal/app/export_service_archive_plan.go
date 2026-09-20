package app

func buildExportArchiveFiles(ctx exportServiceContext, output string) ([]string, string, error) {
	generated, err := loadGeneratedExportArtifacts(ctx)
	if err != nil {
		return nil, "", err
	}
	files, err := buildExportArchiveFileSet(ctx, generated)
	if err != nil {
		return nil, "", err
	}
	outputPath := resolveExportArchiveOutputPath(ctx, output)
	files = dropExportArchiveOutput(files, ctx.root, outputPath)
	return files, outputPath, nil
}
