package domain

import "time"

const (
	TreeDigestAlgorithm = "agentplugins-tree-sha256-v1"
	DefaultMaxFiles     = 10_000
	DefaultMaxFileBytes = int64(64 << 20)
	DefaultMaxTreeBytes = int64(256 << 20)
	DefaultMaxDepth     = 64
)

type PackageLimits struct {
	MaxFiles     int   `json:"max_files"`
	MaxFileBytes int64 `json:"max_file_bytes"`
	MaxTreeBytes int64 `json:"max_tree_bytes"`
	MaxDepth     int   `json:"max_depth"`
}

type PackageSnapshot struct {
	Root            string           `json:"-"`
	TreeDigest      string           `json:"tree_digest"`
	DigestAlgorithm string           `json:"digest_algorithm"`
	FileCount       int              `json:"file_count"`
	TotalBytes      int64            `json:"total_bytes"`
	ExecutableFiles []string         `json:"executable_files,omitempty"`
	Source          SourceIdentity   `json:"source"`
	OriginMode      OriginMode       `json:"-"`
	Directory       *DirectoryOrigin `json:"-"`
	AcquiredAt      time.Time        `json:"-"`
}

// AcquiredSnapshot is the public installer input for verified host-acquired
// bytes. Root and staging stay request-local; all immutable and provenance
// facts are copied and checked again before Prepare uses them.
type AcquiredSnapshot struct {
	TreeDigest      string             `json:"tree_digest"`
	DigestAlgorithm string             `json:"digest_algorithm"`
	ExecutableFiles []string           `json:"executable_files,omitempty"`
	Source          SourceIdentity     `json:"source"`
	OriginMode      OriginMode         `json:"origin_mode"`
	Directory       *DirectoryOrigin   `json:"directory,omitempty"`
	Compatibility   CompatibilityHints `json:"-"`
}
