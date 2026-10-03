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
	d, ok := qualifiedLinuxObserverEvidence(version)
	if !ok {
		return NativeObserverDescriptor{}, false
	}
	return NativeObserverEvidenceForImage(version, "linux", "amd64", d.Tuple.ImageSHA256)
}

// NativeObserverEvidenceForImage recognizes only literal exact image cells.
// A native_source_pending descriptor records byte identity, not qualification;
// neither lookup nor its informational capability names grant runtime authority.
// No platform, image or version is inferred when an input is missing.
func NativeObserverEvidenceForImage(version, goos, goarch, imageSHA256 string) (NativeObserverDescriptor, bool) {
	for _, d := range nativeObserverQualifications() {
		if observerImageMatches(d.Tuple, version, goos, goarch, imageSHA256) {
			return d, true
		}
	}
	for _, d := range nativeObserverCandidates() {
		if observerImageMatches(d.Tuple, version, goos, goarch, imageSHA256) {
			return d, true
		}
	}
	return NativeObserverDescriptor{}, false
}

func observerImageMatches(t NativeObserverTuple, version, goos, goarch, imageSHA256 string) bool {
	return t.Version == version && t.GOOS == goos && t.GOARCH == goarch && t.ImageSHA256 == imageSHA256
}

// This closed source table is the qualification merge seam: future independently
// reviewed per-image descriptors belong here as literal data. There is no public
// setter, environment override or inheritance from a candidate's source mapping.
// Qualified rows take precedence over candidates for the same exact image cell.
func nativeObserverQualifications() []NativeObserverDescriptor {
	floor, _ := qualifiedLinuxObserverEvidence("1.18.33")
	current, _ := qualifiedLinuxObserverEvidence("1.18.34")
	v2, _ := qualifiedLinuxObserverEvidence("2.0.21")
	return []NativeObserverDescriptor{
		floor, current, v2,
		{Tuple: NativeObserverTuple{
			Version:             "1.18.33",
			ImageSHA256:         "986fef2069a03b5181a9ec920786836f98fe3e4950c630941908687854e42757",
			GOOS:                "linux",
			GOARCH:              "arm64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v1_source_causal_sync_callback_tombstones",
			ProvenanceBasis:     "v1_assistant_completed_or_assistant_created_lower_bound",
			UpstreamCommit:      "51ef4be1d3c122f18fefb510dca8d778571f4f18",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:1.18.33:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:c0324e2ef2f6cbbd01928365b08fc3630077cac28fb3a0116947058c4c9567a2:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV1,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "c0324e2ef2f6cbbd01928365b08fc3630077cac28fb3a0116947058c4c9567a2", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "2.0.21",
			ImageSHA256:         "d2f4c9ee106d9930d20ca5cf5f2c2216aab6fed836992cf24979d9481242c01c",
			GOOS:                "linux",
			GOARCH:              "arm64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare",
			ProvenanceBasis:     "v2_native_envelope_created",
			UpstreamCommit:      "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:2.0.21:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:0bd94e3d931627d350359f1d5e7b29450237b98d2c2e2768f68ab6d944754e04:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV2,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "0bd94e3d931627d350359f1d5e7b29450237b98d2c2e2768f68ab6d944754e04", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "1.18.33",
			ImageSHA256:         "f53aae8eb68d832ab1bcd27bed88c02de910be61f4b5f90068ae8e93d5e794c9",
			GOOS:                "darwin",
			GOARCH:              "amd64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v1_source_causal_sync_callback_tombstones",
			ProvenanceBasis:     "v1_assistant_completed_or_assistant_created_lower_bound",
			UpstreamCommit:      "51ef4be1d3c122f18fefb510dca8d778571f4f18",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:1.18.33:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:2a7c72fb56ad69c08b7dd6eab5351aa0b20277684ef6fee0d59a43b47c353ada:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV1,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "2a7c72fb56ad69c08b7dd6eab5351aa0b20277684ef6fee0d59a43b47c353ada", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "2.0.21",
			ImageSHA256:         "4642b7da61279c8aa5d389d9f29454936e449fea6bc510689e9cc976fff6579f",
			GOOS:                "darwin",
			GOARCH:              "amd64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare",
			ProvenanceBasis:     "v2_native_envelope_created",
			UpstreamCommit:      "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:2.0.21:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:4b15b7bad6b64de07f5b91c9da52d8fc032cacec829c54cafaadd1086c29116f:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV2,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "4b15b7bad6b64de07f5b91c9da52d8fc032cacec829c54cafaadd1086c29116f", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "1.18.33",
			ImageSHA256:         "139ddeb6a46ba276827bb8f79c7b28208621746e4fd6914d9ae71cc1a0a57524",
			GOOS:                "darwin",
			GOARCH:              "arm64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v1_source_causal_sync_callback_tombstones",
			ProvenanceBasis:     "v1_assistant_completed_or_assistant_created_lower_bound",
			UpstreamCommit:      "51ef4be1d3c122f18fefb510dca8d778571f4f18",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:1.18.33:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:bbf07afffe9cb83a2de78c5be9d4e95b24581d88dee83a8f7301cdb687303ad2:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV1,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "bbf07afffe9cb83a2de78c5be9d4e95b24581d88dee83a8f7301cdb687303ad2", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "2.0.21",
			ImageSHA256:         "0b2b68c1efaf20a29aaf636c2ffccc1abb56243a82f48cce45e257d232e03442",
			GOOS:                "darwin",
			GOARCH:              "arm64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare",
			ProvenanceBasis:     "v2_native_envelope_created",
			UpstreamCommit:      "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:2.0.21:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:e1cf86b4ded224cd1507a376bc79c056f3187f365830d1cc7e8a1721c2ce22d6:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV2,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "e1cf86b4ded224cd1507a376bc79c056f3187f365830d1cc7e8a1721c2ce22d6", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "1.18.33",
			ImageSHA256:         "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c",
			GOOS:                "windows",
			GOARCH:              "amd64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v1_source_causal_sync_callback_tombstones",
			ProvenanceBasis:     "v1_assistant_completed_or_assistant_created_lower_bound",
			UpstreamCommit:      "51ef4be1d3c122f18fefb510dca8d778571f4f18",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:1.18.33:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:cd7409a4a5b5d09215f5ea0e92c71948ac769465b30aebb722154de9aea6f1f5:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV1,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "cd7409a4a5b5d09215f5ea0e92c71948ac769465b30aebb722154de9aea6f1f5", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
		{Tuple: NativeObserverTuple{
			Version:             "2.0.21",
			ImageSHA256:         "ec7a3909bad41ef88e4650f737ab6f0b0c402a7f49a588812d0a79820c2dfc1f",
			GOOS:                "windows",
			GOARCH:              "amd64",
			Entry:               "official_native_serve_default_dual_autoload",
			ReaderContract:      "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare",
			ProvenanceBasis:     "v2_native_envelope_created",
			UpstreamCommit:      "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72",
			NativeAdapterSHA256: "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901",
			EvidenceID:          "native-observer:2.0.21:sha256:d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901:sha256:6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea:sha256:0e752ddd0428d50ed24cb5f07a4025505a356ad922559c870f19a2ae6402ddce:sha256:59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f:sha256:b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9",
			Adapter:             ObserverV2,
			EvidenceHashes:      [4]string{"6568a59ddc5bc120162f374b62eb9f849d1d0d989183b4d72b94630071f573ea", "0e752ddd0428d50ed24cb5f07a4025505a356ad922559c870f19a2ae6402ddce", "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "b7664499f49b738f16905c41dec31128ecd8529045274848f57e62a90a062cf9"},
		}, NativeStatus: "native_source_qualified", Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError}},
	}
}

// Custody input SHA256: 28c63700240d3fbdbbae33dbc0b2ca734e52c3f5965263e2500b74bbad491d5c.
// These custody-only candidates remain separate from reviewed qualifications.
// Candidate mappings cannot substitute for the exact qualified evidence tuple.
func nativeObserverCandidates() [8]NativeObserverDescriptor {
	images := [...]NativeObserverTuple{
		{Version: "1.18.33", GOOS: "linux", GOARCH: "arm64", ImageSHA256: "986fef2069a03b5181a9ec920786836f98fe3e4950c630941908687854e42757"},
		{Version: "2.0.21", GOOS: "linux", GOARCH: "arm64", ImageSHA256: "d2f4c9ee106d9930d20ca5cf5f2c2216aab6fed836992cf24979d9481242c01c"},
		{Version: "1.18.33", GOOS: "darwin", GOARCH: "amd64", ImageSHA256: "f53aae8eb68d832ab1bcd27bed88c02de910be61f4b5f90068ae8e93d5e794c9"},
		{Version: "2.0.21", GOOS: "darwin", GOARCH: "amd64", ImageSHA256: "4642b7da61279c8aa5d389d9f29454936e449fea6bc510689e9cc976fff6579f"},
		{Version: "1.18.33", GOOS: "darwin", GOARCH: "arm64", ImageSHA256: "139ddeb6a46ba276827bb8f79c7b28208621746e4fd6914d9ae71cc1a0a57524"},
		{Version: "2.0.21", GOOS: "darwin", GOARCH: "arm64", ImageSHA256: "0b2b68c1efaf20a29aaf636c2ffccc1abb56243a82f48cce45e257d232e03442"},
		{Version: "1.18.33", GOOS: "windows", GOARCH: "amd64", ImageSHA256: "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c"},
		{Version: "2.0.21", GOOS: "windows", GOARCH: "amd64", ImageSHA256: "ec7a3909bad41ef88e4650f737ab6f0b0c402a7f49a588812d0a79820c2dfc1f"},
	}
	var candidates [8]NativeObserverDescriptor
	for i, tuple := range images {
		tuple.Entry = "official_native_serve_default_dual_autoload"
		tuple.NativeAdapterSHA256 = "d2d75185eb6283d6f8a34d2019aa201ada9ec9ea9db578b4f2f28dd7d9d8d901"
		switch tuple.Version {
		case "1.18.33":
			tuple.Adapter = ObserverV1
			tuple.ReaderContract = "v1_source_causal_sync_callback_tombstones"
			tuple.ProvenanceBasis = "v1_assistant_completed_or_assistant_created_lower_bound"
			tuple.UpstreamCommit = "51ef4be1d3c122f18fefb510dca8d778571f4f18"
		case "2.0.21":
			tuple.Adapter = ObserverV2
			tuple.ReaderContract = "v2_direct_asynciterable_data_sparse_error_end_checkpoint_after_prepare"
			tuple.ProvenanceBasis = "v2_native_envelope_created"
			tuple.UpstreamCommit = "8a8bd622a3d7dc29ccf30ec17f84e363ed95ed72"
		}
		candidates[i] = NativeObserverDescriptor{
			Tuple:        tuple,
			NativeStatus: "native_source_pending",
			Capabilities: [4]Capability{ObserverCompletion, ObserverQuestion, ObserverPermission, ObserverTerminalError},
		}
	}
	return candidates
}

func qualifiedLinuxObserverEvidence(version string) (NativeObserverDescriptor, bool) {
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
	d, ok := NativeObserverEvidenceForImage(actual.Version, actual.GOOS, actual.GOARCH, actual.ImageSHA256)
	if !ok || d.NativeStatus != "native_source_qualified" || actual.Version != e.Version || e.Source != "host_runtime" || e.ProbeStatus != "ok" || e.ExecutableIdentity == "" || actual != d.Tuple {
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
	t := p.nativeObserver
	d, ok := NativeObserverEvidenceForImage(t.Version, t.GOOS, t.GOARCH, t.ImageSHA256)
	if ok && d.NativeStatus == "native_source_qualified" && t.Version == p.Version && p.nativeIdentity != "" && t == d.Tuple && adapter == d.Tuple.Adapter {
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
