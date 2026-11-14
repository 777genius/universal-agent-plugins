#!/usr/bin/env python3
"""Offline contract regressions; these tests are never a lifecycle receipt."""
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock
import zipfile

SPEC = importlib.util.spec_from_file_location("released_source_audit", Path(__file__).with_name("released-source-audit.py"))
audit = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(audit)


def snapshot():
    return {"revocations": [], "evidence": [], "distributions": [{
        "id": "fixture/portable", "status": "active", "releases": [{
            "sequence": 1, "manifest_name": "portable", "components": ["mcp"],
            "package_source": {"repository": "fixture/catalog", "revision": "a" * 40, "path": "plugins/portable"},
            "tree_digest": "sha256:" + "b" * 64, "manifest_digest": "sha256:" + "c" * 64,
        }], "release_policies": [{"release_sequence": 1, "status": "active", "targets": [{"client": "codex"}]}],
    }]}


def case(value=None):
    return audit.source_case(value or snapshot(), "fixture/portable", 1, "pi", "project")


def state():
    selected = case()
    binding = {"client_id": "pi", "client_binding_id": "binding", "scope": "project", "materialization": "materialized",
               "package_revision": {"resolved_revision": "a" * 40, "tree_digest": selected["tree_digest"], "manifest_digest": selected["manifest_digest"]}}
    return {"installations": [{"installation_id": "install", "declared_name": "portable", "origin_mode": "direct",
                               "source": {"repository": "fixture/catalog", "package_subpath": "plugins/portable", "resolved_revision": "a" * 40},
                               "clients": {"binding": binding}}],
            "environment_receipts": [{"installation_id": "install", "client_id": "pi", "client_binding_id": "binding", "phase": "committed",
                                      "source_key": "github:fixture/catalog:plugins/portable", "package_digest": selected["tree_digest"],
                                      "environment_id": "project-default", "alias": "portable", "receipt_id": "receipt"}]}


class SourceSelectionTests(unittest.TestCase):
    def test_exact_source_is_not_a_signed_target_grant(self):
        result = case()
        self.assertEqual(result["selector"], "github:fixture/catalog@" + "a" * 40 + "//plugins/portable")
        self.assertEqual(result["expected"], "materialized-lifecycle")
        self.assertNotIn("authorized", result)

    def test_suspended_and_revoked_source_refused(self):
        for field in ("distribution", "policy", "revocation"):
            value = snapshot()
            if field == "distribution":
                value["distributions"][0]["status"] = "suspended"
            elif field == "policy":
                value["distributions"][0]["release_policies"][0]["status"] = "revoked"
            else:
                value["revocations"] = [{"distribution_id": "fixture/portable", "release_sequence": 1}]
            with self.subTest(field=field), self.assertRaises(ValueError):
                case(value)

    def test_existing_signed_target_is_never_reinterpreted(self):
        value = snapshot()
        value["distributions"][0]["release_policies"].append({"release_sequence": 2, "status": "revoked", "targets": [{"client": "pi"}]})
        with self.assertRaises(ValueError):
            case(value)

    def test_existing_target_evidence_requires_review(self):
        value = snapshot()
        value["evidence"] = [{"client": "pi", "distribution_id": "fixture/portable", "status": "failed"}]
        with self.assertRaises(ValueError):
            case(value)

    def test_no_extension_or_app_binding_fallback(self):
        for kind in ("extension", "binding"):
            value = snapshot()
            row = value["distributions"][0]
            if kind == "extension":
                row["releases"][0]["components"].append("extensions")
            else:
                row["release_policies"][0]["targets"][0]["app_binding"] = {"app_id": "other"}
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                case(value)

    def test_source_pins_are_complete_and_safe(self):
        for field, bad in (("revision", "main"), ("path", "../escape"), ("path", "plugins//portable"), ("repository", "evil.example/x/y")):
            value = snapshot()
            value["distributions"][0]["releases"][0]["package_source"][field] = bad
            with self.subTest(field=field, bad=bad), self.assertRaises(ValueError):
                case(value)

    def test_exact_release_digest_required(self):
        value = snapshot()
        value["distributions"][0]["releases"][0]["tree_digest"] = "sha256:short"
        with self.assertRaises(ValueError):
            case(value)

    def test_duplicate_distribution_rejected(self):
        value = snapshot()
        value["distributions"].append(copy.deepcopy(value["distributions"][0]))
        with self.assertRaises(ValueError):
            case(value)


class ReceiptTests(unittest.TestCase):
    def test_exact_saved_receipt(self):
        installation, binding, receipt = audit.checked_binding(state(), case())
        self.assertEqual(installation["installation_id"], "install")
        self.assertEqual(binding["client_binding_id"], receipt["client_binding_id"])

    def test_source_repository_revision_and_subpath_are_not_interchangeable(self):
        for field in ("repository", "resolved_revision", "package_subpath"):
            value = state()
            value["installations"][0]["source"][field] = "different"
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.checked_binding(value, case())

    def test_binding_scope_digest_and_materialization_fail_closed(self):
        for field in ("client_id", "scope", "materialization", "tree_digest", "manifest_digest", "resolved_revision"):
            value = state()
            binding = value["installations"][0]["clients"]["binding"]
            target = binding["package_revision"] if field.endswith("digest") or field == "resolved_revision" else binding
            target[field] = "different"
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.checked_binding(value, case())

    def test_saved_ownership_required(self):
        for field in ("phase", "installation_id", "client_binding_id", "environment_id", "source_key", "package_digest"):
            value = state()
            value["environment_receipts"][0][field] = "different"
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.checked_binding(value, case())

    def test_state_cannot_be_supplied_by_an_acquired_package(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            foreign = root / "uap-data/cache/acquired-package/state-v2.json"
            foreign.parent.mkdir(parents=True)
            foreign.write_text(json.dumps(state()), encoding="utf-8")
            self.assertEqual(audit.state_file(root, "user"), {"installations": []})


class SemanticProofTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        pin = audit.live.MARKETING_KIT_SIGNED_CLOSURE["777genius/notion"]
        self.selected = {"distribution": "777genius/notion", "release_sequence": 1, "name": "notion",
                         "source": {"repository": pin["repository"], "revision": pin["resolved_revision"], "path": pin["package_path"]},
                         "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"],
                         "components": ["mcp"], "client": "pi", "scope": "project"}
        self.documents = [self.root / "uap.toml", self.root / "uap.lock"]
        self.manifest = ('schema_version = 1\n[dependencies.notion]\n'
                         'git = "https://github.com/777genius/universal-agent-plugins.git"\n'
                         'path = "plugins/notion"\ncommit = "2ddbb99dd190c1792b79904f9875e6322bccd243"\n'
                         '[targets]\npi = ["notion"]\n')
        self.lock = {"lock_version": 1,
                     "manifest_digest": "sha256:babf3e66cc3f91ba5339d085a688ec287d700ba2c14dafb97e2203ff40269a67",
                     "packages": [{"alias": "notion", "targets": ["pi"], "version": "0.1.0",
                         "source": {"git": {"repository": pin["repository"], "path": pin["package_path"],
                             "requested": {"kind": "commit", "value": pin["resolved_revision"]}, "commit": pin["resolved_revision"]}},
                         "source_key": "github:777genius/universal-agent-plugins:plugins/notion",
                         "digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": pin["tree_digest"],
                         "manifest_digest": pin["manifest_digest"]}]}
        self.write_saved()

    def write_saved(self, manifest=None, locked=None):
        self.documents[0].write_text(self.manifest if manifest is None else manifest, encoding="utf-8")
        self.documents[1].write_text(json.dumps(self.lock if locked is None else locked), encoding="utf-8")

    def saved(self, alias="notion"):
        return audit.checked_saved_intent(self.documents, alias, self.selected, self.root)

    def native(self, client, scope="user"):
        selected = {**self.selected, "client": client, "scope": scope}
        native_root = self.root / (client + "-" + scope)
        locator = native_root / "plugins/notion" if client == "antigravity" else self.root / (client + "-managed-" + scope)
        locator.mkdir(parents=True, exist_ok=True)
        native_root.mkdir(exist_ok=True)
        binding = {"client_id": client, "scope": scope, "target_locator": str(locator), "physical_artifact_id": "notion-physical",
                   "native_objects": [{"object_id": "package:" + client + ":notion-physical", "kind": "managed_package_directory",
                        "logical_name": "notion", "path": str(locator), "protection_class": "managed", "managed_digest": "sha256:" + "d" * 64}]}
        collection = "mcp_servers" if client == "hermes" else "mcpServers"
        entry = {"serverUrl" if client == "antigravity" else "url": "https://mcp.notion.com/mcp"}
        path = locator / "mcp_config.json" if client == "antigravity" else native_root / ("config.yaml" if client == "hermes" else "mcp.json")
        if client == "antigravity":
            (locator / "plugin.json").write_text(json.dumps({"$schema": "https://antigravity.google/schemas/v1/plugin.json",
                "name": "notion", "displayName": "notion", "version": "0.1.0"}), encoding="utf-8")
        else:
            kind = "hermes_user_mcp" if client == "hermes" else f"pi_{scope}_mcp_server"
            binding["native_objects"].append({"object_id": kind + ":notion", "kind": kind, "logical_name": "notion",
                                              "path": str(path), "protection_class": "managed"})
        value = {collection: {"notion": entry, "foreign": {"url": "https://mcp.notion.com/mcp"}}}
        self.write_native(path, client, value)
        args = native_root, locator, selected, binding, self.root
        return args, path, value, collection

    @staticmethod
    def write_native(path, client, value):
        body = audit.yaml.safe_dump(value) if client == "hermes" else json.dumps(value)
        path.write_text(body, encoding="utf-8")

    def test_exact_saved_pin_and_native_formats_are_observed(self):
        self.assertEqual(self.saved()["alias"], "notion")
        for client, scope in (("hermes", "user"), ("pi", "user"), ("pi", "project"),
                              ("antigravity", "user"), ("antigravity", "project")):
            with self.subTest(client=client, scope=scope):
                args, path, _, _ = self.native(client, scope)
                proof = audit.checked_native_materialization(*args)
                self.assertEqual(proof["path"], path.relative_to(self.root).as_posix())
                self.assertEqual(proof["logical_name"], "notion")

    def test_saved_alias_and_manifest_mutations_after_first_observation_refuse(self):
        self.saved()
        with self.assertRaisesRegex(ValueError, "alias"):
            self.saved("display-name")
        for changed in (self.manifest.replace("commit =", "ref ="),
                        self.manifest.replace('commit = "2ddbb99dd190c1792b79904f9875e6322bccd243"\n', ""),
                        self.manifest.replace('pi = ["notion"]', 'hermes = ["notion"]'),
                        self.manifest.replace('plugins/notion', 'plugins/foreign'),
                        self.manifest.replace('2ddbb99dd190c1792b79904f9875e6322bccd243', 'a' * 40),
                        self.manifest + 'cursor = ["notion"]\n',
                        self.manifest + '[dependencies.foreign]\nregistry = "uap"\n',
                        self.manifest.replace('git =', 'archive =')):
            with self.subTest(manifest=changed), self.assertRaisesRegex(ValueError, "saved manifest"):
                self.write_saved(manifest=changed)
                self.saved()

    def test_moving_requested_intent_cannot_pass_with_same_resolved_commit(self):
        for replacement in ({"kind": "ref", "value": "main"}, {"kind": "default"}):
            locked = copy.deepcopy(self.lock)
            locked["packages"][0]["source"]["git"]["requested"] = replacement
            with self.subTest(requested=replacement), self.assertRaisesRegex(ValueError, "requested Git source"):
                self.write_saved(locked=locked)
                self.saved()

    def test_lock_source_and_digests_mutated_after_first_observation_refuse(self):
        self.saved()
        for field in ("alias", "targets", "source_key", "digest_algorithm", "tree_digest", "manifest_digest"):
            locked = copy.deepcopy(self.lock)
            locked["packages"][0][field] = "different"
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.write_saved(locked=locked)
                self.saved()
        for field in ("repository", "path", "requested", "commit"):
            locked = copy.deepcopy(self.lock)
            locked["packages"][0]["source"]["git"][field] = "different"
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.write_saved(locked=locked)
                self.saved()

    def test_lock_extra_root_alternative_source_and_stale_intent_refuse(self):
        for change in (lambda value: value["packages"].append(copy.deepcopy(value["packages"][0])),
                       lambda value: value["packages"][0]["source"].update({"registry": {}}),
                       lambda value: value.update({"manifest_digest": "sha256:" + "0" * 64})):
            locked = copy.deepcopy(self.lock)
            change(locked)
            with self.subTest(lock=locked), self.assertRaises(ValueError):
                self.write_saved(locked=locked)
                self.saved()

    def test_foreign_matching_url_does_not_prove_missing_or_wrong_owned_entry(self):
        for client in audit.CLIENTS:
            args, path, value, collection = self.native(client)
            audit.checked_native_materialization(*args)
            for replacement in (None, {}, {"url": "https://foreign.example/mcp"},
                                {"url": "https://mcp.notion.com/mcp", "command": "foreign"}):
                changed = copy.deepcopy(value)
                if replacement is None:
                    del changed[collection]["notion"]
                else:
                    changed[collection]["notion"] = replacement
                with self.subTest(client=client, entry=replacement), self.assertRaises(ValueError):
                    self.write_native(path, client, changed)
                    audit.checked_native_materialization(*args)

    def test_portable_source_copy_cannot_replace_selected_native_config(self):
        for client in audit.CLIENTS:
            args, path, _, _ = self.native(client)
            path.unlink()
            (args[1] / "mcp.json").write_text('{"mcpServers":{"notion":{"type":"streamable-http","url":"https://mcp.notion.com/mcp"}}}', encoding="utf-8")
            with self.subTest(client=client), self.assertRaisesRegex(ValueError, "native MCP config is absent"):
                audit.checked_native_materialization(*args)

    def test_hermes_native_yaml_rejects_duplicate_keys(self):
        valid = b"mcp_servers:\n  notion:\n    url: https://mcp.notion.com/mcp\n"
        self.assertEqual(audit.strict_native_yaml(valid),
                         {"mcp_servers": {"notion": {"url": "https://mcp.notion.com/mcp"}}})
        for case in (
            b"mcp_servers:\n  notion:\n    url: https://evil.invalid/mcp\n  notion:\n    url: https://mcp.notion.com/mcp\n",
            b"mcp_servers:\n  notion:\n    url: https://evil.invalid/mcp\nmcp_servers:\n  notion:\n    url: https://mcp.notion.com/mcp\n",
            b"mcp_servers:\n  notion:\n    url: https://evil.invalid/mcp\n    url: https://mcp.notion.com/mcp\n",
            b"mcp_servers:\n  1: invalid\n",
        ):
            with self.subTest(case=case), self.assertRaisesRegex(ValueError, "duplicate or nonstring native YAML key"):
                audit.strict_native_yaml(case)

    def test_owned_native_path_identity_scope_and_transport_are_required(self):
        for client in ("hermes", "pi"):
            args, path, _, _ = self.native(client)
            for field in ("path", "logical_name", "kind", "object_id", "protection_class"):
                binding = copy.deepcopy(args[3])
                binding["native_objects"][1][field] = "different"
                with self.subTest(client=client, field=field), self.assertRaises(ValueError):
                    audit.checked_native_materialization(*args[:3], binding, self.root)
            binding = {**args[3], "scope": "project"}
            with self.assertRaises(ValueError):
                audit.checked_native_materialization(*args[:3], binding, self.root)

    def test_antigravity_manifest_default_load_contract_is_required(self):
        args, _, _, _ = self.native("antigravity")
        manifest = args[1] / "plugin.json"
        original = json.loads(manifest.read_text(encoding="utf-8"))
        for field, replacement in (("name", "foreign"), ("$schema", "unknown"), ("version", "2.0.0"), ("mcp", "foreign.json")):
            with self.subTest(field=field), self.assertRaises(ValueError):
                manifest.write_text(json.dumps({**original, field: replacement}), encoding="utf-8")
                audit.checked_native_materialization(*args)

    def test_generic_directory_ownership_and_extra_native_members_cannot_be_ignored(self):
        for client in audit.CLIENTS:
            args, _, _, _ = self.native(client)
            for field in ("object_id", "logical_name", "path", "kind", "protection_class", "managed_digest"):
                binding = copy.deepcopy(args[3])
                binding["native_objects"][0][field] = "foreign"
                with self.subTest(client=client, field=field), self.assertRaises(ValueError):
                    audit.checked_native_materialization(*args[:3], binding, self.root)
            for change in (lambda value: value["native_objects"].pop(0),
                           lambda value: value["native_objects"].append({"kind": "foreign", "path": str(args[1])}),
                           lambda value: value["native_objects"].append(copy.deepcopy(value["native_objects"][0])),
                           lambda value: value.update({"physical_artifact_id": "another-artifact"})):
                binding = copy.deepcopy(args[3])
                change(binding)
                with self.subTest(client=client, binding=binding), self.assertRaises(ValueError):
                    audit.checked_native_materialization(*args[:3], binding, self.root)

    def test_unreviewed_source_cannot_borrow_the_notion_native_oracle(self):
        args, _, _, _ = self.native("pi")
        for field in ("distribution", "source", "tree_digest", "manifest_digest", "components"):
            selected = {**args[2], field: "different"}
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.checked_native_materialization(*args[:2], selected, *args[3:])

    def test_semantic_reads_preserve_hardlink_refusal(self):
        args, path, _, _ = self.native("pi")
        for owned in (self.documents[0], self.documents[1], path):
            link = owned.with_name(owned.name + ".other")
            os.link(owned, link)
            try:
                with self.subTest(file=owned.name), self.assertRaisesRegex(ValueError, "single-link"):
                    self.saved() if owned in self.documents else audit.checked_native_materialization(*args)
            finally:
                link.unlink()


class SavedScopeTests(unittest.TestCase):
    def test_other_scope_documents_and_state_are_observed_but_shared_store_is_not(self):
        for scope in ("user", "project"):
            with self.subTest(scope=scope), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                native = root / "other-native"
                native.mkdir()
                documents = root / ("project" if scope == "user" else "uap-data")
                mutable = documents / ".uap" if scope == "user" else documents
                mutable.mkdir(parents=True)
                before = audit.protected_scope_snapshot(root, scope, native)
                for path in (documents / "uap.toml", documents / "uap.lock", mutable / "state-v2.json",
                             mutable / "operations-v2/receipt.json", mutable / "managed/plugin/plugin.json"):
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text("unwanted write", encoding="utf-8")
                    self.assertNotEqual(before, audit.protected_scope_snapshot(root, scope, native), str(path))
                    path.unlink()
                    while path.parent != mutable and path.parent != documents:
                        path.parent.rmdir()
                        path = path.parent
                cache = root / "uap-data/store/package"
                cache.mkdir(parents=True, exist_ok=True)
                (cache / "plugin.json").write_text("shared immutable cache", encoding="utf-8")
                self.assertEqual(before, audit.protected_scope_snapshot(root, scope, native))

    def test_saved_records_refuse_hardlinks_and_unbounded_bytes(self):
        for variant in ("hardlink", "oversized"):
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                native = root / "other-native"
                native.mkdir()
                documents = root / "project"
                documents.mkdir()
                original = root / "control.txt"
                original.write_text("original fixture", encoding="utf-8")
                selected = documents / "uap.toml"
                if variant == "hardlink":
                    os.link(original, selected)
                else:
                    selected.write_bytes(b"x" * ((1 << 20) + 1))
                with self.assertRaisesRegex(ValueError, "bounded single-link regular file"):
                    audit.protected_scope_snapshot(root, "user", native)

    def test_saved_record_refuses_symlink_even_to_in_case_file(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            native = root / "other-native"
            native.mkdir()
            documents = root / "project"
            documents.mkdir()
            original = root / "control.txt"
            original.write_text("original fixture", encoding="utf-8")
            selected = documents / "uap.toml"
            try:
                selected.symlink_to(original)
            except (OSError, NotImplementedError) as error:
                self.skipTest("native symlink creation unavailable: " + str(error))
            with self.assertRaisesRegex(ValueError, "bounded single-link regular file"):
                audit.read_saved_record(selected)
            with self.assertRaisesRegex(ValueError, "native path escaped disposable case"):
                audit.protected_scope_snapshot(root, "user", native)

    def test_saved_record_preserves_exact_crlf_bytes_and_absence(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "uap.toml"
            self.assertEqual(audit.read_saved_record(path), (None, None))
            body = b"schema_version = 1\r\n[dependencies]\r\n[targets]\r\n"
            path.write_bytes(body)
            observed, identity = audit.read_saved_record(path)
            self.assertEqual(observed, body)
            self.assertEqual(identity.st_size, len(body))

    @staticmethod
    def stat_with(value, **changes):
        fields = {name: getattr(value, name) for name in dir(value) if name.startswith("st_")}
        fields.update(changes)
        return SimpleNamespace(**fields)

    def test_saved_record_allows_distinct_path_and_descriptor_ctime(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "uap.toml"
            body = b"schema_version = 1\r\n[dependencies]\r\n[targets]\r\n"
            path.write_bytes(body)
            path_ctime = path.lstat().st_ctime_ns
            original_fstat = os.fstat

            def stat_descriptor(fd):
                return self.stat_with(original_fstat(fd), st_ctime_ns=path_ctime + 1)

            with mock.patch.object(audit.os, "fstat", side_effect=stat_descriptor) as observed:
                actual, identity = audit.read_saved_record(path)
            self.assertEqual(actual, body)
            self.assertEqual(identity.st_ctime_ns, path_ctime)
            self.assertEqual(observed.call_count, 2)

    def test_saved_record_refuses_descriptor_ctime_change_during_read(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "uap.toml"
            path.write_bytes(b"unchanged bytes")
            original_fstat = os.fstat
            changes = iter((0, 1))

            def stat_descriptor(fd):
                value = original_fstat(fd)
                return self.stat_with(value, st_ctime_ns=value.st_ctime_ns + next(changes))

            with mock.patch.object(audit.os, "fstat", side_effect=stat_descriptor):
                with self.assertRaisesRegex(ValueError, "changed during read"):
                    audit.read_saved_record(path)

    def test_saved_record_refuses_path_ctime_change_during_read(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / "uap.toml"
            path.write_bytes(b"unchanged bytes")
            original_lstat = type(path).lstat
            changes = iter((0, 1))

            def stat_path(selected):
                value = original_lstat(selected)
                return self.stat_with(value, st_ctime_ns=value.st_ctime_ns + next(changes))

            with mock.patch.object(type(path), "lstat", stat_path):
                with self.assertRaisesRegex(ValueError, "changed during read"):
                    audit.read_saved_record(path)

    def test_first_command_cannot_modify_the_other_scope_then_hide_it_at_remove(self):
        for path in ("home/.pi/agent/unwanted.json", "uap-data/uap.toml", "uap-data/state-v2.json"):
            with self.subTest(path=path), tempfile.TemporaryDirectory() as temp:
                root = Path(temp) / "case"

                def called(*args, **kwargs):
                    (root / path).write_text("unwanted write", encoding="utf-8")
                    return {"exit_code": 0, "timeout": False, "_json": {}}

                with mock.patch.object(audit.live, "call", side_effect=called), \
                     mock.patch.object(audit.live, "initialize_project_skill_git_root"):
                    result = audit.run_case(Path("fixture-cli"), case(), root, None)
                self.assertEqual(result["status"], "failed")
                self.assertEqual(result["reason"], "add changed the other scope")
                self.assertEqual([row["operation"] for row in result["trace"]], ["add"])

    def test_saved_removal_rejects_dependency_target_or_lock_residue(self):
        manifest = "schema_version = 1\n[dependencies]\n[targets]\n"
        source_key = "github:fixture/catalog:plugins/portable"
        mutations = (
            (manifest, {"lock_version": 1, "packages": []}, None),
            ('schema_version = 1\n[dependencies]\nportable = "*"\n',
             {"lock_version": 1, "packages": []}, "saved dependency"),
            ('schema_version = 1\n[targets]\npi = ["portable"]\n',
             {"lock_version": 1, "packages": []}, "target assignment"),
            (manifest, {"lock_version": 1, "packages": [{"alias": "portable"}]}, "locked package"),
            (manifest, {"lock_version": 1, "packages": [{"alias": "other", "source_key": source_key}]}, "locked package"),
            (manifest, {"lock_version": 1}, "invalid lock"),
        )
        for body, lock, failure in mutations:
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                documents = [root / "uap.toml", root / "uap.lock"]
                documents[0].write_text(body, encoding="utf-8")
                documents[1].write_text(json.dumps(lock), encoding="utf-8")
                if failure:
                    with self.assertRaisesRegex(ValueError, failure):
                        audit.checked_saved_removal(documents, "portable", case(), root)
                else:
                    audit.checked_saved_removal(documents, "portable", case(), root)


    def test_saved_removal_rejects_duplicate_lock_keys(self):
        # Last-write-wins JSON could hide a retained dependency before remove.
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            manifest, lock = root / "uap.toml", root / "uap.lock"
            manifest.write_text("schema_version = 1\n[dependencies]\n[targets]\n", encoding="utf-8")
            lock.write_text('{"lock_version":1,"packages":[{"alias":"portable"}],"packages":[]}',
                            encoding="utf-8")
            with self.assertRaisesRegex(ValueError, "duplicate JSON field"):
                audit.checked_saved_removal([manifest, lock], "portable", case(), root)

    def test_saved_removal_refuses_hardlinked_or_oversized_lock(self):
        for variant in ("hardlink", "oversized"):
            with self.subTest(variant=variant), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                manifest, lock = root / "uap.toml", root / "uap.lock"
                manifest.write_text("schema_version = 1\n[dependencies]\n[targets]\n", encoding="utf-8")
                foreign = root / "foreign.lock"
                foreign.write_text('{"lock_version":1,"packages":[]}', encoding="utf-8")
                if variant == "hardlink":
                    os.link(foreign, lock)
                else:
                    lock.write_bytes(b"x" * ((1 << 20) + 1))
                with self.assertRaisesRegex(ValueError, "bounded single-link regular file"):
                    audit.checked_saved_removal([manifest, lock], "portable", case(), root)

    def test_saved_removal_refuses_symlinked_lock(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            manifest, lock = root / "uap.toml", root / "uap.lock"
            manifest.write_text("schema_version = 1\n[dependencies]\n[targets]\n", encoding="utf-8")
            foreign = root / "foreign.lock"
            foreign.write_text('{"lock_version":1,"packages":[]}', encoding="utf-8")
            try:
                lock.symlink_to(foreign)
            except (OSError, NotImplementedError) as error:
                self.skipTest("native symlink creation unavailable: " + str(error))
            with self.assertRaisesRegex(ValueError, "bounded single-link regular file"):
                audit.read_saved_record(lock)
            with self.assertRaisesRegex(ValueError, "native path escaped disposable case"):
                audit.checked_saved_removal([manifest, lock], "portable", case(), root)


class RefusalTests(unittest.TestCase):
    selected = {"distribution": "777genius/marketing-kit", "name": "marketing-kit"}

    @staticmethod
    def output(message):
        return {"exit_code": 1, "timeout": False, "stdout_full": "", "stderr_full": message,
                "stdout_tail": "", "stderr_tail": message[-1800:]}

    def test_exact_required_distribution_and_version(self):
        for dependency, version in (("hubspot-crm", "0.1.0"), ("notion", "0.1.0"), ("playwright-bridge", "0.0.80")):
            message = f"agentplugins: dependency conflict for directory:777genius/{dependency}: marketing-kit version {version}\n"
            refused = audit.checked_dependency_refusal(self.output(message), self.selected)
            self.assertEqual(refused["distribution"], "777genius/" + dependency)
            self.assertEqual(refused["version"], version)

    def test_unrelated_failures_do_not_certify_dependency_refusal(self):
        for message in (
            "agentplugins: no compatible candidate for 777genius/marketing-kit",
            "agentplugins: fetch 777genius/notion failed while resolving dependency",
            "agentplugins: dependency conflict for directory:777genius/marketing-kit: marketing-kit version 0.1.0",
            "agentplugins: dependency conflict for directory:777genius/other: marketing-kit version 0.1.0",
            "agentplugins: dependency conflict for directory:777genius/notion: marketing-kit version 0.2.0",
            "agentplugins: dependency conflict for directory:777genius/notion: unrelated version 0.1.0",
            "transport error: dependency conflict for directory:777genius/notion: marketing-kit version 0.1.0",
        ):
            with self.subTest(message=message), self.assertRaises(ValueError):
                audit.checked_dependency_refusal(self.output(message), self.selected)

    def test_timeout_success_or_output_injection_is_not_refusal(self):
        message = "agentplugins: dependency conflict for directory:777genius/notion: marketing-kit version 0.1.0"
        for field, value in (("timeout", True), ("exit_code", 0), ("exit_code", None), ("stdout_full", "unrelated failure")):
            output = self.output(message)
            output[field] = value
            with self.subTest(field=field), self.assertRaises(ValueError):
                audit.checked_dependency_refusal(output, self.selected)

    def test_truncated_tail_cannot_hide_an_unrelated_error(self):
        message = "agentplugins: dependency conflict for directory:777genius/notion: marketing-kit version 0.1.0"
        output = self.output(message)
        output["stderr_full"] = "unrelated acquisition failure\n" + "x" * 2000 + "\n" + message
        with self.assertRaises(ValueError):
            audit.checked_dependency_refusal(output, self.selected)
        del output["stderr_full"]
        with self.assertRaises(ValueError):
            audit.checked_dependency_refusal(output, self.selected)


class NativeExtractionTests(unittest.TestCase):
    @staticmethod
    def archive(kind, size=64, names=None, symlink=False, prefix=None):
        prefix = prefix if prefix is not None else (b"MZ" if kind == "zip" else b"\x7fELF")
        names = names or ["antigravity.exe" if kind == "zip" else "antigravity"]
        output = io.BytesIO()

        class Payload:
            position = 0

            def read(self, count):
                count = min(count, size - self.position)
                head = prefix[self.position:self.position + count]
                self.position += count
                return head + b"\0" * (count - len(head))

        if kind == "zip":
            with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED) as package:
                for name in names:
                    item = zipfile.ZipInfo(name)
                    item.compress_type = zipfile.ZIP_DEFLATED
                    item.external_attr = (0o120777 if symlink else 0o600) << 16
                    source = Payload()
                    with package.open(item, "w") as stream:
                        while block := source.read(1 << 20):
                            stream.write(block)
        else:
            with tarfile.open(fileobj=output, mode="w:gz") as package:
                for name in names:
                    item = tarfile.TarInfo(name)
                    item.size = size
                    if symlink:
                        item.type, item.linkname, item.size = tarfile.SYMTYPE, "outside", 0
                    package.addfile(item, Payload())
        return output.getvalue()

    def test_real_pinned_vendor_sizes_above_compressed_archive_bound(self):
        # Actual metadata of the SHA256-pinned 1.5.3 Windows/Linux archives.
        # Stream compressible fixture payloads to avoid giant test input files.
        for kind, size in (("zip", 225159320), ("tar.gz", 246837648)):
            with self.subTest(kind=kind):
                body = self.archive(kind, size)
                self.assertLess(len(body), audit.MAX_NATIVE_ARCHIVE_BYTES)
                executable = audit.extract_antigravity_executable(body, "native." + kind)
                self.assertEqual(len(executable), size)
                del executable

    def test_macos_native_magic_is_preserved(self):
        body = self.archive("tar.gz", prefix=b"\xcf\xfa\xed\xfe")
        self.assertTrue(audit.extract_antigravity_executable(body, "native.tar.gz").startswith(b"\xcf\xfa\xed\xfe"))

    def test_multiple_executables_never_select_the_first(self):
        for kind, names in (("zip", ["antigravity.exe", "updater.exe"]),
                            ("tar.gz", ["antigravity", "updater"])):
            with self.subTest(kind=kind), self.assertRaisesRegex(ValueError, "exactly one executable"):
                audit.extract_antigravity_executable(self.archive(kind, names=names), "native." + kind)

    def test_payload_bound_has_a_distinct_error(self):
        for kind in ("zip", "tar.gz"):
            with self.subTest(kind=kind), mock.patch.object(audit, "MAX_NATIVE_EXECUTABLE_BYTES", 64):
                with self.assertRaisesRegex(ValueError, "exceeds payload size bound"):
                    audit.extract_antigravity_executable(self.archive(kind, size=65), "native." + kind)

    def test_symlink_or_non_native_payload_is_not_an_executable(self):
        for kind in ("zip", "tar.gz"):
            for options in ({"symlink": True}, {"prefix": b"text"}):
                with self.subTest(kind=kind, options=options), self.assertRaises(ValueError):
                    audit.extract_antigravity_executable(self.archive(kind, **options), "native." + kind)


class NativeCleanupTests(unittest.TestCase):
    def test_empty_owned_entries_and_foreign_config_survive(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            native = root / ".pi"
            native.mkdir()
            (native / "mcp.json").write_text('{"mcpServers":{"foreign":{"url":"https://portable.example/mcp"}}}', encoding="utf-8")
            result = audit.checked_native_cleanup(native, case(), {"native_objects": [{"logical_name": "owned-api"}]}, root)
            self.assertEqual(result["remaining"], [])

    def test_new_json_or_yaml_owned_mcp_entry_blocks_pass(self):
        for filename, text in (("mcp.json", '{"mcpServers":{"owned-api":{"url":"https://example.test"}}}'),
                               ("config.yaml", 'mcp_servers:\n  owned-api:\n    url: https://example.test\n')):
            with self.subTest(filename=filename), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                native = root / "profile"
                native.mkdir()
                (native / filename).write_text(text, encoding="utf-8")
                # This file was absent before add, and its server identity
                # differs from the package name. Both were former blind spots.
                binding = {"native_objects": [{"logical_name": "owned-api", "path": str(native / filename)}]}
                with self.assertRaisesRegex(ValueError, "owned native entries remain"):
                    audit.checked_native_cleanup(native, case(), binding, root)

    def test_retained_package_file_blocks_pass(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            native = root / ".agents"
            package = native / "plugins/portable"
            package.mkdir(parents=True)
            (package / "plugin.json").write_text('{"name":"portable"}', encoding="utf-8")
            with self.assertRaises(ValueError):
                audit.checked_native_cleanup(native, case(), {}, root)


if __name__ == "__main__":
    unittest.main()
