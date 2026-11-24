"""Offline negative controls; never a live closure qualification."""
import copy
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

    def test_retained_locked_child_after_remove_refused(self):
        owners = self.check()
        for owner in owners.values(): owner["locator"].rmdir()
        (self.data / "state-v2.json").write_text(json.dumps({"schema_version": 5, "installations": [], "environment_receipts": []}))
        self.documents[0].write_text("schema_version = 1\n[dependencies]\n[targets]\n")
        with mock.patch.object(audit, "checked_native_cleanup", return_value={}):
            with self.assertRaises(ValueError): closure.checked_cleanup(audit, owners, self.root, self.case, self.root, self.documents)

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
