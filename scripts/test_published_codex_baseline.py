"""Negative controls for the finite historical npm observer, never native PASS."""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


def module():
    path = Path(__file__).with_name("published_codex_baseline.py")
    spec = importlib.util.spec_from_file_location("published_codex_baseline", path)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


BASELINE = module()


def fixture():
    return {"name": "selected", "repo": "owner/repo", "folder": "selected",
            "source": "github:owner/repo@" + "a" * 40 + "//skills/selected"}


def installation():
    return {"name": "selected", "source": "owner/repo/skills/selected",
            "clients": [{"client_id": "codex", "scope": "project",
                         "materialization": "materialized", "activation": "active",
                         "verification": "installation_verified",
                         "authentication": "not_required", "policy": "allowed"}]}


def dto(repeated=False):
    return {"status": "succeeded", "repairInstructions": [], "items": [{
        "alias": "selected", "candidateId": "candidate-" + ("b" if repeated else "a") * 64,
        "destinationPath": "/case/project/.agents/skills/selected",
        "message": "", "status": "unchanged" if repeated else "succeeded", "targetId": "codex"}]}


def valid_report():
    row = {"index": 0, "source": fixture()["source"], "identity": "selected",
           "target": "codex", "kind": "skill", "status": "incomplete-native-proof",
           "codex_public_add_ready": False, "codex_public_repeat_ready": False,
           "repeat_verified": False,
           "historical_npm_proof": {"package": "universal-agent-plugins", "version": "0.1.95",
                "commit": BASELINE.COMMIT, "binary_sha256": BASELINE.DIGEST,
                "binary_size": BASELINE.SIZE, "launcher_sources_verified": True, "commands_verified": 6},
           "matched_installation_ids": ["one"], "repeat_installation_ids": ["one"],
           "repeat_native_unchanged": True, "native_residuals": [],
           "post_removal_skill": None, "post_removal_leftovers": [], "post_skill_tree_delta": [],
           "codex_runtime_proof": {"native_version": "0.153.4", "observations": []}}
    for key in ("historical_add_verified", "historical_repeat_add_verified",
                "historical_repeat_lifecycle_verified", "listed_identity", "lock_written",
                "lock_contains_identity", "lock_exact_source_pinned", "skill_bytes_match",
                "removal_verified", "sentinels_preserved", "post_list_schema_valid",
                "repeat_lock_unchanged"):
        row[key] = True
    for key in ("failed_install_registered", "failed_install_residue",
                "post_remove_identity_registered", "post_lock_pinned"):
        row[key] = False
    for key in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
        row[key] = {"exit_code": 0, "timeout": False}
    for phase, present in (("before-install", False), ("installed", True),
                           ("repeat-installed", True), ("removed", False)):
        row["codex_runtime_proof"]["observations"].append({
            "phase": phase, "selected_skill_present": present, "native_exit_code": 0,
            "foreign_skill_present": True, "runtime_integrity_verified": True,
            "runtime_semantics_unchanged": True, "runtime_files_fully_read": {},
            "temporary_git_files_fully_read": {}})
    second = copy.deepcopy(row)
    second["index"] = 1
    return {"acceptance": BASELINE.ACCEPTANCE, "target": "codex", "kind": "skill",
            "platform": "linux", "architecture": "x86_64", "cases_attempted": 2,
            "passed": 0, "full_coverage": False, "source_selection_complete": False,
            "cli_version": {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0},
            "client_version": {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0},
            "results": [row, second]}


class HistoricalDTOTest(unittest.TestCase):
    def check(self, value, matches=None, repeated=False):
        return BASELINE.historical_add(value, [installation()] if matches is None else matches,
            fixture(), "/case/project/.agents/skills/selected/SKILL.md", repeated)

    def test_exact_old_shape_and_independent_session_candidate_ids(self):
        self.assertTrue(self.check(dto()))
        self.assertTrue(self.check(dto(True), repeated=True))
        self.assertNotEqual(dto()["items"][0]["candidateId"], dto(True)["items"][0]["candidateId"])
        self.assertFalse(self.check(dto(), repeated=True))

    def test_modern_missing_null_wrong_field_type_source_and_destination_refused(self):
        for key, value in (("schema_version", 1), ("readiness", "ready"),
                           ("repairInstructions", None), ("items", {}),
                           ("status", "partial")):
            wrong = dto()
            wrong[key] = value
            self.assertFalse(self.check(wrong), (key, value))
        for key in dto():
            wrong = dto()
            del wrong[key]
            self.assertFalse(self.check(wrong), key)
        for key, value in (("candidateId", ""), ("candidateId", "a" * 64),
                           ("destinationPath", "/other/selected"), ("alias", "other"),
                           ("targetId", "claude"), ("message", None), ("status", True)):
            wrong = dto()
            wrong["items"][0][key] = value
            self.assertFalse(self.check(wrong), (key, value))
        for change in ({"source": "other/repo/skills/selected"}, {"name": "other"}):
            self.assertFalse(self.check(dto(), [{**installation(), **change}]))
        for key in ("scope", "activation", "verification", "authentication", "policy"):
            wrong = installation()
            wrong["clients"][0][key] = "unknown"
            self.assertFalse(self.check(dto(), [wrong]), key)


class HistoricalSummaryTest(unittest.TestCase):
    def check(self, report):
        return BASELINE.summary(report, {"skills": [fixture(), fixture()]})["historical_observation_complete"]

    def test_complete_observation_keeps_native_status_and_count(self):
        report = valid_report()
        original = copy.deepcopy(report)
        self.assertTrue(self.check(report))
        self.assertEqual(report, original)

    def test_wrong_binary_source_version_fields_phases_and_removal_fail(self):
        mutations = [
            lambda r: r.update(acceptance="native"),
            lambda r: r.update(passed=2),
            lambda r: r.update(cases_attempted=True),
            lambda r: r["cli_version"].update(value="agentplugins 0.1.95+source.abc"),
            lambda r: r["results"][0].update(source="other"),
            lambda r: r["results"][0].update(index=False),
            lambda r: r["results"][0].update(status="passed"),
            lambda r: r["results"][0].update(codex_public_add_ready=True),
            lambda r: r["results"][0].update(historical_add_verified=1),
            lambda r: r["results"][0].update(removal_verified=False),
            lambda r: r["results"][0]["cleanup"].update(exit_code=1),
            lambda r: r["results"][0]["cleanup"].update(exit_code=False),
            lambda r: r["results"][0]["historical_npm_proof"].update(binary_sha256="0" * 64),
            lambda r: r["results"][0]["historical_npm_proof"].update(commands_verified=5),
            lambda r: r["results"][0]["codex_runtime_proof"]["observations"].pop(),
            lambda r: r["results"][0]["codex_runtime_proof"]["observations"][1].update(
                foreign_skill_present=False),
        ]
        for index, mutate in enumerate(mutations):
            report = valid_report()
            mutate(report)
            self.assertFalse(self.check(report), index)


class PublishedProvenanceTest(unittest.TestCase):
    def test_source_binary_with_release_version_is_not_an_npm_launcher(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "agentplugins"
            path.write_bytes(b"agentplugins 0.1.95")
            with self.assertRaises(ValueError):
                BASELINE.PublishedBaseline(path, {}, Path(temp))

    def test_wrong_package_version_platform_and_asset_metadata_fail(self):
        observer = object.__new__(BASELINE.PublishedBaseline)
        observer.root, observer.project, observer.environment = Path("/package"), Path("/project"), {}
        valid = {"name": "universal-agent-plugins", "version": "0.1.95",
                 "platform": "linux-amd64", "tag": "agentplugins-v0.1.95",
                 "commit": BASELINE.COMMIT,
                 "asset": {"sha256": BASELINE.DIGEST, "size": BASELINE.SIZE,
                           "file": "agentplugins_0.1.95_linux_amd64"}, "binary": "/cache/binary"}
        import json
        from types import SimpleNamespace
        for key, value in (("name", "other"), ("version", "0.1.95+source.abc"),
                           ("platform", "linux-arm64"), ("commit", "0" * 40), ("asset", None)):
            wrong = {**valid, key: value}
            with patch.object(observer, "verify_sources"), patch.object(
                    BASELINE.subprocess, "run", return_value=SimpleNamespace(stdout=json.dumps(wrong))):
                with self.assertRaisesRegex(ValueError, "provenance mismatch"):
                    observer.read_metadata()

    def test_cached_source_binary_cannot_impersonate_release(self):
        with tempfile.TemporaryDirectory() as temp:
            binary = Path(temp) / "agentplugins"
            binary.write_bytes(b"source binary labeled 0.1.95")
            observer = object.__new__(BASELINE.PublishedBaseline)
            observer.metadata = {"binary": str(binary)}
            with patch.object(observer, "read_metadata", return_value=observer.metadata):
                with self.assertRaisesRegex(ValueError, "release binary mismatch"):
                    observer.after_command()


if __name__ == "__main__":
    unittest.main()
