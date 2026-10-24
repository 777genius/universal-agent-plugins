package sourcespec

import (
	"net/url"
	"strings"
	"testing"
)

func TestArchiveHTTPSAndLocalSelectorsDecodeExactlyOnce(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	for _, selected := range []string{"plugins/c++", "plugins/a&b", "plugins/%41", "plugins/a/b", "plugins/日本語", "ordinary"} {
		t.Run(selected, func(t *testing.T) {
			selector := "#sha256=" + checksum + "&path=" + url.QueryEscape(selected)
			for _, endpoint := range []string{"https://packages.test/plugin.zip", "/tmp/plugin.zip"} {
				spec, err := Parse(endpoint + selector)
				if err != nil || spec.Archive == nil || spec.Archive.Path != selected || spec.Archive.SHA256 != checksum || strings.Contains(spec.Archive.URL, "#") {
					t.Fatalf("selector %s: %+v %v", endpoint, spec, err)
				}
			}
		})
	}
	for _, fragment := range []string{
		"sha256=" + checksum + "&path=a%ZZ", "sha256=" + checksum + "&path=a&path=b",
		"sha256=" + checksum + "&unknown=x", "sha256=short&path=a", "sha256=" + checksum + "&path=../outside",
	} {
		if _, err := Parse("https://packages.test/plugin.zip#" + fragment); err == nil {
			t.Fatalf("invalid archive fragment admitted: %s", fragment)
		}
	}
}

func TestArchiveSelectorsPreserveLiteralPlus(t *testing.T) {
	checksum := strings.Repeat("b", 64)
	for _, endpoint := range []string{"https://packages.test/plugin.zip", "/tmp/plugin.zip"} {
		for _, spelling := range []string{"plugins/c++", "plugins/c%2B%2B"} {
			spec, err := Parse(endpoint + "#sha256=" + checksum + "&path=" + spelling)
			if err != nil || spec.Archive == nil || spec.Archive.Path != "plugins/c++" {
				t.Fatalf("literal plus selector changed: %+v %v", spec, err)
			}
		}
	}
}
