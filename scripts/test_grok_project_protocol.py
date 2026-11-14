"""Inert contracts; these tests do not claim native Grok acceptance."""
import base64
from contextlib import ExitStack
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
import grok_project_protocol_profile as profile_case
import grok_acp_bootstrap as bootstrap_case


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
        before_bytes = json.dumps(before).encode()
        after_bytes = (json.dumps(after, indent=2) + "\n").encode()
        selected = after["installations"][0]
        expected_observation = {
            "partial": True,
            "no_consent_config_unchanged": True,
            "no_consent_exact_directory_repair": {
                "operation_id": selected["operation_group_id"],
                "receipt": selected["clients"]["binding"]["receipts"][-1],
                "updated_at": selected["updated_at"],
                "started_at_unix_ns": window[0], "finished_at_unix_ns": window[1]},
            "post_repair_state_base64": base64.b64encode(after_bytes).decode(),
            "post_repair_state_bytes": len(after_bytes),
            "before_state_sha256": hashlib.sha256(before_bytes).hexdigest(),
            "after_state_sha256": hashlib.sha256(after_bytes).hexdigest(),
            "mcp_witness_execution_observed": False,
            "internal_acp_zero_proved_by_this_capture": False,
        }
        qualification = {key: False for key in ("passed", "model_invoked", "vendor_authentication_used",
            "grok_tools_call_qualified", "unmanaged_project_owned_enablement_qualified")}
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
        for fault in (None, "no-commit", "package", "data", "source", "project-b", "config", "foreign-home", "witness", "dry-command", "dry-command-package", "dry-command-state", "dry-command-witness", "dry-state", "dry-config"):
            for path, body in preserved.items(): path.write_bytes(body)
            for path in (event, self.data / "foreign"):
                if path.exists(): path.unlink()
            self.state_path.write_bytes(before_bytes)
            evidence = dict(qualification)
            journey = SimpleNamespace(root=self.root, a=self.root, b=other, home=home, source=source, log=event, evidence=evidence, check=check)
            def command(label, verb, selector, *, extra=()):
                self.assertEqual((verb, selector), ("repair", "install"))
                if label == "repair-no-enable-consent":
                    self.assertEqual(extra, ("--yes",))
                    self.state_path.write_bytes(before_bytes if fault == "no-commit" else after_bytes)
                    changed = {"package": self.active / "probe", "data": self.data / "foreign", "source": source / "source",
                               "project-b": other / "foreign", "config": config, "foreign-home": foreign, "witness": event}
                    if fault in changed: changed[fault].write_bytes(b"unreviewed effect")
                    return copy.deepcopy(response)
                self.assertEqual((label, extra), ("repair-enable-dry-run", ("--confirm-plugin-enable", "--dry-run", "--yes")))
                if fault == "dry-command-package": (self.active / "probe").write_bytes(b"unreviewed effect")
                if fault == "dry-command-state": self.state_path.write_bytes(before_bytes)
                if fault == "dry-command-witness": event.write_bytes(b"unreviewed effect")
                if fault is not None and fault.startswith("dry-command"): raise ValueError("command failed: repair-enable-dry-run")
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
                    expected_error = {
                        "dry-command": "command failed: repair-enable-dry-run",
                        "dry-command-package": "repair changed native config, package, data, source or Project B",
                        "dry-command-state": "repair preview changed saved ownership",
                        "dry-command-witness": "repair preview executed the MCP witness",
                    }.get(fault, ".")
                    with self.assertRaisesRegex(ValueError, expected_error): preview_enablement(journey, before["installations"][0])
                    self.assertEqual(journey.cli.call_count, 2 if fault.startswith("dry-") else 1)
                    self.assertNotIn("enablement_previews", evidence)
                for key in qualification:
                    self.assertIs(evidence[key], False)
                if fault is None or fault.startswith("dry-"):
                    self.assertIn("ordinary_repair_observation", evidence)
                    partial = evidence["ordinary_repair_observation"]
                    self.assertEqual(partial, expected_observation)
                    self.assertIs(partial["partial"], True)
                    self.assertEqual(base64.b64decode(partial["post_repair_state_base64"], validate=True), after_bytes)
                    self.assertNotIn("dry_run_config_and_state_unchanged", partial)
                else:
                    self.assertNotIn("ordinary_repair_observation", evidence)

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

    def test_failed_command_keeps_primary_exit_and_checks_original_guards(self):
        before = b'[plugins]\nenabled=["foreign"]\n'
        after = b'[plugins]\nenabled=["foreign", "project/full-id/owned"]\n'
        for exit_code, error, observed in ((1, None, before), (0, 'capture-refused', before),
                                           (1, None, after), (1, None, b'foreign drift')):
            with self.subTest(exit=exit_code, error=error, changed=observed != before):
                with tempfile.TemporaryDirectory(dir=self.root) as temporary:
                    journey = Journey(Path(temporary), Path('/unused'), Path('/unused'), {'commands': []})
                    config = journey.home / '.grok/config.toml'
                    config.parent.mkdir()
                    config.write_bytes(before)
                    prior = hashlib.sha256(before).hexdigest()
                    journey.protected[config] = prior
                    def failed(*_args, **_kwargs):
                        config.write_bytes(observed)
                        return {'exit': exit_code, 'error': error}, b'original stdout', b'original stderr'
                    with mock.patch('native_client_research.run_probe', side_effect=failed):
                        with self.assertRaisesRegex(ValueError, '^command failed: selected-effect$'):
                            journey.run('selected-effect', ['/unused'], config_after=after)
                    self.assertEqual(journey.protected[config], prior)
                    self.assertEqual(journey.evidence['failed_command'], dict(label='selected-effect', exit=exit_code,
                        capture_complete=error is None, protected_originals_unchanged=observed == before))
                    record = journey.evidence['commands'][0]
                    self.assertEqual(base64.b64decode(record['stdout_base64']), b'original stdout')
                    self.assertEqual(base64.b64decode(record['stderr_base64']), b'original stderr')

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


class NativeProfileContracts(unittest.TestCase):
    """Inert filesystem/capture contracts; no pinned vendor is executed."""
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.binary_pin = hashlib.sha256(b"inert binary identity").hexdigest()
        self.after_help = self.after_writer = lambda: None
        self.writer_calls = 0
        self.pins = {}
        for name in bootstrap_case.PINNED:
            if name.startswith("docs/user-guide/"):
                body = ("inert document " + name).encode()
                self.pins[name] = (len(body), hashlib.sha256(body).hexdigest(), 0o644)
        self.stack = ExitStack()
        self.addCleanup(self.stack.close)
        self.stack.enter_context(mock.patch.dict(bootstrap_case.PINNED, self.pins))
        self.stack.enter_context(mock.patch.dict(profile_case.ACQUISITION, binary_sha256=self.binary_pin))
        self.stack.enter_context(mock.patch("native_client_research.run_probe", side_effect=self.probe))
        self.start_journey(self.root)

    def start_journey(self, root):
        self.root = root
        self.journey = Journey(root, Path("/unused"), Path("/unused"), {"commands": []})
        self.journey.grok.write_bytes(b"inert binary identity")
        self.journey.grok.chmod(0o700)
        self.journey.run("grok-version", [self.journey.grok, "--version"], self.root)

    def probe(self, argv, cwd, env, output, label, **kwargs):
        self.assertEqual(argv[0], str(self.journey.grok))
        self.assertEqual(env, self.journey.env)
        self.assertTrue(kwargs["memory_only"])
        stderr = b""
        if argv[1:] == ["--version"]:
            stdout = b"grok 1.0.70 (f50904eaee6c)\n"
        elif argv[1:] == ["mcp", "--help"]:
            stdout = b"Usage: grok mcp [OPTIONS] <COMMAND>\nCommands:\n  disable  Disable an MCP server\n"
        elif argv[1:] == ["mcp", "disable", "--help"]:
            stdout = b"Usage: grok mcp disable [OPTIONS] <NAME>\n"
            self.after_help()
        elif argv[1:] == ["mcp", "disable", profile_case.LOCK_NAME]:
            self.writer_calls += 1
            self.assertFalse(os.path.lexists(self.root / profile_case.LOCK))
            profile = self.journey.home / ".grok"
            for name in ("docs", "docs/user-guide", "logs"):
                (profile / name).mkdir()
                (profile / name).chmod(0o755)
            for name in self.pins:
                (profile / name).write_bytes(("inert document " + name).encode())
                (profile / name).chmod(0o644)
            (profile / "logs/unified.jsonl").write_bytes(b"inert\n")
            (profile / "logs/unified.jsonl").chmod(0o644)
            config = profile / "config.toml"
            body = config.read_bytes().replace(b"disabled_mcp_servers = []", b'disabled_mcp_servers = ["'
                + profile_case.LOCK_NAME.encode() + b'"]').replace(b"enabled = true", b"enabled = false")
            config.write_bytes(body)
            lock = self.root / profile_case.LOCK
            with lock.open("xb"):
                pass
            lock.chmod(0o644)
            self.after_writer()
            stdout = b"Disabled exact inert witness\n"
        else:
            self.fail("unexpected native command: " + repr(argv))
        record = dict(probe=label, argv=argv, cwd=str(cwd), exit=0, error=None)
        for name, body in (("stdout", stdout), ("stderr", stderr)):
            record.update({name + "_bytes": len(body), name + "_sha256": hashlib.sha256(body).hexdigest()})
        return record, stdout, stderr

    def prepare(self):
        profile_case.prepare_native_profile(self.journey)

    def test_only_native_writer_creates_retained_lock_before_final_authored_config(self):
        original_binary = self.journey.grok.stat()
        self.prepare()
        proof = self.journey.evidence["native_profile_preparation"]
        self.assertTrue(proof["complete"] and proof["native_lock_created"] and proof["temporary_fixture_removed"])
        self.assertEqual(self.writer_calls, 1)
        self.assertNotIn(profile_case.LOCK, proof["before"])
        self.assertNotIn(profile_case.LOCK, proof["before_writer"])
        self.assertIn(profile_case.LOCK, proof["after"])
        self.assertEqual(len(set(proof["after"]) - set(proof["before"])), 32)
        self.assertFalse((self.root / profile_case.CONFIG).exists())
        self.assertFalse((self.root / profile_case.WITNESS).exists())
        self.assertEqual(profile_case.stable(self.journey.grok.stat()), profile_case.stable(original_binary))
        self.assertEqual(self.journey.evidence["commands"][proof["writer_command_index"]]["argv"],
                         [str(self.journey.grok), "mcp", "disable", profile_case.LOCK_NAME])
        self.assertEqual(proof["help_command_indices"], [0, 1, 2])
        (self.root / profile_case.CONFIG).write_bytes(b"separately authored final fixture")
        self.journey.check()
        with self.assertRaisesRegex(ValueError, "repeated"):
            self.prepare()

    def test_lock_created_by_help_is_not_native_writer_proof(self):
        self.after_help = lambda: (self.root / profile_case.LOCK).touch(mode=0o644)
        with self.assertRaisesRegex(ValueError, "before writer"):
            self.prepare()
        self.assertEqual(self.writer_calls, 0)
        self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])

    def test_existing_profile_refuses_without_overwriting_any_member(self):
        profile = self.journey.home / ".grok"
        profile.mkdir()
        path = profile / "config.toml"
        path.write_bytes(b"foreign fixture bytes")
        before = path.stat()
        with self.assertRaisesRegex(ValueError, "not fresh"):
            self.prepare()
        self.assertEqual(path.read_bytes(), b"foreign fixture bytes")
        self.assertEqual(profile_case.stable(path.stat()), profile_case.stable(before))
        self.assertEqual(self.writer_calls, 0)

    def test_changed_help_config_does_not_reach_writer(self):
        self.after_help = lambda: (self.root / profile_case.CONFIG).write_bytes(b"changed = true\n")
        with self.assertRaisesRegex(ValueError, "before writer"):
            self.prepare()
        self.assertEqual(self.writer_calls, 0)

    def test_writer_must_create_lock_and_preserve_all_other_authority(self):
        def fault(kind):
            lock = self.root / profile_case.LOCK
            if kind == "missing": lock.unlink()
            elif kind == "nonempty": lock.write_bytes(b"foreign")
            elif kind == "mode": lock.chmod(0o600)
            elif kind == "hardlink": os.link(lock, self.root / "extra-link")
            elif kind == "symlink":
                lock.unlink(); lock.symlink_to(self.journey.grok)
            elif kind == "unknown": (self.root / "unknown-native-output").write_bytes(b"unexpected")
            elif kind == "trust": (self.journey.home / ".grok/trusted_folders.toml").write_text("[folders]\n")
            elif kind == "witness": (self.root / profile_case.MARKER).write_bytes(b"unexpected")
            elif kind == "config":
                with (self.root / profile_case.CONFIG).open("ab") as stream: stream.write(b"extra = true\n")
            elif kind == "docs": (self.journey.home / ".grok/docs/user-guide/01-getting-started.md").write_bytes(b"changed")
            elif kind == "binary": self.journey.grok.write_bytes(b"changed binary")
        base = self.root
        for kind in ("missing", "nonempty", "mode", "hardlink", "symlink", "unknown", "trust", "witness", "config", "docs", "binary"):
            with self.subTest(kind=kind):
                # Each attempt owns a distinct fixture and cannot rebaseline the
                # previous refusal's files or authority.
                root = base / kind
                root.mkdir()
                self.start_journey(root)
                self.after_writer = lambda: fault(kind)
                with self.assertRaises((ValueError, OSError)):
                    self.prepare()
                self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])
                self.assertTrue((root / profile_case.CONFIG).exists())

    def test_retained_empty_lock_replacement_is_refused(self):
        self.prepare()
        lock = self.root / profile_case.LOCK
        retained = lock.stat()
        lock.rename(lock.with_name("original-lock"))
        lock.touch(mode=0o644)
        self.assertNotEqual(lock.stat().st_ino, retained.st_ino)
        with self.assertRaisesRegex(ValueError, "authority"):
            self.journey.check()
        lock.unlink()
        lock.with_name("original-lock").rename(lock)
        # Renaming the original also changes ctime and must stay refused.
        with self.assertRaisesRegex(ValueError, "authority"):
            self.journey.check()

    def test_retained_lock_cannot_move_to_another_profile_identity(self):
        self.prepare()
        profile = self.journey.home / ".grok"
        saved = self.journey.home / "saved-profile"
        profile.rename(saved)
        profile.mkdir(mode=0o700)
        os.link(saved / ".config-init.lock", profile / ".config-init.lock")
        with self.assertRaisesRegex(ValueError, "directory"):
            self.journey.check()

    def test_cleanup_keeps_replaced_fixture_and_complete_flag_false(self):
        original = profile_case.remove_fixture
        def replace_before_remove(journey, after, config):
            path = self.root / profile_case.CONFIG
            path.rename(path.with_name("native-original.toml"))
            path.write_bytes(config)
            return original(journey, after, config)
        with mock.patch.object(profile_case, "remove_fixture", side_effect=replace_before_remove):
            with self.assertRaisesRegex(ValueError, "authority"):
                self.prepare()
        self.assertTrue((self.root / profile_case.CONFIG).exists())
        self.assertTrue((self.root / profile_case.WITNESS).exists())
        self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])

    def test_cleanup_rejoins_witness_immediately_before_its_unlink(self):
        original = profile_case.os.unlink
        changed = b"foreign replacement must remain"
        def substitute(name, *, dir_fd=None):
            original(name, dir_fd=dir_fd)
            if name == "config.toml":
                path = self.root / profile_case.WITNESS
                path.rename(path.with_suffix(".original"))
                path.write_bytes(changed)
        with mock.patch.object(profile_case.os, "unlink", side_effect=substitute):
            with self.assertRaisesRegex(ValueError, "during removal"):
                self.prepare()
        self.assertEqual((self.root / profile_case.WITNESS).read_bytes(), changed)
        self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])
        self.journey.check()

    def test_incomplete_or_failed_writer_does_not_adopt_created_lock(self):
        original = self.probe
        def incomplete(*args, **kwargs):
            record, stdout, stderr = original(*args, **kwargs)
            if args[4] == "native-profile-disable-witness":
                record.update(exit=1, error="probe deadline exceeded")
            return record, stdout, stderr
        with mock.patch("native_client_research.run_probe", side_effect=incomplete):
            with self.assertRaisesRegex(ValueError, "command failed: native-profile-disable-witness"):
                self.prepare()
        self.assertIsNone(self.journey.native_profile)
        self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])
        self.assertEqual(self.journey.evidence["failed_command"], dict(label="native-profile-disable-witness",
            exit=1, capture_complete=False, protected_originals_unchanged=True))
        self.assertTrue((self.root / profile_case.LOCK).exists())
        self.assertTrue((self.root / profile_case.CONFIG).exists())

    def test_help_adapter_preserves_dual_receipts_and_refuses_nonempty_stderr(self):
        row = self.journey.evidence["commands"][0]
        before = copy.deepcopy(row)
        result = profile_case.stdout_for_help(row)
        self.assertEqual(row, before)
        self.assertEqual(result["merged_base64"], row["stdout_base64"])
        for field, value in (("exit", True), ("stderr_bytes", True), ("stdout_sha256", "0" * 64),
                             ("error", "capture failed"), ("stdout_bytes", 1 << 21)):
            with self.subTest(field=field), self.assertRaises(ValueError):
                profile_case.stdout_for_help(dict(row, **{field: value}))
        changed = dict(row, stderr_base64=base64.b64encode(b"foreign").decode(), stderr_bytes=7,
                       stderr_sha256=hashlib.sha256(b"foreign").hexdigest())
        with self.assertRaisesRegex(ValueError, "stderr"):
            profile_case.stdout_for_help(changed)


class DelegatedPublicationTests(unittest.TestCase):
    """Inert outer-admission receipts only; no native journey is manufactured."""
    def setUp(self):
        import grok_project_protocol_service as service
        self.service = service
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name).resolve()
        self.budget = service.shared.Budget(900)
        self.inputs = service.Inputs(self.budget)
        self.addCleanup(self.inputs.close)
        self.expected = dict(source_commit="a" * 40, source_tree="b" * 40,
                             installer_sha256="c" * 64, probe_sha256="d" * 64)
        self.env = dict(GITHUB_RUN_ID="123", GITHUB_RUN_ATTEMPT="1", GITHUB_EVENT_NAME="pull_request")

    def evidence(self, case, passed=False):
        value = {"schema_version": 1, **self.expected, "passed": passed, "commands": [],
            "stages": [{"stage": name} for name in self.service.STAGES] if passed else [],
            "vendor_authentication_used": False, "unmanaged_project_owned_enablement_qualified": passed}
        if not passed:
            value["error"] = "ValueError: inert original refusal"
        if case == "grok-protocol":
            value.update(model_invoked=False, grok_tools_call_qualified=False, transport="stdio")
            if passed:
                value.update(native_doctor_initialize_and_list_qualified=True, direct_installed_config_tools_call_qualified=True)
        else:
            value.update(github_run_id="123", github_run_attempt="1", github_event="pull_request", platform="Linux-x86_64",
                actual_model_used=False, scripted_local_provider=True, grok_native_tools_call_qualified=passed)
        return value

    def admit(self, value, case="grok-noauth", exit_code=1, error=None):
        return self.service.admit_evidence(json.dumps(value).encode(), case, {"exit": exit_code, "error": error},
                                          self.expected, self.env, self.budget)

    def test_literal_case_flags_source_exit_and_stage_prefix_are_joined(self):
        for case in self.service.CASE_FIELDS:
            negative = self.evidence(case)
            self.assertFalse(self.admit(negative, case))
            positive = self.evidence(case, True)
            self.assertTrue(self.admit(positive, case, 0))
            for change in ({"passed": True}, {"passed": 0}, {"vendor_authentication_used": 0},
                    {"source_commit": "e" * 40}, {"installer_sha256": "e" * 64}, {"source_tree": "e" * 40},
                    {"probe_sha256": "e" * 64}, {"unknown": False}, {"unmanaged_project_owned_enablement_qualified": True},
                    {"stages": [{"stage": "unknown"}]}, {"error": ""}, {"schema_version": True}):
                with self.subTest(case=case, change=change), self.assertRaises(ValueError):
                    self.admit({**negative, **change}, case)
            for bad_exit, error in ((0, None), (True, None), (2, None), (1, "capture incomplete")):
                with self.assertRaises(ValueError):
                    self.admit(negative, case, bad_exit, error)
            for change in ({"error": "later failure"}, {"stages": []}, {"unmanaged_project_owned_enablement_qualified": False}):
                with self.assertRaises(ValueError):
                    self.admit({**positive, **change}, case, 0)
        negative = self.evidence("grok-noauth")
        for change in ({"grok_native_tools_call_qualified": True}, {"actual_model_used": True}, {"scripted_local_provider": 1},
                       {"github_run_id": "other"}, {"github_run_attempt": "2"}, {"github_event": "push"}, {"platform": "other"}):
            with self.assertRaises(ValueError):
                self.admit({**negative, **change})

    def test_dual_original_streams_require_exact_bytes_digests_types_names_and_unique_labels(self):
        row = dict(probe="inert-version", argv=["/inert/grok", "--version"], cwd="/inert/project", error=None,
                   exit=0, elapsed_seconds=0.01)
        for name, body in (("stdout", b"inert original output\n"), ("stderr", b"")):
            row.update({name: body.decode(), name + "_base64": base64.b64encode(body).decode(),
                name + "_bytes": len(body), name + "_sha256": hashlib.sha256(body).hexdigest(),
                name + "_file": "inert-version." + name + ".log"})
        value = self.evidence("grok-noauth")
        value["commands"] = [row]
        self.assertFalse(self.admit(value))
        for field, changed in (("stdout_bytes", True), ("stdout_sha256", "0" * 64), ("stdout", "changed"),
                ("stdout_file", "foreign.log"), ("elapsed_seconds", True), ("elapsed_seconds", 901),
                ("exit", True), ("argv", ["/inert", "bad\x00"]), ("cwd", "relative"), ("unknown", False)):
            with self.subTest(field=field), self.assertRaises(ValueError):
                self.admit({**value, "commands": [{**row, field: changed}]})
        with self.assertRaises(ValueError):
            self.admit({**value, "commands": [row, row]})

    def ordinary_prefix_fixture(self, case="grok-noauth"):
        fixture = ContractTests()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        before, after, response, window = fixture.ordinary_repair_fixture()
        home, source, other = (fixture.root / name for name in ("home", "source", "other"))
        for path in (home / ".grok", source, other):
            path.mkdir(parents=True)
        (home / ".grok/config.toml").write_bytes(b'[plugins]\nenabled = ["foreign"]\n')
        fixture.state_path.write_bytes(json.dumps(before).encode())
        value = dict(self.evidence(case), prepared_owned_state=before)
        def command(label, verb, selector, *, extra=()):
            success = label == "repair-no-enable-consent"
            row = dict(probe=label, argv=["/inert/agentplugins", verb, selector, "--scope", "project",
                "--project-root", str(fixture.root), "--target", "grok", "--format", "json", *extra],
                cwd=str(fixture.root), error=None, exit=0 if success else 1, elapsed_seconds=0.01)
            for name, raw in (("stdout", json.dumps(response).encode() if success else b""),
                              ("stderr", b"" if success else b"inert unsupported enablement\n")):
                row.update({name: raw.decode(), name + "_base64": base64.b64encode(raw).decode(),
                    name + "_bytes": len(raw), name + "_sha256": hashlib.sha256(raw).hexdigest(),
                    name + "_file": label + "." + name + ".log"})
            value["commands"].append(row)
            if success:
                fixture.state_path.write_bytes(json.dumps(after, indent=2).encode() + b"\n")
                return response
            raise ValueError("inert dry-run refusal")
        journey = SimpleNamespace(root=fixture.root, a=fixture.root, b=other, home=home, source=source,
            log=fixture.root / "events", evidence=value, check=lambda: None, cli=command)
        with mock.patch("grok_project_protocol_enablement.time.time_ns", side_effect=window):
            with self.assertRaisesRegex(ValueError, "inert dry-run refusal"):
                preview_enablement(journey, before["installations"][0])
        self.assertNotIn("enablement_previews", value)
        return value

    def test_service_retains_actual_partial_prefix_without_promoting_the_failed_journey(self):
        for case in self.service.CASE_FIELDS:
            value = self.ordinary_prefix_fixture(case)
            original = json.dumps(value, sort_keys=True)
            self.assertFalse(self.admit(value, case))
            self.assertEqual(json.dumps(value, sort_keys=True), original)
            self.assertEqual(value["stages"], [])
            self.assertFalse(value["passed"] or value["unmanaged_project_owned_enablement_qualified"])
            self.assertEqual(value["commands"][-1]["exit"], 1)
            with self.assertRaises(ValueError):
                self.admit(value, case, exit_code=0)
            value.pop("ordinary_repair_observation")
            self.assertFalse(self.admit(value, case))

    def test_partial_prefix_rejects_unjoined_original_bytes_state_identity_and_command(self):
        original = self.ordinary_prefix_fixture()
        changes = (("unknown", False), ("partial", 1), ("no_consent_config_unchanged", 1),
            ("mcp_witness_execution_observed", 0), ("internal_acp_zero_proved_by_this_capture", 0),
            ("before_state_sha256", "not-original"), ("after_state_sha256", "0" * 64),
            ("post_repair_state_bytes", True), ("post_repair_state_bytes", (1 << 20) + 1),
            ("post_repair_state_base64", "!"), ("post_repair_state_base64", "A" * 1398105))
        for key, changed in changes:
            value = copy.deepcopy(original)
            value["ordinary_repair_observation"][key] = changed
            with self.subTest(key=key), self.assertRaises(ValueError):
                self.admit(value)
        for fault in ("source", "receipt", "identity", "data", "pending", "unknown-state", "duplicate-json",
                      "prior-identity", "prior-source", "proof", "window", "command", "failed-repair", "unknown-prefix"):
            value = copy.deepcopy(original)
            observation = value["ordinary_repair_observation"]
            state = document(base64.b64decode(observation["post_repair_state_base64"]))
            installation = state["installations"][0]
            binding = installation["clients"]["binding"]
            if fault == "source": installation["source"]["tree_digest"] = "sha256:" + "f" * 64
            if fault == "receipt": binding["receipts"][-1]["after_digest"] = "sha256:" + "f" * 64
            if fault == "identity": installation["installation_id"] = "foreign"
            if fault == "data": installation["data_receipts"] = {}
            if fault == "pending": binding["native_activation_attempt"] = "foreign"
            if fault == "unknown-state": state["unrecorded"] = True
            raw = json.dumps(state).encode()
            if fault == "duplicate-json": raw = b'{"schema_version":5,"schema_version":5}'
            observation.update(post_repair_state_base64=base64.b64encode(raw).decode(),
                post_repair_state_bytes=len(raw), after_state_sha256=hashlib.sha256(raw).hexdigest())
            if fault == "prior-identity": value["prepared_owned_state"]["installations"][0]["installation_id"] = "foreign"
            if fault == "prior-source": value["prepared_owned_state"]["installations"][0]["source"]["tree_digest"] = "sha256:" + "f" * 64
            if fault == "proof": observation["no_consent_exact_directory_repair"]["receipt"]["sequence"] = 3
            if fault == "window": observation["no_consent_exact_directory_repair"]["finished_at_unix_ns"] = 1
            if fault == "command": value["commands"][0]["argv"].append("--confirm-plugin-enable")
            if fault == "failed-repair": value["commands"][0]["exit"] = 1
            if fault == "unknown-prefix": observation["no_consent_exact_directory_repair"]["unknown"] = True
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                self.admit(value)

    def test_held_file_replacement_and_moved_outer_ancestor_refuse_before_read(self):
        parent = self.root / "outer" / "inner"
        parent.mkdir(parents=True)
        path = parent / "source"
        path.write_bytes(b"inert exact source")
        path.chmod(0o600)
        self.inputs.pin(path, ((os.getuid(), os.getgid()),), 1024)
        held = self.inputs.files[path][0]
        position = os.lseek(held, 0, os.SEEK_CUR)
        (self.root / "outer").rename(self.root / "moved")
        (self.root / "outer").mkdir()
        (self.root / "moved" / "inner").rename(self.root / "outer" / "inner")
        with self.assertRaisesRegex(ValueError, "ancestry changed"):
            self.inputs.read(path)
        self.assertEqual(os.lseek(held, 0, os.SEEK_CUR), position)
        self.assertEqual(path.read_bytes(), b"inert exact source")

    def publication_fixture(self):
        output = self.root / "output"
        output.mkdir(mode=0o700)
        self.inputs.directory(output)
        sink = self.root / "github-output"
        sink.write_bytes(b"")
        sink.chmod(0o600)
        descriptor = os.open(sink, os.O_RDWR | os.O_APPEND)
        self.addCleanup(os.close, descriptor)
        identity = self.service.file_identity(os.fstat(descriptor))
        body = json.dumps(self.evidence("grok-noauth")).encode()
        receipt = {"delegated_service": {"closed": True, "forced_stop": False, "forced_kill": False},
                   "native_passed": False, "evidence_sha256": hashlib.sha256(body).hexdigest()}
        return output, sink, body, receipt, (descriptor, identity, b"")

    def test_only_clean_closure_publishes_exact_original_negative_bytes_and_literal_grant(self):
        output, sink, body, receipt, held = self.publication_fixture()
        with mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(sink)}):
            self.service.publish(output, body, receipt, self.inputs, held)
        self.assertEqual((output / "upload/evidence.json").read_bytes(), body)
        self.assertEqual(json.loads((output / "upload/delegation.json").read_bytes()), receipt)
        self.assertEqual(sink.read_bytes(), b"native_passed=false\npublication=admitted\n")
        self.assertFalse(json.loads(body)["passed"])

    def test_unclean_service_and_expired_original_budget_create_no_upload_or_grant(self):
        output, sink, body, receipt, held = self.publication_fixture()
        with mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(sink)}):
            for key, changed in (("closed", False), ("closed", 1), ("forced_stop", True), ("forced_kill", True)):
                bad = copy.deepcopy(receipt)
                bad["delegated_service"][key] = changed
                with self.assertRaises(ValueError):
                    self.service.publish(output, body, bad, self.inputs, held)
                self.assertFalse((output / "upload").exists())
            self.budget.deadline = 0
            with self.assertRaises(Exception):
                self.service.publish(output, body, receipt, self.inputs, held)
        self.assertFalse((output / "upload").exists())
        self.assertEqual(sink.read_bytes(), b"")

    def test_output_ancestor_replacement_and_sink_replacement_preserve_foreign_members_without_grant(self):
        output, sink, body, receipt, held = self.publication_fixture()
        moved = self.root / "moved-output"
        output.rename(moved)
        output.mkdir(mode=0o700)
        (output / "foreign").write_bytes(b"must remain")
        with mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(sink)}), self.assertRaises(ValueError):
            self.service.publish(output, body, receipt, self.inputs, held)
        self.assertEqual((output / "foreign").read_bytes(), b"must remain")
        self.assertFalse((output / "upload").exists())
        self.assertEqual(sink.read_bytes(), b"")

    def test_short_or_expired_workflow_grant_restores_only_its_own_exact_tail(self):
        _, sink, _, _, held = self.publication_fixture()
        original_write = os.write
        def short(descriptor, body):
            return original_write(descriptor, body[:-1])
        with mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(sink)}), mock.patch.object(os, "write", side_effect=short):
            with self.assertRaisesRegex(ValueError, "incomplete"):
                self.service.workflow_output(*held, self.inputs, False)
        self.assertEqual(sink.read_bytes(), b"")
        held = (held[0], self.service.file_identity(os.fstat(held[0])), b"")
        def expire(descriptor, body):
            count = original_write(descriptor, body)
            self.budget.deadline = 0
            return count
        with mock.patch.dict(os.environ, {"GITHUB_OUTPUT": str(sink)}), mock.patch.object(os, "write", side_effect=expire):
            with self.assertRaises(Exception):
                self.service.workflow_output(*held, self.inputs, False)
        self.assertEqual(sink.read_bytes(), b"")

    def test_new_launcher_does_not_forward_parent_sink_secrets_or_loader_controls(self):
        ambient = {**self.env, "GITHUB_OUTPUT": "/private/output", "XAI_API_KEY": "secret", "PYTHONPATH": "/foreign", "LD_PRELOAD": "foreign"}
        with mock.patch.dict(os.environ, ambient):
            env = self.service.child_environment(self.root, "a" * 40)
        self.assertEqual(set(env), self.service.runtime.GROK_ENV)
        self.assertTrue(all(key not in env for key in ("GITHUB_OUTPUT", "XAI_API_KEY", "PYTHONPATH", "LD_PRELOAD")))

    def test_actual_parent_keeps_child_private_until_clean_completion_and_returns_original_negative(self):
        import io
        for scenario in ("clean-negative", "unclean", "transport", "changed-source", "changed-input", "sink-replaced",
                         "sink-after-grant", "sink-same-size", "foreign-tail", "child-extra", "late-budget", "late-close"):
            with self.subTest(scenario=scenario), tempfile.TemporaryDirectory(dir=self.root) as temporary:
                root = Path(temporary).resolve()
                tools = root / "tools"
                tools.mkdir()
                installer, probe = tools / "agentplugins", tools / "probe"
                for path in (installer, probe):
                    path.write_bytes(b"inert never executed " + path.name.encode())
                    path.chmod(0o700)
                sink = root / "github-output"
                sink.write_bytes(b"")
                sink.chmod(0o600)
                output = root / "output"
                source_root = root / "source"
                script = source_root / "scripts/grok_noauth_call_e2e.py"
                script.parent.mkdir(parents=True)
                script.write_bytes(b"inert owned script; never executed")
                script.chmod(0o600)
                evidence = self.evidence("grok-noauth")
                evidence.update(installer_sha256=hashlib.sha256(installer.read_bytes()).hexdigest(),
                                probe_sha256=hashlib.sha256(probe.read_bytes()).hexdigest())
                body = json.dumps(evidence).encode()
                def service(ctx, case, selected, env, destination, origin):
                    self.assertEqual((case, selected[1:], destination), ("grok-noauth", (installer, probe), output))
                    self.assertEqual(ctx.budget.deadline, origin / 1e9 + 900)
                    self.assertEqual(sink.read_bytes(), b"")
                    self.assertFalse((output / "upload").exists())
                    self.assertNotIn("GITHUB_OUTPUT", env)
                    (output / "child").mkdir(mode=0o700)
                    (output / "child/evidence.json").write_bytes(body)
                    (output / "child/evidence.json").chmod(0o600)
                    if scenario == "child-extra":
                        (output / "child/foreign.txt").write_bytes(b"unknown child member")
                    if scenario == "changed-input":
                        installer.write_bytes(b"changed inert input")
                    if scenario == "sink-replaced":
                        sink.rename(root / "original-sink")
                        sink.write_bytes(b"foreign bytes must stay")
                    if scenario == "transport":
                        raise ValueError("inert secret must not reach console")
                    ctx.summary["delegated_service"] = dict(closed=scenario != "unclean", forced_stop=False, forced_kill=False)
                    return dict(exit=1, error=None), b"", b""
                def source(_ctx, phase):
                    return dict(source_commit=("e" if scenario == "changed-source" and phase == "after" else "a") * 40,
                                source_tree="b" * 40)
                env = {**self.env, "GITHUB_OUTPUT": str(sink), "EXPECTED_COMMIT": "a" * 40,
                       "GITHUB_ACTIONS": "true", "RUNNER_ENVIRONMENT": "github-hosted",
                       "AGENTPLUGINS_NATIVE_DISPOSABLE_HOSTED": "1"}
                original_pin = self.service.Inputs.pin
                original_close = self.service.Inputs.close
                original_exit = self.service.shared.Budget.__exit__
                original_write = os.write
                python = Path("/usr/bin/python3").resolve()
                def pin(owner, path, owners, maximum, **flags):
                    if path == python:
                        # This host's interpreter is not a hosted-runner proof.
                        # The native launch is inert; only its fixed pin request
                        # is admitted at this outer-control-flow test boundary.
                        self.assertEqual((owners, maximum, flags), (((0, 0),), 64 << 20, {"executable": True}))
                        return "e" * 64
                    return original_pin(owner, path, owners, maximum, **flags)
                def write(descriptor, content):
                    count = original_write(descriptor, content)
                    if scenario == "sink-after-grant":
                        sink.rename(root / "granted-original")
                        sink.write_bytes(b"native_passed=true\npublication=admitted\nforeign=preserve\n")
                    elif scenario == "foreign-tail":
                        original_write(descriptor, b"foreign=preserve\n")
                    elif scenario == "sink-same-size":
                        current = sink.read_bytes()
                        sink.write_bytes(b"X" + current[1:])
                    return count
                def exit_budget(budget, *args):
                    result = original_exit(budget, *args)
                    if scenario == "late-budget":
                        self.assertIn(b"publication=admitted", sink.read_bytes())
                        raise self.service.shared.legacy.CollectorDeadline("inert late deadline")
                    return result
                def close(owner):
                    original_close(owner)
                    if scenario == "late-close":
                        self.assertIn(b"publication=admitted", sink.read_bytes())
                        raise OSError("inert late close refusal")
                with mock.patch.dict(os.environ, env), mock.patch.object(self.service.runtime, "cline_host"), \
                        mock.patch.object(self.service.shared.native, "require_hosted"), \
                        mock.patch.object(self.service.runtime, "SOURCE", source_root), \
                        mock.patch.object(self.service.Inputs, "pin", pin), \
                        mock.patch.object(self.service.Inputs, "close", close), \
                        mock.patch.object(self.service.shared.Budget, "__exit__", exit_budget), \
                        mock.patch.object(os, "write", side_effect=write), \
                        mock.patch.object(self.service, "source_identity", side_effect=source), \
                        mock.patch.object(self.service.runtime, "grok_service", side_effect=service), \
                        mock.patch.object(sys, "executable", "/usr/bin/python3"), \
                        mock.patch.object(sys, "stdout", new_callable=io.StringIO) as console, \
                        mock.patch.object(self.service, "admit_evidence", wraps=self.service.admit_evidence) as admitted:
                    result = self.service.run("grok-noauth", installer, probe, output)
                self.assertNotIn("secret", console.getvalue())
                if scenario == "clean-negative":
                    self.assertEqual(result, 0)  # transport/admission, not native success
                    self.assertEqual((output / "upload/evidence.json").read_bytes(), body)
                    self.assertEqual(sink.read_bytes(), b"native_passed=false\npublication=admitted\n")
                    self.assertEqual(admitted.call_count, 1)
                else:
                    self.assertEqual(result, 1)
                    # Even a copied/foreign stale grant cannot pass the actual
                    # workflow's independent successful-launcher-outcome guard.
                    upload = result == 0 and b"publication=admitted\n" in sink.read_bytes().splitlines(keepends=True)
                    self.assertFalse(upload)
                    if scenario not in ("sink-after-grant", "sink-same-size", "foreign-tail", "late-close"):
                        self.assertNotIn(b"publication=admitted", sink.read_bytes())
                    if scenario in ("unclean", "transport", "changed-source", "changed-input", "child-extra"):
                        self.assertEqual(admitted.call_count, 0)
                    if scenario == "sink-replaced":
                        self.assertEqual(sink.read_bytes(), b"foreign bytes must stay")
                    if scenario in ("sink-after-grant", "foreign-tail"):
                        self.assertTrue(sink.read_bytes().endswith(b"foreign=preserve\n"))
                    if scenario == "sink-same-size":
                        self.assertTrue(sink.read_bytes().startswith(b"Xative_passed=false\n"))

    def test_both_actual_workflow_verdicts_keep_negative_native_result_and_stale_grants_failed(self):
        import subprocess
        workflow_root = Path(__file__).resolve().parents[1] / ".github/workflows"
        condition = "if: ${{ always() && steps.grok_native.outcome == 'success' && steps.grok_native.outputs.publication == 'admitted' }}"
        for name in ("grok-project-protocol.yml", "grok-noauth-call.yml"):
            text = (workflow_root / name).read_text()
            self.assertEqual(text.count(condition), 1)
            final = text.split("      - name: Require the original native result to pass\n")[1]
            self.assertIn("        if: ${{ always() }}\n", final)
            script = final.split("        run: |\n")[1]
            script = "\n".join(line[10:] for line in script.splitlines() if line.strip())
            self.assertEqual(script.splitlines(), ['test "$GROK_LAUNCH_OUTCOME" = success',
                'test "$GROK_PUBLICATION" = admitted', 'test "$GROK_NATIVE_PASSED" = true'])
            for outcome, publication, passed in (("success", "admitted", "true"), ("success", "admitted", "false"),
                    ("failure", "admitted", "true"), ("failure", "admitted", "false"), ("success", "", "true"),
                    ("skipped", "admitted", "true"), ("success", "admitted", "1")):
                result = subprocess.run(["/bin/bash", "--noprofile", "--norc", "-e", "-c", script],
                    env={"PATH": "/usr/bin:/bin", "GROK_LAUNCH_OUTCOME": outcome, "GROK_PUBLICATION": publication,
                         "GROK_NATIVE_PASSED": passed}, capture_output=True, timeout=2, check=False)
                self.assertEqual(result.returncode == 0, (outcome, publication, passed) == ("success", "admitted", "true"))
                self.assertEqual((result.stdout, result.stderr), (b"", b""))


if __name__ == "__main__":
    unittest.main()
