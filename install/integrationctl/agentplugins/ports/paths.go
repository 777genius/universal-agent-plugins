package ports

// PathPolicy is the containment contract every destructive or path-trusting
// decision goes through. The only implementation is adapters/pathpolicy.Policy;
// a permissive stand-in would silently disable symlink and escape rejection, so
// internal/archtest fails the build on a second implementation and
// ports/contracttest is the mandatory harness for any candidate.
type PathPolicy interface {
	// ValidateLeafID rejects an identifier that is not one portable, relative
	// filesystem path component.
	ValidateLeafID(value string) error
	// RequireContainedChild rejects a candidate that is not a strict lexical
	// child of base, or whose existing ancestry contains a symlink.
	RequireContainedChild(base, candidate string) error
	// RequireExactPath rejects a candidate that is not the exact expected
	// managed path, and then applies RequireContainedChild to it.
	RequireExactPath(expected, candidate string) error
}
