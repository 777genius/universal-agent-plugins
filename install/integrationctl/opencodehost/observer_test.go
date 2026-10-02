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

// RED if the compatibility wrapper loses an independent qualified Linux cell,
// exact lookup differs from its legacy descriptor, or binding loses a capability.
func TestExactImageLookupPreservesQualifiedLinuxCompatibilityAndBinding(t *testing.T) {
	for _, tc := range nativeHosts {
		t.Run(tc.version, func(t *testing.T) {
			legacy, ok := host.NativeObserverEvidence(tc.version)
			if !ok || legacy.Tuple.Version != tc.version || legacy.Tuple.ImageSHA256 != tc.image || legacy.Tuple.GOOS != "linux" || legacy.Tuple.GOARCH != "amd64" || legacy.Tuple.Adapter != tc.adapter || legacy.Tuple.ReaderContract != tc.reader || legacy.Tuple.ProvenanceBasis != tc.basis || legacy.NativeStatus != "native_source_qualified" || legacy.Capabilities != observerFacts {
				t.Fatalf("lost compatibility descriptor: %+v", legacy)
			}
			d, ok := host.NativeObserverEvidenceForImage(tc.version, "linux", "amd64", tc.image)
			if !ok || d != legacy {
				t.Fatalf("exact Linux descriptor differs: %+v", d)
			}
			p := host.BindNativeObserver(runtimeEvidence(tc.version), d.Tuple)
			for _, fact := range observerFacts {
				selection, err := host.Select(p, observerRequirements(tc.adapter, fact))
				if err != nil || selection.Adapter != tc.adapter || selection.ArtifactID != "native-observer" {
					t.Fatalf("qualified exact tuple lost %s: %+v %v", fact, selection, err)
				}
			}
		})
	}
}

var observerFacts = [4]host.Capability{host.ObserverCompletion, host.ObserverQuestion, host.ObserverPermission, host.ObserverTerminalError}

// Independent exact cells from TASK-INPUTS/contract.md and the frozen custody
// input (SHA256 28c63700240d3fbdbbae33dbc0b2ca734e52c3f5965263e2500b74bbad491d5c).
// Archive custody proves these image bytes only, never observer qualification.
var pendingNativeHosts = []struct {
	version, goos, goarch, image string
	adapter                      host.AdapterID
}{
	{"1.18.33", "linux", "arm64", "986fef2069a03b5181a9ec920786836f98fe3e4950c630941908687854e42757", host.ObserverV1},
	{"2.0.21", "linux", "arm64", "d2f4c9ee106d9930d20ca5cf5f2c2216aab6fed836992cf24979d9481242c01c", host.ObserverV2},
	{"1.18.33", "darwin", "amd64", "f53aae8eb68d832ab1bcd27bed88c02de910be61f4b5f90068ae8e93d5e794c9", host.ObserverV1},
	{"2.0.21", "darwin", "amd64", "4642b7da61279c8aa5d389d9f29454936e449fea6bc510689e9cc976fff6579f", host.ObserverV2},
	{"1.18.33", "darwin", "arm64", "139ddeb6a46ba276827bb8f79c7b28208621746e4fd6914d9ae71cc1a0a57524", host.ObserverV1},
	{"2.0.21", "darwin", "arm64", "0b2b68c1efaf20a29aaf636c2ffccc1abb56243a82f48cce45e257d232e03442", host.ObserverV2},
	{"1.18.33", "windows", "amd64", "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c", host.ObserverV1},
	{"2.0.21", "windows", "amd64", "ec7a3909bad41ef88e4650f737ab6f0b0c402a7f49a588812d0a79820c2dfc1f", host.ObserverV2},
}

func assertObserverSelectionsDenied(t *testing.T, p host.Profile, adapter host.AdapterID) {
	t.Helper()
	for _, facts := range [][]host.Capability{
		{host.ObserverCompletion}, {host.ObserverQuestion}, {host.ObserverPermission}, {host.ObserverTerminalError}, observerFacts[:],
	} {
		selection, err := host.Select(p, observerRequirements(adapter, facts...))
		if !errors.Is(err, host.ErrUnverifiedCapability) || selection.ArtifactID != "" || selection.Adapter != "" || len(selection.Unverified) != len(facts) {
			t.Fatalf("pending/forged observer authority for %v: %+v %v", facts, selection, err)
		}
	}
}

// RED at baseline is API/candidate discovery: the exact-image lookup does not
// exist. After discovery, RED means premature binding or selection authority,
// including any single capability after copied evidence or forged public flags.
func TestKnownNativeCandidatesStayPendingAndCannotBind(t *testing.T) {
	for _, tc := range pendingNativeHosts {
		t.Run(tc.version+"/"+tc.goos+"/"+tc.goarch, func(t *testing.T) {
			d, ok := host.NativeObserverEvidenceForImage(tc.version, tc.goos, tc.goarch, tc.image)
			if !ok || d.NativeStatus != "native_source_pending" || d.Tuple.Version != tc.version || d.Tuple.GOOS != tc.goos || d.Tuple.GOARCH != tc.goarch || d.Tuple.ImageSHA256 != tc.image || d.Tuple.Adapter != tc.adapter || d.Capabilities != observerFacts {
				t.Fatalf("missing exact pending cell: %+v", d)
			}
			if d.Tuple.EvidenceID != "" || d.Tuple.EvidenceHashes != ([4]string{}) {
				t.Fatalf("candidate acquired qualification evidence: %+v", d.Tuple)
			}
			original := d
			linux, _ := host.NativeObserverEvidence(tc.version)
			copiedEvidence := d.Tuple
			copiedEvidence.EvidenceID = linux.Tuple.EvidenceID
			copiedEvidence.EvidenceHashes = linux.Tuple.EvidenceHashes
			copiedLinux := linux.Tuple
			copiedLinux.GOOS, copiedLinux.GOARCH, copiedLinux.ImageSHA256 = tc.goos, tc.goarch, tc.image
			d.NativeStatus = "native_source_qualified"
			d.Tuple.EvidenceID = linux.Tuple.EvidenceID
			d.Tuple.EvidenceHashes = linux.Tuple.EvidenceHashes
			d.Capabilities[0] = host.MCPStdio
			for _, actual := range []host.NativeObserverTuple{original.Tuple, copiedEvidence, copiedLinux, d.Tuple} {
				p := host.BindNativeObserver(runtimeEvidence(tc.version), actual)
				if !reflect.DeepEqual(p, host.Resolve(runtimeEvidence(tc.version))) {
					t.Fatal("pending tuple changed generic resolution or acquired private authority")
				}
				assertNoObserver(t, p, tc.adapter, observerFacts[:])
				assertObserverSelectionsDenied(t, p, tc.adapter)
				for _, fact := range observerFacts {
					p.Capabilities[fact] = host.Supported
				}
				p.EvidenceID = linux.Tuple.EvidenceID
				assertObserverSelectionsDenied(t, p, tc.adapter)
				encoded, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				var serialized host.Profile
				if err := json.Unmarshal(encoded, &serialized); err != nil {
					t.Fatal(err)
				}
				assertObserverSelectionsDenied(t, serialized, tc.adapter)
			}
			fresh, ok := host.NativeObserverEvidenceForImage(tc.version, tc.goos, tc.goarch, tc.image)
			if !ok || fresh != original {
				t.Fatal("candidate snapshot mutation changed closed lookup")
			}
			if tc.version == "1.18.33" {
				if current, ok := host.NativeObserverEvidenceForImage("1.18.34", tc.goos, tc.goarch, tc.image); ok || current != (host.NativeObserverDescriptor{}) {
					t.Fatal("pending floor image inherited unsupported current cell")
				}
			}
		})
	}
}

// RED means an absent/unknown input, registry architecture alias, platform swap,
// cross-cell image, or version variant is silently inferred or inherits authority.
func TestExactImageLookupDeniesUnknownAndMixedCells(t *testing.T) {
	var tuples []host.NativeObserverTuple
	for _, tc := range nativeHosts {
		d, ok := host.NativeObserverEvidenceForImage(tc.version, "linux", "amd64", tc.image)
		if !ok {
			t.Fatal("missing qualified test cell")
		}
		tuples = append(tuples, d.Tuple)
	}
	for _, tc := range pendingNativeHosts {
		d, ok := host.NativeObserverEvidenceForImage(tc.version, tc.goos, tc.goarch, tc.image)
		if !ok {
			t.Fatal("missing pending test cell")
		}
		tuples = append(tuples, d.Tuple)
	}
	for _, tuple := range tuples {
		t.Run(tuple.Version+"/"+tuple.GOOS+"/"+tuple.GOARCH, func(t *testing.T) {
			denied := []host.NativeObserverTuple{{}}
			for _, version := range []string{"", "1.18.35", "1.19.0", "2.0.22", "3.0.0", tuple.Version + "-rc.1", tuple.Version + "+build"} {
				actual := tuple
				actual.Version = version
				denied = append(denied, actual)
			}
			for _, goos := range []string{"", "freebsd", "Linux", "linux", "darwin", "windows"} {
				if goos != tuple.GOOS {
					actual := tuple
					actual.GOOS = goos
					denied = append(denied, actual)
				}
			}
			for _, goarch := range []string{"", "x64", "386", "AMD64", "amd64", "arm64"} {
				if goarch != tuple.GOARCH {
					actual := tuple
					actual.GOARCH = goarch
					denied = append(denied, actual)
				}
			}
			images := []string{"", "unproved-image", strings.ToUpper(tuple.ImageSHA256)}
			for _, foreign := range tuples {
				if foreign.ImageSHA256 != tuple.ImageSHA256 {
					images = append(images, foreign.ImageSHA256)
				}
			}
			for _, image := range images {
				actual := tuple
				actual.ImageSHA256 = image
				denied = append(denied, actual)
			}
			for _, actual := range denied {
				if d, ok := host.NativeObserverEvidenceForImage(actual.Version, actual.GOOS, actual.GOARCH, actual.ImageSHA256); ok || d != (host.NativeObserverDescriptor{}) {
					t.Fatalf("mixed/unknown cell recognized: %+v", actual)
				}
				p := host.BindNativeObserver(runtimeEvidence(tuple.Version), actual)
				if !reflect.DeepEqual(p, host.Resolve(runtimeEvidence(tuple.Version))) {
					t.Fatalf("rejected exact cell altered generic profile: %+v", actual)
				}
				assertObserverSelectionsDenied(t, p, tuple.Adapter)
			}
		})
	}
}
