package pluginmanifest

import "strings"

func defaultManifest(projectName, platform, runtime, description string) Manifest {
	platform = normalizeTarget(platform)
	if strings.TrimSpace(description) == "" {
		description = "plugin-kit-ai plugin"
	}
	return Manifest{
		APIVersion:  APIVersionV1,
		Name:        projectName,
		Version:     "0.1.0",
		Description: description,
		Targets:     []string{platform},
	}
}

func defaultLauncher(projectName, runtime string) Launcher {
	runtime = normalizeRuntime(runtime)
	if runtime == "" {
		runtime = "go"
	}
	return Launcher{
		Runtime:    runtime,
		Entrypoint: "./bin/" + projectName,
	}
}
