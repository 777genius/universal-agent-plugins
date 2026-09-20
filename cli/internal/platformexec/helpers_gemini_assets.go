package platformexec

import "path/filepath"

func geminiExtraContextArtifactPath(rel string) string {
	return filepath.ToSlash(filepath.Join("contexts", filepath.Base(rel)))
}
