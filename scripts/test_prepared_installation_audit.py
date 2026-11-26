"""Adversarial evidence tests for explicit Cursor/Grok preparation qualification."""
from copy import deepcopy
import json
import hashlib
import uuid
import contextlib
import io
from pathlib import Path
from runpy import run_path
import tempfile
import sys
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[1]
AUDIT = run_path(str(ROOT / "scripts/prepared_installation_audit.py"))
FIXTURES = json.loads((ROOT / "tests/live-install/fixtures.json").read_text())


def envelopes(kind, identity, target="cursor", item=None, project="/fixture/project"):
    scope = "project" if kind == "skill" else "user"
    names = ["marketing-kit", "notion", "hubspot-crm", "playwright"] if identity == "marketing-kit" else [identity]
    state = {"materialization": "materialized", "policy": "allowed",
             "activation": "prepared" if kind == "skill" else "manual_activation_required",
             "authentication": "not_required" if kind == "skill" else "not_checked",
             "verification": "installation_verified" if kind == "skill" else "package_validated"}
    if target == "grok":
        state.update(authentication="not_checked", verification="package_validated")
    rows = [{"name": name, "installation_id": "id-" + name, "mixed_version": False,
             "clients": [{"client_id": target, "scope": scope, **state}]} for name in names]
    if target == "grok":
        candidate = hashlib.sha256("\0".join((item["repo"], item["ref"], "skills/" + item["folder"])).encode()).hexdigest()
        digest = hashlib.sha256(("github-skill:" + item["repo"].lower() + ":" + identity + ":skills/" + item["folder"]).encode()).hexdigest()
        rows[0].update(name="skill-" + digest[:56],
                       installation_id=str(uuid.uuid5(uuid.NAMESPACE_URL, identity)),
                       source=item["repo"] + "/skills/" + item["folder"])
        rows[0]["clients"][0]["package_revision"] = {
            "resolved_revision": item["ref"], "tree_digest": "sha256:" + "a" * 64,
            "manifest_digest": "sha256:" + "b" * 64}
        bundle = Path(project) / ".grok/plugins" / (rows[0]["name"][:51].rstrip(".-") + "-" + hashlib.sha256(rows[0]["installation_id"].encode()).hexdigest()[:12])
    def inventory(values):
        return {"schema_version": 1, "command": "list", "result": "success",
                "data": {"installations": deepcopy(values)}}
    result = {"list": inventory(rows), "repeat_list": inventory(rows), "post_remove_list": inventory([])}
    for phase, repeated in (("install", False), ("repeat_add", True)):
        if kind == "skill":
            result[phase] = {"schema_version": 1, "command": "skills add", "result": "success",
                             "status": "succeeded", "scope": scope, "scope_selection_reason": "explicit",
                             "readiness": "needs_action", "repairInstructions": [],
                             "items": [{"alias": identity, "targetId": target,
                                        "status": "unchanged" if repeated else "succeeded"}],
                             "next_actions": [{"code": "verify_native", "actor": "native_client"}]}
            if target == "grok":
                result[phase]["items"][0].update(candidateId="sha256:" + candidate, destinationPath=str(bundle))
        else:
            data = {"schema_version": 1, "command": "agentplugins add", "result": "success",
                    "outcome": "completed", "scope": scope, "scope_selection_reason": "explicit",
                    "environment": {"scope": scope, "scope_selection_reason": "explicit",
                                    "selected_targets": ["cursor"]},
                    "actions": [{"alias": name, "installation_id": "id-" + name, "client_id": "cursor",
                                 "action": "unchanged" if repeated else "install"} for name in names],
                    "results": [] if repeated else [
                        {"installation_id": row["installation_id"], "client_id": "cursor",
                         "operation": "install", "outcome": "completed"} for row in rows]}
            result[phase] = {"schema_version": 1, "command": "add", "result": "success", "data": data}
    result["cleanup"] = {"schema_version": 1, "command": "agentplugins remove", "result": "success",
                         "outcome": "completed", "scope": scope, "scope_selection_reason": "explicit",
                         "actions": [{"installation_id": row["installation_id"], "client_id": target,
                                      "action": "remove"} for row in rows],
                         "results": [{"installation_id": row["installation_id"], "client_id": target,
                                      "operation": "remove", "outcome": "completed"} for row in rows]}
    return result


def observe(kind, identity, responses, target="cursor", item=None, project="/fixture/project"):
    # Keep the existing source-verified selection separate from mutated raw
    # response fixtures, so an injected response cannot redefine its authority.
    original = envelopes(kind, identity, target, item, project)
    return AUDIT["observe"](kind, identity, responses,
                            original["list"]["data"]["installations"],
                            original["repeat_list"]["data"]["installations"], target=target, skill_fixture=item)


def report(kind, target="cursor"):
    sources = FIXTURES["skills" if kind == "skill" else "plugin_aliases"]
    rows = []
    for index, item in enumerate(sources):
        identity = item["name"] if kind == "skill" else FIXTURES["plugin_declared_names"].get(item, item)
        scope = "project" if kind == "skill" else "user"
        responses = envelopes(kind, identity, target, item)
        state = responses["list"]["data"]["installations"][0]["clients"][0]
        state = {key: value for key, value in state.items() if key not in ("client_id", "scope", "package_revision")}
        row = {"kind": kind, "target": target, "index": index, "identity": identity,
               "source": item["source"] if kind == "skill" else item,
               "status": "incomplete-native-proof" if kind == "skill" and target == "cursor" else "prepared-lifecycle-passed",
               "matched_installation_ids": [responses["list"]["data"]["installations"][0]["installation_id"]],
               "repeat_installation_ids": [responses["list"]["data"]["installations"][0]["installation_id"]],
               target + "_preparation_envelopes": observe(kind, identity, responses, target, item),
               "prepared_binding": kind == "plugin" or target == "grok", "prepared_components_only": kind == "plugin",
               "public_skill_ready": {"after-add": False, "repeat-add": False} if kind == "skill" else None,
               "native_file_delta": 0 if kind == "skill" and target == "cursor" else 3,
               "native_identity_files": [] if kind == "skill" else [".cursor/plugins/local/example/plugin.json"],
               "native_component_candidates": [], "native_residuals": [], "post_removal_leftovers": [],
               "post_skill_tree_delta": [], "post_removal_skill": None, "post_lock_pinned": False,
               "repeat_project_unchanged": True if target == "grok" else None,
               "cursor_repeat_project_unchanged": True if target == "cursor" else None,
               "retained_passive_native_files": [], "signed_release_observation": [],
               "skill_path": f".cursor/skills/{identity}/SKILL.md" if kind == "skill" else None}
        if target == "grok":
            row["skill_path"] = row["grok_preparation_envelopes"]["skill_path"]
        for key in ("listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid", "sentinels_preserved",
                    "lock_written", "lock_contains_identity", "lock_exact_source_pinned", "skill_bytes_match"):
            row[key] = True
        for key in ("repeat_native_runtime_only", "failed_install_registered", "failed_install_residue",
                    "post_remove_identity_registered"):
            row[key] = False
        for key in AUDIT["COMMANDS"]:
            row[key] = {"exit_code": 0, "timeout": False}
        row["public_target_states"] = {
            phase: {"status": "observed", "selected_installations": int(phase != "after-remove"),
                    "selected_bindings": int(phase != "after-remove"), "invalid_rows": 0,
                    "states": [] if phase == "after-remove" else [deepcopy(state)], "truncated": False}
            for phase in ("after-add", "repeat-add", "after-remove")}
        if kind == "plugin" and not item.startswith("github:"):
            pin = FIXTURES["signed_release_pins"][item]
            row["signed_release_observation"] = [{
                "source": pin["repository"] + "/" + pin["path"], "recorded_distribution": pin["distribution"],
                "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"],
                "resolved_revision": pin["revision"], "release_sequence": pin["sequence"],
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}]
        rows.append(row)
    return {"target": target, "kind": kind, "platform": "linux", "architecture": "x86_64",
            "client_version": {"status": "reported", "value": AUDIT["GROK_VERSION"] if target == "grok" else AUDIT["VERSIONS"][kind], "exit_code": 0},
            "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0},
            "fixture_count": 30, "cases_attempted": 30, "source_selection_complete": True,
            "full_coverage": True, "passed": 0, "verified_negative_coverage": 0,
            "prepared_lifecycle_passed": 0 if kind == "skill" and target == "cursor" else 30, "results": rows}


class PreparedInstallationTest(unittest.TestCase):
    def test_cursor_skill_summary_requires_exact_project_repeat_custody(self):
        for value in (None, False, True, 1, "true", "missing"):
            with self.subTest(value=value):
                document = report("skill")
                row = document["results"][0]
                self.assertIs(row["repeat_verified"], True)
                if value == "missing":
                    row.pop("cursor_repeat_project_unchanged")
                else:
                    row["cursor_repeat_project_unchanged"] = value
                result = self.summary(document)
                self.assertIs(result["installation_acceptance_passed"], value is True)

    def summary(self, value):
        mode = ("grok-prepared-skills-installation" if value["target"] == "grok" else
                "cursor-prepared-skills-installation" if value["kind"] == "skill" else "cursor-prepared-installation")
        return AUDIT["summary"](value, FIXTURES, mode)

    def test_complete_grok_preparation_preserves_rows_and_zero_native_claims(self):
        value = report("skill", "grok")
        before = deepcopy(value)
        proof = self.summary(value)
        self.assertTrue(proof["installation_acceptance_passed"])
        self.assertEqual(proof["installation_passed"], 30)
        self.assertEqual(proof["qualification_scope"], "owned Grok preparation lifecycle")
        self.assertTrue(all(flag is False for key, flag in proof.items() if key.endswith("_qualified")))
        self.assertEqual(value, before)
        self.assertEqual(value["passed"], 0)
        self.assertTrue(all(row["status"] == "prepared-lifecycle-passed" for row in value["results"]))

    def test_grok_raw_envelopes_require_source_selected_singleton_bundle_and_full_lifecycle(self):
        item = FIXTURES["skills"][0]
        identity = item["name"]
        good = envelopes("skill", identity, "grok", item)
        mutations = [lambda d: d["install"]["items"][0].update(targetId="cursor"),
                     lambda d: d["install"]["items"][0].update(candidateId="unverified"),
                     lambda d: d["install"]["items"][0].update(destinationPath=".grok/plugins/relative"),
                     lambda d: d["install"]["items"][0].update(destinationPath="/tmp/../" + d["install"]["items"][0]["destinationPath"].lstrip("/")),
                     lambda d: d["repeat_add"]["items"][0].update(candidateId="sha256:" + "f" * 64),
                     lambda d: d["repeat_add"]["items"][0].update(destinationPath="/foreign" + d["repeat_add"]["items"][0]["destinationPath"]),
                     lambda d: d["install"].update(readiness="ready"),
                     lambda d: d["repeat_add"]["items"][0].update(status="succeeded"),
                     lambda d: d["install"]["next_actions"][0].update(actor="user"),
                     lambda d: d["cleanup"]["results"][0].update(client_id="cursor"),
                     lambda d: d["cleanup"]["actions"][0].update(installation_id="foreign"),
                     lambda d: d["cleanup"].update(outcome="failed"),
                     lambda d: d["post_remove_list"]["data"]["installations"].append({"residue": True})]
        for phase in AUDIT["COMMANDS"]:
            mutations += [lambda d, phase=phase: d.update({phase: None}),
                          lambda d, phase=phase: d[phase].update(schema_version=True),
                          lambda d, phase=phase: d[phase].update(result="failure")]
        for phase in ("list", "repeat_list"):
            mutations += [lambda d, phase=phase: d[phase]["data"]["installations"].append(deepcopy(d[phase]["data"]["installations"][0])),
                          lambda d, phase=phase: d[phase]["data"]["installations"][0].update(source="foreign"),
                          lambda d, phase=phase: d[phase]["data"]["installations"][0].update(installation_id="reminted"),
                          lambda d, phase=phase: d[phase]["data"]["installations"][0]["clients"][0].update(scope="user"),
                          lambda d, phase=phase: d[phase]["data"]["installations"][0]["clients"][0].update(authentication="not_required"),
                          lambda d, phase=phase: d[phase]["data"]["installations"][0]["clients"][0].update(activation="active")]
        for index, mutate in enumerate(mutations):
            with self.subTest(mutation=index):
                changed = deepcopy(good); mutate(changed)
                self.assertEqual(observe("skill", identity, changed, "grok", item)["status"], "unqualified")
        for before, after in (([], good["repeat_list"]["data"]["installations"]),
                              (good["list"]["data"]["installations"], [])):
            self.assertEqual(AUDIT["observe"]("skill", identity, good, before, after, target="grok", skill_fixture=item)["status"], "unqualified")
        self.assertEqual(AUDIT["observe"]("plugin", identity, good, [], [], target="grok", skill_fixture=item)["status"], "unqualified")

    def test_grok_summary_rejects_wrong_route_source_counters_states_and_custody(self):
        mutations = [lambda d: d.update(target="cursor"), lambda d: d.update(kind="plugin"),
                     lambda d: d.update(platform="darwin"), lambda d: d.update(architecture="arm64"),
                     lambda d: d.update(passed=1), lambda d: d.update(passed=False),
                     lambda d: d.update(prepared_lifecycle_passed=True), lambda d: d.update(prepared_lifecycle_passed=29),
                     lambda d: d.update(verified_negative_coverage=1), lambda d: d.update(cases_attempted=29),
                     lambda d: d.update(source_selection_complete=False), lambda d: d.update(full_coverage=False),
                     lambda d: d["client_version"].update(value="grok 1.0.70"),
                     lambda d: d["cli_version"].update(exit_code=False),
                     lambda d: d["results"].pop(), lambda d: d["results"].reverse()]
        row_mutations = [lambda r: r.update(index=False), lambda r: r.update(source="foreign"),
                         lambda r: r.update(identity="foreign"), lambda r: r.update(status="passed"),
                         lambda r: r.update(prepared_binding=False), lambda r: r.update(native_file_delta=True),
                         lambda r: r.update(native_file_delta=0), lambda r: r.update(repeat_project_unchanged=False),
                         lambda r: r.update(repeat_installation_ids=["reminted"]),
                         lambda r: r.update(native_residuals=["residue"]),
                         lambda r: r.update(retained_passive_native_files=["foreign"]),
                         lambda r: r.update(skill_path=r["skill_path"].replace("plugins/", "plugins/../plugins/")),
                         lambda r: r.update(skill_path=r["skill_path"].replace("SKILL.md", "foreign.md")),
                         lambda r: r["grok_preparation_envelopes"].update(skill_path="foreign"),
                         lambda r: r["grok_preparation_envelopes"].update(status="unqualified"),
                         lambda r: r.update(post_skill_tree_delta=["foreign"]),
                         lambda r: r.update(post_lock_pinned=True)]
        for key in ("listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "sentinels_preserved", "lock_written", "lock_contains_identity",
                    "lock_exact_source_pinned", "skill_bytes_match"):
            row_mutations.append(lambda r, key=key: r.update({key: False}))
        for phase in AUDIT["COMMANDS"]:
            row_mutations += [lambda r, phase=phase: r[phase].update(exit_code=False),
                              lambda r, phase=phase: r[phase].update(timeout=True)]
        for phase in ("after-add", "repeat-add", "after-remove"):
            row_mutations += [lambda r, phase=phase: r["public_target_states"][phase].update(truncated=True),
                              lambda r, phase=phase: r["public_target_states"][phase].update(selected_bindings=True)]
        for index, mutate in enumerate(mutations):
            with self.subTest(report_mutation=index):
                value = report("skill", "grok"); mutate(value)
                self.assertFalse(AUDIT["summary"](value, FIXTURES, "grok-prepared-skills-installation")["installation_acceptance_passed"])
        for index, mutate in enumerate(row_mutations):
            with self.subTest(row_mutation=index):
                value = report("skill", "grok"); mutate(value["results"][0])
                self.assertFalse(self.summary(value)["installation_acceptance_passed"])

    def test_grok_real_source_verifier_requires_bundle_receipt_lock_and_immutable_revision(self):
        match = run_path(str(ROOT / "scripts/live-install-audit.py"))["grok_prepared_skill_installations"]
        item = FIXTURES["skills"][0]
        with tempfile.TemporaryDirectory() as temp:
            project = Path(temp).resolve()
            good = envelopes("skill", item["name"], "grok", item, project)
            receipt = good["install"]
            bundle = Path(receipt["items"][0]["destinationPath"])
            skill = bundle / "skills" / item["name"] / "SKILL.md"
            skill.parent.mkdir(parents=True); skill.write_text("owned fixture")
            path = str(skill.relative_to(project))
            node = good["list"]["data"]["installations"][0]
            revision = node["clients"][0]["package_revision"]
            lock = project / "uap.lock"
            lock.write_text(json.dumps({"packages": [{"alias": item["name"], **revision}]}))
            def selected(response, full=receipt, repeated=False):
                return match(response, item, path, project, {"stdout_tail": "unparseable bounded tail"},
                             repeated=repeated, full_receipt=full)
            self.assertEqual(selected(good["list"]), [node])
            self.assertEqual(selected(good["repeat_list"], good["repeat_add"], True), [node])
            proof = AUDIT["observe"]("skill", item["name"], good, selected(good["list"]),
                                     selected(good["repeat_list"], good["repeat_add"], True), target="grok", skill_fixture=item)
            self.assertEqual(proof["status"], "verified")
            for field in ("source", "name", "installation_id", "resolved_revision", "tree_digest", "manifest_digest"):
                with self.subTest(field=field):
                    bad = deepcopy(good["list"]); row = bad["data"]["installations"][0]
                    (row if field in ("source", "name", "installation_id") else row["clients"][0]["package_revision"])[field] = "foreign"
                    self.assertEqual(selected(bad), [])
            for field in ("candidateId", "destinationPath", "targetId", "alias", "status"):
                with self.subTest(receipt_field=field):
                    bad = deepcopy(receipt); bad["items"][0][field] = "foreign"
                    self.assertEqual(selected(good["list"], bad), [])
            lock.write_text(json.dumps({"packages": [{"alias": item["name"], **revision,
                                                       "tree_digest": "sha256:" + "f" * 64}]}))
            self.assertEqual(selected(good["list"]), [])

    def test_grok_coordinated_valid_looking_identity_substitutions_are_refused(self):
        match = run_path(str(ROOT / "scripts/live-install-audit.py"))["grok_prepared_skill_installations"]
        item = FIXTURES["skills"][0]
        for mutation in ("candidate", "synthetic_name", "installation_uuid", "physical_suffix"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temp:
                project = Path(temp).resolve()
                responses = envelopes("skill", item["name"], "grok", item, project)
                if mutation == "candidate":
                    for phase in ("install", "repeat_add"):
                        responses[phase]["items"][0]["candidateId"] = "sha256:" + "f" * 64
                elif mutation == "synthetic_name":
                    for phase in ("list", "repeat_list"):
                        node = responses[phase]["data"]["installations"][0]
                        node["name"] = node["name"][:51] + "c" * 11
                elif mutation == "installation_uuid":
                    changed_uuid = "11111111-2222-3333-4444-555555555555"
                    for phase in ("list", "repeat_list"):
                        responses[phase]["data"]["installations"][0]["installation_id"] = changed_uuid
                    for field in ("actions", "results"):
                        responses["cleanup"][field][0]["installation_id"] = changed_uuid
                else:
                    for phase in ("install", "repeat_add"):
                        row = responses[phase]["items"][0]
                        row["destinationPath"] = row["destinationPath"][:-12] + "f" * 12
                bundle = Path(responses["install"]["items"][0]["destinationPath"])
                skill = bundle / "skills" / item["name"] / "SKILL.md"
                skill.parent.mkdir(parents=True); skill.write_text("owned fixture")
                node = responses["list"]["data"]["installations"][0]
                revision = node["clients"][0]["package_revision"]
                (project / "uap.lock").write_text(json.dumps({"packages": [{"alias": item["name"], **revision}]}))
                selections = [match(responses[inventory], item, str(skill.relative_to(project)), project,
                                    {"stdout_tail": "unparseable bounded tail"}, repeated=repeated,
                                    full_receipt=responses[receipt])
                              for inventory, receipt, repeated in (("list", "install", False),
                                                                   ("repeat_list", "repeat_add", True))]
                # The legacy source matcher is necessary but does not prove
                # deterministic candidate/name/UUID-to-physical identity.
                self.assertEqual([len(selected) for selected in selections], [1, 1])
                proof = AUDIT["observe"]("skill", item["name"], responses, *selections,
                                         target="grok", skill_fixture=item)
                self.assertEqual(proof["status"], "unqualified")

    def test_grok_observer_requires_the_selected_immutable_skill_fixture(self):
        item = FIXTURES["skills"][0]
        responses = envelopes("skill", item["name"], "grok", item)
        selections = [responses[phase]["data"]["installations"] for phase in ("list", "repeat_list")]
        for fixture in (None, {}, {**item, "name": "foreign"}, {**item, "repo": "foreign/repo"},
                        {**item, "ref": "b" * 40}, {**item, "ref": None},
                        {**item, "folder": "foreign"}):
            with self.subTest(fixture=fixture):
                proof = AUDIT["observe"]("skill", item["name"], responses, *selections,
                                         target="grok", skill_fixture=fixture)
                self.assertEqual(proof["status"], "unqualified")

    def test_complete_preparation_preserves_zero_native_claims_and_original_rows(self):
        for kind in ("plugin", "skill"):
            with self.subTest(kind=kind):
                value = report(kind)
                before = deepcopy(value)
                proof = self.summary(value)
                self.assertTrue(proof["installation_acceptance_passed"])
                self.assertEqual(proof["installation_passed"], 30)
                self.assertTrue(all(flag is False for key, flag in proof.items() if key.endswith("_qualified")))
                self.assertEqual(value, before)
                self.assertEqual(value["passed"], 0)

    def test_grok_failed_first_add_preserves_original_result_without_repeat(self):
        runner = run_path(str(ROOT / "scripts/live-install-audit.py"))
        do_case = runner["do_case"]
        item = deepcopy(FIXTURES["skills"][0])
        calls = []

        def failed_install(argv, env, cwd, timeout=105, **options):
            calls.append(list(argv))
            self.assertTrue(options.get("capture_full"))
            if argv[1:3] == ["skills", "add"]:
                self.assertEqual(len(calls), 1)
                return {"exit_code": 1, "timeout": False, "stdout_tail": "",
                        "stderr_tail": "source refused\n", "stdout_full": "",
                        "stderr_full": "source refused\n", "_json": None}
            self.assertEqual(argv[1], "list")
            self.assertEqual(len(calls), 2)
            document = {"schema_version": 1, "command": "list", "result": "success",
                        "data": {"installations": []}}
            output = json.dumps(document)
            return {"exit_code": 0, "timeout": False, "stdout_tail": output,
                    "stderr_tail": "", "stdout_full": output, "stderr_full": "",
                    "_json": document}

        with tempfile.TemporaryDirectory() as temp, mock.patch.dict(do_case.__globals__, {
                "call": failed_install, "initialize_project_skill_git_root": mock.Mock()}), \
                mock.patch.object(runner["subprocess"], "run", side_effect=AssertionError("unexpected child")):
            row = do_case("/fixture/agentplugins", "grok", "skill", item, 0,
                          Path(temp).resolve() / "run", {}, {},
                          installation_audit="grok-prepared-skills-installation")
        self.assertEqual(len(calls), 2)
        self.assertEqual(row["status"], "install-failed")
        self.assertEqual(row["install"]["exit_code"], 1)
        self.assertEqual(row["grok_preparation_envelopes"], {"status": "unqualified"})
        self.assertEqual(row["repeat_installation_ids"], [])
        self.assertIsNone(row["repeat_add"])
        self.assertIsNone(row["cleanup"])
        self.assertTrue(row["sentinels_preserved"])

    def test_cursor_repeat_skill_tree_changes_refuse_proof_and_still_remove(self):
        runner = run_path(str(ROOT / "scripts/live-install-audit.py"))
        do_case = runner["do_case"]
        item = deepcopy(FIXTURES["skills"][0])
        body = ("---\nname: " + item["name"] + "\ndescription: Inert immutable upstream fixture\n---\nPinned test bytes.\n").encode()
        item["skill_md_git_blob_sha1"] = hashlib.sha1(
            b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()
        for mutation in (None, "initial-list", "repeat-add", "repeat-list", "rewrite-restore",
                         "empty-repeat-add", "empty-repeat-list", "unavailable-repeat-add"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as temp:
                calls = []
                def invoke(argv, env, cwd, timeout=105, **options):
                    phase = AUDIT["COMMANDS"][len(calls)]
                    calls.append(phase)
                    documents = envelopes("skill", item["name"], "cursor", item, cwd)
                    for inventory in ("list", "repeat_list"):
                        documents[inventory]["data"]["installations"][0]["source"] = item["repo"] + "/skills/" + item["folder"]
                    skill = cwd / ".cursor/skills" / item["name"] / "SKILL.md"
                    extra = cwd / ".cursor/skills/unknown-empty"
                    if phase == "install":
                        skill.parent.mkdir(); skill.write_bytes(body)
                        (cwd / "uap.toml").write_text("# inert saved agreement\n")
                        (cwd / "uap.lock").write_text(json.dumps({"packages": [{
                            "alias": item["name"], "targets": ["cursor"],
                            "tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "b" * 64,
                            "source": {"git": {"kind": "skill", "skill": item["name"],
                                "repository": item["repo"], "commit": item["ref"],
                                "path": "skills/" + item["folder"]}}}]}))
                    if ((phase == "list" and mutation == "initial-list")
                            or (phase == "repeat_add" and mutation in ("repeat-add", "rewrite-restore"))
                            or (phase == "repeat_list" and mutation == "repeat-list")):
                        skill.write_bytes(b"rewritten project Skill\n")
                    if ((phase == "repeat_add" and mutation == "initial-list")
                            or (phase == "repeat_list" and mutation == "rewrite-restore")):
                        skill.write_bytes(body)
                    if ((phase == "repeat_add" and mutation == "empty-repeat-add")
                            or (phase == "repeat_list" and mutation == "empty-repeat-list")):
                        extra.mkdir()
                    if phase == "repeat_list" and mutation == "empty-repeat-add": extra.rmdir()
                    if phase == "cleanup":
                        self.assertEqual(argv[1:3], ["remove", item["name"]])
                        skill.unlink(); skill.parent.rmdir()
                        if extra.exists(): extra.rmdir()
                        (cwd / "uap.lock").unlink()
                    document = documents[phase]
                    output = json.dumps(document)
                    return {"exit_code": 0, "timeout": False, "stdout_tail": output[-1800:],
                            "stderr_tail": "", "stdout_full": output, "stderr_full": "", "_json": document}
                original_snapshot = runner["scoped_snapshot"]
                def skill_snapshot(roots):
                    if mutation == "unavailable-repeat-add" and calls[-1] == "repeat_add":
                        raise AssertionError("injected unavailable Skill-tree comparison")
                    return original_snapshot(roots)
                with mock.patch.dict(do_case.__globals__, {"call": invoke, "AGENT_BINARIES": {},
                        "scoped_snapshot": skill_snapshot,
                        "initialize_project_skill_git_root": mock.Mock()}), \
                     mock.patch.object(runner["subprocess"], "run", side_effect=AssertionError("unexpected child")):
                    row = do_case("/fixture/agentplugins", "cursor", "skill", item, 0,
                                  Path(temp).resolve() / "run", {}, {},
                                  installation_audit="cursor-prepared-skills-installation")
                self.assertEqual(calls, list(AUDIT["COMMANDS"]))
                self.assertTrue(row["skill_bytes_match"])
                self.assertTrue(row["repeat_native_unchanged"])
                self.assertTrue(row["repeat_lock_unchanged"])
                self.assertTrue(row["removal_verified"])
                self.assertEqual(row["post_skill_tree_delta"], [])
                self.assertIsNone(row["repeat_project_unchanged"])
                self.assertEqual(row["cursor_preparation_envelopes"]["status"], "verified")
                self.assertIs(row["cursor_repeat_project_unchanged"], mutation is None)
                self.assertIs(row["repeat_verified"], mutation is None)
                self.assertEqual(row["status"], "incomplete-native-proof" if mutation is None else "repeat-add-unqualified")
                value = report("skill")
                value["results"][0] = row
                self.assertIs(self.summary(value)["installation_acceptance_passed"], mutation is None)

    def test_main_reports_resolved_executed_cli_and_keeps_unexecuted_input(self):
        main = run_path(str(ROOT / "scripts/live-install-audit.py"))["main"]
        windsurf = main.__globals__["WINDSURF_INSTALLATION"]
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            cli = root / "agentplugins"; cli.write_text("inert test artifact; never executed")
            for spelling, bootstrap in (("agentplugins", True), (str(cli), True), ("missing-relative-cli", False)):
                with self.subTest(cli=spelling, bootstrap=bootstrap):
                    calls, probes = [], []
                    destination = root / ("relative" if spelling == "agentplugins" else "absolute" if bootstrap else "blocked")
                    def case(*args, **kwargs):
                        calls.append(args[0])
                        return {"index": args[4], "target": "windsurf", "kind": "plugin",
                                "source": args[3], "status": "incomplete-native-proof"}
                    def version(argv, *_):
                        probes.append(argv)
                        return {"status": "reported", "exit_code": 0,
                                "value": windsurf["CLI_VERSION"] if argv[1] == "version" else windsurf["CLIENT_VERSION"]}
                    substitutions = {"do_case": case, "vendor_bootstrap_attested": lambda: bootstrap,
                        "version_evidence": version, "AGENT_BINARIES": {"devin": "/fixture/devin"},
                        "isolated_client_env": lambda *args: {}, "published_audit_temp_parent": lambda: None}
                    argv = ["audit", "--cli", spelling, "--kind", "plugin", "--target", "windsurf",
                            "--limit", "30", "--report-dir", str(destination), "--acceptance", windsurf["ACCEPTANCE"]]
                    with mock.patch.dict(main.__globals__, substitutions), mock.patch("sys.argv", argv), \
                         mock.patch("sys.platform", "linux"), mock.patch("platform.machine", return_value="x86_64"), \
                         contextlib.chdir(root), contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
                        self.assertEqual(main(), 1 if bootstrap else 2)
                    written = json.loads((destination / "windsurf-plugin.json").read_text())
                    self.assertFalse(written["installation_acceptance_passed"])
                    if bootstrap:
                        self.assertEqual(calls, [str(cli)] * 30)
                        self.assertEqual(probes[0][0], str(cli))
                        self.assertEqual(written["cli"], str(cli))
                        self.assertEqual(written["passed"], 0)
                    else:
                        self.assertEqual(written["cli"], spelling)
                        self.assertEqual(written["cli_version"], {"status": "not-probed"})
                        self.assertEqual(calls, []); self.assertEqual(probes, [])

    def test_grok_explicit_main_mode_keeps_default_failure_and_original_prepared_rows(self):
        main = run_path(str(ROOT / "scripts/live-install-audit.py"))["main"]
        value = report("skill", "grok")
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            cli = root / "agentplugins"; cli.write_text("fixture")
            calls = []
            def case(*args, **kwargs):
                calls.append(kwargs)
                return deepcopy(value["results"][args[4]])
            substitutions = {"do_case": case, "vendor_bootstrap_attested": lambda: True,
                             "version_evidence": lambda argv, *_: deepcopy(
                                 value["cli_version"] if argv[1] == "version" else value["client_version"]),
                             "AGENT_BINARIES": {"grok": "/fixture/grok"},
                             "isolated_client_env": lambda *args: {}, "published_audit_temp_parent": lambda: None}
            for mode, expected_code in (("native", 1), ("grok-prepared-skills-installation", 0)):
                calls.clear()
                argv = ["audit", "--cli", str(cli), "--kind", "skill", "--target", "grok",
                        "--limit", "30", "--report-dir", str(root / mode), "--acceptance", mode]
                with mock.patch.dict(main.__globals__, substitutions), mock.patch("sys.argv", argv), \
                     mock.patch("sys.platform", "linux"), mock.patch("platform.machine", return_value="x86_64"), \
                     contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(main(), expected_code)
                written = json.loads((root / mode / "grok-skill.json").read_text())
                self.assertEqual(written["passed"], 0)
                self.assertEqual(written["prepared_lifecycle_passed"], 30)
                self.assertEqual(len(calls), 30)
                self.assertTrue(all(call == ({} if mode == "native" else {"installation_audit": mode}) for call in calls))
                self.assertEqual(written["results"], value["results"])
                if mode != "native":
                    self.assertTrue(written["installation_acceptance_passed"])
                    self.assertFalse(written["native_activation_qualified"])

    def test_grok_explicit_main_mode_rejects_unapproved_routes_before_execution(self):
        main = run_path(str(ROOT / "scripts/live-install-audit.py"))["main"]
        for target, kind, limit, host, arch in (("cursor", "skill", 30, "linux", "x86_64"),
                                               ("grok", "plugin", 30, "linux", "x86_64"),
                                               ("grok", "skill", 2, "linux", "x86_64"),
                                               ("grok", "skill", 30, "darwin", "arm64"),
                                               ("grok", "skill", 30, "linux", "arm64")):
            argv = ["audit", "--cli", "/not-used", "--target", target, "--kind", kind, "--limit", str(limit),
                    "--report-dir", "/not-used", "--acceptance", "grok-prepared-skills-installation"]
            with self.subTest(route=(target, kind, limit, host, arch)), mock.patch("sys.argv", argv), \
                 mock.patch("sys.platform", host), mock.patch("platform.machine", return_value=arch), \
                 contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
                main()
            self.assertEqual(raised.exception.code, 2)

    def test_incomplete_wrong_source_or_promoted_native_reports_are_refused(self):
        mutations = [lambda d: d.update(target="qoder"), lambda d: d.update(platform="darwin"),
                     lambda d: d.update(architecture="arm64"), lambda d: d.update(cases_attempted=29),
                     lambda d: d.update(full_coverage=False), lambda d: d.update(passed=1),
                     lambda d: d.update(passed=False), lambda d: d.update(verified_negative_coverage=1),
                     lambda d: d["client_version"].update(value="latest"),
                     lambda d: d["cli_version"].update(exit_code=False),
                     lambda d: d["results"].pop(), lambda d: d["results"].append(deepcopy(d["results"][0])),
                     lambda d: d["results"][0].update(index=False),
                     lambda d: d["results"][0].update(source="github:foreign/source@" + "a" * 40),
                     lambda d: d["results"][0].update(identity="foreign")]
        for kind in ("plugin", "skill"):
            for index, mutate in enumerate(mutations):
                with self.subTest(kind=kind, mutation=index):
                    value = report(kind); mutate(value)
                    self.assertFalse(self.summary(value)["installation_acceptance_passed"])

    def test_no_receipt_cleanup_source_or_foreign_state_requirement_can_be_dropped(self):
        mutations = [lambda r: r.update(cursor_preparation_envelopes={"status": "unqualified"}),
                     lambda r: r["cursor_preparation_envelopes"].update(scope="foreign"),
                     lambda r: r.update(repeat_installation_ids=["reminted"]),
                     lambda r: r.update(native_residuals=["owned residue"]),
                     lambda r: r.update(sentinels_preserved=False),
                     lambda r: r.update(repeat_lock_unchanged=False),
                     lambda r: r.update(repeat_native_unchanged=False),
                     lambda r: r.update(removal_verified=False),
                     lambda r: r.update(post_remove_identity_registered=True),
                     lambda r: r["public_target_states"]["after-add"].update(truncated=True),
                     lambda r: r["public_target_states"]["after-add"].update(selected_bindings=True),
                     lambda r: r["public_target_states"]["after-add"]["states"][0].update(activation="active")]
        for phase in AUDIT["COMMANDS"]:
            mutations += [lambda r, phase=phase: r[phase].update(exit_code=1),
                          lambda r, phase=phase: r[phase].update(timeout=True)]
        for kind in ("plugin", "skill"):
            for index, mutate in enumerate(mutations):
                with self.subTest(kind=kind, mutation=index):
                    value = report(kind); mutate(value["results"][0])
                    self.assertFalse(self.summary(value)["installation_acceptance_passed"])
        for field in ("resolved_revision", "tree_digest", "manifest_digest", "recorded_distribution"):
            value = report("plugin"); value["results"][0]["signed_release_observation"][0][field] = "foreign"
            self.assertFalse(self.summary(value)["installation_acceptance_passed"])
        for field, wrong in (("lock_exact_source_pinned", False), ("skill_bytes_match", False),
                             ("skill_path", ".cursor/skills/foreign/SKILL.md"),
                             ("post_skill_tree_delta", ["foreign"]), ("post_lock_pinned", True)):
            value = report("skill"); value["results"][0][field] = wrong
            self.assertFalse(self.summary(value)["installation_acceptance_passed"])

    def test_marketing_repeat_ids_remain_bound_to_their_original_aliases(self):
        good = envelopes("plugin", "marketing-kit")
        self.assertEqual(observe("plugin", "marketing-kit", good)["status"], "verified")
        changed = deepcopy(good)
        actions = changed["repeat_add"]["data"]["actions"]
        actions[0]["installation_id"], actions[1]["installation_id"] = (
            actions[1]["installation_id"], actions[0]["installation_id"])
        self.assertEqual(observe("plugin", "marketing-kit", changed)["status"], "unqualified")
        changed = deepcopy(good)
        rows = changed["repeat_list"]["data"]["installations"]
        rows[1]["installation_id"], rows[2]["installation_id"] = (
            rows[2]["installation_id"], rows[1]["installation_id"])
        self.assertEqual(observe("plugin", "marketing-kit", changed)["status"], "unqualified")

    def test_complete_raw_envelopes_and_full_marketing_closure_are_required(self):
        for kind, identity in (("skill", "frontend-design"), ("plugin", "atlassian"), ("plugin", "marketing-kit")):
            good = envelopes(kind, identity)
            self.assertEqual(observe(kind, identity, good)["status"], "verified")
            mutations = [lambda d: d["cleanup"].update(outcome="failed"),
                         lambda d: d["cleanup"].update(scope="foreign"),
                         lambda d: d["cleanup"]["results"].pop(),
                         lambda d: d["cleanup"]["actions"].pop(),
                         lambda d: d["cleanup"]["results"][0].update(installation_id="reminted"),
                         lambda d: d["post_remove_list"]["data"]["installations"].append({"residue": True}),
                         lambda d: d["list"]["data"]["installations"].append({"foreign": True}),
                         lambda d: d["list"]["data"]["installations"][0]["clients"].append({"client_id": "codex"}),
                         lambda d: d["repeat_list"]["data"]["installations"][0]["clients"][0].update(scope="foreign")]
            for phase in AUDIT["COMMANDS"]:
                mutations += [lambda d, phase=phase: d[phase].update(result="failure"),
                              lambda d, phase=phase: d.update({phase: None})]
            if kind == "skill":
                mutations += [lambda d: d["install"].update(readiness="ready"),
                              lambda d: d["repeat_add"]["items"][0].update(status="succeeded"),
                              lambda d: d["install"]["next_actions"][0].update(actor="user")]
            else:
                mutations += [lambda d: d["install"]["data"]["results"][0].update(outcome="failed"),
                              lambda d: d["repeat_add"]["data"]["actions"][0].update(installation_id="foreign"),
                              lambda d: d["repeat_add"]["data"]["actions"][0].update(alias=[])]
            for index, mutate in enumerate(mutations):
                with self.subTest(kind=kind, identity=identity, mutation=index):
                    value = deepcopy(good); mutate(value)
                    self.assertEqual(observe(kind, identity, value)["status"], "unqualified")

    def test_every_marketing_child_is_matched_to_immutable_signed_source_in_both_inventories(self):
        runner = run_path(str(ROOT / "scripts/live-install-audit.py"))
        response = envelopes("plugin", "marketing-kit")["list"]
        closure = runner["MARKETING_KIT_SIGNED_CLOSURE"]
        distributions = runner["MARKETING_KIT_DISTRIBUTIONS"]
        for node in response["data"]["installations"]:
            distribution = distributions[node["name"]]
            pin = closure[distribution]
            node["source"] = pin["repository"] + "/" + pin["package_path"]
            node["directory"] = {"product_id": node["name"], "recorded_distribution": distribution,
                                 "recorded_revision": pin["resolved_revision"],
                                 "recorded_release_sequence": pin["release_sequence"]}
            node["clients"][0]["package_revision"] = {
                "distribution_id": distribution, "resolved_revision": pin["resolved_revision"],
                "release_sequence": pin["release_sequence"], "tree_digest": pin["tree_digest"],
                "manifest_digest": pin["manifest_digest"]}
        match = runner["cursor_preparation_source_matches"]
        self.assertEqual(len(match(response, "marketing-kit", "plugin", "marketing-kit",
                                   FIXTURES["signed_release_pins"])), 4)
        for member in ("marketing-kit", "notion", "hubspot-crm", "playwright"):
            for field in ("source", "resolved_revision", "tree_digest", "manifest_digest", "distribution_id", "release_sequence"):
                with self.subTest(member=member, field=field):
                    changed = deepcopy(response)
                    node = next(row for row in changed["data"]["installations"] if row["name"] == member)
                    if field == "source":
                        node[field] = "foreign/unsigned/replacement"
                    else:
                        node["clients"][0]["package_revision"][field] = True if field == "release_sequence" else "foreign"
                    self.assertEqual(match(changed, "marketing-kit", "plugin", "marketing-kit",
                                           FIXTURES["signed_release_pins"]), [])

    def test_explicit_main_mode_preserves_default_failure_and_original_native_counts(self):
        runner = run_path(str(ROOT / "scripts/live-install-audit.py"))
        main = runner["main"]
        for kind, executable, acceptance in (
                ("skill", "cursor-agent", "cursor-prepared-skills-installation"),
                ("plugin", "cursor", "cursor-prepared-installation")):
            with self.subTest(kind=kind):
                self.check_explicit_cursor_mode(main, kind, executable, acceptance)

    def test_explicit_main_mode_rejects_wrong_route_before_any_execution(self):
        main = run_path(str(ROOT / "scripts/live-install-audit.py"))["main"]
        for target, kind, limit, host, arch in (("qoder", "skill", 30, "linux", "x86_64"),
                                               ("cursor", "plugin", 30, "linux", "x86_64"),
                                               ("cursor", "skill", 2, "linux", "x86_64"),
                                               ("cursor", "skill", 30, "darwin", "arm64")):
            argv = ["audit", "--cli", "/not-used", "--target", target, "--kind", kind, "--limit", str(limit),
                    "--report-dir", "/not-used", "--acceptance", "cursor-prepared-skills-installation"]
            with self.subTest(route=(target, kind, limit, host, arch)), mock.patch("sys.argv", argv), \
                 mock.patch("sys.platform", host), mock.patch("platform.machine", return_value=arch), \
                 contextlib.redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
                main()
            self.assertEqual(raised.exception.code, 2)

    def test_bounded_complete_capture_rejects_duplicate_and_nonfinite_json(self):
        call = run_path(str(ROOT / "scripts/live-install-audit.py"))["call"]
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            env = {"TEMP": str(root), "HOME": str(root)}
            for raw in ('{"result":"failure","result":"success"}', '{"value":NaN}', '{"value":Infinity}'):
                with self.subTest(raw=raw):
                    result = call([sys.executable, "-c", "import sys;sys.stdout.write(" + repr(raw) + ")"],
                                  env, root, 30, capture_full=True)
                    self.assertEqual(result["exit_code"], 0)
                    self.assertIsNone(result["_json"])
            raw = json.dumps({"result": "success", "large": "x" * 4000})
            result = call([sys.executable, "-c", "import sys;sys.stdout.write(" + repr(raw) + ")"],
                          env, root, 30, capture_full=True)
            self.assertEqual(result["_json"], json.loads(raw))
            self.assertEqual(len(result["stdout_tail"]), 1800)
            self.assertEqual(result["stdout_full"], raw)
            result = call([sys.executable, "-c", "print('x' * ((1 << 20) + 1))"],
                          env, root, 30, capture_full=True)
            self.assertIsNone(result["_json"])
            self.assertIsNone(result["exit_code"])

    def check_explicit_cursor_mode(self, main, kind, executable, acceptance):
        value = report(kind)
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve()
            cli = root / "agentplugins"; cli.write_text("fixture")
            def version(argv, *_):
                return deepcopy(value["cli_version"] if argv[1] == "version" else value["client_version"])
            calls = []
            def case(*args, **kwargs):
                calls.append(kwargs)
                return deepcopy(value["results"][args[4]])
            substitutions = {"do_case": case,
                             "version_evidence": version, "vendor_bootstrap_attested": lambda: True,
                             "AGENT_BINARIES": {executable: "/fixture/" + executable},
                             "isolated_client_env": lambda *args: {}, "published_audit_temp_parent": lambda: None}
            for mode, expected_code in (("native", 1), (acceptance, 0)):
                calls.clear()
                argv = ["audit", "--cli", str(cli), "--kind", kind, "--target", "cursor",
                        "--limit", "30", "--report-dir", str(root / mode), "--acceptance", mode]
                with mock.patch.dict(main.__globals__, substitutions), mock.patch("sys.argv", argv), \
                     mock.patch("sys.platform", "linux"), mock.patch("platform.machine", return_value="x86_64"), \
                     contextlib.redirect_stdout(io.StringIO()):
                    self.assertEqual(main(), expected_code)
                written = json.loads((root / mode / f"cursor-{kind}.json").read_text())
                self.assertEqual(written["passed"], 0)
                self.assertEqual(len(calls), 30)
                self.assertTrue(all(call == ({} if mode == "native" else {"installation_audit": mode}) for call in calls))
                self.assertEqual(written["results"], value["results"])
                self.assertEqual(written["prepared_lifecycle_passed"], value["prepared_lifecycle_passed"])
                if mode != "native":
                    self.assertTrue(written["installation_acceptance_passed"])
                    self.assertFalse(written["native_activation_qualified"])


if __name__ == "__main__":
    unittest.main()
