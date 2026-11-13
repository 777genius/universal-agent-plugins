"""Cline's native MCP file uses transport envelopes, not flat command entries."""
import unittest
import importlib.util
import sys
import subprocess
from unittest import mock
import os
from pathlib import Path
import tempfile
import json
import hashlib
import copy

from cline_managed_session_e2e import NAME, SessionJourney, SERVER, cline_owned_stdio, read_owned, snapshot, validate_events, write
from project_native_collectors.cline import ACQUISITION, platform_acquisition


class ClineTransportReadbackTests(unittest.TestCase):
    def setUp(self):
        self.entry = {"transport": {"type": "stdio", "command": "/owned/bin/probe",
                      "cwd": "/owned", "args": ["--inert"],
                      "env": {"PLUGIN_ROOT": "/owned", "PLUGIN_DATA": "/data"}},
                      "disabled": False, "autoApprove": []}

    def test_exact_native_transport_preserved(self):
        selected = cline_owned_stdio(self.entry)
        self.assertIs(selected, self.entry["transport"])
        self.assertEqual(selected["command"], "/owned/bin/probe")

    def test_flat_or_foreign_transport_cannot_be_misread_as_owned(self):
        bad = [
            {"command": "/owned/bin/probe", "env": {}},
            {"transport": dict(self.entry["transport"], type="sse")},
            {"transport": dict(self.entry["transport"], command="")},
            {"transport": dict(self.entry["transport"], cwd=None)},
            {"transport": dict(self.entry["transport"], env={"TOKEN": 10})},
            {"transport": dict(self.entry["transport"], args=["ok", 10])},
            {"transport": dict(self.entry["transport"], unexpected="unsafe")},
            dict(self.entry, disabled=True),
            dict(self.entry, autoApprove=["*"]),
            dict(self.entry, foreign=True),
            {"transport": []}, None, "stdio",
        ]
        for value in bad:
            with self.subTest(value=value), self.assertRaises(ValueError):
                cline_owned_stdio(value)


class ClinePortableFixtureTests(unittest.TestCase):
    def test_direct_session_preserves_authority_and_prior_events(self):
        for change in (None, "state", "helper", "prefix", "session"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as tmp:
                root = Path(tmp).resolve()
                journey = SessionJourney.__new__(SessionJourney)
                journey.root, journey.a, journey.nonce = root, root / "a", "owned"
                active = journey.a / "active"
                (active / "bin").mkdir(parents=True)
                helper, probe = active / "helper", active / ("bin/probe.exe" if os.name == "nt" else "bin/probe")
                helper.write_bytes(b"helper"); probe.write_bytes(b"probe")
                journey.probe, journey.events = probe, root / "events.jsonl"
                journey.events.write_bytes(b"{}\n")
                config, state = journey.a / ".uap/clients/cline/mcp.json", journey.a / ".uap/state-v2.json"
                config.parent.mkdir(parents=True)
                state.write_bytes(b"{}")
                config.write_text(json.dumps({"mcpServers": {SERVER: {"transport": {
                    "type": "stdio", "command": str(helper), "cwd": str(active), "env": {}}}}}))
                journey.evidence = {"installer_sha256": hashlib.sha256(b"helper").hexdigest()}
                facts = {"nonce": "owned", "revision": "0", "operation": "write", "pid": 1, "session": "one"}
                def run(action):
                    self.assertEqual(action, "direct-create")
                    rows = [{"method": method, "pid": 1, "session": "one", "nonce": "owned",
                             "facts": facts if method == "tools/call" else None}
                            for method in ("initialize", "tools/list", "tools/call")]
                    if change == "session": rows[0]["session"] = "foreign"
                    prefix = b"{ }\n" if change == "prefix" else b"{}\n"
                    journey.events.write_bytes(prefix + b"".join(json.dumps(row).encode() + b"\n" for row in rows))
                    if change == "state": state.write_bytes(b"changed")
                    if change == "helper":
                        helper.rename(active / "old-helper"); helper.write_bytes(b"helper")
                    return json.dumps({"caller": "direct-installed-stdio-client", "native_cline_tools_call": False,
                                       "facts": facts}).encode()
                journey.run = run
                if change is None:
                    self.assertEqual(journey.direct("direct-create", "0")["facts"], facts)
                else:
                    with self.assertRaises(ValueError): journey.direct("direct-create", "0")

    def test_native_platforms_are_exact_and_independent(self):
        self.assertEqual(platform_acquisition("Linux", "x86_64"), ACQUISITION)
        self.assertEqual(platform_acquisition("Windows", "AMD64")["entrypoint"], "package/bin/cline.exe")
        self.assertIn("cli-darwin-arm64-3.0.73", platform_acquisition("Darwin", "arm64")["url"])
        with self.assertRaises(ValueError):
            platform_acquisition("Darwin", "x86_64")
        local = platform_acquisition("Linux", "x86_64")
        local["entrypoint"] = "changed"
        self.assertEqual(ACQUISITION["entrypoint"], "package/bin/cline")

    def test_owned_reads_refuse_alias_hardlink_and_growth(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            source = root / "source"
            source.write_bytes(b"owned")
            self.assertEqual(read_owned(source, root), b"owned")
            with self.assertRaises(ValueError):
                read_owned(source, root, 4)
            hardlink = root / "linked"
            os.link(source, hardlink)
            with self.assertRaises(ValueError):
                read_owned(source, root)
            hardlink.unlink()
            # Symlink creation is permission-dependent on Windows; hardlink,
            # reparse identity and actual native checks still run there.
            if os.name != "nt":
                hardlink.symlink_to(source)
                with self.assertRaises(ValueError):
                    read_owned(hardlink, root)

    def test_foreign_control_replacement_and_extra_directory_are_observed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp).resolve()
            (root / "foreign").write_bytes(b"kept")
            original = snapshot(root)
            (root / "extra").mkdir()
            with self.assertRaises(ValueError):
                snapshot(root)
            (root / "extra").rmdir()
            (root / "foreign").rename(root / "held-old")
            (root / "foreign").write_bytes(b"kept")
            (root / "held-old").unlink()
            self.assertNotEqual(snapshot(root), original)

    def test_event_proof_requires_every_exact_session_and_no_extras(self):
        direct = [{"facts": {"pid": index + 1, "session": "s" + str(index), "nonce": "owned"}} for index in range(5)]
        result = {"stages": [{"direct_installed_mcp": value} for value in direct[:3]],
                  "cold_locked_restore": {"direct_installed_mcp": direct[3], "repeat_mcp": direct[4]}}
        rows = [{**value["facts"], "method": method, "facts": value["facts"] if method == "tools/call" else None}
                for value in direct for method in ("initialize", "tools/list", "tools/call")]
        encode = lambda values: b"\n".join(json.dumps(value).encode() for value in values)
        validate_events(result, encode(rows))
        for invalid in ([], rows[:-1], rows + rows[:1], [dict(rows[0], session="foreign"), *rows[1:]]):
            with self.assertRaises(ValueError):
                validate_events(result, encode(invalid))


class ClineWarmContinuityTests(unittest.TestCase):
    def fixture(self, root):
        probe = root / "probe"
        probe.write_bytes(b"inert fixture")
        journey = SessionJourney(root, probe, probe, probe, probe, {"commands": [], "stages": []}, None, {})
        physical, receipt_id = "owned-package", "data-original"
        data = journey.a / ".uap/plugin-data" / physical
        active = journey.a / ".uap/managed/clients/cline" / physical
        active.mkdir(parents=True)
        receipt = {"data_receipt_id": receipt_id, "physical_backend_id": physical, "scope": "project", "state": "owned",
                   "locator": str(data), "ownership_digest": "sha256:" + "a" * 64,
                   "created_at": "2026-11-13T00:00:00Z", "updated_at": "2026-11-13T00:00:00Z"}
        binding = {"client_binding_id": "binding-original", "client_id": "cline", "scope": "project", "activation": "prepared",
                   "target_locator": str(active), "physical_artifact_id": physical, "native_profile_root": str(journey.a),
                   "verification": "installation_verified", "authentication": "unchecked", "policy": "not_required",
                   "data_receipt_id": receipt_id}
        installation = {"installation_id": "installation-original", "declared_name": NAME,
                        "source": {"source_binding_id": "source-original", "requested_source": str(journey.source),
                                   "canonical_source": str(journey.source), "resolved_revision": ""},
                        "package": {"declared_name": NAME, "loader_kind": "standard", "inventory": {}},
                        "clients": {"cline": binding}, "data_receipts": {receipt_id: receipt}}
        state = {"schema_version": 5, "installations": [installation]}
        def save():
            write(journey.a / ".uap/state-v2.json", state)
            for value in installation["data_receipts"].values():
                write(Path(value["locator"]) / ".agentplugins-data-owner.json", value)
        def revision(value):
            journey.author(value)
            manifest = "sha256:" + hashlib.sha256((journey.source / "plugin.json").read_bytes()).hexdigest()
            tree = "sha256:" + hashlib.sha256(("tree-" + value).encode()).hexdigest()
            installation["source"]["tree_digest"] = tree
            installation["package"].update(version="1.0." + value, manifest_digest=manifest)
            binding["package_revision"] = {"version": "1.0." + value, "tree_digest": tree, "manifest_digest": manifest}
            save()
        revision("0")
        (active / "helper").write_bytes(b"owned helper")
        write(journey.a / ".uap/clients/cline/mcp.json", {"mcpServers": {SERVER: {"transport": {
            "type": "stdio", "command": str(active / "helper"), "cwd": str(active),
            "env": {"PLUGIN_ROOT": str(active), "UAP_TEST_NONCE": journey.nonce}}}}})
        accepted, selected, _ = journey.owned()
        journey.warm_origin, journey.warm_pin = journey.warm_facts(accepted, selected, "0")
        write(data / "native-marker.txt", "kept-" + journey.nonce)
        journey.marker_before = journey.marker_facts()
        return journey, state, save, revision

    def test_rebound_authority_or_copied_marker_refuses_before_bridge(self):
        for change in ("installation", "source", "binding", "data", "marker", "manifest", "pending"):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as tmp:
                journey, state, save, _ = self.fixture(Path(tmp).resolve())
                installed = state["installations"][0]
                binding = installed["clients"]["cline"]
                if change == "installation": installed["installation_id"] = "foreign"
                if change == "source": installed["source"]["source_binding_id"] = "foreign"
                if change == "binding": binding["client_binding_id"] = "foreign"
                if change == "pending": binding["pending_native_intent"] = {}
                if change == "manifest": write(journey.source / "plugin.json", {"name": NAME, "version": "1.0.0"})
                if change == "data":
                    old = next(iter(installed["data_receipts"].values()))
                    replacement = dict(old, data_receipt_id="data-replaced", physical_backend_id="relocated",
                                       locator=str(journey.a / ".uap/plugin-data/relocated"))
                    installed["data_receipts"] = {"data-replaced": replacement}
                    binding.update(data_receipt_id="data-replaced", physical_artifact_id="relocated",
                                   target_locator=str(journey.a / ".uap/managed/clients/cline/relocated"))
                    Path(binding["target_locator"]).mkdir()
                    write(Path(replacement["locator"]) / "native-marker.txt", "kept-" + journey.nonce)
                if change == "marker":
                    marker = Path(next(iter(installed["data_receipts"].values()))["locator"]) / "native-marker.txt"
                    marker.rename(marker.with_name("old-marker")); write(marker, "kept-" + journey.nonce)
                save()
                with mock.patch("cline_managed_session_e2e.invoke_bridge", side_effect=AssertionError("native tripwire")) as bridge:
                    with self.assertRaises(ValueError): journey.session("repaired", "0")
                    bridge.assert_not_called()

    def test_update_repair_and_remove_use_first_authority_and_last_accepted_revision(self):
        with tempfile.TemporaryDirectory() as tmp:
            journey, state, save, revision = self.fixture(Path(tmp).resolve())
            original = copy.deepcopy(journey.warm_origin)
            config = journey.a / ".uap/clients/cline/mcp.json"
            rows = [{"name": SERVER, "path": str(config), "transportType": "stdio", "disabled": False}]
            record = {"exit": 0, "runtime_sha256": {key: "a" * 64 for key in ("native", "installer", "probe", "git", "go", "fixture_source")}}
            with mock.patch("cline_managed_session_e2e.invoke_bridge", return_value=(record, json.dumps(rows).encode(), b"")) as bridge, \
                 mock.patch.object(journey, "direct", return_value={"facts": {}}):
                journey.session("installed", "0")
                revision("1")
                journey.session("updated", "1")
                journey.session("repaired", "1")
                self.assertEqual(bridge.call_count, 3)
                self.assertEqual(journey.warm_origin, original)
                self.assertEqual(journey.warm_pin["package"]["version"], "1.0.1")
                revision("2")
                with self.assertRaises(ValueError): journey.session("repaired", "2")
                self.assertEqual(bridge.call_count, 3)
            revision("1")
            retained = state["installations"][0]
            retained["clients"], retained["data_retained"] = {}, True
            save()
            journey.verify_retained(state)
            for change in ("installation", "receipt", "revision", "marker"):
                invalid = copy.deepcopy(state)
                item = invalid["installations"][0]
                if change == "installation": item["installation_id"] = "replaced"
                if change == "receipt": next(iter(item["data_receipts"].values()))["updated_at"] = "renewed"
                if change == "revision": item["package"]["version"] = "1.0.2"
                if change == "marker":
                    marker = Path(next(iter(item["data_receipts"].values()))["locator"]) / "native-marker.txt"
                    marker.rename(marker.with_name("held-marker")); write(marker, "kept-" + journey.nonce)
                with self.subTest(change=change), self.assertRaises(ValueError): journey.verify_retained(invalid)


class UnavailablePOSIXResearchContracts(unittest.TestCase):
    def test_import_without_resource_refuses_before_popen(self):
        spec = importlib.util.spec_from_file_location("cline_inert_no_resource", Path(__file__).with_name("native_client_research.py"))
        module = importlib.util.module_from_spec(spec)
        with mock.patch.dict(sys.modules, {"resource": None}):
            spec.loader.exec_module(module)
            with mock.patch.object(module.subprocess, "Popen") as start:
                with self.assertRaises(ValueError):
                    module.run_probe(["must-not-start"], Path("."), {}, Path("."), "inert", memory_only=True)
                with self.assertRaises(ImportError):
                    module.child_limits()
                start.assert_not_called()
        code = "import sys; sys.modules['resource']=None; sys.path.insert(0, " + repr(str(Path(__file__).resolve().parent)) + "); import cline_managed_session_e2e"
        completed = subprocess.run([sys.executable, "-I", "-c", code], stdin=subprocess.DEVNULL,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=10, check=False)
        self.assertEqual((completed.returncode, completed.stdout, completed.stderr), (0, b"", b""))
        self.assertEqual(module.bounded_read(__import__("io").BytesIO(b"safe"), 4), b"safe")


if __name__ == "__main__":
    unittest.main()
