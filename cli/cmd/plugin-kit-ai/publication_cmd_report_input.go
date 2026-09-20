package main

import "github.com/777genius/plugin-kit-ai/cli/internal/app"

func publicationRoot(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func inspectPublicationReport(runner inspectRunner, root, target string) (publicationInspection, []publicationWarning, error) {
	report, warnings, err := runner.Inspect(app.PluginInspectOptions{
		Root:   root,
		Target: target,
	})
	return report, warnings, err
}
