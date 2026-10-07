package vscodelocal_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Authentic fixtures captured BEFORE consumer editing from compiling frozen
// 18c26a0 Engine/NewLocal install (and owned-empty refresh), then fresh Store.Load.
// The compressed snapshots contain original nil state/package/selector bytes.
// Relocation changes only TEST paths and their dependent package/object digests;
// it never clears an observation or calls candidate install/maintenance.
func historicalLocal(t *testing.T, empty bool) (*localFixture, string) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("captured historical fixtures use the Linux source tuple; native qualification remains separate")
	}
	encoded := historicalSelected
	if empty {
		encoded = historicalEmpty
	}
	compressed, err := base64.StdEncoding.DecodeString(encoded)
	must(t, err)
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	must(t, err)
	raw, err := io.ReadAll(reader)
	must(t, err)
	must(t, reader.Close())
	var captured struct {
		Root   string
		Config vscode.LocalConfig
		Files  []struct {
			Path string
			Body []byte
			Mode os.FileMode
		}
	}
	must(t, json.Unmarshal(raw, &captured))
	root := t.TempDir()
	configRaw, err := json.Marshal(captured.Config)
	must(t, err)
	must(t, json.Unmarshal(bytes.ReplaceAll(configRaw, []byte(captured.Root), []byte(root)), &captured.Config))
	f := &localFixture{root: root, pkg: filepath.Join(root, "package"), settings: filepath.Join(root, "selected-profile", "settings.json"), state: filepath.Join(root, "state"), runtime: filepath.Join(root, "TEST runtime ' Ω $(touch sentinel)"), config: captured.Config}
	for _, file := range captured.Files {
		if filepath.Base(file.Path) == ".agentplugins-data-owner.json" {
			continue
		} // Relocated data gets an actual public-manager owner record below.
		writeLocal(t, filepath.Join(root, file.Path), bytes.ReplaceAll(file.Body, []byte(captured.Root), []byte(root)), file.Mode)
	}
	executable, err := os.Executable()
	must(t, err)
	writeLocal(t, f.runtime, readLocal(t, executable), 0700)
	path := filepath.Join(f.state, "state-v2.json")
	stateRaw := readLocal(t, path)
	var state domain.StateFileV2
	must(t, json.Unmarshal(stateRaw, &state))
	prior := onlyLocalBinding(t, state)
	if prior.LocalEntryObservation != nil {
		t.Fatal("captured historical fixture was not nil from inception")
	}
	facts, _ := prior.SelectedDelivery.LocalFacts()
	canonical := historicalDigest(t, f.pkg, f.root)
	projection := historicalDigest(t, facts.Registration.Selector, f.root)
	objectID := "vscode-local:" + testDigest([]byte(f.settings+"\x00"+facts.Registration.Selector))
	replacements := strings.NewReplacer(facts.CanonicalDigest, canonical, facts.ProjectionDigest, projection, facts.Registration.ObjectID, objectID,
		state.Installations[0].Source.SourceBindingID, domain.ComputeSourceBindingID(domain.SourceIdentity{CanonicalSource: state.Installations[0].Source.CanonicalSource, Repository: state.Installations[0].Source.Repository, PackageSubpath: state.Installations[0].Source.PackageSubpath}),
		prior.ClientBindingID, domain.ComputeClientBindingID(state.Installations[0].InstallationID, prior.ClientID, prior.Scope, prior.TargetLocator))
	stateRaw = []byte(replacements.Replace(string(stateRaw)))
	state = domain.StateFileV2{} // Decode the relocated snapshot into a fresh value.
	must(t, json.Unmarshal(stateRaw, &state))
	receipt, _, dataErr := (providers.PluginDataManager{Base: filepath.Join(f.state, "plugin-data")}).EnsureData(t.Context(), state.Installations[0].InstallationID, prior.PhysicalArtifact, prior.Scope)
	must(t, dataErr)
	state.Installations[0].DataReceipts = map[string]domain.DataReceipt{receipt.DataReceiptID: receipt}
	for key, binding := range state.Installations[0].Clients {
		binding.DataReceiptID = receipt.DataReceiptID
		state.Installations[0].Clients[key] = binding
	}
	must(t, (statev2.Store{Path: path}).Save(state))
	f.config.DeclaredHookDigest = testDigest(readLocal(t, filepath.Join(f.pkg, hookPath())))
	f.adapter, err = vscode.NewLocal(f.config)
	must(t, err)
	f.registry, err = clients.NewRegistry(f.adapter)
	must(t, err)
	reloaded, err := (statev2.Store{Path: path}).Load()
	must(t, err)
	binding := onlyLocalBinding(t, reloaded)
	if binding.LocalEntryObservation != nil || !binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		t.Fatal("relocation changed historical authority")
	}
	t.Logf("authentic old nil relocated: captured_payload=%s installed_id=%s", testDigest(raw), state.Installations[0].InstallationID)
	return f, state.Installations[0].InstallationID
}

func historicalDigest(t *testing.T, path, tmp string) string {
	t.Helper()
	snapshot, err := (packagedigest.Builder{TempRoot: tmp}).Snapshot(t.Context(), path, domain.SourceIdentity{})
	must(t, err)
	defer func() { must(t, packagedigest.Remove(snapshot)) }()
	return snapshot.TreeDigest
}

// Regression: historical nil silently acquires absence authority, or a candidate
// observation is erased to make the old guard pass. Boundary: authentic captured
// old bytes, fresh real Store/Engine, readonly Inspect, explicit repair refusal.
func assertHistoricalAbsentRefusal(t *testing.T, empty bool) {
	t.Helper()
	f, id := historicalLocal(t, empty)
	engine := f.engine(t, true)
	b := onlyLocalBinding(t, loadLocalState(t, f.state))
	facts, _ := b.SelectedDelivery.LocalFacts()
	disabled := []byte(`{"chat.pluginLocations":{` + quoteLocal(facts.Registration.Selector) + `:false}}`)
	writeLocal(t, f.settings, disabled, 0600)
	state, inspectErr := f.adapter.InspectRegistration(t.Context(), nativeconfig.New(), b.SelectedDelivery, b.NativeObjects)
	must(t, inspectErr)
	if state != vscode.RegistrationDisabled {
		t.Fatal("captured historical selector did not own actual false bytes")
	}
	before := snapshotLocalFiles(t, f.state, f.settings)
	_, err := engine.Inspect(t.Context())
	must(t, err)
	assertLocalFiles(t, before)
	if onlyLocalBinding(t, loadLocalState(t, f.state)).LocalEntryObservation != nil {
		t.Fatal("Inspect minted historical observation")
	}
	absent := []byte(`{"chat.pluginLocations":{},"TEST-late-foreign":true}`)
	writeLocal(t, f.settings, absent, 0600)
	before = snapshotLocalFiles(t, filepath.Join(f.state, "state-v2.json"), f.settings)
	h, err := engine.Prepare(t.Context(), f.request(installer.OpRepair, id))
	if h != nil {
		defer func() { must(t, h.Close()) }()
	}
	if err == nil {
		_, err = engine.Apply(t.Context(), h, installer.Decision{Confirmed: true})
	}
	if err == nil {
		t.Fatal("historical nil restored disappeared false entry")
	}
	if !bytes.Equal(absent, readLocal(t, f.settings)) {
		t.Fatal("historical refusal changed native bytes")
	}
	assertLocalFiles(t, before)
	b = onlyLocalBinding(t, loadLocalState(t, f.state))
	if b.LocalEntryObservation != nil || !b.SelectedDelivery.OwnsProfileEntry(b.NativeObjects) {
		t.Fatal("historical refusal lost prior ownership or minted authority")
	}
}

const historicalSelected = "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" +
	"bZTWax0wOhwnbB2DID4Iw8py7cLuAdtU816JZhzLQVYkz61oWlBCE0WVwJFgLyLtqTqH3eO5qSOHFY29FTXqVFV4i1H4YiRYqpzQ" +
	"QkKO1OKhzvDSmw/fyBFvIO+GlQj+W/sy/MxMofnk9svY2ZQy9g7ttxc4W54pcrsi+FjVsmM+ayXdtUumZ2seO7dlCXCeExh2Mds/" +
	"OTorH0tdzBmvNYXROaaHjL+5SYUVBvFy5JLdc45IROTiZmgW05rVHglUHfFKAp6YKC/WKMsnjbUqaQxuwr1su5yPeSyG4MvIVYKi" +
	"a4HVdgtajCMTXEXRp4i9xtscdmH9k3yMO3WflGJWA7eNadGNEaPgQC72AO8pWL7tvPZ7pp+d3Wd4gbzRFUOG0/D1+Sk3jmNXGD1j" +
	"oqVK4B9mzdSne8ZDLqzHyvRjh52HbfMLY57Lx/d8+XuW5a9isPRpb4MchtiPd9fwYMh8r9cBrs/2qInqVluew/84C7lg2APeFzF8" +
	"Z8RRAi6kWhF4NEEuADfrCIpq1qxoLtD2lAfv3sGnC4q8YLXeynLg+yWWl3Lpo/BtrFP49jkXboSMf9GQg/65ncKPIMMCeQWfVg11" +
	"Ac5d+iS1L/gZ7/bYnow/dqtc9+t5+a/BFmB4zOxYsLX93Qt+eHz2NAR/R34VUIsQYCtqT3DeiHHeaW4lNHw7t+SrMbAuqmQf4+eY" +
	"wI71i2OZnU/Y9rotn3lR562eqxzLnl/CmYMM7KbKWkKOp8qP33nZRV52+jlDPtvi7GvrbSw+2xW8ApzGKwyc1evNSh8Hl8i9sNIB" +
	"i9cBWQKTGGZd05mIuA6ABVcxktm9wlShuo1gNnJCvpqHM+Fs8QlLYENgL+OQL/XE9BZnbm9X4VteR91DUevKkQ2cAj6e42Wpd2BN" +
	"wbC9zCmwBbvtHPAsj4zbJnIWOH48MjQO/Aj8II5Usp+r0NnAOPk89HH62Tj64ZTpmsOcjSUoz/2xb/murY9LrPX683dd/3pdX8KM" +
	"Mr9bxphdZ6QuD55drcv57zXbd2CDeNJhfrE+OOVMQwuc0tdYbYc9e/v4Sg44+P3hmkQTSZn3j3Ug6w2jdkPNcMWuZe5F/RKvYaOX" +
	"/aBTXwU500uG7QZBzq8j7+YUnBI5s1CIkluqsqNqhzcjCfrXIrPoJlavI9DWlRhlNQ2rgRDTBhGLiscerh0v66Ns/7NrM+jvyWW9" +
	"q96u4hPABM+YZQMiLx3wSfsre550eojf+LlerF4PMI59CA71kFlT2lYyRE2kRMjhagN8QynMQgYnavCKitoXXAg2jhTdmpuqy53q" +
	"3bPPC95wrvM6k4HAXsXxStbry7w1uewPL/olV/icCb6kRDSEnKjjO8JQVwjqcgEch5iqScxizoGLFIrenbP6zSQSMSPUQ1GDDFVz" +
	"ZyXgcmqMah9cNLHmmCcoX8XZJeOtZd/vSl8Ba6BmVFF3Fl1WQxGrJ+2ttrlXeqMCnBM1FdPxjKc9ZW8xXlsoNewdDPVugpq0bhKF" +
	"V3pdjOmweTjHpVrmYN8EnBkyHGu5r8YL8MlOgH2nnsFr/nR/6inPUSlPuWFPDqk+ZnrZg7+hZvNCSrTYTMCRCy8Eb97RnhzTqIEa" +
	"CbFRzAQlMes0kXiTWImpmxxF7CiJxCnq/Ho8PnPA9zrpj7smvo4TyG3F4f6LmPqer/pU7BWW6QW/O6sRWq+uIq9jAOJ5v2S2Mxhu" +
	"JSUvfO6dPz8rUJexPkvRDGg0rdNiHAN7OVMFBiVWZL6NzbN1j3X8US8X6jLCcP/QWz/cXb0ec8Lp8zutdy51gUtx7B5oQFgfjp1x" +
	"tPXY3SBquGeZhe/uqZz3aAI3ZTHcPces7+8LsBrbALfR90v/DCdZ/5C9jWnr3bVtLONLdTxswtmGVdmr4gc72GRnsli5NP5MZxXu" +
	"1KqcDd8wyr4r6zld6AWkzS3ydFRxS5ndQ8Evyvu21/phXOcM16r1n/vLWhm/y+qO7eI+7L6X4QfqXI0gdxzWuFQXy2951dt+8p/X" +
	"a78kx5/XE/4m/v0B/cwAvKHKD13kB0NOKgzBebvrT1f58uSVrN23sjosTgwrRt3+3hf5/foiJ72SAxfrn98zn/vP+iLfw7m/sH1O" +
	"ue4Yz9P7I/YOv7uPXa1tk3hzwNhnjmEIJ7xAbDPfZbq6ugb2TA/YvTHZq4Jv++rXam1g6njxzv3+wdwvOebu8T9HPYPY8Up+Udaq" +
	"7N6wKPHszCff1pwv7lovv+L7T18DySv2rsYhpk+Ye7hjPXLRMhd/T+y9x9I/LpbS9x73H9bj/jZOIUbl2FRHe3DHiEZSYUVepLTj" +
	"RNHlRInGc+gFnDNA3HZDhSg7k4znQ7UZWG0W53JE2zSgbS2k7VGNRh5sLYe/AafK9/UYHzv43NYNpTdrPI1eYoDxpgcmv7xXv8Dn" +
	"Lvd5ZNS3Wv763TRHaGRKD2dUgzm4M0/b05zpwcyv6VvcsdecD+dLEWu8e8LMrdvidmai8MMew8Ap8E1GrSRHV/VFtL2na5vqfbXG" +
	"/goGZuBZxz3PXn0eL92k6g35rHeIWL+2zx/UYzjopBsd+pfvXOVPwVf23qVX2hR64ez3WuQ3//naIKn88/R+Gc/sxt4dSgeqAuxk" +
	"74eCuRVT3ixoiDi5xlvK92g8o/luk9/fJgfdArsLmrB3pYBOBVW1SCGo6a5y+m7ile8TXu5/WO1GbkXTGvLAnmE4e1dZ0UdEUZtz" +
	"pWgIwHR251Wj6jhSdC1WIrcY9sBj1ThATg2HOkUeQd6M5ns2jyadvdK6yokWpkHP/4Rl6fUZTvymnPid77ggrxdmjufk177jAlt+" +
	"zzs0NeS+Arl0r7/IGb+DnGU/vem9+JMe9s7733/5X3DnPRg7RgAA"

const historicalEmpty = "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"
