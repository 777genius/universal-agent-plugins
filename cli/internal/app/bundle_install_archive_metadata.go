package app

import (
	"encoding/json"
	"fmt"
	"io"
)

func readBundleArchiveMetadataEntry(reader io.Reader, expectedSize int64) (exportMetadata, error) {
	body, err := io.ReadAll(io.LimitReader(reader, expectedSize+1))
	if err != nil {
		return exportMetadata{}, err
	}
	if int64(len(body)) != expectedSize {
		return exportMetadata{}, fmt.Errorf("bundle install metadata changed size while reading")
	}
	metadata, err := decodeBundleArchiveMetadata(body)
	return metadata, err
}

func decodeBundleArchiveMetadata(body []byte) (exportMetadata, error) {
	var metadata exportMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return exportMetadata{}, fmt.Errorf("bundle install requires valid .plugin-kit-ai-export.json: %w", err)
	}
	return metadata, nil
}
