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


class RuntimeLineageDiagnosticTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.selected = {**case(), "expected": "dependency-refusal", "name": "marketing-kit",
                         "distribution": "777genius/marketing-kit"}
        self.path = self.root / "project/.uap/state-v2.json"
        self.path.parent.mkdir(parents=True)

    def make_state(self):
        source = self.selected["source"]
        records = [{"origin_mode": "direct", "declared_name": "marketing-kit",
                    "source": {"repository": source["repository"], "package_subpath": source["path"],
                               "resolved_revision": source["revision"], "private": "secret-source"}}]
        for name, distribution in (("notion", "777genius/notion"), ("hubspot-crm", "777genius/hubspot-crm"),
                                   ("playwright", "777genius/playwright-bridge")):
            records.append({"origin_mode": "directory", "declared_name": name,
                            "directory": {"distribution_id": distribution, "snapshot_schema": 1,
                                          "snapshot_sequence": 49, "snapshot_digest": "sha256:" + "a" * 64,
                                          "private": "/secret/private/path"}})
        return {"schema_version": 5, "installations": records}

    def write_state(self, state):
        self.path.write_text(json.dumps(state), encoding="utf-8")

    def observe(self):
        return audit.declared_runtime_lineage(self.root, self.selected)

    def test_only_unverified_allowlisted_facts_are_returned(self):
        self.write_state(self.make_state())
        result = self.observe()
        self.assertEqual(result["status"], "unresolved")
        self.assertEqual(result["reason"], "unverified_local_state")
        self.assertTrue(result["declared_only"])
        self.assertEqual(len(result["records"]), 4)
        self.assertTrue(all(record["expected_identity_match"] for record in result["records"]))
        self.assertEqual(result["records"][1]["snapshot_sequence"], 49)
        self.assertEqual(result["records"][1]["snapshot_digest"], "sha256:" + "a" * 64)
        self.assertNotIn("secret", json.dumps(result))
        self.assertNotIn(str(self.root), json.dumps(result))
        self.assertNotIn(self.selected["source"]["repository"], json.dumps(result))

    def test_missing_malformed_duplicate_and_oversize_are_unresolved(self):
        self.assertEqual(self.observe()["reason"], "state_unavailable_or_invalid")
        for body in ('{', '{"schema_version":5,"installations":[],"installations":[]}',
                     '{"schema_version":5,"installations":[{"directory":{"x":1,"x":2}}]}',
                     '{"schema_version":5,"installations":[],"ignored":NaN}',
                     '{"schema_version":5,"installations":[],"ignored":{"nested":Infinity}}',
                     '{"schema_version":5,"installations":[],"ignored":[-Infinity]}',
                     '{"schema_version":5,"installations":' + ' ' * (1 << 20) + '[]}'):
            with self.subTest(prefix=body[:50]):
                self.path.write_text(body, encoding="utf-8")
                self.assertEqual(self.observe(), {"status": "unresolved", "reason": "state_unavailable_or_invalid"})

    def test_symlink_and_hardlink_state_are_unresolved(self):
        outside = self.root / "outside-state.json"
        outside.write_text(json.dumps(self.make_state()), encoding="utf-8")
        try:
            self.path.symlink_to(outside)
        except OSError as error:
            self.skipTest(str(error))
        self.assertEqual(self.observe()["reason"], "state_unavailable_or_invalid")
        self.path.unlink()
        try:
            os.link(outside, self.path)
        except OSError as error:
            self.skipTest(str(error))
        self.assertEqual(self.observe()["reason"], "state_unavailable_or_invalid")

    @unittest.skipUnless(os.open in os.supports_dir_fd and hasattr(os, "O_DIRECTORY")
                         and hasattr(os, "O_NOFOLLOW"), "held directory descriptors unavailable")
    def test_swapped_ancestor_is_not_read_or_disclosed(self):
        self.write_state(self.make_state())
        with tempfile.TemporaryDirectory() as foreign:
            foreign_dir = Path(foreign)
            other = self.make_state()
            other["installations"][1]["directory"]["snapshot_sequence"] = 987654
            (foreign_dir / "state-v2.json").write_text(json.dumps(other), encoding="utf-8")
            original_open = os.open
            swapped = []
            def intercepted(name, *args, **kwargs):
                if name == ".uap" and kwargs.get("dir_fd") is not None and not swapped:
                    self.path.parent.rename(self.root / "project/original-uap")
                    self.path.parent.symlink_to(foreign_dir, target_is_directory=True)
                    swapped.append(True)
                return original_open(name, *args, **kwargs)
            with mock.patch.object(audit.live.os, "open", side_effect=intercepted) as hooked:
                with mock.patch.object(audit.live.os, "supports_dir_fd", {hooked}):
                    result = self.observe()
            self.assertTrue(swapped)
            self.assertEqual(result, {"status": "unresolved", "reason": "state_unavailable_or_invalid"})
            self.assertNotIn("987654", json.dumps(result))
            self.assertNotIn(foreign, json.dumps(result))

    def test_bounded_unknown_and_mismatched_identities_are_redacted(self):
        value = self.make_state()
        value["installations"][0]["source"]["repository"] = "secret-foreign"
        value["installations"][1]["directory"]["distribution_id"] = "/secret/foreign"
        value["installations"] += [copy.deepcopy(value["installations"][2]) for _ in range(9)]
        self.write_state(value)
        result = self.observe()
        self.assertEqual(len(result["records"]), 8)
        self.assertEqual(result["total_installations"], 13)
        self.assertTrue(result["truncated"])
        self.assertFalse(result["records"][0]["expected_identity_match"])
        self.assertEqual(result["records"][1]["distribution_id"], "<other>")
        self.assertFalse(result["records"][1]["expected_identity_match"])
        self.assertNotIn("secret", json.dumps(result))

    def test_invalid_snapshot_fields_are_unresolved(self):
        for field, replacement in (("snapshot_digest", "sha256:secret"), ("snapshot_sequence", True),
                                   ("snapshot_sequence", 1 << 64), ("snapshot_schema", 7)):
            value = self.make_state()
            value["installations"][1]["directory"][field] = replacement
            self.write_state(value)
            with self.subTest(field=field, replacement=replacement):
                self.assertEqual(self.observe()["reason"], "state_unavailable_or_invalid")

    def test_state_changed_after_held_read_is_unresolved(self):
        self.write_state(self.make_state())
        original_read = audit.live.copilot_settings_read
        def changed(path, root):
            result = original_read(path, root)
            path.write_text('{"schema_version":5,"installations":[]}', encoding="utf-8")
            return result
        with mock.patch.object(audit.live, "copilot_settings_read", side_effect=changed):
            self.assertEqual(self.observe()["reason"], "state_unavailable_or_invalid")

    def test_user_scope_reads_only_the_case_owned_user_state(self):
        self.write_state(self.make_state())
        user = {**self.selected, "scope": "user"}
        self.assertEqual(audit.declared_runtime_lineage(self.root, user)["reason"],
                         "state_unavailable_or_invalid")
        path = self.root / "uap-data/state-v2.json"
        path.parent.mkdir()
        path.write_text(json.dumps(self.make_state()), encoding="utf-8")
        self.assertEqual(audit.declared_runtime_lineage(self.root, user)["reason"], "unverified_local_state")

    def test_successful_add_includes_unresolved_lineage_and_stays_failed(self):
        root = self.root / "case"
        state = self.make_state()
        def fake_add(*args, **kwargs):
            path = root / "project/.uap/state-v2.json"
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps(state), encoding="utf-8")
            return {"exit_code": 0, "timeout": False, "_json": {"result": "success"},
                    "stdout_tail": "", "stderr_tail": ""}
        with mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
             mock.patch.object(audit.live, "call", side_effect=fake_add):
            receipt = audit.run_case(Path("/test-only-cli"), self.selected, root, None)
        self.assertEqual(receipt["status"], "failed")
        self.assertIn("inconclusive", receipt["reason"])
        self.assertEqual(receipt["declared_runtime_lineage"]["status"], "unresolved")
        self.assertEqual(len(receipt["declared_runtime_lineage"]["records"]), 4)
        self.assertFalse(receipt["runtime_qualified"])

    def test_scope_mismatch_reason_is_not_masked_by_lineage(self):
        root = self.root / "case"
        before = {"directories": {"metadata:0:foreign": (1, 2, 3, 4)}, "files": {}}
        after = {"directories": {"metadata:0:foreign": (1, 2, 3, 5)}, "files": {}}
        def fake_add(*args, **kwargs):
            path = root / "project/.uap/state-v2.json"
            path.parent.mkdir(parents=True)
            path.write_text(json.dumps(self.make_state()), encoding="utf-8")
            return {"exit_code": 0, "timeout": False, "_json": {"result": "success"},
                    "stdout_tail": "", "stderr_tail": ""}
        with mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
             mock.patch.object(audit.live, "call", side_effect=fake_add), \
             mock.patch.object(audit, "protected_scope_snapshot", side_effect=[before, after]):
            receipt = audit.run_case(Path("/test-only-cli"), self.selected, root, None)
        self.assertEqual(receipt["status"], "failed")
        self.assertEqual(receipt["reason"], "add changed the other scope")
        self.assertIn("protected_scope_changes", receipt)
        self.assertEqual(receipt["declared_runtime_lineage"]["status"], "unresolved")


class ProtectedScopeDiagnosticTests(unittest.TestCase):
    def test_bounded_paths_redact_values_and_distinguish_metadata(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            native = root / "home/.pi/agent"
            before = {"directories": {"metadata:0:foreign.json": (1, 2, 3, 4)}, "files": {}}
            after = {"directories": {"metadata:0:foreign.json": (1, 2, 3, 5),
                                      "link:0:../../PRIVATE": "secret link destination"},
                     "files": {f"uap-data/entry-{index}": {"digest": "private-value"}
                               for index in range(40)}}
            result = audit.protected_scope_changes(before, after, root, "project", native)
            self.assertEqual(result["total"], 42)
            self.assertEqual(len(result["entries"]), 32)
            self.assertTrue(result["truncated"])
            self.assertIn({"path": "home/.pi/agent/foreign.json", "kind": "changed",
                           "field": "metadata", "changed_fields": ["mtime"]}, result["entries"])
            self.assertIn({"path": "<redacted>", "kind": "added", "field": "entry"}, result["entries"])
            encoded = json.dumps(result)
            self.assertNotIn(temporary, encoded)
            self.assertNotIn("private-value", encoded)
            self.assertNotIn("secret link destination", encoded)
            self.assertNotIn("PRIVATE", encoded)

    def test_scope_mismatch_remains_failure_with_redacted_receipt(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "case"
            before = {"directories": {"metadata:0:foreign.json": (1, 2, 3, 4)}, "files": {}}
            after = {"directories": {"metadata:0:foreign.json": (1, 2, 3, 5)}, "files": {}}
            result = {"exit_code": 0, "timeout": False, "_json": {"result": "success"},
                      "stdout_tail": "", "stderr_tail": ""}
            with mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
                 mock.patch.object(audit.live, "call", return_value=result), \
                 mock.patch.object(audit, "protected_scope_snapshot", side_effect=[before, after]):
                receipt = audit.run_case(Path("/test-only-cli"), case(), root, None)
            self.assertEqual(receipt["status"], "failed")
            self.assertEqual(receipt["reason"], "add changed the other scope")
            changes = receipt["protected_scope_changes"]
            self.assertEqual(changes["entries"][0]["changed_fields"], ["mtime"])
            self.assertNotIn(temporary, json.dumps(changes))

    def test_successful_dependency_add_remains_inconclusive(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "case"
            selected = {**case(), "expected": "dependency-refusal"}
            result = {"exit_code": 0, "timeout": False, "_json": {"result": "success"},
                      "stdout_tail": "", "stderr_tail": ""}
            with mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
                 mock.patch.object(audit.live, "call", return_value=result):
                receipt = audit.run_case(Path("/test-only-cli"), selected, root, None)
            self.assertEqual(receipt["status"], "failed")
            self.assertIn("inconclusive", receipt["reason"])
            self.assertFalse(receipt["activation_qualified"])

    def test_exact_refusal_still_requires_no_scope_mutation(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary) / "case"
            selected = {**case(), "expected": "dependency-refusal", "name": "marketing-kit",
                        "distribution": "777genius/marketing-kit"}
            message = "agentplugins: dependency conflict for directory:777genius/notion: marketing-kit version 0.1.0"
            result = {"exit_code": 1, "timeout": False, "_json": None, "stdout_full": "",
                      "stderr_full": message, "stdout_tail": "", "stderr_tail": message}
            with mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
                 mock.patch.object(audit.live, "call", return_value=result):
                receipt = audit.run_case(Path("/test-only-cli"), selected, root, None)
            self.assertEqual(receipt["status"], "expected-dependency-refusal")
            self.assertEqual(receipt["refusal"]["distribution"], "777genius/notion")
            self.assertNotIn("protected_scope_changes", receipt)


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



class UnrelatedNativeMCPTests(unittest.TestCase):
    def test_native_foreign_mapping_survives_owned_add_and_refuses_mutations(self):
        for client in ("hermes", "pi"):
            with self.subTest(client=client), tempfile.TemporaryDirectory() as temp:
                root = Path(temp)
                profile = root / "selected"
                profile.mkdir()
                control = audit.seed_unrelated_native_mcp(profile, client, root)
                self.assertIsNotNone(control)
                audit.check_unrelated_native_mcp(control, root)
                path = control[1]
                if client == "hermes":
                    body = ("mcp_servers:\n  " + audit.FOREIGN_NATIVE_MCP +
                            ":\n    command: echo\n    args: [unrelated-native-audit]\n"
                            "  notion:\n    url: https://mcp.notion.com/mcp\n")
                    damaged = "mcp_servers: {}\n"
                else:
                    body = json.dumps({"mcpServers": {
                        audit.FOREIGN_NATIVE_MCP: control[2],
                        "notion": {"url": "https://mcp.notion.com/mcp"}}}) + "\n"
                    damaged = '{"mcpServers":{}}\n'
                path.write_text(body, encoding="utf-8")
                audit.check_unrelated_native_mcp(control, root)
                path.write_text(damaged, encoding="utf-8")
                with self.assertRaisesRegex(ValueError, "unrelated native MCP"):
                    audit.check_unrelated_native_mcp(control, root)
                path.unlink()
                with self.assertRaisesRegex(ValueError, "unrelated native MCP"):
                    audit.check_unrelated_native_mcp(control, root)



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




class PiProjectTrustTests(unittest.TestCase):
    # Frozen .116 hook fixture, not a generated-marker approximation.
    HOOK = b"// Generated by Universal Agent Plugins: Pi project trust hook, schema 1.\n// Answers \"yes\" only for a project recorded by a confirmed UAP installation\n// whose .pi holds nothing but UAP-managed resources with unchanged content,\n// when Pi has no saved decision or non-default policy for it. Everything else,\n// including any error, stays \"undecided\" so Pi's own trust flow decides.\nimport { createHash } from \"node:crypto\";\nimport { closeSync, constants, fstatSync, lstatSync, openSync, readdirSync, readFileSync, realpathSync } from \"node:fs\";\nimport { homedir } from \"node:os\";\nimport { basename, dirname, join, resolve } from \"node:path\";\nimport { fileURLToPath } from \"node:url\";\n\nconst LIMIT = 4194304;\nconst TREE_ENTRIES = 16384;\nconst TREE_BYTES = 67108864;\nconst SURFACES = [\"extensions\", \"skills\", \"prompts\", \"themes\"];\nconst ENTRIES = new Set([...SURFACES, \"mcp.json\", \"mcp.json.agentplugins.lock\"]);\nconst OWNED = [\"extensions\", \"skills\", \"prompts\", \"themes\", \"mcp_servers\"];\n\nfunction comparable(path) {\n  const full = resolve(path);\n  return process.platform === \"win32\" ? full.toLowerCase() : full;\n}\n\nfunction kind(path) {\n  try {\n    const info = lstatSync(path);\n    return info.isDirectory() ? \"directory\" : info.isFile() ? \"file\" : \"other\";\n  } catch (error) {\n    if (error && error.code === \"ENOENT\") return \"absent\";\n    throw error;\n  }\n}\n\nfunction object(value) {\n  return typeof value === \"object\" && value !== null && !Array.isArray(value);\n}\n\nfunction document(path, absent) {\n  const found = kind(path);\n  if (found === \"absent\" && absent !== undefined) return absent;\n  if (found !== \"file\" || lstatSync(path).size > LIMIT) throw new Error(\"unexpected trust input\");\n  const value = JSON.parse(readFileSync(path, \"utf8\").replace(/^\\uFEFF/, \"\"));\n  if (!object(value)) throw new Error(\"unexpected trust input\");\n  return value;\n}\n\nfunction bytewise(left, right) {\n  return Buffer.compare(Buffer.from(left), Buffer.from(right));\n}\n\nfunction fileDigest(path, budget) {\n  const fd = openSync(path, constants.O_RDONLY | (constants.O_NOFOLLOW ?? 0));\n  try {\n    const info = fstatSync(fd);\n    budget.bytes += info.size;\n    if (!info.isFile() || info.nlink !== 1 || budget.bytes > TREE_BYTES) throw new Error(\"owned file is not plain\");\n    const body = readFileSync(fd);\n    if (body.length !== info.size) throw new Error(\"owned file changed\");\n    return createHash(\"sha256\").update(body).digest(\"hex\");\n  } finally {\n    closeSync(fd);\n  }\n}\n\nfunction walk(directory, relative, hash, budget) {\n  for (const name of readdirSync(directory).sort(bytewise)) {\n    if (++budget.entries > TREE_ENTRIES) throw new Error(\"owned resource is too large\");\n    const path = join(directory, name);\n    const child = relative ? relative + \"/\" + name : name;\n    const found = kind(path);\n    if (found === \"directory\") {\n      hash.update(\"d\\0\" + child + \"\\0\");\n      walk(path, child, hash, budget);\n    } else if (found === \"file\") {\n      hash.update(\"f\\0\" + child + \"\\0\" + fileDigest(path, budget) + \"\\0\");\n    } else {\n      throw new Error(\"owned resource holds a link or special file\");\n    }\n  }\n}\n\nfunction treeDigest(root) {\n  if (kind(root) !== \"directory\") throw new Error(\"owned resource is not a directory\");\n  const hash = createHash(\"sha256\");\n  walk(root, \"\", hash, { entries: 0, bytes: 0 });\n  return \"sha256:\" + hash.digest(\"hex\");\n}\n\nfunction frame(value, hash) {\n  if (value === null) hash.update(\"n\");\n  else if (value === true || value === false) hash.update(value ? \"t\" : \"f\");\n  else if (typeof value === \"number\") {\n    const bits = Buffer.alloc(8);\n    bits.writeDoubleBE(value);\n    hash.update(\"d\" + bits.toString(\"hex\"));\n  } else if (typeof value === \"string\") {\n    const bytes = Buffer.from(value, \"utf8\");\n    hash.update(\"s\" + bytes.length + \":\");\n    hash.update(bytes);\n  } else if (Array.isArray(value)) {\n    hash.update(\"a\" + value.length + \":\");\n    for (const item of value) frame(item, hash);\n  } else if (object(value)) {\n    const keys = Object.keys(value).sort(bytewise);\n    hash.update(\"o\" + keys.length + \":\");\n    for (const key of keys) {\n      frame(key, hash);\n      frame(value[key], hash);\n    }\n  } else throw new Error(\"unsupported value\");\n}\n\nfunction valueDigest(value) {\n  const hash = createHash(\"sha256\");\n  frame(value, hash);\n  return \"sha256:\" + hash.digest(\"hex\");\n}\n\nfunction owned(config, owners) {\n  const names = Object.fromEntries(OWNED.map((surface) => [surface, new Map()]));\n  for (const [artifact, entry] of Object.entries(owners)) {\n    if (!object(entry)) throw new Error(\"invalid owner\");\n    // A removed installation leaves no live package directory to vouch for.\n    if (kind(join(config, \"extensions\", artifact)) !== \"directory\") continue;\n    for (const surface of OWNED) {\n      const recorded = entry[surface] ?? {};\n      if (!object(recorded)) throw new Error(\"invalid owner\");\n      for (const [name, digest] of Object.entries(recorded)) {\n        if (typeof digest !== \"string\" || (names[surface].has(name) && names[surface].get(name) !== digest)) throw new Error(\"invalid owner\");\n        names[surface].set(name, digest);\n      }\n    }\n  }\n  return names;\n}\n\nfunction onlyOwned(project, owners) {\n  const config = join(project, \".pi\");\n  if (kind(config) !== \"directory\") return false;\n  const names = owned(config, owners);\n  if (readdirSync(config).some((name) => !ENTRIES.has(name))) return false;\n  for (const surface of SURFACES) {\n    const directory = join(config, surface);\n    const found = kind(directory);\n    if (found === \"absent\") continue;\n    if (found !== \"directory\") return false;\n    for (const name of readdirSync(directory)) {\n      if (!names[surface].has(name) || treeDigest(join(directory, name)) !== names[surface].get(name)) return false;\n    }\n  }\n  const mcp = document(join(config, \"mcp.json\"), {});\n  if (Object.keys(mcp).some((key) => key !== \"mcpServers\")) return false;\n  const servers = mcp.mcpServers ?? {};\n  if (!object(servers)) return false;\n  for (const [name, entry] of Object.entries(servers)) {\n    if (!names.mcp_servers.has(name) || valueDigest(entry) !== names.mcp_servers.get(name)) return false;\n  }\n  let home = resolve(process.env.HOME || homedir());\n  try {\n    home = realpathSync(home);\n  } catch {}\n  const userSkills = comparable(join(home, \".agents\", \"skills\"));\n  for (let directory = project; ; directory = dirname(directory)) {\n    const skills = join(directory, \".agents\", \"skills\");\n    if (comparable(skills) !== userSkills && kind(skills) !== \"absent\") return false;\n    if (dirname(directory) === directory) return true;\n  }\n}\n\nfunction leftToHook(agent, project) {\n  // A saved trust.json decision or a non-default policy belongs to Pi.\n  const settings = document(join(agent, \"settings.json\"), {});\n  if (settings.defaultProjectTrust === \"always\" || settings.defaultProjectTrust === \"never\") return false;\n  const saved = document(join(agent, \"trust.json\"), {});\n  for (let directory = project; ; directory = dirname(directory)) {\n    for (const [path, decision] of Object.entries(saved)) {\n      if ((decision === true || decision === false) && comparable(path) === comparable(directory)) return false;\n    }\n    if (dirname(directory) === directory) return true;\n  }\n}\n\nfunction agentDir() {\n  // Pi's own agent directory: PI_CODING_AGENT_DIR with ~ and file:// support,\n  // otherwise ~/.pi/agent. Other spellings stay unmatched and undecided.\n  const configured = process.env.PI_CODING_AGENT_DIR;\n  if (!configured) return join(homedir(), \".pi\", \"agent\");\n  if (configured === \"~\") return homedir();\n  if (configured.startsWith(\"~/\") || (process.platform === \"win32\" && configured.startsWith(\"~\\\\\"))) return join(homedir(), configured.slice(2));\n  if (configured.startsWith(\"file://\")) return fileURLToPath(configured);\n  return resolve(configured);\n}\n\nfunction decide(cwd) {\n  const hook = dirname(realpathSync(fileURLToPath(import.meta.url)));\n  if (basename(hook) !== \"agentplugins-project-trust\" || basename(dirname(hook)) !== \"extensions\") return false;\n  // Answer only for the profile the hook was installed into. Another profile\n  // reaching it through a symlinked extensions directory keeps its own trust.\n  const agent = agentDir();\n  if (kind(join(agent, \"extensions\")) !== \"directory\" || comparable(realpathSync(agent)) !== comparable(dirname(dirname(hook)))) return false;\n  const recorded = document(join(hook, \"projects.json\"));\n  if (recorded.schema_version !== 1 || !object(recorded.projects)) return false;\n  const project = realpathSync(resolve(cwd));\n  const matches = Object.entries(recorded.projects).filter(([root]) => comparable(root) === comparable(project));\n  if (matches.length !== 1 || !object(matches[0][1])) return false;\n  return leftToHook(agent, project) && onlyOwned(project, matches[0][1]);\n}\n\nexport default function agentPluginsProjectTrust(pi) {\n  pi.on(\"project_trust\", (event) => {\n    try {\n      return { trusted: typeof event?.cwd === \"string\" && decide(event.cwd) ? \"yes\" : \"undecided\" };\n    } catch {\n      return { trusted: \"undecided\" };\n    }\n  });\n}\n"

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.native = self.root / "home/.pi/agent"
        self.native.mkdir(parents=True)
        self.project = self.root / "project"
        self.project.mkdir()
        self.locator = self.project / ".pi/extensions/notion-owner"
        self.locator.mkdir(parents=True)
        (self.locator / "plugin.json").write_text("{}", encoding="utf-8")
        self.oracle = audit.PiProjectTrustOracle(self.root, self.native)
        self.case = {**case(), "name": "notion"}
        self.binding = {"target_locator": str(self.locator), "physical_artifact_id": "notion-owner"}
        self.success = {"exit_code": 0, "timeout": False}
        self.original = audit.protected_scope_snapshot(self.root, "project", self.native)
        self.addCleanup(mock.patch.stopall)
        mock.patch.object(audit, "checked_binding", return_value=({}, self.binding, {})).start()
        mock.patch.object(audit, "checked_native_materialization", return_value={}).start()
        mock.patch.object(audit, "state_file", return_value={}).start()

    def install(self):
        self.oracle.hook.mkdir(parents=True, mode=0o700, exist_ok=True)
        self.oracle.hook.parent.chmod(0o700)
        self.oracle.hook.chmod(0o700)
        self.record = {"schema_version": 1, "projects": {str(self.project): {"notion-owner": {
            "extensions": {"notion-owner": self.oracle.tree_digest(self.locator)},
            "skills": {}, "mcp_servers": {"notion": self.oracle.value_digest(
                {"url": "https://mcp.notion.com/mcp"})}}}}}
        for path, body in ((self.oracle.files[0], self.HOOK),
                           (self.oracle.files[1], json.dumps(self.record).encode()),
                           (self.oracle.files[2], b""), (self.oracle.files[3], b"")):
            path.write_bytes(body)
            path.chmod(0o600)

    def observe(self, operation="add", result=None):
        current = audit.protected_scope_snapshot(self.root, "project", self.native)
        return self.oracle.observe(operation, self.success if result is None else result, current, self.case)

    def rewrite_record(self):
        self.oracle.files[1].write_text(json.dumps(self.record), encoding="utf-8")

    def test_owned_hook_has_exact_phase_lifecycle_and_foreign_snapshot(self):
        (self.native / "foreign.txt").write_text("foreign", encoding="utf-8")
        self.original = audit.protected_scope_snapshot(self.root, "project", self.native)
        self.install()
        self.assertEqual(self.observe(), self.original)
        for operation in ("info", "list", "update", "repair"):
            self.assertEqual(self.observe(operation), self.original)
        for path in self.oracle.files[:2]:
            path.unlink()
        self.assertEqual(self.observe("remove"), self.original)
        self.assertEqual(self.observe("list"), self.original)
        self.assertEqual(self.oracle.phase, "removed")

    def test_forged_hook_marker_is_not_ownership(self):
        self.install()
        self.oracle.files[0].write_bytes(self.HOOK.replace(b"node:crypto", b"evil:crypto"))
        with self.assertRaisesRegex(ValueError, "frozen release"):
            self.observe()

    def test_wrong_project_owner_and_resource_digest_are_refused(self):
        for mutation in ("project", "owner", "digest", "extra", "schema"):
            with self.subTest(mutation=mutation):
                self.install()
                owners = self.record["projects"][str(self.project)]
                if mutation == "project":
                    self.record["projects"][str(self.root / "elsewhere")] = self.record["projects"].pop(str(self.project))
                elif mutation == "owner":
                    owners["foreign"] = owners.pop("notion-owner")
                elif mutation == "digest":
                    owners["notion-owner"]["extensions"]["notion-owner"] = "sha256:" + "0" * 64
                elif mutation == "extra":
                    owners["notion-owner"]["mcp_servers"]["foreign"] = "sha256:" + "0" * 64
                else:
                    self.record["schema_version"] = True
                self.rewrite_record()
                with self.assertRaises(ValueError):
                    self.observe()

    def test_foreign_hook_entry_and_mutated_inert_lock_are_refused(self):
        self.install()
        foreign = self.oracle.hook / "index.js"
        foreign.write_text("foreign", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "foreign"):
            self.observe()
        foreign.unlink()
        self.oracle.files[2].write_bytes(b"foreign lock")
        with self.assertRaisesRegex(ValueError, "inert"):
            self.observe()

    def test_foreign_user_mutation_is_not_projected_out(self):
        self.install()
        self.assertEqual(self.observe(), self.original)
        (self.native / "trust.json").write_text("{}", encoding="utf-8")
        self.assertNotEqual(self.observe("info"), self.original)

    def test_wrong_phase_failed_add_and_missing_cleanup_are_refused(self):
        self.install()
        for operation, output in (("repair", self.success), ("add", {"exit_code": 1, "timeout": False}),
                                  ("add", {"exit_code": 0, "timeout": True})):
            with self.assertRaises(ValueError):
                self.observe(operation, output)
        self.observe()
        with self.assertRaises(ValueError):
            self.observe("remove")
        with self.assertRaises(ValueError):
            self.observe("add")

    @unittest.skipIf(os.name == "nt", "POSIX mode and symlink fixture")
    def test_links_modes_and_replaced_directory_are_refused(self):
        self.install()
        path = self.oracle.files[0]
        body = path.read_bytes()
        path.unlink()
        outside = self.root / "outside"
        outside.write_bytes(body)
        path.symlink_to(outside)
        with self.assertRaises(ValueError):
            self.observe()
        path.unlink()
        path.write_bytes(body)
        path.chmod(0o644)
        with self.assertRaisesRegex(ValueError, "mode"):
            self.observe()
        path.chmod(0o600)
        self.observe()
        old = self.oracle.hook.with_name("old-hook")
        self.oracle.hook.rename(old)
        self.oracle.hook.mkdir(mode=0o700)
        with self.assertRaisesRegex(ValueError, "replaced"):
            self.observe("info")

    def test_equal_bytes_replacement_after_add_is_refused(self):
        self.install()
        self.observe()
        path = self.oracle.files[0]
        replacement = self.root / "replacement"
        replacement.write_bytes(path.read_bytes())
        replacement.chmod(0o600)
        replacement.replace(path)
        with self.assertRaisesRegex(ValueError, "changed after"):
            self.observe("repair")

    def test_replacement_during_held_read_is_refused(self):
        self.install()
        path = self.oracle.files[0]
        original = audit.live.copilot_settings_read
        def replace_after_read(selected, root):
            value = original(selected, root)
            if selected == path:
                replacement = self.root / "racing-replacement"
                replacement.write_bytes(path.read_bytes())
                replacement.chmod(0o600)
                replacement.replace(path)
            return value
        with mock.patch.object(audit.live, "copilot_settings_read", side_effect=replace_after_read):
            with self.assertRaisesRegex(ValueError, "replaced"):
                self.observe()

    def test_duplicate_trust_key_and_extra_foreign_owner_are_refused(self):
        self.install()
        self.oracle.files[1].write_text('{"schema_version":1,"schema_version":1,"projects":{}}', encoding="utf-8")
        with self.assertRaises((ValueError, AssertionError)):
            self.observe()

    @unittest.skipIf(os.name == "nt", "POSIX hardlink fixture")
    def test_hardlinked_hook_is_refused(self):
        self.install()
        os.link(self.oracle.files[0], self.root / "hook-hardlink")
        with self.assertRaisesRegex(ValueError, "single-link"):
            self.observe()

    def test_tree_and_value_framing_match_frozen_contract(self):
        import hashlib
        expected = hashlib.sha256(b"f\0plugin.json\0" +
                                  hashlib.sha256(b"{}").hexdigest().encode() + b"\0").hexdigest()
        self.assertEqual(self.oracle.tree_digest(self.locator), "sha256:" + expected)
        self.assertEqual(self.oracle.value_digest({"url": "https://mcp.notion.com/mcp"}),
                         audit.digest(b"o1:s3:urls26:https://mcp.notion.com/mcp"))




class PiClosureNativeTests(unittest.TestCase):
    CAMPAIGN = "---\nname: campaign-planning\ndescription: Plan a marketing campaign, content calendar or landing-page launch using an audience brief, existing Notion content, HubSpot CRM insights and Playwright checks.\n---\n\n# Campaign planning\n\nUse this workflow when the user requests a campaign plan, content calendar,\nlaunch brief or landing-page marketing review.\n\n## Establish the brief\n\nStart from the information the user supplied. Identify the product or offer,\naudience, objective, channels, dates, budget and brand constraints. Ask only for\nmissing details that would materially change the plan. Mark estimates and\nassumptions explicitly; never invent customer research or campaign results.\n\n## Gather relevant evidence\n\n- **Notion:** find the user's relevant brand guidelines, product notes and\n  existing campaign material. Cite the pages used. Draft a content calendar in\n  the answer; create or edit Notion pages when the user requests saving it.\n- **HubSpot CRM:** read authorized aggregate pipeline and audience context to\n  inform personas and channel priorities. This planning workflow uses read\n  tools; it does not create contacts, send campaigns or update CRM records.\n  Do not copy a contact database into the campaign plan.\n- **Playwright:** inspect the requested landing page for message consistency,\n  primary calls to action, mobile layout and obvious broken navigation. Report\n  observed issues with the page URL and evidence. Do not submit live forms or\n  create purchases as part of a review.\n\nUse only the integrations relevant to the request. If a workspace connection\nis unavailable, continue from the brief and identify the missing evidence.\nInstalling a plugin does not supply Notion or HubSpot account authorization.\n\n## Produce a usable plan\n\nKeep the result proportional to the campaign. Include:\n\n1. Objective, audience, offer and evidence-backed assumptions.\n2. The main message and two or three supporting content themes.\n3. A channel calendar with date, asset, owner and call to action.\n4. Draft copy for the highest-priority asset.\n5. Landing-page issues and the smallest useful changes.\n6. Success measures, their data source and the next review date.\n\nDistinguish proposed work from completed actions. Link supporting Notion pages\nand reviewed URLs. Planning does not itself publish content or contact leads;\nperform those actions only when they are part of the user's explicit request.\n"

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        native = self.root / "home/.pi/agent"
        native.mkdir(parents=True)
        (self.root / "project").mkdir()
        self.selected = self.root / "project/.pi"
        self.selected.mkdir()
        self.oracle = audit.PiProjectTrustOracle(self.root, native)
        self.case = {"client": "pi", "scope": "project"}
        self.closure = SimpleNamespace(checked_playwright_runtime=mock.Mock())
        self.owners = {}
        self.entries = {
            audit.FOREIGN_NATIVE_MCP: {"url": "https://foreign.invalid/mcp", "enabled": False},
            "notion": {"url": "https://mcp.notion.com/mcp"},
            "hubspot-crm": {"url": "https://mcp.hubspot.com"}}
        for alias in audit.live.MARKETING_KIT_DISTRIBUTIONS:
            physical = alias + "-owner"
            locator = self.selected / "extensions" / physical
            locator.mkdir(parents=True)
            (locator / "plugin.json").write_text("{}", encoding="utf-8")
            package = {"object_id": "package:pi:" + physical, "kind": "managed_package_directory",
                       "logical_name": alias, "path": str(locator), "protection_class": "managed",
                       "managed_digest": "sha256:" + "a" * 64}
            if alias == "marketing-kit":
                path = self.selected / "skills/campaign-planning"
                path.mkdir(parents=True)
                source = locator / "skills/campaign-planning"
                source.mkdir(parents=True)
                for directory in (path, source):
                    (directory / "SKILL.md").write_bytes(self.CAMPAIGN.encode())
                body = self.CAMPAIGN.encode()
                image = {"SKILL.md": ("file", (path / "SKILL.md").lstat().st_mode,
                                      None, None, None, len(body), audit.hashlib.sha256(body).hexdigest())}
                adapter = type("SkillReader", (), {"bytes": lambda _, p: p.read_bytes()})()
                md = self.oracle.resource_audit.PiProjectSkillAudit.skill_digest(adapter, path, image)
                kind = "pi_project_skill_directory"
                member = {"kind": kind, "object_id": kind + ":campaign-planning", "logical_name": "campaign-planning",
                          "source_relative": "skills/campaign-planning", "path": str(path),
                          "managed_digest": md, "protection_class": "managed"}
            else:
                kind = "pi_project_mcp_server"
                member = {"kind": kind, "object_id": kind + ":" + alias, "logical_name": alias,
                          "path": str(self.selected / "mcp.json"), "managed_digest": "sha256:" + "b" * 64,
                          "protection_class": "managed"}
            binding = {"client_binding_id": alias, "physical_artifact_id": physical,
                       "native_objects": [package, member], "receipts": [{"phase": "committed",
                       "client_binding_id": alias, "active_path": str(locator), "after_digest": package["managed_digest"]}]}
            self.owners[alias] = {"binding": binding, "locator": locator}
        locator = self.owners["playwright"]["locator"]
        self.entries["playwright"] = {"command": "node", "args": [
            str(locator / "io.github.777genius.agentplugins/runtime/launcher.mjs")]}
        self.config()

    def config(self):
        (self.selected / "mcp.json").write_text(json.dumps({"mcpServers": self.entries}), encoding="utf-8")

    def check(self, alias):
        return audit.checked_pi_closure_native(self.oracle, self.closure, self.case, self.selected,
                                               alias, self.owners[alias])

    def test_all_four_exact_native_owners_produce_trust_map(self):
        proofs = {alias: self.check(alias) for alias in self.owners}
        self.assertTrue(all(p["content_verified"] is True for p in proofs.values()))
        self.assertEqual(set(proofs["marketing-kit"]["trust_owner"]["skills"]), {"campaign-planning"})
        for alias in ("notion", "hubspot-crm", "playwright"):
            self.assertEqual(set(proofs[alias]["trust_owner"]["mcp_servers"]), {alias})
        self.closure.checked_playwright_runtime.assert_called_once()

    def test_wrong_transport_and_extra_native_server_are_refused(self):
        self.entries["playwright"]["args"].append("--evil")
        self.config()
        with self.assertRaisesRegex(ValueError, "transport"):
            self.check("playwright")
        self.entries["foreign-extra"] = {"command": "evil"}
        self.config()
        with self.assertRaisesRegex(ValueError, "transport"):
            self.check("notion")

    def test_extra_native_object_wrong_scope_or_missing_receipt_are_refused(self):
        binding = self.owners["notion"]["binding"]
        original = copy.deepcopy(binding)
        for mutation in ("object", "scope", "receipt"):
            binding.clear()
            binding.update(copy.deepcopy(original))
            if mutation == "object":
                binding["native_objects"].append(copy.deepcopy(binding["native_objects"][0]))
            elif mutation == "scope":
                binding["native_objects"][1]["kind"] = "pi_user_mcp_server"
            else:
                binding["receipts"] = []
            with self.assertRaises(ValueError):
                self.check("notion")

    def test_forged_skill_digest_extra_skill_and_entrypoint_are_refused(self):
        member = self.owners["marketing-kit"]["binding"]["native_objects"][1]
        before = member["managed_digest"]
        member["managed_digest"] = "sha256:" + "f" * 64
        with self.assertRaisesRegex(ValueError, "digest"):
            self.check("marketing-kit")
        member["managed_digest"] = before
        path = self.selected / "skills/foreign"
        path.mkdir()
        with self.assertRaisesRegex(ValueError, "unreviewed Skill"):
            self.check("marketing-kit")
        foreign = self.owners["notion"]["locator"] / "index.ts"
        foreign.write_text("evil", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "entrypoint"):
            self.check("notion")

    def test_playwright_source_oracle_failure_prevents_native_proof(self):
        self.closure.checked_playwright_runtime.side_effect = ValueError("forged source")
        with self.assertRaisesRegex(ValueError, "forged source"):
            self.check("playwright")


class ClosureLifecycleIntegrationTests(unittest.TestCase):
    def selected(self):
        pin = audit.live.MARKETING_KIT_SIGNED_CLOSURE["777genius/marketing-kit"]
        return {"name": "marketing-kit", "distribution": "777genius/marketing-kit", "client": "hermes",
                "scope": "user", "expected": "dependency-closure", "components": ["skills"],
                "selector": "github:fixture", "source": {"repository": pin["repository"],
                "revision": pin["resolved_revision"], "path": pin["package_path"]},
                "tree_digest": pin["tree_digest"], "manifest_digest": pin["manifest_digest"]}

    def exercise(self, *, policy="eligible", error=None, retain_child=False, missing_native=False):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        root = Path(temporary.name).resolve() / "case"
        calls, stages, owners = [], [], {}
        selected = self.selected()
        helper = SimpleNamespace()
        real_closure = audit.module("real_cleanup_contract", "released_source_closure.py")
        def authorize(api, context, case_root, case, stage):
            stages.append(stage)
            if error is not None:
                raise ValueError(error)
            return {"policy_status": policy, "ineligible_distributions": ["777genius/notion"]}
        def join(*args):
            return owners
        def native(api, joined, verifier):
            if missing_native:
                raise ValueError("native content incomplete")
            return {alias: {"content_verified": True} for alias in joined}
        def cli(argv, env, project, *args, **kwargs):
            operation = argv[1]
            calls.append((operation, argv[2] if operation in ("info", "update", "repair", "remove") else None))
            data = root / "uap-data"
            if operation == "add" and policy == "eligible":
                native = root / "home/.hermes"
                skill = native / "skills/campaign-planning"
                skill.mkdir(parents=True)
                (skill / "SKILL.md").write_text("fixture", encoding="utf-8")
                (native / "config.yaml").write_text(audit.yaml.safe_dump({"mcp_servers": {
                    audit.FOREIGN_NATIVE_MCP: {"command": "echo", "args": ["unrelated-native-audit"]},
                    "notion": {}, "hubspot-crm": {}, "playwright": {}}}), encoding="utf-8")
                for alias in audit.live.MARKETING_KIT_DISTRIBUTIONS:
                    locator = root / "home/.hermes/plugins" / alias
                    locator.mkdir(parents=True)
                    (locator / "plugin.json").write_text("{}", encoding="utf-8")
                    owners[alias] = {"installation": {"installation_id": alias},
                        "binding": {"client_binding_id": alias, "physical_artifact_id": alias},
                        "locator": locator}
                (data / "uap.toml").write_text('schema_version=1\n[dependencies]\n[targets]\n', encoding="utf-8")
                (data / "uap.lock").write_text('{"lock_version":1,"packages":[]}', encoding="utf-8")
            if operation == "remove":
                native = root / "home/.hermes"
                (native / "skills/campaign-planning/SKILL.md").unlink()
                (native / "skills/campaign-planning").rmdir()
                (native / "config.yaml").write_text(audit.yaml.safe_dump({"mcp_servers": {
                    audit.FOREIGN_NATIVE_MCP: {"command": "echo", "args": ["unrelated-native-audit"]}}}), encoding="utf-8")
                for owner in owners.values():
                    (owner["locator"] / "plugin.json").unlink()
                    owner["locator"].rmdir()
                if retain_child:
                    (data / "uap.lock").write_text('{"lock_version":1,"packages":[{"alias":"notion","source_key":"directory:777genius/notion"}]}', encoding="utf-8")
                (data / "state-v2.json").write_text('{"schema_version":5,"installations":[],"environment_receipts":[]}', encoding="utf-8")
            refused = operation == "add" and policy == "ineligible"
            return {"exit_code": 1 if refused else 0, "timeout": False,
                    "_json": {"data": {"installations": []}},
                    "stdout_full": "", "stderr_full": ("agentplugins: dependency conflict for directory:777genius/notion: marketing-kit version 1.0.0" if refused else ""),
                    "stdout_tail": "", "stderr_tail": ""}
        helper.authorize = authorize
        helper.checked_closure = join
        helper.checked_native = native
        helper.checked_cleanup = real_closure.checked_cleanup
        helper.checked_member_native = lambda *args: {"content_verified": True}
        helper.checked_namespace = real_closure.checked_namespace
        helper.held_read = lambda api, path, case_root: path.read_bytes()
        helper.checked_refusal_authority = lambda *args: None
        original_module = audit.module
        with mock.patch.object(audit, "module", side_effect=lambda name, filename: helper if filename == "released_source_closure.py" else original_module(name, filename)), \
             mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
             mock.patch.object(audit.live, "call", side_effect=cli), \
             mock.patch.object(audit, "checked_dependency_refusal", return_value={"distribution": "777genius/notion"}):
            outcome = audit.run_case(Path("/fixture-cli"), selected, root, None, policy_context={"verified": True})
        return outcome, calls, stages

    def test_complete_closure_exercises_all_ids_and_reauthenticates_mutations(self):
        def windows_write_text(path, data, encoding=None, errors=None, newline=None):
            with path.open("w", encoding=encoding, errors=errors, newline="\r\n" if newline is None else newline) as stream:
                return stream.write(data)
        # Model Windows text-mode newline translation even on POSIX hosts.
        with mock.patch.object(Path, "write_text", autospec=True, side_effect=windows_write_text):
            outcome, calls, stages = self.exercise()
        self.assertEqual(outcome["status"], "materialized-closure-lifecycle-passed", outcome.get("reason"))
        self.assertEqual(stages, ["add", "update", "repair"])
        self.assertEqual([target for op, target in calls if op == "info"], list(audit.live.MARKETING_KIT_DISTRIBUTIONS))
        self.assertIn(("update", "marketing-kit"), calls)
        self.assertIn(("repair", "marketing-kit"), calls)
        self.assertIn(("remove", "marketing-kit"), calls)
        self.assertFalse(outcome["runtime_qualified"])

    def test_authentication_missing_native_and_retained_child_lock_fail_closed(self):
        for kwargs, reason in (({"error": "bad signature"}, "bad signature"),
                               ({"missing_native": True}, "native content incomplete"),
                               ({"retain_child": True}, "retained locked child")):
            with self.subTest(kwargs=kwargs):
                outcome, _, _ = self.exercise(**kwargs)
                self.assertEqual(outcome["status"], "failed")
                self.assertIn(reason, outcome["reason"])

    def test_authenticated_ineligible_refusal_has_no_effects(self):
        outcome, calls, stages = self.exercise(policy="ineligible")
        self.assertEqual(outcome["status"], "expected-dependency-refusal", outcome.get("reason"))
        self.assertEqual(len(calls), 1)
        self.assertEqual(stages, ["add"])

    def test_new_closure_without_runtime_context_never_runs_cli(self):
        with tempfile.TemporaryDirectory() as temporary, \
             mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
             mock.patch.object(audit.live, "call") as cli:
            outcome = audit.run_case(Path("/fixture-cli"), self.selected(), Path(temporary) / "case", None)
        self.assertEqual(outcome["status"], "failed")
        self.assertIn("prerequisites", outcome["reason"])
        cli.assert_not_called()




class PairedChannelTests(unittest.TestCase):
    def test_public_npm_prefix_executes_wrapper_for_actual_operation(self):
        selected = ClosureLifecycleIntegrationTests().selected()
        selected["expected"] = "dependency-refusal"
        output = {"exit_code": 1, "timeout": False, "stdout_full": "", "stderr_full": "",
                  "_json": None, "stdout_tail": "", "stderr_tail": ""}
        prefix = [str(Path("/absolute-node").absolute()), str(Path("/public-package/bin/agentplugins.js").absolute())]
        with tempfile.TemporaryDirectory() as temporary, \
             mock.patch.object(audit.live, "initialize_project_skill_git_root"), \
             mock.patch.object(audit.live, "call", return_value=output) as call, \
             mock.patch.object(audit, "checked_dependency_refusal", return_value={"distribution": "777genius/notion"}):
            result = audit.run_case(prefix, selected, Path(temporary) / "case", None)
        self.assertEqual(result["status"], "expected-dependency-refusal")
        self.assertEqual(call.call_args.args[0][:3], [*prefix, "add"])
        self.assertEqual(result["trace"][0]["argv"][:3], [*prefix, "add"])

    def test_main_runs_both_channels_with_fresh_roots_and_one_policy_build(self):
        with tempfile.TemporaryDirectory() as temporary:
            base = Path(temporary)
            native = base / "hermes"
            native.write_text("fixture", encoding="utf-8")
            args = SimpleNamespace(client="hermes", target="linux-amd64", native_bin=native, report_dir=base / "receipt")
            prefix = [str(base / "node"), str(base / "public/bin/agentplugins.js")]
            context = {"frozen": "verified"}
            npm = SimpleNamespace(provision=mock.Mock(return_value={"command": prefix, "proof": {"verified": True},
                                                                   "env": {"PATH": str(base)}}))
            calls = []
            def selected(snapshot, distribution, sequence, client, scope, refusal):
                return {"name": "notion" if distribution.endswith("/notion") else "marketing-kit",
                        "client": client, "scope": scope, "expected": "dependency-refusal" if refusal else "materialized-lifecycle"}
            def run(command, case, root, executable, policy_context=None, command_env=None):
                calls.append((command, case.copy(), root, policy_context, command_env))
                return {**case, "status": "materialized-lifecycle-passed"}
            with mock.patch.object(audit.argparse.ArgumentParser, "parse_args", return_value=args), \
                 mock.patch.object(audit.native, "require_hosted"), \
                 mock.patch.object(audit.native, "provision_release", return_value=(base / "binary", {"verified": True})), \
                 mock.patch.object(audit, "authenticated_snapshot", return_value={}), \
                 mock.patch.object(audit, "provision_native", return_value=(native, {"verified": True})), \
                 mock.patch.object(audit, "prepare_policy_context", return_value=context) as prepare, \
                 mock.patch.object(audit, "module", return_value=npm), \
                 mock.patch.object(audit, "source_case", side_effect=selected), \
                 mock.patch.object(audit, "run_case", side_effect=run), \
                 mock.patch("sys.stdout", new_callable=io.StringIO), \
                 mock.patch.dict(os.environ, {"RUNNER_TEMP": str(base)}):
                self.assertEqual(audit.main(), 0)
            prepare.assert_called_once()
            npm.provision.assert_called_once()
            self.assertEqual(len(calls), 4)
            self.assertEqual({c[2].name for c in calls}, {"binary-notion-user", "binary-marketing-kit-user",
                                                        "npm-notion-user", "npm-marketing-kit-user"})
            self.assertTrue(all(c[3] is context for c in calls))
            self.assertTrue(all(c[0] == prefix for c in calls if c[1]["channel"] == "npm"))
            self.assertTrue(all(c[1]["expected"] == "dependency-closure" for c in calls if c[1]["name"] == "marketing-kit"))
            receipt = json.loads((args.report_dir / "receipt.json").read_text())
            self.assertEqual({c["channel"] for c in receipt["cases"]}, {"binary", "npm"})

    def test_npm_env_cannot_replace_case_home(self):
        with tempfile.TemporaryDirectory() as temporary:
            with self.assertRaisesRegex(ValueError, "isolation"):
                audit.run_case([str(Path("/node").absolute()), str(Path("/wrapper.js").absolute())],
                               ClosureLifecycleIntegrationTests().selected(), Path(temporary) / "case", None,
                               command_env={"HOME": "/foreign"})

if __name__ == "__main__":
    unittest.main()
