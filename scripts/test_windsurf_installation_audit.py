"""Real filesystem and raw-envelope controls for the explicit installation lane."""
import copy
import hashlib
import json
import os
import shutil
from pathlib import Path
from io import StringIO
from runpy import run_path
import stat
import sys
from tempfile import TemporaryDirectory
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import windsurf_installation_audit as installation

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

    def fixture(self, root, cli="/fixture/agentplugins", *, saved=None):
        case = root / "plugin-case"
        for relative in ("home/.config", "home/.windsurf", "project", "uap-data"):
            (case / relative).mkdir(parents=True)
        for name, body in (saved or {}).items():
            (case / "uap-data" / name).write_bytes(body)
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
            client["package_revision"] = {"resolved_revision": source.rsplit("@", 1)[1],
                "tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "b" * 64}
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
            revision = self.raw_list(index)["data"]["installations"][0]["clients"][0]["package_revision"]
            if operation == "add":
                entry.update(package_digest=revision["tree_digest"], revision=revision["resolved_revision"])
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

    def author_saved(self, observer, row, position):
        """Independent inert producer: saved intent, binding and managed receipt."""
        if row["index"] == 13 or row["index"] in COMPONENT_REFUSALS:
            return
        data = observer.data
        if position == 0:
            from test_windsurf_global_skill_audit import source_digest
            node = self.raw_list(row["index"])["data"]["installations"][0]
            client = node["clients"][0]
            identity, source = row["identity"], row["source"]
            physical = identity[:51].rstrip(".-") + "-" + hashlib.sha256(node["installation_id"].encode()).hexdigest()[:12]
            active = data / "managed/clients/windsurf" / physical
            active.mkdir(parents=True)
            files = {"plugin.json": json.dumps({"name": identity, "version": "1.0.0"}).encode(),
                     "mcp.json": b'{"mcpServers":{}}'}
            for name, body in files.items(): (active / name).write_bytes(body)
            binding_id = "client_" + hashlib.sha256("\0".join((node["installation_id"], "windsurf", "user", str(active))).encode()).hexdigest()[:24]
            revision = client["package_revision"]
            binding = {**copy.deepcopy(client), "client_binding_id": binding_id,
                "physical_artifact_id": physical, "target_locator": str(active),
                "native_profile_root": str(observer.global_root),
                "native_objects": [{"kind": "managed_package_directory", "object_id": "package:windsurf:" + physical,
                    "logical_name": identity, "path": str(active), "managed_digest": source_digest(files), "protection_class": "managed"}]}
            if source.startswith("github:"):
                repo, commit = source[7:].rsplit("@", 1); repo = repo.lower(); subpath = ""
                dep = f'git = "https://github.com/{repo}.git"\ncommit = "{commit}"\n'
                locked_source = {"git": {"repository": repo, "commit": commit, "requested": {"kind": "commit", "value": commit}}}
            else:
                release = self.fixtures["signed_release_pins"][source]
                repo, subpath, commit = release["repository"], release["path"], release["revision"]
                dep = f'registry = "uap"\ndistribution = "{release["distribution"]}"\nrelease = "{release["sequence"]}"\n'
                locked_source = {"registry": {"distribution_id": release["distribution"],
                    "release_sequence": release["sequence"], "repository": repo, "path": subpath, "commit": commit}}
            source_key = "github:" + repo + ":" if source.startswith("github:") else "directory:" + release["distribution"].lower()
            selector = {"kind": "commit", "value": commit} if source.startswith("github:") else {"kind": "release", "value": str(release["sequence"])}
            intent = {"schema_version": 1, "dependencies": [{"alias": identity, "source_key": source_key,
                "selector": selector}], "targets": [{"id": "windsurf", "aliases": [identity]}]}
            manifest_digest = "sha256:" + hashlib.sha256(json.dumps(intent, separators=(",", ":")).encode()).hexdigest()
            state = {"schema_version": 5, "installations": [{"installation_id": node["installation_id"], "declared_name": identity,
                "source": {"repository": repo, "package_subpath": subpath, "resolved_revision": commit, "tree_digest": revision["tree_digest"]},
                "package": {"manifest_digest": revision["manifest_digest"]}, "clients": {binding_id: binding}}]}
            (data / "state-v2.json").write_text(json.dumps(state))
            (data / "uap.toml").write_text(f'schema_version = 1\n[dependencies.{json.dumps(identity)}]\n' + dep
                + f'[targets]\nwindsurf = [{json.dumps(identity)}]\n')
            (data / "uap.lock").write_text(json.dumps({"lock_version": 1, "manifest_digest": manifest_digest, "packages": [{"alias": identity, "source_key": source_key,
                "source": locked_source, "targets": ["windsurf"], "digest_algorithm": "agentplugins-tree-sha256-v1",
                **{key: revision[key] for key in ("tree_digest", "manifest_digest")}}]}))
        elif position == 4:
            state = json.loads((data / "state-v2.json").read_text())
            for record in state["installations"]:
                for binding in record["clients"].values(): shutil.rmtree(binding["target_locator"])
            (data / "state-v2.json").write_text('{"schema_version":5,"installations":[]}')
            (data / "uap.toml").write_text('schema_version = 1\n[dependencies]\n[targets]\n')
            (data / "uap.lock").write_text('{"lock_version":1,"packages":[]}')

    def authored_sequence(self, observer, row, sequence):
        for position, entry in enumerate(sequence):
            self.author_saved(observer, row, position)
            yield entry

    def finish(self, observer, row, sequence=None):
        for argv, outcome in self.authored_sequence(observer, row, sequence or self.sequence(observer, row)):
            observer.observe(argv, outcome)
        return observer.finish(row, row["source"], row["index"], self.fixtures)

    def native_lifecycle(self, mutation=None, *, transport_mode="good", constructor_stage="fresh"):
        # Reuse the existing transport fixture, never its verdict: all native
        # source/receipt/inventory/custody methods below are the real verifier.
        import test_windsurf_global_skill_audit as native_fixture
        live = native_fixture.audit
        with TemporaryDirectory(prefix="windsurf-installation-unit-") as temporary:
            root = Path(temporary).resolve()
            transport = native_fixture.InertPublicAndNative(root, transport_mode)
            cls = live.WINDSURF_GLOBAL_AUDIT["WindsurfGlobalSkillAudit"]
            globals_copy = {**vars(live), "WINDSURF_GLOBAL_AUDIT": {
                **live.WINDSURF_GLOBAL_AUDIT, "WindsurfGlobalSkillAudit": cls}}
            holder = {}

            def real_factory(case, home, project, item, executable, version):
                runtime = cls(case, home, project, item, executable, version)
                transport.verifier = runtime
                try:
                    if constructor_stage == "after-probe-start":
                        runtime.begin_probe("before")
                    env = {"HOME": str(home), "XDG_CONFIG_HOME": str(home / ".config"),
                           "AGENTPLUGINS_HOME": str(case / "uap-data")}
                    before = native_fixture.proof.frame(runtime.dirs[runtime.home][0])
                    observer = WindsurfInstallationAudit("/fixture/agentplugins", env, project, case,
                                                         globals_copy, native_runtime=runtime)
                    self.assertEqual(before, native_fixture.proof.frame(runtime.dirs[runtime.home][0]))
                    self.assertFalse((runtime.profile / "foreign-neighbor.txt").exists())
                    holder["observer"] = observer
                    transition = runtime.after_public_transition
                    def observe_transition(phase, raw, outcome, argv):
                        transition(phase, raw, outcome, argv)
                        observer.observe(argv, outcome)
                    runtime.after_public_transition = observe_transition
                    return runtime
                except BaseException:
                    runtime.close()
                    raise

            def public_call(argv, *args, **kwargs):
                result = transport.call(argv, *args, **kwargs)
                if argv[1] == "list":
                    holder["observer"].observe(argv, result)
                return result

            original_case = live._do_case
            def completed_case(*args, **kwargs):
                row = original_case(*args, **kwargs)
                if mutation is not None:
                    mutation(row, transport)
                prior = copy.deepcopy(row)
                row["windsurf_installation_audit"] = holder["observer"].finish(
                    row, native_fixture.ITEM, 26, self.fixtures)
                self.assertEqual({k: v for k, v in row.items() if k != "windsurf_installation_audit"}, prior)
                self.assertEqual(row["windsurf_installation_audit"], holder["observer"].finish(
                    row, native_fixture.ITEM, 26, self.fixtures))
                return row

            pins = {**native_fixture.proof.CASES, native_fixture.ITEM: transport.expected_pin}
            with patch.dict(cls.__init__.__globals__, {
                    "BINARY_SHA": native_fixture.hashed(transport.binary.read_bytes()), "CASES": pins}),                     patch.object(installation, "GLOBAL_SKILL_CASES", pins),                     patch.dict(live.WINDSURF_GLOBAL_AUDIT, {"WindsurfGlobalSkillAudit": real_factory}),                     patch.dict(live.AGENT_BINARIES, {"devin": str(transport.binary)}, clear=True),                     patch.object(live, "call", side_effect=public_call),                     patch.dict(live.NATIVE_RESEARCH, {"run_probe": transport.run_probe}),                     patch.object(live, "_do_case", side_effect=completed_case):
                result = live.do_case("/fixture/agentplugins", "windsurf", "plugin", native_fixture.ITEM, 26,
                    root / "cases", {native_fixture.ITEM: "elements-of-style"}, client_version={
                        "status": "reported", "value": CLIENT_VERSION, "exit_code": 0})
            self.assertTrue(transport.verifier.closed)
            return result, pins

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
        self.assertFalse(result["retained_native_acceptance_passed"])

        native_row, pins = self.native_lifecycle()
        self.assertEqual(native_row["status"], "passed")  # Inert proof, not a native run.
        self.assertEqual(native_row["windsurf_installation_audit"]["observations"][4]["argv"][2],
                         native_row["matched_installation_ids"][0])
        self.assertFalse(native_row["windsurf_installation_audit"]["native_runtime_qualified"])
        combined = copy.deepcopy(report)
        combined["results"][26] = native_row
        combined["passed"] = 1
        combined["client_binary"] = native_row["windsurf_global_native_proof"]["binary"]["path"]
        with patch.object(installation, "GLOBAL_SKILL_CASES", pins):
            joined = summary(combined, self.fixtures, fixture_sha256=self.fixture_hash)
            self.assertTrue(joined["installation_acceptance_passed"])
            self.assertFalse(joined["retained_native_acceptance_passed"])
            self.assertFalse(joined["native_runtime_qualified"])
            for corruption in ("lost-proof", "flag-only", "phase", "source-pin", "uuid", "digest", "count"):
                with self.subTest(summary_corruption=corruption):
                    bad = copy.deepcopy(combined)
                    row = bad["results"][26]
                    if corruption == "lost-proof": row.pop("windsurf_global_native_proof")
                    elif corruption == "flag-only": row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    elif corruption == "phase": row["windsurf_global_native_proof"]["observations"].pop()
                    elif corruption == "source-pin": row["windsurf_global_native_proof"]["independent_source_pin"]["tree_digest"] = "sha256:" + "a" * 64
                    elif corruption == "uuid": row["matched_installation_ids"] = ["39b5780a-ea06-41f3-9b42-41918272b261"]
                    elif corruption == "digest": row["windsurf_installation_audit"]["preserved_native_proof"]["sha256"] = "a" * 64
                    else: bad["passed"] = 0
                    verdict = summary(bad, self.fixtures, fixture_sha256=self.fixture_hash)
                    self.assertFalse(verdict["installation_acceptance_passed"])
                    self.assertFalse(verdict["retained_native_acceptance_passed"])

        for corruption in ("lost-proof", "flag-only", "phase", "source-pin", "uuid", "forged-stream", "stale-finished", "foreign-change"):
            with self.subTest(live_corruption=corruption), self.assertRaises(ValueError):
                def corrupt(row, transport):
                    if corruption == "lost-proof": row.pop("windsurf_global_native_proof")
                    elif corruption == "flag-only": row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    elif corruption == "phase": row["windsurf_global_native_proof"]["observations"].pop()
                    elif corruption == "source-pin": row["windsurf_global_native_proof"]["independent_source_pin"]["tree_digest"] = "sha256:" + "a" * 64
                    elif corruption == "uuid": row["matched_installation_ids"] = ["39b5780a-ea06-41f3-9b42-41918272b261"]
                    elif corruption == "forged-stream": row["windsurf_global_native_proof"]["observations"][1]["stdout"]["base64"] = "e30="
                    elif corruption == "stale-finished": transport.verifier._failure("inert late custody loss")
                    else: (transport.home / "foreign-untouched.txt").write_bytes(b"changed foreign")
                self.native_lifecycle(corrupt)
        with self.assertRaises(ValueError):
            self.native_lifecycle(constructor_stage="after-probe-start")
        for index in (0, 26):
            with self.subTest(unwitnessed_passed=index), TemporaryDirectory() as temporary, self.assertRaises(ValueError):
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(index)
                row["status"] = "passed"
                row["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                self.finish(observer, row)
        failed_native, _ = self.native_lifecycle(transport_mode="native-missing")
        self.assertEqual(failed_native["status"], "prepared-lifecycle-passed")
        self.assertNotIn("preserved_native_proof", failed_native["windsurf_installation_audit"])
        failed_report = copy.deepcopy(report)
        failed_report["results"][26] = failed_native
        with patch.object(installation, "GLOBAL_SKILL_CASES", pins):
            verdict = summary(failed_report, self.fixtures, fixture_sha256=self.fixture_hash)
        self.assertTrue(verdict["installation_acceptance_passed"])
        self.assertFalse(verdict["retained_native_acceptance_passed"])

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
            self.author_saved(observer, row, 0)
            for argv, outcome in sequence[:4]:
                observer.observe(argv, outcome)
            owned.unlink()
            self.author_saved(observer, row, 4)
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

    def test_fresh_internal_profile_symlink_and_raced_substitution_never_redirect_sentinel(self):
        for raced in (False, True):
            with self.subTest(raced=raced), TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                observer, env = self.fixture(root)
                # Fresh construction, with no occupied sentinel to mask the
                # pre-write directory refusal seen in the original regression.
                shutil.rmtree(observer.global_root)
                sibling = observer.xdg / "sibling"
                sibling.mkdir()
                original_open = os.open
                if not raced:
                    observer.global_root.symlink_to(sibling, target_is_directory=True)
                else:
                    observer.global_root.mkdir()
                def substitute(path, flags, *args, **kwargs):
                    if raced and path == "foreign-neighbor.txt":
                        observer.global_root.rename(observer.xdg / "original")
                        observer.global_root.symlink_to(sibling, target_is_directory=True)
                    return original_open(path, flags, *args, **kwargs)
                with patch.object(installation.os, "open", side_effect=substitute), self.assertRaises(ValueError):
                    WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)
                self.assertEqual(list(sibling.iterdir()), [])

    def test_profile_directory_creation_uses_verified_xdg_descriptor_during_substitution(self):
        with TemporaryDirectory() as temporary:
            observer, env = self.fixture(Path(temporary).resolve())
            shutil.rmtree(observer.global_root)
            sibling = observer.home / "unmanaged-sibling"
            sibling.mkdir()
            original_mkdir = os.mkdir
            def substitute(path, mode=0o777, *, dir_fd=None):
                if path == "devin":
                    self.assertIsNotNone(dir_fd)
                    observer.xdg.rename(observer.home / "original-config")
                    observer.xdg.symlink_to(sibling, target_is_directory=True)
                return original_mkdir(path, mode=mode, dir_fd=dir_fd)
            with patch.object(installation.os, "mkdir", side_effect=substitute), self.assertRaises(ValueError):
                WindsurfInstallationAudit(observer.cli, env, observer.project, observer.case, self.audit)
            self.assertEqual(list(sibling.iterdir()), [])
            self.assertFalse((sibling / "devin").exists())

    def test_exact_negatives_refuse_saved_intent_and_hidden_binding_residue(self):
        for index in (13, *COMPONENT_REFUSALS):
            for mutation in ("dependency", "binding", "lock", "managed", "foreign-data"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve())
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    # A valid exact refusal with an empty public list remains
                    # insufficient if the failed add changed private ownership.
                    if mutation == "dependency":
                        (observer.data / "uap.toml").write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
                    elif mutation == "binding":
                        self.author_saved(observer, self.row(0), 0)
                    elif mutation == "lock":
                        (observer.data / "uap.lock").write_text('{"lock_version":1,"packages":[{"alias":"residue"}]}')
                    elif mutation == "foreign-data":
                        (observer.data / "unowned.json").write_text('{}')
                    else:
                        path = observer.data / "managed/hidden/package.json"
                        path.parent.mkdir(parents=True); path.write_text('{}')
                    with self.assertRaises(ValueError): self.finish(observer, row, sequence)
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            cache = observer.data / "store/cache/inert.json"
            cache.parent.mkdir(parents=True); cache.write_text('{}')
            (observer.data / "mutation.lock").write_bytes(b"")
            proof = self.finish(observer, self.row(13))
            self.assertEqual(proof["status"], "verified-installation-negative")

    def test_exact_negatives_allow_only_producer_cache_names_and_bounded_coordination(self):
        for index in (13, *COMPONENT_REFUSALS):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                for name in ("directory-v1-cache.json", "discovery-v1-cache.json"):
                    (observer.data / name).write_text('{"schema_version":1,"sequence":1,"snapshot":"e30=","envelope":"e30="}')
                    (observer.data / (name + ".lock")).write_bytes(b"")
                self.assertEqual(self.finish(observer, self.row(index))["status"], "verified-installation-negative")
        for mutation in ("wrong-name", "link", "fifo"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                path = observer.data / "directory-v1-cache.json"
                if mutation == "wrong-name": path.with_name("directory-v1-cache.json.retained").write_text('{}')
                elif mutation == "link": path.symlink_to(observer.foreign)
                else: os.mkfifo(path)
                with self.assertRaises(ValueError): self.finish(observer, self.row(13))

    def test_exact_negatives_preserve_full_saved_authority_bytes_existence_and_metadata(self):
        empty = {"uap.toml": b'schema_version=1\n[dependencies]\n[targets]\n',
                 "uap.lock": json.dumps({"lock_version": 1, "manifest_digest": "sha256:" + hashlib.sha256(
                     b'{"schema_version":1,"dependencies":[],"targets":[]}').hexdigest(), "packages": []}).encode(),
                 "state-v2.json": b'{"schema_version":5,"installations":[]}'}
        for index in (13, *COMPONENT_REFUSALS):
            with self.subTest(index=index, mutation="untouched"), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve(), saved=empty)
                self.assertEqual(self.finish(observer, self.row(index))["status"], "verified-installation-negative")
            for mutation in ("new-lock", "new-state", "new-manifest", "rewrite", "same-bytes-replace", "mode", "removed", "transient"):
                with self.subTest(index=index, mutation=mutation), TemporaryDirectory() as temporary:
                    observer, _ = self.fixture(Path(temporary).resolve(), saved=empty if not mutation.startswith("new-") else None)
                    row = self.row(index)
                    sequence = self.sequence(observer, row)
                    manifest = observer.data / "uap.toml"
                    if mutation == "new-lock":
                        (observer.data / "uap.lock").write_text('{"lock_version":1,"manifest_digest":"corrupt","packages":[]}')
                    elif mutation == "new-state":
                        (observer.data / "state-v2.json").write_text('{"schema_version":5,"installations":[],"unexpected":"retained"}')
                    elif mutation == "new-manifest": manifest.write_bytes(empty["uap.toml"])
                    elif mutation in ("rewrite", "transient"): manifest.write_bytes(b'unknown="injected"\n' + empty["uap.toml"])
                    elif mutation == "same-bytes-replace":
                        pending = manifest.with_name("replacement")
                        pending.write_bytes(manifest.read_bytes()); pending.replace(manifest)
                    elif mutation == "mode": manifest.chmod(0o400)
                    elif mutation == "removed": manifest.unlink()
                    observer.observe(*sequence[0])
                    if mutation == "transient": manifest.write_bytes(empty["uap.toml"])
                    observer.observe(*sequence[1])
                    with self.assertRaises(ValueError): observer.finish(row, row["source"], index, self.fixtures)

    def test_negative_cannot_erase_private_residue_before_empty_list(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(13)
            sequence = self.sequence(observer, row)
            manifest = observer.data / "uap.toml"
            manifest.write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
            observer.observe(*sequence[0])
            manifest.unlink()
            observer.observe(*sequence[1])
            with self.assertRaises(ValueError): observer.finish(row, row["source"], row["index"], self.fixtures)

    def test_saved_binding_delivery_lock_repeat_and_removal_are_required(self):
        for mutation in ("no-saved", "manifest-missing", "lock-missing", "state-missing", "wrong-source",
                         "wrong-revision", "wrong-binding", "wrong-state", "wrong-lock", "wrong-lock-intent",
                         "receipt-digest", "wrong-locator", "missing-managed", "managed-bytes",
                         "repeat-binding", "repeat-delivery", "retained-binding", "retained-package", "retained-intent"):
            with self.subTest(mutation=mutation), TemporaryDirectory() as temporary:
                observer, _ = self.fixture(Path(temporary).resolve())
                row = self.row(0)
                sequence = self.sequence(observer, row)
                def corrupt_sequence():
                    for position, entry in enumerate(sequence):
                        if mutation != "no-saved":
                            retained_state = None
                            if position == 4 and mutation in ("retained-binding", "retained-package"):
                                retained_state = (observer.data / "state-v2.json").read_text()
                                retained_path = Path(next(iter(json.loads(retained_state)["installations"][0]["clients"].values()))["target_locator"])
                            if position == 4 and mutation == "state-missing":
                                (observer.data / "state-v2.json").write_text(saved_state)
                            self.author_saved(observer, row, position)
                            if retained_state is not None:
                                if mutation == "retained-binding":
                                    (observer.data / "state-v2.json").write_text(retained_state)
                                else:
                                    retained_path.mkdir(); (retained_path / "retained.json").write_text('{}')
                            if position == (2 if mutation.startswith("repeat-") else 0):
                                state_path = observer.data / "state-v2.json"
                                state = json.loads(state_path.read_text())
                                record = state["installations"][0]
                                binding = next(iter(record["clients"].values()))
                                saved_state = json.dumps(state)
                                if mutation.endswith("missing") and mutation != "missing-managed":
                                    (observer.data / {"manifest-missing": "uap.toml", "lock-missing": "uap.lock", "state-missing": "state-v2.json"}[mutation]).unlink()
                                elif mutation == "wrong-source": record["source"]["repository"] = "foreign/source"
                                elif mutation == "wrong-revision": binding["package_revision"]["tree_digest"] = "sha256:" + "0" * 64
                                elif mutation in ("wrong-binding", "repeat-binding"):
                                    binding["client_binding_id"] = "foreign"
                                elif mutation == "wrong-state": binding["activation"] = "active"
                                elif mutation in ("wrong-lock", "wrong-lock-intent"):
                                    lock_path = observer.data / "uap.lock"
                                    locked = json.loads(lock_path.read_text())
                                    if mutation == "wrong-lock": locked["packages"][0]["tree_digest"] = "sha256:" + "0" * 64
                                    else: locked["manifest_digest"] = "sha256:" + "0" * 64
                                    lock_path.write_text(json.dumps(locked))
                                elif mutation == "receipt-digest": binding["native_objects"][0]["managed_digest"] = "sha256:" + "0" * 64
                                elif mutation == "wrong-locator": binding["target_locator"] += "-foreign"
                                elif mutation == "missing-managed": shutil.rmtree(binding["target_locator"])
                                elif mutation in ("managed-bytes", "repeat-delivery"):
                                    (Path(binding["target_locator"]) / "mcp.json").write_text('{"changed":true}')
                                if mutation not in ("state-missing",): state_path.write_text(json.dumps(state))
                            if position == 4 and mutation == "retained-intent":
                                (observer.data / "uap.toml").write_text('schema_version=1\n[dependencies]\nresidue="1.0.0"\n')
                        yield entry
                with self.assertRaises(ValueError):
                    for argv, outcome in corrupt_sequence(): observer.observe(argv, outcome)
                    observer.finish(row, row["source"], row["index"], self.fixtures)

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
                         "rekey", "remove-id", "remove-outcome", "add-id", "add-revision", "add-digest", "add-missing-revision", "add-missing-digest", "add-result", "result-type", "implicit-scope", "nan", "large", "timeout", "exit", "stderr"):
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
                elif mutation == "add-id": sequence[0][1]["_json"]["actions"][0]["installation_id"] = "foreign-installation"
                elif mutation == "add-revision": sequence[0][1]["_json"]["actions"][0]["revision"] = "0" * 40
                elif mutation == "add-digest": sequence[0][1]["_json"]["actions"][0]["package_digest"] = "sha256:" + "0" * 64
                elif mutation == "add-missing-revision": sequence[0][1]["_json"]["actions"][0].pop("revision")
                elif mutation == "add-missing-digest": sequence[0][1]["_json"]["actions"][0].pop("package_digest")
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
                for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
                    if when == "repeat" and position == 2:
                        (observer.global_root / "unexpected").write_text("changed during repeat")
                    observer.observe(argv, outcome)
                if when == "late":
                    (observer.global_root / "late").write_text("post-observation write")
                with self.assertRaises(ValueError) as caught:
                    observer.finish(row, row["source"], 0, self.fixtures)
                if when == "repeat":
                    changes = caught.exception.windsurf_installation_diagnostics["comparisons"]
                    self.assertEqual(changes[1]["phase"], "after-repeat-add")
                    self.assertGreater(changes[1]["total"], 0)
                    self.assertTrue(any("unexpected" in entry.get("relative_key", "")
                                        for entry in changes[1]["entries"]))

    def test_unattributed_global_leftover_cannot_hide_behind_empty_identity_scanner(self):
        with TemporaryDirectory() as temporary:
            observer, _ = self.fixture(Path(temporary).resolve())
            row = self.row(0)
            sequence = self.sequence(observer, row)
            for position, (argv, outcome) in enumerate(self.authored_sequence(observer, row, sequence)):
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
        for mutation in ("missing-proof", "missing-saved-proof", "source", "index-bool", "residue", "sentinel", "repeat", "global-omitted",
                         "image-count", "negative-image", "state-promotion", "writer-claim", "model-native-count",
                         "missing-row", "duplicate-row", "cli-bool"):
            with self.subTest(mutation=mutation):
                report = copy.deepcopy(original)
                row = report["results"][0]
                proof = row["windsurf_installation_audit"]
                if mutation == "missing-proof": del row["windsurf_installation_audit"]
                elif mutation == "missing-saved-proof": proof.pop("initial_saved_authority_sha256")
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
            for argv, outcome in self.authored_sequence(observer, row, self.sequence(observer, row)):
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

    def integrated_native_case(self, cli, root, index, mode="good"):
        """Real combined route with complete 16/26/1/8 Skill sets and inert I/O."""
        import test_windsurf_global_skill_audit as native_fixture
        # Independent literal packagedigest preimages for the authored test
        # files below. Production source pins are never used for inert bytes.
        digests = {
            22: ("51e1ae43dc56a04dc78e19caa4de20770df0131f631ce1244ac00a562d5be22e", "78163fe349ca9830bcc6c13f875c62406d814dce006cb882a4fff5b94dd85d89"),
            25: ("3b6992d52cf94825bf93f7bca7b28123ea9b16a239bedbe3cadcf2c2bb375259", "f935ed81b4e58a7498613b90b5ddfe6a5ca5aa0d4961cd76bd5399107326821d"),
            26: ("4cf078c031e51e4bf846c8aa0c7e16e8f139290a2405b56933c378bd5265de57", "7f863d62e433cda5dd3cc8e06198a8d526275b498f9f91f699283870a54123f7"),
            29: ("11d1cffcbbee21931c852b685a564ee3b9566f6e651ad5cb5a470362a74b07c4", "cd286b901c0aff9d9af3b42bfbf51e3a836dde3c351f1d81e6a1354b3370e16e"),
        }
        item, identity = selected(self.fixtures, index)
        pin = copy.deepcopy(native_fixture.proof.CASES[item])
        pin.update(tree_digest="sha256:" + digests[index][0], manifest_digest="sha256:" + digests[index][1])
        names = pin["skills"]
        bodies = {name: (f"---\nname: {name}\ndescription: {native_fixture.DESCRIPTION}\n---\nA unit-only fixture.\n").encode()
                  for name in names}
        files = {"plugin.json": native_fixture.packed({"name": identity, "version": pin["version"]})}
        for name in names:
            files.update({f"skills/{name}/SKILL.md": bodies[name],
                          f"skills/{name}/references/note.txt": native_fixture.REFERENCE})

        class CompleteTransport(native_fixture.InertPublicAndNative):
            def __init__(self):
                super().__init__(root, mode)
                self.repo = self.repo.lower()
                self.source_key = "github:" + self.repo + ":"

            def author_installation(self, env, cwd):
                super().author_installation(env, cwd)
                for relative, body in files.items():
                    native_fixture.write(self.managed / relative, body)
                installed = self.state["installations"][0]
                installed["package"]["inventory"]["skills"] = list(names)
                source = "github:" + self.repo + "@" + self.commit
                installed["source"].update(requested_source=source, canonical_source=source)
                objects = installed["clients"][self.binding_id]["native_objects"]
                objects[0]["managed_digest"] = native_fixture.source_digest(files)
                for name in names[1:]:
                    directory = self.home / ".config/devin/skills" / name
                    native_fixture.write(directory / "SKILL.md", bodies[name])
                    native_fixture.write(directory / "references/note.txt", native_fixture.REFERENCE)
                    stream = (b"f\0SKILL.md\0-\0" + bodies[name] + b"\0d\0references\0f\0references/note.txt\0-\0"
                              + native_fixture.REFERENCE + b"\0")
                    objects.append({"object_id": "devin_global_skill_directory:" + name,
                        "kind": "devin_global_skill_directory", "logical_name": name,
                        "path": str(directory), "source_relative": "skills/" + name,
                        "managed_digest": "sha256:" + native_fixture.hashed(stream), "protection_class": "managed"})
                native_fixture.write(self.data / "state-v2.json", native_fixture.packed(self.state))

            def add_document(self, repeated):
                raw = super().add_document(repeated)
                if not repeated:
                    raw["data"]["actions"][0]["component_support"] = [
                        {"kind": "skill", "name": name, "support": "prepared"} for name in names]
                return raw

            def call(self, argv, *args, **kwargs):
                if argv[1] == "remove":
                    for name in names[1:]:
                        native_fixture.shutil.rmtree(self.home / ".config/devin/skills" / name)
                return super().call(argv, *args, **kwargs)

            def run_probe(self, argv, cwd, env, directory, label, **options):
                record, stdout, stderr = super().run_probe(argv, cwd, env, directory, label, **options)
                if label in ("windsurf-global-add", "windsurf-global-repeat"):
                    rows = json.loads(stdout)
                    rows.extend({"name": name, "description": native_fixture.DESCRIPTION, "provider": "Devin",
                        "base_dir": str(self.home / ".config/devin/skills" / name), "display_name": name,
                        "triggers": ["user", "model"], "warnings": [], "errors": []} for name in names[1:])
                    stdout = native_fixture.packed(rows)
                    record.update(stdout_bytes=len(stdout), stdout_sha256=native_fixture.hashed(stdout))
                return record, stdout, stderr

        live = self.audit["do_case"].__globals__
        cls = live["WINDSURF_GLOBAL_AUDIT"]["WindsurfGlobalSkillAudit"]
        original_init = cls.__init__
        installation_globals = live["WINDSURF_INSTALLATION"]["summary"].__globals__
        with patch.dict(vars(native_fixture), {"ITEM": item, "SKILL": names[0], "BODY": bodies[names[0]],
                "SOURCE_FILES": files, "UNIT_TREE": pin["tree_digest"], "UNIT_MANIFEST": pin["manifest_digest"]}):
            transport = CompleteTransport()
            def retain_instance(instance, *args, **kwargs):
                original_init(instance, *args, **kwargs)
                transport.verifier = instance
            pins = {**native_fixture.proof.CASES, item: pin}
            with patch.dict(original_init.__globals__, {"CASES": pins,
                    "BINARY_SHA": native_fixture.hashed(transport.binary.read_bytes())}), \
                    patch.dict(installation_globals, {"GLOBAL_SKILL_CASES": pins}), \
                    patch.object(cls, "__init__", retain_instance), \
                    patch.dict(live, {"call": transport.call, "AGENT_BINARIES": {"devin": str(transport.binary)}}), \
                    patch.dict(live["NATIVE_RESEARCH"], {"run_probe": transport.run_probe}):
                row = self.audit["do_case"](cli, "windsurf", "plugin", item, index, root / "cases",
                    self.fixtures["plugin_declared_names"], self.fixtures["signed_release_pins"],
                    client_version={"status": "reported", "value": CLIENT_VERSION, "exit_code": 0},
                    installation_audit="windsurf-global-installation", installation_fixtures=self.fixtures)
            self.assertTrue(transport.verifier.closed)
            self.assertEqual(len([event for event in transport.events if event[0] == "public"]), 6)
            self.assertEqual([event for event in transport.events if event[0] == "native"],
                             [("native", phase) for phase in ("before", "add", "repeat", "remove")])
            self.assertEqual(row["windsurf_global_native_proof"]["immutable_source_store"]["complete_skills"], list(names))
            self.assertFalse((transport.home / ".config/devin/foreign-neighbor.txt").exists())
            return row, {item: pin}

    def test_real_do_case_observes_wrapped_envelopes_before_consuming_them(self):
        # Only public subprocess responses are substituted. Run the real case
        # setup, native roots, source matcher, repeat/removal and final observer.
        cli = str(Path(sys.executable).resolve())
        for index, count in ((0, 6), (22, 6), (13, 2), (23, 2)):
            with self.subTest(index=index), TemporaryDirectory() as temporary:
                expected_row = self.row(index)
                if index in ACTIVE_SLOTS:
                    row, _ = self.integrated_native_case(cli, Path(temporary).resolve(), index)
                    self.assertEqual(row["status"], "passed")  # Inert transport only.
                    proof = row["windsurf_installation_audit"]
                    self.assertEqual(proof["public_state"], ACTIVE_SKILLS)
                    self.assertEqual(len(proof["observations"]), count)
                    self.assertEqual(proof["observations"][4]["argv"][2], row["matched_installation_ids"][0])
                    self.assertTrue(row["windsurf_global_native_proof"]["native_discovery_verified"])
                    self.assertFalse(proof["native_runtime_qualified"])
                    for position, operation in enumerate(("install", "list", "repeat_add", "repeat_list", "cleanup", "post_remove_list")):
                        self.assertNotIn("_json", row[operation])
                        # Existing native callers retain complete streams; the
                        # installation observer hashes the same parsed JSON.
                        self.assertEqual(installation.digest(json.loads(row[operation]["stdout_full"]), 1 << 20),
                                         proof["observations"][position]["json_sha256"])
                        self.assertEqual(row[operation]["stderr_full"], "")
                    continue
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
                    self.author_saved(SimpleNamespace(data=Path(env["AGENTPLUGINS_HOME"]),
                        global_root=Path(env["XDG_CONFIG_HOME"]) / "devin"), expected_row, len(calls))
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

    def first_main_case_with_data_transition(self, mutation=None):
        """Actual main/case/observer; only version and public child I/O are inert."""
        class StopAfterFirstCase(BaseException):
            pass

        cli = str(Path(sys.executable).resolve())
        live = self.audit["main"].__globals__
        sequence = self.sequence(SimpleNamespace(cli=cli), self.row(0))
        for position in (0, 2):
            outcome = sequence[position][1]
            outcome["_json"] = {"schema_version": 1, "command": "add", "result": "success",
                                "data": outcome["_json"]}
            outcome["stdout_tail"] = json.dumps(outcome["_json"])[-1800:]
        calls, identities, transitions = [], [], []
        def identity(path):
            info = path.lstat()
            return info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid

        def version(argv, env, cwd):
            if argv[1] == "version" and mutation in ("existing-mode", "symlink"):
                data = Path(cwd).parent / "plugin-00/uap-data"
                data.parent.mkdir()
                if mutation == "existing-mode":
                    data.mkdir(mode=0o755)
                    data.chmod(0o755)
                else:
                    foreign = Path(cwd).parent / "foreign-data"
                    foreign.mkdir()
                    data.symlink_to(foreign, target_is_directory=True)
            return {"status": "reported", "value": CLI_VERSION if argv[1] == "version" else CLIENT_VERSION,
                    "exit_code": 0}

        def public_call(argv, env, project, *args, **kwargs):
            if project.parent.name != "plugin-00":
                self.assertEqual(project.parent.name, "plugin-01")
                # The first row is already saved by the real main. This scoped
                # test does not run or qualify the remaining 29 native cases.
                raise StopAfterFirstCase()
            expected, outcome = sequence[len(calls)]
            self.assertEqual(argv, expected)
            self.assertIs(kwargs.get("capture_full"), True)
            data = Path(env["AGENTPLUGINS_HOME"])
            identities.append(identity(data))
            self.author_saved(SimpleNamespace(data=data, global_root=Path(env["XDG_CONFIG_HOME"]) / "devin"),
                              self.row(0), len(calls))
            calls.append(list(argv))
            if len(calls) == 1:
                if mutation == "mode":
                    data.chmod(0o755)
                elif mutation == "inode":
                    data.rename(data.with_name("saved-data"))
                    data.mkdir(mode=0o700)
                else:
                    # The real state and mutation-lock writers protect this
                    # parent before returning the first public add envelope.
                    data.chmod(0o700)
                transitions.append(identity(data))
            return copy.deepcopy(outcome)

        with TemporaryDirectory() as temporary:
            output = Path(temporary).resolve() / "report"
            previous_umask = os.umask(0o022)
            try:
                with patch.dict(live, {"call": public_call, "version_evidence": version,
                        "vendor_bootstrap_attested": lambda: True, "AGENT_BINARIES": {"devin": cli}}), \
                        patch.object(self.audit["subprocess"], "Popen", side_effect=AssertionError(
                            "child forbidden in this inert data-mode regression")) as children, \
                        patch.object(self.audit["platform"], "machine", return_value="x86_64"), \
                        patch.object(sys, "stdout", StringIO()), \
                        patch.object(sys, "argv", ["audit", "--cli", cli, "--kind", "plugin", "--target", "windsurf",
                            "--limit", "30", "--acceptance", "windsurf-global-installation", "--report-dir", str(output)]):
                    with self.assertRaises(StopAfterFirstCase):
                        self.audit["main"]()
                    children.assert_not_called()
            finally:
                os.umask(previous_umask)
            report = json.loads((output / "windsurf-plugin.json").read_text())
        self.assertEqual(report["cases_attempted"], 1)
        self.assertEqual(len(report["results"]), 1)
        self.assertFalse(report["full_coverage"])
        self.assertFalse(report["installation_acceptance_passed"])
        self.assertFalse(report["retained_native_acceptance_passed"])
        return report["results"][0], calls, identities, transitions

    def test_real_main_authors_private_data_before_first_public_add(self):
        row, calls, identities, transitions = self.first_main_case_with_data_transition()
        self.assertEqual(row["status"], "prepared-lifecycle-passed",
                         f"first data identities: {identities}; after chmod: {transitions}; {row.get('error')}")
        self.assertEqual(len(calls), 6)
        self.assertEqual(stat.S_IMODE(identities[0][0]), 0o700)
        self.assertTrue(all(value == identities[0] for value in identities + transitions))
        self.assertTrue(row["repeat_verified"] and row["removal_verified"] and row["sentinels_preserved"])
        proof = row["windsurf_installation_audit"]
        self.assertEqual(len(proof["observations"]), 6)
        self.assertEqual(proof["public_state"], PREPARED)
        self.assertFalse(proof["native_runtime_qualified"])

    def test_real_main_keeps_data_mode_inode_and_symlink_refusals(self):
        for mutation in ("mode", "inode", "existing-mode", "symlink"):
            with self.subTest(mutation=mutation):
                row, calls, identities, transitions = self.first_main_case_with_data_transition(mutation)
                self.assertEqual(row["status"], "harness-error")
                self.assertNotIn("windsurf_installation_audit", row)
                if mutation == "symlink":
                    self.assertEqual(calls, [])
                    self.assertIn("nested audit directory escapes disposable case", row["error"])
                else:
                    self.assertEqual(len(calls), 1)
                    self.assertNotEqual(identities[0], transitions[0])
                    self.assertIn("Devin Global installation evidence is unresolved", row["error"])

    def test_real_main_installation_exit_retains_native_gate_and_counts(self):
        cli = str(Path(sys.executable).resolve())
        ordinary = self.completed_report(cli)["results"]
        live = self.audit["main"].__globals__
        observer_globals = live["WINDSURF_INSTALLATION"]["summary"].__globals__
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            qualified, pins = {}, dict(observer_globals["GLOBAL_SKILL_CASES"])
            for index in ACTIVE_SLOTS:
                row, selected_pin = self.integrated_native_case(cli, root, index)
                self.assertEqual(row["status"], "passed")  # Inert transport only.
                qualified[index] = row
                pins.update(selected_pin)
            failed, _ = self.integrated_native_case(cli, root / "failed-native", 26, "native-missing")
            self.assertEqual(failed["status"], "prepared-lifecycle-passed")
            binary = qualified[26]["windsurf_global_native_proof"]["binary"]["path"]
            cases = (
                ("no-native", (), None, True, False, 1),
                ("one-native", (26,), None, True, False, 1),
                ("all-four", tuple(ACTIVE_SLOTS), None, True, True, 0),
                ("failed-native", tuple(ACTIVE_SLOTS), "failed-native", True, False, 1),
                ("missing-installation", tuple(ACTIVE_SLOTS), "missing-installation", False, False, 1),
                ("missing-native", tuple(ACTIVE_SLOTS), "missing-native", False, False, 1),
                ("forged-native", tuple(ACTIVE_SLOTS), "forged-native", False, False, 1),
            )
            for name, admitted, mutation, installed, retained, expected_exit in cases:
                with self.subTest(case=name):
                    selected_rows = copy.deepcopy(ordinary)
                    for index in admitted:
                        selected_rows[index] = copy.deepcopy(qualified[index])
                    if mutation == "failed-native": selected_rows[26] = copy.deepcopy(failed)
                    elif mutation == "missing-installation": selected_rows[0].pop("windsurf_installation_audit")
                    elif mutation == "missing-native": selected_rows[26].pop("windsurf_global_native_proof")
                    elif mutation == "forged-native": selected_rows[26]["windsurf_global_native_proof"] = {"native_discovery_verified": True}
                    def do_case(*args, **kwargs):
                        self.assertEqual(kwargs.get("installation_audit"), "windsurf-global-installation")
                        self.assertEqual(kwargs.get("installation_fixtures"), self.fixtures)
                        self.assertEqual(args[3], self.fixtures["plugin_aliases"][args[4]])
                        return copy.deepcopy(selected_rows[args[4]])
                    def version(argv, *args):
                        return {"status": "reported", "value": CLI_VERSION if argv[1] == "version" else CLIENT_VERSION,
                                "exit_code": 0}
                    output = root / ("report-" + name)
                    with patch.dict(live, {"do_case": do_case, "vendor_bootstrap_attested": lambda: True,
                            "AGENT_BINARIES": {"devin": binary}, "version_evidence": version}), \
                            patch.dict(observer_globals, {"GLOBAL_SKILL_CASES": pins}), \
                            patch.object(sys, "platform", "linux"), \
                            patch.object(self.audit["platform"], "machine", return_value="x86_64"), \
                            patch.object(sys, "stdout", StringIO()), \
                            patch.object(sys, "argv", ["audit", "--cli", cli, "--kind", "plugin", "--target", "windsurf",
                                "--limit", "30", "--acceptance", "windsurf-global-installation", "--report-dir", str(output)]):
                        code = self.audit["main"]()
                    report = json.loads((output / "windsurf-plugin.json").read_text())
                    self.assertEqual(code, expected_exit)
                    self.assertEqual(report["passed"], len(admitted) - (mutation == "failed-native"))
                    self.assertEqual(report["verified_negative_coverage"], 0)
                    self.assertEqual(report["cases_attempted"], 30)
                    self.assertIs(report["installation_acceptance_passed"], installed)
                    self.assertIs(report["retained_native_acceptance_passed"], retained)
                    self.assertFalse(any(report[key] for key in ("native_runtime_qualified", "authentication_qualified",
                                                                "model_qualified", "mcp_tools_qualified")))
                    if installed:
                        self.assertEqual((report["installation_passed"], report["installation_verified_negative_coverage"]), (25, 5))
                    self.assertEqual([row["status"] for row in report["results"]],
                                     [row["status"] for row in selected_rows])

    def test_summary_requires_exact_versions_complete_selection_and_new_proof(self):
        base = self.completed_report()
        self.assertTrue(summary(base, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
        missing = copy.deepcopy(base)
        missing["results"][0].pop("windsurf_installation_audit")
        self.assertFalse(summary(missing, self.fixtures, fixture_sha256=self.fixture_hash)["installation_acceptance_passed"])
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
