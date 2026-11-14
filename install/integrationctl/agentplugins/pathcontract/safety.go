package pathcontract

import "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"

// ValidatePortablePathSegment exposes the shared package-tree naming contract to
// application consumers without making them construct an internal path policy.
func ValidatePortablePathSegment(value string) error {
	return pathpolicy.ValidatePortablePathSegment(value)
}

// RequireContainedChild applies the existing strict-child and symlink-ancestry
// policy. It does not introduce a second PathPolicy implementation.
func RequireContainedChild(base, candidate string) error {
	return pathpolicy.RequireContainedChild(base, candidate)
}
