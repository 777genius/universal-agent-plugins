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
import grok_project_protocol_trace_probe as trace_case


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
        for fault in (None, "no-commit", "package", "data", "source", "project-b", "config", "foreign-home", "witness", "ordinary-command", "ordinary-command-package", "ordinary-command-state",
                      "ordinary-command-witness", "dry-command", "dry-command-package", "dry-command-state",
                      "dry-command-witness", "dry-state", "dry-config"):
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
                    self.state_path.write_bytes(before_bytes if fault in (
                        "no-commit", "ordinary-command", "ordinary-command-package",
                        "ordinary-command-witness") else after_bytes)
                    changed = {"package": self.active / "probe", "data": self.data / "foreign", "source": source / "source",
                               "project-b": other / "foreign", "config": config, "foreign-home": foreign, "witness": event}
                    if fault in changed: changed[fault].write_bytes(b"unreviewed effect")
                    if fault == "ordinary-command-package":
                        (self.active / "probe").write_bytes(b"unreviewed effect")
                    if fault == "ordinary-command-witness":
                        event.write_bytes(b"unreviewed effect")
                    if fault is not None and fault.startswith("ordinary-command"):
                        raise ValueError("command failed: repair-no-enable-consent")
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
                        "ordinary-command": "command failed: repair-no-enable-consent",
                        "ordinary-command-package": "repair changed native config, package, data, source or Project B",
                        "ordinary-command-state": "failed ordinary repair changed saved ownership",
                        "ordinary-command-witness": "repair preview executed the MCP witness",
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

    def test_original_snapshot_survives_two_uap_writer_lock_cycles(self):
        import fcntl
        self.prepare()
        (self.root / profile_case.CONFIG).write_bytes(b"[plugins]\nenabled = []\ndisabled = []\n")
        before = snapshot(self.root)
        lock = self.journey.home / ".grok/config.toml.agentplugins.lock"
        for _ in range(2):
            descriptor = os.open(lock, os.O_RDWR | os.O_CREAT | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600)
            try:
                fcntl.flock(descriptor, fcntl.LOCK_EX | fcntl.LOCK_NB)
                fcntl.flock(descriptor, fcntl.LOCK_UN)
            finally:
                os.close(descriptor)
        self.assertEqual(snapshot(self.root), before)
        self.journey.check()

    def test_uap_writer_lock_collision_is_not_adopted_or_overwritten(self):
        original = profile_case.prepare_uap_writer_lock
        saved = {}
        def collide(journey):
            path = self.root / profile_case.UAP_LOCK
            path.write_bytes(b"foreign coordination leaf")
            saved["identity"] = profile_case.stable(path.lstat())
            return original(journey)
        with mock.patch.object(profile_case, "prepare_uap_writer_lock", side_effect=collide):
            with self.assertRaises(FileExistsError):
                self.prepare()
        path = self.root / profile_case.UAP_LOCK
        self.assertEqual(path.read_bytes(), b"foreign coordination leaf")
        self.assertEqual(profile_case.stable(path.lstat()), saved["identity"])
        self.assertNotIn(profile_case.UAP_LOCK, self.journey.native_profile)
        self.assertFalse(self.journey.evidence["native_profile_preparation"]["complete"])

    def test_retained_uap_writer_lock_changes_are_refused_without_mutation(self):
        base = self.root
        for kind in ("nonempty", "mode", "hardlink", "symlink", "replacement"):
            with self.subTest(kind=kind):
                root = base / kind
                root.mkdir()
                self.start_journey(root)
                self.prepare()
                lock = root / profile_case.UAP_LOCK
                if kind == "nonempty": lock.write_bytes(b"foreign")
                elif kind == "mode": lock.chmod(0o644)
                elif kind == "hardlink": os.link(lock, root / "foreign-link")
                elif kind == "symlink":
                    lock.unlink()
                    lock.symlink_to(self.journey.grok)
                else:
                    lock.rename(root / "original-uap-lock")
                    lock.touch(mode=0o600)
                before = profile_case.stable(lock.lstat())
                body = os.readlink(lock) if lock.is_symlink() else lock.read_bytes()
                with self.assertRaises((ValueError, OSError)):
                    self.journey.check()
                self.assertEqual(profile_case.stable(lock.lstat()), before)
                self.assertEqual(os.readlink(lock) if lock.is_symlink() else lock.read_bytes(), body)

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
            value.update(model_invoked=False, grok_tools_call_qualified=False, transport="stdio", inventory_test_sha256="e" * 64)
            if passed:
                value.update(native_doctor_initialize_and_list_qualified=True, direct_installed_config_tools_call_qualified=True)
                self.readonly_proof(value)
        else:
            value.update(github_run_id="123", github_run_attempt="1", github_event="pull_request", platform="Linux-x86_64",
                actual_model_used=False, scripted_local_provider=True, grok_native_tools_call_qualified=passed)
        return value

    def admit(self, value, case="grok-noauth", exit_code=1, error=None):
        expected = {**self.expected, **({"inventory_test_sha256": "e" * 64} if case == "grok-protocol" else {})}
        return self.service.admit_evidence(json.dumps(value).encode(), case, {"exit": exit_code, "error": error},
                                          expected, self.env, self.budget,
                                          inventory_test=self.root / "tools/grok-inventory-test")

    def readonly_proof(self, value):
        project = self.root / "project-a"
        active = project / ".grok/plugins/owned"
        value["prepared_owned_state"] = {"installations": [{"declared_name": "fixture", "package": {"version": "1.0.0"},
            "clients": {"binding": {"client_id": "grok", "scope": "project", "target_locator": str(active)}}}]}
        value["readonly_inventory"] = {"calls": 2, "input_snapshot_sha256": "f" * 64}
        row = dict(probe="readonly-production-inventory", argv=[str(self.root / "tools/grok-inventory-test"),
            "-test.run=^TestEnablementReadonlyNativeInventory$", "-test.v", "-test.count=1", "-test.timeout=45s",
            "-grok-readonly-home=" + str(self.root / "home"), "-grok-readonly-project=" + str(project),
            "-grok-readonly-native=" + str(self.root / "bin/grok"), "-grok-readonly-path=" + str(active),
            "-grok-readonly-name=fixture", "-grok-readonly-version=1.0.0"], cwd=str(project), error=None,
            exit=0, elapsed_seconds=0.01)
        stdout = b"=== RUN   TestEnablementReadonlyNativeInventory\nGROK_READONLY_NATIVE_INVENTORY_PASS\n--- PASS: TestEnablementReadonlyNativeInventory (0.01s)\nPASS\n"
        for name, body in (("stdout", stdout), ("stderr", b"")):
            row.update({name: body.decode(), name + "_base64": base64.b64encode(body).decode(),
                name + "_bytes": len(body), name + "_sha256": hashlib.sha256(body).hexdigest(),
                name + "_file": row["probe"] + "." + name + ".log"})
        value["commands"].append(row)

    def test_literal_case_flags_source_exit_and_stage_prefix_are_joined(self):
        for case in self.service.CASE_FIELDS:
            negative = self.evidence(case)
            self.assertFalse(self.admit(negative, case))
            positive = self.evidence(case, True)
            self.assertTrue(self.admit(positive, case, 0))
            for change in ({"passed": True}, {"passed": 0}, {"vendor_authentication_used": 0},
                    {"source_commit": "e" * 40}, {"installer_sha256": "e" * 64}, {"source_tree": "e" * 40},
                    {"probe_sha256": "e" * 64}, {"inventory_test_sha256": "f" * 64}, {"unknown": False}, {"unmanaged_project_owned_enablement_qualified": True},
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

    def test_partial_readonly_proof_requires_executed_source_pinned_selected_test(self):
        value = self.evidence("grok-protocol")
        self.readonly_proof(value)
        self.assertFalse(self.admit(value, "grok-protocol"))
        for fault in ("calls", "snapshot", "missing-command", "different-test", "relative-test", "different-native", "different-package", "failed-command", "no-test"):
            changed = copy.deepcopy(value)
            if fault == "calls": changed["readonly_inventory"]["calls"] = True
            elif fault == "snapshot": changed["readonly_inventory"]["input_snapshot_sha256"] = "unknown"
            elif fault == "missing-command": changed["commands"] = []
            elif fault == "different-test": changed["commands"][0]["argv"][0] = "/foreign/grok-inventory-test"
            elif fault == "relative-test": changed["commands"][0]["argv"][0] = "grok-inventory-test"
            elif fault == "different-native": changed["commands"][0]["argv"][7] = "-grok-readonly-native=/foreign/grok"
            elif fault == "different-package": changed["commands"][0]["argv"][8] = "-grok-readonly-path=/foreign/plugin"
            elif fault == "failed-command": changed["commands"][0]["exit"] = 1
            else:
                row, body = changed["commands"][0], b"testing: warning: no tests to run\nPASS\n"
                row.update(stdout=body.decode(), stdout_base64=base64.b64encode(body).decode(),
                           stdout_bytes=len(body), stdout_sha256=hashlib.sha256(body).hexdigest())
            with self.subTest(fault=fault), self.assertRaises(ValueError):
                self.admit(changed, "grok-protocol")

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

    def test_runtime_root_requires_immutable_owner_and_parent_python_remains_selected(self):
        runtime_root = self.root / "runtime"
        runtime_root.mkdir(mode=0o777)
        runtime_root.chmod(0o777)
        with self.assertRaisesRegex(ValueError, "root-owned immutable"):
            self.inputs.runtime_directory(runtime_root)
        alias = self.root / "runtime-alias"
        alias.symlink_to(runtime_root, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "canonical"):
            self.service.runtime_interpreters(self.inputs, alias)
        owner = SimpleNamespace(runtime_directory=mock.Mock(return_value=(1, 2, 3, 0, 0)), pin=mock.Mock())
        foreign = self.root / "foreign-python"
        foreign.write_bytes(b"different inert parent interpreter")
        with mock.patch.object(sys, "executable", str(foreign)):
            with self.assertRaisesRegex(ValueError, "parent interpreter"):
                self.service.runtime_interpreters(owner, runtime_root)
        owner.pin.assert_not_called()

    def test_actual_parent_keeps_child_private_until_clean_completion_and_returns_original_negative(self):
        import io
        for scenario in ("clean-negative", "unclean", "transport", "changed-source", "changed-input", "sink-replaced",
                         "sink-after-grant", "sink-same-size", "foreign-tail", "child-extra", "late-budget", "late-close",
                         "changed-child-interpreter", "changed-runtime-root"):
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
                clean_root = root / "runtime"
                child_python = clean_root / "usr/bin/python3.12"
                child_python.parent.mkdir(parents=True)
                child_body = b"different inert Noble interpreter; never executed"
                child_python.write_bytes(child_body)
                child_python.chmod(0o700)
                evidence = self.evidence("grok-noauth")
                evidence.update(installer_sha256=hashlib.sha256(installer.read_bytes()).hexdigest(),
                                probe_sha256=hashlib.sha256(probe.read_bytes()).hexdigest())
                body = json.dumps(evidence).encode()
                def service(ctx, case, selected, env, destination, origin, *, runtime_root):
                    self.assertEqual((case, selected[1:], destination), ("grok-noauth", (installer, probe), output))
                    self.assertEqual((selected[0], runtime_root), (self.service.runtime.GROK_CHILD_PYTHON, clean_root))
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
                    if scenario == "changed-child-interpreter":
                        child_python.rename(child_python.with_name("original-python"))
                        child_python.write_bytes(child_body)
                        child_python.chmod(0o700)
                    if scenario == "changed-runtime-root":
                        clean_root.rename(root / "original-runtime")
                        clean_root.mkdir()
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
                def runtime_directory(owner, path):
                    # Retain real fixture directory authority; this ordinary-user
                    # control does not qualify the root-owned Noble provision.
                    self.assertEqual(path, clean_root)
                    return self.service.directory_identity(os.fstat(owner.directory(path)))
                def pin(owner, path, owners, maximum, **flags):
                    if path == python:
                        # This host's interpreter is not a hosted-runner proof.
                        # The native launch is inert; only its fixed pin request
                        # is admitted at this outer-control-flow test boundary.
                        self.assertEqual((owners, maximum, flags), (((0, 0),), 64 << 20, {"executable": True}))
                        return "e" * 64
                    if path == child_python:
                        self.assertEqual((owners, maximum, flags), (((0, 0),), 64 << 20, {"executable": True}))
                        return original_pin(owner, path, ((os.getuid(), os.getgid()),), maximum, **flags)
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
                        mock.patch.object(self.service.Inputs, "runtime_directory", runtime_directory), \
                        mock.patch.object(self.service.Inputs, "pin", pin), \
                        mock.patch.object(self.service.Inputs, "close", close), \
                        mock.patch.object(self.service.shared.Budget, "__exit__", exit_budget), \
                        mock.patch.object(os, "write", side_effect=write), \
                        mock.patch.object(self.service, "source_identity", side_effect=source), \
                        mock.patch.object(self.service.runtime, "grok_service", side_effect=service), \
                        mock.patch.object(sys, "executable", "/usr/bin/python3"), \
                        mock.patch.object(sys, "stdout", new_callable=io.StringIO) as console, \
                        mock.patch.object(self.service, "admit_evidence", wraps=self.service.admit_evidence) as admitted:
                    result = self.service.run("grok-noauth", installer, probe, output, runtime_root=clean_root)
                self.assertNotIn("secret", console.getvalue())
                if scenario == "clean-negative":
                    self.assertEqual(result, 0)  # transport/admission, not native success
                    self.assertEqual((output / "upload/evidence.json").read_bytes(), body)
                    self.assertEqual(sink.read_bytes(), b"native_passed=false\npublication=admitted\n")
                    receipt = json.loads((output / "upload/delegation.json").read_bytes())
                    self.assertEqual(receipt["parent_python_sha256"], "e" * 64)
                    self.assertEqual(receipt["python_sha256"], hashlib.sha256(child_body).hexdigest())
                    self.assertNotEqual(receipt["python_sha256"], receipt["parent_python_sha256"])
                    self.assertEqual(receipt["runtime_root"], str(clean_root))
                    self.assertEqual(admitted.call_count, 1)
                else:
                    self.assertEqual(result, 1)
                    # Even a copied/foreign stale grant cannot pass the actual
                    # workflow's independent successful-launcher-outcome guard.
                    upload = result == 0 and b"publication=admitted\n" in sink.read_bytes().splitlines(keepends=True)
                    self.assertFalse(upload)
                    if scenario not in ("sink-after-grant", "sink-same-size", "foreign-tail", "late-close"):
                        self.assertNotIn(b"publication=admitted", sink.read_bytes())
                    if scenario in ("unclean", "transport", "changed-source", "changed-input", "child-extra",
                                    "changed-child-interpreter", "changed-runtime-root"):
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
            self.assertEqual(text.count("run: bash scripts/provision_grok_native_runtime.sh"), 1)
            self.assertEqual(text.count('--runtime-root "$RUNNER_TEMP/grok-runtime-root"'), 1)
            self.assertLess(text.index("id: grok_runtime_setup"), text.index("id: grok_native"))
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


@unittest.skipUnless(sys.platform == "linux" and hasattr(os, "WNOWAIT"), "Linux nonreaping child observation required")
class InertTraceMechanicsContracts(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.fixture = self.root / "fixture"
        self.fixture.mkdir(mode=0o700)
        (self.fixture / "original.txt").write_bytes(b"inert original input\n")
        (self.fixture / "original.txt").chmod(0o600)
        self.env = {"HOME": str(self.fixture), "PATH": "/usr/bin:/bin", "LANG": "C", "LC_ALL": "C"}

    def collect_inert(self, code, *, seconds=3.0, events=None):
        import time
        original_popen = trace_case.subprocess.Popen
        original_signal, original_waitid = os.killpg, os.waitid
        observed = [] if events is None else events

        def spawn(argv, **kwargs):
            self.assertEqual(argv[:6], ["/usr/bin/strace", "-f", "--kill-on-exit", "--decode-pids=pidns",
                                       "-e", "trace=%process,%network"])
            channel = argv[argv.index("-o") + 1]
            # Actual inert child and original pipes; no tracer/native success claim.
            child = original_popen([sys.executable, "-I", "-B", "-c", code, channel], **kwargs)
            original_wait = child.wait

            def wait(*args, **kwargs):
                observed.append(("wait", child.pid))
                return original_wait(*args, **kwargs)

            child.wait = wait
            return child

        def signal_group(pid, number):
            self.assertFalse(any(event[0] == "wait" for event in observed))
            status = original_waitid(os.P_PID, pid, os.WEXITED | os.WNOHANG | os.WNOWAIT)
            observed.append(("signal", pid, status.si_pid if status else None))
            return original_signal(pid, number)

        with mock.patch.object(trace_case.subprocess, "Popen", side_effect=spawn), \
                mock.patch.object(trace_case.os, "killpg", side_effect=signal_group):
            row = trace_case.collect("contract", ["/usr/bin/true"], self.root, self.fixture, self.env,
                                     time.monotonic() + seconds, trace=True)
        self.assertEqual(sum(event[0] == "wait" for event in observed), 1)
        self.assertEqual(observed[-1], ("wait", row["root_pid"]))
        self.assertFalse((self.root / "contract.fifo").exists())
        return row

    def test_each_live_channel_refuses_actual_inert_overrun(self):
        for name in ("stdout", "stderr", "trace"):
            with self.subTest(stream=name):
                descriptor = {"stdout": "1", "stderr": "2", "trace": "channel"}[name]
                code = "import os,sys\nchannel=os.open(sys.argv[1],os.O_WRONLY)\n"
                code += f"body=b'x'*{trace_case.LIMITS[name] + 1}\n"
                code += f"while body:\n body=body[os.write({descriptor},body):]\n"
                row = self.collect_inert(code)
                self.assertEqual(row["error"], name + "-limit")
                self.assertEqual(len(row["streams"][name]["body"]), trace_case.LIMITS[name])
                self.assertTrue(row["wait_observed"])
                self.assertTrue(row["group_signal_sent"])
                self.assertTrue(all(item["eof"] for item in row["streams"].values()))

    def test_exited_original_leader_keeps_group_reserved_until_pipe_child_is_stopped(self):
        import time
        events = []
        code = """import os,sys,time
channel=os.open(sys.argv[1],os.O_WRONLY)
pid=os.fork()
if pid == 0:
    time.sleep(2)
    os._exit(0)
os.write(channel,b'original-leader-exiting\\n')
os._exit(0)
"""
        started = time.monotonic()
        row = self.collect_inert(code, seconds=1.6, events=events)
        self.assertEqual(row["error"], "original-deadline")
        self.assertTrue(row["unreaped_exit_observed"])
        self.assertTrue(row["wait_observed"])
        self.assertEqual(row["exit"], 0)
        self.assertTrue(all(item["eof"] for item in row["streams"].values()))
        self.assertEqual(events, [("signal", row["root_pid"], row["root_pid"]), ("wait", row["root_pid"])])
        self.assertLess(time.monotonic() - started, 1.6)
        with self.assertRaises(ChildProcessError):
            os.waitid(os.P_PID, row["root_pid"], os.WEXITED | os.WNOHANG | os.WNOWAIT)

    def test_expired_original_budget_opens_no_channel_and_starts_no_process(self):
        import time
        with mock.patch.object(trace_case.subprocess, "Popen") as start, mock.patch.object(trace_case.os, "mkfifo") as fifo:
            with self.assertRaisesRegex(RuntimeError, "original-deadline"):
                trace_case.collect("expired", ["/usr/bin/true"], self.root, self.fixture, self.env,
                                   time.monotonic() - 1.0, trace=True)
        start.assert_not_called()
        fifo.assert_not_called()

    def test_untraced_control_reuses_original_capture_without_a_trace_channel(self):
        import time
        argv = [sys.executable, "-I", "-B", "-c", "import os; os.write(1,b'STEP0_READONLY_EROFS\\n')"]
        original_popen, original_signal = trace_case.subprocess.Popen, os.killpg
        waits = []

        def spawn(command, **kwargs):
            self.assertEqual(command, argv)
            self.assertEqual(kwargs["env"], self.env)
            self.assertEqual(kwargs["cwd"], self.fixture)
            self.assertTrue(kwargs["close_fds"])
            self.assertTrue(kwargs["start_new_session"])
            self.assertNotIn("pass_fds", kwargs)
            child = original_popen(command, **kwargs)
            original_wait = child.wait

            def wait(*args, **kwargs):
                waits.append(child.pid)
                return original_wait(*args, **kwargs)

            child.wait = wait
            return child

        receipt = dict(runs=[], complete_process_family_admitted=False, native_qualification=False)
        before = trace_case.fixture_snapshot(self.fixture)
        with mock.patch.object(trace_case.subprocess, "Popen", side_effect=spawn), \
                mock.patch.object(trace_case.os, "mkfifo") as channel, \
                mock.patch.object(trace_case.os, "killpg", wraps=original_signal) as signal_group:
            trace_case.untraced_readonly_control(receipt, argv, self.root, self.fixture, self.env,
                                                time.monotonic() + 3, before)
        row = receipt["runs"][0]
        self.assertEqual(waits, [row["root_pid"]])
        self.assertEqual(set(row["streams"]), {"stdout", "stderr"})
        self.assertTrue(row["wait_observed"])
        self.assertTrue(all(stream["eof"] for stream in row["streams"].values()))
        self.assertEqual(receipt["readonly_untraced_fixture_after"], before)
        self.assertNotIn("process_observation", row)
        self.assertFalse(receipt["complete_process_family_admitted"])
        self.assertFalse(receipt["native_qualification"])
        for name, body in (("stdout", b"STEP0_READONLY_EROFS\n"), ("stderr", b"")):
            stream = row["streams"][name]
            self.assertEqual(base64.b64decode(stream["base64"], validate=True), body)
            self.assertEqual(stream["bytes"], len(body))
            self.assertEqual(stream["sha256"], hashlib.sha256(body).hexdigest())
        channel.assert_not_called()
        signal_group.assert_not_called()

    def test_unexpected_original_reap_cannot_become_a_successful_wait(self):
        import time
        original_observe = trace_case.observe_unreaped
        reaped = []

        def observe(child):
            result = original_observe(child)
            if result and not reaped:
                reaped.append(os.waitpid(child.pid, 0))
            return result

        with mock.patch.object(trace_case, "observe_unreaped", side_effect=observe), \
                mock.patch.object(trace_case.os, "killpg") as signal_group:
            row = trace_case.collect("reaped", [sys.executable, "-I", "-B", "-c", "pass"],
                                     self.root, self.fixture, self.env, time.monotonic() + 3.0)
        self.assertEqual(len(reaped), 1)
        self.assertEqual(row["error"], "original-wait-authority-lost")
        self.assertFalse(row["wait_observed"])
        self.assertFalse(row["wait_called"])
        self.assertIsNone(row["exit"])
        signal_group.assert_not_called()

    def test_bwrap_loopback_denial_is_only_a_specific_readonly_refusal(self):
        message = b'bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted' + bytes([10])
        row = {'exit': 1, 'error': None, 'wait_observed': True, 'streams': {
            'stdout': {'body': bytearray(), 'eof': True},
            'stderr': {'body': bytearray(message), 'eof': True},
            'trace': {'body': bytearray(b'original process trace'), 'eof': True}}}
        self.assertEqual(trace_case.readonly_failure_reason(row), 'readonly-bwrap-loopback-permission')
        for key, value in (('exit', 0), ('exit', 2), ('error', 'capture-incomplete'),
                           ('wait_observed', False)):
            altered = copy.deepcopy(row)
            altered[key] = value
            with self.subTest(field=key, value=value):
                self.assertEqual(trace_case.readonly_failure_reason(altered), 'readonly-tracer-refused')
        for stream, key, value in (('stderr', 'body', bytearray(message + b'extra')),
                                   ('stderr', 'body', bytearray()),
                                   ('stdout', 'body', bytearray(b'unexpected')),
                                   ('trace', 'eof', False)):
            altered = copy.deepcopy(row)
            altered['streams'][stream][key] = value
            with self.subTest(stream=stream, field=key):
                self.assertEqual(trace_case.readonly_failure_reason(altered), 'readonly-tracer-refused')

    def test_first_control_refusal_does_not_attempt_the_readonly_control(self):
        import time

        def capture(label, *args, **kwargs):
            streams = {name: dict(body=bytearray(), eof=True) for name in ("stdout", "stderr")}
            if label == "direct":
                streams["trace"] = dict(body=bytearray(), eof=True)
            return dict(label=label, exit=1 if label == "direct" else 0, error=None,
                        wait_observed=True, streams=streams)

        receipt = dict(versions=[], runs=[])
        with mock.patch.object(trace_case, "collect", side_effect=capture) as calls, \
                mock.patch.object(trace_case, "readonly_arguments") as readonly:
            with self.assertRaisesRegex(RuntimeError, "direct-tracer-refused"):
                trace_case.controls(receipt, self.root, self.fixture, self.env, time.monotonic() + 3.0)
        self.assertEqual([row.args[0] for row in calls.call_args_list], ["strace-version", "bwrap-version", "direct"])
        readonly.assert_not_called()
        self.assertEqual(len(receipt["runs"]), 1)
        self.assertEqual(set(path.name for path in self.root.iterdir()),
                         {"fixture", "direct.stdout", "direct.stderr", "direct.trace"})

    def test_unknown_capability_result_or_actual_tool_replacement_refuses(self):
        import errno
        import time
        tool = self.root / "inert-tool"
        tool.write_bytes(b"original executable fixture")
        tool.chmod(0o700)
        with ExitStack() as stack, mock.patch.object(trace_case, "TOOLS", (str(tool),)):
            held = trace_case.hold_tools(stack, time.monotonic() + 3.0)
            facts = trace_case.tool_facts(held, time.monotonic() + 3.0)
            self.assertEqual(facts[str(tool)]["capabilities"], "absent")
            with mock.patch.object(trace_case.os, "getxattr", side_effect=OSError(errno.EACCES, "inert refusal")):
                with self.assertRaisesRegex(RuntimeError, "capability-unproved"):
                    trace_case.tool_facts(held, time.monotonic() + 3.0)
            original = tool.read_bytes()
            tool.rename(self.root / "original-held-tool")
            tool.write_bytes(original)
            tool.chmod(0o700)
            with self.assertRaisesRegex(RuntimeError, "tool-original-changed"):
                trace_case.tool_facts(held, time.monotonic() + 3.0)

    def test_fixed_child_source_and_existing_native_workflow_boundary_are_preserved(self):
        import ast
        self.assertEqual(hashlib.sha256(trace_case.CHILD_CODE.encode()).hexdigest(),
                         "987a1e88b975f58eda8351b955431002aeb910cd11dd5c6368a4dfd1e3fbf308")
        markers = {node.value for node in ast.walk(ast.parse(trace_case.CHILD_CODE))
                   if isinstance(node, ast.Constant) and isinstance(node.value, bytes)}
        self.assertEqual(markers, {b"STEP0_DIRECT_DONE\n", b"STEP0_READONLY_EROFS\n"})
        workflow = (Path(__file__).resolve().parents[1] / ".github/workflows/grok-project-protocol.yml").read_text()
        ordered = ["Run real MCP without provider", "Preserve original bounded protocol evidence",
                   "Provision the inert tracer after original native evidence", "Measure bounded inert tracer mechanics",
                   "Preserve bounded inert tracer evidence separately", "Require the original native result to pass"]
        self.assertEqual([workflow.index(name) for name in ordered], sorted(workflow.index(name) for name in ordered))
        setup = workflow.split("      - name: Provision the inert tracer", 1)[1].split("      - name: Measure bounded", 1)[0]
        self.assertIn("steps.grok_native_upload.outcome == 'success'", setup)
        driver = workflow.split("      - name: Measure bounded", 1)[1].split("      - name: Preserve bounded inert", 1)[0]
        self.assertIn("steps.grok_tracer_setup.outcome == 'success'", driver)
        self.assertIn('exit "$grok_tracer_status"', driver)


class InertProcessTraceContracts(unittest.TestCase):
    @staticmethod
    def python_argv(fixture, mode="direct"):
        target = "direct-owned" if mode == "direct" else "read-only-attempt"
        return ["/usr/bin/python3", "-I", "-B", "-c", trace_case.CHILD_CODE, str(fixture / target), mode]

    @staticmethod
    def exec_line(pid, argv, environment):
        return f"{pid} execve({json.dumps(argv[0])}, {json.dumps(argv)}, {json.dumps(environment)}) = 0"

    def direct_lines(self, argv, parent=11263, child=11264):
        fixture = str(Path(argv[-2]).parent)
        environment = [f"HOME={fixture}", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"]
        # Exact line forms retained from the hosted strace6.8 original. Only
        # fixture paths/PIDs are parameters; this is not a new tracer execution.
        return [
            self.exec_line(parent, argv, environment),
            f"{parent} clone(child_stack=NULL, flags=CLONE_CHILD_CLEARTID|CLONE_CHILD_SETTID|SIGCHLD, child_tidptr=0x7f8bb11cf350) = {child}",
            f"{parent} wait4({child},  <unfinished ...>",
            self.exec_line(child, ["/usr/bin/true"], ["PATH=/usr/bin:/bin"]),
            f"{child} exit_group(0)                     = ?",
            f"{child} +++ exited with 0 +++",
            f"{parent} <... wait4 resumed>[{{WIFEXITED(s) && WEXITSTATUS(s) == 0}}], 0, NULL) = {child}",
            f"{parent} --- SIGCHLD {{si_signo=SIGCHLD, si_code=CLD_EXITED, si_pid={child}, si_uid=1001, si_status=0, si_utime=0, si_stime=0}} ---",
            f"{parent} exit_group(0)                     = ?",
            f"{parent} +++ exited with 0 +++",
        ]

    @staticmethod
    def encoded(lines):
        return ("\n".join(lines) + "\n").encode("ascii")

    def decode(self, lines, argv, payload=None):
        import time
        return trace_case.decode_process_trace(self.encoded(lines), argv, payload or argv, time.monotonic() + 2)

    def hosted_readonly_original(self):
        # Original 58d/run43736745240, artifact14172629455: no tracer execution.
        # Keep the complete captured bytes, including the namespace annotations.
        body = (
            b'21262 execve("/usr/bin/bwrap", ["/usr/bin/bwrap", "--unshare-user", "--uid", "1001", "--gid", "1001", "--unshare-pid", "--unshare-net", "--unshare-ipc", "--unshare-uts", "--disable-userns", "--die-with-parent", "--new-session", "--cap-drop", "ALL", "--tmpfs", "/", "--ro-bind", "/usr", "/usr", "--symlink", "usr/bin", "/bin", "--symlink", "usr/lib", "/lib", "--symlink", "usr/lib64", "/lib64", "--ro-bind", "/home/runner/work/_temp/grok-inert-tracer-step0/fixture", "/home/runner/work/_temp/grok-inert-tracer-step0/fixture", "--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev", "--remount-ro", "/dev/null", "--remount-ro", "/dev/zero", "--remount-ro", "/dev/full", "--remount-ro", "/dev/random", "--remount-ro", "/dev/urandom", "--remount-ro", "/dev/tty", "--remount-ro", "/dev/pts", "--remount-ro", "/dev", "--remount-ro", "/", "--chdir", "/home/runner/work/_temp/grok-inert-tracer-step0/fixture", "--", "/usr/bin/python3", "-I", "-B", "-c", "import errno, os, sys\\npid = os.fork()\\nif pid == 0:\\n    os.execve(\'/usr/bin/true\', [\'/usr/bin/true\'], {\'PATH\': \'/usr/bin:/bin\'})\\ngot, status = os.waitpid(pid, 0)\\nif got != pid or not os.WIFEXITED(status) or os.WEXITSTATUS(status) != 0:\\n    raise SystemExit(72)\\ntarget, readonly = sys.argv[1], sys.argv[2] == \'readonly\'\\ntry:\\n    fd = os.open(target, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)\\nexcept OSError as error:\\n    if not readonly or error.errno != errno.EROFS:\\n        raise\\n    os.write(1, b\'STEP0_READONLY_EROFS\\\\n\')\\nelse:\\n    os.close(fd)\\n    os.unlink(target)\\n    if readonly:\\n        raise SystemExit(73)\\n    os.write(1, b\'STEP0_DIRECT_DONE\\\\n\')\\n", "/home/runner/work/_temp/grok-inert-tracer-step0/fixture/read-only-attempt", "readonly"], ["HOME=/home/runner/work/_temp/grok-inert-tracer-step0/fixture", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"]) = 0\n'
            b'21262 wait4(-1, 0x7ffc43fc9020, WNOHANG, NULL) = -1 ECHILD (No child processes)\n'
            b'21262 clone(child_stack=NULL, flags=CLONE_NEWNS|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWUSER|CLONE_NEWPID|CLONE_NEWNET|SIGCHLD) = 21263\n'
            b'21263 socket(AF_UNIX, SOCK_DGRAM|SOCK_CLOEXEC, 0) = 5\n'
            b'21263 socket(AF_NETLINK, SOCK_RAW|SOCK_CLOEXEC, NETLINK_ROUTE) = 5\n'
            b"21263 bind(5, {sa_family=AF_NETLINK, nl_pid=1 /* 21263 in strace's PID NS */, nl_groups=00000000}, 12) = 0\n"
            b'21263 sendto(5, [{nlmsg_len=40, nlmsg_type=0x14 /* NLMSG_??? */, nlmsg_flags=NLM_F_REQUEST|NLM_F_ACK|0x600, nlmsg_seq=0, nlmsg_pid=1 /* 21263 in strace\'s PID NS */}, "\\x02\\x08\\x80\\xfe\\x01\\x00\\x00\\x00\\x08\\x00\\x02\\x00\\x7f\\x00\\x00\\x01\\x08\\x00\\x01\\x00\\x7f\\x00\\x00\\x01"], 40, 0, {sa_family=AF_NETLINK, nl_pid=0, nl_groups=00000000}, 12) = 40\n'
            b"21263 recvfrom(5, [{nlmsg_len=36, nlmsg_type=NLMSG_ERROR, nlmsg_flags=NLM_F_CAPPED, nlmsg_seq=0, nlmsg_pid=1 /* 21263 in strace's PID NS */}, {error=0, msg={nlmsg_len=40, nlmsg_type=0x14 /* NLMSG_??? */, nlmsg_flags=NLM_F_REQUEST|NLM_F_ACK|0x600, nlmsg_seq=0, nlmsg_pid=1 /* 21263 in strace's PID NS */}}], 1024, 0, NULL, NULL) = 36\n"
            b'21263 sendto(5, [{nlmsg_len=32, nlmsg_type=0x10 /* NLMSG_??? */, nlmsg_flags=NLM_F_REQUEST|NLM_F_ACK, nlmsg_seq=1, nlmsg_pid=1 /* 21263 in strace\'s PID NS */}, "\\x00\\x00\\x00\\x00\\x01\\x00\\x00\\x00\\x01\\x00\\x00\\x00\\x01\\x00\\x00\\x00"], 32, 0, {sa_family=AF_NETLINK, nl_pid=0, nl_groups=00000000}, 12) = 32\n'
            b"21263 recvfrom(5, [{nlmsg_len=36, nlmsg_type=NLMSG_ERROR, nlmsg_flags=NLM_F_CAPPED, nlmsg_seq=1, nlmsg_pid=1 /* 21263 in strace's PID NS */}, {error=0, msg={nlmsg_len=32, nlmsg_type=0x10 /* NLMSG_??? */, nlmsg_flags=NLM_F_REQUEST|NLM_F_ACK, nlmsg_seq=1, nlmsg_pid=1 /* 21263 in strace's PID NS */}}], 1024, 0, NULL, NULL) = 36\n"
            b"21263 clone(child_stack=NULL, flags=CLONE_CHILD_CLEARTID|CLONE_CHILD_SETTID|SIGCHLD, child_tidptr=0x7f0d2366a4d0) = 2 /* 21267 in strace's PID NS */\n"
            b'21267 execve("/usr/bin/python3", ["/usr/bin/python3", "-I", "-B", "-c", "import errno, os, sys\\npid = os.fork()\\nif pid == 0:\\n    os.execve(\'/usr/bin/true\', [\'/usr/bin/true\'], {\'PATH\': \'/usr/bin:/bin\'})\\ngot, status = os.waitpid(pid, 0)\\nif got != pid or not os.WIFEXITED(status) or os.WEXITSTATUS(status) != 0:\\n    raise SystemExit(72)\\ntarget, readonly = sys.argv[1], sys.argv[2] == \'readonly\'\\ntry:\\n    fd = os.open(target, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)\\nexcept OSError as error:\\n    if not readonly or error.errno != errno.EROFS:\\n        raise\\n    os.write(1, b\'STEP0_READONLY_EROFS\\\\n\')\\nelse:\\n    os.close(fd)\\n    os.unlink(target)\\n    if readonly:\\n        raise SystemExit(73)\\n    os.write(1, b\'STEP0_DIRECT_DONE\\\\n\')\\n", "/home/runner/work/_temp/grok-inert-tracer-step0/fixture/read-only-attempt", "readonly"], ["HOME=/home/runner/work/_temp/grok-inert-tracer-step0/fixture", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "PWD=/home/runner/work/_temp/grok-inert-tracer-step0/fixture"] <unfinished ...>\n'
            b'21263 wait4(-1,  <unfinished ...>\n'
            b'21267 <... execve resumed>)             = 0\n'
            b"21267 clone(child_stack=NULL, flags=CLONE_CHILD_CLEARTID|CLONE_CHILD_SETTID|SIGCHLD, child_tidptr=0x7f2f4d745350) = 3 /* 21268 in strace's PID NS */\n"
            b"21267 wait4(3 /* 21268 in strace's PID NS */,  <unfinished ...>\n"
            b'21268 execve("/usr/bin/true", ["/usr/bin/true"], ["PATH=/usr/bin:/bin"]) = 0\n'
            b'21268 exit_group(0)                     = ?\n'
            b'21268 +++ exited with 0 +++\n'
            b'21267 <... wait4 resumed>[{WIFEXITED(s) && WEXITSTATUS(s) == 0}], 0, NULL) = 3\n'
            b'21267 --- SIGCHLD {si_signo=SIGCHLD, si_code=CLD_EXITED, si_pid=3, si_uid=1001, si_status=0, si_utime=0, si_stime=0} ---\n'
            b'21267 exit_group(0)                     = ?\n'
            b'21267 +++ exited with 0 +++\n'
            b'21263 <... wait4 resumed>[{WIFEXITED(s) && WEXITSTATUS(s) == 0}], 0, NULL) = 2\n'
            b'21263 wait4(-1, 0x7ffc43fc900c, 0, NULL) = -1 ECHILD (No child processes)\n'
            b'21262 exit_group(0 <unfinished ...>\n'
            b'21263 exit_group(0 <unfinished ...>\n'
            b'21262 <... exit_group resumed>)         = ?\n'
            b'21263 <... exit_group resumed>)         = ?\n'
            b'21262 +++ exited with 0 +++\n'
            b'21263 +++ exited with 0 +++\n'
        )
        self.assertEqual(len(body), 5837)
        self.assertEqual(hashlib.sha256(body).hexdigest(),
                         "2ed7928f15cba51340611cb41fb67162458a3babe606eed507b96a269e07af2c")
        fixture = Path("/home/runner/work/_temp/grok-inert-tracer-step0/fixture")
        with mock.patch.object(trace_case.os, "getuid", return_value=1001), \
                mock.patch.object(trace_case.os, "getgid", return_value=1001), \
                mock.patch.object(trace_case.os, "readlink", side_effect=lambda path: {
                    "/bin": "usr/bin", "/lib": "usr/lib", "/lib64": "usr/lib64"}[path]):
            argv = trace_case.readonly_arguments(fixture)
        return body, argv, self.python_argv(fixture, "readonly")

    def test_hosted_readonly_trace_joins_network_and_namespace_children(self):
        import time
        body, argv, payload = self.hosted_readonly_original()
        self.assertEqual(trace_case.decode_process_trace(body, argv, payload, time.monotonic() + 2),
                         dict(complete=True, processes=4, execs=3, silent_child_execs=1))

    def test_hosted_readonly_altered_network_or_namespace_records_refuse(self):
        body, argv, payload = self.hosted_readonly_original()
        original = body.decode("ascii").splitlines()
        variants = {}
        changes = {
            "unknown-family": (3, "AF_UNIX", "AF_INET"),
            "unknown-socket-kind": (3, "SOCK_DGRAM", "SOCK_STREAM"),
            "different-netlink-fd": (4, "= 5", "= 6"),
            "different-bind-fd": (5, "bind(5", "bind(6"),
            "wrong-netlink-observer": (5, "21263 in", "21267 in"),
            "wrong-netlink-local-pid": (5, "nl_pid=1", "nl_pid=2"),
            "unknown-send-payload": (6, r"\x7f", r"\x7e"),
            "unknown-netlink-sequence": (6, "nlmsg_seq=0", "nlmsg_seq=2"),
            "failed-netlink-ack": (7, "error=0", "error=-EPERM"),
            "partial-netlink-ack": (7, ") = 36", ") = 35"),
            "unknown-loopback-flags": (8, "NLM_F_REQUEST|NLM_F_ACK", "NLM_F_REQUEST"),
            "different-network-process": (3, "21263 socket", "21262 socket"),
            "wrong-clone-observer": (10, "21267 in", "21266 in"),
            "missing-clone-observer": (10, " /* 21267 in strace's PID NS */", ""),
            "wrong-wait-observer": (15, "21268 in", "21263 in"),
            "wrong-wait-local-pid": (15, "wait4(3", "wait4(2"),
            "wrong-wait-return": (19, "= 3", "= 2"),
            "wrong-signal-local-pid": (20, "si_pid=3", "si_pid=2"),
            "unknown-echild-option": (24, ", 0, NULL)", ", __WALL, NULL)"),
        }
        for name, (index, before, after) in changes.items():
            self.assertIn(before, original[index], name)
            changed = list(original)
            changed[index] = changed[index].replace(before, after)
            variants[name] = changed
        for index in range(3, 10):
            variants[f"missing-network-row-{index}"] = original[:index] + original[index + 1:]
        variants["network-row-after-setup"] = original[:10] + [original[3]] + original[10:]
        variants["network-row-after-payload"] = original[:14] + [original[3]] + original[14:]
        variants["echild-before-wait"] = original[:15] + [
            "21267 wait4(-1, 0x7ffc, 0, NULL) = -1 ECHILD (No child processes)"] + original[15:]
        variants["missing-silent-terminal"] = [row for row in original if row != "21268 +++ exited with 0 +++"]
        variants["missing-parent-terminal"] = original[:-1]
        variants["unmatched-payload-resume"] = original[:11] + original[12:]
        for name, lines in variants.items():
            with self.subTest(case=name), self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
                self.decode(lines, argv, payload)

    def test_hosted_direct_shape_observes_the_silent_child(self):
        argv = self.python_argv(Path("/home/runner/work/_temp/grok-inert-tracer-step0/fixture"))
        lines = self.direct_lines(argv)
        self.assertEqual(len(self.encoded(lines)), 1540)
        self.assertEqual(self.decode(lines, argv),
                         dict(complete=True, processes=2, execs=2, silent_child_execs=1))

    def test_child_events_before_split_clone_return_join_to_that_original_fork(self):
        argv = self.python_argv(Path("/fixture"))
        lines = self.direct_lines(argv)
        prefix = lines[1].split(" = ")[0]
        split = [lines[0], prefix + " <unfinished ...>", *lines[3:6],
                 "11263 <... clone resumed> = 11264",
                 "11263 wait4(11264, [{WIFEXITED(s) && WEXITSTATUS(s) == 0}], 0, NULL) = 11264",
                 *lines[7:]]
        self.assertEqual(self.decode(split, argv)["silent_child_execs"], 1)
        with self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
            self.decode([lines[0], *lines[3:6], lines[1], *split[6:]], argv)

    def test_missing_or_foreign_process_events_never_complete(self):
        argv = self.python_argv(Path("/fixture"))
        original = self.direct_lines(argv)
        variants = {
            "silent-exec-absent": [row for row in original if '11264 execve(' not in row],
            "child-terminal-absent": [row for row in original if row != "11264 +++ exited with 0 +++"],
            "root-terminal-absent": original[:-1],
            "uncreated-child": original[:1] + original[2:],
            "unmatched-resume": [original[0], original[1], *original[3:]],
            "unmatched-unfinished": [row for row in original if "<... wait4 resumed>" not in row],
            "pid-reuse": [*original[:7], original[1], *original[7:]],
            "event-after-terminal": [*original[:6], original[3], *original[6:]],
            "detached": [*original[:8], "strace: Process 11264 detached", *original[8:]],
            "namespace-annotation": [row.replace("11264 ", "11264[2] ", 1) for row in original],
            "unobserved-network": [original[0], "11263 socket(AF_NETLINK, SOCK_RAW, NETLINK_ROUTE) = 3", *original[1:]],
            "clone3": [row.replace(" clone(", " clone3(", 1) for row in original],
            "execveat": [row.replace(" execve(", " execveat(", 1) for row in original],
            "foreign-exec": [row.replace('["/usr/bin/true"]', '["/usr/bin/true", "foreign"]') for row in original],
            "wrong-wait-child": [row.replace(") = 11264", ") = 11265") if "resumed>" in row else row for row in original],
            "wrong-signal-child": [row.replace("si_pid=11264", "si_pid=11265") for row in original],
            "nonzero-exit": [row.replace("11264 exit_group(0)", "11264 exit_group(1)") for row in original],
        }
        for name, lines in variants.items():
            with self.subTest(case=name), self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
                self.decode(lines, argv)
        with self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
            self.decode(original, [*argv[:-1], "different"])
        initial = ["/usr/bin/bwrap", "--", *argv]
        before_payload = self.readonly_lines(initial, argv)
        before_payload[3], before_payload[4] = before_payload[4], before_payload[3]
        with self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
            self.decode(before_payload, initial, argv)

    def test_exec_strings_and_arrays_require_complete_observed_bytes(self):
        argv = self.python_argv(Path("/fixture"))
        original = self.direct_lines(argv)
        initial = original[0]
        variants = (
            initial.replace('["/usr/bin/python3",', '[0x1234,', 1),
            initial.replace('"direct"],', '"direct"...],', 1),
            initial.replace('"direct"],', '"direct", ...],', 1),
            initial.replace('["HOME=/fixture", "PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"]', "0x7fff"),
            initial.replace('"/usr/bin/python3"', '"/usr/bin/py\\x74hon3"', 1),
            initial.replace('"/usr/bin/python3"', '"/usr/bin/py\\164hon3"', 1),
            initial.replace('"/usr/bin/python3"', '"/usr/bin/python3"...', 1),
            initial[:-1],
        )
        for row in variants:
            with self.subTest(row=row[:100]), self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
                self.decode([row, *original[1:]], argv)
        import time
        for body in (self.encoded(original)[:-1], self.encoded(original) + b"\n",
                     self.encoded(original).replace(b"11264 execve", b"\xff1264 execve"),
                     b"x" * (trace_case.LIMITS["trace"] + 1)):
            with self.subTest(bytes=len(body)), self.assertRaisesRegex(RuntimeError, "^process-trace-refused$"):
                trace_case.decode_process_trace(body, argv, argv, time.monotonic() + 2)

    def test_original_deadline_still_applies_during_decoding(self):
        argv = self.python_argv(Path("/fixture"))
        body = self.encoded(self.direct_lines(argv))
        with mock.patch.object(trace_case.time, "monotonic", return_value=2):
            with self.assertRaisesRegex(RuntimeError, "^original-deadline$"):
                trace_case.decode_process_trace(body, argv, argv, 1)
        samples = iter([0, 0, 0, 2])
        with mock.patch.object(trace_case.time, "monotonic", side_effect=lambda: next(samples)):
            with self.assertRaisesRegex(RuntimeError, "^original-deadline$"):
                trace_case.decode_process_trace(body, argv, argv, 1)

    def readonly_lines(self, argv, payload):
        # Synthetic composition of the two observed clone forms. This tests the
        # decoder/control wiring, not visibility inside a real readonly child.
        return [
            self.exec_line(11260, argv, ["PATH=/usr/bin:/bin"]),
            "11260 wait4(-1, 0x7ffd, WNOHANG, NULL) = -1 ECHILD (No child processes)",
            "11260 clone(child_stack=NULL, flags=CLONE_NEWNS|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWUSER|CLONE_NEWPID|CLONE_NEWNET|SIGCHLD) = 11263",
            *self.direct_lines(payload),
            "11260 wait4(-1, [{WIFEXITED(s) && WEXITSTATUS(s) == 0}], WNOHANG, NULL) = 11263",
            "11260 exit_group(0) = ?",
            "11260 +++ exited with 0 +++",
        ]

    def run_controls(self, fault=None, observations=None):
        import time
        clock = [time.monotonic()]
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            fixture = root / "fixture"
            fixture.mkdir(mode=0o700)
            (fixture / "original.txt").write_bytes(b"inert original input\n")
            (fixture / "original.txt").chmod(0o600)
            receipt = dict(versions=[], runs=[], complete_process_family_admitted=False, native_qualification=False)
            labels = []

            def collect(label, argv, *args, **kwargs):
                labels.append(label)
                if observations is not None:
                    observations.append(dict(label=label, argv=list(argv), cwd=args[1],
                                             environment=dict(args[2]), deadline=args[3],
                                             trace=kwargs.get("trace", False)))
                streams = {name: dict(body=bytearray(), eof=True) for name in ("stdout", "stderr")}
                row = dict(label=label, argv=argv, exit=0, error=None, wait_observed=True, streams=streams)
                if label == "readonly-untraced":
                    streams["stdout"]["body"] = bytearray(b"STEP0_READONLY_EROFS\n")
                    if fault == "untraced-loopback":
                        row["exit"] = 1
                        streams["stdout"]["body"] = bytearray()
                        streams["stderr"]["body"] = bytearray(b"bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted\n")
                    elif fault == "untraced-partial":
                        streams["stderr"]["eof"] = False
                    elif fault == "untraced-binary":
                        row["exit"] = 1
                        streams["stderr"]["body"] = bytearray(b"\xff\x00\\unknown")
                    elif fault == "untraced-marker":
                        streams["stdout"]["body"] = bytearray(b"STEP0_DIRECT_DONE\n")
                    elif fault == "untraced-mutation":
                        (fixture / "unexpected").write_bytes(b"inert unexpected effect")
                    elif fault == "untraced-deadline":
                        clock[0] = args[3] + 0.01
                    return row
                if label not in ("direct", "readonly"):
                    return row
                payload = argv if label == "direct" else self.python_argv(fixture, "readonly")
                lines = self.direct_lines(argv) if label == "direct" else self.readonly_lines(argv, payload)
                if fault == label:
                    lines = [line for line in lines if "11264 +++ exited" not in line]
                streams["trace"] = dict(body=bytearray(self.encoded(lines)), eof=True)
                streams["stdout"]["body"] = bytearray(b"STEP0_DIRECT_DONE\n" if label == "direct"
                                                     else b"STEP0_READONLY_EROFS\n")
                if fault == "loopback" and label == "readonly":
                    row["exit"] = 1
                    streams["stdout"]["body"] = bytearray()
                    streams["stderr"]["body"] = bytearray(b"bwrap: loopback: Failed RTM_NEWADDR: Operation not permitted\n")
                    streams["trace"]["body"] = bytearray(b"unparsed original network diagnostic\n")
                return row

            error = None
            with mock.patch.object(trace_case, "collect", side_effect=collect), \
                    mock.patch.object(trace_case.os, "readlink", side_effect=lambda path: {
                        "/bin": "usr/bin", "/lib": "usr/lib", "/lib64": "usr/lib64"}[path]), \
                    mock.patch.object(trace_case.time, "monotonic", side_effect=lambda: clock[0]):
                try:
                    env = dict(HOME=str(fixture), PATH="/usr/bin:/bin", LANG="C", LC_ALL="C")
                    trace_case.controls(receipt, root, fixture, env, time.monotonic() + 2)
                except RuntimeError as caught:
                    error = str(caught)
            self.assertFalse(receipt["complete_process_family_admitted"])
            self.assertFalse(receipt["native_qualification"])
            return receipt, labels, error

    def test_actual_control_path_requires_decoder_for_both_captures(self):
        receipt, labels, error = self.run_controls()
        self.assertIsNone(error)
        self.assertEqual(labels, ["strace-version", "bwrap-version", "direct", "readonly-untraced", "readonly"])
        self.assertEqual([row.get("process_observation") for row in receipt["runs"]], [
            dict(complete=True, processes=2, execs=2, silent_child_execs=1),
            None,
            dict(complete=True, processes=3, execs=3, silent_child_execs=1)])
        for label in ("direct", "readonly"):
            with self.subTest(control=label):
                receipt, labels, error = self.run_controls(label)
                self.assertEqual(error, label + "-process-trace-refused")
                self.assertEqual(labels[-1], label)
                self.assertNotIn("process_observation", receipt["runs"][-1])
                self.assertEqual(receipt["runs"][-1]["streams"]["trace"]["file"], label + ".trace")

    def test_original_loopback_failure_precedes_decoder_admission(self):
        receipt, labels, error = self.run_controls("loopback")
        self.assertEqual(labels[-1], "readonly")
        self.assertEqual(error, "readonly-bwrap-loopback-permission")
        self.assertTrue(receipt["runs"][0]["process_observation"]["complete"])
        self.assertNotIn("process_observation", receipt["runs"][-1])

    def test_untraced_and_traced_readonly_use_the_same_original_inputs_and_budget(self):
        observations = []
        receipt, _, error = self.run_controls(observations=observations)
        self.assertIsNone(error)
        before, after = observations[-2:]
        self.assertEqual(before["label"], "readonly-untraced")
        self.assertEqual(after["label"], "readonly")
        for key in ("argv", "cwd", "environment", "deadline"):
            self.assertEqual(before[key], after[key])
        self.assertFalse(before["trace"])
        self.assertTrue(after["trace"])
        self.assertEqual(len({row["deadline"] for row in observations}), 1)
        self.assertEqual(receipt["readonly_fixture_before"], receipt["readonly_untraced_fixture_after"])
        self.assertEqual(receipt["readonly_fixture_before"], receipt["readonly_fixture_after"])
        self.assertNotIn("process_observation", receipt["runs"][1])

    def test_untraced_failures_are_retained_before_any_traced_readonly_attempt(self):
        faults = {"untraced-loopback": "readonly-untraced-bwrap-loopback-permission",
                  "untraced-partial": "readonly-untraced-refused", "untraced-marker": "readonly-untraced-refused",
                  "untraced-binary": "readonly-untraced-refused",
                  "untraced-mutation": "fixture-members"}
        for fault, reason in faults.items():
            with self.subTest(fault=fault):
                receipt, labels, error = self.run_controls(fault)
                self.assertEqual(error, reason)
                self.assertEqual(labels[-1], "readonly-untraced")
                self.assertNotIn("readonly", labels)
                row = receipt["runs"][-1]
                self.assertNotIn("process_observation", row)
                self.assertEqual(set(row["streams"]), {"stdout", "stderr"})
                for stream in row["streams"].values():
                    raw = base64.b64decode(stream["base64"], validate=True)
                    self.assertEqual(len(raw), stream["bytes"])
                    self.assertEqual(hashlib.sha256(raw).hexdigest(), stream["sha256"])

    def test_untraced_budget_exhaustion_does_not_start_the_traced_control(self):
        observations = []
        receipt, labels, error = self.run_controls("untraced-deadline", observations)
        self.assertEqual(error, "original-deadline")
        self.assertEqual(labels[-1], "readonly-untraced")
        self.assertNotIn("readonly", labels)
        self.assertEqual(len({row["deadline"] for row in observations}), 1)
        self.assertEqual(receipt["runs"][-1]["exit"], 0)


if __name__ == "__main__":
    unittest.main()
