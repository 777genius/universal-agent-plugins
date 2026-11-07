"""Regression proof for released-CLI native component attribution and negative outcomes."""
import importlib.util
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
            skill.write_text("---\\nname: gcp-playbooks\\n---\\n")
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

    def test_prepared_negative_requires_exact_source_and_verified_lifecycle(self):
        audit = load_audit()
        case = {"status": "prepared-lifecycle-passed", "kind": "plugin",
                "target": "opencode", "source": "docker-hub",
                "prepared_components_only": True, "native_identity_files": [],
                "repeat_verified": True, "removal_verified": True,
                "sentinels_preserved": True, "failed_install_residue": False}
        self.assertTrue(audit.expected_negative_coverage(case))
        for key, value in (("source", "atlassian"), ("target", "claude"),
                           ("native_identity_files", ["native.json"]),
                           ("repeat_verified", False), ("removal_verified", False),
                           ("prepared_components_only", False)):
            self.assertFalse(audit.expected_negative_coverage({**case, key: value}))


if __name__ == "__main__":
    unittest.main()
