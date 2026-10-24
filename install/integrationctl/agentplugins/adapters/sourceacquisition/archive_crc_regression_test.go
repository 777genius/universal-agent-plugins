package sourceacquisition

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"testing"
)

func archiveCorruptZIPPayload(t *testing.T, source []byte) []byte {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(source), int64(len(source)))
	if err != nil || len(reader.File) == 0 {
		t.Fatalf("invalid ZIP fixture: %v", err)
	}
	first := reader.File[0]
	offset, err := first.DataOffset()
	if err != nil || first.CompressedSize64 < 3 || offset < 0 || uint64(offset)+first.CompressedSize64 > uint64(len(source)) {
		t.Fatalf("invalid ZIP payload location: offset%d size%d error%v", offset, first.CompressedSize64, err)
	}
	// This is inside compressed data, away from its leading deflate control bit
	// and from names, extra fields, descriptors and central-directory metadata.
	index := offset + int64(first.CompressedSize64/2)
	corrupt := bytes.Clone(source)
	corrupt[index] ^= 1
	return corrupt
}

func TestArchiveActualStoredZIPChecksumFailureNeverFallsBack(t *testing.T) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.CreateHeader(&zip.FileHeader{Name: "plugin.json", Method: zip.Store})
	if err != nil {
		t.Fatal(err)
	}
	const content = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"crc-proof","version":"1.0.0"}`
	if _, err := io.WriteString(file, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	valid := buffer.Bytes()
	reader, err := zip.NewReader(bytes.NewReader(valid), int64(len(valid)))
	if err != nil || len(reader.File) != 1 || reader.File[0].Name != "plugin.json" {
		t.Fatalf("normal ZIP metadata: %v", err)
	}
	stream, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(stream)
	closeErr := stream.Close()
	if readErr != nil || closeErr != nil || string(body) != content {
		t.Fatalf("normal ZIP data: %q %v %v", body, readErr, closeErr)
	}
	corrupt := archiveCorruptZIPPayload(t, valid)
	reader, err = zip.NewReader(bytes.NewReader(corrupt), int64(len(corrupt)))
	if err != nil || len(reader.File) != 1 || reader.File[0].Name != "plugin.json" {
		t.Fatalf("payload mutation damaged ZIP metadata: %v", err)
	}
	stream, err = reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	_, readErr = io.ReadAll(stream)
	closeErr = stream.Close()
	if !errors.Is(readErr, zip.ErrChecksum) || closeErr != nil {
		t.Fatalf("actual stored payload did not fail CRC: %v %v", readErr, closeErr)
	}
	archiveRejectLocal(t, corrupt)
}
