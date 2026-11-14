package app

type exportArchivePlan struct {
	outputPath string
	files      []string
	metadata   exportMetadata
}

func prepareExportArchivePlan(ctx exportServiceContext, output string) (exportArchivePlan, error) {
	files, outputPath, err := buildExportArchiveFiles(ctx, output)
	if err != nil {
		return exportArchivePlan{}, err
	}
	return exportArchivePlan{
		outputPath: outputPath,
		files:      files,
		metadata:   buildExportMetadata(ctx),
	}, nil
}
