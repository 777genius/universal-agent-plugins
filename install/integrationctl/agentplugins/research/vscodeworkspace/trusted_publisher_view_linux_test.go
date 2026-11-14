//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublisherRuntimeClosedFacts(t *testing.T) {
	p := isolationProof{PID: 101, UID: 1001, GID: 1001, Namespace: 200}
	executable := policyIdentity{Device: 300, Inode: 400}
	digest := strings.Repeat("a", 64)
	good := map[string]any{"pid": json.Number("101"), "mount_namespace": json.Number("500"), "network_namespace": json.Number("200"), "apparmor_label": "vscode (unconfined)", "executable_device": json.Number("300"), "executable_inode": json.Number("400"), "cgroup_sha256": digest, "uid": json.Number("1001"), "gid": json.Number("1001"), "capabilities_zero": true, "no_new_privileges": true}
	if err := validatePublisherRuntimeFacts(good, p, 500, executable, digest); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]any{"pid": json.Number("102"), "mount_namespace": json.Number("501"), "network_namespace": json.Number("201"), "apparmor_label": "unconfined", "executable_device": json.Number("301"), "executable_inode": json.Number("401"), "cgroup_sha256": strings.Repeat("b", 64), "uid": json.Number("0"), "gid": json.Number("0"), "capabilities_zero": false, "no_new_privileges": false, "unknown": "private text"} {
		t.Run(name, func(t *testing.T) {
			bad := map[string]any{}
			for k, v := range good {
				bad[k] = v
			}
			bad[name] = value
			if err := validatePublisherRuntimeFacts(bad, p, 500, executable, digest); err == nil {
				t.Fatal("wrong post-exec fact accepted")
			}
		})
	}
	for _, value := range []any{true, "500", float64(500), json.Number("500.0"), json.Number("-1"), json.Number("999999999999999999999999")} {
		bad := map[string]any{}
		for k, v := range good {
			bad[k] = v
		}
		bad["mount_namespace"] = value
		if err := validatePublisherRuntimeFacts(bad, p, 500, executable, digest); err == nil {
			t.Fatalf("non-exact namespace accepted: %T", value)
		}
	}
}

func TestPublisherViewReleaseRequiresActualTreeProof(t *testing.T) {
	v := &trustedPublisherView{}
	c := &controller{publisherView: v, children: []childRecord{{Label: "empty-a", TreeReaped: false}}}
	if err := c.closePublisherView(); err == nil {
		t.Fatal("unproved tree permitted release")
	}
	if v.released {
		t.Fatal("failed cleanup manufactured release")
	}
	c.children[0].TreeReaped = true
	v.uncertainMount = true
	if err := c.closePublisherView(); err == nil {
		t.Fatal("unmeasured acquired mount permitted release")
	}
	if v.released {
		t.Fatal("uncertain mount manufactured release")
	}
}

func TestPublisherHeldLayerReplacementRefused(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("root-owned layer fixture requires root; no mount or native child")
	}
	root := t.TempDir()
	path := filepath.Join(root, "upper")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	v := &trustedPublisherView{}
	if err := v.holdLayer(path); err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, n := range v.layers {
			_ = n.file.Close()
		}
	}()
	if err := v.verifyLayers(); err != nil {
		t.Fatal(err)
	}
	moved := path + "-original"
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := v.verifyLayers(); err == nil {
		t.Fatal("replacement upper borrowed held authority")
	}
	if _, err := os.Stat(moved); err != nil {
		t.Fatal("original owned layer lost", err)
	}
}

func publisherPolicyFixture() hostPolicySnapshot {
	yes := true
	one, zero, namespaces := int64(1), int64(0), int64(58121)
	files := map[string]policyFileFact{}
	for key, digest := range map[string]string{"code": stockCodeProfileSHA, "abi": "e510bb8f6788b45e48de2f859a6f94a7b8416cbac5a1051814cfce925fa911bd", "global": "2c1d72013b53bf103fb8abe2199ce5775f8493df1acd3ef497046156c09fdf7c", "alias": "e7b8b278db3f7aca74e0176f22cbd3adc3c89212cc9e66a90a2787b3ceee8030"} {
		files[key] = policyFileFact{State: "present", SHA256: digest}
	}
	files["local_code"] = policyFileFact{State: "absent"}
	return hostPolicySnapshot{Files: files, Entries: map[string]string{"disable_code": "absent", "force_complain_code": "absent"}, Sysctls: map[string]*int64{"apparmor_userns": &one, "apparmor_unconfined": &zero, "userns_clone": &one, "max_user_namespaces": &namespaces}, Grammar: "known-ubuntu-vscode-userns", KnownProfileMatch: true, LoadedMode: "unconfined", Enabled: &yes, CodeAbsent: &yes}
}

func TestPublisherPolicyAdmissionDoesNotInferUnknownState(t *testing.T) {
	if err := admittedPublisherPolicy(publisherPolicyFixture()); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*hostPolicySnapshot){
		"local-override": func(s *hostPolicySnapshot) {
			s.Files["local_code"] = policyFileFact{State: "present", SHA256: stockCodeProfileSHA}
		},
		"include-mismatch": func(s *hostPolicySnapshot) {
			s.Files["alias"] = policyFileFact{State: "present", SHA256: stockCodeProfileSHA}
		},
		"unloaded":        func(s *hostPolicySnapshot) { s.LoadedMode = "absent" },
		"complain":        func(s *hostPolicySnapshot) { s.LoadedMode = "complain" },
		"missing-enabled": func(s *hostPolicySnapshot) { s.Enabled = nil },
		"existing-code":   func(s *hostPolicySnapshot) { no := false; s.CodeAbsent = &no },
		"disabled":        func(s *hostPolicySnapshot) { s.Entries["disable_code"] = "symlink" },
		"force-complain":  func(s *hostPolicySnapshot) { s.Entries["force_complain_code"] = "ordinary" },
		"unknown-grammar": func(s *hostPolicySnapshot) { s.Grammar = "unknown" },
		"missing-sysctl":  func(s *hostPolicySnapshot) { delete(s.Sysctls, "apparmor_userns") },
	} {
		t.Run(name, func(t *testing.T) {
			s := publisherPolicyFixture()
			change(&s)
			if err := admittedPublisherPolicy(s); err == nil {
				t.Fatal("unadmitted stock-policy state accepted")
			}
		})
	}
}

func TestPublisherCgroupUsesLiveDriverNotController(t *testing.T) {
	stat := func(start string) []byte {
		return []byte("101 (inert worker) S " + strings.Repeat("0 ", 18) + start + " 0\n")
	}
	for _, changed := range []bool{false, true} {
		calls := []string{}
		statReads := 0
		read := func(path string, limit int64, uid uint32) ([]byte, error) {
			calls = append(calls, path)
			if uid != 1001 {
				t.Fatal("wrong expected driver owner")
			}
			switch path {
			case "/proc/101/stat":
				statReads++
				if limit != 8192 {
					t.Fatal("stat bound changed")
				}
				if changed && statReads == 2 {
					return stat("1235"), nil
				}
				return stat("1234"), nil
			case "/proc/101/cgroup":
				if limit != 4096 {
					t.Fatal("cgroup bound changed")
				}
				return []byte("0::/owned-duplex-driver\n"), nil
			default:
				t.Fatalf("read outside actual live driver: %s", path)
				return nil, nil
			}
		}
		body, err := publisherDriverCgroupUsing(isolationProof{PID: 101, UID: 1001}, read)
		if changed {
			if err == nil {
				t.Fatal("reused driver PID accepted")
			}
		} else if err != nil || string(body) != "0::/owned-duplex-driver\n" {
			t.Fatal("live driver membership lost", err)
		}
		if len(calls) != 3 {
			t.Fatal("missing before/after process join", calls)
		}
	}
}

func TestPublisherParentLifetimeRefusesWithoutAncestryFallback(t *testing.T) {
	var missing *publisherParent
	if err := missing.verify(); err == nil {
		t.Fatal("missing actual sudo parent accepted")
	}
	changed := &publisherParent{pid: os.Getppid() + 1}
	if err := changed.verify(); err == nil {
		t.Fatal("different actual parent accepted")
	}
	for _, pair := range [][2]uint64{{0, 1}, {1, 0}, {1, 1}} {
		if parent, err := holdPublisherParent(pair[0], pair[1], 1001); err == nil || parent != nil {
			t.Fatal("unproved namespace pair accepted", pair)
		}
	}
}
