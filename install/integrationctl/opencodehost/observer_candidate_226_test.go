package opencodehost_test

import (
	"testing"

	host "github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// Official 2.2.6 source-only observations are never private observer grants.
func TestOpenCode226PendingNativeCandidatesFailClosed(t *testing.T) {
	cells := []struct {
		os, arch, image string
	}{
		{"linux", "amd64", "50503d24577fa4117a7c95ee85b2cd679606a87eacf20ca54ed5db5c9e1cf895"},
		{"darwin", "arm64", "90e4d03597b03d2051b65b4d793467764e55bacd8e22bf4824479c7233ea0e15"},
	}
	for _, cell := range cells {
		t.Run(cell.os+"/"+cell.arch, func(t *testing.T) {
			d, ok := host.NativeObserverEvidenceForEntry("2.2.6", cell.os, cell.arch, cell.image, "official_native_serve_default_dual_autoload")
			if !ok || d.NativeStatus != "native_source_pending" || d.Tuple.Version != "2.2.6" ||
				d.Tuple.UpstreamCommit != "dc2ffcf8bf26f9e669b6aa22b95f50e12be15e06" ||
				d.Tuple.Adapter != host.ObserverV2 || d.Tuple.ImageSHA256 != cell.image ||
				d.Tuple.EvidenceID != "" || d.Tuple.EvidenceHashes != ([4]string{}) {
				t.Fatalf("candidate unexpectedly qualified or malformed: %+v", d)
			}
			for _, source := range []string{"host_runtime", "executable_version", "config", "event"} {
				e := host.VersionEvidence{Version: "2.2.6", Source: source, ProbeStatus: "ok", ExecutableIdentity: "TEST-held"}
				p := host.BindNativeObserver(e, d.Tuple)
				assertNoObserver(t, p, host.ObserverV2, d.Capabilities[:])
			}
			for _, changed := range []struct{ version, os, arch, image, entry string }{
				{"2.2.7", cell.os, cell.arch, cell.image, "official_native_serve_default_dual_autoload"},
				{"2.2.6", cell.os, cell.arch, "bad-image", "official_native_serve_default_dual_autoload"},
				{"2.2.6", cell.os, cell.arch, cell.image, "official_native_tui_default_dual_autoload"},
				{"2.2.6", cell.os, "unproved", cell.image, "official_native_serve_default_dual_autoload"},
			} {
				if got, ok := host.NativeObserverEvidenceForEntry(changed.version, changed.os, changed.arch, changed.image, changed.entry); ok || got != (host.NativeObserverDescriptor{}) {
					t.Fatalf("unknown native source identity inherited authority: %+v", got)
				}
			}
		})
	}
}
