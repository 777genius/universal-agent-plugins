package scaffold

func pathsForRuntime(platform, runtime, name string, extras bool, typescript bool, sharedRuntimePackage bool) []string {
	def, ok := LookupPlatform(platform)
	if !ok {
		return nil
	}
	runtime = normalizeRuntime(runtime)
	return planPaths(expandTemplateFiles(planFilesFor(def.Name, runtime, extras, typescript, sharedRuntimePackage), Data{
		ProjectName:          name,
		Platform:             def.Name,
		Runtime:              runtime,
		TypeScript:           typescript,
		SharedRuntimePackage: sharedRuntimePackage,
		ExecutionMode:        defaultExecutionMode(runtime),
		Entrypoint:           "./bin/" + name,
		WithExtras:           extras,
	}))
}

func planPaths(tasks []PlannedFile) []string {
	out := make([]string, 0, len(tasks))
	for _, task := range tasks {
		out = append(out, task.RelPath)
	}
	return out
}
