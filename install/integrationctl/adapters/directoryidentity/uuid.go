package directoryidentity

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
)

func decodeLinuxUUID(b []byte) (string, error) {
	if len(b) != 17 || b[0] < 1 || b[0] > 16 {
		return "", errors.New("malformed filesystem UUID length")
	}
	meaningful := b[1 : 1+int(b[0])]
	if allZero(meaningful) {
		return "", errors.New("zero filesystem UUID")
	}
	return hex.EncodeToString(meaningful), nil
}

// Darwin target-ABI packed record: uint32 length, attribute_set_t (5 uint32),
// then the requested UUID. No native alignment padding belongs in this buffer.
const (
	darwinReturnedAttrs = 0x80000000
	darwinVolumeInfo    = 0x80000000
	darwinVolumeUUID    = 0x00040000
)

func decodeDarwinUUID(b []byte) (string, error) {
	if len(b) < 24 || len(b) > 40 {
		return "", errors.New("malformed volume attribute buffer")
	}
	n := int(binary.LittleEndian.Uint32(b[:4]))
	if n < 24 || n > len(b) {
		return "", errors.New("malformed packed attribute length")
	}
	common, volume := binary.LittleEndian.Uint32(b[4:8]), binary.LittleEndian.Uint32(b[8:12])
	if common != darwinReturnedAttrs || volume & ^uint32(darwinVolumeInfo|darwinVolumeUUID) != 0 || !allZero(b[12:24]) {
		return "", errors.New("unexpected returned volume attributes")
	}
	if volume&darwinVolumeUUID == 0 {
		if n != 24 {
			return "", errors.New("malformed absent volume UUID")
		}
		return "", ErrUnsupported
	}
	if n != 40 || allZero(b[24:40]) {
		return "", errors.New("malformed or zero volume UUID")
	}
	return hex.EncodeToString(b[24:40]), nil
}
