"""Inert contracts; these tests do not claim native Grok acceptance."""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

from grok_project_protocol_client import (document, exercise, installed_server,
                                          read_owned, selected_installation, retained_data, STATE_SCHEMA,
                                          HELPER_DIRECTORY)
from grok_project_protocol_e2e import observed_protocol, repair_refusal, Journey, snapshot
from grok_project_protocol_enablement import (enabled_receipt, expect_retained_receipt,
    ordinary_repair_state, preview_enablement, timestamp_ns)


class ContractTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name).resolve()
        self.active = self.root / ".grok/plugins/owned"
        self.data = self.root / ".uap/plugin-data/owned"
        self.active.mkdir(parents=True)
        self.data.mkdir(parents=True)
        self.binding = {"client_id": "grok", "scope": "project", "activation": "prepared",
            "verification": "package_validated", "physical_artifact_id": "owned",
            "target_locator": str(self.active), "native_profile_root": str(self.root / ".grok"),
            "data_receipt_id": "receipt", "client_binding_id": "binding"}
        self.installation = {"declared_name": "fixture", "installation_id": "install",
            "clients": {"grok": self.binding}, "data_receipts": {"receipt": {
                "scope": "project", "locator": str(self.data), "physical_backend_id": "owned", "state": "owned"}}}
        self.state = {"schema_version": STATE_SCHEMA, "installations": [self.installation]}
        self.state_path = self.root / ".uap/state-v2.json"
        self.write_state()
        command = self.active / "probe"
        command.write_text("fixture")
        child = self.active / "bin/probe"
        child.parent.mkdir()
        child.write_text("inert Go probe fixture")
        child.chmod(0o700)
        helper = self.active / HELPER_DIRECTORY / "agentplugins"
        helper.parent.mkdir(parents=True)
        helper.write_bytes(b"inert trusted-helper contract bytes")
        helper.chmod(0o700)
        self.helper_hash = hashlib.sha256(helper.read_bytes()).hexdigest()
        (helper.parent / "metadata.json").write_text(json.dumps({"protocol": 1,
            "sha256": self.helper_hash, "cliVersion": "inert-contract"}))
        self.server = {"command": str(helper), "args": ["--internal-stdio-v1", str(self.active),
            str(self.data), str(self.active), "plugin", "--argv-base64-v1",
            base64.b64encode(b"./bin/probe").decode()], "env": {
            "PLUGIN_ROOT": str(self.active), "PLUGIN_DATA": str(self.data),
            "UAP_TEST_ROOT": str(self.root), "UAP_TEST_NONCE": "nonce"}}

    def write_state(self):
        self.state_path.write_text(json.dumps(self.state))

    def select(self):
        self.write_state()
        return selected_installation(self.root, "fixture")

    def test_exact_owned_receipt(self):
        installation, binding, active, data = self.select()
        self.assertEqual(installation["installation_id"], "install")
        self.assertEqual(binding["client_binding_id"], "binding")
        self.assertEqual((active, data), (self.active, self.data))

    def test_owned_enablement_receipt_is_unique_settled_and_unchanged(self):
        owned = {"kind": "grok_enabled_plugin", "object_id": "exact", "managed_digest": "recorded"}
        binding = {"native_objects": [owned]}
        self.assertEqual(enabled_receipt(binding), owned)
        journey = type("Receipt", (), {"enabled_receipt": owned})()
        for changed in (
            {"native_objects": [owned, owned]},
            dict(binding, native_activation_attempt="pending"),
            dict(binding, pending_native_intent={"operation_id": "pending"}),
            {"native_objects": [dict(owned, object_id="foreign")]},
            {"native_objects": [dict(owned, managed_digest="changed")]},
        ):
            with self.subTest(binding=changed), self.assertRaises(ValueError):
                expect_retained_receipt(journey, changed)

    def ordinary_repair_fixture(self):
        old_group, group = "op-" + "1" * 32, "op-" + "2" * 32
        stamp, updated = "2026-11-14T00:00:01.1Z", "2026-11-14T00:00:02.123456789Z"
        tree, manifest, managed = ("sha256:" + value * 64 for value in "abc")
        self.installation.update(source={"tree_digest": tree}, package={"version": "1.0.0", "manifest_digest": manifest},
            operation_group_id=old_group, created_at=stamp, updated_at=stamp, data_retained=False)
        self.installation["clients"] = {"binding": self.binding}
        lifecycle = dict(activation="prepared", authentication="not_checked", policy="allowed", verification="package_validated")
        self.binding.update(**lifecycle, materialization="materialized", updated_at=stamp,
            package_revision=dict(version="1.0.0", tree_digest=tree, manifest_digest=manifest),
            native_objects=[dict(object_id="package:grok:owned", kind="managed_package_directory", logical_name="fixture",
                                 path=str(self.active), managed_digest=managed, protection_class="managed")])
        def receipt(identifier, sequence):
            operation = identifier + "-001"
            suffix = hashlib.sha256(operation.encode()).hexdigest()[:16]
            return dict(operation_id=operation, operation_group_id=identifier, sequence=sequence, mutation_type="directory_swap",
                        client_binding_id="binding", active_path=str(self.active), after_digest=managed, phase="committed",
                        staging_path=str(self.active.parent.parent / (".agentplugins-staging-" + suffix)),
                        backup_path=str(self.active.parent / (".agentplugins-backup-" + suffix)))
        self.binding["receipts"] = [receipt(old_group, 1)]
        self.state["transaction_receipts"] = []
        before = copy.deepcopy(self.state)
        after = copy.deepcopy(before)
        selected = after["installations"][0]
        selected.update(operation_group_id=group, updated_at=updated)
        selected["clients"]["binding"]["updated_at"] = updated
        selected["clients"]["binding"]["receipts"].append(dict(receipt(group, 2), before_digest=managed))
        result = dict(installation_id="install", mutated=True, requires_confirmation=False, group_phase="external_completed",
                      plan=dict(client_id="grok", scope="project", physical_artifact_id="owned", status="ready", package_mode="native", **lifecycle),
                      activation=lifecycle)
        output = dict(operation_id=group, plugin="fixture", version="1.0.0", source="local", tree_digest=tree,
                      manifest_digest=manifest, dry_run=False, result=result)
        response = dict(schema_version=1, command="repair", result="success", data=dict(operation_id=group,
            plugin="fixture", batch=True, status="completed", succeeded=1, failed=0, dry_run=False,
            targets=[dict(target="grok", status="external_completed", output=output)]))
        return before, after, response, (1794614402000000000, 1794614403000000000)

    def repair_proof(self, before, after, response, window):
        selected = before["installations"][0]
        return ordinary_repair_state(before, after, selected, selected["clients"]["binding"], self.active, response, window)

    def test_no_consent_repair_accepts_only_the_exact_producer_delta(self):
        before, after, response, window = self.ordinary_repair_fixture()
        original = copy.deepcopy(before)
        proof = self.repair_proof(before, after, response, window)
        self.assertEqual(before, original)
        self.assertEqual(proof["receipt"], after["installations"][0]["clients"]["binding"]["receipts"][-1])
        self.assertEqual(proof["updated_at"], "2026-11-14T00:00:02.123456789Z")
        for fault in ("no-commit", "history", "extra-commit", "phase", "sequence", "boolean-sequence", "group", "operation",
                      "before-digest", "after-digest", "staging", "backup", "scope", "id", "revision", "source", "data", "package",
                      "native-object", "native-attempt", "native-intent", "foreign-state", "extra-field", "typed-false", "timestamp-split",
                      "timestamp-outside", "timestamp-regressed"):
            changed = copy.deepcopy(after)
            selected = changed["installations"][0]; binding = selected["clients"]["binding"]; commit = binding["receipts"][-1]
            if fault == "no-commit": binding["receipts"].pop()
            elif fault == "history": binding["receipts"][0]["operation_id"] = "foreign"
            elif fault == "extra-commit": binding["receipts"].append(copy.deepcopy(commit))
            elif fault == "phase": commit["phase"] = "state_committed"
            elif fault == "sequence": commit["sequence"] = 3
            elif fault == "boolean-sequence": commit["sequence"] = True
            elif fault == "group": commit["operation_group_id"] = "op-" + "3" * 32
            elif fault == "operation": commit["operation_id"] = commit["operation_group_id"] + "-002"
            elif fault == "before-digest": commit["before_digest"] = "sha256:" + "0" * 64
            elif fault == "after-digest": commit["after_digest"] = "sha256:" + "0" * 64
            elif fault in ("staging", "backup"): commit[fault + "_path"] += "-foreign"
            elif fault == "scope": binding["scope"] = "user"
            elif fault == "id": selected["installation_id"] = "foreign"
            elif fault == "revision": binding["package_revision"]["version"] = "2.0.0"
            elif fault == "source": selected["source"]["tree_digest"] = "sha256:" + "0" * 64
            elif fault == "data": selected["data_receipts"] = {}
            elif fault == "package": selected["package"]["version"] = "2.0.0"
            elif fault == "native-object": binding["native_objects"].append(dict(kind="grok_enabled_plugin", object_id="adopted"))
            elif fault == "native-attempt": binding["native_activation_attempt"] = "pending"
            elif fault == "native-intent": binding["pending_native_intent"] = {"operation_id": "pending"}
            elif fault == "foreign-state": changed["transaction_receipts"].append({"operation_id": "foreign"})
            elif fault == "extra-field": selected["unreviewed"] = True
            elif fault == "typed-false": selected["data_retained"] = 0
            elif fault == "timestamp-split": binding["updated_at"] = "2026-11-14T00:00:02.2Z"
            else:
                selected["updated_at"] = binding["updated_at"] = "2026-11-14T00:00:" + ("04Z" if fault == "timestamp-outside" else "01Z")
            with self.subTest(fault=fault), self.assertRaises(ValueError): self.repair_proof(before, changed, response, window)

    def test_no_consent_repair_joins_the_actual_public_response(self):
        before, after, response, window = self.ordinary_repair_fixture()
        for fault in ("envelope", "batch", "count", "target", "operation", "id", "source", "tree", "manifest", "version", "scope", "artifact", "mutated", "confirmation", "active", "reuse"):
            body, changed = copy.deepcopy(response), copy.deepcopy(after)
            batch = body["data"]; target = batch["targets"][0]; output = target["output"]; result = output["result"]
            if fault == "envelope": body["schema_version"] = True
            elif fault == "batch": batch["batch"] = 1
            elif fault == "count": batch["succeeded"] = True
            elif fault == "target": target["target"] = "foreign"
            elif fault == "operation": output["operation_id"] = "op-" + "3" * 32
            elif fault == "id": result["installation_id"] = "foreign"
            elif fault == "source": output["source"] = "remote"
            elif fault == "tree": output["tree_digest"] = "sha256:" + "0" * 64
            elif fault == "manifest": output["manifest_digest"] = "sha256:" + "0" * 64
            elif fault == "version": output["version"] = "2.0.0"
            elif fault == "scope": result["plan"]["scope"] = "user"
            elif fault == "artifact": result["plan"]["physical_artifact_id"] = "foreign"
            elif fault == "mutated": result["mutated"] = False
            elif fault == "confirmation": result["requires_confirmation"] = True
            elif fault == "active": result["activation"]["activation"] = "active"
            else:
                old = before["installations"][0]["operation_group_id"]
                batch["operation_id"] = output["operation_id"] = changed["installations"][0]["operation_group_id"] = old
            with self.subTest(fault=fault), self.assertRaises(ValueError): self.repair_proof(before, changed, body, window)

    def test_repair_timestamps_are_canonical_exact_and_bounded(self):
        self.assertEqual(timestamp_ns("2026-11-14T00:00:02.123456789Z"), 1794614402123456789)
        self.assertEqual(timestamp_ns("2026-11-14T00:00:02Z"), 1794614402000000000)
        for value in (None, 1794614402, "2026-11-14T00:00:02+00:00", "2026-11-14T00:00:02.0Z",
                      "2026-11-14T00:00:02.1234567890Z", "2026-02-30T00:00:02Z", "2026-11-14T00:00:60Z"):
            with self.subTest(value=value), self.assertRaises(ValueError): timestamp_ns(value)
        before, after, response, window = self.ordinary_repair_fixture()
        for invalid in ((True, window[1]), (window[1], window[0]), (0, 1794614402123456788), list(window)):
            with self.subTest(window=invalid), self.assertRaises(ValueError): self.repair_proof(before, after, response, invalid)

    def test_preview_preserves_files_and_uses_only_verified_post_repair_state(self):
        before, after, response, window = self.ordinary_repair_fixture()
        home, source, other = (self.root / name for name in ("preview-home", "preview-source", "preview-other"))
        for path in (home / ".grok", source, other): path.mkdir(parents=True)
        config, foreign, event = home / ".grok/config.toml", home / "foreign", self.root / "preview-events"
        config.write_bytes(b'[plugins]\nenabled = ["uap-foreign-enabled"]\n')
        foreign.write_bytes(b"foreign home bytes")
        (source / "source").write_bytes(b"original source")
        (other / "foreign").write_bytes(b"other project")
        preserved = {path: path.read_bytes() for path in (config, foreign, self.active / "probe", source / "source", other / "foreign")}
        def check():
            for path in (config, foreign):
                if path.read_bytes() != preserved[path]: raise ValueError("protected fixture changed")
        for fault in (None, "no-commit", "package", "data", "source", "project-b", "config", "foreign-home", "witness", "dry-state", "dry-config"):
            for path, body in preserved.items(): path.write_bytes(body)
            for path in (event, self.data / "foreign"):
                if path.exists(): path.unlink()
            self.state_path.write_text(json.dumps(before))
            evidence = {}
            journey = SimpleNamespace(root=self.root, a=self.root, b=other, home=home, source=source, log=event, evidence=evidence, check=check)
            def command(label, verb, selector, *, extra=()):
                self.assertEqual((verb, selector), ("repair", "install"))
                if label == "repair-no-enable-consent":
                    self.assertEqual(extra, ("--yes",))
                    self.state_path.write_text(json.dumps(before if fault == "no-commit" else after))
                    changed = {"package": self.active / "probe", "data": self.data / "foreign", "source": source / "source",
                               "project-b": other / "foreign", "config": config, "foreign-home": foreign, "witness": event}
                    if fault in changed: changed[fault].write_bytes(b"unreviewed effect")
                    return copy.deepcopy(response)
                self.assertEqual((label, extra), ("repair-enable-dry-run", ("--confirm-plugin-enable", "--dry-run", "--yes")))
                if fault == "dry-state": self.state_path.write_text(json.dumps(before))
                if fault == "dry-config": config.write_bytes(b"unreviewed dry-run effect")
                return {"result": "success"}
            journey.cli = mock.Mock(side_effect=command)
            with self.subTest(fault=fault), mock.patch("grok_project_protocol_enablement.time.time_ns", side_effect=window):
                if fault is None:
                    preview_enablement(journey, before["installations"][0])
                    self.assertEqual(journey.cli.call_count, 2)
                    proof = evidence["enablement_previews"]
                    self.assertTrue(proof["no_consent_config_unchanged"] and proof["dry_run_config_and_state_unchanged"])
                    self.assertEqual(proof["post_repair_state"], after)
                    self.assertNotEqual(proof["before_state_sha256"], proof["after_state_sha256"])
                    self.assertNotIn("no_consent_config_and_state_unchanged", proof)
                else:
                    with self.assertRaises(ValueError): preview_enablement(journey, before["installations"][0])
                    self.assertEqual(journey.cli.call_count, 2 if fault.startswith("dry-") else 1)
                    self.assertNotIn("enablement_previews", evidence)

    def test_native_config_transition_accepts_only_exact_expected_foreign_bytes(self):
        for extra in (b"", b"foreign drift"):
            with self.subTest(extra=extra):
                root = self.root / ("exact-transition" if not extra else "foreign-transition")
                root.mkdir()
                journey = Journey(root, Path("/unused"), Path("/unused"), {"commands": []})
                config = journey.home / ".grok/config.toml"
                config.parent.mkdir()
                before = b'[plugins]\nenabled=["foreign"]\n'
                after = b'[plugins]\nenabled=["foreign", "project/full-id/owned"]\n'
                config.write_bytes(before)
                original_digest = hashlib.sha256(before).hexdigest()
                journey.protected[config] = original_digest
                def observed(*_args, **_kwargs):
                    config.write_bytes(after + extra)
                    return {"exit": 0, "error": None}, b"{}", b""
                with mock.patch("native_client_research.run_probe", side_effect=observed):
                    if extra:
                        with self.assertRaisesRegex(ValueError, "exact owned native member"):
                            journey.run("selected-effect", ["/unused"], config_after=after)
                        self.assertEqual(journey.protected[config], original_digest)
                    else:
                        journey.run("selected-effect", ["/unused"], config_after=after)
                        self.assertEqual(journey.protected[config], hashlib.sha256(after).hexdigest())
                self.assertEqual(config.read_bytes(), after + extra)

    def test_snapshot_detects_empty_directory_and_mode_changes(self):
        before = snapshot(self.active)
        empty = self.active / "empty"
        empty.mkdir(mode=0o700)
        self.assertNotEqual(snapshot(self.active), before)
        with_empty = snapshot(self.active)
        empty.chmod(0o500)
        self.assertNotEqual(snapshot(self.active), with_empty)
        empty.rmdir()
        self.assertEqual(snapshot(self.active), before)
        (self.active / "probe").chmod(0o700)
        self.assertNotEqual(snapshot(self.active), before)
        before_root_mode = snapshot(self.active)
        self.active.chmod(0o500)
        self.assertNotEqual(snapshot(self.active), before_root_mode)

    def test_snapshot_refuses_aliases_and_special_nodes(self):
        alias = self.active / "alias"
        alias.symlink_to(self.active / "probe")
        with self.assertRaisesRegex(ValueError, "alias"):
            snapshot(self.active)
        alias.unlink()
        os.mkfifo(self.active / "fifo")
        with self.assertRaisesRegex(ValueError, "special node"):
            snapshot(self.active)

    def test_foreign_scope_path_activation_and_receipt_refused(self):
        for key, value in (("scope", "user"), ("client_id", "codex"), ("activation", "active"),
                           ("target_locator", "/outside"), ("physical_artifact_id", ".."),
                           ("native_profile_root", "/outside"), ("data_receipt_id", "foreign")):
            with self.subTest(key=key):
                saved = self.binding[key]
                self.binding[key] = value
                with self.assertRaises(ValueError):
                    self.select()
                self.binding[key] = saved

    def test_legacy_future_and_noninteger_state_schema_refused(self):
        for version in (2, 3, 4, 6, 5.0, "5", None):
            with self.subTest(version=version):
                self.state["schema_version"] = version
                with self.assertRaisesRegex(ValueError, "state schema"):
                    self.select()

    def test_duplicate_and_absent_installation_refused(self):
        for entries in ([], [self.installation, self.installation]):
            with self.subTest(entries=len(entries)):
                self.state["installations"] = entries
                with self.assertRaises(ValueError):
                    self.select()

    def test_removed_installation_retains_exact_data_authority(self):
        held = copy.deepcopy(self.installation)
        self.installation["clients"] = {}
        self.installation["data_retained"] = True
        self.write_state()
        self.assertEqual(retained_data(self.root, held)["data_receipts"], held["data_receipts"])
        with self.assertRaises(ValueError):
            selected_installation(self.root, "fixture")

    def test_removed_data_loss_substitution_and_malformed_state_refused(self):
        held = copy.deepcopy(self.installation)
        removed = dict(copy.deepcopy(held), clients={}, data_retained=True)
        for key, replacement in (("data_receipts", {}), ("data_retained", False),
                                 ("clients", held["clients"]), ("installation_id", "foreign")):
            with self.subTest(key=key):
                self.state["installations"] = [dict(removed, **{key: replacement})]
                self.write_state()
                with self.assertRaises(ValueError):
                    retained_data(self.root, held)
        self.state["installations"] = []
        self.write_state()
        with self.assertRaises(ValueError):
            retained_data(self.root, held)
        self.state_path.write_text("{malformed")
        with self.assertRaises(ValueError):
            retained_data(self.root, held)

    def test_json_duplicates_nonfinite_and_limit_refused(self):
        for body in (b'{"a":1,"a":2}', b'{"a":NaN}', b'{"a":Infinity}', b"", b"a" * ((1 << 20) + 1)):
            with self.subTest(body=body[:20]), self.assertRaises(ValueError):
                document(body)

    def test_symlink_and_foreign_read_refused(self):
        alias = self.root / "alias"
        alias.symlink_to(self.state_path)
        for path in (alias, Path("/etc/passwd")):
            with self.subTest(path=path), self.assertRaises(ValueError):
                read_owned(path, self.root)

    def test_installed_config_is_exact_and_credential_free(self):
        sidecar = self.active / ".mcp.json"
        sidecar.write_text(json.dumps({"mcpServers": {"probe": self.server}}))
        self.assertEqual(installed_server(self.active, self.data, self.root, "probe", "nonce", self.helper_hash, self.active), self.server)
        self.server["env"]["XAI_API_KEY"] = "not-a-real-key"
        sidecar.write_text(json.dumps({"mcpServers": {"probe": self.server}}))
        with self.assertRaisesRegex(ValueError, "environment"):
            installed_server(self.active, self.data, self.root, "probe", "nonce", self.helper_hash, self.active)

    def test_foreign_command_and_extra_server_refused(self):
        for command, extra in (("/bin/true", False), (self.server["command"], True)):
            self.server["command"] = command
            servers = {"probe": self.server}
            if extra:
                servers["foreign"] = self.server
            (self.active / ".mcp.json").write_text(json.dumps({"mcpServers": servers}))
            with self.assertRaises(ValueError):
                installed_server(self.active, self.data, self.root, "probe", "nonce", self.helper_hash, self.active)

    def test_installed_launcher_rejects_legacy_cwd_wrong_roots_transport_and_source_hash(self):
        for fault in ("cwd", "data", "anchor", "encoding", "child", "helper-hash", "metadata"):
            with self.subTest(fault=fault):
                server = copy.deepcopy(self.server)
                expected_hash = self.helper_hash
                if fault == "cwd": server["cwd"] = str(self.active)
                if fault == "data": server["args"][2] = str(self.root / "foreign")
                if fault == "anchor": server["args"][4] = "data"
                if fault == "encoding": server["args"][5] = "--"
                if fault == "child": server["args"][6] = base64.b64encode(b"/foreign").decode()
                if fault == "helper-hash": expected_hash = "0" * 64
                metadata = Path(server["command"]).parent / "metadata.json"
                before = metadata.read_bytes()
                if fault == "metadata": metadata.write_text(json.dumps({"protocol": True, "sha256": self.helper_hash, "cliVersion": "inert"}))
                (self.active / ".mcp.json").write_text(json.dumps({"mcpServers": {"probe": server}}))
                try:
                    with self.assertRaises(ValueError):
                        installed_server(self.active, self.data, self.root, "probe", "nonce", expected_hash, self.active)
                finally:
                    metadata.write_bytes(before)

    def test_actual_installed_launcher_data_cwd_is_admitted_without_rewriting(self):
        self.server["args"][3:5] = [str(self.data), "data"]
        sidecar = self.active / ".mcp.json"
        sidecar.write_text(json.dumps({"mcpServers": {"probe": self.server}}))
        original = sidecar.read_bytes()
        self.assertEqual(installed_server(self.active, self.data, self.root, "probe", "nonce", self.helper_hash, self.data), self.server)
        self.assertEqual(sidecar.read_bytes(), original)

    def test_native_events_must_share_session_and_nonce(self):
        good = [{"session": "s", "pid": 1, "nonce": "n", "method": method}
                for method in ("initialize", "tools/list")]
        self.assertEqual(len(observed_protocol(good, "n", ("initialize", "tools/list"))), 1)
        for bad in (good[:1], list(reversed(good)), [dict(row, session="") for row in good],
                    [dict(row, pid=0) for row in good],
                    [good[0], dict(good[1], session="other")],
                    [good[0], dict(good[1], nonce="wrong")]):
            with self.subTest(rows=bad), self.assertRaises(ValueError):
                observed_protocol(bad, "n", ("initialize", "tools/list"))

    def test_partial_repair_refusal_requires_exact_failure(self):
        value = {"schema_version": 1, "command": "repair", "result": "failure", "data": {
            "status": "preflight_failed", "succeeded": 0, "failed": 1,
            "targets": [{"target": "grok", "output": {"result": {"mutated": False}}}]}}
        stderr = "native identity ownership is indeterminate; refusing repair\n"
        self.assertEqual(repair_refusal(json.dumps(value).encode(), stderr), value)
        for kind in ("success", "mutated", "wrong-target", "unrelated", "missing-target"):
            bad, diagnostic = copy.deepcopy(value), stderr
            if kind == "success":
                bad["result"] = "success"
            elif kind == "mutated":
                bad["data"]["targets"][0]["output"]["result"]["mutated"] = True
            elif kind == "wrong-target":
                bad["data"]["targets"][0]["target"] = "codex"
            elif kind == "unrelated":
                diagnostic = "network unavailable\n"
            else:
                bad["data"]["targets"] = []
            with self.subTest(kind=kind), self.assertRaises(ValueError):
                repair_refusal(json.dumps(bad).encode(), diagnostic)

    def test_partial_repair_refusal_rejects_boolean_and_float_integers(self):
        value = {"schema_version": 1, "command": "repair", "result": "failure", "data": {
            "status": "preflight_failed", "succeeded": 0, "failed": 1,
            "targets": [{"target": "grok", "output": {"result": {"mutated": False}}}]}}
        stderr = "native identity ownership is indeterminate; refusing repair\n"
        self.assertEqual(repair_refusal(json.dumps(value).encode(), stderr), value)
        for field, replacements in (("schema_version", (True, 1.0)),
                                    ("succeeded", (False, 0.0)), ("failed", (True, 1.0))):
            for replacement in replacements:
                bad = copy.deepcopy(value)
                target = bad if field == "schema_version" else bad["data"]
                target[field] = replacement
                with self.subTest(field=field, value=replacement), self.assertRaises(ValueError):
                    repair_refusal(json.dumps(bad).encode(), stderr)

    def test_missing_repair_restores_package_without_adopting_partial_changes(self):
        # Inert lifecycle contract only; hosted E2E must invoke the real UAP/Grok.
        (self.root / "journey").mkdir()
        journey = Journey(self.root / "journey", Path("/unused"), Path("/unused"), {"commands": []})
        active, data = journey.a / ".grok/plugins/owned", journey.a / ".uap/plugin-data/owned"
        active.mkdir(parents=True)
        data.mkdir(parents=True)
        original = {".mcp.json": b'{"mcpServers":{}}', "plugin.json": b'{"name":"fixture"}'}
        for name, body in original.items():
            (active / name).write_bytes(body)
        (data / "marker").write_text("preserved")
        state = journey.a / ".uap/state-v2.json"
        state.write_text('{"schema_version":5}')
        journey.inspect = lambda *args: None
        refusal = {"schema_version": 1, "command": "repair", "result": "failure", "data": {
            "status": "preflight_failed", "succeeded": 0, "failed": 1,
            "targets": [{"target": "grok", "output": {"result": {"mutated": False}}}]}}
        def run(label, *args, **kwargs):
            if label == "partial-repair-refusal":
                journey.evidence["commands"].append({"stderr":
                    "native identity ownership is indeterminate; refusing repair\n"})
                return json.dumps(refusal).encode()
            return b"{}"
        journey.run = run
        def restore(*args):
            active.mkdir()
            for name, body in original.items():
                (active / name).write_bytes(body)
        journey.cli = restore
        journey.repair_owned(active, data, {"installation_id": "held"})
        self.assertTrue(journey.evidence["repair"]["missing_owned_package_restored"])
        self.assertEqual(set(snapshot(journey.root / "repair-quarantine/package")), {".", "plugin.json"})
        self.assertEqual((data / "marker").read_text(), "preserved")

    def test_sequential_direct_protocol_and_bad_nonce(self):
        # Contract responder only; the genuine hosted E2E uses the unchanged Go probe.
        child = self.active / "contract_server.py"
        child.write_text('''import json, os, sys
for line in sys.stdin:
 r=json.loads(line)
 if "id" not in r: continue
 m=r["method"]; response={"jsonrpc":"2.0","id":r["id"]}
 if m=="initialize": value={"protocolVersion":"2025-06-18","serverInfo":{"name":"agentplugins-native-probe"}}
 elif m=="tools/list": value={"tools":[{"name":"inspect_runtime"}]}
 else:
  if r["params"]["arguments"]["nonce"]!="nonce":
   response["error"]={"code":-32602}; print(json.dumps(response),flush=True); continue
  facts={"nonce":"nonce","revision":"0","marker":"marker","plugin_root":os.environ["PLUGIN_ROOT"],"plugin_data":os.environ["PLUGIN_DATA"],"cwd":os.getcwd(),"session":"contract"}
  value={"isError":False,"content":[{"type":"text","text":json.dumps(facts)}]}
 response["result"]=value; print(json.dumps(response),flush=True)
''')
        server = dict(self.server, command=sys.executable, args=[str(child)])
        result = exercise(server, {"PATH": "/usr/bin:/bin"}, "nonce", "0", "marker", True, self.active, self.active)
        self.assertEqual(result["caller"], "direct-installed-stdio-client")
        self.assertFalse(result["grok_model_invoked"])
        self.assertEqual(len(result["transcript"]), 9)
        self.assertEqual(result["transcript"][-1]["message"]["error"]["code"], -32602)


if __name__ == "__main__":
    unittest.main()
