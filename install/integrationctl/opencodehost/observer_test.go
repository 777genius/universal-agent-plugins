package opencodehost_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	host "github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// Expected image pins are the supplied official-artifact contract, independent
// of the implementation. All assertions below exercise the public authority.
var nativeHosts = []struct {
	version, image string
	adapter        host.AdapterID
	reader, basis  string
}{
	{"1.18.33", "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427", host.ObserverV1, "v1_source_causal_sync_callback_tombstones", "v1_assistant_completed_or_assistant_created_lower_bound"},
	{"1.18.34", "9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2", host.ObserverV1, "v1_source_causal_sync_callback_tombstones", "v1_assistant_completed_or_assistant_created_lower_bound"},
	{"2.0.21", "f916986543348d7953d8d43aa048516cdbc3f84f4d0dc9c0c5b9d1da3030cea7", host.ObserverV2, "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare", "v2_native_envelope_created"},
}

func runtimeEvidence(version string) host.VersionEvidence {
	return host.VersionEvidence{Version: version, Source: "host_runtime", ProbeStatus: "ok", ExecutableIdentity: "private-live-instance"}
}

func observerRequirements(adapter host.AdapterID, facts ...host.Capability) []host.ArtifactRequirement {
	return []host.ArtifactRequirement{{ID: "native-observer", Adapter: adapter, Required: append([]host.Capability{host.LocalPluginDual}, facts...)}}
}

func assertNoObserver(t *testing.T, p host.Profile, adapter host.AdapterID, facts []host.Capability) {
	t.Helper()
	for _, fact := range facts {
		if p.Capabilities[fact] == host.Supported {
			t.Fatalf("unexpected snapshot grant %s: %+v", fact, p)
		}
	}
	s, err := host.Select(p, observerRequirements(adapter, facts...))
	if err == nil || s.Adapter != "" || s.ArtifactID != "" {
		t.Fatalf("unexpected authority: %+v %v", s, err)
	}
}

func TestVersionLookupNeverGrantsObserver(t *testing.T) {
	for _, tc := range nativeHosts {
		d, ok := host.NativeObserverEvidence(tc.version)
		if !ok || d.Tuple.ImageSHA256 != tc.image || d.Tuple.Adapter != tc.adapter || d.Tuple.ReaderContract != tc.reader || d.Tuple.ProvenanceBasis != tc.basis || d.NativeStatus != "native_source_qualified" {
			t.Fatalf("source descriptor %s: %+v", tc.version, d)
		}
		for _, source := range []string{"executable_version", "host_runtime"} {
			e := runtimeEvidence(tc.version)
			e.Source = source
			p := host.Resolve(e)
			for _, fact := range d.Capabilities {
				if p.Capabilities[fact] != host.Unverified {
					t.Fatalf("plain Resolve %s/%s grants %s", tc.version, source, fact)
				}
			}
			s, err := host.Select(p, observerRequirements(tc.adapter, d.Capabilities[:]...))
			if !errors.Is(err, host.ErrUnverifiedCapability) || s.Adapter != "" || s.ArtifactID != "" || len(s.Unverified) != 4 {
				t.Fatalf("version-only: %+v %v", s, err)
			}
		}
	}
}

func TestExactNativeBindingsSelectAllAndIndividualFacts(t *testing.T) {
	for _, tc := range nativeHosts {
		t.Run(tc.version, func(t *testing.T) {
			d, _ := host.NativeObserverEvidence(tc.version)
			p := host.BindNativeObserver(runtimeEvidence(tc.version), d.Tuple)
			for _, fact := range d.Capabilities {
				if p.Capabilities[fact] != host.Supported {
					t.Fatalf("missing native fact %s", fact)
				}
				s, err := host.Select(p, observerRequirements(tc.adapter, fact))
				if err != nil || s.Adapter != tc.adapter || s.ArtifactID != "native-observer" {
					t.Fatalf("individual %s: %+v %v", fact, s, err)
				}
			}
			s, err := host.Select(p, observerRequirements(tc.adapter, d.Capabilities[:]...))
			if err != nil || s.Adapter != tc.adapter || s.ArtifactID != "native-observer" {
				t.Fatalf("all four: %+v %v", s, err)
			}
			other := host.ObserverV1
			if tc.adapter == other {
				other = host.ObserverV2
			}
			if s, err := host.Select(p, observerRequirements(other, d.Capabilities[:]...)); !errors.Is(err, host.ErrNoAdapter) || s.Adapter != "" {
				t.Fatalf("wrong family adapter: %+v %v", s, err)
			}
		})
	}
}

func TestNativeTupleMismatchAndMissingAuthorityFailClosed(t *testing.T) {
	mutations := map[string]func(*host.NativeObserverTuple){
		"image":            func(v *host.NativeObserverTuple) { v.ImageSHA256 = "unproved-image" },
		"image-absent":     func(v *host.NativeObserverTuple) { v.ImageSHA256 = "" },
		"tui":              func(v *host.NativeObserverTuple) { v.Entry = "tui" },
		"desktop":          func(v *host.NativeObserverTuple) { v.Entry = "desktop" },
		"wrapper":          func(v *host.NativeObserverTuple) { v.Entry = "wrapper_serve" },
		"entry-absent":     func(v *host.NativeObserverTuple) { v.Entry = "" },
		"arm64":            func(v *host.NativeObserverTuple) { v.GOARCH = "arm64" },
		"darwin":           func(v *host.NativeObserverTuple) { v.GOOS = "darwin" },
		"windows":          func(v *host.NativeObserverTuple) { v.GOOS = "windows" },
		"os-absent":        func(v *host.NativeObserverTuple) { v.GOOS = "" },
		"arch-absent":      func(v *host.NativeObserverTuple) { v.GOARCH = "" },
		"reader":           func(v *host.NativeObserverTuple) { v.ReaderContract = "external_sse_replay" },
		"reader-absent":    func(v *host.NativeObserverTuple) { v.ReaderContract = "" },
		"basis":            func(v *host.NativeObserverTuple) { v.ProvenanceBasis = "receive_time" },
		"basis-absent":     func(v *host.NativeObserverTuple) { v.ProvenanceBasis = "" },
		"adapter":          func(v *host.NativeObserverTuple) { v.Adapter = host.ConfigV1 },
		"adapter-contract": func(v *host.NativeObserverTuple) { v.NativeAdapterSHA256 = "" },
		"source":           func(v *host.NativeObserverTuple) { v.UpstreamCommit = "" },
		"evidence-id":      func(v *host.NativeObserverTuple) { v.EvidenceID = "" },
		"version":          func(v *host.NativeObserverTuple) { v.Version = "2.0.0" },
	}
	for _, tc := range nativeHosts {
		d, _ := host.NativeObserverEvidence(tc.version)
		for name, mutate := range mutations {
			t.Run(tc.version+"/"+name, func(t *testing.T) {
				actual := d.Tuple
				mutate(&actual)
				p := host.BindNativeObserver(runtimeEvidence(tc.version), actual)
				if !reflect.DeepEqual(p, host.Resolve(runtimeEvidence(tc.version))) {
					t.Fatal("rejected tuple changed generic status or granted a fact")
				}
				assertNoObserver(t, p, tc.adapter, d.Capabilities[:])
			})
		}
		for i := range d.Tuple.EvidenceHashes {
			actual := d.Tuple
			actual.EvidenceHashes[i] = ""
			assertNoObserver(t, host.BindNativeObserver(runtimeEvidence(tc.version), actual), tc.adapter, d.Capabilities[:])
		}
		for _, other := range nativeHosts {
			if other.version != tc.version {
				foreign, _ := host.NativeObserverEvidence(other.version)
				assertNoObserver(t, host.BindNativeObserver(runtimeEvidence(tc.version), foreign.Tuple), tc.adapter, d.Capabilities[:])
			}
		}
		assertNoObserver(t, host.BindNativeObserver(runtimeEvidence(tc.version), host.NativeObserverTuple{}), tc.adapter, d.Capabilities[:])
		for _, source := range []string{"executable_version", "explicit_version", "config", "event", ""} {
			e := runtimeEvidence(tc.version)
			e.Source = source
			assertNoObserver(t, host.BindNativeObserver(e, d.Tuple), tc.adapter, d.Capabilities[:])
		}
		for _, status := range []string{"absent", "failed", "timed_out", "malformed", "output_limit", "not_requested", ""} {
			e := runtimeEvidence(tc.version)
			e.ProbeStatus = status
			assertNoObserver(t, host.BindNativeObserver(e, d.Tuple), tc.adapter, d.Capabilities[:])
		}
		e := runtimeEvidence(tc.version)
		e.ExecutableIdentity = ""
		assertNoObserver(t, host.BindNativeObserver(e, d.Tuple), tc.adapter, d.Capabilities[:])
	}
}

func TestNoNativeVersionInheritance(t *testing.T) {
	d, _ := host.NativeObserverEvidence("2.0.21")
	for _, version := range []string{"", "2.0", "1.18.29", "1.18.32", "1.18.35", "1.19.0", "2.0.0", "2.0.22", "2.1.0", "3.0.0", "1.18.33-rc.1", "1.18.34+build", "2.0.21-rc.1", "2.0.21+build"} {
		if snapshot, ok := host.NativeObserverEvidence(version); ok || snapshot != (host.NativeObserverDescriptor{}) {
			t.Fatalf("inherited source evidence %s", version)
		}
		actual := d.Tuple
		actual.Version = version
		p := host.BindNativeObserver(runtimeEvidence(version), actual)
		assertNoObserver(t, p, host.ObserverV1, d.Capabilities[:])
		assertNoObserver(t, p, host.ObserverV2, d.Capabilities[:])
	}
}

func TestDetachedSnapshotsAndSerializedFlagsCannotGrantAuthority(t *testing.T) {
	for _, tc := range nativeHosts {
		d, _ := host.NativeObserverEvidence(tc.version)
		original := d
		p := host.BindNativeObserver(runtimeEvidence(tc.version), d.Tuple)
		d.Tuple.ImageSHA256 = "tampered"
		d.Tuple.EvidenceHashes[0] = "tampered"
		d.Capabilities[0] = host.MCPStdio
		fresh, _ := host.NativeObserverEvidence(tc.version)
		if fresh != original {
			t.Fatal("descriptor snapshot aliases authority")
		}
		clone := p.Clone()
		for _, fact := range original.Capabilities {
			clone.Capabilities[fact] = host.Unverified
			if p.Capabilities[fact] != host.Supported {
				t.Fatal("profile clone aliases map")
			}
		}
		p.Capabilities[host.ObserverCompletion] = host.SupportUnsupported
		if s, err := host.Select(p, observerRequirements(tc.adapter, original.Capabilities[:]...)); err != nil || s.Adapter != tc.adapter {
			t.Fatalf("public flags overrode private authority: %+v %v", s, err)
		}
		plain := host.Resolve(runtimeEvidence(tc.version))
		for _, fact := range original.Capabilities {
			plain.Capabilities[fact] = host.Supported
		}
		encoded, err := json.Marshal(host.BindNativeObserver(runtimeEvidence(tc.version), original.Tuple))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(encoded), "private-live-instance") {
			t.Fatal("serialized profile leaks runtime identity")
		}
		var serialized host.Profile
		if err := json.Unmarshal(encoded, &serialized); err != nil {
			t.Fatal(err)
		}
		for _, forged := range []host.Profile{plain, serialized} {
			if s, err := host.Select(forged, observerRequirements(tc.adapter, original.Capabilities[:]...)); !errors.Is(err, host.ErrUnverifiedCapability) || s.Adapter != "" || s.ArtifactID != "" {
				t.Fatalf("public/serialized flags grant observer: %+v %v", s, err)
			}
		}
		p.Version = "2.0.0"
		if s, err := host.Select(p, observerRequirements(tc.adapter, original.Capabilities[:]...)); err == nil || s.Adapter != "" {
			t.Fatalf("profile version swap retained authority: %+v %v", s, err)
		}
		assertNoObserver(t, host.Resolve(runtimeEvidence(tc.version)), tc.adapter, original.Capabilities[:])
		qualified := host.BindNativeObserver(runtimeEvidence(tc.version), fresh.Tuple)
		if s, err := host.Select(qualified, observerRequirements(tc.adapter, original.Capabilities[:]...)); err != nil || s.Adapter != tc.adapter {
			t.Fatalf("snapshot mutation affected future decision: %+v %v", s, err)
		}
	}
}

func TestBindingPreservesGenericSelectorAndEvidence(t *testing.T) {
	for _, tc := range nativeHosts {
		d, _ := host.NativeObserverEvidence(tc.version)
		plain := host.Resolve(runtimeEvidence(tc.version))
		bound := host.BindNativeObserver(runtimeEvidence(tc.version), d.Tuple)
		if bound.EvidenceID != plain.EvidenceID || bound.Qualification != plain.Qualification || bound.Reason != plain.Reason {
			t.Fatal("observer qualification overwrote generic evidence semantics")
		}
		for _, adapter := range []host.AdapterID{host.DualPlacement, host.SkillDirectory, host.ConfigV1, host.ConfigV2} {
			var required []host.Capability
			switch adapter {
			case host.SkillDirectory:
				required = []host.Capability{host.GlobalSkillDirectory}
			case host.ConfigV1, host.ConfigV2:
				required = []host.Capability{host.MCPStdio, host.MCPStreamableHTTP}
			}
			req := []host.ArtifactRequirement{{ID: "generic", Adapter: adapter, Required: required}}
			a, ae := host.Select(plain, req)
			b, be := host.Select(bound, req)
			if !reflect.DeepEqual(a, b) || !errors.Is(ae, be) {
				t.Fatalf("generic %s changed: %+v %v => %+v %v", adapter, a, ae, b, be)
			}
		}
	}
}
