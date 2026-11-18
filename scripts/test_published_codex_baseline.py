"""Negative controls for the finite historical npm observer, never native PASS."""
import copy
import importlib.util
import json
import os
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


# Genuine published 0.1.95 / Codex 0.153.4 artifact 14386239913.
# codex-skill.json SHA256 adadc5dc2b760a2ca0822e371ab48261cd12e4703f983250d542c9d1e4c811bd.
# All four add/list phase pairs are retained; only the private project prefix changed.
RAW_CASES = [{'fixture': {'repo': 'anthropics/skills',
              'ref': 'ca45fc496a3436251f191354091d557da31c15b5',
              'folder': 'frontend-design',
              'name': 'frontend-design',
              'source': 'https://github.com/anthropics/skills/tree/ca45fc496a3436251f191354091d557da31c15b5/skills/frontend-design',
              'skill_md_git_blob_sha1': 'a5333457c414d20d625f307df945842c0952ecc3'},
  'install': '{"status":"succeeded","items":[{"alias":"frontend-design","candidateId":"sha256:b596c15579514b31901b660dcee94def801033e595d112ff5afb9ebfefea6e8b","destinationPath":"/case/project/.uap/managed/clients/codex/skill-b0a5b0253f26020d7acb797250495459c1bd758fe4405-e38b1408fca0","message":"The '
             'Skill was installed.","status":"succeeded","targetId":"codex"}],"repairInstructions":[]}\n',
  'list': '{"schema_version":1,"command":"list","result":"success","data":{"installations":[{"installation_id":"e607fe42-39e6-4efd-ae13-77fa856ca440","name":"skill-b0a5b0253f26020d7acb797250495459c1bd758fe440530b3db1347e","source":"anthropics/skills/skills/frontend-design","clients":[{"client_id":"codex","scope":"project","materialization":"materialized","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"ca45fc496a3436251f191354091d557da31c15b5","tree_digest":"sha256:554fe3a1771a691a315cbf77927c47466ca6d7c0095ba9eba8f91527c2045c83","manifest_digest":"sha256:fd6cec3517f7197db2abcfadd5ef85c5473dcd905b792013c6a5c3f8b41ae99d"},"affected_surfaces":["codex"]}],"mixed_version":false}]}}\n',
  'repeat_add': '{"status":"succeeded","items":[{"alias":"frontend-design","candidateId":"sha256:b596c15579514b31901b660dcee94def801033e595d112ff5afb9ebfefea6e8b","destinationPath":"/case/project/.uap/managed/clients/codex/skill-b0a5b0253f26020d7acb797250495459c1bd758fe4405-e38b1408fca0","message":"The '
                'pinned Skill already '
                'matches.","status":"unchanged","targetId":"codex"}],"repairInstructions":[]}\n',
  'repeat_list': '{"schema_version":1,"command":"list","result":"success","data":{"installations":[{"installation_id":"e607fe42-39e6-4efd-ae13-77fa856ca440","name":"skill-b0a5b0253f26020d7acb797250495459c1bd758fe440530b3db1347e","source":"anthropics/skills/skills/frontend-design","clients":[{"client_id":"codex","scope":"project","materialization":"materialized","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"ca45fc496a3436251f191354091d557da31c15b5","tree_digest":"sha256:554fe3a1771a691a315cbf77927c47466ca6d7c0095ba9eba8f91527c2045c83","manifest_digest":"sha256:fd6cec3517f7197db2abcfadd5ef85c5473dcd905b792013c6a5c3f8b41ae99d"},"affected_surfaces":["codex"]}],"mixed_version":false}]}}\n'},
 {'fixture': {'repo': 'anthropics/skills',
              'ref': 'ca45fc496a3436251f191354091d557da31c15b5',
              'folder': 'pdf',
              'name': 'pdf',
              'source': 'https://github.com/anthropics/skills/tree/ca45fc496a3436251f191354091d557da31c15b5/skills/pdf',
              'skill_md_git_blob_sha1': 'd3e046a5ae107a6cb23cfb16c219837094ab35d3'},
  'install': '{"status":"succeeded","items":[{"alias":"pdf","candidateId":"sha256:ad1dd23110116893f4762cf23bcbe4bad649a6eef584fdaaf5223b281bc720f2","destinationPath":"/case/project/.uap/managed/clients/codex/skill-da182d36cdc9982baaa0d65a8e10ce05d57ea882eb83a-de0e4ab626b6","message":"The '
             'Skill was installed.","status":"succeeded","targetId":"codex"}],"repairInstructions":[]}\n',
  'list': '{"schema_version":1,"command":"list","result":"success","data":{"installations":[{"installation_id":"a51ddaa9-3fae-40e6-97c2-b6c87b53d301","name":"skill-da182d36cdc9982baaa0d65a8e10ce05d57ea882eb83a03398d4b00a","source":"anthropics/skills/skills/pdf","clients":[{"client_id":"codex","scope":"project","materialization":"materialized","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"ca45fc496a3436251f191354091d557da31c15b5","tree_digest":"sha256:451b323a647513e5e2d9fec6d3dac2ef32eedaae9a400fd57ec82c9370718b1e","manifest_digest":"sha256:08e01df8ca12c5ff3349857211b34cea5ec5e440ef5b5edacbba18669e702745"},"affected_surfaces":["codex"]}],"mixed_version":false}]}}\n',
  'repeat_add': '{"status":"succeeded","items":[{"alias":"pdf","candidateId":"sha256:ad1dd23110116893f4762cf23bcbe4bad649a6eef584fdaaf5223b281bc720f2","destinationPath":"/case/project/.uap/managed/clients/codex/skill-da182d36cdc9982baaa0d65a8e10ce05d57ea882eb83a-de0e4ab626b6","message":"The '
                'pinned Skill already '
                'matches.","status":"unchanged","targetId":"codex"}],"repairInstructions":[]}\n',
  'repeat_list': '{"schema_version":1,"command":"list","result":"success","data":{"installations":[{"installation_id":"a51ddaa9-3fae-40e6-97c2-b6c87b53d301","name":"skill-da182d36cdc9982baaa0d65a8e10ce05d57ea882eb83a03398d4b00a","source":"anthropics/skills/skills/pdf","clients":[{"client_id":"codex","scope":"project","materialization":"materialized","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"ca45fc496a3436251f191354091d557da31c15b5","tree_digest":"sha256:451b323a647513e5e2d9fec6d3dac2ef32eedaae9a400fd57ec82c9370718b1e","manifest_digest":"sha256:08e01df8ca12c5ff3349857211b34cea5ec5e440ef5b5edacbba18669e702745"},"affected_surfaces":["codex"]}],"mixed_version":false}]}}\n'}]


def fixture(index=0):
    return copy.deepcopy(RAW_CASES[index]["fixture"])


def installation(index=0):
    return json.loads(RAW_CASES[index]["list"])["data"]["installations"][0]


def dto(repeated=False, index=0):
    return json.loads(RAW_CASES[index]["repeat_add" if repeated else "install"])


def native_skill_path(index=0, project="/case/project"):
    return str(Path(project) / ".agents/skills" / fixture(index)["name"] / "SKILL.md")


def valid_report():
    row = {"index": 0, "source": fixture()["source"], "identity": fixture()["name"],
           "target": "codex", "kind": "skill", "status": "incomplete-native-proof",
           "codex_public_add_ready": False, "codex_public_repeat_ready": False,
           "repeat_verified": False,
           "historical_npm_proof": {"package": "universal-agent-plugins", "version": "0.1.95",
                "commit": BASELINE.COMMIT, "binary_sha256": BASELINE.DIGEST,
                "binary_size": BASELINE.SIZE, "launcher_sources_verified": True, "commands_verified": 6},
           "matched_installation_ids": [installation()["installation_id"]],
           "repeat_installation_ids": [installation()["installation_id"]],
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
    second.update(index=1, source=fixture(1)["source"], identity=fixture(1)["name"],
                  matched_installation_ids=[installation(1)["installation_id"]],
                  repeat_installation_ids=[installation(1)["installation_id"]])
    return {"acceptance": BASELINE.ACCEPTANCE, "target": "codex", "kind": "skill",
            "platform": "linux", "architecture": "x86_64", "cases_attempted": 2,
            "passed": 0, "full_coverage": False, "source_selection_complete": False,
            "cli_version": {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0},
            "client_version": {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0},
            "results": [row, second]}


class HistoricalDTOTest(unittest.TestCase):
    def check(self, value, matches=None, repeated=False):
        return BASELINE.historical_add(value, [installation()] if matches is None else matches,
            fixture(), native_skill_path(), repeated)

    def test_exact_release_shape_and_stable_candidate_ids(self):
        self.assertTrue(self.check(dto()))
        self.assertTrue(self.check(dto(True), repeated=True))
        self.assertEqual(dto()["items"][0]["candidateId"], dto(True)["items"][0]["candidateId"])
        self.assertFalse(self.check(dto(), repeated=True))

    def test_actual_four_raw_dtos_and_lists_observe_only_historical_contract(self):
        for index, case in enumerate(RAW_CASES):
            for added, listed, repeated in (("install", "list", False),
                                             ("repeat_add", "repeat_list", True)):
                with self.subTest(skill=fixture(index)["name"], phase=added):
                    value = json.loads(case[added])
                    matches = json.loads(case[listed])["data"]["installations"]
                    before = copy.deepcopy((value, matches))
                    self.assertTrue(BASELINE.historical_add(
                        value, matches, fixture(index), native_skill_path(index), repeated))
                    self.assertEqual((value, matches), before)
                    self.assertEqual(matches[0]["clients"][0]["activation"], "prepared")
                    self.assertEqual(matches[0]["clients"][0]["authentication"], "not_checked")

    def test_source_derived_identity_managed_destination_and_state_fail_closed(self):
        original = installation()
        for field, value in (("candidateId", "candidate-" + "a" * 64),
                             ("candidateId", "sha256:" + "0" * 64),
                             ("destinationPath", str(Path(native_skill_path()).parent)),
                             ("destinationPath", dto()["items"][0]["destinationPath"] + "/."),
                             ("destinationPath", "/outside/" + Path(dto()["items"][0]["destinationPath"]).name)):
            wrong = dto()
            wrong["items"][0][field] = value
            self.assertFalse(self.check(wrong), (field, value))
        for change in ({"name": fixture()["name"]}, {"installation_id": "other"},
                       {"installation_id": ""}, {"installation_id": None}):
            self.assertFalse(self.check(dto(), [{**original, **change}]), change)
        for field, value in (("activation", "active"), ("authentication", "not_required"),
                             ("scope", "user"), ("policy", "blocked")):
            wrong = copy.deepcopy(original)
            wrong["clients"][0][field] = value
            self.assertFalse(self.check(dto(), [wrong]), field)
        wrong = copy.deepcopy(original)
        wrong["clients"][0]["package_revision"]["resolved_revision"] = "0" * 40
        self.assertFalse(self.check(dto(), [wrong]))
        for field in ("repo", "ref", "folder", "name"):
            changed = fixture()
            changed[field] = "other"
            self.assertFalse(BASELINE.historical_add(dto(), [original], changed, native_skill_path()), field)
        for path in (None, "relative/SKILL.md", "/case/project/.agents/skills/other/SKILL.md"):
            self.assertFalse(BASELINE.historical_add(dto(), [original], fixture(), path), path)

    def test_legacy_observation_never_replaces_current_public_ready(self):
        path = Path(__file__).with_name("codex_runtime_audit.py")
        spec = importlib.util.spec_from_file_location("historical_ready_guard", path)
        runtime = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(runtime)
        for index in range(2):
            for repeated in (False, True):
                self.assertFalse(runtime.public_skill_ready(
                    dto(repeated, index), [installation(index)], fixture(index)["name"]))
        modern = {**dto(), "schema_version": 1, "command": "skills add", "result": "success",
                  "scope": "project", "readiness": "ready", "next_actions": []}
        active = installation()
        active["clients"][0].update(activation="active", authentication="not_required")
        self.assertTrue(runtime.public_skill_ready(modern, [active], fixture()["name"]))
        self.assertFalse(BASELINE.historical_add(modern, [active], fixture(), native_skill_path()))

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
        return BASELINE.summary(report, {"skills": [fixture(), fixture(1)]})["historical_observation_complete"]

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
            (Path(temp) / "home").mkdir()
            with self.assertRaises(ValueError):
                BASELINE.PublishedBaseline(path, {"HOME": str(Path(temp) / "home"),
                    "XDG_CACHE_HOME": str(Path(temp) / "home/.cache")}, Path(temp) / "project")

    def test_wrong_package_version_platform_and_asset_metadata_fail(self):
        observer = object.__new__(BASELINE.PublishedBaseline)
        observer.root, observer.project, observer.environment = Path("/package"), Path("/project"), {}
        observer.binary = Path("/cache/binary")
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
                    BASELINE, "regular_bytes", return_value=b"{}"), patch.object(
                    BASELINE.subprocess, "run", return_value=SimpleNamespace(stdout=json.dumps(wrong), stderr="")):
                with self.assertRaisesRegex(ValueError, "provenance mismatch"):
                    observer.read_metadata()

    def test_cached_source_binary_cannot_impersonate_release(self):
        with tempfile.TemporaryDirectory() as temp:
            binary = Path(temp) / "agentplugins"
            binary.write_bytes(b"source binary labeled 0.1.95")
            observer = object.__new__(BASELINE.PublishedBaseline)
            observer.home = Path(temp)
            observer.binary = binary
            observer.home_identity = BASELINE.directory_chain(observer.home, observer.home)
            observer.metadata = {"binary": str(binary)}
            with patch.object(observer, "read_metadata", return_value=observer.metadata):
                with self.assertRaisesRegex(ValueError, "size mismatch"):
                    observer.after_command()



class HistoricalModeTest(unittest.TestCase):
    def load_live(self):
        path = Path(__file__).with_name("live-install-audit.py")
        spec = importlib.util.spec_from_file_location("historical_mode_live", path)
        live = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(live)
        return live

    def test_mode_refuses_other_sources_counts_platforms_and_acceptances(self):
        live = self.load_live()
        cases = [("codex", "plugin", "2", "linux", "x86_64", BASELINE.ACCEPTANCE),
                 ("claude", "skill", "2", "linux", "x86_64", BASELINE.ACCEPTANCE),
                 ("codex", "skill", "3", "linux", "x86_64", BASELINE.ACCEPTANCE),
                 ("codex", "skill", "2", "darwin", "x86_64", BASELINE.ACCEPTANCE),
                 ("codex", "skill", "2", "linux", "aarch64", BASELINE.ACCEPTANCE),
                 ("codex", "skill", "2", "linux", "x86_64", "kiro-prepared-installation")]
        for target, kind, count, platform, machine, acceptance in cases:
            with self.subTest(case=(target, kind, count, platform, machine, acceptance)), \
                    patch("sys.argv", ["audit", "--cli", "/inert", "--target", target,
                        "--kind", kind, "--limit", count, "--report-dir", "/inert",
                        "--acceptance", acceptance]), patch.object(live.sys, "platform", platform), \
                    patch.object(live.platform, "machine", return_value=machine), \
                    patch.object(live, "do_case") as case, patch("sys.stderr"):
                with self.assertRaises(SystemExit) as rejected:
                    live.main()
                self.assertEqual(rejected.exception.code, 2)
                case.assert_not_called()

    def test_source_version_string_cannot_qualify_historical_mode_or_native_ready(self):
        live = self.load_live()
        with tempfile.TemporaryDirectory() as temporary:
            cli = Path(temporary) / "agentplugins-source"
            cli.write_text("source build labeled agentplugins 0.1.95")
            versions = [{"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0},
                        {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0}]
            def version(argv, *args):
                return versions[0] if str(argv[0]) == str(cli) else versions[1]
            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                    patch.dict(live.AGENT_BINARIES, {"codex": "/inert/codex"}), \
                    patch("sys.argv", ["audit", "--cli", str(cli), "--target", "codex",
                        "--kind", "skill", "--limit", "2", "--report-dir", temporary,
                        "--acceptance", BASELINE.ACCEPTANCE]), \
                    patch.object(live, "version_evidence", side_effect=version), \
                    patch.object(live, "call", side_effect=AssertionError("must not execute")) as call, \
                    patch.object(live.sys, "platform", "linux"), \
                    patch.object(live.platform, "machine", return_value="x86_64"), patch("builtins.print"):
                self.assertEqual(live.main(), 1)
                call.assert_not_called()
            report = json.loads((Path(temporary) / "codex-skill.json").read_text())
            self.assertEqual(report["cli_version"], versions[0])
            self.assertEqual(report["passed"], 0)
            self.assertEqual(report["historical_lifecycle_observed"], 0)
            self.assertFalse(report["historical_observation_complete"])
            self.assertEqual([row["status"] for row in report["results"]], ["harness-error"] * 2)
            self.assertTrue(all("actual npm launcher" in row["error"] for row in report["results"]))


class FiniteReadTest(unittest.TestCase):
    def test_sparse_oversize_rejected_before_open(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "huge"
            with path.open("wb") as stream:
                stream.truncate(BASELINE.SIZE + 1)
            with patch.object(BASELINE.os, "open", side_effect=AssertionError("must not open")):
                with self.assertRaisesRegex(ValueError, "size mismatch"):
                    BASELINE.regular_bytes(path, BASELINE.SIZE, exact_size=BASELINE.SIZE)

    def test_exact_bounded_file_and_metadata_cap(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "file"
            path.write_bytes(b"abc")
            self.assertEqual(BASELINE.regular_bytes(path, 3, exact_size=3), b"abc")
            with self.assertRaisesRegex(ValueError, "size mismatch"):
                BASELINE.regular_bytes(path, 2)

    def test_final_named_check_keeps_descriptor_held(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "file"
            path.write_bytes(b"abc")
            original_lstat = Path.lstat
            calls = 0
            descriptor = None
            original_fstat = BASELINE.os.fstat
            def held_stat(fd):
                nonlocal descriptor
                descriptor = fd
                return original_fstat(fd)
            def named_stat(value):
                nonlocal calls
                if value == path:
                    calls += 1
                    if calls == 2:
                        self.assertIsNotNone(descriptor)
                        original_fstat(descriptor)  # EBADF if the reader closed early.
                return original_lstat(value)
            with patch.object(BASELINE.os, "fstat", side_effect=held_stat), patch.object(
                    Path, "lstat", named_stat):
                self.assertEqual(BASELINE.regular_bytes(path, 3), b"abc")
            self.assertEqual(calls, 2)

    def test_symlink_cache_ancestor_refused_without_reading_external_file(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            home, external = root / "home", root / "external"
            home.mkdir()
            external.mkdir()
            (home / ".cache").symlink_to(external, target_is_directory=True)
            with self.assertRaisesRegex(ValueError, "ordinary directory"):
                BASELINE.directory_chain(home, home / ".cache/agentplugins")

    def test_cache_path_outside_home_and_leaf_symlink_refused(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            home = root / "home"
            home.mkdir()
            with self.assertRaisesRegex(ValueError, "escapes"):
                BASELINE.directory_chain(home, root)
            outside = root / "outside"
            outside.write_bytes(b"abc")
            link = home / "link"
            link.symlink_to(outside)
            with self.assertRaisesRegex(ValueError, "size mismatch"):
                BASELINE.regular_bytes(link, 3)



class HistoricalOrchestrationTest(unittest.TestCase):
    def test_real_do_case_preserves_six_commands_four_checkpoints_and_negative_readiness(self):
        path = Path(__file__).with_name("live-install-audit.py")
        spec = importlib.util.spec_from_file_location("historical_live_audit", path)
        live = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(live)
        events, observations = [], []
        class Runtime:
            def __init__(self, *args):
                self.checkpoint("before-install", False)
            def guard(self):
                pass
            def checkpoint(self, phase, present):
                events.append(("native", phase))
                observations.append({"phase": phase, "selected_skill_present": present})
            def temporary_git_state(self):
                return {}
            def evidence(self):
                return {"native_version": "0.153.4", "observations": list(observations)}
        class Observer:
            def __init__(self, *args):
                self.count, self.proof = 0, None
            def after_command(self):
                self.count += 1
                events.append(("observe", self.count))
                self.proof = {"commands_verified": self.count}
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp) / "run"
            selected = fixture()
            selected["skill_md_git_blob_sha1"] = "a" * 40
            add_count = 0
            removed = False
            def invoke(argv, env, project, *args, **kwargs):
                nonlocal add_count, removed
                command = "add" if argv[1:3] == ["skills", "add"] else argv[1]
                events.append(("command", command))
                skill = project / ".agents/skills" / selected["name"] / "SKILL.md"
                if command == "add":
                    add_count += 1
                    if add_count == 1:
                        skill.parent.mkdir()
                        skill.write_text("---\nname: " + selected["name"] + "\n---\n")
                        (project / "uap.toml").write_text("selected")
                        (project / "uap.lock").write_text("selected")
                    document = dto(add_count == 2)
                    document["items"][0]["destinationPath"] = document["items"][0]["destinationPath"].replace(
                        "/case/project", str(project))
                elif command == "remove":
                    removed = True
                    skill.unlink()
                    skill.parent.rmdir()
                    (project / "uap.toml").unlink()
                    (project / "uap.lock").unlink()
                    document = {"result": "success"}
                else:
                    self.assertEqual(command, "list")
                    registered = installation()
                    document = {"schema_version": 1, "command": "list", "result": "success",
                                "data": {"installations": [] if removed else [registered]}}
                return {"exit_code": 0, "timeout": False, "stdout_tail": "", "stderr_tail": "",
                        "_json": document}
            with patch.object(live, "call", side_effect=invoke), patch.object(
                    live, "locked_skill_is_pinned", return_value=True), patch.object(
                    live, "matches_pinned_skill_bytes", return_value=True), patch.dict(
                    live.AGENT_BINARIES, {"codex": "/inert/codex"}), patch.dict(
                    live.CODEX_RUNTIME_AUDIT, {"CodexRuntimeAudit": Runtime}), patch.dict(
                    live.HISTORICAL_CODEX, {"PublishedBaseline": Observer}):
                row = live.do_case("/inert/npm-launcher", "codex", "skill", selected, 0,
                                   root, {}, historical_baseline=True)
            self.assertEqual(events, [
                ("native", "before-install"), ("command", "add"), ("observe", 1),
                ("command", "list"), ("observe", 2), ("native", "installed"),
                ("command", "add"), ("observe", 3), ("command", "list"), ("observe", 4),
                ("native", "repeat-installed"), ("command", "remove"), ("observe", 5),
                ("command", "list"), ("observe", 6), ("native", "removed")])
            self.assertEqual(row["status"], "incomplete-native-proof")
            self.assertIs(row["codex_public_add_ready"], False)
            self.assertIs(row["codex_public_repeat_ready"], False)
            self.assertIs(row["repeat_verified"], False)
            for key in ("historical_add_verified", "historical_repeat_add_verified",
                        "historical_repeat_lifecycle_verified", "removal_verified", "sentinels_preserved"):
                self.assertIs(row[key], True, key)
            self.assertEqual(row["historical_npm_proof"]["commands_verified"], 6)
            self.assertEqual([value["phase"] for value in row["codex_runtime_proof"]["observations"]],
                             ["before-install", "installed", "repeat-installed", "removed"])


if __name__ == "__main__":
    unittest.main()
