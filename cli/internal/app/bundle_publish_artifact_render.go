package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func buildBundlePublishArtifact(metadata exportMetadata, body []byte) bundlePublishArtifact {
	bundleName := fmt.Sprintf("%s_%s_%s_bundle.tar.gz", metadata.PluginName, metadata.Platform, metadata.Runtime)
	return bundlePublishArtifact{
		Metadata:    metadata,
		Body:        body,
		BundleName:  bundleName,
		SidecarName: bundleName + ".sha256",
		SidecarBody: buildBundlePublishSidecar(bundleName, body),
	}
}

func buildBundlePublishSidecar(bundleName string, body []byte) []byte {
	sum := sha256.Sum256(body)
	return []byte(hex.EncodeToString(sum[:]) + "  " + bundleName + "\n")
}
