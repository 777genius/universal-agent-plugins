"""Offline negative controls; never a live closure qualification."""
import copy
import base64
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import released_source_closure as closure

spec = importlib.util.spec_from_file_location("closure_audit", Path(__file__).with_name("released-source-audit.py"))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class ClosureTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        self.pins = closure.pins_for(audit)
        p = self.pins["marketing-kit"]
        self.case = {"name": "marketing-kit", "client": "antigravity", "scope": "user",
            "source": {"repository": p["repository"], "revision": p["resolved_revision"], "path": p["package_path"]},
            "tree_digest": p["tree_digest"], "manifest_digest": p["manifest_digest"]}
        self.authority = {"producer_commit": closure.PRODUCER, "snapshot_schema": 1,
            "snapshot_sequence": 49, "snapshot_digest": "sha256:" + "a" * 64,
            "client": "antigravity", "scope": "user", "signed_project_grant": False,
            "policy_status": "eligible", "ineligible_distributions": [],
            "key_id": "uap-directory-2026-11", "target": "linux-amd64", "observed_at": "2026-11-23T10:00:00Z",
            "selections": {p["distribution"]: p for a, p in self.pins.items() if a != "marketing-kit"}}
        self.data = self.root / "uap-data"
        self.data.mkdir()
        self.documents = [self.data / "uap.toml", self.data / "uap.lock"]
        self.documents[0].write_text('schema_version = 1\n[dependencies.marketing-kit]\ngit = "https://github.com/' + p["repository"] + '.git"\npath = "' + p["package_path"] + '"\ncommit = "' + p["resolved_revision"] + '"\n[targets]\nantigravity = ["marketing-kit"]\n')
        intent = {"schema_version": 1, "dependencies": [{"alias": "marketing-kit", "source_key": closure.source_key("marketing-kit", p),
            "path": p["package_path"], "selector": {"kind": "commit", "value": p["resolved_revision"]}}],
            "targets": [{"id": "antigravity", "aliases": ["marketing-kit"]}]}
        self.lock = {"lock_version": 1, "manifest_digest": audit.digest(json.dumps(intent, separators=(",", ":")).encode()), "packages": []}
        self.state = {"schema_version": 5, "installations": [], "environment_receipts": []}
        for alias, p in self.pins.items():
            key = closure.source_key(alias, p)
            locator = self.root / "managed" / alias
            locator.mkdir(parents=True)
            registry = {"distribution": p["distribution"], "distribution_id": p["distribution"],
                "release_sequence": p["release_sequence"], "repository": p["repository"], "path": p["package_path"],
                "commit": p["resolved_revision"], "requested": {"kind": "release", "value": str(p["release_sequence"])}}
            git = {"repository": p["repository"], "path": p["package_path"], "commit": p["resolved_revision"],
                "requested": {"kind": "commit", "value": p["resolved_revision"]}}
            package = {"alias": alias, "source_key": key, "version": p["package_version"], "targets": ["antigravity"],
                "digest_algorithm": "agentplugins-tree-sha256-v1", "tree_digest": p["tree_digest"], "manifest_digest": p["manifest_digest"],
                "source": {"git": git} if alias == "marketing-kit" else {"registry": registry}}
            if alias != "marketing-kit":
                package["required_by"] = [{"alias": "marketing-kit", "selector": {"kind": "version", "value": p["package_version"]}}]
            self.lock["packages"].append(package)
            binding = {"client_binding_id": alias, "client_id": "antigravity", "scope": "user", "materialization": "materialized",
                "physical_artifact_id": alias, "target_locator": str(locator), "package_revision": {
                "tree_digest": p["tree_digest"], "manifest_digest": p["manifest_digest"], "resolved_revision": p["resolved_revision"],
                "distribution_id": p["distribution"], "release_sequence": p["release_sequence"]}}
            directory = {"distribution_id": p["distribution"], "desired_release_sequence": p["release_sequence"],
                **{k: self.authority[k] for k in ("snapshot_schema", "snapshot_sequence", "snapshot_digest")}}
            self.state["installations"].append({"installation_id": alias, "declared_name": alias,
                "origin_mode": "direct" if alias == "marketing-kit" else "directory", "directory": None if alias == "marketing-kit" else directory,
                "source": {"repository": p["repository"], "package_subpath": p["package_path"],
                    "resolved_revision": p["resolved_revision"], "tree_digest": p["tree_digest"]},
                "package": {"version": p["package_version"], "manifest_digest": p["manifest_digest"]}, "clients": {alias: binding}})
            self.state["environment_receipts"].append({"installation_id": alias, "client_binding_id": alias, "receipt_id": alias,
                "phase": "committed", "client_id": "antigravity", "alias": alias, "environment_id": "user-default", "source_key": key, "package_digest": p["tree_digest"]})

    def check(self):
        self.documents[1].write_text(json.dumps(self.lock))
        (self.data / "state-v2.json").write_text(json.dumps(self.state))
        return closure.checked_closure(audit, self.case, self.root, self.documents, self.authority)

    def test_complete_exact_join(self):
        self.assertEqual(set(self.check()), set(self.pins))

    def test_state_hint_is_not_authority(self):
        self.authority = {"status": "unresolved", "records": self.state["installations"]}
        with self.assertRaises(ValueError): self.check()

    def test_newer_or_different_snapshot_cannot_replace_recorded_authority(self):
        for field, value in (("snapshot_sequence", 50), ("snapshot_digest", "sha256:" + "b" * 64)):
            with self.subTest(field=field):
                changed = copy.deepcopy(self.state)
                self.state["installations"][1]["directory"][field] = value
                with self.assertRaises(ValueError): self.check()
                self.state = changed

    def test_closure_member_or_dependency_edge_missing(self):
        original = copy.deepcopy(self.lock)
        for mutate in (lambda: self.lock["packages"].pop(), lambda: self.lock["packages"][1].pop("required_by")):
            self.lock = copy.deepcopy(original)
            mutate()
            with self.assertRaises(ValueError): self.check()

    def test_changed_signed_selection_refused(self):
        self.authority["selections"][self.pins["notion"]["distribution"]] = {**self.pins["notion"], "release_sequence": 2}
        with self.assertRaises(ValueError): self.check()

    def test_foreign_receipt_or_duplicate_owner_refused(self):
        for field in ("receipt_id", "source_key", "client_binding_id"):
            original = copy.deepcopy(self.state)
            self.state["environment_receipts"][1][field] = self.state["environment_receipts"][0][field]
            with self.assertRaises(ValueError): self.check()
            self.state = original

    def test_native_existence_alone_is_not_proof(self):
        owners = self.check()
        for verifier in (None, lambda alias, owner: {}, lambda alias, owner: {"content_verified": False}):
            with self.assertRaises(ValueError): closure.checked_native(audit, owners, verifier)

    def test_cleanup_requires_every_owned_locator_absent(self):
        owners = self.check()
        with self.assertRaises(ValueError): closure.checked_cleanup(audit, owners, self.root, self.case, self.root)

    def test_boolean_schema_and_release_identity_refused(self):
        for target, field in ((self.authority, "snapshot_schema"), (self.lock, "lock_version"),
                              (self.lock["packages"][1]["source"]["registry"], "release_sequence")):
            original = target[field]
            target[field] = True
            with self.subTest(field=field), self.assertRaises(ValueError): self.check()
            target[field] = original

    def test_nonobject_inventory_entries_refused(self):
        self.state["installations"][0] = None
        with self.assertRaises(ValueError): self.check()

    def test_playwright_path_without_source_bytes_is_not_proof(self):
        with self.assertRaises(ValueError):
            closure.checked_playwright_runtime(audit, self.root, self.root)

    def test_held_reader_cannot_be_replaced_by_pathname_bytes(self):
        with mock.patch.object(audit.live, "copilot_settings_read", return_value=(None, "unavailable")):
            with self.assertRaises(ValueError): self.check()

    def test_same_path_changed_between_held_observations_refused(self):
        self.documents[1].write_text(json.dumps(self.lock))
        with mock.patch.object(audit.live, "copilot_settings_read", side_effect=[("first", None), ("second", None)]):
            with self.assertRaises(ValueError): closure.held_read(audit, self.documents[1], self.root)

    def test_hermes_wrong_native_skill_owner_is_not_admitted(self):
        owner = self.check()["marketing-kit"]
        binding = owner["binding"]
        selected = self.root / "home/.hermes"
        binding["native_objects"] = [
            {"kind": "managed_package_directory", "logical_name": "marketing-kit", "object_id": "package:hermes:marketing-kit",
             "path": str(owner["locator"]), "protection_class": "managed", "managed_digest": "sha256:" + "a" * 64},
            {"kind": "foreign_skill", "logical_name": "campaign-planning", "protection_class": "managed",
             "path": str(selected / "skills/campaign-planning")},
        ]
        case = {**self.case, "client": "hermes"}
        with self.assertRaisesRegex(ValueError, "skill ownership"):
            closure.checked_member_native(audit, case, self.root, selected, "marketing-kit", owner)

    def test_antigravity_mcp_member_rejects_extra_native_files_and_directories(self):
        owner = self.check()["notion"]
        selected = self.root / "native"
        plugins = selected / "plugins"
        plugins.mkdir(parents=True)
        locator = plugins / "notion"
        owner["locator"].rename(locator)
        owner["locator"] = locator
        owner["binding"]["native_objects"] = [{"kind": "managed_package_directory",
            "logical_name": "notion", "object_id": "package:antigravity:notion", "path": str(locator),
            "protection_class": "managed", "managed_digest": "sha256:" + "a" * 64}]
        (locator / "README.md").write_text("immutable source documentation fixture\n")
        (locator / "plugin.json").write_text(json.dumps({
            "$schema": "https://antigravity.google/schemas/v1/plugin.json", "name": "notion", "version": "0.1.0"}))
        (locator / "mcp_config.json").write_text(json.dumps({
            "mcpServers": {"notion": {"serverUrl": "https://mcp.notion.com/mcp"}}}))
        def check():
            return closure.checked_member_native(audit, self.case, self.root, selected, "notion", owner)
        self.assertTrue(check()["content_verified"])
        for extra in ("skills/extra/SKILL.md", "hooks/hooks.json", "agents/extra.md", "unreviewed.txt", "empty/"):
            with self.subTest(extra=extra):
                path = locator / extra
                if extra.endswith("/"):
                    path.mkdir()
                else:
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text("unreviewed native fixture\n")
                with self.assertRaisesRegex(ValueError, "native inventory differs"):
                    check()
                import shutil
                if path.parent != locator:
                    shutil.rmtree(locator / Path(extra).parts[0])
                elif path.is_dir():
                    path.rmdir()
                else:
                    path.unlink()
        self.assertTrue(check()["content_verified"])

    def test_antigravity_reviewed_inventory_includes_all_directories(self):
        fixtures = {
            "marketing-kit": ["README.md", "SOURCES.md", "plugin.json", "uap.toml", "skills/campaign-planning/SKILL.md"],
            "notion": ["README.md", "plugin.json", "mcp_config.json"],
            "hubspot-crm": ["README.md", "plugin.json", "mcp_config.json"],
            "playwright": ["NOTICE", "README.md", "plugin.json", "mcp_config.json",
                "io.github.777genius.agentplugins/runtime/launcher.mjs",
                "io.github.777genius.agentplugins/runtime/package.json",
                "io.github.777genius.agentplugins/runtime/package-lock.json",
                "io.github.777genius.agentplugins/runtime/runtime.json"],
        }
        for alias, files in fixtures.items():
            with self.subTest(alias=alias):
                locator = self.root / ("native-" + alias)
                locator.mkdir()
                for relative in files:
                    path = locator / relative
                    path.parent.mkdir(parents=True, exist_ok=True)
                    path.write_text("reviewed inventory fixture\n")
                closure.checked_antigravity_inventory(audit, locator, self.root, alias)
                extra = locator / ("skills" if alias != "marketing-kit" else "skills/unreviewed-empty")
                extra.mkdir(parents=True)
                with self.assertRaisesRegex(ValueError, "native inventory differs"):
                    closure.checked_antigravity_inventory(audit, locator, self.root, alias)

    def test_antigravity_inventory_rechecks_directories_after_held_reads(self):
        locator = self.root / "replaced-plugin"
        locator.mkdir()
        for name in ("README.md", "plugin.json", "mcp_config.json"):
            (locator / name).write_text("held inventory fixture\n")
        original = closure.held_read
        changed = []
        def read_then_add(*args, **kwargs):
            body = original(*args, **kwargs)
            if not changed:
                (locator / "empty-added-during-read").mkdir()
                changed.append(True)
            return body
        with mock.patch.object(closure, "held_read", side_effect=read_then_add):
            with self.assertRaisesRegex(ValueError, "native inventory changed during held reads"):
                closure.checked_antigravity_inventory(audit, locator, self.root, "notion")
        self.assertEqual(changed, [True])

    def test_version_edge_and_release_pin_are_distinct(self):
        self.lock["packages"][1]["source"]["registry"]["requested"] = {"kind": "version", "value": "0.1.0"}
        with self.assertRaises(ValueError): self.check()

    def test_authenticated_refusal_partition_and_version(self):
        authority = copy.deepcopy(self.authority)
        distribution = self.pins["notion"]["distribution"]
        authority["selections"].pop(distribution)
        authority["ineligible_distributions"] = [distribution]
        authority["policy_status"] = "ineligible"
        refusal = {"code": "dependency_conflict", "distribution": distribution, "version": "0.1.0"}
        closure.checked_refusal_authority(audit, self.case, authority, refusal)
        for mutate in (lambda: authority["ineligible_distributions"].append(distribution),
                       lambda: authority["selections"].clear(),
                       lambda: refusal.update(version="9.9.9")):
            original = copy.deepcopy(authority)
            mutate()
            with self.assertRaises(ValueError): closure.checked_refusal_authority(audit, self.case, authority, refusal)
            authority = original

    def removed_closure_fixture(self):
        owners = self.check()
        selected = self.root / "native"
        selected.mkdir()
        plugins = selected / "plugins"
        plugins.mkdir()
        (selected / "foreign.txt").write_bytes(b"unmanaged source audit neighbor\n")
        records = []
        for alias, owner in owners.items():
            owner["locator"].rmdir()
            iid = owner["installation"]["installation_id"]
            physical = alias + "-" + audit.hashlib.sha256(iid.encode()).hexdigest()[:12]
            owner["binding"]["physical_artifact_id"] = physical
            owner["locator"] = plugins / physical
            directory = self.data / "plugin-data" / physical
            directory.mkdir(parents=True, mode=0o700)
            receipt_id = "data_" + audit.hashlib.sha256((iid + "\0" + physical + "\0user").encode()).hexdigest()[:24]
            receipt = {"data_receipt_id": receipt_id, "physical_backend_id": physical,
                "scope": "user", "locator": str(directory), "state": "owned",
                "ownership_digest": audit.digest(("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + str(directory)).encode())}
            marker = directory / ".agentplugins-data-owner.json"
            marker.write_bytes(json.dumps(receipt).encode() + b"\n")
            marker.chmod(0o600)
            record = copy.deepcopy(owner["installation"])
            record.update(clients={}, data_retained=True, data_receipts={receipt_id: receipt})
            records.append(record)
        (self.data / "state-v2.json").write_text(json.dumps({
            "schema_version": 5, "installations": records}))
        self.documents[0].write_text("schema_version = 1\n[dependencies]\n[targets]\n")
        self.documents[1].write_text(json.dumps({"lock_version": 1, "packages": []}))
        return owners, selected

    def test_complete_removed_closure_passes_cleanup(self):
        owners, selected = self.removed_closure_fixture()
        with mock.patch.object(audit, "checked_native_cleanup", return_value={}):
            retained = closure.checked_cleanup(audit, owners, selected, self.case, self.root, self.documents)
        self.assertEqual(set(retained), set(self.pins))
        self.assertTrue(all(value["state"] == "owned" for value in retained.values()))

    def test_remove_cannot_erase_retained_data_owners(self):
        owners, selected = self.removed_closure_fixture()
        (self.data / "state-v2.json").write_text(json.dumps({
            "schema_version": 5, "installations": [], "environment_receipts": []}))
        with mock.patch.object(audit, "checked_native_cleanup", return_value={}):
            with self.assertRaisesRegex(ValueError, "retained data owner inventory"):
                closure.checked_cleanup(audit, owners, selected, self.case, self.root, self.documents)

    def test_remove_refuses_remaining_or_null_environment_receipts(self):
        owners, selected = self.removed_closure_fixture()
        path = self.data / "state-v2.json"
        state = json.loads(path.read_text())
        for receipts in ([{"phase": "committed"}], None):
            with self.subTest(receipts=receipts):
                state["environment_receipts"] = receipts
                path.write_text(json.dumps(state))
                with mock.patch.object(audit, "checked_native_cleanup", return_value={}):
                    with self.assertRaisesRegex(ValueError, "closure retained saved owners"):
                        closure.checked_cleanup(audit, owners, selected, self.case, self.root, self.documents)

    def test_retained_locked_child_after_remove_refused(self):
        owners, selected = self.removed_closure_fixture()
        with mock.patch.object(audit, "checked_native_cleanup", return_value={}):
            closure.checked_cleanup(audit, owners, selected, self.case, self.root, self.documents)
            self.documents[1].write_text(json.dumps({"lock_version": 1, "packages": [self.lock["packages"][1]]}))
            with self.assertRaisesRegex(ValueError, "closure retained locked child or dependency edge"):
                closure.checked_cleanup(audit, owners, selected, self.case, self.root, self.documents)

    def test_runtime_cache_uses_encoded_artifact_bound_and_retains_exact_bytes(self):
        snapshot, envelope = b" " * (4 << 20), b"e" * (16 << 10)
        cache = {"schema_version": 1, "sequence": 49,
                 "snapshot": base64.b64encode(snapshot).decode(),
                 "envelope": base64.b64encode(envelope).decode()}
        body = json.dumps(cache).encode()
        cache_path = self.data / "directory-v1-cache.json"
        cache_path.write_bytes(body)
        evidence = self.root / "evidence"
        context = {"helper": self.root / "policy-helper", "frozen": self.root,
                   "target": "linux-amd64", "evidence_dir": evidence}
        authority = copy.deepcopy(self.authority)
        authority["snapshot_digest"] = audit.digest(snapshot)
        def authenticate(argv, **kwargs):
            self.assertEqual(Path(argv[argv.index("-cache") + 1]).read_bytes(), body)
            self.assertEqual(argv[argv.index("-digest") + 1], audit.digest(snapshot))
            authority["observed_at"] = argv[argv.index("-at") + 1]
            return json.dumps(authority)
        with mock.patch.object(closure.subprocess, "check_output", side_effect=authenticate) as helper:
            result = closure.authorize(audit, context, self.root, self.case, "add")
        helper.assert_called_once()
        retained = evidence / result["evidence_path"]
        self.assertEqual((retained / "snapshot.json").read_bytes(), snapshot)
        self.assertEqual((retained / "envelope.json").read_bytes(), envelope)
        self.assertEqual(cache_path.read_bytes(), body)
        with self.assertRaisesRegex(ValueError, "held ancestor read unavailable"):
            closure.held_read(audit, cache_path, self.root)

    def test_runtime_cache_oversized_record_and_decoded_artifacts_refused(self):
        cache_path = self.data / "directory-v1-cache.json"
        context = {"helper": self.root / "policy-helper", "frozen": self.root,
                   "target": "linux-amd64", "evidence_dir": self.root / "evidence"}
        for snapshot, envelope in ((b"s" * ((4 << 20) + 1), b"e"),
                                   (b"s", b"e" * ((16 << 10) + 1))):
            with self.subTest(snapshot=len(snapshot), envelope=len(envelope)):
                cache_path.write_text(json.dumps({"schema_version": 1, "sequence": 49,
                    "snapshot": base64.b64encode(snapshot).decode(),
                    "envelope": base64.b64encode(envelope).decode()}))
                with mock.patch.object(closure.subprocess, "check_output") as helper:
                    with self.assertRaisesRegex(ValueError, "signed artifacts exceed bounds"):
                        closure.authorize(audit, context, self.root, self.case, "add")
                    helper.assert_not_called()
        cache_path.write_bytes(b" " * (closure.MAX_CACHE_BYTES + 1))
        with self.assertRaisesRegex(ValueError, "held ancestor read unavailable"):
            closure.held_read(audit, cache_path, self.root, max_bytes=closure.MAX_CACHE_BYTES)

    def test_dangling_owned_package_is_not_clean(self):
        owners = self.check()
        locator = owners["marketing-kit"]["locator"]
        locator.rmdir()
        try:
            locator.symlink_to(self.root / "absent")
        except OSError:
            self.skipTest("host cannot create symlink")
        with self.assertRaises(ValueError): closure.checked_cleanup(audit, owners, self.root, self.case, self.root)

    def test_unowned_package_namespace_cannot_become_baseline(self):
        owners = self.check()
        selected = self.root / "native"
        selected.mkdir()
        (selected / "foreign.txt").write_bytes(b"unmanaged source audit neighbor\n")
        plugins = selected / "plugins"
        plugins.mkdir()
        for alias, owner in owners.items():
            owner["locator"].rmdir()
            owner["locator"] = plugins / alias
            owner["locator"].mkdir()
        closure.checked_namespace(audit, self.case, self.root, selected, owners)
        (plugins / "unowned-extra").mkdir()
        with self.assertRaisesRegex(ValueError, "unowned entries"):
            closure.checked_namespace(audit, self.case, self.root, selected, owners)

    def test_unowned_mcp_key_after_remove_refused(self):
        owners = self.check()
        for owner in owners.values(): owner["locator"].rmdir()
        selected = self.root / "native"
        selected.mkdir()
        (selected / "foreign.txt").write_bytes(b"unmanaged source audit neighbor\n")
        config = {audit.FOREIGN_NATIVE_MCP: {"command": "echo", "args": ["unrelated-native-audit"]},
                  "unowned-extra": {"url": "https://extra.invalid/mcp"}}
        (selected / "config.yaml").write_text(audit.yaml.safe_dump({"mcp_servers": config}))
        case = {**self.case, "client": "hermes"}
        with self.assertRaisesRegex(ValueError, "unowned MCP"):
            closure.checked_namespace(audit, case, self.root, selected, owners, removed=True)


if __name__ == "__main__":
    unittest.main()
