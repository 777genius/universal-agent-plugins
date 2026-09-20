package main

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

type publicationLocalRootVerifier interface {
	PublicationVerifyRoot(app.PluginPublicationVerifyRootOptions) (app.PluginPublicationVerifyRootResult, error)
}

func shouldVerifyPublicationLocalRoot(dest, diagnosisStatus string) bool {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return false
	}
	switch diagnosisStatus {
	case "inactive", "needs_channels":
		return false
	default:
		return true
	}
}

func verifyPublicationLocalRootWithRunner(runner inspectRunner, opts app.PluginPublicationVerifyRootOptions) (app.PluginPublicationVerifyRootResult, error) {
	verifier, ok := any(runner).(publicationLocalRootVerifier)
	if !ok {
		return app.PluginPublicationVerifyRootResult{}, fmt.Errorf("publication doctor local-root verification is not available for this runner")
	}
	return verifier.PublicationVerifyRoot(opts)
}

func publicationLocalRootOptions(root, requestedTarget, dest, packageRoot string) app.PluginPublicationVerifyRootOptions {
	return app.PluginPublicationVerifyRootOptions{
		Root:        root,
		Target:      requestedTarget,
		Dest:        strings.TrimSpace(dest),
		PackageRoot: packageRoot,
	}
}
