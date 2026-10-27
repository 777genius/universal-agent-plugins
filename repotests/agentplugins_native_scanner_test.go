package pluginkitairepo_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The verified archive identifies the cache variant, not the installer producer.
// The Python source/frozen acquisition boundary independently binds that policy.
func nativeScannerInputPlatform(goos, goarch, actual, expected, archive string) (string, error) {
	if len(expected) != 64 || actual != expected {
		return "", errors.New("provisioned lintai binary digest mismatch")
	}
	pins := map[string]string{
		"darwin-arm64":     "be8b263e2323074080d928ea7c2129458299a6d03f7a9f178dfc1aa8e6bc17ff",
		"darwin-amd64":     "abc170612a847bf1a896ef85a4ee93977baa8275f303dac3ed27bf34050b7513",
		"linux-arm64":      "132a37610575bd251ecaf0be4c6090dad144dd1397c99aad989a3944c63c3d4a",
		"linux-amd64":      "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
		"linux-amd64-musl": "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b",
		"windows-amd64":    "2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911",
		"windows-arm64":    "484c30e7ef55310e0aec595c06870dbb5454ae7d29404ca797e00acad6011453",
	}
	platform := goos + "-" + goarch
	pin, ok := pins[platform]
	if !ok || (goarch != "amd64" && goarch != "arm64") {
		return "", fmt.Errorf("lintai security scanner has no pinned platform asset for %s", platform)
	}
	if platform == "linux-amd64" && archive == pins["linux-amd64-musl"] {
		platform += "-musl"
		pin = pins[platform]
	}
	if archive != pin {
		return "", errors.New("lintai archive provenance does not match pinned platform asset")
	}
	return platform, nil
}

func TestAgentpluginsNativeFixtureScannerInputAdmission(t *testing.T) {
	digest := strings.Repeat("a", 64)
	cases := []struct {
		goos, goarch, archive, platform string
	}{
		{"linux", "amd64", "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", "linux-amd64"},
		{"linux", "amd64", "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b", "linux-amd64-musl"},
		{"linux", "arm64", "132a37610575bd251ecaf0be4c6090dad144dd1397c99aad989a3944c63c3d4a", "linux-arm64"},
		{"darwin", "amd64", "abc170612a847bf1a896ef85a4ee93977baa8275f303dac3ed27bf34050b7513", "darwin-amd64"},
		{"darwin", "arm64", "be8b263e2323074080d928ea7c2129458299a6d03f7a9f178dfc1aa8e6bc17ff", "darwin-arm64"},
		{"windows", "amd64", "2f61f6a83a160afa3feed9ea1722b82d0d938ebff865a4e20d39b5f55270c911", "windows-amd64"},
		{"windows", "arm64", "484c30e7ef55310e0aec595c06870dbb5454ae7d29404ca797e00acad6011453", "windows-arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.platform, func(t *testing.T) {
			platform, err := nativeScannerInputPlatform(tc.goos, tc.goarch, digest, digest, tc.archive)
			if err != nil || platform != tc.platform {
				t.Fatalf("scanner platform = %q, %v; want %s", platform, err, tc.platform)
			}
			for _, bad := range [][3]string{
				{digest, "", tc.archive}, {digest, strings.Repeat("b", 64), tc.archive},
				{digest, digest, ""}, {digest, digest, strings.Repeat("0", 64)},
			} {
				if _, err := nativeScannerInputPlatform(tc.goos, tc.goarch, bad[0], bad[1], bad[2]); err == nil {
					t.Fatal("unverified scanner input admitted")
				}
			}
		})
	}
	for _, target := range [][2]string{{"plan9", "amd64"}, {"linux", "amd64-musl"}, {"linux", "riscv64"}} {
		if _, err := nativeScannerInputPlatform(target[0], target[1], digest, digest, cases[1].archive); err == nil {
			t.Fatalf("unsupported scanner OS/CPU admitted: %v", target)
		}
	}
}

func TestAgentpluginsNativeFixtureScannerCacheVariants(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("two-cache Linux amd64 fixture; declared non-Linux mappings tested independently")
	}
	variants := map[string]string{
		"linux-amd64":      "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da",
		"linux-amd64-musl": "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b",
	}
	for platform, archive := range variants {
		t.Run(platform, func(t *testing.T) {
			f := &nativeFixture{Root: t.TempDir()}
			binary := filepath.Join(f.Root, "inert-lintai")
			// These ordinary bytes are copied and hashed only; never executed.
			body := []byte("inert scanner input for " + platform)
			nativeWrite(t, binary, body, 0o700)
			t.Setenv("AGENTPLUGINS_LINTAI_BIN", binary)
			t.Setenv("AGENTPLUGINS_LINTAI_SHA256", nativeSHA(t, binary))
			t.Setenv("AGENTPLUGINS_LINTAI_ARCHIVE_SHA256", archive)
			cache := filepath.Join(f.Root, "installer-state", "security", "lintai", "0.1.3")
			for other := range variants {
				if other != platform {
					nativeWrite(t, filepath.Join(cache, other, "lintai"), []byte("other cache unchanged"), 0o700)
				}
			}
			receipt := nativeProvisionScanner(t, f)
			if receipt["platform"] != platform || receipt["archive_sha256"] != archive || receipt["binary_sha256"] != nativeSHA(t, binary) {
				t.Fatalf("incorrect scanner receipt: %+v", receipt)
			}
			for selected := range variants {
				got, err := os.ReadFile(filepath.Join(cache, selected, "lintai"))
				want := "other cache unchanged"
				if selected == platform {
					want = string(body)
				}
				if err != nil || string(got) != want {
					t.Fatalf("cache %s = %q, %v; want %q", selected, got, err, want)
				}
			}
		})
	}
}
