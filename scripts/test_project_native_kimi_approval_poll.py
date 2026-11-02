"""Inert polling-refusal controls; no Kimi, installer or provider execution."""
import copy
import io
import json
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from project_native_collectors import kimi_approval_facts as row_facts
from project_native_collectors import kimi_approval_poll as poll
from project_native_collectors import kimi_attempt as attempt
import test_project_native_kimi_attempt as inert


class Hostile:
    def __eq__(self, other):
        raise AssertionError("untrusted equality must not execute")

    def __repr__(self):
        raise AssertionError("untrusted representation must not execute")

    __hash__ = object.__hash__


class KimiApprovalPollContracts(unittest.TestCase):
    observe_capture = inert.KimiAttemptContracts.observe_capture

    def test_actual_observer_preserves_both_refusals_and_quarantine(self):
        for fault, reason, count in (("multiple-pending", "multiple-pending", 1),
                                     ("empty-pending", "no-pending-after-polls", 20)):
            with self.subTest(fault=fault), tempfile.TemporaryDirectory() as temporary:
                ctx = inert.CaptureContext(temporary, fault=fault)
                diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
                diagnostic.ctx = ctx
                sleeps, stdout = [], io.BytesIO()
                # Only the observer binding changes. Supervisor/readiness time
                # and its inert child remain real and independently supervised.
                with patch.object(attempt, "time", SimpleNamespace(sleep=sleeps.append)), \
                        patch.object(attempt.sys, "stdout", SimpleNamespace(buffer=stdout)):
                    with self.assertRaisesRegex(ValueError, "inert common service refused") as caught:
                        self.observe_capture(ctx, diagnostic=diagnostic)
                self.assertEqual(stdout.getvalue(), b"")
                calls = [row[0] for row in ctx.services[0].calls]
                self.assertEqual([label for label in calls if label.startswith("approvals-")],
                                 ["approvals-" + str(index) for index in range(count)])
                self.assertEqual(len(sleeps), 20 if fault == "empty-pending" else 0)
                self.assertTrue(all(0 < value <= 0.25 for value in sleeps))
                self.assertFalse(any(label in ("approve-once", "history", "shutdown")
                                     or label.startswith("snapshot-") for label in calls))
                self.assertFalse((ctx.root / "events.jsonl").exists())
                self.assertEqual(list(ctx.data.iterdir()), [])
                line = diagnostic.line(caught.exception)
                self.assertLessEqual(len(line.encode()), 2048)
                for private in (inert.TOKEN, inert.KEY, "/private/path", "private-pending-row"):
                    self.assertNotIn(private, line)
                value = json.loads(line.removeprefix("Kimi collector failure: "))
                self.assertEqual(value["approval_poll_refusal"], poll.Refusal(reason, count).public())
                self.assertNotIn("approval_refusal", value)
                self.assertEqual((value["stage"], value["phase"], value["exception_kind"]),
                                 ("approvals", "validate", "validation"))
                self.assertTrue(value["failure"])
                self.assertTrue(value["cleanup"]["group_reaped"])
                self.assertTrue(value["cleanup"]["port_closed"])
                self.assertFalse(value["cleanup"]["forced_kill"])
                self.assertTrue(value["cleanup"]["sigterm_sent"])
                self.assertFalse(value["cleanup"]["shutdown_acknowledged"])
                self.assertFalse(any(ctx.summary["qualification"].values()))

    def test_closed_facts_reject_unknown_and_hostile_values_before_operations(self):
        good = poll.Refusal("multiple-pending", 1).public()
        bad = [None, [], Hostile(), {Hostile(): "private"}, {**good, "extra": "private"}]
        bad.extend({key: value for key, value in good.items() if key != missing} for missing in good)
        for key in ("topic", "status", "reason"):
            bad.extend({**good, key: value} for value in (None, True, 1, [], {}, Hostile(), "private"))
        bad.extend({**good, "polls": value} for value in (True, False, 0, -1, 21, 1.0, "1", [], {}, Hostile()))
        bad.extend({**good, "reason": "no-pending-after-polls", "polls": value} for value in (1, 19))
        for value in bad:
            with self.assertRaises(ValueError):
                poll.validate(value)
        for count in (1, 20):
            self.assertEqual(poll.Refusal("multiple-pending", count).public()["polls"], count)
        self.assertEqual(poll.Refusal("no-pending-after-polls", 20).public()["polls"], 20)
        error = poll.Refusal("multiple-pending", 1)
        projected = error.public()
        projected["reason"] = "private"
        self.assertEqual(error.public(), good)

    @staticmethod
    def raise_error(error):
        raise error

    def test_first_cause_is_shared_by_row_and_poll_facts(self):
        row = row_facts.Refusal(row_facts.project({}, "session_owned", "provider_owned", {},
                                                attempt.TOOL, attempt.kimi._instant))
        pending = poll.Refusal("multiple-pending", 1)
        for first, second, field in ((row, pending, "approval_refusal"),
                                     (pending, row, "approval_poll_refusal")):
            diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
            diagnostic.step("approvals", "validate")
            for error in (first, second):
                with self.assertRaises(ValueError):
                    diagnostic.observer(lambda _service, held=error: self.raise_error(held))(None)
            value = json.loads(diagnostic.line(second).removeprefix("Kimi collector failure: "))
            self.assertEqual(value[field], first.public())
            self.assertEqual(set(value) & {"approval_refusal", "approval_poll_refusal"}, {field})
            diagnostic.step("service-cleanup")
            cleanup = json.loads(diagnostic.line(OSError("private-cleanup")).removeprefix("Kimi collector failure: "))
            self.assertFalse(set(cleanup) & {"approval_refusal", "approval_poll_refusal"})
            diagnostic.step("approvals", "validate")
            self.assertEqual(json.loads(diagnostic.line(second).removeprefix("Kimi collector failure: "))[field],
                             first.public())

    def test_unrelated_or_corrupt_optional_facts_keep_generic_failure(self):
        for collector, stage, phase in (("installed", "approvals", "validate"),
                                        ("single-tool", "ready", "enter"),
                                        ("single-tool", "approvals", "request")):
            diagnostic = attempt.kimi._KimiFailureDiagnostic(collector)
            diagnostic.step(stage, phase)
            with self.assertRaises(ValueError):
                diagnostic.observer(lambda _service: self.raise_error(poll.Refusal("multiple-pending", 1)))(None)
            self.assertIsNone(diagnostic.approval_poll_refusal)
        diagnostic = attempt.kimi._KimiFailureDiagnostic("single-tool")
        diagnostic.step("approvals", "validate")
        with self.assertRaises(ValueError):
            diagnostic.observer(lambda _service: self.raise_error(ValueError("private-error")))(None)
        self.assertIsNone(diagnostic.approval_poll_refusal)
        for corrupt in ({"private": "secret"}, {**poll.Refusal("multiple-pending", 1).public(), "polls": True}):
            diagnostic.approval_poll_refusal = copy.deepcopy(corrupt)
            stream = io.StringIO()
            attempt.kimi._emit_failure_diagnostic(diagnostic, ValueError("private-error"), stream)
            value = json.loads(stream.getvalue().removeprefix("Kimi collector failure: "))
            self.assertTrue(value["failure"])
            self.assertNotIn("approval_poll_refusal", value)
            self.assertNotIn("private", stream.getvalue())
        diagnostic.approval_poll_refusal = poll.Refusal("multiple-pending", 1).public()
        diagnostic.approval_refusal = dict(row_facts.REFUSED)
        self.assertNotIn("approval_poll_refusal", diagnostic.line(ValueError()))
        self.assertNotIn("approval_refusal", diagnostic.line(ValueError()))


if __name__ == "__main__":
    unittest.main()
