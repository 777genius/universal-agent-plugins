//go:build !linux && !(darwin && arm64) && !(windows && (amd64 || arm64))

package packageview

import "os"

// Fail closed: these hosts have no proven no-device-open replacement gate yet.
// This is a capability result, never a failed portable conformance result.
type source struct{ legacyInfo os.FileInfo }
type pinned struct {
	file *os.File
	info os.FileInfo
}

func openSource(string, GeneratedStaging) (*source, error) { return nil, fail("platform_unavailable") }
func (*source) close() error                               { return nil }
func (*source) pin(string, bool) (*pinned, error) {
	return nil, fail("platform_unavailable")
}
func stateOf(error) State                     { return Blocked }
func same(a, b os.FileInfo) bool              { return false }
func multipleLinks(os.FileInfo) bool          { return true }
func (*pinned) link(int64) (string, error)    { return "", fail("platform_unavailable") }
func (*pinned) reopen(bool) (*os.File, error) { return nil, fail("platform_unavailable") }
