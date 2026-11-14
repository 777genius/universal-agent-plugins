package app

func runExportService(opts PluginExportOptions) (PluginExportResult, error) {
	ctx, plan, err := loadExportServiceRunContext(opts)
	if err != nil {
		return PluginExportResult{}, err
	}
	if err := writeExportArchive(ctx.root, plan.outputPath, plan.files, plan.metadata); err != nil {
		return PluginExportResult{}, err
	}
	return buildExportServiceResult(ctx, plan), nil
}

func loadExportServiceRunContext(opts PluginExportOptions) (exportServiceContext, exportArchivePlan, error) {
	ctx, err := loadExportServiceContext(opts)
	if err != nil {
		return exportServiceContext{}, exportArchivePlan{}, err
	}
	plan, err := prepareExportArchivePlan(ctx, opts.Output)
	if err != nil {
		return exportServiceContext{}, exportArchivePlan{}, err
	}
	return ctx, plan, nil
}

func buildExportServiceResult(ctx exportServiceContext, plan exportArchivePlan) PluginExportResult {
	return PluginExportResult{Lines: buildExportResultLines(ctx, plan)}
}
