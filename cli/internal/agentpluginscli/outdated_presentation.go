package agentpluginscli

import (
	"fmt"
	"io"
)

func renderOutdated(writer io.Writer, result outdatedResult) error {
	if len(result.Installations) == 0 {
		_, err := fmt.Fprintln(writer, "No tracked Agent Plugins installations.")
		return err
	}
	for _, installation := range result.Installations {
		if _, err := fmt.Fprintf(writer, "%s: %s - %s\n", installation.Name, installation.Status, installation.Reason); err != nil {
			return err
		}
		if installation.Installed != nil {
			if _, err := fmt.Fprintf(writer, "  Installed: %s (%s)\n", outdatedIdentityLabel(installation.Installed), installation.Installed.Revision); err != nil {
				return err
			}
		}
		if err := renderOutdatedAvailable(writer, installation); err != nil {
			return err
		}
	}
	return nil
}

func renderOutdatedAvailable(writer io.Writer, installation outdatedInstallation) error {
	if installation.Available == nil {
		return nil
	}
	label := outdatedIdentityLabel(installation.Available)
	_, err := fmt.Fprintf(writer, "  Available: %s (%s)\n", label, installation.Available.Revision)
	return err
}

func outdatedIdentityLabel(identity *outdatedReleaseIdentity) string {
	if identity == nil {
		return "absent"
	}
	label := identity.PackageVersion
	if identity.ReleaseSequence > 0 {
		label = fmt.Sprintf("%s release %d", identity.DistributionID, identity.ReleaseSequence)
	}
	if label == "" {
		label = identity.TreeDigest
	}
	return label
}
