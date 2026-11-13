"""Adversarial contracts for pending genuine Cursor/Qoder MCP call receipts."""
from datetime import datetime, timezone, timedelta
import json
import unittest

from project_native_missing_call_proof import read_events, require_server_call, require_unchanged_after_negative

NONCE = "a" * 32
START = datetime(2026, 11, 14, 0, 0, 0, tzinfo=timezone.utc)
END = START + timedelta(seconds=30)


def row(method, *, pid=712, session="712-123456789", nonce=NONCE, sec=1, facts=None):
    return {"method": method, "pid": pid, "session": session, "nonce": nonce,
            "timestamp": (START + timedelta(seconds=sec)).isoformat().replace("+00:00", "Z"),
            "facts": facts}


def raw(rows):
    return b"".join(json.dumps(value, separators=(",", ":")).encode() + b"\n" for value in rows)


def successful():
    return [
        row("initialize"),
        row("tools/list", sec=2),
        row("tools/call", sec=3, facts={"nonce": NONCE, "pid": 712, "session": "712-123456789",
                                        "cwd": "/tmp/a", "plugin_root": "/tmp/a/fixture"})
    ]


class NativeMissingCallProofTest(unittest.TestCase):
    def accept(self, rows):
        return require_server_call(raw(rows), nonce=NONCE, started=START, ended=END)

    def reject(self, rows):
        with self.assertRaises(ValueError):
            self.accept(rows)

    def test_real_server_side_method_order(self):
        receipt = self.accept(successful())
        self.assertEqual(receipt["session"], "712-123456789")
        self.assertEqual(receipt["server_pid"], 712)
        self.assertEqual(receipt["sessions_observed"], 1)

    def test_read_only_sessions_can_precede_the_call(self):
        rows = [row("initialize", pid=711, session="711-12345"),
                row("tools/list", pid=711, session="711-12345", sec=2)] + [
                    {**r, "timestamp": (START + timedelta(seconds=i)).isoformat().replace("+00:00", "Z")}
                    for i, r in enumerate(successful(), 3)]
        self.assertEqual(self.accept(rows)["sessions_observed"], 2)

    def test_listing_is_not_call(self):
        self.reject(successful()[:2])

    def test_call_without_initialized_list_is_refused(self):
        self.reject(successful()[1:])
        self.reject([successful()[0], successful()[2]])

    def test_duplicate_calls_refused(self):
        rows = successful()
        self.reject(rows + [{**rows[-1], "timestamp": (START+timedelta(seconds=4)).isoformat().replace("+00:00","Z")}])

    def test_forged_and_replayed_nonces_refused(self):
        for index in (0, 1, 2):
            rows = successful()
            rows[index] = {**rows[index], "nonce": "b" * 32}
            self.reject(rows)
        rows = successful()
        rows[2] = {**rows[2], "facts": {**rows[2]["facts"], "nonce": "b" * 32}}
        self.reject(rows)

    def test_session_and_pid_mismatch_refused(self):
        rows = successful()
        rows[2] = {**rows[2], "session": "715-123456789"}
        self.reject(rows)
        rows = successful()
        rows[2] = {**rows[2], "facts": {**rows[2]["facts"], "pid": 999}}
        self.reject(rows)

    def test_stale_and_out_of_order_refused(self):
        rows = successful()
        rows[0] = {**rows[0], "timestamp": (START-timedelta(seconds=3)).isoformat().replace("+00:00","Z")}
        self.reject(rows)
        rows = successful()
        rows[2] = {**rows[2], "timestamp": (START+timedelta(seconds=1)).isoformat().replace("+00:00","Z")}
        self.reject(rows)

    def test_unbounded_or_noncanonical_json_refused(self):
        with self.assertRaises(ValueError):
            read_events(b"{\"method\":\"initialize\",\"method\":\"tools/list\"}\n")
        with self.assertRaises(ValueError):
            read_events(raw(successful()).rstrip(b"\n"))
        with self.assertRaises(ValueError):
            read_events(b"A" * ((1 << 20) + 1))
        with self.assertRaises(ValueError):
            read_events(b"\xff\n")

    def test_unknown_fields_and_noncall_facts_refused(self):
        rows = successful()
        rows[0] = {**rows[0], "extra": "untracked"}
        self.reject(rows)
        rows = successful()
        rows[1] = {**rows[1], "facts": {"nonce": NONCE}}
        self.reject(rows)

    def test_failed_negative_control_refused(self):
        before = raw(successful())
        require_unchanged_after_negative(before, before)
        with self.assertRaises(ValueError):
            require_unchanged_after_negative(before, before + raw([row("initialize", sec=4)]))

    def test_window_must_be_finite_and_bounded(self):
        with self.assertRaises(ValueError):
            require_server_call(raw(successful()), nonce=NONCE, started=START,
                                ended=START + timedelta(hours=2))
        with self.assertRaises(ValueError):
            require_server_call(raw(successful()), nonce="not-a-nonce", started=START, ended=END)


if __name__ == "__main__":
    unittest.main()
