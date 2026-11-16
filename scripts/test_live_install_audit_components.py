"""Regression proof for released-CLI native component attribution and negative outcomes."""
import hashlib
import importlib.util
import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest


def load_audit():
    file = Path(__file__).resolve().parent / "live-install-audit.py"
    spec = importlib.util.spec_from_file_location("live_install_audit", file)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class PublishedCatalogNativeProofTest(unittest.TestCase):
    def test_native_component_identity_must_match_real_changed_client_file(self):
        audit = load_audit()
        parsed = {"data": {"actions": [{"action": "install", "component_support": [
            {"kind": "skill", "name": "gcp-playbooks", "support": "native"},
            {"kind": "extension", "name": "external-extension", "support": "unsupported"},
            {"kind": "mcp_server", "name": "no-credentials", "support": "prepared"},
        ]}]}}
        names = audit.native_components_from_add(parsed)
        self.assertEqual(names, ("gcp-playbooks",))
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".gemini"
            skill = root / "skills/gcp-playbooks/SKILL.md"
            skill.parent.mkdir(parents=True)
            skill.write_text("---\nname: gcp-playbooks\n---\n")
            other = root / "foreign-neighbor.txt"
            other.write_text("unrelated content")
            changes = {
                (".gemini/skills/gcp-playbooks/SKILL.md", "sha256:owned"),
                (".gemini/foreign-neighbor.txt", "sha256:foreign"),
            }
            observed = audit.attributable_native_changes(
                [root], changes, "sre-extension", "gemini", names)
            self.assertEqual(observed, [".gemini/skills/gcp-playbooks/SKILL.md"])
            self.assertEqual(audit.attributable_native_changes(
                [root], changes, "sre-extension", "gemini"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "sre-extension", [c[0] for c in changes],
                "gemini", names), observed)
            skill.unlink()
            self.assertEqual(audit.native_identity_residuals(
                [root], "sre-extension", [c[0] for c in changes],
                "gemini", names), [])

    def test_mcp_server_native_registration_under_declared_component_id(self):
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".cline"
            settings = root / "data/settings/cline_mcp_settings.json"
            settings.parent.mkdir(parents=True)
            settings.write_text('{"mcpServers":{"dockerhub":{"command":"node"}}}')
            changed = {(".cline/data/settings/cline_mcp_settings.json", "owned")}
            self.assertEqual(audit.attributable_native_changes(
                [root], changed, "docker-hub", "cline", ("dockerhub",)),
                [".cline/data/settings/cline_mcp_settings.json"])
            self.assertEqual(audit.attributable_native_changes(
                [root], changed, "docker-hub", "cline"), [])
            self.assertEqual(audit.native_identity_residuals(
                [root], "docker-hub", [".cline/data/settings/cline_mcp_settings.json"],
                "cline", ("dockerhub",)),
                [".cline/data/settings/cline_mcp_settings.json"])

    def test_prepared_support_is_never_mislabeled_native(self):
        audit = load_audit()
        prepared = {"data": {"actions": [{"component_support": [
            {"kind": "mcp_server", "name": "dockerhub", "support": "prepared"}]}]}}
        self.assertTrue(audit.prepared_only_add(prepared))
        self.assertEqual(audit.native_components_from_add(prepared), ())
        for corrupt in (None, {"data": {"actions": [{"component_support": None}]}},
                        {"data": {"actions": [{"component_support": [
                            {"kind": "mcp_server", "name": "dockerhub", "support": "native"}]}]}}):
            self.assertFalse(audit.prepared_only_add(corrupt))

    def test_expected_policy_refusal_is_negative_proof_not_native_pass(self):
        audit = load_audit()
        row = {"status": "documented-or-source-unsupported", "kind": "plugin",
               "target": "cline", "source": "marketing-kit", "native_file_delta": 0,
               "native_identity_files": [], "listed_identity": False,
               "failed_install_registered": False, "failed_install_residue": False,
               "sentinels_preserved": True,
               "install": {"exit_code": 1, "stderr_tail":
                           "release does not support complete target set; missing cline"}}
        self.assertTrue(audit.expected_negative_coverage(row))
        for key, value in (("native_file_delta", 1), ("failed_install_residue", True),
                           ("sentinels_preserved", False), ("target", "gemini"),
                           ("status", "passed")):
            self.assertFalse(audit.expected_negative_coverage({**row, key: value}))

    def test_prepared_negative_requires_full_lifecycle_and_never_native_pass(self):
        audit = load_audit()
        case = {"status": "prepared-lifecycle-passed", "kind": "plugin",
                "target": "opencode", "source": "docker-hub",
                "prepared_components_only": True, "native_component_candidates": [],
                "native_identity_files": [], "native_residuals": [],
                "listed_identity": True, "repeat_verified": True,
                "removal_verified": True, "sentinels_preserved": True,
                "post_remove_identity_registered": False,
                "failed_install_residue": False,
                "install": {"exit_code": 0}, "cleanup": {"exit_code": 0}}
        self.assertTrue(audit.expected_negative_coverage(case))
        # A fully verified prepared lifecycle is a valid *non-native* result
        # regardless of source, including a harmless OpenCode config write.
        self.assertTrue(audit.expected_negative_coverage({
            **case, "source": "atlassian",
            "native_identity_files": ["opencode/opencode.json"]}))
        self.assertTrue(audit.expected_negative_coverage({
            **case, "source": "cloudflare",
            "native_identity_files": ["opencode/opencode.json",
                                      "opencode/skills/cloudflare/SKILL.md",
                                      "opencode/skills/cloudflare/references/readme.md"]}))
        for key, value in (("status", "passed"), ("target", "claude"),
                           ("native_identity_files", ["native.json"]),
                           ("post_remove_identity_registered", True),
                           ("native_component_candidates", ["dockerhub"]),
                           ("listed_identity", False),
                           ("native_residuals", ["opencode/active.json"]),
                           ("install", {"exit_code": 1}),
                           ("cleanup", {"exit_code": 1}),
                           ("repeat_verified", False), ("removal_verified", False),
                           ("prepared_components_only", False)):
            self.assertFalse(audit.expected_negative_coverage({**case, key: value}))


    def test_known_unqualified_opencode_sources_are_negative_not_native(self):
        audit = load_audit()
        sources = (
            "github:itential/builder-skills@2ccdf77358b018ad4df125422a067fe84e63de1d",
            "github:GoogleCloudPlatform/google-cloud-storage-plugin@b346e39698fcdfdc711ca3ee35e41c8dbaa1aa2b",
            "github:cometchat/cometchat-skills@8652f1ea41d12e5058123db1e4e6babef9c918f1",
            "github:uizze/uizze@fc401e9e654cbafe427b0c98142fc8bccd0c4e7e",
        )
        row = {"status": "package-installed-native-unqualified",
               "kind": "plugin", "target": "opencode",
               "source": sources[0],
               "prepared_components_only": False, "native_component_candidates": [],
               "native_identity_files": [], "listed_identity": True,
               "repeat_verified": True, "removal_verified": True,
               "post_remove_identity_registered": False,
               "native_residuals": [], "sentinels_preserved": True,
               "failed_install_residue": False,
               "install": {"exit_code": 0}, "cleanup": {"exit_code": 0}}
        # Only four exact frozen sources with independently observed clean
        # package lifecycles can qualify as non-native negative coverage.
        for source in sources:
            for version in ("agentplugins 0.1.95", "agentplugins 0.1.101",
                            "agentplugins 0.1.102"):
                with self.subTest(source=source, version=version):
                    self.assertTrue(audit.expected_negative_coverage(
                        {**row, "source": source}, version))
        for version in (None, "agentplugins 0.1.103", "agentplugins 0.2.0"):
            self.assertFalse(audit.expected_negative_coverage(row, version))
        for key, bad in (
            ("source", "github:unknown/plugin@" + "a" * 40),
            ("status", "passed"),
            ("target", "claude"),
            ("native_identity_files", ["opencode/opencode.json"]),
            ("native_component_candidates", ["builder-agent"]),
            ("listed_identity", False),
            ("repeat_verified", False),
            ("removal_verified", False),
            ("native_residuals", ["opencode/registry.json"]),
            ("post_remove_identity_registered", True),
            ("sentinels_preserved", False),
            ("failed_install_residue", True),
            ("prepared_components_only", True),
            ("install", {"exit_code": 1}),
            ("cleanup", {"exit_code": 1}),
        ):
            with self.subTest(key=key):
                self.assertFalse(audit.expected_negative_coverage(
                    {**row, key: bad}, "agentplugins 0.1.102"))

    def test_pi_explicit_unsupported_refusals_never_become_native_pass(self):
        audit = load_audit()
        row = {"kind": "plugin", "target": "pi", "source": "atlassian",
               "status": "documented-or-source-unsupported",
               "native_file_delta": 0, "native_identity_files": [],
               "listed_identity": False, "failed_install_registered": False,
               "failed_install_residue": False, "sentinels_preserved": True,
               "install": {"exit_code": 1, "stderr_tail":
                           "release does not support complete target set; missing pi"}}
        self.assertTrue(audit.expected_negative_coverage(row))
        self.assertTrue(audit.expected_negative_coverage({
            **row, "install": {"exit_code": 1, "stderr_tail":
                              "pi package manifest overrides selected extension discovery"}}))
        for key, bad in (("target", "codex"), ("status", "passed"),
                         ("native_file_delta", 1), ("listed_identity", True),
                         ("failed_install_registered", True),
                         ("native_identity_files", ["pi/active.json"]),
                         ("sentinels_preserved", False),
                         ("install", {"exit_code": 0, "stderr_tail": "missing pi"}),
                         ("install", {"exit_code": 1, "stderr_tail": "network timeout"})):
            with self.subTest(key=key, bad=bad):
                self.assertFalse(audit.expected_negative_coverage({**row, key: bad}))

    def test_grok_vendor_user_guide_is_not_an_active_plugin_registration(self):
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".grok"
            docs = root / "docs/user-guide"
            docs.mkdir(parents=True)
            ignored = ["05-configuration.md", "16-subagents.md"]
            for name in ignored:
                (docs / name).write_text("name: documentation\\n", encoding="utf-8")
            active = root / "plugins/documentation/plugin.json"
            active.parent.mkdir(parents=True)
            active.write_text('{"name":"documentation"}')
            observed = [".grok/docs/user-guide/" + name for name in ignored]
            observed.append(".grok/plugins/documentation/plugin.json")
            self.assertEqual(audit.native_identity_residuals(
                [root], "documentation", observed, "grok"),
                [".grok/plugins/documentation/plugin.json"])
            self.assertFalse(audit.passive_native_path(
                "grok", "docs/user-guide/custom-plugin.json"))
            self.assertFalse(audit.passive_native_path(
                "grok", "plugins/documentation/plugin.json"))


class SnapshotChangeEvidenceTest(unittest.TestCase):
    def test_reports_created_changed_deleted_hashes_without_file_contents(self):
        audit = load_audit()
        with TemporaryDirectory() as tmp:
            root = Path(tmp) / ".codex"
            root.mkdir()
            (root / "changed").write_bytes(b"private before")
            (root / "deleted").write_bytes(b"private deleted")
            (root / "stable").write_bytes(b"private stable")
            before = audit.snapshot([root])
            (root / "changed").write_bytes(b"private after")
            (root / "deleted").unlink()
            (root / "created").write_bytes(b"private created")
            after = audit.snapshot([root])
            evidence = audit.snapshot_change_evidence(before, after)
            self.assertEqual(evidence["total"], 3)
            self.assertEqual(evidence["omitted"], 0)
            entries = {entry["path"]: entry for entry in evidence["entries"]}
            self.assertEqual(list(entries), [".codex/changed", ".codex/created", ".codex/deleted"])
            self.assertEqual(entries[".codex/changed"]["before"],
                             hashlib.sha256(b"private before").hexdigest())
            self.assertEqual(entries[".codex/changed"]["after"],
                             hashlib.sha256(b"private after").hexdigest())
            self.assertIsNone(entries[".codex/created"]["before"])
            self.assertEqual(entries[".codex/created"]["after"], after[".codex/created"])
            self.assertEqual(entries[".codex/deleted"]["before"], before[".codex/deleted"])
            self.assertIsNone(entries[".codex/deleted"]["after"])
            self.assertFalse(any(entry["path_truncated"] for entry in evidence["entries"]))
            self.assertNotIn("private", json.dumps(evidence))
            self.assertEqual(audit.snapshot_change_evidence(after, after),
                             {"total": 0, "omitted": 0, "entries": []})

    def test_bounds_entries_and_paths_and_preserves_unsafe_markers(self):
        audit = load_audit()
        long_path = ".codex/000-" + "x" * 3000
        before = {long_path: "symlink"}
        after = {f".codex/{index:03d}": "a" * 64 for index in range(1, 105)}
        after[long_path] = "unreadable"
        evidence = audit.snapshot_change_evidence(before, after)
        self.assertEqual(evidence["total"], 105)
        self.assertEqual(len(evidence["entries"]), 100)
        self.assertEqual(evidence["omitted"], 5)
        first = evidence["entries"][0]
        self.assertEqual(len(first["path"]), 2048)
        self.assertTrue(first["path_truncated"])
        self.assertEqual(first["before"], "symlink")
        self.assertEqual(first["after"], "unreadable")
        self.assertEqual(evidence, audit.snapshot_change_evidence(
            dict(reversed(list(before.items()))), dict(reversed(list(after.items())))))


if __name__ == "__main__":
    unittest.main()
