package app

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestResolveFlagBundleChecksumParsesInlineHash(t *testing.T) {
	t.Parallel()

	sum, source, err := resolveFlagBundleChecksum(strings.Repeat("a", 64))
	if err != nil {
		t.Fatalf("resolveFlagBundleChecksum: %v", err)
	}
	if source != "flag --sha256" || hex.EncodeToString(sum) != strings.Repeat("a", 64) {
		t.Fatalf("sum/source = %q %q", hex.EncodeToString(sum), source)
	}
}

func TestChecksumLinesSkipsBlankEntries(t *testing.T) {
	t.Parallel()

	lines := checksumLines([]byte(" \nabc\n\n def \n"))
	if len(lines) != 2 || lines[0] != "abc" || lines[1] != "def" {
		t.Fatalf("lines = %#v", lines)
	}
}
