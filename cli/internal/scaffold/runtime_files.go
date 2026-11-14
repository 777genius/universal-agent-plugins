package scaffold

func filesFor(platform, runtime string, extras, typescript, sharedRuntimePackage bool) []TemplateFile {
	if platform == "gemini" && runtime == RuntimeGo {
		return filesForGeminiGo(extras)
	}
	if runtime == RuntimeGo {
		return generatedPlatforms[platform].Files
	}

	files := baseRuntimeFiles(platform)
	platformFiles, terminal := filesForPlatform(platform, extras)
	files = append(files, platformFiles...)
	if terminal {
		return files
	}

	files = append(files, filesForRuntime(platform, runtime, extras, typescript, sharedRuntimePackage)...)
	files = append(files, sharedExtraFiles(platform, extras)...)
	return files
}
