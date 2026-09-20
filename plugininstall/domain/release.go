package domain

// Release is a GitHub release (subset of API).
type Release struct {
	ID         int64
	TagName    string
	Draft      bool
	Prerelease bool
	UploadURL  string
	Assets     []Asset
}

// Asset is a release attachment.
type Asset struct {
	ID                 int64
	Name               string
	BrowserDownloadURL string
	Size               int64
}
