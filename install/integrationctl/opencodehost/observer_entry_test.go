package opencodehost_test

import (
	"testing"

	host "github.com/777genius/plugin-kit-ai/install/integrationctl/opencodehost"
)

// Expected pins/results are the accepted amendment's independent native reader
// observations, not a traversal of the implementation's qualification table.
var localEntryObservations = []struct {
	version, goos, goarch, image, entry, result string
}{
	{"1.18.33", "linux", "amd64", "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427", "official_native_tui_default_dual_autoload", "f6f3fa9165082fcb9e48b687b878e3c7d9932b7bc87126e743d8b2cff85c7ce9"},
	{"1.18.33", "linux", "amd64", "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427", "official_native_run_local_dual_autoload", "942378e878a6dcc1fc00a484cb2b91d2e78b5b3b136c4f19c2ed90b2ee8b04e7"},
	{"1.18.33", "linux", "arm64", "986fef2069a03b5181a9ec920786836f98fe3e4950c630941908687854e42757", "official_native_tui_default_dual_autoload", "0ff9cd338937ddc37e3af930384b1ee5ea2ece10e210e8252ca459b4744715ba"},
	{"1.18.33", "linux", "arm64", "986fef2069a03b5181a9ec920786836f98fe3e4950c630941908687854e42757", "official_native_run_local_dual_autoload", "a721c36a8ad13e724a2b902eacbc5744d0b1631b810aed736c2d268d194559f6"},
	{"1.18.33", "darwin", "amd64", "f53aae8eb68d832ab1bcd27bed88c02de910be61f4b5f90068ae8e93d5e794c9", "official_native_tui_default_dual_autoload", "b5c8707263de154d6655be577c83bc14cd2fcea094e76fc918cd781d6f5a06df"},
	{"1.18.33", "darwin", "amd64", "f53aae8eb68d832ab1bcd27bed88c02de910be61f4b5f90068ae8e93d5e794c9", "official_native_run_local_dual_autoload", "7c4eed8eda68a1268303566ebe1aa1bf0f3fde8ec4e43d4883cbbecd62f08682"},
	{"1.18.33", "darwin", "arm64", "139ddeb6a46ba276827bb8f79c7b28208621746e4fd6914d9ae71cc1a0a57524", "official_native_tui_default_dual_autoload", "7422f61d53ba2ddff0ba13feb786b56177e8d430c4cb6206916f9fa90608a89d"},
	{"1.18.33", "darwin", "arm64", "139ddeb6a46ba276827bb8f79c7b28208621746e4fd6914d9ae71cc1a0a57524", "official_native_run_local_dual_autoload", "792eb54b386494917f2f1db04253067e0685dcbca61315b44bad2e08d543fd62"},
	{"1.18.33", "windows", "amd64", "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c", "official_native_run_local_dual_autoload", "e649f0dc8e5728b3dc3b26693f082c64f864e24966c3d3f54ffb7706df88e52f"},
	{"1.18.34", "linux", "amd64", "9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2", "official_native_tui_default_dual_autoload", "36e2e4916ded9ea9759e45547625a1eb9ed1b8849fa04f0f5713465413b4a74a"},
	{"1.18.34", "linux", "amd64", "9ca0b9953d49997601655e54f846a3efa464f237e47c6f1b04716d0f2e64c4c2", "official_native_run_local_dual_autoload", "6500fb712c5a01f73f30120b2338ac8e1fda7b509a6dca6552780d052f8389ba"},
	{"1.18.33", "windows", "amd64", "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c", "official_native_tui_default_dual_autoload", "0b9d7578a419e07becaa67a885e4a5971b220581d3ec2f805085e84ad8258af8"},
}

func localEntryEvidence(version string) host.VersionEvidence {
	return host.VersionEvidence{Version: version, Source: "host_runtime", ProbeStatus: "ok", ExecutableIdentity: "TEST-held-native-object"}
}

func assertLocalEntryRejected(t *testing.T, tuple host.NativeObserverTuple) {
	t.Helper()
	p := host.BindNativeObserver(localEntryEvidence(tuple.Version), tuple)
	facts := []host.Capability{host.ObserverCompletion, host.ObserverQuestion, host.ObserverPermission, host.ObserverTerminalError}
	for _, fact := range facts {
		if p.Capabilities[fact] == host.Supported {
			t.Fatalf("rejected entry tuple grants %s", fact)
		}
	}
	selection, err := host.Select(p, []host.ArtifactRequirement{{ID: "TEST-native-reader", Adapter: host.ObserverV1, Required: append([]host.Capability{host.LocalPluginDual}, facts...)}})
	if err == nil || selection.Adapter != "" || selection.ArtifactID != "" {
		t.Fatalf("rejected entry selected observer: %+v %v", selection, err)
	}
}

func TestLocalEntryLookupBindsOwnAcceptedObservation(t *testing.T) {
	for _, tc := range localEntryObservations {
		t.Run(tc.goos+"/"+tc.goarch+"/"+tc.version+"/"+tc.entry, func(t *testing.T) {
			d, ok := host.NativeObserverEvidenceForEntry(tc.version, tc.goos, tc.goarch, tc.image, tc.entry)
			if !ok || d.NativeStatus != "native_source_qualified" || d.Tuple.Version != tc.version || d.Tuple.GOOS != tc.goos || d.Tuple.GOARCH != tc.goarch || d.Tuple.ImageSHA256 != tc.image || d.Tuple.Entry != tc.entry {
				t.Fatalf("missing exact accepted entry: %+v", d)
			}
			expected := [4]string{"73f2c805e0a5ea8785621bcd9f90c14ef6f8685fdeba699b6f2c0cbc97ca49f4", tc.result, "59368f673e651f00e8a4d0a675365ba15f0752eacabc3e2ea68a2e91cf6b166f", "26d17b7bb6eb3b3a1e61d342784c4df569cbb1845fa473f5776553c64cd33308"}
			if d.Tuple.EvidenceHashes != expected {
				t.Fatalf("entry observation/provenance substituted: %+v", d.Tuple.EvidenceHashes)
			}
			serve, ok := host.NativeObserverEvidenceForImage(tc.version, tc.goos, tc.goarch, tc.image)
			if !ok || serve.Tuple.Entry != "official_native_serve_default_dual_autoload" || serve.Tuple.EvidenceID == d.Tuple.EvidenceID {
				t.Fatal("legacy image lookup changed or local entry borrowed serve identity")
			}
			p := host.BindNativeObserver(localEntryEvidence(tc.version), d.Tuple)
			facts := []host.Capability{host.ObserverCompletion, host.ObserverQuestion, host.ObserverPermission, host.ObserverTerminalError}
			for _, fact := range facts {
				if p.Capabilities[fact] != host.Supported {
					t.Fatalf("accepted source binding missing %s", fact)
				}
			}
			selection, err := host.Select(p, []host.ArtifactRequirement{{ID: "TEST-native-reader", Adapter: host.ObserverV1, Required: append([]host.Capability{host.LocalPluginDual}, facts...)}})
			if err != nil || selection.Adapter != host.ObserverV1 || selection.ArtifactID != "TEST-native-reader" {
				t.Fatalf("accepted local entry not selected: %+v %v", selection, err)
			}
		})
	}
}

func TestEntryLookupRequiresEveryIdentityField(t *testing.T) {
	type key struct{ version, goos, goarch, image, entry string }
	good := key{"1.18.33", "linux", "amd64", "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427", "official_native_run_local_dual_autoload"}
	mutations := map[string]func(*key){
		"version": func(k *key) { k.version = "1.18.35" },
		"os":      func(k *key) { k.goos = "freebsd" },
		"arch":    func(k *key) { k.goarch = "386" },
		"image":   func(k *key) { k.image = "" },
		"entry":   func(k *key) { k.entry = "official_native_run_remote_dual_autoload" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			k := good
			mutate(&k)
			if d, ok := host.NativeObserverEvidenceForEntry(k.version, k.goos, k.goarch, k.image, k.entry); ok || d != (host.NativeObserverDescriptor{}) {
				t.Fatalf("partial identity qualified: %+v", d)
			}
		})
	}
	for _, entry := range []string{"", "tui", "run", "native", "wrapper", "official_native_attach_dual_autoload"} {
		if _, ok := host.NativeObserverEvidenceForEntry(good.version, good.goos, good.goarch, good.image, entry); ok {
			t.Fatalf("unknown entry %q accepted", entry)
		}
	}
	if _, ok := host.NativeObserverEvidenceForEntry("1.18.33", "windows", "arm64", "52f60248a576b34c9a6dcaa27e0a7f08089af35bcdc0dfb10c04d3e00a98314c", "official_native_tui_default_dual_autoload"); ok {
		t.Fatal("unsupported Windows ARM64 inherited source qualification")
	}
	if _, ok := host.NativeObserverEvidenceForEntry("2.0.21", "linux", "amd64", "f916986543348d7953d8d43aa048516cdbc3f84f4d0dc9c0c5b9d1da3030cea7", good.entry); ok {
		t.Fatal("V2 inherited local V1 entry")
	}
}

func TestSameImageCrossEntryProofSubstitutionIsRejected(t *testing.T) {
	entries := []string{"official_native_serve_default_dual_autoload", "official_native_tui_default_dual_autoload", "official_native_run_local_dual_autoload"}
	descriptors := make([]host.NativeObserverDescriptor, len(entries))
	for i, entry := range entries {
		d, ok := host.NativeObserverEvidenceForEntry("1.18.33", "linux", "amd64", "0abbb7c32ab0294c0a7bfa2705f9ff0df5dce5ab721d1f00cccfe393f2a11427", entry)
		if !ok {
			t.Fatalf("missing entry %s", entry)
		}
		descriptors[i] = d
	}
	for i, origin := range descriptors {
		for j, target := range descriptors {
			if i == j {
				continue
			}
			if origin.Tuple.EvidenceID == target.Tuple.EvidenceID {
				t.Fatal("distinct native entry proofs share an identity")
			}
			borrowed := origin.Tuple
			borrowed.Entry = target.Tuple.Entry
			assertLocalEntryRejected(t, borrowed)
			borrowed = target.Tuple
			borrowed.EvidenceID = origin.Tuple.EvidenceID
			assertLocalEntryRejected(t, borrowed)
			borrowed = target.Tuple
			borrowed.EvidenceHashes = origin.Tuple.EvidenceHashes
			assertLocalEntryRejected(t, borrowed)
		}
	}
}
