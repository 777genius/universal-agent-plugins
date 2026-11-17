"""Synthetic controls for retention custody; native 30-source evidence is separate."""
import copy
import hashlib
import json
import os
from pathlib import Path
from runpy import run_path
import sys
import tempfile
import unittest
from unittest.mock import patch

API = run_path(str(Path(__file__).with_name("codex_plugin_retention_audit.py")))


@unittest.skipUnless(sys.platform == "linux", "Linux fd custody only")
class RetentionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.profile, self.data = self.root / ".codex", self.root / "uap-data"
        self.profile.mkdir()
        self.data.mkdir()
        (self.profile / "catalog").mkdir()
        (self.profile / "catalog/server.js").write_bytes(b"selected identity\n" * 80000)
        self.audit = API["CodexPluginRetention"](self.profile, "0.162.0")
        self.addCleanup(self.audit.close)
        revision = {"version": "1.0.0", "resolved_revision": "a" * 40,
                    "tree_digest": "sha256:" + "b" * 64,
                    "manifest_digest": "sha256:" + "c" * 64}
        self.target = self.data / "artifacts/exact"
        self.target.mkdir(parents=True)
        self.public = {"installation_id": "installation-1", "name": "selected",
                       "source": "owner/repo/plugin", "clients": [{
                           "client_id": "codex", "scope": "user",
                           "materialization": "materialized", "activation": "active",
                           "verification": "installation_verified", "policy": "allowed",
                           "package_revision": revision}]}
        binding = dict(self.public["clients"][0], client_binding_id="binding-1",
                       physical_artifact_id="physical-1", target_locator=str(self.target),
                       native_profile_root=str(self.profile),
                       receipts=[{"client_binding_id": "binding-1",
                                  "active_path": str(self.target), "phase": "committed"}])
        self.state = {"schema_version": 5, "installations": [{
            "installation_id": "installation-1", "declared_name": "selected",
            "source": {"repository": "owner/repo", "package_subpath": "plugin",
                       "resolved_revision": "a" * 40, "tree_digest": revision["tree_digest"]},
            "package": {"declared_name": "selected", "version": "1.0.0",
                        "manifest_digest": revision["manifest_digest"]},
            "clients": {"binding-1": binding}}]}
        self.write_state()
        market = "agentplugins-" + hashlib.sha256(b"physical-1").hexdigest()[:12]
        (self.profile / "config.toml").write_text(
            '[plugins."selected@' + market + '"]\nenabled = true\n'
            + '[marketplaces.' + market + ']\nsource_type = "local"\nsource = '
            + json.dumps(str(self.target)) + "\n")
        namespace = hashlib.sha256((market + "\0selected").encode()).hexdigest()
        self.namespace = self.profile / "plugins/data/agent-plugins" / namespace
        (self.namespace / "npm-runtime/node_modules/.bin").mkdir(parents=True)
        self.leaf = self.namespace / "npm-runtime/node_modules/tool.js"
        self.leaf.write_bytes(b"selected native runtime\n")
        self.link = self.namespace / "npm-runtime/node_modules/.bin/tool"
        self.link.symlink_to("../tool.js")

    def write_state(self):
        (self.data / "state-v2.json").write_text(json.dumps(self.state))

    def seal(self):
        self.audit.before_remove(self.data, self.public, "selected",
                                 selector="github:owner/repo@" + "a" * 40)

    def raw(self, path, suffix=""):
        return ".codex/" + str(path.relative_to(self.profile)) + suffix

    def test_exact_baseline_and_preserved_data_keep_raw_findings(self):
        self.seal()
        raw = [".codex/catalog/server.js", self.raw(self.leaf),
               self.raw(self.link, ": symlink")]
        # Ordinary removal may change config. Baseline is per exact object;
        # preserved data remains one completely unchanged namespace.
        (self.profile / "config.toml").write_text("")
        result = self.audit.finish(raw)
        self.assertEqual(result["status"], "verified-unchanged-or-preserved-at-checkpoint")
        self.assertEqual([row["raw"] for row in result["residuals"]], raw)
        self.assertEqual([row["authority"] for row in result["residuals"]],
                         ["unchanged-preinstall-object",
                          "ordinary-remove-preserved-data", "ordinary-remove-preserved-data"])

    def test_unscanned_large_reason_requires_full_bytes(self):
        self.seal()
        self.assertEqual(self.audit.finish([
            ".codex/catalog/server.js: unscanned-too-large"])["status"],
            "verified-unchanged-or-preserved-at-checkpoint")

    def test_changed_baseline_bytes_refused(self):
        self.seal()
        (self.profile / "catalog/server.js").write_bytes(b"changed")
        self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                         "unresolved-plugin-retention")

    def test_replaced_identical_baseline_refused(self):
        self.seal()
        path = self.profile / "catalog/server.js"
        replacement = self.profile / "replacement"
        replacement.write_bytes(path.read_bytes())
        replacement.replace(path)
        self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                         "unresolved-plugin-retention")

    def test_new_unknown_catalog_file_refused(self):
        self.seal()
        (self.profile / "catalog/new.js").write_bytes(b"selected")
        self.assertEqual(self.audit.finish([".codex/catalog/new.js"])["status"],
                         "unresolved-plugin-retention")

    def test_preserved_data_member_addition_refused(self):
        self.seal()
        (self.namespace / "new").write_bytes(b"x")
        self.assertEqual(self.audit.finish([self.raw(self.leaf)])["status"],
                         "unresolved-plugin-retention")

    def test_symlink_escape_and_chain_refused(self):
        for target in ("../../../../../../outside", "../second"):
            with self.subTest(target=target):
                self.link.unlink()
                self.link.symlink_to(target)
                with self.assertRaises(ValueError):
                    API["HeldTree"](self.namespace, bin_links=True)

    def test_wrong_source_revision_or_physical_registration_refused(self):
        for mutate in (
                lambda b: b.update(physical_artifact_id="foreign"),
                lambda b: b["package_revision"].update(resolved_revision="d" * 40),
                lambda b: b.update(native_profile_root=str(self.root))):
            with self.subTest(mutate=mutate):
                original = copy.deepcopy(self.state)
                mutate(self.state["installations"][0]["clients"]["binding-1"])
                self.write_state()
                with self.assertRaises(ValueError):
                    self.seal()
                self.state = original

    def test_unreadable_special_walk_and_ambiguous_paths_refused(self):
        for raw in (".codex/catalog/server.js: unreadable",
                    ".codex/catalog/server.js: non-regular",
                    ".codex/.walk-error: unreadable-walk",
                    ".codex/../foreign", ".codex/x: unreadable: symlink"):
            with self.subTest(raw=raw), self.assertRaises(ValueError):
                API["residual_path"](self.profile, raw)

    def test_hardlinks_and_bound_refused(self):
        extra = self.namespace / "hardlink"
        os.link(self.leaf, extra)
        with self.assertRaises(ValueError):
            API["HeldTree"](self.namespace, bin_links=True)
        extra.unlink()
        globals_ = API["HeldTree"].__init__.__globals__
        previous = globals_["FILE_LIMIT"]
        globals_["FILE_LIMIT"] = 1
        try:
            with self.assertRaises(ValueError):
                API["HeldTree"](self.namespace, bin_links=True)
        finally:
            globals_["FILE_LIMIT"] = previous

    def test_changed_data_bytes_refused(self):
        self.seal()
        self.leaf.write_bytes(b"changed")
        self.assertEqual(self.audit.finish([self.raw(self.leaf)])["status"],
                         "unresolved-plugin-retention")

    def test_replaced_data_parent_refused(self):
        self.seal()
        old = self.namespace / "npm-runtime"
        old.rename(self.namespace / "saved-runtime")
        old.mkdir()
        self.assertEqual(self.audit.finish([self.raw(self.leaf)])["status"],
                         "unresolved-plugin-retention")

    def test_preexisting_selected_data_namespace_refused(self):
        self.audit.close()
        self.link.unlink()
        self.audit = API["CodexPluginRetention"](self.profile, "0.162.0")
        self.addCleanup(self.audit.close)
        with self.assertRaises(ValueError):
            self.seal()

    def test_physical_traversal_and_actual_link_chain_refused(self):
        other = self.link.parent / "other"
        other.symlink_to("../tool.js")
        for target in ("../missing/../tool.js", "other/../tool", "../tool.js/.",
                       "../tool.js/", "other"):
            with self.subTest(target=target):
                self.link.unlink()
                self.link.symlink_to(target)
                with self.assertRaises(ValueError):
                    API["HeldTree"](self.namespace, bin_links=True)

    def test_named_replacement_after_full_read_refused(self):
        self.seal()
        original = API["_read_regular"]
        descriptor = self.audit.baseline.fds["catalog/server.js"]
        path = self.profile / "catalog/server.js"
        def read(fd, *args, **kwargs):
            result = original(fd, *args, **kwargs)
            if fd == descriptor:
                replacement = self.profile / "replacement"
                replacement.write_bytes(path.read_bytes())
                replacement.replace(path)
            return result
        with patch.dict(API["HeldTree"].verify.__globals__, _read_regular=read):
            self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                             "unresolved-plugin-retention")

    def test_later_data_read_cannot_mutate_earlier_baseline_or_members(self):
        for mutation in ("baseline", "data-file", "data-members"):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp).resolve()
                profile = root / ".codex"
                profile.mkdir()
                first = profile / "baseline"
                first.write_bytes(b"original")
                audit = API["CodexPluginRetention"](profile, "0.162.0")
                data = profile / "data"
                data.mkdir()
                (data / "a").write_bytes(b"a")
                (data / "b").write_bytes(b"b")
                audit.data_tree = API["HeldTree"](data, quota=audit.quota)
                audit.owner = {"synthetic": "custody-only-control"}
                descriptors = {audit.data_tree.fds[name]: data / name for name in ("a", "b")}
                seen = []
                original = API["_read_regular"]
                def read(fd, *args, **kwargs):
                    result = original(fd, *args, **kwargs)
                    if fd in descriptors:
                        seen.append(descriptors[fd])
                        if len(seen) == 2:
                            if mutation == "baseline":
                                first.write_bytes(b"mutated")
                            elif mutation == "data-file":
                                seen[0].write_bytes(b"mutated")
                            else:
                                (data / "new").write_bytes(b"new")
                    return result
                try:
                    with patch.dict(API["HeldTree"].verify.__globals__, _read_regular=read):
                        self.assertEqual(audit.finish([".codex/baseline", ".codex/data/a"])["status"],
                                         "unresolved-plugin-retention")
                finally:
                    audit.close()

    def test_all_descriptors_closed_after_one_close_error(self):
        tree = API["HeldTree"](self.namespace, bin_links=True)
        descriptors = tuple(tree.fds.values())
        real_close, attempts = os.close, []
        def close(fd):
            attempts.append(fd)
            real_close(fd)
            if len(attempts) == 1:
                raise OSError("injected close failure after actual close")
        with patch.object(os, "close", close), self.assertRaises(OSError):
            tree.close()
        self.assertEqual(set(attempts), set(descriptors))
        self.assertFalse(tree.fds)
        self.assertFalse(tree.quota.owned)
        for fd in descriptors:
            with self.assertRaises(OSError):
                os.fstat(fd)

    def test_aggregate_descriptor_quota_covers_transient_authority_read(self):
        globals_ = API["HeldTree"].__init__.__globals__
        quota = API["Descriptors"]()
        tree = API["HeldTree"](self.namespace, bin_links=True, quota=quota)
        held = set(quota.owned)
        try:
            with patch.dict(globals_, FD_LIMIT=len(held)), self.assertRaises(ValueError):
                API["_read"](self.data / "state-v2.json", quota, API["Budget"]())
            self.assertEqual(quota.owned, held)
        finally:
            tree.close()

    def test_cooperative_deadline_checked_after_full_read(self):
        clock = [0.]
        original = os.read
        def read(fd, size):
            result = original(fd, size)
            clock[0] = 6.
            return result
        with patch.object(API["time"], "monotonic", lambda: clock[0]):
            with patch.object(os, "read", read), self.assertRaises(ValueError):
                API["HeldTree"](self.namespace, bin_links=True)

    def test_ownership_metadata_includes_uid_and_gid(self):
        from types import SimpleNamespace
        info = self.profile.stat()
        value = dict(st_dev=info.st_dev, st_ino=info.st_ino, st_mode=info.st_mode,
                     st_uid=info.st_uid, st_gid=info.st_gid)
        self.assertNotEqual(API["_object"](SimpleNamespace(**value)),
                            API["_object"](SimpleNamespace(**dict(value, st_uid=info.st_uid + 1))))
        self.assertNotEqual(API["_object"](SimpleNamespace(**value)),
                            API["_object"](SimpleNamespace(**dict(value, st_gid=info.st_gid + 1))))

    def test_needs_rebind_and_native_entry_types_refused(self):
        for value in (True, 0, None):
            self.state["installations"][0]["needs_rebind"] = value
            self.write_state()
            with self.subTest(value=value), self.assertRaises(ValueError):
                self.seal()
        del self.state["installations"][0]["needs_rebind"]
        self.write_state()
        config = self.profile / "config.toml"
        original = config.read_text()
        for replacement in ("enabled = false", "enabled = 1"):
            config.write_text(original.replace("enabled = true", replacement))
            with self.subTest(replacement=replacement), self.assertRaises(ValueError):
                self.seal()
        config.write_text(original)
        market = "agentplugins-" + hashlib.sha256(b"physical-1").hexdigest()[:12]
        config.write_text(original.replace('[plugins."selected@' + market
                          + '"]\nenabled = true', '[plugins]\n"selected@' + market + '" = true'))
        with self.assertRaises(ValueError):
            self.seal()
        config.write_text(original)

    def signed_fixture(self):
        # Source values are the existing fixtures.json atlassian pin; state
        # construction follows usecase/directory_feed_transition_test.go.
        fixtures = json.loads((Path(__file__).resolve().parents[1] /
                               "tests/live-install/fixtures.json").read_text())
        pin = fixtures["signed_release_pins"]["atlassian"]
        record = self.state["installations"][0]
        record["declared_name"] = "atlassian"
        record["package"]["declared_name"] = "atlassian"
        self.public["name"] = "atlassian"
        config = self.profile / "config.toml"
        config.write_text(config.read_text().replace("selected@", "atlassian@"))
        market = "agentplugins-" + hashlib.sha256(b"physical-1").hexdigest()[:12]
        namespace = hashlib.sha256((market + "\0atlassian").encode()).hexdigest()
        new_namespace = self.namespace.parent / namespace
        leaf_relative, link_relative = self.leaf.relative_to(self.namespace), self.link.relative_to(self.namespace)
        self.namespace.rename(new_namespace)
        self.namespace = new_namespace
        self.leaf, self.link = self.namespace / leaf_relative, self.namespace / link_relative
        source = record["source"]
        source.update(repository=pin["repository"], package_subpath=pin["path"],
                      resolved_revision=pin["revision"], tree_digest=pin["tree_digest"])
        record["package"]["manifest_digest"] = pin["manifest_digest"]
        revision = record["clients"]["binding-1"]["package_revision"]
        revision.update(resolved_revision=pin["revision"], tree_digest=pin["tree_digest"],
                        manifest_digest=pin["manifest_digest"], distribution_id=pin["distribution"],
                        release_sequence=pin["sequence"])
        record.update(origin_mode="directory", directory={
            "product_id": "atlassian", "distribution_id": pin["distribution"],
            "distribution_kind": "community", "desired_release_sequence": pin["sequence"]})
        self.public.update(source=pin["repository"] + "/" + pin["path"], directory={
            "product_id": "atlassian", "recorded_distribution": pin["distribution"],
            "recorded_revision": pin["revision"], "recorded_release_sequence": pin["sequence"]})
        self.write_state()
        return pin

    def test_reviewed_signed_git_pin_positive(self):
        pin = self.signed_fixture()
        self.audit.before_remove(self.data, self.public, "atlassian",
                                 selector="atlassian", expected_pin=pin)
        self.assertEqual(self.audit.finish([self.raw(self.leaf)])["status"],
                         "verified-unchanged-or-preserved-at-checkpoint")

    def test_signed_distribution_sequence_and_boolean_sequence_refused(self):
        pin = self.signed_fixture()
        for field, value in (("distribution_id", "foreign/plugin"),
                             ("desired_release_sequence", pin["sequence"] + 1),
                             ("desired_release_sequence", True)):
            original = copy.deepcopy(self.state)
            self.state["installations"][0]["directory"][field] = value
            self.write_state()
            with self.subTest(field=field, value=value), self.assertRaises(ValueError):
                self.audit.before_remove(self.data, self.public, "atlassian",
                                         selector="atlassian", expected_pin=pin)
            self.state = original
        self.write_state()
        wrong = dict(pin, sequence=True)
        with self.assertRaises(ValueError):
            self.audit.before_remove(self.data, self.public, "atlassian",
                                     selector="atlassian", expected_pin=wrong)

    def test_optional_archive_source_remains_outside_proof_scope(self):
        source = self.state["installations"][0]["source"]
        source.pop("repository")
        source.pop("resolved_revision")
        source.update(source_kind="archive", source_url="https://example.test/source.zip",
                      canonical_source="https://example.test/source.zip")
        self.write_state()
        with self.assertRaises(ValueError):
            self.seal()

    def remove_namespace(self):
        self.link.unlink()
        self.leaf.unlink()
        self.link.parent.rmdir()
        self.leaf.parent.rmdir()
        (self.namespace / "npm-runtime").rmdir()
        self.namespace.rmdir()

    def test_public_unsettled_or_untyped_flags_refused(self):
        original = copy.deepcopy(self.public)
        for flag in ("needs_rebind", "mixed_version"):
            for value in (True, 0, None):
                self.public = copy.deepcopy(original)
                self.public[flag] = value
                with self.subTest(flag=flag, value=value), self.assertRaises(ValueError):
                    self.seal()
        self.public = original
        self.public.update(needs_rebind=False, mixed_version=False)
        self.seal()
        self.assertEqual(self.audit.finish([self.raw(self.leaf)])["status"],
                         "verified-unchanged-or-preserved-at-checkpoint")

    def test_held_absence_allows_unchanged_baseline_with_explicit_scope(self):
        self.remove_namespace()
        self.seal()
        result = self.audit.finish([".codex/catalog/server.js"])
        self.assertEqual(result["status"], "verified-unchanged-or-preserved-at-checkpoint")
        self.assertEqual(result["namespace_absence"]["missing_components"], [self.namespace.name])
        self.assertEqual(result["namespace_absence"]["nearest_existing_parent"],
                         str(self.namespace.parent))
        self.assertIs(result["atomic_snapshot_claimed"], False)
        self.assertIs(result["writer_closure_claimed"], False)
        self.assertEqual(result["semantics"], "sequential-per-object-checkpoints")

    def test_new_empty_namespace_refuses_baseline_residue(self):
        self.remove_namespace()
        self.seal()
        self.namespace.mkdir()
        result = self.audit.finish([".codex/catalog/server.js"])
        self.assertEqual(result["status"], "unresolved-plugin-retention")
        self.assertIs(result["atomic_snapshot_claimed"], False)
        self.assertIs(result["writer_closure_claimed"], False)

    def test_new_nonidentity_data_refuses_baseline_residue(self):
        self.remove_namespace()
        self.seal()
        self.namespace.mkdir()
        (self.namespace / "ordinary.txt").write_text("ordinary persistent data")
        self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                         "unresolved-plugin-retention")

    def test_absence_parent_replacement_refuses_baseline_residue(self):
        self.remove_namespace()
        self.seal()
        parent = self.namespace.parent
        parent.rename(parent.with_name("replaced-parent"))
        parent.mkdir()
        self.assertFalse(self.namespace.exists())
        self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                         "unresolved-plugin-retention")

    def test_absence_tracks_all_missing_components_from_nearest_parent(self):
        self.remove_namespace()
        self.namespace.parent.rmdir()
        self.namespace.parent.parent.rmdir()
        self.seal()
        result = self.audit.finish([".codex/catalog/server.js"])
        self.assertEqual(result["status"], "verified-unchanged-or-preserved-at-checkpoint")
        self.assertEqual(result["namespace_absence"]["missing_components"],
                         ["data", "agent-plugins", self.namespace.name])
        self.namespace.parent.parent.mkdir()
        self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                         "unresolved-plugin-retention")

    def test_absence_rechecked_after_baseline_full_read(self):
        self.remove_namespace()
        self.seal()
        original = API["_read_regular"]
        descriptor = self.audit.baseline.fds["catalog/server.js"]
        def read(fd, *args, **kwargs):
            result = original(fd, *args, **kwargs)
            if fd == descriptor:
                self.namespace.mkdir()
            return result
        with patch.dict(API["HeldTree"].verify.__globals__, _read_regular=read):
            self.assertEqual(self.audit.finish([".codex/catalog/server.js"])["status"],
                             "unresolved-plugin-retention")

    def test_version_scope_refused(self):
        with self.assertRaises(ValueError):
            API["CodexPluginRetention"](self.profile, "0.153.4")

    def test_each_lifecycle_gate_is_required(self):
        fields = dict(installed=True, repeat_verified=True, sentinels_preserved=True,
                      native_uninstall={"completed": True}, cleanup={"exit_code": 0},
                      post_list={"exit_code": 0}, post_list_schema_valid=True,
                      post_list_empty=True, old_ids={"old"}, remaining_ids=set(),
                      post_remove_identity_registered=False)
        self.assertTrue(API["lifecycle_ready"](**fields))
        negatives = dict(installed=False, repeat_verified=False, sentinels_preserved=False,
                         native_uninstall={"completed": False}, cleanup={"exit_code": 1},
                         post_list={"exit_code": 1}, post_list_schema_valid=False,
                         post_list_empty=False, remaining_ids={"old"},
                         post_remove_identity_registered=True)
        for key, value in negatives.items():
            with self.subTest(key=key):
                self.assertFalse(API["lifecycle_ready"](**dict(fields, **{key: value})))


@unittest.skipUnless(sys.platform == "linux", "Linux retention integration")
class RetentionLifecycleIntegrationTests(unittest.TestCase):
    def run_case(self, *, native_completed=True, custody_error=False):
        import importlib.util
        path = Path(__file__).with_name("live-install-audit.py")
        spec = importlib.util.spec_from_file_location("retention_live_control", path)
        audit = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(audit)
        events, commands, removed = [], [], [False]
        source = "github:owner/repo@" + "a" * 40
        public = {"installation_id": "installed", "name": "selected", "source": "owner/repo",
                  "clients": [{"client_id": "codex", "scope": "user",
                               "materialization": "materialized", "activation": "active",
                               "verification": "installation_verified",
                               "package_revision": {"resolved_revision": "a" * 40}}]}
        class SQLite:
            native_version = "0.162.0"
            def __init__(self, *args):
                events.append("native-warm")
            def guard(self):
                pass
            def checkpoint(self, phase):
                events.append("sqlite:" + phase)
            def evidence(self):
                return {"synthetic": True}
        class Retention:
            def __init__(self, profile, version):
                events.append("baseline-custody")
                self.profile = profile
            def before_remove(self, data, selected, identity, **kwargs):
                events.append("before-remove-custody")
                if custody_error:
                    raise ValueError("injected custody refusal")
            def finish(self, residuals):
                events.append("retention-finish")
                return {"status": "verified-unchanged-or-preserved-at-checkpoint",
                        "residuals": [{"raw": value} for value in residuals]}
            def close(self):
                events.append("retention-close")
        def call(argv, env, cwd, *args, **kwargs):
            command = argv[1]
            commands.append(list(argv))
            events.append("public:" + command)
            if command == "add":
                file = Path(env["CODEX_HOME"]) / "catalog/selected.txt"
                if not file.exists():
                    file.parent.mkdir()
                    file.write_text("selected native identity")
                document = {"data": {"actions": []}}
            elif command == "remove":
                removed[0] = True
                document = {"data": {}}
            elif command == "list":
                document = {"data": {"installations": [] if removed[0] else [public]}}
            else:
                raise AssertionError("unexpected public command")
            return {"exit_code": 0, "stdout_tail": "", "stderr_tail": "",
                    "timeout": False, "_json": copy.deepcopy(document)}
        def uninstall(*args):
            events.append("native-uninstall")
            return {"completed": native_completed}
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp).resolve() / "run"
            with patch.object(audit, "AGENT_BINARIES", {"codex": "/inert/codex"}), \
                 patch.dict(audit.CODEX_RUNTIME_AUDIT, CodexPluginSQLiteAudit=SQLite), \
                 patch.dict(audit.CODEX_RETENTION_AUDIT, CodexPluginRetention=Retention), \
                 patch.object(audit, "call", call), \
                 patch.object(audit, "uninstall_native_codex_plugin", uninstall):
                row = audit.do_case("/inert/uap", "chatgpt", "plugin", source, 0, root,
                                    {source: "selected"}, {})
        return row, events, commands

    def test_actual_do_case_orders_custody_and_preserves_raw_findings(self):
        row, events, commands = self.run_case()
        self.assertEqual([argv[1] for argv in commands],
                         ["add", "list", "add", "list", "remove", "list"])
        source = "github:owner/repo@" + "a" * 40
        add = ["/inert/uap", "add", source, "--target", "chatgpt", "--scope", "user",
               "--format", "json"]
        listing = ["/inert/uap", "list", "--target", "chatgpt", "--scope", "user",
                   "--format", "json"]
        remove = ["/inert/uap", "remove", "selected", "--target", "chatgpt", "--scope", "user",
                  "--external-uninstalled", "--format", "json"]
        self.assertEqual(commands, [add, listing, add, listing, remove, listing])
        self.assertLess(events.index("native-warm"), events.index("baseline-custody"))
        self.assertLess(events.index("before-remove-custody"), events.index("native-uninstall"))
        self.assertLess(events.index("native-uninstall"), events.index("public:remove"))
        self.assertLess(events.index("public:remove"), events.index("retention-finish"))
        self.assertEqual(row["native_residuals"], [".codex/catalog/selected.txt"])
        self.assertEqual(row["codex_plugin_retention_proof"]["residuals"],
                         [{"raw": ".codex/catalog/selected.txt"}])
        self.assertEqual(row["status"], "passed")

    def test_reached_native_negative_prevents_retention_override(self):
        row, events, commands = self.run_case(native_completed=False)
        self.assertEqual([argv[1] for argv in commands],
                         ["add", "list", "add", "list", "remove", "list"])
        self.assertIn("before-remove-custody", events)
        self.assertIn("native-uninstall", events)
        self.assertNotIn("retention-finish", events)
        self.assertFalse(row["removal_verified"])
        self.assertEqual(row["native_residuals"], [".codex/catalog/selected.txt"])
        self.assertEqual(row["status"], "cleanup-failed")

    def test_reached_custody_error_preserves_cleanup_and_failure(self):
        row, events, commands = self.run_case(custody_error=True)
        self.assertEqual([argv[1] for argv in commands],
                         ["add", "list", "add", "list", "remove", "list"])
        self.assertIn("native-uninstall", events)
        self.assertNotIn("retention-finish", events)
        self.assertFalse(row["removal_verified"])
        self.assertEqual(row["codex_plugin_retention_proof"]["status"],
                         "unresolved-plugin-retention-owner")
        self.assertEqual(row["native_residuals"], [".codex/catalog/selected.txt"])
        self.assertIn("retention-close", events)


if __name__ == "__main__":
    unittest.main()
