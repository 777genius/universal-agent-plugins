package sourceacquisition

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

// archiveGunzip validates one member (including CRC/ISIZE) and preserves the
// exact next byte so concatenated members and outer trailers cannot disappear
// into gzip's buffering. Expansion is bounded before writing the TAR stream.
func archiveGunzip(ctx context.Context, compressed, destination string) error {
	input, err := os.Open(compressed)
	if err != nil {
		return err
	}
	defer input.Close()
	buffer := bufio.NewReader(&archiveContextReader{ctx: ctx, reader: input})
	reader, err := gzip.NewReader(buffer)
	if err != nil {
		return errors.New("invalid gzip archive header")
	}
	defer reader.Close()
	reader.Multistream(false)
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(output, io.LimitReader(reader, archiveMaxBytes+1))
	closeErr := output.Close()
	if n > archiveMaxBytes {
		return errors.New("gzip archive exceeds 256 MiB expanded stream limit")
	}
	if copyErr != nil || closeErr != nil {
		return errors.New("invalid or truncated gzip archive (checksum/trailer required)")
	}
	if _, err := buffer.ReadByte(); err != io.EOF {
		return errors.New("gzip archive has a second member or trailing bytes")
	}
	return nil
}

// archiveValidateTAR audits raw framing before tar.Reader can hide metadata
// records or accept missing end blocks. PAX path/size overrides are interpreted
// again by tar.Reader during extraction; their effective paths use one policy.
func archiveValidateTAR(ctx context.Context, file *os.File, size int64) error {
	if size > archiveMaxBytes || size%512 != 0 {
		return errors.New("TAR stream exceeds its byte limit or has an unaligned ending")
	}
	var offset, expanded int64
	paxSize := int64(-1)
	for entries := 0; ; entries++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		var block [512]byte
		if _, err := file.ReadAt(block[:], offset); err != nil {
			return errors.New("TAR requires two zero end blocks")
		}
		if archiveZero(block[:]) {
			if offset+1024 > size {
				return errors.New("TAR requires two zero end blocks")
			}
			return archiveZeroTail(ctx, io.NewSectionReader(file, offset, size-offset))
		}
		if entries >= 10000 {
			return errors.New("TAR exceeds 10000 entries including metadata")
		}
		declared, err := archiveTarNumber(block[124:136])
		if err != nil || declared > 64<<20 {
			return errors.New("TAR has invalid or oversized declared entry size")
		}
		typeflag := block[156]
		entrySize := declared
		metadata := typeflag == tar.TypeXHeader || typeflag == tar.TypeGNULongName
		if !metadata {
			if paxSize >= 0 {
				entrySize = paxSize
			}
			paxSize = -1
			switch typeflag {
			case tar.TypeReg, tar.TypeRegA:
			case tar.TypeDir:
				if entrySize != 0 {
					return errors.New("TAR directory has nonzero size")
				}
			default:
				return errors.New("TAR refuses links, sparse files, global metadata and special entries")
			}
		}
		if entrySize < 0 || entrySize > 64<<20 || entrySize > archiveMaxBytes-expanded {
			return errors.New("TAR exceeds file or expanded byte limits")
		}
		expanded += entrySize
		padded := (entrySize + 511) / 512 * 512
		if offset+512+padded > size {
			return errors.New("truncated TAR entry")
		}
		if metadata {
			// Go's TAR parser has the same metadata bound. Enforce it before
			// allocation and account for every hidden metadata entry above.
			if entrySize > 1<<20 {
				return errors.New("TAR metadata exceeds 1 MiB")
			}
			if typeflag == tar.TypeXHeader {
				body := make([]byte, entrySize)
				if _, err := file.ReadAt(body, offset+512); err != nil {
					return errors.New("truncated TAR metadata")
				}
				paxSize, err = archivePAXSize(string(body))
				if err != nil {
					return err
				}
			}
		}
		offset += 512 + padded
	}
}

func archivePAXSize(body string) (int64, error) {
	size := int64(-1)
	for body != "" {
		prefix, _, ok := strings.Cut(body, " ")
		length, err := strconv.Atoi(prefix)
		if !ok || err != nil || length <= len(prefix)+3 || length > len(body) || body[length-1] != '\n' {
			return 0, errors.New("invalid TAR PAX record")
		}
		key, value, ok := strings.Cut(body[len(prefix)+1:length-1], "=")
		if !ok || key == "" || strings.Contains(key, "\x00") || strings.HasPrefix(key, "GNU.sparse") || key == "SCHILY.filetype" && value == "sparse" {
			return 0, errors.New("unsupported TAR PAX metadata")
		}
		if key == "size" {
			size, err = strconv.ParseInt(value, 10, 64)
			if err != nil || size < 0 {
				return 0, errors.New("invalid TAR PAX size")
			}
		}
		body = body[length:]
	}
	return size, nil
}

func archiveTarNumber(body []byte) (int64, error) {
	if len(body) > 0 && body[0]&0x80 != 0 {
		if body[0]&0x40 != 0 {
			return 0, errors.New("negative TAR size")
		}
		value := int64(body[0] & 0x3f)
		for _, digit := range body[1:] {
			if value > (math.MaxInt64-int64(digit))/256 {
				return 0, errors.New("overflowing TAR size")
			}
			value = value*256 + int64(digit)
		}
		return value, nil
	}
	text := strings.Trim(string(body), " \x00")
	if text == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(text, 8, 64)
	if err != nil || value < 0 {
		return 0, errors.New("invalid TAR size")
	}
	return value, nil
}

func archiveZeroTail(ctx context.Context, reader io.Reader) error {
	var buffer [32 << 10]byte
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := reader.Read(buffer[:])
		if !archiveZero(buffer[:n]) {
			return errors.New("TAR has nonzero trailing bytes or a second archive")
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read TAR end: %w", err)
		}
	}
}

func archiveExtractTAR(ctx context.Context, file *os.File, size int64, output *archiveExtraction) error {
	if err := archiveValidateTAR(ctx, file, size); err != nil {
		return err
	}
	reader := tar.NewReader(&archiveContextReader{ctx: ctx, reader: io.NewSectionReader(file, 0, size)})
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.New("invalid TAR header or payload")
		}
		if err := output.write(ctx, header.Name, header.Typeflag == tar.TypeDir, header.Size, header.Mode&0o111 != 0, reader); err != nil {
			return err
		}
	}
}
