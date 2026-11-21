"""Real filesystem and raw-envelope controls for the explicit installation lane."""
import copy
import hashlib
import json
import os
from pathlib import Path
from io import StringIO
from runpy import run_path
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
from types import SimpleNamespace

from windsurf_installation_audit import (ACTIVE_SLOTS, ACTIVE_SKILLS, CLIENT_VERSION, CLI_VERSION,
    COMPONENT_REFUSALS, PREPARED, WindsurfInstallationAudit, selected, summary)

ROOT = Path(__file__).resolve().parents[1]


@unittest.skipUnless(sys.platform == "linux", "Exact Linux Devin Global installation audit")
class WindsurfInstallationAuditTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.audit = run_path(str(ROOT / "scripts/live-install-audit.py"))
        cls.fixture_bytes = (ROOT / "tests/live-install/fixtures.json").read_bytes()
        cls.fixtures = json.loads(cls.fixture_bytes)
        cls.fixture_hash = hashlib.sha256(cls.fixture_bytes).hexdigest()

    def fixture(self, root, cli="/fixture/agentplugins"):
        case = root / "plugin-case"
        for relative in ("home/.config", "home/.windsurf", "project", "uap-data"):
            (case / relative).mkdir(parents=True)
        env = {"HOME": str(case / "home"), "XDG_CONFIG_HOME": str(case / "home/.config"),
               "AGENTPLUGINS_HOME": str(case / "uap-data")}
        observer = WindsurfInstallationAudit(cli, env, case / "project", case, self.audit)
        return observer, env

    def row(self, index):
        source, identity = selected(self.fixtures, index)
        negative = index == 13 or index in COMPONENT_REFUSALS
        return {"index": index, "source": source, "identity": identity, "target": "windsurf", "kind": "plugin",
                "status": "documented-or-source-unsupported" if index == 13 else
                          "install-failed" if negative else "prepared-lifecycle-passed",
                "sentinels_preserved": True, "failed_install_registered": False,
                "failed_install_residue": False, "post_remove_identity_registered": False,
                "native_residuals": [], "listed_identity": not negative, "native_file_delta": 0,
                "native_identity_files": [], "native_component_candidates": [],
                "matched_installation_ids": [] if negative else ["fixture-installation"],
                "repeat_installation_ids": [] if negative else ["fixture-installation"],
                "repeat_verified": not negative, "removal_verified": not negative,
                "repeat_add": None, "repeat_list": None, "cleanup": None, "post_remove_list": None,
                "post_removal_leftovers": [], "repeat_native_unchanged": True,
                "repeat_lock_unchanged": True, "post_list_schema_valid": True,
                "prepared_components_only": not negative}

    def raw_list(self, index):
        source, identity = selected(self.fixtures, index)
        state = ACTIVE_SKILLS if index in ACTIVE_SLOTS else PREPARED
        client = {**state, "client_id": "windsurf", "scope": "user"}
        node = {"installation_id": "fixture-installation", "name": identity,
                "mixed_version": False, "clients": [client]}
        if source.startswith("github:"):
            node["source"] = source[len("github:"):].rsplit("@", 1)[0]
            client["package_revision"] = {"resolved_revision": source.rsplit("@", 1)[1]}
        else:
            pin = self.fixtures["signed_release_pins"][source]
            node["source"] = pin["repository"] + "/" + pin["path"]
            node["directory"] = {"recorded_distribution": pin["distribution"],
                "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"],
                "product_id": "github" if source == "777genius/github-bridge" else source}
            client["package_revision"] = {"distribution_id": pin["distribution"],
                "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}
        return {"schema_version": 1, "command": "list", "result": "success",
                "data": {"installations": [node]}}

    def outcome(self, raw=None, *, code=0, stderr=""):
        return {"exit_code": code, "timeout": False, "_json": raw,
                "stdout_tail": "" if raw is None else json.dumps(raw)[-1800:],
                "stderr_tail": stderr[-1800:], "stderr_full": stderr}

    def sequence(self, observer, row):
        index, source, identity = row["index"], row["source"], row["identity"]
        scope = ["--target", "windsurf", "--scope", "user", "--format", "json"]
        add = [observer.cli, "add", source, *scope]
        listed = [observer.cli, "list", *scope]
        if index == 13 or index in COMPONENT_REFUSALS:
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing windsurf\n'
                          if index == 13 else f"agentplugins: preflight {identity}/windsurf: "
                          "devin Global supports only selected Skills and MCP components\n")
            return [(add, self.outcome(code=1, stderr=diagnostic)),
                    (listed, self.outcome({"schema_version": 1, "command": "list", "result": "success",
                                          "data": {"installations": []}}))]
        def mutation(operation, action, initial=False):
            entry = {"action": action, "alias": identity, "client_id": "windsurf",
                     "installation_id": "fixture-installation"}
            if initial:
                entry["component_support"] = [{"kind": "skill" if index in ACTIVE_SLOTS else "mcp_server",
                                                "name": identity, "support": "prepared"}]
            result = {"operation": "install" if operation == "add" else "remove",
                      "client_id": "windsurf", "installation_id": "fixture-installation", "outcome": "completed"}
            return self.outcome({"schema_version": 1, "command": "agentplugins " + operation,
                "scope": "user", "scope_selection_reason": "explicit", "result": "success", "outcome": "completed",
                "actions": [entry], "results": [] if action == "unchanged" else [result]})
        return [(add, mutation("add", "install", True)), (listed, self.outcome(self.raw_list(index))),
                (add, mutation("add", "unchanged")), (listed, self.outcome(self.raw_list(index))),
                ([observer.cli, "remove", identity, *scope], mutation("remove", "remove")),
                (listed, self.outcome({"schema_version": 1, "command": "list", "result": "success",
                                      "data": {"installations": []}}))]

    def finish(self, observer, row, sequence=None):
        for argv, outcome in sequence or self.sequence(observer, row):
            observer.observe(argv, outcome)
        return observer.finish(row, row["source"], row["index"], self.fixtures)

    def test_all_exact_slots_preserve_prepared_active_and_five_negatives(self):
        proofs = []
        for index in range(30):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                proof = self.finish(observer, row)
                row["windsurf_installation_audit"] = proof
                proofs.append(row)
                self.assertFalse(proof["native_runtime_qualified"])
                self.assertNotIn("_json", json.dumps(proof))
                self.assertNotIn("unmanaged Devin", json.dumps(proof))
                if index in ACTIVE_SLOTS:
                    self.assertEqual(proof["public_state"], ACTIVE_SKILLS)
        report = self.report(proofs)
        result = summary(report, self.fixtures, fixture_sha256=self.fixture_hash)
        self.assertTrue(result["installation_acceptance_passed"])
        self.assertEqual((result["installation_passed"], result["installation_verified_negative_coverage"],
                          result["installation_prepared_lifecycle_passed"], result["installation_active_skill_lifecycle_passed"]),
                         (25, 5, 21, 4))
        self.assertFalse(any(result[k] for k in ("native_runtime_qualified", "authentication_qualified",
                                               "model_qualified", "mcp_tools_qualified")))
        self.assertEqual(report["passed"], 0)

    def report(self, rows):
        return {"target": "windsurf", "kind": "plugin", "platform": "linux", "architecture": "x86_64",
                "cli": "/fixture/agentplugins", "client_binary": "/fixture/vendor/bin/devin",
                "cli_version": {"status": "reported", "value": CLI_VERSION, "exit_code": 0},
                "client_version": {"status": "reported", "value": CLIENT_VERSION, "exit_code": 0},
                "fixture_sha256": self.fixture_hash, "fixture_count": 30, "cases_attempted": 30,
                "full_coverage": True, "source_selection_complete": True, "passed": 0, "results": rows}

    def test_old_report_without_actual_global_custody_never_qualifies(self):
        self.assertFalse(summary(self.report([self.row(i) for i in range(30)]), self.fixtures,
                                 fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def test_actual_global_and_legacy_roots_are_observed(self):
        with TemporaryDirectory() as temporary:
            observer, env = self.fixture(Path(temporary).resolve())
            self.assertEqual(observer.global_root, Path(env["XDG_CONFIG_HOME"]) / "devin")
            self.assertEqual(observer.native_roots, (Path(env["HOME"]) / ".windsurf",
                Path(env["HOME"]) / ".devin", Path(env["XDG_CONFIG_HOME"]) / "devin"))
            row = self.row(0)
            sequence = self.sequence(observer, row)
            owned = observer.global_root / "owned-fixture.json"
            owned.write_text('{"name":"atlassian"}')
            for argv, outcome in sequence[:4]:
                observer.observe(argv, outcome)
            owned.unlink()
            for argv, outcome in sequence[4:]:
                observer.observe(argv, outcome)
            proof = observer.finish(row, row["source"], 0, self.fixtures)
            self.assertNotEqual(proof["initial_native_image"], proof["observations"][0]["native_image"])
            self.assertEqual(proof["initial_native_image"], proof["observations"][-1]["native_image"])

    def test_profile_environment_aliases_and_occupied_sentinel_refused(self):
        for mutation in ("HOME", "XDG_CONFIG_HOME", "AGENTPLUGINS_HOME", "global-link", "occupied"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                observer, env = self.fixture(root)
                if mutation == "global-link":
                    saved = observer.global_root.with_name("saved")
                    observer.global_root.rename(saved)
                    observer.global_root.symlink_to(saved, target_is_directory=True)
                elif mutation != "occupied":
                    env[mutation] += "-wrong"
                with self.assertRaises((ValueError, FileExistsError)):
                    WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)

    def test_original_global_foreign_bytes_and_root_identity_are_pinned(self):
        for mutation in ("foreign", "global", "home", "project", "xdg", "data", "case"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                if mutation == "foreign":
                    observer.foreign.write_text("changed foreign")
                else:
                    path = getattr(observer, "global_root" if mutation == "global" else mutation)
                    saved = path.with_name(path.name + "-old")
                    path.rename(saved)
                    path.mkdir()
                with self.assertRaises((ValueError, AssertionError, FileNotFoundError)):
                    observer.observe(*self.sequence(observer, self.row(0))[0])

    def test_links_special_files_and_project_writes_refuse(self):
        for mutation in ("symlink", "fifo", "project"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                if mutation == "symlink":
                    (observer.global_root / "redirect").symlink_to(observer.foreign)
                elif mutation == "fifo":
                    os.mkfifo(observer.global_root / "pipe")
                else:
                    (observer.project / "unexpected").write_text("changed")
                with self.assertRaises((ValueError, AssertionError)):
                    observer.observe(*self.sequence(observer, self.row(0))[0])

    def test_raw_envelope_missing_wrong_scope_state_source_or_ids_refuse(self):
        for mutation in ("missing-json", "tail-only", "wrong-scope", "extra-client", "extra-row", "wrong-source",
                         "wrong-sha", "wrong-digest", "policy", "authentication", "verification", "activation",
                         "rekey", "remove-id", "remove-outcome", "add-result", "result-type", "implicit-scope", "nan", "large", "timeout", "exit", "stderr"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                outcome = sequence[1][1]
                node = outcome["_json"]["data"]["installations"][0]
                client = node["clients"][0]
                if mutation in ("missing-json", "tail-only"):
                    outcome.pop("_json")
                elif mutation == "wrong-scope": client["scope"] = "project"
                elif mutation == "extra-client": node["clients"].append(copy.deepcopy(client))
                elif mutation == "extra-row": outcome["_json"]["data"]["installations"].append(copy.deepcopy(node))
                elif mutation == "wrong-source": node["source"] += "-other"
                elif mutation == "wrong-sha": client["package_revision"]["resolved_revision"] = "0" * 40
                elif mutation == "wrong-digest": client["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
                elif mutation in ("policy", "authentication", "verification", "activation"): client[mutation] = "invalid"
                elif mutation == "rekey": sequence[3][1]["_json"]["data"]["installations"][0]["installation_id"] = "new"
                elif mutation == "remove-id": sequence[4][1]["_json"]["results"][0]["installation_id"] = "foreign"
                elif mutation == "remove-outcome": sequence[4][1]["_json"]["outcome"] = "failed"
                elif mutation == "add-result": sequence[0][1]["_json"]["results"] = []
                elif mutation == "result-type": sequence[4][1]["_json"]["results"] = [42]
                elif mutation == "implicit-scope": sequence[0][1]["_json"]["scope_selection_reason"] = "default"
                elif mutation == "nan": outcome["_json"]["extra"] = float("nan")
                elif mutation == "large": outcome["_json"]["extra"] = "x" * (1 << 20)
                elif mutation == "timeout": outcome["timeout"] = True
                elif mutation == "exit": outcome["exit_code"] = 1
                elif mutation == "stderr": outcome["stderr_full"] = "hidden warning"
                with self.assertRaises((ValueError, AssertionError)):
                    self.finish(observer, row, sequence)

    def test_exact_actual_argv_and_command_order_required(self):
        for mutation in ("scope", "other-cli", "extra", "missing", "reorder"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                if mutation == "scope": sequence[0][0][sequence[0][0].index("user")] = "project"
                elif mutation == "other-cli": sequence[0][0][0] = "/other/cli"
                elif mutation == "extra": sequence[0][0].append("--external-uninstalled")
                elif mutation == "missing": sequence.pop()
                else: sequence[0], sequence[1] = sequence[1], sequence[0]
                with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def test_negative_exact_complete_stderr_empty_inventory_and_all_images(self):
        for index in (13, 23, 24, 27, 28):
            for mutation in ("hidden-banner", "substring", "timeout", "exit", "registered", "global-write", "legacy-write"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve())
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    if mutation == "hidden-banner": sequence[0][1]["stderr_full"] = "x" * 3000 + sequence[0][1]["stderr_tail"]
                    elif mutation == "substring": sequence[0][1]["stderr_full"] += "unexpected\n"
                    elif mutation == "timeout": sequence[0][1]["timeout"] = True
                    elif mutation == "exit": sequence[0][1]["exit_code"] = 2
                    elif mutation == "registered": sequence[1][1]["_json"]["data"]["installations"] = [{}]
                    else:
                        path = observer.global_root if mutation == "global-write" else observer.home / ".windsurf"
                        (path / "unexpected").write_text("native mutation")
                    with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def test_repeat_and_late_native_mutations_refuse(self):
        for when in ("repeat", "late"):
            with self.subTest(when=when), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                for position, (argv, outcome) in enumerate(sequence):
                    if when == "repeat" and position == 2:
                        (observer.global_root / "unexpected").write_text("changed during repeat")
                    observer.observe(argv, outcome)
                if when == "late":
                    (observer.global_root / "late").write_text("post-observation write")
                with self.assertRaises(ValueError): observer.finish(row, row["source"], 0, self.fixtures)

    def test_unattributed_global_leftover_cannot_hide_behind_empty_identity_scanner(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            sequence = self.sequence(observer, row)
            for position, (argv, outcome) in enumerate(sequence):
                if position == 4:
                    (observer.global_root / "unrelated-new-file").write_text("must not survive cleanup")
                observer.observe(argv, outcome)
            self.assertEqual(row["native_residuals"], [])
            self.assertEqual(observer.observations[4][2], observer.observations[5][2])
            with self.assertRaises(ValueError): observer.finish(row, row["source"], 0, self.fixtures)

    def test_lifecycle_flags_never_override_failed_observations(self):
        for key, value in (("repeat_verified", False), ("removal_verified", False), ("sentinels_preserved", False),
                           ("failed_install_residue", True), ("native_residuals", ["unowned"]),
                           ("post_list_schema_valid", False), ("prepared_components_only", False)):
            with self.subTest(key=key), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                row[key] = value
                with self.assertRaises(ValueError): self.finish(observer, row)

    def completed_report(self, cli="/fixture/agentplugins"):
        rows = []
        for index in range(30):
            with TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve(), cli)
                row = self.row(index)
                row["windsurf_installation_audit"] = self.finish(observer, row)
                rows.append(row)
        return self.report(rows)

    def test_summary_refuses_tampered_row_custody_scope_and_counts(self):
        original = self.completed_report()
        self.assertTrue(summary(original, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        for mutation in ("missing-proof", "source", "index-bool", "residue", "sentinel", "repeat", "global-omitted",
                         "image-count", "negative-image", "state-promotion", "writer-claim", "model-native-count",
                         "missing-row", "duplicate-row", "cli-bool"):
            with self.subTest(mutation=mutation):
                report = copy.deepcopy(original)
                row = report["results"][0]
                proof = row["windsurf_installation_audit"]
                if mutation == "missing-proof": del row["windsurf_installation_audit"]
                elif mutation == "source": row["source"] = "other-source"
                elif mutation == "index-bool": row["index"] = False
                elif mutation == "residue": row["native_residuals"] = ["unowned"]
                elif mutation == "sentinel": row["sentinels_preserved"] = False
                elif mutation == "repeat": row["repeat_verified"] = False
                elif mutation == "global-omitted": proof["native_roots"].pop()
                elif mutation == "image-count": proof["observations"][0]["native_image"]["entries"] = False
                elif mutation == "negative-image":
                    report["results"][13]["windsurf_installation_audit"]["observations"][0]["native_image"]["sha256"] = "0" * 64
                elif mutation == "state-promotion": proof["public_state"] = dict(ACTIVE_SKILLS)
                elif mutation == "writer-claim": proof["writer_closure_claimed"] = True
                elif mutation == "model-native-count": report["passed"] = 25
                elif mutation == "missing-row": report["results"].pop()
                elif mutation == "duplicate-row": report["results"][1] = copy.deepcopy(row)
                elif mutation == "cli-bool": report["cli_version"]["exit_code"] = False
                self.assertFalse(summary(report, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])

    def test_full_parsed_observations_survive_caller_consumption_without_output_export(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            for argv, outcome in self.sequence(observer, row):
                observer.observe(argv, outcome)
                outcome.pop("_json")
                outcome.pop("stderr_full")
            proof = observer.finish(row, row["source"], 0, self.fixtures)
            self.assertEqual(proof["status"], "verified-installation-lifecycle")
            self.assertNotIn("installations", json.dumps(proof))

    def test_observed_active_and_prepared_tuples_cannot_substitute(self):
        for index in (0, *ACTIVE_SLOTS):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                sequence = self.sequence(observer, row)
                for position in (1, 3):
                    client = sequence[position][1]["_json"]["data"]["installations"][0]["clients"][0]
                    client.update(PREPARED if index in ACTIVE_SLOTS else ACTIVE_SKILLS)
                with self.assertRaises(ValueError): self.finish(observer, row, sequence)

    def test_real_do_case_observes_wrapped_envelopes_before_consuming_them(self):
        # Only public subprocess responses are substituted. Run the real case
        # setup, native roots, source matcher, repeat/removal and final observer.
        cli = str(Path(sys.executable).resolve())
        for index, count in ((0, 6), (22, 6), (13, 2), (23, 2)):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                expected_row = self.row(index)
                sequence = self.sequence(SimpleNamespace(cli=cli), expected_row)
                if count == 6:
                    # Actual add/repeat output uses the public outer envelope;
                    # bare mutation envelopes cannot drive prepared_only_add.
                    for position in (0, 2):
                        outcome = sequence[position][1]
                        outcome["_json"] = {"schema_version": 1, "command": "add", "result": "success",
                                            "data": outcome["_json"]}
                        outcome["stdout_tail"] = json.dumps(outcome["_json"])[-1800:]
                calls = []
                def public_call(argv, env, project, *args, **kwargs):
                    expected, outcome = sequence[len(calls)]
                    self.assertEqual(argv, expected)
                    self.assertIs(kwargs.get("capture_full"), True)
                    self.assertTrue((Path(env["XDG_CONFIG_HOME"]) / "devin/foreign-neighbor.txt").is_file())
                    self.assertEqual(project, Path(temporary).resolve() / f"plugin-{index:02d}/project")
                    calls.append(list(argv))
                    return copy.deepcopy(outcome)
                with patch.dict(self.audit["do_case"].__globals__, {"call": public_call}):
                    row = self.audit["do_case"](cli, "windsurf", "plugin", expected_row["source"], index,
                        Path(temporary).resolve(), self.fixtures["plugin_declared_names"],
                        self.fixtures["signed_release_pins"], installation_audit="windsurf-global-installation",
                        installation_fixtures=self.fixtures)
                self.assertEqual(len(calls), count)
                self.assertEqual(row["status"], expected_row["status"])
                proof = row["windsurf_installation_audit"]
                self.assertEqual(proof["public_state"], None if count == 2 else ACTIVE_SKILLS if index == 22 else PREPARED)
                self.assertFalse(proof["native_runtime_qualified"])
                for operation in ("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list"):
                    if row[operation] is not None:
                        self.assertTrue({"_json", "stdout_full", "stderr_full"}.isdisjoint(row[operation]))

    def test_real_do_case_rejects_wrong_route_before_any_public_call(self):
        with TemporaryDirectory() as temporary, patch.dict(self.audit["do_case"].__globals__, {
                "call": lambda *args, **kwargs: self.fail("wrong route executed a public command")}):
            with self.assertRaisesRegex(ValueError, "does not match"):
                self.audit["do_case"](str(Path(sys.executable).resolve()), "cursor", "plugin",
                    self.fixtures["plugin_aliases"][0], 0, Path(temporary).resolve(),
                    self.fixtures["plugin_declared_names"], self.fixtures["signed_release_pins"],
                    installation_audit="windsurf-global-installation", installation_fixtures=self.fixtures)

    def test_real_main_installation_exit_keeps_native_pass_count_zero(self):
        cli = str(Path(sys.executable).resolve())
        rows = self.completed_report(cli)["results"]
        for missing_proof, expected_exit in ((False, 0), (True, 1)):
            with self.subTest(missing_proof=missing_proof), TemporaryDirectory() as temporary:
                selected_rows = copy.deepcopy(rows)
                if missing_proof:
                    selected_rows[0].pop("windsurf_installation_audit")
                def do_case(*args, **kwargs):
                    self.assertEqual(kwargs.get("installation_audit"), "windsurf-global-installation")
                    self.assertEqual(kwargs.get("installation_fixtures"), self.fixtures)
                    self.assertEqual(args[3], self.fixtures["plugin_aliases"][args[4]])
                    return copy.deepcopy(selected_rows[args[4]])
                def version(argv, *args):
                    return {"status": "reported", "value": CLI_VERSION if argv[1] == "version" else CLIENT_VERSION,
                            "exit_code": 0}
                output = Path(temporary) / "report"
                with patch.dict(self.audit["main"].__globals__, {
                        "do_case": do_case, "vendor_bootstrap_attested": lambda: True,
                        "AGENT_BINARIES": {"devin": "/fixture/bin/devin"}, "version_evidence": version}), \
                        patch.object(sys, "platform", "linux"), \
                        patch.object(self.audit["platform"], "machine", return_value="x86_64"), \
                        patch.object(sys, "stdout", StringIO()), \
                        patch.object(sys, "argv", ["audit", "--cli", cli, "--kind", "plugin", "--target", "windsurf",
                            "--limit", "30", "--acceptance", "windsurf-global-installation", "--report-dir", str(output)]):
                    code = self.audit["main"]()
                report = json.loads((output / "windsurf-plugin.json").read_text())
                self.assertEqual(code, expected_exit)
                self.assertEqual(report["passed"], 0)
                self.assertEqual(report["verified_negative_coverage"], 0)
                self.assertEqual(report["cases_attempted"], 30)
                self.assertIs(report["installation_acceptance_passed"], not missing_proof)
                self.assertFalse(any(report[key] for key in ("native_runtime_qualified", "authentication_qualified",
                                                            "model_qualified", "mcp_tools_qualified")))
                if not missing_proof:
                    self.assertEqual((report["installation_passed"], report["installation_verified_negative_coverage"]), (25, 5))

    def test_summary_requires_exact_versions_complete_selection_and_new_proof(self):
        # Missing proof is refused independently of all old success booleans.
        base = self.report([self.row(i) for i in range(30)])
        for key, value in (("platform", "darwin"), ("architecture", "arm64"), ("target", "cursor"),
                           ("kind", "skill"), ("cases_attempted", 29), ("full_coverage", False),
                           ("fixture_sha256", "0" * 64), ("passed", 25),
                           ("client_binary", "/fixture/windsurf"),
                           ("client_version", {"status": "reported", "value": "devin 3000.12.58", "exit_code": 0})):
            with self.subTest(key=key):
                report = copy.deepcopy(base)
                report[key] = value
                self.assertFalse(summary(report, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])


if __name__ == "__main__":
    unittest.main()
