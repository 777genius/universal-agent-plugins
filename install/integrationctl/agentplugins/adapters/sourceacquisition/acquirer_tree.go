// Package sourceacquisition resolves direct package sources into one sealed,
// digest-verified local snapshot. It never executes package content.
package sourceacquisition

import (
	"bytes"
	"fmt"
	"strings"
)

func executablePathsFromTree(tree []byte, subpath string) ([]string, error) {
	return executablePathsFromTreePolicy(tree, subpath, true)
}

func executablePathsFromTreePolicy(tree []byte, subpath string, allowRootGitlinks bool) ([]string, error) {
	var executable []string
	prefix := ""
	if subpath != "" {
		prefix = subpath + "/"
	}
	for _, record := range bytes.Split(tree, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, filename, ok := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !ok || len(fields) != 3 {
			return nil, fmt.Errorf("git tree returned a malformed entry")
		}
		mode, objectType := string(fields[0]), string(fields[1])
		if mode == "160000" || objectType == "commit" {
			// A repository-root Agent Plugin may live beside unrelated Git
			// submodules. The isolated checkout never fetches their content, so
			// they cannot contribute files or executable bits to the snapshot.
			// Keep rejecting a gitlink inside an explicitly selected package
			// subpath, where omitting it could silently produce a partial package.
			if subpath == "" && allowRootGitlinks {
				continue
			}
			return nil, fmt.Errorf("git submodule content is unsupported in plugin subpath")
		}
		if mode != "100755" {
			continue
		}
		relative := string(filename)
		if prefix != "" {
			if !strings.HasPrefix(relative, prefix) {
				return nil, fmt.Errorf("git tree entry %q is outside plugin subpath %q", relative, subpath)
			}
			relative = strings.TrimPrefix(relative, prefix)
		}
		if relative == "" {
			return nil, fmt.Errorf("git tree returned an empty executable path")
		}
		executable = append(executable, relative)
	}
	return executable, nil
}
