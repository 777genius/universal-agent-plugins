package directoryv1

// SignedFeedID is compile-time client configuration, not remote metadata.
// A second signed schema-1 feed must use a distinct identity and cache path.
func (client Client) SignedFeedID() string { return client.FeedID }
