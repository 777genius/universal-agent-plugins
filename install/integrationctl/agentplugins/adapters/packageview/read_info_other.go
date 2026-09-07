//go:build !windows || !(amd64 || arm64)

package packageview

import "os"

// Linux and Darwin carry link metadata directly in FileInfo. Keep the fresh
// post-open observation, including Darwin's additional legacy authorization.
func (*pinned) postOpenInfo(f *os.File) (os.FileInfo, error) { return f.Stat() }
