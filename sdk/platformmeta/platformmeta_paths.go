package platformmeta

import "path"

const (
	CanonicalAuthoredRoot = "plugin"
)

func authoredPath(rel string) string {
	return path.Join(CanonicalAuthoredRoot, rel)
}
