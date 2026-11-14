#!/usr/bin/env python3
"""Offline contract regressions; these tests are never a lifecycle receipt."""
import copy
import base64
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import tarfile
import tempfile
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


def ui_value(selected):
    commands = {"saveMode": "saved"}
    for operation in ("add", "update", "repair", "remove"):
        operand = selected["selector"] if operation == "add" else selected["name"]
        words = [*audit.NPM_PREFIX, operation, operand, "--target", selected["client"]]
        if selected["scope"] == "project":
            words += ["--scope", "project"]
        commands[operation] = " ".join(words)
    return {"selection": {"selector": selected["selector"], "source": {**selected["source"],
                         "tree_sha256": selected["tree_digest"], "manifest_sha256": selected["manifest_digest"]},
                         "components": selected["components"], "client": selected["client"],
                         "scope": selected["scope"], "manifestName": selected["name"]}, "commands": commands}


class CopiedUiCommandTests(unittest.TestCase):
    def test_exact_ui_command_kept_and_only_ci_flags_added(self):
        selected = case()
        value = audit.checked_ui_commands(ui_value(selected), selected)
        raw = value["commands"]["update"]
        argv, process = audit.npm_command(raw, "update", {})
        self.assertEqual(argv, raw.split(" ") + ["--format", "json", "--yes"])
        self.assertNotIn("--no-save", argv)
        self.assertNotIn("--project-root", argv)
        if audit.os.name != "nt":
            self.assertEqual(process, argv)

    def test_wrong_source_digest_scope_or_manifest_name_is_not_ui_evidence(self):
        for key in ("selector", "client", "scope", "manifestName", "components", "source"):
            value = ui_value(case())
            value["selection"][key] = "different"
            with self.subTest(key=key), self.assertRaises(ValueError):
                audit.checked_ui_commands(value, case())

    def test_alias_target_scope_invocation_and_fake_update_are_refused(self):
        for change in (lambda s: s.replace("update portable", "add portable"),
                       lambda s: s.replace("portable", "display-name"),
                       lambda s: s.replace("--target pi", "--target codex"),
                       lambda s: s.replace(" --scope project", ""),
                       lambda s: s.replace("@0.1.108", "@latest"),
                       lambda s: s + " --no-save"):
            value = ui_value(case())
            value["commands"]["update"] = change(value["commands"]["update"])
            with self.subTest(command=value["commands"]["update"]), self.assertRaises(ValueError):
                audit.checked_ui_commands(value, case())

    def test_shell_operators_never_reach_windows_command_shell(self):
        raw = ui_value(case())["commands"]["add"]
        for suffix in (";echo-bad", " & echo-bad", " %TOKEN%", " $(command)", "\necho-bad"):
            with self.subTest(suffix=suffix), self.assertRaises(ValueError):
                audit.npm_command(raw + suffix, "add", {})
        with mock.patch.object(audit.os, "name", "nt"):
            argv, process = audit.npm_command(raw, "add", {"COMSPEC": "C:\\Windows\\System32\\cmd.exe"})
        self.assertEqual(process, ["C:\\Windows\\System32\\cmd.exe", "/d", "/s", "/c", " ".join(argv)])

    def test_real_typescript_exports_are_callable_without_a_second_command_generator(self):
        # This committed snapshot is unit input only; hosted main() separately
        # authenticates exact snapshot47 before using the same Node bridge.
        path = audit.ROOT / "cli/cmd/agentplugins/directory_bootstrap_inputs/snapshot.json"
        document = json.loads(path.read_text(encoding="utf-8"))
        for name, client, scope in (("notion", "pi", "project"), ("marketing-kit", "hermes", "user")):
            selected = audit.source_case(document, "777genius/" + name, 1, client, scope, name == "marketing-kit")
            value = audit.frontend_commands(path, selected)
            self.assertEqual(value["selection"]["manifestName"], selected["name"])
            self.assertIn(" update " + name + " --target " + client, value["commands"]["update"])


class PublicNpmEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.payload = b"\x7fELFfixture-native-bytes"
        self.installer = {"commit": audit.RELEASE_COMMIT, "manifest_sha256": "a" * 64,
                          "file": "agentplugins_0.1.108_linux_amd64", "size": len(self.payload),
                          "binary_sha256": hashlib.sha256(self.payload).hexdigest()}
        self.manifest = {"version": audit.NPM_VERSION, "tag": audit.RELEASE_TAG,
                         "producer": {"repository": "777genius/plugin-kit-ai", "commit": audit.RELEASE_COMMIT,
                                      "release_manifest": {"sha256": "a" * 64}},
                         "assets": {"linux-amd64": {key: self.installer[field] for key, field in
                                    (("file", "file"), ("sha256", "binary_sha256"), ("size", "size"))}}}
        self.contents = {"package.json": json.dumps({"name": audit.NPM_PACKAGE, "version": audit.NPM_VERSION,
                         "bin": {"agentplugins": "bin/agentplugins.js"}}).encode(),
                         "assets.json": json.dumps(self.manifest).encode(),
                         "bin/agentplugins.js": b"// exact fixture launcher\n", "lib/bootstrap.js": b"// exact fixture bootstrap\n"}
        self.body = self.archive(self.contents)
        pin = "sha512-" + base64.b64encode(hashlib.sha512(self.body).digest()).decode()
        patched = mock.patch.object(audit, "NPM_INTEGRITY", pin)
        patched.start()
        self.addCleanup(patched.stop)
        self.reference = audit.npm_reference(self.body)
        self.env = {"NPM_CONFIG_CACHE": str(self.root / "npm-cache"),
                    "XDG_CACHE_HOME": str(self.root / "home/.cache"), "LOCALAPPDATA": str(self.root / "home/AppData/Local")}
        self.package = self.root / "npm-cache/_npx/fixture/node_modules/universal-agent-plugins"
        for name, content in self.contents.items():
            path = self.package / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(content)
        self.lock = self.package.parents[1] / "package-lock.json"
        self.lock.write_text(json.dumps({"packages": {"node_modules/" + audit.NPM_PACKAGE:
                             {"version": audit.NPM_VERSION, "integrity": pin, "resolved": audit.NPM_TARBALL}}}), encoding="utf-8")
        self.binary = self.root / "home/.cache/agentplugins/0.1.108/linux-amd64/agentplugins"
        self.binary.parent.mkdir(parents=True)
        self.binary.write_bytes(self.payload)

    @staticmethod
    def archive(contents, symlink=False):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode="w:gz") as package:
            for name, content in contents.items():
                item = tarfile.TarInfo("package/" + name)
                item.size = len(content)
                if symlink:
                    item.type, item.linkname, item.size = tarfile.SYMTYPE, "outside", 0
                package.addfile(item, io.BytesIO(content))
        return output.getvalue()

    def observe(self):
        return audit.checked_npm_installation(self.root, self.env, "linux-amd64", self.reference, self.installer)

    def test_installed_package_and_actual_cached_binary_both_match(self):
        observed = self.observe()
        self.assertEqual(observed["binary_sha256"], self.installer["binary_sha256"])
        self.assertEqual(observed["verified_package_files"], len(self.contents))
        self.assertEqual(observed["producer_repository"], "777genius/plugin-kit-ai")

    def test_integrity_and_archive_bound_checked_before_members(self):
        for body in (self.body + b"different", b"x" * (audit.MAX_NPM_BYTES + 1)):
            with self.subTest(size=len(body)), self.assertRaises(ValueError):
                audit.npm_reference(body)

    def test_archive_traversal_alias_or_symlink_is_not_an_installed_member(self):
        for names, symlink in (({"../outside": b"x"}, False), ({"./alias": b"x"}, False), ({"member": b"x"}, True)):
            body = self.archive(names, symlink)
            pin = "sha512-" + base64.b64encode(hashlib.sha512(body).digest()).decode()
            with self.subTest(names=names), mock.patch.object(audit, "NPM_INTEGRITY", pin), self.assertRaises(ValueError):
                audit.npm_reference(body)

    def test_tampered_or_extra_launcher_files_refused(self):
        path = self.package / "bin/agentplugins.js"
        path.write_bytes(b"different launcher")
        with self.assertRaisesRegex(ValueError, "contents differ"):
            self.observe()
        path.write_bytes(self.contents["bin/agentplugins.js"])
        (self.package / "extra.js").write_text("unreviewed", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "contents differ"):
            self.observe()

    def test_lock_or_attested_asset_mismatch_refused(self):
        original = self.lock.read_bytes()
        value = json.loads(original)
        value["packages"]["node_modules/" + audit.NPM_PACKAGE]["integrity"] = "sha512:wrong"
        self.lock.write_text(json.dumps(value), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "different public package"):
            self.observe()
        self.lock.write_bytes(original)
        self.installer["manifest_sha256"] = "b" * 64
        with self.assertRaisesRegex(ValueError, "differs from attested"):
            self.observe()

    def test_manifest_cannot_substitute_for_real_cached_bytes(self):
        self.binary.write_bytes(b"x" * len(self.payload))
        with self.assertRaisesRegex(ValueError, "differs from attested"):
            self.observe()
        self.binary.unlink()
        with self.assertRaisesRegex(ValueError, "missing or wrong size"):
            self.observe()

    def test_cache_outside_case_and_duplicate_package_are_refused(self):
        original = self.env["NPM_CONFIG_CACHE"]
        self.env["NPM_CONFIG_CACHE"] = str(self.root.parent)
        with self.assertRaises(ValueError):
            self.observe()
        self.env["NPM_CONFIG_CACHE"] = original
        second = self.root / "npm-cache/_npx/other/node_modules/universal-agent-plugins/package.json"
        second.parent.mkdir(parents=True)
        second.write_bytes(self.contents["package.json"])
        with self.assertRaisesRegex(ValueError, "exactly one public npm"):
            self.observe()


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
