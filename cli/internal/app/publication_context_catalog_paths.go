package app

import "path/filepath"

func publicationContextDestPackageRoot(ctx publicationContext) string {
	return filepath.Join(ctx.dest, filepath.FromSlash(ctx.packageRoot))
}
