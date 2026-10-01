package opencodehost

// NativeObserverTuple describes fixed source eligibility, never a live image
// attestation. Only trusted runtime composition may supply it to the binder.
type NativeObserverTuple struct {
	Version, ImageSHA256, GOOS, GOARCH, Entry string
	ReaderContract, ProvenanceBasis           string
	Adapter                                   AdapterID
	UpstreamCommit, NativeAdapterSHA256       string
	EvidenceID                                string
	// Amendment, live-image proof, official artifact manifest, current discriminators.
	EvidenceHashes [4]string
}

// NativeObserverDescriptor is a detached value snapshot. NativeStatus describes
// host source evidence; it does not qualify an installed product or its clock.
type NativeObserverDescriptor struct {
	Tuple        NativeObserverTuple
	NativeStatus string
	Capabilities [4]Capability
}

// NativeObserverEvidence returns only the three exact Linux amd64 native serve
// descriptors. Lookup alone grants nothing; there is no version inheritance.
func NativeObserverEvidence(version string) (NativeObserverDescriptor, bool) {
	d := NativeObserverDescriptor{
		NativeStatus: "native_source_qualified",
		Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError},
		Tuple: NativeObserverTuple{
			Version: version, GOOS: "linux", GOARCH: "amd64",
			Entry:               "official_native_serve_default_dual_autoload",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceHashes: [4]string{
				"219b716d5ef855bd35e4c3b18132d49535a1e43e8fac0952bb78b963d4c63e76",
				"5ad9f81ce36cf2e1b2b424a74ca53d991fe75c8af3f71a4654b6a563a74d89cb",
				"ab4435c0c8c1e9dbfbc731a490e9a01042428cdd9e84e426f13e50cff333986a",
				"e9baee317279d5fc8c421becc6d5cbf7096f9fc0146f3b130b07b5ed7af057ed",
			},
		},
	}
	t := &d.Tuple
	t.Adapter = ObserverV1
	t.ReaderContract = "v1_source_causal_sync_callback_tombstones"
	t.ProvenanceBasis = "v1_assistant_completed_or_assistant_created_lower_bound"
	switch version {
	case "1.18.33":
		t.ImageSHA256 = "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427"
		t.UpstreamCommit = "51ef4be1d3c122f18fefb510dca8d778571f4f18"
	case "1.18.34":
		t.ImageSHA256 = "9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2"
		t.UpstreamCommit = "aec0b9a6d8898f68f923aaf08b7306d931fd9d76"
	case "2.0.21":
		t.ImageSHA256 = "f916986543348d7953d8d43aa048516cdbc3f84f4d0dc9c0c5b9d1da3030cea7"
		t.UpstreamCommit = "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72"
		t.Adapter = ObserverV2
		t.ReaderContract = "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare"
		t.ProvenanceBasis = "v2_native_envelope_created"
	default:
		return NativeObserverDescriptor{}, false
	}
	// Includes the amendment and all its supplemental evidence, without I/O.
	t.EvidenceID = "native-observer:" + version + ":sha256:" + t.NativeAdapterSHA256
	for _, hash := range t.EvidenceHashes {
		t.EvidenceID += ":sha256:" + hash
	}
	return d, true
}

// BindNativeObserver is a TRUSTED-CALLER boundary, not an authentication API.
// The AN Go helper must derive e from OS-verified live identity plus the shared
// target probe and independently validate the actual reader/adapter. Never call
// with config, event/IPC assertions, PATH or an install-time cached target. This
// pure comparison cannot verify that the claimed image is executing. Retain the
// result only within that verified runtime/reader lifetime; changes revoke it.
func BindNativeObserver(e VersionEvidence, actual NativeObserverTuple) Profile {
	p := Resolve(e)
	d, ok := NativeObserverEvidence(e.Version)
	if !ok || e.Source != "host_runtime" || e.ProbeStatus != "ok" || e.ExecutableIdentity == "" || actual != d.Tuple {
		return p
	}
	p.nativeObserver = d.Tuple
	p.nativeIdentity = e.ExecutableIdentity
	for _, c := range d.Capabilities {
		p.Capabilities[c] = Supported
	}
	return p
}

func (p Profile) observerSupport(adapter AdapterID, c Capability) Support {
	d, ok := NativeObserverEvidence(p.Version)
	if ok && p.nativeIdentity != "" && p.nativeObserver == d.Tuple && adapter == d.Tuple.Adapter {
		if c == LocalPluginDual {
			return Supported
		}
		for _, fact := range d.Capabilities {
			if c == fact {
				return Supported
			}
		}
	}
	if c == LocalPluginDual {
		return p.Capabilities[c] // preserve placement diagnostics, never observer authority
	}
	return Unverified
}
