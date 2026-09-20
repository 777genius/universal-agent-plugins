package app

import (
	"archive/tar"
	"fmt"
	"io"
)

func inspectBundleArchive(archivePath string) (exportMetadata, error) {
	tr, closer, err := openBundleArchive(archivePath)
	if err != nil {
		return exportMetadata{}, err
	}
	defer closer()

	policy := newBundleArchivePolicy()
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return exportMetadata{}, fmt.Errorf("bundle install requires .plugin-kit-ai-export.json in archive root")
		}
		if err != nil {
			return exportMetadata{}, err
		}
		name, err := policy.validate(header)
		if err != nil {
			return exportMetadata{}, err
		}
		if header.Typeflag == tar.TypeReg && name == ".plugin-kit-ai-export.json" {
			return readBundleArchiveMetadataEntry(tr, header.Size)
		}
		if _, err := io.CopyN(io.Discard, tr, header.Size); err != nil {
			return exportMetadata{}, err
		}
	}
}
