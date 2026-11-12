"""Cline's native MCP file uses transport envelopes, not flat command entries."""
import unittest

from cline_managed_session_e2e import cline_owned_stdio


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


if __name__ == "__main__":
    unittest.main()
