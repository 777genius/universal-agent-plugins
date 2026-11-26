"""Inert filesystem/public-call tests; no Kiro discovery or runtime claim."""
from copy import deepcopy
from contextlib import ExitStack, redirect_stdout, redirect_stderr
import hashlib
import io
import json
import os
from pathlib import Path
from runpy import run_path
import shutil
import sys
import tempfile
import unittest
from unittest.mock import patch
import uuid

import prepared_installation_audit as prepared
from test_prepared_installation_audit import envelopes as existing_envelopes

ROOT = Path(__file__).resolve().parents[1]
FIXTURES = json.loads((ROOT / "tests/live-install/fixtures.json").read_text())


def skill_body(name):
    return f"---\nname: {name}\ndescription: inert exact-byte seam\n---\n".encode()


# Explicit inert byte authorities exercise held-read/source binding. They do
# not supply evidence for the immutable production fixtures or native Kiro.
for fixture in FIXTURES["skills"]:
    body = skill_body(fixture["name"])
    fixture["skill_md_git_blob_sha1"] = hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()


def raw(kind, index, project):
    item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
    identity = item["name"] if kind == "skill" else FIXTURES["plugin_declared_names"].get(item, item)
    if kind == "skill":
        value = existing_envelopes(kind, identity, "grok", item, str(project))
        node = value["list"]["data"]["installations"][0]
        derived = prepared.grok_skill_identity(item, node["installation_id"])
        for phase in ("install", "repeat_add"):
            value[phase]["items"][0].update(targetId="kiro", destinationPath=str(project / ".uap/managed/clients/kiro" / derived[2]))
    else:
        value = existing_envelopes(kind, identity)
        node = value["list"]["data"]["installations"][0]
        node["installation_id"] = str(uuid.uuid5(uuid.NAMESPACE_URL, item))
        if item.startswith("github:"):
            node["source"] = item[len("github:"):].rsplit("@", 1)[0].lower()
            revision = {"resolved_revision": item.rsplit("@", 1)[1],
                        "tree_digest": "sha256:" + "a" * 64, "manifest_digest": "sha256:" + "b" * 64}
        else:
            pin = FIXTURES["signed_release_pins"][item]
            node["source"] = pin["repository"] + "/" + pin["path"]
            node["directory"] = {"product_id": "github" if item == "777genius/github-bridge" else item,
                "recorded_distribution": pin["distribution"], "recorded_revision": pin["revision"],
                "recorded_release_sequence": pin["sequence"]}
            revision = {"distribution_id": pin["distribution"], "resolved_revision": pin["revision"],
                        "release_sequence": pin["sequence"], "tree_digest": pin["tree_digest"],
                        "manifest_digest": pin["manifest_digest"]}
        node["clients"][0]["package_revision"] = revision
        for phase in ("install", "repeat_add"):
            data = value[phase]["data"]
            data["environment"]["selected_targets"] = ["kiro"]
            action = data["actions"][0]
            action.update(client_id="kiro", installation_id=node["installation_id"],
                          package_digest=revision["tree_digest"], revision=revision["resolved_revision"])
            if phase == "install":
                action["component_support"] = [{"kind": "skill" if index in prepared.KIRO_ACTIVE else "mcp_server",
                                                "name": identity, "support": "native"}]
            else:
                action.update(from_digest=revision["tree_digest"], from_revision=revision["resolved_revision"])
            for result in data["results"]:
                result.update(client_id="kiro", installation_id=node["installation_id"])
    state = prepared.expected_state(kind, "kiro")
    if kind == "plugin" and index in prepared.KIRO_ACTIVE:
        state.update(activation="active", verification="installation_verified")
    node["clients"][0].update(client_id="kiro", **state)
    value["repeat_list"] = deepcopy(value["list"])
    for field in ("actions", "results"):
        for row in value["cleanup"][field]:
            row.update(client_id="kiro", installation_id=node["installation_id"])
    return value


def write_projection(observer, kind, index, documents):
    """Create inert existing state/projection receipts, never compile a source."""
    item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
    node = documents["list"]["data"]["installations"][0]
    client = node["clients"][0]
    state_root = observer.project / ".uap" if kind == "skill" else observer.data
    profile = observer.project / ".kiro" if kind == "skill" else observer.home / ".kiro"
    physical = node["name"][:51].rstrip(".-") + "-" + hashlib.sha256(node["installation_id"].encode()).hexdigest()[:12]
    active = state_root / "managed/clients/kiro" / physical
    active.mkdir(parents=True, exist_ok=True)
    objects = [{"kind": "managed_package_directory", "object_id": "package:kiro:" + physical,
                "path": str(active), "protection_class": "managed"}]
    components = ([{"kind": "skill", "name": item["name"], "support": "native"}] if kind == "skill"
                  else documents["install"]["data"]["actions"][0]["component_support"])
    projection = {"mcpServers": {}}
    for component in components:
        if component["support"] != "native": continue
        name = component["name"]
        if component["kind"] == "skill":
            relative = "skills/" + name
            (active / relative).mkdir(parents=True, exist_ok=True)
            (active / relative / "SKILL.md").write_bytes(skill_body(name))
            objects.append({"kind": "kiro_global_skill_directory", "object_id": "kiro-skill:" + name,
                "logical_name": name, "source_relative": relative, "path": str(profile / "skills" / name),
                "managed_digest": "sha256:" + "a" * 64, "protection_class": "managed"})
        else:
            projection["mcpServers"][name] = {"command": "inert"}
            objects.append({"kind": "kiro_global_mcp_server", "object_id": "kiro-mcp:" + name,
                "logical_name": name, "path": str(profile / "settings/mcp.json"),
                "managed_digest": "sha256:" + "b" * 64, "protection_class": "managed"})
    (active / "mcp.json").write_text(json.dumps(projection))
    if projection["mcpServers"] and kind == "plugin":
        # Model the default native writer's persistent coordination file.
        lock = profile / "settings/mcp.json.agentplugins.lock"
        fd = os.open(lock, os.O_CREAT | os.O_RDWR | os.O_NOFOLLOW, 0o600)
        os.close(fd)
    repository = (item["repo"] if kind == "skill" else item[len("github:"):].rsplit("@", 1)[0]
                  if item.startswith("github:") else FIXTURES["signed_release_pins"][item]["repository"])
    subpath = ("skills/" + item["folder"] if kind == "skill" else "" if item.startswith("github:")
               else FIXTURES["signed_release_pins"][item]["path"])
    record = {"installation_id": node["installation_id"], "declared_name": node["name"],
              "source": {"repository": repository, "package_subpath": subpath,
                  "resolved_revision": client["package_revision"]["resolved_revision"], "tree_digest": client["package_revision"]["tree_digest"]},
              "package": {"manifest_digest": client["package_revision"]["manifest_digest"]},
              "clients": {"fixture-binding": {**deepcopy(client), "client_binding_id": "fixture-binding",
                  **({"native_profile_root": str(profile)} if kind == "skill" else {}),
                  "physical_artifact_id": physical,
                  "target_locator": str(active), "native_objects": objects}}}
    (state_root / "state-v2.json").write_text(json.dumps({"schema_version": 5, "installations": [record]}))


def clear_state(observer, kind):
    state = (observer.project / ".uap" if kind == "skill" else observer.data) / "state-v2.json"
    old = json.loads(state.read_bytes())
    for record in old["installations"]:
        for binding in record["clients"].values():
            shutil.rmtree(binding["target_locator"])
    state.write_text('{"schema_version":5,"installations":[]}')
    if kind == "skill":
        (observer.project / "uap.toml").write_text("schema_version = 1\n")
        (observer.project / "uap.lock").write_text(json.dumps({"lock_version": 1,
            "manifest_digest": "sha256:2110fb382f40916fd7f7b674ece50f3969d1ad918fd7e316b863085f3cef0613",
            "packages": []}))



class KiroInstallationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.audit = run_path(str(ROOT / "scripts/live-install-audit.py"))

    def fixture(self, root, kind):
        case = root / "case"
        for relative in ("home/.config", "home/.cache", "home/.local/share", "project", "uap-data"):
            (case / relative).mkdir(parents=True)
        env = self.audit["isolated_client_env"](case / "home", case, case / "uap-data")
        with patch.object(sys, "platform", "darwin"):
            observer = prepared.KiroInstallationAudit("/fixture/agentplugins", env, case / "project", case, self.audit, kind)
        self.addCleanup(observer.close)
        return observer

    @staticmethod
    def outcome(document, code=0, stderr=""):
        text = "" if document is None else json.dumps(document)
        return {"exit_code": code, "timeout": False, "_json": document,
                "stdout_full": text, "stderr_full": stderr,
                "stdout_tail": text[-1800:], "stderr_tail": stderr[-1800:]}

    def row(self, kind, index, documents):
        item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
        node = documents["list"]["data"]["installations"][0]
        identity = item["name"] if kind == "skill" else FIXTURES["plugin_declared_names"].get(item, item)
        row = {"index": index, "target": "kiro", "kind": kind, "source": item["source"] if kind == "skill" else item,
               "identity": identity, "status": "incomplete-native-proof" if kind == "skill" else
                   "package-installed-native-unqualified" if index in prepared.KIRO_ACTIVE else "prepared-lifecycle-passed",
               "matched_installation_ids": [node["installation_id"]], "repeat_installation_ids": [node["installation_id"]],
               "prepared_binding": kind == "plugin" and index not in prepared.KIRO_ACTIVE,
               "prepared_components_only": False, "public_skill_ready": {"after-add": False, "repeat-add": False} if kind == "skill" else None,
               "native_file_delta": 0 if kind == "skill" else 1, "native_identity_files": [] if kind == "skill" else [".kiro/skills/owned/SKILL.md"],
               "native_component_candidates": [], "signed_release_observation": [], "native_residuals": [],
               "post_removal_leftovers": [], "post_skill_tree_delta": [], "post_removal_skill": None,
               "post_lock_pinned": False, "repeat_project_unchanged": None,
               "skill_path": f'.kiro/skills/{identity}/SKILL.md' if kind == "skill" else None}
        for key in ("listed_identity", "repeat_verified", "repeat_native_unchanged", "repeat_lock_unchanged",
                    "removal_verified", "post_list_schema_valid", "sentinels_preserved", "lock_written", "lock_contains_identity",
                    "lock_exact_source_pinned", "skill_bytes_match"):
            row[key] = True
        for key in ("repeat_native_runtime_only", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered"):
            row[key] = False
        for phase in prepared.COMMANDS:
            row[phase] = {"exit_code": 0, "timeout": False}
        state = {k: node["clients"][0][k] for k in prepared.expected_state(kind, "kiro")}
        row["public_target_states"] = {phase: {"status": "observed", "selected_installations": int(phase != "after-remove"),
            "selected_bindings": int(phase != "after-remove"), "invalid_rows": 0,
            "states": [] if phase == "after-remove" else [state], "truncated": False}
            for phase in ("after-add", "repeat-add", "after-remove")}
        return row

    def sequence(self, observer, kind, index, documents):
        item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
        identity = item["name"] if kind == "skill" else FIXTURES["plugin_declared_names"].get(item, item)
        scope = ["--scope", "project", "--project-root", str(observer.project)] if kind == "skill" else ["--scope", "user"]
        add = ([observer.cli, "skills", "add", item["source"], "--skill", identity, "--target", "kiro", *scope,
                "--no-ui", "--yes", "--format", "json"] if kind == "skill" else
               [observer.cli, "add", item, "--target", "kiro", *scope, "--format", "json"])
        listed = [observer.cli, "list", "--target", "kiro", *scope, "--format", "json"]
        iid = documents["list"]["data"]["installations"][0]["installation_id"]
        return [(argv, self.outcome(documents[phase])) for argv, phase in zip(
            (add, listed, add, listed, [observer.cli, "remove", iid, "--target", "kiro", *scope, "--format", "json"], listed), prepared.COMMANDS)]

    def finish(self, observer, kind, index, documents, row=None):
        row = row or self.row(kind, index, documents)
        owned = (observer.project if kind == "skill" else observer.home) / ".kiro/skills" / row["identity"]
        baseline = observer.mcp.read_bytes()
        for position, (argv, outcome) in enumerate(self.sequence(observer, kind, index, documents)):
            if position == 0:
                if kind == "skill" or index in prepared.KIRO_ACTIVE:
                    owned.mkdir(); (owned / "SKILL.md").write_bytes(skill_body(row["identity"]))
                else:
                    body = json.loads(baseline); body["mcpServers"][row["identity"]] = {"command": "inert"}
                    temporary = observer.mcp.with_suffix(".new"); temporary.write_text(json.dumps(body))
                    temporary.chmod(observer.mcp.stat().st_mode & 0o777); temporary.replace(observer.mcp)
                write_projection(observer, kind, index, documents)
            if position == 4:
                if owned.exists(): shutil.rmtree(owned)
                if kind == "plugin" and index not in prepared.KIRO_ACTIVE:
                    temporary = observer.mcp.with_suffix(".new"); temporary.write_bytes(baseline)
                    temporary.chmod(observer.mcp.stat().st_mode & 0o777); temporary.replace(observer.mcp)
                clear_state(observer, kind)
            observer.observe(argv, outcome)
        item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
        return observer.finish(row, item, index, FIXTURES), row

    @unittest.skipUnless(hasattr(os, "mkfifo") and hasattr(os, "O_NONBLOCK"), "POSIX FIFO custody")
    def test_receipt_fifo_refuses_without_a_blocking_open(self):
        with tempfile.TemporaryDirectory() as temporary:
            observer = self.fixture(Path(temporary).resolve(), "skill")
            receipt = observer.project / "receipt.json"
            receipt.write_bytes(b"{}")
            self.assertEqual(observer.read_bytes(receipt), b"{}")
            receipt.unlink()
            os.mkfifo(receipt)
            real_open = os.open

            def checked_open(path, flags, *args, **kwargs):
                # Fail before opening on a regression; a real FIFO with no
                # writer would otherwise hang the entire test process.
                self.assertEqual(flags & (os.O_NOFOLLOW | os.O_NONBLOCK),
                                 os.O_NOFOLLOW | os.O_NONBLOCK)
                return real_open(path, flags, *args, **kwargs)

            with patch.object(prepared.os, "open", side_effect=checked_open):
                with self.assertRaises(ValueError):
                    observer.read_bytes(receipt)

    def test_saved_removal_requires_empty_held_project_receipts(self):
        for position, mutation in ((4, "missing"), (4, "malformed-lock"), (4, "manifest-schema"),
                                   (4, "residual-dependency"), (4, "residual-target"), (4, "foreign-lock"),
                                   (4, "symlink"), (4, "hardlink"), (5, "same-byte-replacement"),
                                   (5, "same-byte-rewrite"), (5, "missing"), (5, "foreign-lock")):
            with self.subTest(position=position, mutation=mutation), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "skill")
                original, calls = observer.observe, []
                manifest, lock = observer.project / "uap.toml", observer.project / "uap.lock"
                def adversarial(argv, outcome):
                    index = len(calls)
                    calls.append(index)
                    if index == position:
                        if mutation == "missing":
                            lock.unlink()
                        elif mutation == "malformed-lock":
                            lock.write_text("not a lock")
                        elif mutation == "manifest-schema":
                            manifest.write_text("schema_version = true")
                        elif mutation == "residual-dependency":
                            manifest.write_text('schema_version = 1\n[dependencies]\nforeign = "x"')
                        elif mutation == "residual-target":
                            manifest.write_text('schema_version = 1\n[targets]\nkiro = ["foreign"]')
                        elif mutation == "foreign-lock":
                            document = json.loads(lock.read_bytes())
                            document["packages"] = [{"alias": "foreign"}]
                            lock.write_text(json.dumps(document))
                        elif mutation == "symlink":
                            foreign = lock.with_suffix(".foreign")
                            lock.rename(foreign)
                            lock.symlink_to(foreign)
                        elif mutation == "hardlink":
                            os.link(lock, lock.with_suffix(".foreign"))
                        elif mutation == "same-byte-replacement":
                            replacement = lock.with_suffix(".new")
                            replacement.write_bytes(lock.read_bytes())
                            replacement.replace(lock)
                        elif mutation == "same-byte-rewrite":
                            info = lock.stat()
                            lock.write_bytes(lock.read_bytes())
                            os.utime(lock, ns=(info.st_atime_ns, info.st_mtime_ns))
                    return original(argv, outcome)
                observer.observe = adversarial
                with self.assertRaises((ValueError, OSError)):
                    self.finish(observer, "skill", 0, raw("skill", 0, observer.project))
                self.assertEqual(len(calls), position + 1)

    def test_all_sources_keep_exact_states_and_complete_six_envelopes(self):
        for kind in ("plugin", "skill"):
            for index in range(30):
                if kind == "plugin" and index == 13:
                    continue
                with self.subTest(kind=kind, index=index), tempfile.TemporaryDirectory() as directory:
                    observer = self.fixture(Path(directory).resolve(), kind)
                    documents = raw(kind, index, observer.project)
                    row = self.row(kind, index, documents)
                    original_status = row["status"]
                    proof, row = self.finish(observer, kind, index, documents, row)
                    self.assertEqual(proof["status"], "verified-installation-lifecycle")
                    self.assertEqual(row["status"], original_status)
                    self.assertEqual(len(proof["observations"]), 6)
                    self.assertFalse(proof["native_runtime_qualified"])
                    self.assertEqual(row["public_target_states"]["after-add"]["states"], [proof["public_state"]])

    def test_exact_signed_marketing_refusal_only(self):
        with tempfile.TemporaryDirectory() as directory:
            observer = self.fixture(Path(directory).resolve(), "plugin")
            diagnostic = ('agentplugins: no eligible directory release for "marketing-kit": '
                          '777genius/marketing-kit: release does not support complete target set; missing kiro\n')
            add = [observer.cli, "add", "marketing-kit", "--target", "kiro", "--scope", "user", "--format", "json"]
            listed = [observer.cli, "list", "--target", "kiro", "--scope", "user", "--format", "json"]
            observer.observe(add, self.outcome(None, 1, diagnostic))
            observer.observe(listed, self.outcome({"schema_version": 1, "command": "list", "result": "success", "data": {"installations": []}}))
            row = {"index": 13, "identity": "marketing-kit", "source": "marketing-kit", "target": "kiro", "kind": "plugin",
                   "status": "documented-or-source-unsupported", "sentinels_preserved": True, "native_residuals": [],
                   **{k: False for k in ("listed_identity", "failed_install_registered", "failed_install_residue", "post_remove_identity_registered")}}
            self.assertEqual(observer.finish(row, "marketing-kit", 13, FIXTURES)["status"], "verified-installation-negative")
            observer.observations[0][1]["stderr_full"] += "other failure\n"
            with self.assertRaises(ValueError):
                observer.finish(row, "marketing-kit", 13, FIXTURES)

    def test_direct_prepared_source_match_preserves_default_pinned_lifecycle(self):
        item = FIXTURES["plugin_aliases"][21]
        documents = raw("plugin", 21, Path("/fixture/project"))
        match = self.audit["matching_installations"]
        self.assertEqual(len(match(documents["list"], "sanity", "kiro", "plugin", item)), 1)
        for key, value in (("source", "other/repo"), ("installation_id", "not-uuid")):
            changed = deepcopy(documents["list"]); changed["data"]["installations"][0][key] = value
            self.assertEqual(match(changed, "sanity", "kiro", "plugin", item), [])
        for key in ("resolved_revision", "tree_digest", "manifest_digest"):
            changed = deepcopy(documents["list"]); changed["data"]["installations"][0]["clients"][0]["package_revision"][key] = "invalid"
            self.assertEqual(match(changed, "sanity", "kiro", "plugin", item), [])

    def test_installation_admission_does_not_reclassify_or_trust_native_status(self):
        for status in ("package-installed-native-unqualified", "installation-lifecycle-observed",
                       "passed", "incomplete-native-proof"):
            with self.subTest(status=status), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "plugin")
                documents = raw("plugin", 22, observer.project)
                row = self.row("plugin", 22, documents)
                row["status"] = status
                if status == "package-installed-native-unqualified":
                    proof, _ = self.finish(observer, "plugin", 22, documents, row)
                    self.assertEqual(proof["status"], "verified-installation-lifecycle")
                    self.assertIs(proof["native_runtime_qualified"], False)
                else:
                    with self.assertRaises(ValueError):
                        self.finish(observer, "plugin", 22, documents, row)
                self.assertEqual(row["status"], status)
        # The normal raw-Skill status is also insufficient without the six
        # actual public observations and held source/ownership projections.
        with tempfile.TemporaryDirectory() as directory:
            observer = self.fixture(Path(directory).resolve(), "skill")
            documents = raw("skill", 0, observer.project)
            row = self.row("skill", 0, documents)
            self.assertEqual(row["status"], "incomplete-native-proof")
            with self.assertRaises(ValueError):
                observer.finish(row, FIXTURES["skills"][0], 0, FIXTURES)

    def test_unknown_cache_foreign_bytes_mode_hardlink_and_parent_changes_refuse(self):
        for change in ("cache", "foreign", "mode", "hardlink", "symlink", "parent",
                       "data-mode", "data-parent", "wrong-scope"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "plugin")
                documents = raw("plugin", 0, observer.project)
                if change == "cache":
                    (observer.home / "unknown-native-cache").write_text("leftover")
                elif change == "foreign":
                    observer.mcp.write_text('{"mcpServers":{}}\n')
                elif change == "mode":
                    observer.mcp.chmod(0o644 if observer.mcp.stat().st_mode & 0o777 != 0o644 else 0o600)
                elif change == "hardlink":
                    os.link(observer.mcp, observer.home / "linked-mcp")
                elif change == "symlink":
                    old = observer.mcp.with_suffix(".old"); observer.mcp.rename(old); observer.mcp.symlink_to(old)
                elif change == "parent":
                    old = observer.mcp.parent.with_name("old-settings")
                    observer.mcp.parent.rename(old); observer.mcp.parent.mkdir(); shutil.copyfile(old / "mcp.json", observer.mcp)
                elif change == "data-mode":
                    observer.data.chmod(0o700 if observer.data.stat().st_mode & 0o777 != 0o700 else 0o755)
                elif change == "data-parent":
                    observer.data.rename(observer.data.with_name("old-data")); observer.data.mkdir()
                else:
                    (observer.project / ".kiro/settings/mcp.json").write_text('{"mcpServers":{"wrong":{}}}\n')
                with self.assertRaises(ValueError):
                    self.finish(observer, "plugin", 0, documents)

    def test_atomic_shared_replacement_requires_exact_returned_bytes_and_held_original(self):
        with tempfile.TemporaryDirectory() as directory:
            observer = self.fixture(Path(directory).resolve(), "plugin")
            documents = raw("plugin", 0, observer.project)
            row = self.row("plugin", 0, documents)
            sequence = self.sequence(observer, "plugin", 0, documents)
            baseline = observer.mcp.read_bytes()
            def replace(body):
                temporary = observer.mcp.with_suffix(".new"); temporary.write_bytes(body)
                temporary.chmod(observer.mcp.stat().st_mode & 0o777); temporary.replace(observer.mcp)
            installed = json.loads(baseline); installed["mcpServers"]["atlassian"] = {"command": "inert"}
            replace(json.dumps(installed).encode())
            write_projection(observer, "plugin", 0, documents)
            for argv, outcome in sequence[:4]: observer.observe(argv, outcome)
            replace(baseline)
            clear_state(observer, "plugin")
            for argv, outcome in sequence[4:]: observer.observe(argv, outcome)
            self.assertEqual(observer.finish(row, FIXTURES["plugin_aliases"][0], 0, FIXTURES)["status"], "verified-installation-lifecycle")
            (observer.home / "scratch").write_bytes(b"x")
            with self.assertRaises(ValueError): observer.finish(row, FIXTURES["plugin_aliases"][0], 0, FIXTURES)

    def test_source_candidate_name_scope_and_readiness_cannot_be_forged(self):
        mutations = [lambda d: d["install"].update(readiness="ready"),
                     lambda d: d["install"]["items"][0].update(candidateId="sha256:" + "c" * 64),
                     lambda d: d["list"]["data"]["installations"][0].update(name="skill-" + "c" * 56),
                     lambda d: d["repeat_list"]["data"]["installations"][0]["clients"][0].update(scope="user"),
                     lambda d: d["cleanup"]["results"][0].update(installation_id="reminted"),
                     lambda d: d["post_remove_list"]["data"]["installations"].append({"foreign": True})]
        for mutate in mutations:
            with self.subTest(mutation=mutate), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "skill")
                documents = raw("skill", 0, observer.project); mutate(documents)
                with self.assertRaises(ValueError): self.finish(observer, "skill", 0, documents)

    def test_owned_skill_bytes_and_shared_foreign_member_are_checked_in_held_reads(self):
        with tempfile.TemporaryDirectory() as directory:
            observer = self.fixture(Path(directory).resolve(), "skill")
            with patch(__name__ + ".skill_body", return_value=b"wrong upstream bytes"), self.assertRaises(ValueError):
                self.finish(observer, "skill", 0, raw("skill", 0, observer.project))
        with tempfile.TemporaryDirectory() as directory:
            observer = self.fixture(Path(directory).resolve(), "plugin")
            documents = raw("plugin", 0, observer.project)
            row = self.row("plugin", 0, documents)
            # Full successful envelopes without any actual owned native image
            # cannot establish installation, even when row booleans say true.
            with self.assertRaises(ValueError):
                for argv, outcome in self.sequence(observer, "plugin", 0, documents):
                    observer.observe(argv, outcome)
                observer.finish(row, FIXTURES["plugin_aliases"][0], 0, FIXTURES)

    def test_original_descriptor_closes_when_real_public_call_raises(self):
        run = self.audit["do_case"]
        instances = []
        original = run.__globals__["PREPARED_INSTALLATION"]["KiroInstallationAudit"]
        class Tracked(original):
            def __init__(self, *args, **kwargs):
                super().__init__(*args, **kwargs); instances.append(self)
        with tempfile.TemporaryDirectory() as directory, \
             patch.dict(run.__globals__["PREPARED_INSTALLATION"], {"KiroInstallationAudit": Tracked}), \
             patch.dict(run.__globals__, {"call": lambda *a, **kw: (_ for _ in ()).throw(RuntimeError("controlled call failure"))}), \
             patch.object(sys, "platform", "darwin"), self.assertRaisesRegex(RuntimeError, "controlled call failure"):
            run("/fixture/agentplugins", "kiro", "plugin", FIXTURES["plugin_aliases"][0], 0, Path(directory).resolve(),
                FIXTURES["plugin_declared_names"], FIXTURES["signed_release_pins"],
                installation_audit="kiro-installation", installation_fixtures=FIXTURES)
        self.assertEqual(len(instances), 1)
        self.assertIsNone(instances[0].mcp_fd)

    def test_reviewed_transient_native_projection_iid_and_foreign_alias_attacks_are_rejected(self):
        for mutation in ("top-level-member", "transient-cache", "wrong-mcp-value", "fresh-iid", "foreign-hardlink"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "plugin")
                documents = raw("plugin", 0, observer.project)
                if mutation == "fresh-iid":
                    documents["install"]["data"]["actions"][0]["installation_id"] = "11111111-1111-4111-8111-111111111111"
                original = observer.observe
                positions = []
                unknown = observer.home / "unowned-sibling-cache"
                def adversarial(argv, outcome):
                    position = len(positions); positions.append(position)
                    if position == 0:
                        if mutation in ("top-level-member", "wrong-mcp-value"):
                            value = json.loads(observer.mcp.read_bytes())
                            if mutation == "top-level-member": value["unowned-policy"] = {"mode": "tampered"}
                            else: value["mcpServers"]["atlassian"] = {"command": "/unrelated/executable", "args": ["wrong source"]}
                            replacement = observer.mcp.with_suffix(".private-new"); replacement.write_text(json.dumps(value))
                            replacement.chmod(observer.mcp.stat().st_mode & 0o777); replacement.replace(observer.mcp)
                        elif mutation == "transient-cache": unknown.write_bytes(b"unowned mutation")
                        elif mutation == "foreign-hardlink":
                            os.link(observer.home / ".kiro/skills/foreign-neighbor/SKILL.md", observer.data / "outside-snapshot-hardlink")
                    if position == 4 and mutation == "transient-cache": unknown.unlink()
                    return original(argv, outcome)
                observer.observe = adversarial
                with self.assertRaises(ValueError): self.finish(observer, "plugin", 0, documents)
                self.assertEqual(len(positions), 6 if mutation == "fresh-iid" else 1)

    def test_source_bound_private_receipt_and_projection_cannot_be_substituted(self):
        for mutation in ("receipt-source", "receipt-id", "receipt-profile", "receipt-user-profile",
                         "receipt-null-profile", "receipt-locator", "projection", "extra-object"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "plugin")
                documents = raw("plugin", 0, observer.project)
                original = observer.observe
                positions = []
                def adversarial(argv, outcome):
                    positions.append(None)
                    if len(positions) == 1:
                        path = observer.data / "state-v2.json"; state = json.loads(path.read_bytes())
                        record = state["installations"][0]; binding = next(iter(record["clients"].values()))
                        if mutation == "receipt-source": record["source"]["resolved_revision"] = "f" * 40
                        elif mutation == "receipt-id": record["installation_id"] = "11111111-1111-4111-8111-111111111111"
                        elif mutation == "receipt-profile": binding["native_profile_root"] = str(observer.project / ".kiro")
                        elif mutation == "receipt-user-profile": binding["native_profile_root"] = str(observer.home / ".kiro")
                        elif mutation == "receipt-null-profile": binding["native_profile_root"] = None
                        elif mutation == "receipt-locator": binding["target_locator"] = str(observer.data / "unowned")
                        elif mutation == "projection":
                            (Path(binding["target_locator"]) / "mcp.json").write_text('{"mcpServers":{"atlassian":{"command":"wrong source"}}}')
                        else:
                            binding["native_objects"].append({"kind": "unowned_cache", "path": str(observer.home / "cache")})
                        path.write_text(json.dumps(state))
                    return original(argv, outcome)
                observer.observe = adversarial
                with self.assertRaises(ValueError): self.finish(observer, "plugin", 0, documents)

    def test_project_private_profile_cannot_use_unbound_user_semantics(self):
        for mutation in ("missing", "empty", "user"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "skill")
                documents = raw("skill", 0, observer.project)
                original = observer.observe
                def adversarial(argv, outcome):
                    path = observer.project / ".uap/state-v2.json"
                    state = json.loads(path.read_bytes())
                    if state["installations"]:
                        binding = next(iter(state["installations"][0]["clients"].values()))
                        if mutation == "missing": binding.pop("native_profile_root", None)
                        else: binding["native_profile_root"] = "" if mutation == "empty" else str(observer.home / ".kiro")
                        path.write_text(json.dumps(state))
                    return original(argv, outcome)
                observer.observe = adversarial
                with self.assertRaises(ValueError): self.finish(observer, "skill", 0, documents)

    def test_native_writer_lock_is_held_before_calls_and_immutable_afterward(self):
        for mutation in (None, "bytes", "mode", "replace", "hardlink", "symlink", "remove"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), "plugin")
                lock = observer.mcp.with_name(observer.mcp.name + ".agentplugins.lock")
                self.assertIn(lock, observer.initial_paths)
                self.assertIn(lock, observer.protected_files)
                self.assertEqual(lock.read_bytes(), b"")
                self.assertEqual(lock.stat().st_mode & 0o777, 0o600)
                if mutation is None:
                    self.finish(observer, "plugin", 0, raw("plugin", 0, observer.project))
                    observer.guard()
                    continue
                if mutation == "bytes": lock.write_bytes(b"unowned")
                elif mutation == "mode": lock.chmod(0o644)
                elif mutation == "replace":
                    lock.unlink(); lock.write_bytes(b""); lock.chmod(0o600)
                elif mutation == "hardlink": os.link(lock, observer.home / "foreign-lock-alias")
                elif mutation == "symlink":
                    held = observer.home / "held-lock"; lock.rename(held); lock.symlink_to(held)
                else: lock.unlink()
                with self.assertRaises((ValueError, OSError)): observer.image()

    def test_owned_active_root_cleanup_and_deterministic_private_physical_identity(self):
        for kind in ("plugin", "skill"):
            for mutation in ("retained-active", "reminted-private", "late-receipt-change"):
                with self.subTest(kind=kind, mutation=mutation), tempfile.TemporaryDirectory() as directory:
                    observer = self.fixture(Path(directory).resolve(), kind)
                    documents = raw(kind, 0, observer.project)
                    original, calls, active = observer.observe, [], []
                    def adversarial(argv, outcome):
                        state_path = (observer.project / ".uap" if kind == "skill" else observer.data) / "state-v2.json"
                        if not calls:
                            state = json.loads(state_path.read_bytes())
                            binding = next(iter(state["installations"][0]["clients"].values()))
                            path = Path(binding["target_locator"])
                            if mutation == "reminted-private":
                                name = "unrelated-physical-root"; path = path.rename(path.with_name(name))
                                binding.update(physical_artifact_id=name, target_locator=str(path))
                                for item in binding["native_objects"]:
                                    if item["kind"] == "managed_package_directory": item.update(path=str(path), object_id="package:kiro:" + name)
                                state_path.write_text(json.dumps(state))
                            active.append(path)
                        if mutation == "late-receipt-change" and len(calls) == 2:
                            state = json.loads(state_path.read_bytes())
                            binding = next(iter(state["installations"][0]["clients"].values()))
                            binding["client_binding_id"] = "reminted-binding"
                            state["installations"][0]["clients"] = {"reminted-binding": binding}
                            state_path.write_text(json.dumps(state))
                        if mutation == "retained-active" and len(calls) == 4:
                            active[0].mkdir(parents=True); (active[0] / "retained-owned-artifact").write_bytes(b"residue")
                        calls.append(None)
                        return original(argv, outcome)
                    observer.observe = adversarial
                    with self.assertRaises(ValueError): self.finish(observer, kind, 0, documents)

    def test_constructor_closes_every_opened_descriptor_on_stat_or_held_read_failure(self):
        original_open, original_read, original_stat = os.open, os.pread, os.fstat
        for failure in ("read", "stat"):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as directory:
                opened, reads, failures = [], [], []
                def tracked_open(*args, **kwargs):
                    fd = original_open(*args, **kwargs); opened.append(fd); return fd
                def read(fd, *args):
                    reads.append(fd)
                    if failure == "read" and len(reads) == 2: raise OSError("controlled protected read failure")
                    return original_read(fd, *args)
                def descriptor_stat(fd):
                    if failure == "stat" and len(opened) == 2 and fd == opened[-1] and not failures:
                        failures.append(fd); raise OSError("controlled protected stat failure")
                    return original_stat(fd)
                with patch.object(os, "open", tracked_open), patch.object(os, "pread", read), \
                     patch.object(os, "fstat", descriptor_stat), self.assertRaises(OSError):
                    self.fixture(Path(directory).resolve(), "plugin")
                self.assertEqual(len(opened), 2)
                for fd in opened:
                    with self.assertRaises(OSError): original_stat(fd)

    def test_actual_do_case_orders_six_calls_and_two_refusal_calls_without_fullstream_exports(self):
        case_function = self.audit["do_case"]
        for kind, index in (("skill", 0), ("plugin", 0), ("plugin", 21), ("plugin", 22), ("plugin", 13)):
            with self.subTest(kind=kind, index=index), tempfile.TemporaryDirectory() as directory:
                root = Path(directory).resolve()
                project = root / f"{kind}-{index:02d}/project"
                documents = raw(kind, index, project) if index != 13 else None
                item = FIXTURES["skills" if kind == "skill" else "plugin_aliases"][index]
                identity = item["name"] if kind == "skill" else FIXTURES["plugin_declared_names"].get(item, item)
                calls, baseline = [], []
                skill = project / f".kiro/skills/{identity}/SKILL.md"
                def fake_call(argv, env, cwd, timeout=105, **kwargs):
                    self.assertIs(kwargs.get("capture_full"), True)
                    self.assertEqual(cwd, project)
                    self.assertTrue((Path(env["HOME"]) / ".kiro/settings/mcp.json").is_file())
                    self.assertTrue((project / ".kiro/settings/mcp.json").is_file())
                    # The real Directory cache and state-v2 writer protect
                    # this owned root with chmod(0700). Seed that mode before
                    # the observer captures it; later mode/inode drift fails.
                    data = Path(env["AGENTPLUGINS_HOME"])
                    self.assertEqual(data.stat().st_mode & 0o777, 0o700)
                    data.chmod(0o700)
                    calls.append(list(argv))
                    if index == 13:
                        if len(calls) == 1:
                            return self.outcome(None, 1, 'agentplugins: no eligible directory release for "marketing-kit": '
                                '777genius/marketing-kit: release does not support complete target set; missing kiro\n')
                        return self.outcome({"schema_version": 1, "command": "list", "result": "success", "data": {"installations": []}})
                    phase = prepared.COMMANDS[len(calls) - 1]
                    mcp = Path(env["HOME"]) / ".kiro/settings/mcp.json"
                    if phase == "install":
                        if kind == "skill":
                            skill.parent.mkdir(); skill.write_bytes(skill_body(identity))
                            (project / "uap.toml").write_text(identity)
                            (project / "uap.lock").write_text(identity)
                        else:
                            baseline.append(mcp.read_bytes())
                            if index in prepared.KIRO_ACTIVE:
                                owned = mcp.parent.parent / "skills" / identity; owned.mkdir()
                                (owned / "SKILL.md").write_bytes(skill_body(identity))
                            else:
                                body = json.loads(baseline[0]); body["mcpServers"][identity] = {"command": "inert"}
                                temp = mcp.with_suffix(".new"); temp.write_text(json.dumps(body)); temp.chmod(mcp.stat().st_mode & 0o777); temp.replace(mcp)
                        from types import SimpleNamespace
                        fixture_observer = SimpleNamespace(project=project, home=Path(env["HOME"]), data=Path(env["AGENTPLUGINS_HOME"]))
                        write_projection(fixture_observer, kind, index, documents)
                    if phase == "cleanup":
                        self.assertEqual(argv[2], documents["list"]["data"]["installations"][0]["installation_id"])
                        if kind == "skill":
                            shutil.rmtree(skill.parent)
                        else:
                            if index in prepared.KIRO_ACTIVE:
                                shutil.rmtree(mcp.parent.parent / "skills" / identity)
                            else:
                                temp = mcp.with_suffix(".new"); temp.write_bytes(baseline[0]); temp.chmod(mcp.stat().st_mode & 0o777); temp.replace(mcp)
                        from types import SimpleNamespace
                        clear_state(SimpleNamespace(project=project, data=Path(env["AGENTPLUGINS_HOME"])), kind)
                    return self.outcome(deepcopy(documents[phase]))
                substitutions = {"call": fake_call,
                    "locked_skill_is_pinned": lambda *args: skill.is_file(),
                    "matches_pinned_skill_bytes": lambda *args: skill.is_file()}
                mode = "kiro-skills-installation" if kind == "skill" else "kiro-installation"
                with patch.dict(case_function.__globals__, substitutions), patch.object(sys, "platform", "darwin"):
                    row = case_function("/fixture/agentplugins", "kiro", kind, item, index, root,
                        FIXTURES["plugin_declared_names"], FIXTURES["signed_release_pins"],
                        installation_audit=mode, installation_fixtures=FIXTURES)
                self.assertEqual(len(calls), 2 if index == 13 else 6)
                self.assertEqual(row["kiro_installation_custody"]["status"],
                                 "verified-installation-negative" if index == 13 else "verified-installation-lifecycle")
                serialized = json.dumps(row)
                for field in ('"_json"', '"stdout_full"', '"stderr_full"'):
                    self.assertNotIn(field, serialized)

    def test_unapproved_route_fails_before_public_calls(self):
        run = self.audit["do_case"]
        with tempfile.TemporaryDirectory() as directory, patch.dict(run.__globals__, {"call": lambda *a, **kw: self.fail("wrong route executed")}), self.assertRaises(ValueError):
            run("/fixture/agentplugins", "cursor", "plugin", "atlassian", 0, Path(directory), {},
                installation_audit="kiro-installation", installation_fixtures=FIXTURES)

    def complete_report(self, kind):
        rows = []
        for index in range(30):
            with tempfile.TemporaryDirectory() as directory:
                observer = self.fixture(Path(directory).resolve(), kind)
                if kind == "plugin" and index == 13:
                    row = {"index": 13, "identity": "marketing-kit", "source": "marketing-kit", "target": "kiro", "kind": kind,
                           "status": "documented-or-source-unsupported", "native_file_delta": 0,
                           "sentinels_preserved": True, "native_residuals": [],
                           **{k: False for k in ("listed_identity", "failed_install_registered", "failed_install_residue",
                               "post_remove_identity_registered", "repeat_verified", "removal_verified")},
                           **{k: [] for k in ("native_identity_files", "native_component_candidates", "matched_installation_ids",
                               "repeat_installation_ids", "post_removal_leftovers")},
                           **{k: None for k in ("repeat_add", "repeat_list", "cleanup", "post_remove_list")}}
                    add = [observer.cli, "add", "marketing-kit", "--target", "kiro", "--scope", "user", "--format", "json"]
                    listed = [observer.cli, "list", "--target", "kiro", "--scope", "user", "--format", "json"]
                    observer.observe(add, self.outcome(None, 1, 'agentplugins: no eligible directory release for "marketing-kit": '
                        '777genius/marketing-kit: release does not support complete target set; missing kiro\n'))
                    observer.observe(listed, self.outcome({"schema_version": 1, "command": "list", "result": "success", "data": {"installations": []}}))
                    proof = observer.finish(row, "marketing-kit", index, FIXTURES)
                else:
                    proof, row = self.finish(observer, kind, index, raw(kind, index, observer.project))
                row["kiro_installation_custody"] = proof
                rows.append(row)
        return {"target": "kiro", "kind": kind, "platform": "darwin", "architecture": "arm64",
                "client_version": {"status": "reported", "value": "kiro-cli 3.0.0", "exit_code": 0},
                "cli_version": {"status": "reported", "value": "agentplugins 0.1.115", "exit_code": 0},
                "fixture_count": 30, "cases_attempted": 30, "source_selection_complete": True,
                "full_coverage": True, "passed": 0, "verified_negative_coverage": 0,
                "prepared_lifecycle_passed": 23 if kind == "plugin" else 0, "results": rows}

    def test_full_30_skills_and_29_packages_plus_exact_refusal_require_all_guards(self):
        changes = [lambda r: r.update(passed=1), lambda r: r.update(cases_attempted=29),
                   lambda r: r.update(platform="linux"), lambda r: r["client_version"].update(value="kiro-cli 3.1"),
                   lambda r: r["results"].pop(), lambda r: r["results"][0].update(source="foreign/source"),
                   lambda r: r["results"][0].update(repeat_verified=False),
                   lambda r: r["results"][0].update(native_residuals=["unknown"]),
                   lambda r: r["results"][0].update(kiro_installation_custody={}),
                   lambda r: r["results"][0]["kiro_installation_custody"].update(writer_closure_claimed=True),
                   lambda r: r["results"][0]["kiro_installation_custody"]["observations"].pop(),
                   lambda r: r["results"][0]["public_target_states"]["after-add"]["states"][0].update(authentication="authenticated")]
        for kind in ("skill", "plugin"):
            good = self.complete_report(kind)
            original = deepcopy(good)
            mode = "kiro-skills-installation" if kind == "skill" else "kiro-installation"
            result = prepared.kiro_summary(good, FIXTURES, mode)
            self.assertTrue(result["installation_acceptance_passed"])
            self.assertEqual(result["installation_passed"], 30 if kind == "skill" else 29)
            self.assertEqual(result["installation_verified_negative_coverage"], int(kind == "plugin"))
            self.assertTrue(all(value is False for key, value in result.items() if key.endswith("_qualified")))
            self.assertEqual(good, original)
            for mutate in changes:
                with self.subTest(kind=kind, mutation=mutate):
                    value = deepcopy(good); mutate(value)
                    self.assertFalse(prepared.kiro_summary(value, FIXTURES, mode)["installation_acceptance_passed"])
            if kind == "plugin":
                for status in ("installation-lifecycle-observed", "passed", "incomplete-native-proof"):
                    changed = deepcopy(good); changed["results"][22]["status"] = status
                    self.assertFalse(prepared.kiro_summary(changed, FIXTURES, mode)["installation_acceptance_passed"])
                for key, value in (("native_residuals", ["unknown"]), ("status", "passed"), ("sentinels_preserved", False)):
                    changed = deepcopy(good); changed["results"][13][key] = value
                    self.assertFalse(prepared.kiro_summary(changed, FIXTURES, mode)["installation_acceptance_passed"])

    def test_actual_main_opt_in_keeps_native_default_failure_and_complete_forwarding(self):
        main = self.audit["main"]
        good = self.complete_report("skill")
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve(); cli = root / "agentplugins"; cli.write_text("inert")
            fixture_file = root / "fixtures.json"; fixture_file.write_text(json.dumps(FIXTURES))
            calls = []
            def case(*args, **kwargs):
                calls.append(kwargs)
                return deepcopy(good["results"][args[4]])
            substitutions = {"do_case": case, "vendor_bootstrap_attested": lambda: True,
                "FIXTURES": fixture_file,
                "AGENT_BINARIES": {"kiro-cli": "/fixture/kiro-cli"},
                "version_evidence": lambda argv, *args: deepcopy(good["cli_version"] if argv[1] == "version" else good["client_version"]),
                "published_audit_temp_parent": lambda: None, "isolated_client_env": lambda *args: {}}
            for mode, code in (("native", 1), ("kiro-skills-installation", 0)):
                calls.clear()
                argv = ["audit", "--cli", str(cli), "--target", "kiro", "--kind", "skill", "--limit", "30",
                        "--report-dir", str(root / mode), "--acceptance", mode]
                with patch.dict(main.__globals__, substitutions), patch("sys.argv", argv), patch.object(sys, "platform", "darwin"), \
                     patch("platform.machine", return_value="arm64"), redirect_stdout(io.StringIO()):
                    self.assertEqual(main(), code)
                written = json.loads((root / mode / "kiro-skill.json").read_text())
                self.assertEqual(written["passed"], 0)
                self.assertEqual(len(calls), 30)
                expected = {} if mode == "native" else {"installation_audit": mode, "installation_fixtures": FIXTURES}
                self.assertTrue(all(call == expected for call in calls))
                if mode != "native": self.assertTrue(written["installation_acceptance_passed"])


if __name__ == "__main__":
    unittest.main()
