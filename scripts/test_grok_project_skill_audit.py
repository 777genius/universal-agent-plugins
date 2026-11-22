"""Hermetic unit inputs for the additive verifier; never native Grok evidence.

The executable is inert and is never run. Only this unit suite substitutes its
digest for the production binary pin. No public child, network or model runs.
"""
import base64
import contextlib
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import stat
import tempfile
import tomllib
import unittest
from unittest.mock import patch

import grok_project_skill_audit as audit


INERT_BINARY = b"Inert unit fixture; never execute this file.\n"
BODY = b"---\nname: frontend-design\ndescription: Inert verifier unit fixture\n---\nNever native proof.\n"
NAME = "skill-b0a5b0253f26020d7acb797250495459c1bd758fe440530b3db1347e"
UID = "12345678-1234-4123-8123-123456789abc"
VERSION = {"status": "reported", "value": audit.VERSION, "exit_code": 0}


def blob(body):
    return hashlib.sha1(b"blob " + str(len(body)).encode() + b"\0" + body).hexdigest()


def raw(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode()


class Fixture:
    def __init__(self, base):
        self.case, self.home, self.project = base / "case", base / "case/home", base / "case/project"
        for path in (self.case, self.home, self.project, self.case / "uap-data"):
            path.mkdir(mode=0o700)
        self.binary = base / "grok-inert-unit"
        self.binary.write_bytes(INERT_BINARY)
        self.binary.chmod(0o700)
        self.foreign = {
            self.project / "foreign-untouched.txt": b"foreign content never belongs to UAP\n",
            self.home / "foreign-untouched.txt": b"foreign profile content never belongs to UAP\n",
            self.home / ".grok/foreign-neighbor.txt": b"unmanaged native neighbor must survive\n",
            self.project / ".grok/skills/foreign-neighbor/SKILL.md":
                b"---\nname: foreign-neighbor\ndescription: unrelated\n---\n",
        }
        for path, body in self.foreign.items():
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(body)
        self.item = {"repo": "anthropics/skills", "name": "frontend-design", "folder": "frontend-design",
                     "ref": "a" * 40, "skill_md_git_blob_sha1": blob(BODY)}
        self.item["source"] = f'https://github.com/{self.item["repo"]}/tree/{self.item["ref"]}/skills/{self.item["folder"]}'
        self.physical = NAME[:51] + "-" + hashlib.sha256(UID.encode()).hexdigest()[:12]
        self.relative = ".grok/plugins/" + self.physical + "/skills/frontend-design/SKILL.md"
        self.active = self.project / ".grok/plugins" / self.physical
        self.revision = {"resolved_revision": self.item["ref"], "tree_digest": "sha256:" + "b" * 64,
                         "manifest_digest": "sha256:" + "c" * 64}
        self.installation = {"installation_id": UID, "name": NAME, "source": "anthropics/skills/skills/frontend-design",
                             "clients": [{"client_id": "grok", "scope": "project", "materialization": "materialized",
                                          "activation": "active", "authentication": "not_required", "policy": "allowed",
                                          "verification": "installation_verified", "package_revision": self.revision}]}
        source = {"kind": "skill", "skill": "frontend-design", "repository": "anthropics/skills",
                  "path": "skills/frontend-design", "resolved_skill_path": "skills/frontend-design",
                  "requested": {"kind": "commit", "value": self.item["ref"]}, "commit": self.item["ref"],
                  "normalizer": "agent-skill-package/v1", "raw_tree_digest": "sha256:" + "d" * 64}
        self.lock = {"lock_version": 1, "manifest_digest": "sha256:" + "e" * 64,
                     "packages": [{"alias": "frontend-design", "source": {"git": source},
                                   "source_key": "github-skill:anthropics/skills:frontend-design:skills/frontend-design",
                                   "targets": ["grok"], "digest_algorithm": "agentplugins-tree-sha256-v1",
                                   "tree_digest": self.revision["tree_digest"], "manifest_digest": self.revision["manifest_digest"]}]}
        self.public = {"schema_version": 1, "command": "skills add", "result": "success", "status": "succeeded",
                       "scope": "project", "readiness": "ready", "next_actions": [],
                       "items": [{"alias": "frontend-design", "targetId": "grok", "status": "succeeded",
                                  "destinationPath": str(self.active), "candidateId": "sha256:" + "f" * 64}]}

    def start(self, version=None):
        return audit.GrokPretrustedSkillAudit(self.case, self.home, self.project, self.item, self.binary,
                                             VERSION if version is None else version)

    def install(self):
        skill = self.project / self.relative
        skill.parent.mkdir(parents=True)
        skill.write_bytes(BODY)
        manifest = self.active / ".grok-plugin/plugin.json"
        manifest.parent.mkdir()
        manifest.write_bytes(raw({"name": NAME, "version": "0.0.0", "skills": "./skills/"}))
        (self.project / "uap.lock").write_bytes(raw(self.lock))

    def publish_empty_agreement(self, locked=None, manifest=b"schema_version = 1\n[dependencies]\n[targets]\n"):
        # Model only the saved-remove publication boundary; no installer executes.
        (self.project / "uap.toml").write_bytes(manifest)
        temporary = self.project / "empty-lock-unit-candidate"
        empty_digest = "sha256:" + hashlib.sha256(b'{"schema_version":1,"dependencies":null,"targets":null}').hexdigest()
        temporary.write_bytes(raw({"lock_version": 1, "manifest_digest": empty_digest, "packages": []}
                                  if locked is None else locked))
        os.replace(temporary, self.project / "uap.lock")

    def inspection(self, owned=False):
        document = {"grokVersion": "1.0.70", "cwd": str(self.project), "projectRoot": str(self.project),
                    "projectTrusted": True, "plugins": [], "mcpServers": [],
                    "skills": [{"name": "foreign-neighbor", "description": "unrelated", "userInvocable": True,
                                "source": {"type": "project", "path": str(self.project / ".grok/skills/foreign-neighbor/SKILL.md")}}]}
        if owned:
            document["plugins"].append({"name": NAME, "scope": "project", "path": str(self.active), "enabled": True,
                                        "provides": {"skills": 1, "agents": 0, "hooks": False, "mcpServers": 0}})
            document["skills"].append({"name": "frontend-design", "description": "Inert verifier unit fixture",
                                       "userInvocable": True, "source": {"type": "plugin", "plugin_name": NAME,
                                                                          "path": str(self.project / self.relative)}})
        return document

    def capture(self, phase, document=None, body=None, stderr=b""):
        stdout = raw(self.inspection(phase in ("add", "repeat")) if document is None else document) if body is None else body
        label = "grok-pretrusted-" + phase
        record = {"probe": label, "argv": [str(self.binary), "inspect", "--json"], "cwd": str(self.project),
                  "error": None, "exit": 0, "elapsed_seconds": 0.01,
                  "stdout_file": label + ".stdout.log", "stderr_file": label + ".stderr.log",
                  "stdout_bytes": len(stdout), "stderr_bytes": len(stderr),
                  "stdout_sha256": hashlib.sha256(stdout).hexdigest(), "stderr_sha256": hashlib.sha256(stderr).hexdigest()}
        return record, stdout, stderr

    def bind(self, verifier, repeated=False):
        public = copy.deepcopy(self.public)
        if repeated:
            public["items"][0]["status"] = "unchanged"
        return verifier.bind_installation(self.installation, self.relative, public, repeated=repeated)

    def removed(self, verifier, delete_lock=False):
        verifier.begin_remove()
        shutil.rmtree(self.active)
        if delete_lock:
            (self.project / "uap.lock").unlink()
        else:
            self.publish_empty_agreement()
        return verifier.observe("remove", *self.capture("remove"))


@contextlib.contextmanager
def fixture(start=True):
    with tempfile.TemporaryDirectory(prefix="grok-pretrusted-unit-") as temporary, \
            patch.object(audit, "BINARY_SHA256", hashlib.sha256(INERT_BINARY).hexdigest()):
        value, verifier = Fixture(Path(temporary)), None
        try:
            if start:
                verifier = value.start()
            yield value, verifier
        finally:
            if verifier is not None:
                verifier.close()


class PretrustedUnitVerifier(unittest.TestCase):
    def prepare_bound(self, value, verifier):
        self.assertTrue(verifier.observe("before", *value.capture("before")))
        value.install()
        self.assertTrue(value.bind(verifier))

    def test_complete_inert_transcript_and_frozen_preconditions(self):
        with fixture() as (value, verifier):
            self.assertEqual(audit.fixture_name(value.item), NAME)
            config = tomllib.loads((value.home / ".grok/config.toml").read_text())
            trust = tomllib.loads((value.home / ".grok/trusted_folders.toml").read_text())
            self.assertEqual(config["plugins"], {"enabled": [NAME], "disabled": []})
            self.assertEqual(trust, {"folders": {str(value.project): {"trusted": True}}})
            self.assertFalse((value.project / ".git").exists())
            self.assertEqual(list((value.case / "uap-data").iterdir()), [])
            self.prepare_bound(value, verifier)
            self.assertTrue(verifier.observe("add", *value.capture("add")))
            self.assertTrue(value.bind(verifier, repeated=True))
            self.assertTrue(verifier.observe("repeat", *value.capture("repeat")))
            self.assertTrue(value.removed(verifier))
            self.assertTrue(verifier.finish())
            evidence = verifier.evidence()
            self.assertFalse(evidence["cold_activation_claimed"])
            self.assertFalse(evidence["writer_closure_claimed"])
            self.assertEqual(evidence["profile"]["initial"], evidence["profile"]["last_verified"])
            self.assertEqual(len(evidence["profile"]["initial"]), 6)
            self.assertEqual(evidence["binary"]["sha256"], hashlib.sha256(INERT_BINARY).hexdigest())
            self.assertNotIn("base64", evidence["binary"])
            self.assertNotEqual(evidence["binding"]["lock"]["inode"], evidence["removal_agreement"]["lock"]["inode"])
            for row in evidence["observations"]:
                self.assertEqual(hashlib.sha256(base64.b64decode(row["stdout"]["base64"])).hexdigest(), row["stdout"]["sha256"])
            verifier.close()
            self.assertEqual(verifier.evidence(), evidence)  # Evidence is frozen, not a late path reread.

    def test_constructor_rejects_existing_child_or_installation_state(self):
        for relative in ("project/.git", "project/uap.lock", "project/uap.toml", "project/.uap", "uap-data/state"):
            with self.subTest(relative=relative), fixture(start=False) as (value, _):
                (value.case / relative).write_bytes(b"preexisting")
                with self.assertRaises(ValueError):
                    value.start()

    def test_constructor_refuses_nonprivate_roots_and_path_aliases(self):
        with fixture(start=False) as (value, _):
            value.home.chmod(0o755)
            with self.assertRaises(ValueError):
                value.start()
        with fixture(start=False) as (value, _):
            alias = value.case.parent / "alias"
            alias.symlink_to(value.case, target_is_directory=True)
            with self.assertRaises(audit.CustodyError):
                audit.GrokPretrustedSkillAudit(alias, alias / "home", alias / "project", value.item, value.binary, VERSION)
        with self.assertRaises(ValueError):
            audit.canonical("/tmp/../tmp/project")

    def test_constructor_refuses_preexisting_config_and_wrong_binary_even_with_reported_version(self):
        with fixture(start=False) as (value, _):
            (value.home / ".grok/config.toml").symlink_to(value.project / "foreign-untouched.txt")
            with self.assertRaises(FileExistsError):
                value.start()
        with fixture(start=False) as (value, _):
            value.binary.write_bytes(b"Different inert executable with the same claimed version.\n")
            with self.assertRaisesRegex(ValueError, "qualified Linux"):
                value.start()
        with fixture(start=False) as (value, _):
            value.binary.unlink()
            value.binary.symlink_to(value.home / "foreign-untouched.txt")
            with self.assertRaises(audit.CustodyError):
                value.start()

    def test_original_profile_foreign_and_executable_mutations_stop_children(self):
        for kind in ("config-bytes", "trust-mode", "profile-replace", "foreign-replace", "foreign-symlink", "binary-replace", "foreign-parent-replace"):
            with self.subTest(kind=kind), fixture() as (value, verifier):
                if kind == "config-bytes":
                    (value.home / ".grok/config.toml").write_bytes(b"modified")
                elif kind == "trust-mode":
                    (value.home / ".grok/trusted_folders.toml").chmod(0o644)
                elif kind == "profile-replace":
                    path = value.home / ".grok/config.toml"
                    body = path.read_bytes()
                    path.unlink()
                    path.write_bytes(body)
                    path.chmod(0o600)
                elif kind == "foreign-replace":
                    path, body = next(iter(value.foreign.items()))
                    path.unlink()
                    path.write_bytes(body)
                elif kind == "foreign-symlink":
                    path = value.home / ".grok/foreign-neighbor.txt"
                    path.unlink()
                    path.symlink_to(value.project / "foreign-untouched.txt")
                elif kind == "foreign-parent-replace":
                    path = value.project / ".grok/skills/foreign-neighbor"
                    path.rename(path.with_name("moved"))
                    path.symlink_to(path.with_name("moved"), target_is_directory=True)
                else:
                    value.binary.unlink()
                    value.binary.write_bytes(INERT_BINARY)
                    value.binary.chmod(0o700)
                with self.assertRaises(audit.CustodyError):
                    verifier.guard()
                self.assertFalse(verifier.evidence()["profile"]["last_guard_passed"])

    def test_uuid_physical_join_rejects_another_valid_suffix(self):
        with fixture() as (value, verifier):
            self.assertTrue(verifier.observe("before", *value.capture("before")))
            value.install()
            changed = copy.deepcopy(value.installation)
            changed["installation_id"] = "12345678-1234-4123-8123-123456789abd"
            self.assertFalse(verifier.bind_installation(changed, value.relative, value.public))
            self.assertIn("UUID", verifier.reasons[0])

    def test_lock_source_digest_target_and_alias_joins(self):
        for change in ("tree_digest", "manifest_digest", "alias", "source_key", "targets", "commit", "repository", "path", "resolved_skill_path", "requested", "duplicate", "duplicate-key"):
            with self.subTest(change=change), fixture() as (value, verifier):
                self.assertTrue(verifier.observe("before", *value.capture("before")))
                value.install()
                lock = copy.deepcopy(value.lock)
                package, git = lock["packages"][0], lock["packages"][0]["source"]["git"]
                if change in ("tree_digest", "manifest_digest"):
                    package[change] = "sha256:" + "0" * 64
                elif change in ("alias", "source_key"):
                    package[change] = "foreign"
                elif change == "targets":
                    package[change] = ["grok", "claude"]
                elif change == "requested":
                    git[change] = {"kind": "ref", "value": "main"}
                elif change == "duplicate":
                    lock["packages"].append(copy.deepcopy(package))
                elif change != "duplicate-key":
                    git[change] = "foreign"
                body = raw(lock)
                if change == "duplicate-key":
                    body = body.replace(b'"lock_version":1', b'"lock_version":1,"lock_version":1')
                (value.project / "uap.lock").write_bytes(body)
                self.assertFalse(value.bind(verifier))

    def test_lock_replacement_and_artifact_symlinks_are_structural_failures(self):
        with fixture() as (value, verifier):
            self.prepare_bound(value, verifier)
            lock = value.project / "uap.lock"
            body = lock.read_bytes()
            lock.unlink()
            lock.write_bytes(body)
            with self.assertRaises(audit.CustodyError):
                verifier.guard()
        with fixture() as (value, verifier):
            self.assertTrue(verifier.observe("before", *value.capture("before")))
            value.install()
            skill = value.project / value.relative
            skill.unlink()
            skill.symlink_to(value.project / ".grok/skills/foreign-neighbor/SKILL.md")
            with self.assertRaises(audit.CustodyError):
                value.bind(verifier)

    def test_public_ready_and_valid_blob_are_not_native_proof(self):
        with fixture() as (value, verifier):
            self.prepare_bound(value, verifier)
            record = value.capture("add", document=value.inspection(False))
            self.assertFalse(verifier.observe("add", *record))
            self.assertTrue(value.bind(verifier, repeated=True))
            self.assertTrue(verifier.observe("repeat", *value.capture("repeat")))
            self.assertTrue(value.removed(verifier))
            self.assertFalse(verifier.finish())
            self.assertEqual(verifier.evidence()["observations"][1]["record"], record[0])

    def test_native_record_must_join_all_full_held_streams(self):
        for change in ("argv", "cwd", "probe", "exit-bool", "exit-nonzero", "error", "bytes-bool", "bytes-count", "hash", "file", "extra", "elapsed", "stderr", "oversize"):
            with self.subTest(change=change), fixture() as (value, verifier):
                record, stdout, stderr = value.capture("before")
                if change == "argv": record["argv"] = [str(value.binary), "inspect"]
                elif change == "cwd": record["cwd"] = str(value.home)
                elif change == "probe": record["probe"] = "grok-pretrusted-add"
                elif change == "exit-bool": record["exit"] = False
                elif change == "exit-nonzero": record["exit"] = 1
                elif change == "error": record["error"] = "output limit reached"
                elif change == "bytes-bool": record["stderr_bytes"] = False
                elif change == "bytes-count": record["stdout_bytes"] -= 1
                elif change == "hash": record["stdout_sha256"] = "0" * 64
                elif change == "file": record["stdout_file"] = "wrong.stdout.log"
                elif change == "extra": record["truncated"] = False
                elif change == "elapsed": record["elapsed_seconds"] = float("nan")
                elif change == "stderr": record, stdout, stderr = value.capture("before", stderr=b"warning\n")
                else: record, stdout, stderr = value.capture("before", body=b" " * (audit.LIMIT + 1))
                self.assertFalse(verifier.observe("before", record, stdout, stderr))
                self.assertFalse(verifier.finish())

    def test_native_selected_source_types_counts_and_foreign_inventory(self):
        for change in ("version", "trust", "project", "plugin-case", "skill-case", "physical-path", "skill-path-alias", "plugin-source", "standalone", "description", "disabled", "invocable", "count-bool", "count-float", "foreign-missing", "foreign-change", "duplicate", "unknown-field", "mcp", "duplicate-json"):
            with self.subTest(change=change), fixture() as (value, verifier):
                self.prepare_bound(value, verifier)
                document = value.inspection(True)
                plugin, skill = document["plugins"][0], document["skills"][1]
                if change == "version": document["grokVersion"] = "1.0.67"
                elif change == "trust": document["projectTrusted"] = False
                elif change == "project": document["projectRoot"] = str(value.home)
                elif change == "plugin-case": plugin["name"] = NAME.upper()
                elif change == "skill-case": skill["name"] = "Frontend-Design"
                elif change == "physical-path": plugin["path"] = str(value.active.parent / "foreign")
                elif change == "skill-path-alias": skill["source"]["path"] = str(value.active) + "/./skills/frontend-design/SKILL.md"
                elif change == "plugin-source": skill["source"]["plugin_name"] = "foreign"
                elif change == "standalone": skill["source"] = {"type": "project", "path": str(value.project / value.relative)}
                elif change == "description": skill["description"] = "unbound description"
                elif change == "disabled": skill["disabled"] = True
                elif change == "invocable": skill["userInvocable"] = False
                elif change == "count-bool": plugin["provides"]["skills"] = True
                elif change == "count-float": plugin["provides"]["skills"] = 1.0
                elif change == "foreign-missing": document["skills"].pop(0)
                elif change == "foreign-change": document["skills"][0]["source"]["path"] += "-renamed"
                elif change == "duplicate": document["skills"].append(copy.deepcopy(skill))
                elif change == "unknown-field": skill["nativeReady"] = True
                elif change == "mcp": document["mcpServers"] = [{"name": "unexpected"}]
                body = raw(document)
                if change == "duplicate-json":
                    body = body.replace(b'"grokVersion":"1.0.70"', b'"grokVersion":"1.0.70","grokVersion":"1.0.70"')
                self.assertFalse(verifier.observe("add", *value.capture("add", body=body)))

    def test_repeat_must_use_the_original_installation_and_receipt(self):
        for change in ("uuid", "candidate", "readiness", "status"):
            with self.subTest(change=change), fixture() as (value, verifier):
                self.prepare_bound(value, verifier)
                self.assertTrue(verifier.observe("add", *value.capture("add")))
                installation, public = copy.deepcopy(value.installation), copy.deepcopy(value.public)
                public["items"][0]["status"] = "unchanged"
                if change == "uuid": installation["installation_id"] = "12345678-1234-4123-8123-123456789abd"
                elif change == "candidate": public["items"][0]["candidateId"] = "sha256:" + "0" * 64
                elif change == "readiness": public["readiness"] = "needs_action"
                else: public["items"][0]["status"] = "succeeded"
                self.assertFalse(verifier.bind_installation(installation, value.relative, public, repeated=True))
                self.assertTrue(verifier.begin_remove())

    def test_missing_repeat_cannot_finish_but_cleanup_is_permitted(self):
        with fixture() as (value, verifier):
            self.prepare_bound(value, verifier)
            self.assertTrue(verifier.observe("add", *value.capture("add")))
            self.assertFalse(value.removed(verifier, delete_lock=True))
            self.assertFalse(verifier.finish())
            self.assertFalse(value.active.exists())

    def test_removal_requires_original_foreign_and_owned_absence(self):
        with fixture() as (value, verifier):
            self.prepare_bound(value, verifier)
            self.assertTrue(verifier.observe("add", *value.capture("add")))
            self.assertTrue(value.bind(verifier, repeated=True))
            self.assertTrue(verifier.observe("repeat", *value.capture("repeat")))
            verifier.begin_remove()
            self.assertFalse(verifier.observe("remove", *value.capture("remove")))
        with fixture() as (value, verifier):
            self.prepare_bound(value, verifier)
            verifier.begin_remove()
            parent = value.active.parent
            parent.rename(parent.with_name("moved-plugins"))
            parent.symlink_to(parent.with_name("moved-plugins"), target_is_directory=True)
            with self.assertRaises(audit.CustodyError):
                verifier.guard()

    def test_saved_remove_empty_lock_transition_is_narrow_and_one_time(self):
        for change in ("package", "digest", "unknown", "bool-version", "manifest", "symlink", "inplace", "second-replacement"):
            with self.subTest(change=change), fixture() as (value, verifier):
                self.prepare_bound(value, verifier)
                verifier.begin_remove()
                locked = {"lock_version": 1, "manifest_digest": audit.EMPTY_INTENT, "packages": []}
                manifest = b"schema_version = 1\n[dependencies]\n[targets]\n"
                if change == "package": locked["packages"] = [value.lock["packages"][0]]
                elif change == "digest": locked["manifest_digest"] = "sha256:" + "0" * 64
                elif change == "unknown": locked["unexpected"] = True
                elif change == "bool-version": locked["lock_version"] = True
                elif change == "manifest": manifest += b'grok = ["frontend-design"]\n'
                if change == "inplace":
                    (value.project / "uap.toml").write_bytes(manifest)
                    (value.project / "uap.lock").write_bytes(raw(locked))
                elif change == "symlink":
                    (value.project / "uap.lock").unlink()
                    (value.project / "uap.lock").symlink_to(value.home / "foreign-untouched.txt")
                else:
                    value.publish_empty_agreement(locked, manifest)
                    if change == "second-replacement":
                        verifier.guard()
                        temporary = value.project / "another-empty-unit-lock"
                        temporary.write_bytes(raw(locked))
                        os.replace(temporary, value.project / "uap.lock")
                with self.assertRaises(audit.CustodyError):
                    verifier.guard()

    def test_stale_or_deleted_lock_cannot_prove_saved_remove(self):
        for deleted in (False, True):
            with self.subTest(deleted=deleted), fixture() as (value, verifier):
                self.prepare_bound(value, verifier)
                self.assertTrue(verifier.observe("add", *value.capture("add")))
                self.assertTrue(value.bind(verifier, repeated=True))
                self.assertTrue(verifier.observe("repeat", *value.capture("repeat")))
                verifier.begin_remove()
                shutil.rmtree(value.active)
                if deleted:
                    (value.project / "uap.lock").unlink()
                self.assertFalse(verifier.observe("remove", *value.capture("remove")))
                self.assertIn("empty saved agreement", verifier.reasons[-1])
                self.assertFalse(verifier.finish())

    def test_skill_blob_and_yaml_frontmatter_bounds(self):
        with fixture(start=False) as (value, _):
            with self.assertRaisesRegex(ValueError, "immutable Git blob"):
                audit.skill_description(BODY + b"changed", value.item)
            for body in (b"name: frontend-design\ndescription: missing delimiter\n",
                         b"---\nname: frontend-design\ndescription: unclosed\n",
                         b"---\nname: frontend-design\nname: frontend-design\ndescription: duplicate\n---\n",
                         b"---\nname: frontend-design\n1: illegal mapping key\ndescription: invalid\n---\n",
                         b"---\nname: frontend-design\ndescription: [not, a, string]\n---\n",
                         b"---\nname: frontend-design\ndescription: valid\ncycle: &cycle [*cycle]\n---\n",
                         b"---\nname: frontend-design\ndescription: " + b"a" * 1025 + b"\n---\n",
                         b"---\nname: frontend-design\ndescription: valid\nextra: " + b"[" * 65 + b"0" + b"]" * 65 + b"\n---\n"):
                with self.subTest(body=body[:80]), self.assertRaises(ValueError):
                    audit.skill_description(body, {**value.item, "skill_md_git_blob_sha1": blob(body)})
            supported = b"\xef\xbb\xbf---\r\nname: frontend-design\r\ndescription: >-\r\n  Inert unit\r\n  description\r\nextra: &a [one]\r\nother: *a\r\n---\r\n"
            self.assertEqual(audit.skill_description(supported, {**value.item, "skill_md_git_blob_sha1": blob(supported)}),
                             "Inert unit description")


if __name__ == "__main__":
    unittest.main()
