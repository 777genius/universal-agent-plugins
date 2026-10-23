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
	count, directoryOffset, directorySize, err := archiveZIPDirectory(file, size)
	if err != nil {
		return nil, err
	}
	reader, err := zip.NewReader(file, size)
	if err != nil || len(reader.File) != count {
		return nil, errors.New("invalid ZIP directory")
	}
	regions := make([]archiveZIPRegion, 0, count)
	offset := directoryOffset
	for _, entry := range reader.File {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		record, err := archiveZIPCentral(file, offset, directoryOffset+directorySize, entry)
		if err != nil {
			return nil, err
		}
		region, err := archiveZIPLocal(file, record, directoryOffset, entry)
		if err != nil {
			return nil, err
		}
		regions = append(regions, region)
		offset = record.next
	}
	if offset != directoryOffset+directorySize {
		return nil, errors.New("ZIP has extra central directory records")
	}
	if err := archiveZIPRegions(regions, directoryOffset); err != nil {
		return nil, err
	}
	return reader, nil
}

func archiveZIPDirectory(file *os.File, size int64) (int, int64, int64, error) {
	if size < 22 || size > archiveMaxBytes {
		return 0, 0, 0, errors.New("invalid ZIP stream size")
	}
	tailSize := min(size, 22+65535)
	tail := make([]byte, tailSize)
	if _, err := file.ReadAt(tail, size-tailSize); err != nil {
		return 0, 0, 0, errors.New("read ZIP end record")
	}
	end := -1
	for index := len(tail) - 22; index >= 0; index-- {
		if string(tail[index:index+4]) == "PK\x05\x06" && index+22+int(archiveZIPOrder.Uint16(tail[index+20:index+22])) == len(tail) {
			end = index
			break
		}
	}
	if end < 0 {
		return 0, 0, 0, errors.New("ZIP requires an exact end record without trailing bytes")
	}
	eocd := tail[end:]
	count := int(archiveZIPOrder.Uint16(eocd[10:12]))
	directorySize := int64(archiveZIPOrder.Uint32(eocd[12:16]))
	directoryOffset := int64(archiveZIPOrder.Uint32(eocd[16:20]))
	if count == 65535 || directorySize == 0xffffffff || directoryOffset == 0xffffffff {
		return 0, 0, 0, errors.New("ZIP64 archives exceed the supported ZIP format limits")
	}
	if archiveZIPOrder.Uint16(eocd[4:6]) != 0 || archiveZIPOrder.Uint16(eocd[6:8]) != 0 || int(archiveZIPOrder.Uint16(eocd[8:10])) != count {
		return 0, 0, 0, errors.New("multi-volume ZIP archives are unsupported")
	}
	if count > 10000 || directoryOffset+directorySize != size-tailSize+int64(end) {
		return 0, 0, 0, errors.New("ZIP central directory has invalid bounds or too many entries")
	}
	return count, directoryOffset, directorySize, nil
}

type archiveZIPRegion struct{ start, end int64 }
type archiveZIPRecord struct {
	localOffset, next        int64
	compressed, uncompressed uint32
	flags, method            uint16
	name                     []byte
}

func archiveZIPCentral(file *os.File, offset, limit int64, entry *zip.File) (archiveZIPRecord, error) {
	var empty archiveZIPRecord
	var header [46]byte
	if offset+46 > limit {
		return empty, errors.New("truncated ZIP directory")
	}
	if _, err := file.ReadAt(header[:], offset); err != nil || string(header[:4]) != "PK\x01\x02" {
		return empty, errors.New("invalid ZIP central header")
	}
	record := archiveZIPRecord{
		flags: archiveZIPOrder.Uint16(header[8:10]), method: archiveZIPOrder.Uint16(header[10:12]),
		compressed: archiveZIPOrder.Uint32(header[20:24]), uncompressed: archiveZIPOrder.Uint32(header[24:28]),
		localOffset: int64(archiveZIPOrder.Uint32(header[42:46])),
	}
	nameLength := int64(archiveZIPOrder.Uint16(header[28:30]))
	extraLength := int64(archiveZIPOrder.Uint16(header[30:32]))
	commentLength := int64(archiveZIPOrder.Uint16(header[32:34]))
	record.next = offset + 46 + nameLength + extraLength + commentLength
	if record.next > limit || archiveZIPOrder.Uint16(header[34:36]) != 0 {
		return empty, errors.New("invalid ZIP entry bounds or disk")
	}
	if err := archiveZIPRecordLimits(record, entry); err != nil {
		return empty, err
	}
	body := make([]byte, nameLength+extraLength)
	if _, err := file.ReadAt(body, offset+46); err != nil || string(body[:nameLength]) != entry.Name {
		return empty, errors.New("invalid ZIP entry name")
	}
	if err := archiveZIPExtra(body[nameLength:]); err != nil {
		return empty, err
	}
	record.name = body[:nameLength]
	return record, nil
}

func archiveZIPRecordLimits(record archiveZIPRecord, entry *zip.File) error {
	if record.compressed == 0xffffffff || record.uncompressed == 0xffffffff || record.localOffset == 0xffffffff || entry.ReaderVersion >= 45 {
		return errors.New("ZIP64 archives exceed the supported ZIP format limits")
	}
	if record.flags&^uint16(0x080e) != 0 || record.method != zip.Store && record.method != zip.Deflate || record.method == zip.Store && record.flags&6 != 0 {
		return errors.New("ZIP supports only unencrypted store/deflate entries")
	}
	if record.uncompressed > 64<<20 || int64(record.compressed) > archiveMaxBytes || entry.UncompressedSize64 != uint64(record.uncompressed) || entry.CompressedSize64 != uint64(record.compressed) {
		return errors.New("ZIP entry exceeds file byte limits")
	}
	return nil
}

func archiveZIPLocal(file *os.File, record archiveZIPRecord, directoryOffset int64, entry *zip.File) (archiveZIPRegion, error) {
	var empty archiveZIPRegion
	var local [30]byte
	if record.localOffset+30 > directoryOffset {
		return empty, errors.New("invalid ZIP local bounds")
	}
	if _, err := file.ReadAt(local[:], record.localOffset); err != nil || string(local[:4]) != "PK\x03\x04" {
		return empty, errors.New("ZIP requires local entries without a prefix")
	}
	nameLength := int64(archiveZIPOrder.Uint16(local[26:28]))
	extraLength := int64(archiveZIPOrder.Uint16(local[28:30]))
	dataOffset := record.localOffset + 30 + nameLength + extraLength
	if archiveZIPOrder.Uint16(local[4:6]) >= 45 || archiveZIPOrder.Uint16(local[6:8]) != record.flags || archiveZIPOrder.Uint16(local[8:10]) != record.method || dataOffset+int64(record.compressed) > directoryOffset || nameLength != int64(len(record.name)) {
		return empty, errors.New("inconsistent ZIP local header")
	}
	body := make([]byte, nameLength+extraLength)
	if _, err := file.ReadAt(body, record.localOffset+30); err != nil || !bytes.Equal(body[:nameLength], record.name) {
		return empty, errors.New("inconsistent ZIP local name")
	}
	if err := archiveZIPExtra(body[nameLength:]); err != nil {
		return empty, err
	}
	regionEnd := dataOffset + int64(record.compressed)
	if record.flags&8 != 0 {
		descriptorSize, err := archiveZIPDescriptor(file, regionEnd, directoryOffset, entry)
		if err != nil {
			return empty, err
		}
		regionEnd += descriptorSize
	} else if archiveZIPOrder.Uint32(local[14:18]) != entry.CRC32 || archiveZIPOrder.Uint32(local[18:22]) != record.compressed || archiveZIPOrder.Uint32(local[22:26]) != record.uncompressed {
		return empty, errors.New("inconsistent ZIP local size or checksum")
	}
	return archiveZIPRegion{record.localOffset, regionEnd}, nil
}

func archiveZIPDescriptor(file *os.File, offset, limit int64, entry *zip.File) (int64, error) {
	var descriptor [16]byte
	if offset+12 > limit {
		return 0, errors.New("truncated ZIP data descriptor")
	}
	n, err := file.ReadAt(descriptor[:], offset)
	if err != nil && err != io.EOF || n < 12 {
		return 0, errors.New("truncated ZIP data descriptor")
	}
	values := descriptor[:12]
	size := int64(12)
	if string(descriptor[:4]) == "PK\x07\x08" {
		values, size = descriptor[4:], 16
	}
	if offset+size > limit || archiveZIPOrder.Uint32(values[:4]) != entry.CRC32 || uint64(archiveZIPOrder.Uint32(values[4:8])) != entry.CompressedSize64 || uint64(archiveZIPOrder.Uint32(values[8:12])) != entry.UncompressedSize64 {
		return 0, errors.New("inconsistent ZIP data descriptor")
	}
	return size, nil
}

func archiveZIPRegions(regions []archiveZIPRegion, directoryOffset int64) error {
	sort.Slice(regions, func(i, j int) bool { return regions[i].start < regions[j].start })
	var position int64
	for _, region := range regions {
		if region.start != position {
			return errors.New("ZIP has a prefix, gap, duplicate or overlapping local entry")
		}
		position = region.end
	}
	if position != directoryOffset {
		return errors.New("ZIP has undeclared data before its directory")
	}
	return nil
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
		if entry.UncompressedSize64 > 64<<20 {
			return errors.New("ZIP entry exceeds file byte limit")
		}
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
