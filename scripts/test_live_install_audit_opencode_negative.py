"""Retain truthful partial OpenCode coverage without claiming native PASS."""
import copy
import unittest

from test_live_install_audit_env import load_audit


class OpenCodePartialCoverageTest(unittest.TestCase):
    # Observed NONPASS rows from run 44911211196, job 136989175529, artifact
    # 14727154686 (ZIP SHA256 9cef82819a6d425a6bfe142bd165cc9dca945d83ee3f83d50e82dad19cda22b4).
    # That run failed coverage with native PASS=0; classification stays non-native.
    # Each has prepared components plus unsupported extensions; Google also
    # reports google-cloud-storage-okf as stdio_runtime_unavailable.
    SOURCES = (
        (23, "itential-builder", "github:itential/builder-skills@2ccdf77358b018ad4df125422a067fe84e63de1d"),
        (24, "google-cloud-storage", "github:GoogleCloudPlatform/google-cloud-storage-plugin@b346e39698fcdfdc711ca3ee35e41c8dbaa1aa2b"),
        (27, "cometchat-skills", "github:cometchat/cometchat-skills@8652f1ea41d12e5058123db1e4e6babef9c918f1"),
        (28, "uizze-ui-slop", "github:uizze/uizze@fc401e9e654cbafe427b0c98142fc8bccd0c4e7e"),
    )

    @staticmethod
    def observed_row(index, identity, source):
        return {
            "index": index, "identity": identity, "source": source,
            "kind": "plugin", "target": "opencode",
            "status": "package-installed-native-unqualified",
            "prepared_components_only": False,
            "native_identity_files": [], "native_component_candidates": [],
            "listed_identity": True, "repeat_verified": True,
            "removal_verified": True, "post_remove_identity_registered": False,
            "native_residuals": [], "sentinels_preserved": True,
            "failed_install_residue": False,
            "install": {"exit_code": 0}, "cleanup": {"exit_code": 0},
        }

    def test_observed_four_partial_rows_are_negative_only(self):
        audit = load_audit()
        rows = [self.observed_row(*source) for source in self.SOURCES]
        before = copy.deepcopy(rows)
        self.assertEqual(sum(audit.expected_negative_coverage(
            row, "agentplugins 0.1.115") for row in rows), 4)
        self.assertEqual(rows, before)
        self.assertEqual(sum(row["status"] == "passed" for row in rows), 0)
        self.assertTrue(all(row["prepared_components_only"] is False for row in rows))
        for version in ("agentplugins 0.1.95", "agentplugins 0.1.101", "agentplugins 0.1.102"):
            self.assertTrue(all(audit.expected_negative_coverage(row, version) for row in rows))

    def test_unknown_cli_source_revision_and_support_status_refuse(self):
        audit = load_audit()
        for source in self.SOURCES:
            row = self.observed_row(*source)
            for version in (None, "agentplugins 0.1.114", "agentplugins 0.1.116", "0.1.115"):
                with self.subTest(source=source[1], version=version):
                    self.assertFalse(audit.expected_negative_coverage(row, version))
            for change in (
                {"source": "github:example/unknown@" + "a" * 40},
                {"source": row["source"].rsplit("@", 1)[0] + "@" + "f" * 40},
                {"status": "passed"}, {"status": "prepared-lifecycle-passed"},
                {"prepared_components_only": True},
                {"native_component_candidates": ["unexpected-native"]},
                {"native_identity_files": ["opencode/opencode.json"]},
            ):
                with self.subTest(source=source[1], change=change):
                    self.assertFalse(audit.expected_negative_coverage(
                        {**row, **change}, "agentplugins 0.1.115"))

    def test_every_lifecycle_and_containment_condition_remains_required(self):
        audit = load_audit()
        for source in self.SOURCES:
            row = self.observed_row(*source)
            for key, value in (
                ("kind", "skill"), ("target", "qoder"),
                ("sentinels_preserved", False), ("failed_install_residue", True),
                ("native_identity_files", ["outside/active.json"]),
                ("native_identity_files", ["opencode/../active.json"]),
                ("native_identity_files", None), ("native_component_candidates", None),
                ("listed_identity", False), ("repeat_verified", False),
                ("removal_verified", False), ("post_remove_identity_registered", True),
                ("native_residuals", ["opencode/skills/retained/SKILL.md"]),
                ("install", {"exit_code": 1}), ("cleanup", {"exit_code": 1}),
                ("install", None), ("cleanup", None),
            ):
                with self.subTest(source=source[1], key=key, value=value):
                    self.assertFalse(audit.expected_negative_coverage(
                        {**row, key: value}, "agentplugins 0.1.115"))


if __name__ == "__main__":
    unittest.main()
