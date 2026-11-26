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


class HistoricalPluginArchiveTest(unittest.TestCase):
    def fixtures(self):
        return json.loads((Path(__file__).resolve().parents[1] / "tests/live-install/fixtures.json").read_text())

    def observer(self, root):
        from types import SimpleNamespace
        root = root.resolve()
        for name in ("home", "uap-data", "project"):
            (root / name).mkdir()
        return SimpleNamespace(home=root / "home", project=root / "project",
            launcher=Path("/actual-npm/bin/agentplugins.js"),
            environment={"HOME": str(root / "home"), "AGENTPLUGINS_HOME": str(root / "uap-data")})

    def refusal(self):
        error = ("agentplugins: load signed Directory: directory snapshot is expired: "
                 "expires_at=2026-10-30T20:03:50Z now=2026-11-21T23:16:22Z\n")
        return {"exit_code": 1, "timeout": False, "stdout_tail": "", "stdout_full": "",
                "stderr_tail": error, "stderr_full": error}

    def info(self, source):
        pin = self.fixtures()["signed_release_pins"][source]
        value = {"exit_code": 0, "timeout": False, "stdout_tail": "bounded output", "stderr_tail": "", "stderr_full": "",
            "_json": {"schema_version": 1, "command": "info", "result": "success", "data": {
                "snapshot_sequence": 48, "snapshot_digest": "sha256:" + BASELINE.ARCHIVE_DIGEST,
                "selected_distribution": pin["distribution"], "selected_release": {
                    key: pin[field] for key, field in (("repository", "repository"), ("package_path", "path"),
                        ("resolved_revision", "revision"), ("release_sequence", "sequence"),
                        ("tree_digest", "tree_digest"), ("manifest_digest", "manifest_digest"))}}}}
        value["stdout_full"] = json.dumps(value["_json"])
        return value

    def proof(self, source):
        return {"scope": "immutable-official-historical-feed", "origin": BASELINE.ARCHIVE_ORIGIN,
                "ledger_commit": BASELINE.ARCHIVE_COMMIT, "sequence": 48, "key_id": BASELINE.ARCHIVE_KEY,
                "snapshot_sha256": BASELINE.ARCHIVE_DIGEST, "envelope_sha256": BASELINE.ARCHIVE_ENVELOPE_DIGEST,
                "generated_at": BASELINE.ARCHIVE_GENERATED, "expires_at": BASELINE.ARCHIVE_EXPIRES,
                "original_cli_trust_unchanged": True, "release_pin": self.fixtures()["signed_release_pins"][source]}

    def feed(self, source):
        return {"current_default": {"status": "current-default-compatibility-failed",
                    "origin": BASELINE.DEFAULT_ORIGIN, "compatibility_passed": False, "native_passed": False,
                    "complete_output_verified": True,
                    "started_at": "2026-11-21T23:16:21+00:00", "ended_at": "2026-11-21T23:16:23+00:00",
                    "observation": self.refusal()},
                "archive": self.proof(source),
                "archive_info": {**BASELINE.bounded_command(self.info(source)), "complete_output_verified": True}}

    def report(self):
        rows = []
        for index, source in enumerate(BASELINE.ARCHIVE_SOURCES):
            row = {"index": index, "source": source, "identity": source, "kind": "plugin", "target": "codex",
                   "status": "historical-archive-lifecycle-observed", "historical_archive_status": "passed",
                   "historical_plugin_feed": self.feed(source),
                   "historical_npm_proof": {"package": "universal-agent-plugins", "version": "0.1.95",
                       "commit": BASELINE.COMMIT, "binary_sha256": BASELINE.DIGEST, "binary_size": BASELINE.SIZE,
                       "launcher_sources_verified": True, "commands_verified": 8},
                   "matched_installation_ids": ["id-" + source], "repeat_installation_ids": ["id-" + source],
                   "native_residuals": [], "native_uninstall": {"completed": True, "native_plugin_absent": True,
                       "native_marketplace_absent": True},
                   "codex_plugin_sqlite_proof": {"native_version": "0.153.4", "runtime_semantics_unchanged": True,
                       "observations": [{"phase": "after-public-command", "runtime_files_fully_read": {}}]}}
            for key in ("repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged", "removal_verified",
                        "sentinels_preserved", "listed_identity", "post_list_schema_valid"):
                row[key] = True
            for key in ("failed_install_registered", "failed_install_residue", "post_remove_identity_registered"):
                row[key] = False
            for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                row[phase] = {"exit_code": 0, "timeout": False}
            rows.append(row)
        return {"acceptance": BASELINE.PLUGIN_ACCEPTANCE, "target": "codex", "kind": "plugin",
                "platform": "linux", "architecture": "x86_64", "cases_attempted": 2, "passed": 0,
                "verified_negative_coverage": 0, "full_coverage": False, "source_selection_complete": False,
                "cli_version": {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0},
                "client_version": {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0}, "results": rows}

    def test_default_failure_is_observed_before_exact_archive_selection(self):
        now = BASELINE.instant("2026-11-21T23:16:22Z")
        for source in BASELINE.ARCHIVE_SOURCES:
            with self.subTest(source=source), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary)); calls = []
                def invoke(argv, environment, project, timeout, *, capture_full=False):
                    self.assertEqual(argv, [str(observer.launcher), "info", source, "--target", "codex", "--format", "json"])
                    self.assertEqual(project, observer.project)
                    self.assertEqual(timeout, 80)
                    self.assertTrue(capture_full)
                    calls.append(dict(environment))
                    return self.refusal() if len(calls) == 1 else self.info(source)
                with patch.object(BASELINE, "utc_now", return_value=now), patch.object(
                        BASELINE, "archive_cache", return_value=self.proof(source)):
                    feed = BASELINE.prepare_plugin_archive(observer, invoke, source,
                        self.fixtures()["signed_release_pins"][source])
                self.assertNotIn("AGENTPLUGINS_DIRECTORY_ORIGIN", calls[0])
                self.assertEqual(calls[1]["AGENTPLUGINS_DIRECTORY_ORIGIN"], BASELINE.ARCHIVE_ORIGIN)
                self.assertTrue(BASELINE.recorded_default_refusal(feed["current_default"]))
                self.assertFalse(feed["current_default"]["compatibility_passed"])
                self.assertEqual(observer.plugin_archive, (source, self.fixtures()["signed_release_pins"][source]))

    def test_unexpected_default_and_archive_outcomes_preserve_failure_evidence(self):
        now = BASELINE.instant("2026-11-21T23:16:22Z")
        for mutation in ("default-success", "default-timeout", "default-text", "default-time",
                         "default-leading-stderr", "default-leading-stdout", "default-truncated-capture",
                         "archive-exit", "archive-json", "archive-scope", "archive-sequence", "archive-pin",
                         "archive-stderr-prefix", "archive-missing-full"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary)); default, archived = self.refusal(), self.info("atlassian")
                if mutation == "default-success": default["exit_code"] = 0
                elif mutation == "default-timeout": default["timeout"] = True
                elif mutation == "default-text": default["stderr_full"] = default["stderr_tail"] = "another error"
                elif mutation == "default-time":
                    default["stderr_full"] = default["stderr_tail"] = default["stderr_tail"].replace("23:16:22", "23:17:22")
                elif mutation == "default-leading-stderr": default["stderr_full"] = "hidden prefix\n" + " " * 2000 + default["stderr_tail"]
                elif mutation == "default-leading-stdout": default["stdout_full"] = "hidden prefix\n" + " " * 2000
                elif mutation == "default-truncated-capture": default.pop("stderr_full")
                elif mutation == "archive-exit": archived["exit_code"] = 1
                elif mutation == "archive-json": archived["_json"] = None
                elif mutation == "archive-scope": archived["_json"]["command"] = "list"
                elif mutation == "archive-sequence": archived["_json"]["data"]["snapshot_sequence"] = True
                elif mutation == "archive-stderr-prefix": archived["stderr_full"] = "hidden prefix\n" + " " * 2000
                elif mutation == "archive-missing-full": archived.pop("stdout_full")
                else: archived["_json"]["data"]["selected_release"]["resolved_revision"] = "f" * 40
                with patch.object(BASELINE, "utc_now", return_value=now), patch.object(
                        BASELINE, "archive_cache") as cache:
                    responses = iter((default, archived))
                    with self.assertRaises(BASELINE.ArchiveUnavailable) as failure:
                        BASELINE.prepare_plugin_archive(observer, lambda *args, **kwargs: next(responses), "atlassian",
                            self.fixtures()["signed_release_pins"]["atlassian"])
                cache.assert_not_called()
                self.assertIs(failure.exception.observation, observer.plugin_feed)
                self.assertNotIn("archive", observer.plugin_feed)
                self.assertFalse(hasattr(observer, "plugin_archive"))
                if mutation.startswith("default-"):
                    observation = observer.plugin_feed["current_default"]["observation"]
                    self.assertNotIn("stdout_full", observation)
                    self.assertNotIn("stderr_full", observation)
                    self.assertNotIn("complete_output_verified", observer.plugin_feed["current_default"])
                if mutation.startswith("archive-"):
                    self.assertNotIn("stdout_full", observer.plugin_feed["archive_info"])
                    self.assertNotIn("stderr_full", observer.plugin_feed["archive_info"])
                    self.assertNotIn("complete_output_verified", observer.plugin_feed["archive_info"])

    def test_cache_requires_exact_bounded_private_unmodified_original_bytes(self):
        import base64
        import hashlib
        now = BASELINE.instant("2026-11-21T23:16:22Z")
        pin = self.fixtures()["signed_release_pins"]["atlassian"]
        snapshot = {"sequence": 48, "generated_at": BASELINE.ARCHIVE_GENERATED,
            "expires_at": BASELINE.ARCHIVE_EXPIRES, "distributions": [{"id": pin["distribution"], "status": "active",
                "releases": [{"sequence": pin["sequence"], "package_source": {"repository": pin["repository"],
                    "path": pin["path"], "revision": pin["revision"]}, "tree_digest": pin["tree_digest"],
                    "manifest_digest": pin["manifest_digest"]}]}]}
        raw = json.dumps(snapshot).encode(); digest = hashlib.sha256(raw).hexdigest()
        envelope = json.dumps({"sequence": 48, "key_id": BASELINE.ARCHIVE_KEY,
                              "snapshot_digest": "sha256:" + digest}).encode()
        for mutation in ("none", "bytes", "symlink", "hardlink", "mode", "parent", "expired", "wrong-pin", "duplicate", "during-read"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary))
                observer.environment["AGENTPLUGINS_DIRECTORY_ORIGIN"] = BASELINE.ARCHIVE_ORIGIN
                cache = Path(observer.environment["AGENTPLUGINS_HOME"]) / "directory-v1-cache.json"
                record = {"schema_version": 1, "sequence": 48, "snapshot": base64.b64encode(raw).decode(),
                          "envelope": base64.b64encode(envelope).decode()}
                cache.write_text(json.dumps(record)); cache.chmod(0o600)
                selected = copy.deepcopy(pin)
                if mutation == "bytes": cache.write_text(json.dumps({**record, "snapshot": "Zg=="}))
                elif mutation in ("symlink", "hardlink"):
                    target = observer.project / "foreign"; cache.rename(target)
                    if mutation == "symlink": cache.symlink_to(target)
                    else: os.link(target, cache)
                elif mutation == "mode": cache.chmod(0o644)
                elif mutation == "parent":
                    saved = cache.parent.with_name("saved-data"); cache.parent.rename(saved); cache.parent.symlink_to(saved, target_is_directory=True)
                elif mutation == "wrong-pin": selected["tree_digest"] = "sha256:" + "f" * 64
                elif mutation == "duplicate": cache.write_text(json.dumps(record)[:-1] + ',"sequence":48}')
                clock = BASELINE.instant(BASELINE.ARCHIVE_EXPIRES) if mutation == "expired" else now
                original_read = BASELINE.regular_bytes
                def read(*args, **kwargs):
                    body = original_read(*args, **kwargs)
                    if mutation == "during-read": cache.chmod(0o644)
                    return body
                with patch.object(BASELINE, "ARCHIVE_DIGEST", digest), patch.object(BASELINE, "ARCHIVE_ENVELOPE_DIGEST",
                        hashlib.sha256(envelope).hexdigest()), patch.object(BASELINE, "utc_now", return_value=clock), \
                        patch.object(BASELINE, "regular_bytes", side_effect=read):
                    if mutation == "none":
                        proof = BASELINE.archive_cache(observer, "atlassian", selected)
                        self.assertEqual(proof["release_pin"], pin)
                    else:
                        with self.assertRaises((ValueError, OSError)):
                            BASELINE.archive_cache(observer, "atlassian", selected)

    def test_original_binary_checkpoint_also_rechecks_the_archive_each_time(self):
        import hashlib
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve(); home = root / "home"; home.mkdir()
            binary = home / "cache/binary"; binary.parent.mkdir(); binary.write_bytes(b"pinned")
            observer = object.__new__(BASELINE.PublishedBaseline)
            observer.home, observer.binary = home, binary
            observer.home_identity = BASELINE.directory_chain(home, home)
            observer.metadata, observer.commands = {"binary": str(binary)}, 0
            observer.plugin_archive = "atlassian", self.fixtures()["signed_release_pins"]["atlassian"]
            observer.plugin_feed = self.feed("atlassian")
            with patch.object(observer, "read_metadata", return_value=observer.metadata), \
                    patch.object(BASELINE, "SIZE", 6), patch.object(BASELINE, "DIGEST", hashlib.sha256(b"pinned").hexdigest()), \
                    patch.object(BASELINE, "archive_cache") as cache:
                observer.after_command()
                cache.assert_called_once_with(observer, *observer.plugin_archive)
                cache.side_effect = ValueError("archive changed after another original command")
                with self.assertRaises(BASELINE.ArchiveUnavailable) as failure:
                    observer.after_command()
                self.assertIs(failure.exception.observation, observer.plugin_feed)
                self.assertEqual(cache.call_count, 2)

    def test_archive_summary_keeps_native_zero_and_all_lifecycle_guards(self):
        now = BASELINE.instant("2026-11-21T23:16:22Z")
        with patch.object(BASELINE, "utc_now", return_value=now):
            report = self.report(); original = copy.deepcopy(report)
            summary = BASELINE.plugin_summary(report, self.fixtures())
            self.assertTrue(summary["historical_plugin_observation_complete"])
            self.assertEqual(summary["archived_plugin_lifecycles_observed"], 2)
            self.assertFalse(summary["current_default_compatibility_passed"])
            self.assertEqual(summary["current_default_native_passed"], 0)
            self.assertEqual(report, original)
            mutations = [lambda r: r.update(passed=2), lambda r: r.update(passed=False),
                lambda r: r.update(cases_attempted=True), lambda r: r.update(kind="skill"),
                lambda r: r.update(acceptance="native"), lambda r: r["cli_version"].update(exit_code=False),
                lambda r: r["results"][0].update(source="foreign"),
                lambda r: r["results"][0].update(status="passed"),
                lambda r: r["results"][0].update(historical_archive_status="cleanup-failed"),
                lambda r: r["results"][0]["historical_npm_proof"].update(commands_verified=6),
                lambda r: r["results"][0]["historical_plugin_feed"]["archive"].update(sequence=True),
                lambda r: r["results"][0]["historical_plugin_feed"]["current_default"].pop("complete_output_verified"),
                lambda r: r["results"][0]["historical_plugin_feed"]["current_default"]["observation"].update(stderr_full="hidden prefix"),
                lambda r: r["results"][0]["historical_plugin_feed"]["archive_info"].pop("complete_output_verified"),
                lambda r: r["results"][0]["historical_plugin_feed"]["archive"]["release_pin"].update(revision="f" * 40),
                lambda r: r["results"][0].update(native_residuals=["unexpected"]),
                lambda r: r["results"][0]["native_uninstall"].update(native_plugin_absent=False),
                lambda r: r["results"][0]["codex_plugin_sqlite_proof"].update(runtime_semantics_unchanged=False)]
            for key in ("repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged", "removal_verified",
                        "sentinels_preserved", "listed_identity", "post_list_schema_valid"):
                mutations.append(lambda r, key=key: r["results"][0].update({key: False}))
            for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                mutations.append(lambda r, phase=phase: r["results"][0][phase].update(timeout=True))
            for index, mutate in enumerate(mutations):
                with self.subTest(mutation=index):
                    report = self.report(); mutate(report)
                    self.assertFalse(BASELINE.plugin_summary(report, self.fixtures())["historical_plugin_observation_complete"])

    def test_main_requires_explicit_archive_mode_and_preserves_default_failure(self):
        for acceptance, mutation in ((BASELINE.PLUGIN_ACCEPTANCE, None), ("native", None),
                                     (BASELINE.PLUGIN_ACCEPTANCE, "unavailable")):
            with self.subTest(acceptance=acceptance, mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                live = HistoricalModeTest().load_live()
                cli = Path(temporary) / "launcher"
                cli.write_text("inert integration fixture")
                sample = self.report()
                def version(argv, *args):
                    return sample["cli_version"] if argv[0] == str(cli) else sample["client_version"]
                def case(*args, **kwargs):
                    self.assertEqual(kwargs, {"historical_plugin_archive": True}
                                     if acceptance == BASELINE.PLUGIN_ACCEPTANCE else {})
                    if mutation:
                        raise live.HISTORICAL_CODEX["ArchiveUnavailable"](self.feed(args[3]))
                    return copy.deepcopy(sample["results"][args[4]])
                summary_globals = live.HISTORICAL_CODEX["plugin_summary"].__globals__
                with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                        patch.dict(live.AGENT_BINARIES, {"codex": "/inert/codex"}), \
                        patch("sys.argv", ["audit", "--cli", str(cli), "--target", "codex", "--kind", "plugin",
                            "--limit", "2", "--report-dir", temporary, "--acceptance", acceptance]), \
                        patch.object(live, "version_evidence", side_effect=version), \
                        patch.object(live, "do_case", side_effect=case) as observed, \
                        patch.object(live.sys, "platform", "linux"), \
                        patch.object(live.platform, "machine", return_value="x86_64"), \
                        patch.dict(summary_globals, {"utc_now": lambda: BASELINE.instant("2026-11-21T23:16:22Z")}), \
                        patch("builtins.print"):
                    self.assertEqual(live.main(), 0 if acceptance == BASELINE.PLUGIN_ACCEPTANCE and not mutation else 1)
                    self.assertEqual(observed.call_count, 2)
                report = json.loads((Path(temporary) / "codex-plugin.json").read_text())
                self.assertEqual(report["passed"], 0)
                self.assertFalse(report["full_coverage"])
                if acceptance == BASELINE.PLUGIN_ACCEPTANCE:
                    self.assertFalse(report["current_default_compatibility_passed"])
                    self.assertEqual(report["current_default_refusals_observed"], 2)
                    self.assertIs(report["historical_plugin_observation_complete"], mutation is None)
                else:
                    self.assertNotIn("historical_plugin_observation_complete", report)
                if mutation:
                    self.assertEqual([row["status"] for row in report["results"]], ["harness-error"] * 2)
                    self.assertTrue(all("historical_plugin_feed" in row for row in report["results"]))

    def test_archive_mode_rejects_wrong_client_scope_count_or_host_before_execution(self):
        live = HistoricalModeTest().load_live()
        for target, kind, count, system, machine in (
                ("cursor", "plugin", 2, "linux", "x86_64"), ("codex", "skill", 2, "linux", "x86_64"),
                ("codex", "plugin", 30, "linux", "x86_64"), ("codex", "plugin", 2, "win32", "x86_64"),
                ("codex", "plugin", 2, "linux", "aarch64")):
            with self.subTest(case=(target, kind, count, system, machine)), \
                    patch("sys.argv", ["audit", "--cli", "/inert", "--target", target, "--kind", kind,
                        "--limit", str(count), "--report-dir", "/inert", "--acceptance", BASELINE.PLUGIN_ACCEPTANCE]), \
                    patch.object(live.sys, "platform", system), \
                    patch.object(live.platform, "machine", return_value=machine), \
                    patch.object(live, "do_case") as observed, patch("sys.stderr"):
                with self.assertRaises(SystemExit) as rejected:
                    live.main()
                self.assertEqual(rejected.exception.code, 2)
                observed.assert_not_called()



class ArchivedPluginFeedTest(unittest.TestCase):
    def test_real_plugin_orchestration_keeps_feed_provenance_without_skill_observer(self):
        live = HistoricalModeTest().load_live()
        fixtures = self.fixtures()
        for index, source in enumerate(BASELINE.ARCHIVE_SOURCES):
            with self.subTest(source=source), tempfile.TemporaryDirectory() as temporary:
                events, observers = [], []
                pin = fixtures["signed_release_pins"][source]
                identifier = "00000000-0000-4000-8000-000000000001"
                registered, native_file = False, None
                class Observer:
                    def __init__(self, launcher, environment, project):
                        self.launcher, self.environment = Path(launcher), environment
                        self.project, self.home = project, Path(environment["HOME"])
                        self.proof = {"package": "universal-agent-plugins", "version": "0.1.95",
                            "commit": BASELINE.COMMIT, "binary_sha256": BASELINE.DIGEST,
                            "binary_size": BASELINE.SIZE, "launcher_sources_verified": True,
                            "commands_verified": 0}
                        observers.append(self)
                    def after_command(self):
                        self.proof["commands_verified"] += 1
                        events.append(("observe", self.proof["commands_verified"]))
                installed = {"installation_id": identifier, "name": source,
                    "source": pin["repository"] + "/" + pin["path"],
                    "directory": {"product_id": source, "recorded_distribution": pin["distribution"],
                        "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"]},
                    "clients": [{"client_id": "codex", "scope": "user",
                        "materialization": "materialized", "activation": "active",
                        "verification": "installation_verified", "authentication": "not_checked",
                        "policy": "allowed", "package_revision": {
                            "distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                            "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
                            "manifest_digest": pin["manifest_digest"]}}]}
                def invoke(argv, environment, project, *args, **kwargs):
                    nonlocal registered, native_file
                    operation = argv[1]
                    origin = environment.get("AGENTPLUGINS_DIRECTORY_ORIGIN")
                    events.append(("command", operation))
                    if operation == "info":
                        if origin is None:
                            return self.default()
                        self.assertEqual(origin, BASELINE.ARCHIVE_ORIGIN)
                        return self.info(source)
                    self.assertEqual(origin, BASELINE.ARCHIVE_ORIGIN)
                    if operation == "add":
                        if not registered:
                            native_file = Path(environment["HOME"]) / ".codex/plugins" / (source + ".json")
                            native_file.parent.mkdir(parents=True)
                            native_file.write_text(source)
                        registered = True
                        document = {"schema_version": 1, "command": "add", "result": "success"}
                    elif operation == "remove":
                        self.assertIn("--external-uninstalled", argv)
                        self.assertFalse(native_file.exists())
                        registered = False
                        document = {"schema_version": 1, "command": "remove", "result": "success"}
                    else:
                        self.assertEqual(operation, "list")
                        document = {"schema_version": 1, "command": "list", "result": "success",
                            "data": {"installations": [copy.deepcopy(installed)] if registered else []}}
                    return {"exit_code": 0, "timeout": False, "stdout_tail": "", "stderr_tail": "",
                            "_json": document}
                def uninstall(identity, home, case, environment, project):
                    self.assertEqual(identity, source)
                    self.assertEqual(native_file.read_text(), source)
                    native_file.unlink()
                    events.append(("native", "uninstall"))
                    return {"completed": True, "native_plugin_absent": True, "native_marketplace_absent": True}
                archive_globals = live.HISTORICAL_CODEX["prepare_plugin_archive"].__globals__
                with patch.object(live, "call", side_effect=invoke), patch.object(
                        live, "initialize_codex_plugin_profile", return_value=(None, None)), patch.object(
                        live, "uninstall_native_codex_plugin", side_effect=uninstall), patch.dict(
                        live.AGENT_BINARIES, {"codex": "/inert/codex"}), patch.dict(
                        live.HISTORICAL_CODEX, {"PublishedBaseline": Observer}), patch.dict(
                        archive_globals, {"utc_now": lambda: BASELINE.instant("2026-11-21T12:43:09Z"),
                            "archive_cache": lambda *args: self.proof(source)}), patch.dict(
                        live.HISTORICAL_CODEX, {"historical_add": unittest.mock.Mock(
                            side_effect=AssertionError("raw Skill observer must not receive a plugin"))}):
                    row = live.do_case("/inert/npm-launcher", "codex", "plugin", source, index,
                        Path(temporary) / "run", fixtures["plugin_declared_names"],
                        fixtures["signed_release_pins"], historical_plugin_archive=True)
                    live.HISTORICAL_CODEX["historical_add"].assert_not_called()
                self.assertEqual([value for kind, value in events if kind == "command"],
                                 ["info", "info", "add", "list", "add", "list", "remove", "list"])
                self.assertEqual([value for kind, value in events if kind == "observe"], list(range(1, 9)))
                self.assertLess(events.index(("native", "uninstall")), events.index(("command", "remove")))
                self.assertEqual(row["historical_npm_proof"]["commands_verified"], 8)
                self.assertEqual(len(observers), 1)
                self.assertIsNone(row["skill_path"])
                self.assertIs(row["historical_add_verified"], False)
                self.assertIs(row["historical_repeat_add_verified"], False)
                for key in ("repeat_verified", "removal_verified", "sentinels_preserved", "listed_identity"):
                    self.assertIs(row[key], True, key)
                self.assertEqual(row["status"], "historical-archive-lifecycle-observed")
                self.assertEqual(row["historical_archive_status"], "passed")
                self.assertEqual(row["native_residuals"], [])
                self.assertFalse(native_file.exists())
                self.assertIs(row["post_remove_identity_registered"], False)
                self.assertTrue(all(row["native_uninstall"][key] for key in (
                    "completed", "native_plugin_absent", "native_marketplace_absent")))
                feed = row["historical_plugin_feed"]
                self.assertTrue(BASELINE.recorded_default_refusal(feed["current_default"]))
                self.assertEqual(feed["archive"], self.proof(source))
                self.assertEqual(feed["archive_info"]["exit_code"], 0)

    def test_real_plugin_repeat_keeps_skill_path_absent_and_finishes_both_lifecycles(self):
        """Inert processes exercise orchestration; they are not vendor evidence."""
        live = HistoricalModeTest().load_live()
        fixtures = self.fixtures()
        rows = []
        now = BASELINE.instant("2026-11-21T12:43:09Z")
        for index, source in enumerate(BASELINE.ARCHIVE_SOURCES):
            with self.subTest(source=source), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary) / "run"
                codex = Path(temporary) / "inert-codex"
                codex.write_text("never executed")
                events, observers = [], []
                removed = False
                pin = fixtures["signed_release_pins"][source]
                registered = {
                    "installation_id": "00000000-0000-4000-8000-000000000001",
                    "name": source, "source": pin["repository"] + "/" + pin["path"],
                    "directory": {"product_id": source, "recorded_distribution": pin["distribution"],
                        "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"]},
                    "clients": [{"client_id": "codex", "scope": "user", "materialization": "materialized",
                        "activation": "active", "verification": "installation_verified",
                        "package_revision": {"distribution_id": pin["distribution"],
                            "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                            "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}}]}

                class Observer:
                    def __init__(observer, launcher, environment, project):
                        observer.launcher, observer.environment = Path(launcher), environment
                        observer.project, observer.home = project, Path(environment["HOME"])
                        observer.count, observer.proof = 0, None
                        observers.append(observer)
                    def after_command(observer):
                        observer.count += 1
                        events.append(("provenance", observer.count))
                        observer.proof = {"package": "universal-agent-plugins", "version": "0.1.95",
                            "commit": BASELINE.COMMIT, "binary_sha256": BASELINE.DIGEST,
                            "binary_size": BASELINE.SIZE, "launcher_sources_verified": True,
                            "commands_verified": observer.count}
                        if getattr(observer, "plugin_archive", None) is not None:
                            BASELINE.archive_cache(observer, *observer.plugin_archive)

                class Runtime:
                    def __init__(runtime, binary, case_root, home, project, environment):
                        runtime.runtime = home / ".local/state/codex-sqlite"
                        runtime.runtime.mkdir(parents=True)
                        runtime.pin = runtime.runtime
                        runtime.config = home / ".codex/config.toml"
                        runtime.config.write_bytes(live.CODEX_RUNTIME_AUDIT["_sqlite_config"](runtime.pin))
                        runtime.observations = [{"runtime_files_fully_read": {}}]
                    def guard(runtime):
                        pass
                    def checkpoint(runtime, phase):
                        events.append(("runtime", phase))
                    def evidence(runtime):
                        return {"native_version": "0.153.4", "runtime_semantics_unchanged": True,
                                "observations": runtime.observations}

                def archive(observer, selected, selected_pin):
                    self.assertEqual(selected, source)
                    self.assertEqual(selected_pin, pin)
                    self.assertEqual(observer.environment["AGENTPLUGINS_DIRECTORY_ORIGIN"], BASELINE.ARCHIVE_ORIGIN)
                    return self.proof(source)

                def invoke(argv, environment, project, *args, **kwargs):
                    nonlocal removed
                    command = argv[1]
                    native = str(argv[0]) == str(codex)
                    events.append(("native" if native else "command", tuple(argv[1:])))
                    home = Path(environment["HOME"])
                    config = home / ".codex/config.toml"
                    marketplace = "agentplugins-" + "a" * 12
                    spec = source + "@" + marketplace
                    managed = home.parent / "uap-data/marketplace"
                    base = live.CODEX_RUNTIME_AUDIT["_sqlite_config"](home / ".local/state/codex-sqlite").decode()
                    market_config = '[marketplaces."' + marketplace + '"]\nsource = ' + json.dumps(str(managed)) + '\n'
                    if native:
                        self.assertEqual(environment["CODEX_HOME"], str(home / ".codex"))
                        if argv[1:3] == ["plugin", "remove"]:
                            self.assertEqual(argv[3:], [spec, "--json"])
                            config.write_text(base + market_config)
                        else:
                            self.assertEqual(argv[1:], ["plugin", "marketplace", "remove", marketplace, "--json"])
                            config.write_text(base)
                        document = {"result": "success"}
                    elif command == "info":
                        self.assertEqual(argv[2], source)
                        if "AGENTPLUGINS_DIRECTORY_ORIGIN" not in environment:
                            return self.default()
                        self.assertEqual(environment["AGENTPLUGINS_DIRECTORY_ORIGIN"], BASELINE.ARCHIVE_ORIGIN)
                        return self.info(source)
                    elif command == "add":
                        self.assertEqual(argv[2], source)
                        self.assertEqual(environment["AGENTPLUGINS_DIRECTORY_ORIGIN"], BASELINE.ARCHIVE_ORIGIN)
                        managed.mkdir(exist_ok=True)
                        config.write_text(base + '[plugins."' + spec + '"]\nenabled = true\n' + market_config)
                        document = {"result": "success"}
                    elif command == "remove":
                        self.assertEqual(argv[2], source)
                        self.assertIn("--external-uninstalled", argv)
                        native_state = live.tomllib.loads(config.read_text())
                        self.assertNotIn(spec, native_state.get("plugins", {}))
                        self.assertNotIn(marketplace, native_state.get("marketplaces", {}))
                        removed = True
                        managed.rmdir()
                        document = {"result": "success"}
                    else:
                        self.assertEqual(command, "list")
                        document = {"schema_version": 1, "command": "list", "result": "success",
                                    "data": {"installations": [] if removed else [registered]}}
                    return {"exit_code": 0, "timeout": False, "stdout_tail": "", "stderr_tail": "",
                            "_json": document}

                with patch.object(live, "call", side_effect=invoke), patch.dict(
                        live.AGENT_BINARIES, {"codex": str(codex)}, clear=True), patch.dict(
                        live.CODEX_RUNTIME_AUDIT, {"CodexPluginSQLiteAudit": Runtime,
                            "plugin_initialization": lambda *a: ({}, (), {}),
                            "system_skill_initialization": lambda *a: ({}, (), {})}), patch.dict(
                        live.HISTORICAL_CODEX, {"PublishedBaseline": Observer,
                            "prepare_plugin_archive": BASELINE.prepare_plugin_archive}), patch.object(
                        BASELINE, "archive_cache", side_effect=archive), patch.object(
                        BASELINE, "utc_now", return_value=now), patch.dict(
                        live.HISTORICAL_CODEX, {"historical_add": unittest.mock.Mock(
                            side_effect=AssertionError("plugin entered the Skill observer"))}):
                    row = live.do_case("/inert/npm-launcher", "codex", "plugin", source, index,
                                       root, fixtures["plugin_declared_names"], fixtures["signed_release_pins"],
                                       historical_plugin_archive=True)
                    live.HISTORICAL_CODEX["historical_add"].assert_not_called()
                self.assertEqual([entry[1][0] for entry in events if entry[0] == "command"],
                                 ["info", "info", "add", "list", "add", "list", "remove", "list"])
                self.assertEqual([entry[1] for entry in events if entry[0] == "provenance"], list(range(1, 9)))
                self.assertEqual(len([entry for entry in events if entry[0] == "native"]), 2)
                self.assertEqual(row["status"], "historical-archive-lifecycle-observed")
                self.assertEqual(row["historical_archive_status"], "passed")
                self.assertIsNone(row["skill_path"])
                self.assertIsNone(row["codex_public_add_ready"])
                self.assertIs(row["historical_repeat_add_verified"], False)
                for key in ("repeat_verified", "removal_verified", "sentinels_preserved", "post_list_schema_valid"):
                    self.assertIs(row[key], True, key)
                self.assertEqual(row["historical_npm_proof"]["commands_verified"], 8)
                self.assertEqual(row["native_residuals"], [])
                self.assertIs(row["post_remove_identity_registered"], False)
                for key in ("completed", "native_plugin_absent", "native_marketplace_absent"):
                    self.assertIs(row["native_uninstall"][key], True, key)
                self.assertEqual(row["historical_plugin_feed"]["archive"], self.proof(source))
                self.assertIs(row["historical_plugin_feed"]["current_default"]["compatibility_passed"], False)
                rows.append(row)
        report = {**self.report(), "results": rows}
        with patch.object(BASELINE, "utc_now", return_value=now):
            summary = BASELINE.plugin_summary(report, fixtures)
        self.assertTrue(summary["historical_plugin_observation_complete"])
        self.assertEqual(summary["archived_plugin_lifecycles_observed"], 2)
        self.assertEqual(summary["current_default_refusals_observed"], 2)
        self.assertIs(summary["current_default_compatibility_passed"], False)
        self.assertEqual(summary["current_default_native_passed"], 0)

    def test_plugin_mode_is_host_target_kind_count_bound_and_keeps_default_failure_on_error(self):
        live = HistoricalModeTest().load_live()
        for target, kind, count, host, machine in (
                ("claude", "plugin", "2", "linux", "x86_64"),
                ("codex", "skill", "2", "linux", "x86_64"),
                ("codex", "plugin", "3", "linux", "x86_64"),
                ("codex", "plugin", "2", "darwin", "x86_64"),
                ("codex", "plugin", "2", "linux", "aarch64")):
            with self.subTest(target=target, kind=kind, count=count, host=host, machine=machine), \
                 patch("sys.argv", ["audit", "--cli", "/inert", "--target", target, "--kind", kind,
                    "--limit", count, "--report-dir", "/inert", "--acceptance", BASELINE.PLUGIN_ACCEPTANCE]), \
                 patch.object(live.sys, "platform", host), patch.object(live.platform, "machine", return_value=machine), \
                 patch.object(live, "do_case") as case, patch("sys.stderr"):
                with self.assertRaises(SystemExit) as refused: live.main()
                self.assertEqual(refused.exception.code, 2); case.assert_not_called()
        with tempfile.TemporaryDirectory() as temporary:
            cli = Path(temporary) / "npm-launcher"; cli.write_text("inert")
            def version(argv, *args):
                return {"status": "reported", "value": "agentplugins 0.1.95" if str(argv[0]) == str(cli) else "codex-cli 0.153.4", "exit_code": 0}
            refusal = self.report()["results"][0]["historical_plugin_feed"]
            refusal.pop("archive"); refusal.pop("archive_info")
            with patch.dict(os.environ, {"AUDIT_VENDOR_OUTCOME": "success"}), \
                 patch.dict(live.AGENT_BINARIES, {"codex": "/inert/codex"}), \
                 patch("sys.argv", ["audit", "--cli", str(cli), "--target", "codex", "--kind", "plugin", "--limit", "2",
                    "--report-dir", temporary, "--acceptance", BASELINE.PLUGIN_ACCEPTANCE]), \
                 patch.object(live, "version_evidence", side_effect=version), \
                 patch.object(live, "do_case", side_effect=live.HISTORICAL_CODEX["ArchiveUnavailable"](refusal)), \
                 patch.object(live.sys, "platform", "linux"), patch.object(live.platform, "machine", return_value="x86_64"), patch("builtins.print"):
                self.assertEqual(live.main(), 1)
            report = json.loads((Path(temporary) / "codex-plugin.json").read_text())
            self.assertEqual(report["passed"], 0)
            self.assertEqual(report["current_default_refusals_observed"], 2)
            self.assertIs(report["current_default_compatibility_passed"], False)
            self.assertFalse(report["historical_plugin_observation_complete"])
            self.assertTrue(all(row["historical_plugin_feed"] == refusal for row in report["results"]))

    def fixtures(self):
        return json.loads((Path(__file__).resolve().parents[1] / "tests/live-install/fixtures.json").read_text())

    def default(self):
        error = ("agentplugins: load signed Directory: directory snapshot is expired: "
                 "expires_at=2026-10-30T20:03:50Z now=2026-11-21T12:43:09Z\n")
        return {"exit_code": 1, "timeout": False, "stdout_tail": "", "stdout_full": "",
                "stderr_tail": error, "stderr_full": error}

    def info(self, source):
        pin = self.fixtures()["signed_release_pins"][source]
        document = {"schema_version": 1, "command": "info", "result": "success", "data": {
            "snapshot_sequence": 48, "snapshot_digest": "sha256:" + BASELINE.ARCHIVE_DIGEST,
            "selected_distribution": pin["distribution"], "selected_release": {
                key: pin[field] for key, field in (("repository", "repository"), ("package_path", "path"),
                    ("resolved_revision", "revision"), ("release_sequence", "sequence"),
                    ("tree_digest", "tree_digest"), ("manifest_digest", "manifest_digest"))}}}
        return {"exit_code": 0, "timeout": False, "stdout_tail": "bounded public info", "stderr_tail": "",
                "stdout_full": json.dumps(document), "stderr_full": "", "_json": document}

    def observer(self, root):
        from types import SimpleNamespace
        home, data, project = root / "home", root / "uap-data", root / "project"
        for directory in (home, data, project): directory.mkdir()
        return SimpleNamespace(home=home, project=project, launcher=Path("/actual-npm/bin/agentplugins.js"),
            environment={"HOME": str(home), "AGENTPLUGINS_HOME": str(data)})

    def proof(self, source):
        return {"scope": "immutable-official-historical-feed", "origin": BASELINE.ARCHIVE_ORIGIN,
            "ledger_commit": BASELINE.ARCHIVE_COMMIT, "sequence": 48, "key_id": BASELINE.ARCHIVE_KEY,
            "snapshot_sha256": BASELINE.ARCHIVE_DIGEST, "envelope_sha256": BASELINE.ARCHIVE_ENVELOPE_DIGEST,
            "generated_at": BASELINE.ARCHIVE_GENERATED, "expires_at": BASELINE.ARCHIVE_EXPIRES,
            "original_cli_trust_unchanged": True, "release_pin": self.fixtures()["signed_release_pins"][source]}

    def test_default_refusal_is_preserved_and_only_fixed_archive_is_selected(self):
        now = BASELINE.instant("2026-11-21T12:43:09Z")
        for source in BASELINE.ARCHIVE_SOURCES:
            with self.subTest(source=source), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary))
                calls = []
                def call(argv, env, cwd, timeout, *, capture_full=False):
                    calls.append(dict(env))
                    self.assertTrue(capture_full)
                    self.assertEqual(argv, [str(observer.launcher), "info", source, "--target", "codex", "--format", "json"])
                    self.assertEqual(cwd, observer.project)
                    return self.default() if len(calls) == 1 else self.info(source)
                with patch.object(BASELINE, "utc_now", return_value=now), patch.object(
                        BASELINE, "archive_cache", return_value=self.proof(source)):
                    evidence = BASELINE.prepare_plugin_archive(observer, call, source,
                        self.fixtures()["signed_release_pins"][source])
                self.assertNotIn("AGENTPLUGINS_DIRECTORY_ORIGIN", calls[0])
                self.assertEqual(calls[1]["AGENTPLUGINS_DIRECTORY_ORIGIN"], BASELINE.ARCHIVE_ORIGIN)
                self.assertEqual(evidence["current_default"]["observation"], self.default())
                self.assertFalse(evidence["current_default"]["compatibility_passed"])
                self.assertFalse(evidence["current_default"]["native_passed"])
                self.assertEqual(evidence["archive"], self.proof(source))

    def test_unexpected_default_never_reaches_archive_and_failure_retains_raw_observation(self):
        for change in ({"exit_code": 0}, {"timeout": True}, {"stdout_tail": "unexpected"},
                       {"stderr_tail": "network failed"}, {"_json": {"result": "success"}}):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary))
                with patch.object(BASELINE, "utc_now", return_value=BASELINE.instant("2026-11-21T12:43:09Z")):
                    calls = []
                    def call(*args, **kwargs): calls.append(True); return {**self.default(), **change}
                    with self.assertRaises(BASELINE.ArchiveUnavailable) as refused:
                        BASELINE.prepare_plugin_archive(observer, call, "atlassian",
                            self.fixtures()["signed_release_pins"]["atlassian"])
                self.assertEqual(calls, [True])
                self.assertEqual(refused.exception.observation["current_default"]["status"], "unexpected-default-outcome")
                self.assertNotIn("AGENTPLUGINS_DIRECTORY_ORIGIN", observer.environment)
        with tempfile.TemporaryDirectory() as temporary:
            observer = self.observer(Path(temporary))
            observer.environment["AGENTPLUGINS_DIRECTORY_ORIGIN"] = "https://caller.invalid/"
            with self.assertRaises(ValueError):
                BASELINE.prepare_plugin_archive(observer, lambda *a: self.fail("inherited origin executed"),
                    "atlassian", self.fixtures()["signed_release_pins"]["atlassian"])

    def test_wrong_archive_pin_or_refusal_keeps_default_failure_and_never_qualifies(self):
        for mutation in ("process", "sequence", "digest", "revision", "cache"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temporary:
                observer = self.observer(Path(temporary))
                archived = self.info("atlassian")
                if mutation == "process": archived["exit_code"] = 1
                elif mutation in ("sequence", "digest"):
                    archived["_json"]["data"]["snapshot_" + mutation] = 49 if mutation == "sequence" else "sha256:" + "0" * 64
                elif mutation == "revision": archived["_json"]["data"]["selected_release"]["resolved_revision"] = "f" * 40
                results = iter((self.default(), archived))
                with patch.object(BASELINE, "utc_now", return_value=BASELINE.instant("2026-11-21T12:43:09Z")), \
                     patch.object(BASELINE, "archive_cache", side_effect=ValueError("invalid cache")):
                    with self.assertRaises(BASELINE.ArchiveUnavailable) as refused:
                        BASELINE.prepare_plugin_archive(observer, lambda *a, **kwargs: next(results), "atlassian",
                            self.fixtures()["signed_release_pins"]["atlassian"])
                self.assertTrue(BASELINE.recorded_default_refusal(refused.exception.observation["current_default"]))

    def test_exact_cache_bytes_and_real_expiry_are_required(self):
        import base64, hashlib
        # Inert cache-parser controls only. No signing key or vendor execution.
        pin = self.fixtures()["signed_release_pins"]["atlassian"]
        snapshot = {"sequence": 48, "generated_at": BASELINE.ARCHIVE_GENERATED,
            "expires_at": BASELINE.ARCHIVE_EXPIRES, "distributions": [{"id": pin["distribution"],
            "status": "active", "releases": [{"sequence": pin["sequence"],
                "package_source": {"repository": pin["repository"], "path": pin["path"], "revision": pin["revision"]},
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}]}]}
        raw = json.dumps(snapshot).encode(); digest = hashlib.sha256(raw).hexdigest()
        envelope = json.dumps({"sequence": 48, "key_id": BASELINE.ARCHIVE_KEY,
                              "snapshot_digest": "sha256:" + digest}).encode()
        with tempfile.TemporaryDirectory() as temporary:
            observer = self.observer(Path(temporary)); observer.environment["AGENTPLUGINS_DIRECTORY_ORIGIN"] = BASELINE.ARCHIVE_ORIGIN
            cache = Path(observer.environment["AGENTPLUGINS_HOME"]) / "directory-v1-cache.json"
            record = {"schema_version": 1, "sequence": 48, "snapshot": base64.b64encode(raw).decode(),
                      "envelope": base64.b64encode(envelope).decode()}
            cache.write_text(json.dumps(record)); cache.chmod(0o600)
            with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", pin)
            with patch.object(BASELINE, "ARCHIVE_DIGEST", digest), patch.object(BASELINE, "ARCHIVE_ENVELOPE_DIGEST", hashlib.sha256(envelope).hexdigest()):
                with patch.object(BASELINE, "utc_now", return_value=BASELINE.instant("2026-11-21T12:43:09Z")):
                    self.assertEqual(BASELINE.archive_cache(observer, "atlassian", pin)["sequence"], 48)
                    for change in ({"revision": "f" * 40}, {"tree_digest": "sha256:" + "0" * 64}):
                        with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", {**pin, **change})
                    altered = dict(record, sequence=49); cache.write_text(json.dumps(altered))
                    with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", pin)
                    cache.write_text(json.dumps(record)); cache.chmod(0o644)
                    with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", pin)
                    cache.chmod(0o600)
                    cache.write_text('{"sequence":49,' + json.dumps(record)[1:])
                    with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", pin)
                    cache.write_text(json.dumps(record))
                for stamp in ("2026-11-14T12:42:16Z", BASELINE.ARCHIVE_EXPIRES, "2027-01-01T00:00:00Z"):
                    with patch.object(BASELINE, "utc_now", return_value=BASELINE.instant(stamp)):
                        with self.assertRaises(ValueError): BASELINE.archive_cache(observer, "atlassian", pin)

    def report(self):
        rows = []
        for index, source in enumerate(BASELINE.ARCHIVE_SOURCES):
            row = {"index": index, "source": source, "identity": source, "kind": "plugin", "target": "codex",
                   "status": "historical-archive-lifecycle-observed", "historical_archive_status": "passed",
                   "historical_npm_proof": {"package": "universal-agent-plugins", "version": "0.1.95", "commit": BASELINE.COMMIT,
                       "binary_sha256": BASELINE.DIGEST, "binary_size": BASELINE.SIZE,
                       "launcher_sources_verified": True, "commands_verified": 8},
                   "historical_plugin_feed": {"archive": self.proof(source),
                       "archive_info": {**BASELINE.bounded_command(self.info(source)), "complete_output_verified": True},
                       "current_default": {"status": "current-default-compatibility-failed", "origin": BASELINE.DEFAULT_ORIGIN,
                           "compatibility_passed": False, "native_passed": False, "complete_output_verified": True,
                           "started_at": "2026-11-21T12:43:09Z", "ended_at": "2026-11-21T12:43:10Z",
                           "observation": self.default()}},
                   "matched_installation_ids": ["id-" + source], "repeat_installation_ids": ["id-" + source],
                   "native_residuals": [], "native_uninstall": {"completed": True, "native_plugin_absent": True,
                       "native_marketplace_absent": True},
                   "codex_plugin_sqlite_proof": {"native_version": "0.153.4", "runtime_semantics_unchanged": True,
                       "observations": [{"phase": "after-public-command", "runtime_files_fully_read": {}}]}}
            for key in ("repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged", "removal_verified",
                        "sentinels_preserved", "listed_identity", "post_list_schema_valid"):
                row[key] = True
            for key in ("failed_install_registered", "failed_install_residue", "post_remove_identity_registered"):
                row[key] = False
            for phase in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                row[phase] = {"exit_code": 0, "timeout": False}
            rows.append(row)
        return {"acceptance": BASELINE.PLUGIN_ACCEPTANCE, "target": "codex", "kind": "plugin",
                "platform": "linux", "architecture": "x86_64", "cases_attempted": 2, "passed": 0,
                "verified_negative_coverage": 0, "full_coverage": False, "source_selection_complete": False,
                "cli_version": {"status": "reported", "value": "agentplugins 0.1.95", "exit_code": 0},
                "client_version": {"status": "reported", "value": "codex-cli 0.153.4", "exit_code": 0}, "results": rows}

    def test_summary_keeps_current_default_nonpass_and_actual_archive_counts(self):
        self.assertIsNone(BASELINE.plugin_summary({"results": []}, self.fixtures())["current_default_compatibility_passed"])
        report = self.report(); before = copy.deepcopy(report)
        with patch.object(BASELINE, "utc_now", return_value=BASELINE.instant("2026-11-21T12:43:09Z")):
            result = BASELINE.plugin_summary(report, self.fixtures())
        self.assertTrue(result["historical_plugin_observation_complete"])
        self.assertFalse(result["current_default_compatibility_passed"])
        self.assertEqual(result["current_default_native_passed"], 0)
        self.assertEqual(result["archived_plugin_lifecycles_observed"], 2)
        self.assertEqual(report, before)
        for path, bad in (("status", "cleanup-failed"), ("repeat_verified", False), ("removal_verified", False),
                          ("sentinels_preserved", False), ("failed_install_residue", True), ("native_uninstall", None)):
            changed = copy.deepcopy(report); changed["results"][0][path] = bad
            self.assertFalse(BASELINE.plugin_summary(changed, self.fixtures())["historical_plugin_observation_complete"])
        for section, key, bad in (("current_default", "compatibility_passed", True), ("current_default", "observation", {}),
                ("archive", "origin", "https://caller.invalid/"), ("archive", "snapshot_sha256", "0" * 64),
                ("archive", "key_id", "test-key"), ("archive", "expires_at", "2030-01-01T00:00:00Z")):
            changed = copy.deepcopy(report); changed["results"][0]["historical_plugin_feed"][section][key] = bad
            self.assertFalse(BASELINE.plugin_summary(changed, self.fixtures())["historical_plugin_observation_complete"])
        changed = copy.deepcopy(report)
        changed["results"][0]["historical_plugin_feed"]["archive"]["release_pin"]["sequence"] = True
        self.assertFalse(BASELINE.plugin_summary(changed, self.fixtures())["historical_plugin_observation_complete"])



if __name__ == "__main__":
    unittest.main()
