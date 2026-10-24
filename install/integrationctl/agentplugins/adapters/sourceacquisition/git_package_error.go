package sourceacquisition

import (
	"bytes"
	"fmt"
)

// gitPackagePathError is private evidence from our own inspected tree/path.
// Error preserves the GitHub public wording; direct transport callers may
// expose only its controlled reason and their already validated selected path.
type gitPackagePathError struct {
	public error
	reason string
}

func (err *gitPackagePathError) Error() string { return err.public.Error() }

func gitPackageFailure(public error, reason string) error {
	return &gitPackagePathError{public: public, reason: reason}
}

func selectedGitLeaf(tree []byte, subpath string) string {
	if subpath == "" {
		return ""
	}
	for _, record := range bytes.Split(tree, []byte{0}) {
		metadata, name, ok := bytes.Cut(record, []byte{'\t'})
		fields := bytes.Fields(metadata)
		if !ok || len(fields) != 3 || string(name) != subpath {
			continue
		}
		switch string(fields[0]) {
		case "100644":
			return "selected package path is a file, not a directory"
		case "100755":
			return "selected package path is an executable file, not a directory"
		case "120000":
			return "selected package path is a symbolic link, not a real directory"
		}
	}
	return ""
}

func gitNonDirectoryFailure(repository, revision, selected, reason string) error {
	public := fmt.Errorf("acquire GitHub repository %q at revision %s: plugin subpath %q is not a real directory", repository, revision, selected)
	return gitPackageFailure(public, reason)
}
