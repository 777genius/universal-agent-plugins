package app

import (
	"archive/tar"
	"encoding/json"
)

func writeExportArchiveMetadata(tw *tar.Writer, metadata exportMetadata) error {
	body, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return writeArchiveEntry(tw, ".plugin-kit-ai-export.json", body, 0o644)
}
