package directoryv1

import "fmt"

// Schema-1 pointer validation continues to use the original strict contract.
func validatePointer(v Pointer) error {
	if v.PointerSchemaVersion != 1 || v.SnapshotSchemaVersion != 1 || v.Sequence < 1 {
		return strictError("unsupported pointer schema or sequence")
	}
	stem := fmt.Sprintf("%020d", v.Sequence)
	if err := exactArtifactPath(v.SnapshotPath, "snapshots/"+stem+".json"); err != nil {
		return err
	}
	if err := exactArtifactPath(v.EnvelopePath, "snapshots/"+stem+".envelope.json"); err != nil {
		return err
	}
	c := v.FetchContract
	if !c.HTTPSRequired || !c.SameOriginRedirectsOnly || c.ForwardCredentialsOnRedirect {
		return strictError("unsafe fetch contract")
	}
	if c.MaxRedirects < 0 || c.MaxRedirects > 2 || c.RetryAttempts < 1 || c.RetryAttempts > 3 {
		return strictError("fetch retry/redirect bounds")
	}
	if c.LatestMaxBytes < 1 || c.LatestMaxBytes > MaxLatestBytes || c.SnapshotMaxBytes < 1 || c.SnapshotMaxBytes > MaxSnapshotBytes || c.EnvelopeMaxBytes < 1 || c.EnvelopeMaxBytes > MaxEnvelopeBytes {
		return strictError("fetch size bounds")
	}
	return nil
}
