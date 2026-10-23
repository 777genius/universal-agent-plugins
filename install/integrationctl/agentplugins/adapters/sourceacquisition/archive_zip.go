package sourceacquisition

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"sort"
)

var archiveZIPOrder = binary.LittleEndian

// archiveValidateZIP supplements archive/zip with a strict single-stream
// boundary audit. archive/zip intentionally accepts SFX prefixes and trailers;
// neither is part of the package archive contract.
func archiveValidateZIP(ctx context.Context, file *os.File, size int64) (*zip.Reader, error) {
	if size < 22 || size > archiveMaxBytes {
		return nil, errors.New("invalid ZIP stream size")
	}
	tailSize := min(size, 22+65535)
	tail := make([]byte, tailSize)
	if _, err := file.ReadAt(tail, size-tailSize); err != nil {
		return nil, errors.New("read ZIP end record")
	}
	end := -1
	for index := len(tail) - 22; index >= 0; index-- {
		if string(tail[index:index+4]) == "PK\x05\x06" && index+22+int(archiveZIPOrder.Uint16(tail[index+20:index+22])) == len(tail) {
			end = index
			break
		}
	}
	if end < 0 {
		return nil, errors.New("ZIP requires an exact end record without trailing bytes")
	}
	eocd := tail[end:]
	count := int(archiveZIPOrder.Uint16(eocd[10:12]))
	directorySize := int64(archiveZIPOrder.Uint32(eocd[12:16]))
	directoryOffset := int64(archiveZIPOrder.Uint32(eocd[16:20]))
	if count == 65535 || directorySize == 0xffffffff || directoryOffset == 0xffffffff {
		return nil, errors.New("ZIP64 archives exceed the supported ZIP format limits")
	}
	if archiveZIPOrder.Uint16(eocd[4:6]) != 0 || archiveZIPOrder.Uint16(eocd[6:8]) != 0 || int(archiveZIPOrder.Uint16(eocd[8:10])) != count {
		return nil, errors.New("multi-volume ZIP archives are unsupported")
	}
	if count > 10000 || directoryOffset+directorySize != size-tailSize+int64(end) {
		return nil, errors.New("ZIP central directory has invalid bounds or too many entries")
	}
	reader, err := zip.NewReader(file, size)
	if err != nil || len(reader.File) != count {
		return nil, errors.New("invalid ZIP directory")
	}
	type region struct{ start, end int64 }
	regions := make([]region, 0, count)
	offset := directoryOffset
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var header [46]byte
		if offset+46 > directoryOffset+directorySize {
			return nil, errors.New("truncated ZIP directory")
		}
		if _, err := file.ReadAt(header[:], offset); err != nil || string(header[:4]) != "PK\x01\x02" {
			return nil, errors.New("invalid ZIP central header")
		}
		flags := archiveZIPOrder.Uint16(header[8:10])
		method := archiveZIPOrder.Uint16(header[10:12])
		compressed := int64(archiveZIPOrder.Uint32(header[20:24]))
		uncompressed := int64(archiveZIPOrder.Uint32(header[24:28]))
		localOffset := int64(archiveZIPOrder.Uint32(header[42:46]))
		nameLength := int64(archiveZIPOrder.Uint16(header[28:30]))
		extraLength := int64(archiveZIPOrder.Uint16(header[30:32]))
		commentLength := int64(archiveZIPOrder.Uint16(header[32:34]))
		next := offset + 46 + nameLength + extraLength + commentLength
		if compressed == 0xffffffff || uncompressed == 0xffffffff || localOffset == 0xffffffff || entry.ReaderVersion >= 45 {
			return nil, errors.New("ZIP64 archives exceed the supported ZIP format limits")
		}
		if next > directoryOffset+directorySize || archiveZIPOrder.Uint16(header[34:36]) != 0 || localOffset+30 > directoryOffset {
			return nil, errors.New("invalid ZIP entry bounds or disk")
		}
		if flags&^uint16(0x080e) != 0 || method != zip.Store && method != zip.Deflate || method == zip.Store && flags&6 != 0 {
			return nil, errors.New("ZIP supports only unencrypted store/deflate entries")
		}
		if uncompressed > 64<<20 || compressed > archiveMaxBytes || entry.UncompressedSize64 != uint64(uncompressed) || entry.CompressedSize64 != uint64(compressed) {
			return nil, errors.New("ZIP entry exceeds file byte limits")
		}
		central := make([]byte, nameLength+extraLength)
		if _, err := file.ReadAt(central, offset+46); err != nil || string(central[:nameLength]) != entry.Name {
			return nil, errors.New("invalid ZIP entry name")
		}
		if err := archiveZIPExtra(central[nameLength:]); err != nil {
			return nil, err
		}
		var local [30]byte
		if _, err := file.ReadAt(local[:], localOffset); err != nil || string(local[:4]) != "PK\x03\x04" {
			return nil, errors.New("ZIP requires local entries without a prefix")
		}
		localName := int64(archiveZIPOrder.Uint16(local[26:28]))
		localExtra := int64(archiveZIPOrder.Uint16(local[28:30]))
		dataOffset := localOffset + 30 + localName + localExtra
		if archiveZIPOrder.Uint16(local[4:6]) >= 45 || archiveZIPOrder.Uint16(local[6:8]) != flags || archiveZIPOrder.Uint16(local[8:10]) != method || dataOffset+compressed > directoryOffset || localName != nameLength {
			return nil, errors.New("inconsistent ZIP local header")
		}
		localBody := make([]byte, localName+localExtra)
		if _, err := file.ReadAt(localBody, localOffset+30); err != nil || !bytes.Equal(localBody[:localName], central[:nameLength]) {
			return nil, errors.New("inconsistent ZIP local name")
		}
		if err := archiveZIPExtra(localBody[localName:]); err != nil {
			return nil, err
		}
		regionEnd := dataOffset + compressed
		if flags&8 != 0 {
			var descriptor [16]byte
			if regionEnd+12 > directoryOffset {
				return nil, errors.New("truncated ZIP data descriptor")
			}
			n, readErr := file.ReadAt(descriptor[:], regionEnd)
			if readErr != nil && readErr != io.EOF || n < 12 {
				return nil, errors.New("truncated ZIP data descriptor")
			}
			values := descriptor[:12]
			descriptorSize := int64(12)
			if string(descriptor[:4]) == "PK\x07\x08" {
				values, descriptorSize = descriptor[4:], 16
			}
			if regionEnd+descriptorSize > directoryOffset || archiveZIPOrder.Uint32(values[:4]) != entry.CRC32 || int64(archiveZIPOrder.Uint32(values[4:8])) != compressed || int64(archiveZIPOrder.Uint32(values[8:12])) != uncompressed {
				return nil, errors.New("inconsistent ZIP data descriptor")
			}
			regionEnd += descriptorSize
		} else if archiveZIPOrder.Uint32(local[14:18]) != entry.CRC32 || int64(archiveZIPOrder.Uint32(local[18:22])) != compressed || int64(archiveZIPOrder.Uint32(local[22:26])) != uncompressed {
			return nil, errors.New("inconsistent ZIP local size or checksum")
		}
		regions = append(regions, region{localOffset, regionEnd})
		offset = next
	}
	if offset != directoryOffset+directorySize {
		return nil, errors.New("ZIP has extra central directory records")
	}
	sort.Slice(regions, func(i, j int) bool { return regions[i].start < regions[j].start })
	var position int64
	for _, region := range regions {
		if region.start != position {
			return nil, errors.New("ZIP has a prefix, gap, duplicate or overlapping local entry")
		}
		position = region.end
	}
	if position != directoryOffset {
		return nil, errors.New("ZIP has undeclared data before its directory")
	}
	return reader, nil
}

func archiveZIPExtra(body []byte) error {
	for len(body) > 0 {
		if len(body) < 4 {
			return errors.New("invalid ZIP extra record")
		}
		kind := archiveZIPOrder.Uint16(body[:2])
		size := int(archiveZIPOrder.Uint16(body[2:4]))
		if kind == 1 {
			return errors.New("ZIP64 records exceed the supported ZIP format limits")
		}
		if size > len(body)-4 {
			return errors.New("truncated ZIP extra record")
		}
		body = body[4+size:]
	}
	return nil
}

func archiveExtractZIP(ctx context.Context, file *os.File, size int64, output *archiveExtraction) error {
	reader, err := archiveValidateZIP(ctx, file, size)
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		mode := entry.Mode()
		if !mode.IsRegular() && !mode.IsDir() {
			return errors.New("ZIP refuses links and special entries")
		}
		body, err := entry.Open()
		if err != nil {
			return errors.New("open ZIP entry")
		}
		err = output.write(ctx, entry.Name, mode.IsDir(), int64(entry.UncompressedSize64), mode.Perm()&0o111 != 0, body)
		closeErr := body.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return errors.New("close ZIP entry")
		}
	}
	return nil
}
