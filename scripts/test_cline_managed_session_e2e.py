"""Cline's native MCP file uses transport envelopes, not flat command entries."""
import unittest
import os
from pathlib import Path
import tempfile
import json
import hashlib

from cline_managed_session_e2e import SessionJourney, SERVER, cline_owned_stdio, read_owned, snapshot, validate_events
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


if __name__ == "__main__":
    unittest.main()
