//go:build linux

package process

import (
	"strings"
	"testing"
)

const readonlyMountDiagnosticRow = "20 1 8:2 /private/source /usr ro,nosuid,nodev - ext4 /private/device rw"

func TestReadonlyChildMountDiagnosticPreservesAcceptedRows(t *testing.T) {
	for _, line := range []string{readonlyMountDiagnosticRow, strings.Replace(readonlyMountDiagnosticRow, " - ", " unbindable - ", 1)} {
		row, err := readonlyChildMountLine(line)
		if err != nil || row.path != "/usr" || row.fs != "ext4" {
			t.Fatalf("ordinary accepted mount row changed: %+v %v", row, err)
		}
	}
}

func TestReadonlyChildMountDiagnosticIdentifiesOriginalRefusal(t *testing.T) {
	for _, item := range []struct {
		name, replace, with, expected string
	}{
		{"master", " - ", " master:42 - ", "field_count=11 expected_separator=6 observed_separator=7 tail_count=3 empty_fields=0x0000 optional_classes=[master] role=runtime"},
		{"shared master", " - ", " shared:41 master:42 - ", "field_count=12 expected_separator=6 observed_separator=8 tail_count=3 empty_fields=0x0000 optional_classes=[shared,master] role=runtime"},
		{"propagation", " - ", " master:42 propagate_from:41 - ", "optional_classes=[master,propagate_from] role=runtime"},
		{"idmapped", " - ", " idmapped - ", "optional_classes=[idmapped] role=runtime"},
		{"empty optional", " - ", "  - ", "observed_separator=7 tail_count=3 empty_fields=0x0040 optional_classes=[empty]"},
		{"empty filesystem", "- ext4 ", "-  ", "observed_separator=6 tail_count=3 empty_fields=0x0080 optional_classes=[]"},
		{"empty source", " /private/device ", "  ", "observed_separator=6 tail_count=3 empty_fields=0x0100 optional_classes=[]"},
		{"empty options", " rw", " ", "observed_separator=6 tail_count=3 empty_fields=0x0200 optional_classes=[]"},
		{"missing separator", " - ", " invalid ", "observed_separator=-1 tail_count=-1 empty_fields=0x0000 optional_classes=[unknown,unknown,unknown,unknown]"},
		{"extra separator", " - ", " - - ", "observed_separator=6 tail_count=4 empty_fields=0x0000 optional_classes=[]"},
		{"malformed tag", " - ", " master:not-an-id - ", "optional_classes=[master] role=runtime"},
	} {
		t.Run(item.name, func(t *testing.T) {
			_, err := readonlyChildMountLine(strings.Replace(readonlyMountDiagnosticRow, item.replace, item.with, 1))
			if err == nil || !strings.HasPrefix(err.Error(), "readonly child mount propagation or filesystem fields differ: ") || !strings.Contains(err.Error(), item.expected) {
				t.Fatalf("original refusal lost its structural cause: %v", err)
			}
		})
	}
}

func TestReadonlyChildMountDiagnosticNeverExposesSourceContent(t *testing.T) {
	secret := "never-log-this-" + strings.Repeat("\x00\x1b\n雪", 2048)
	for _, point := range []string{"/", "/proc", "/dev", "/dev/pts", "/usr", readonlyBootstrapPath, "/" + secret} {
		for _, separated := range []bool{false, true} {
			fields := []string{"20", "1", "8:2", "/" + secret, point, "ro,nosuid,nodev"}
			count := 10
			if separated {
				count = 6
			}
			for range count {
				fields = append(fields, "propagate_from:"+secret)
			}
			if separated {
				fields = append(fields, "-", secret, secret, secret)
			}
			_, err := readonlyChildMountLine(strings.Join(fields, " "))
			if err == nil || len(err.Error()) > 384 || strings.Contains(err.Error(), "never-log-this-") || strings.Contains(err.Error(), "/") {
				t.Fatalf("mount diagnostic leaked content or exceeded its bound: %v", err)
			}
			for _, value := range err.Error() {
				if value < 32 || value > 126 {
					t.Fatalf("mount diagnostic contains a non-ASCII or control character: %q", err)
				}
			}
		}
	}
	_, err := readonlyChildMountLine(strings.Replace(readonlyMountDiagnosticRow, " - ", " "+secret+" - ", 1))
	if err == nil || !strings.Contains(err.Error(), "optional_classes=[unknown]") || strings.Contains(err.Error(), "never-log-this-") {
		t.Fatalf("unknown mount token escaped its finite class: %v", err)
	}
}
