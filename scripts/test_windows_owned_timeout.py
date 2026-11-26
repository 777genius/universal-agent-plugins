"""Failure-only observation must preserve the owned Windows cleanup boundary."""

import ctypes as C
import io
import json
from pathlib import Path
from runpy import run_path
import subprocess
import sys
import time
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from scripts.test_live_install_audit_env import load_audit


ROOT = Path(__file__).resolve().parent
OBSERVER = run_path(str(ROOT / "windows_owned_timeout.py"))["snapshot"]


class FakeJob:
    class Accounting(C.Structure):
        _fields_ = [("times", C.c_int64 * 4), ("faults", C.c_uint32),
                    ("total", C.c_uint32), ("active", C.c_uint32), ("terminated", C.c_uint32)]

    def __init__(self, mutation="clean", image=r"C:\private\agentplugins-source.exe"):
        self.C, self.handle = C, 100
        self.mutation, self.image = mutation, image
        self.closed, self.opened, self.terminations = [], [], []
        self.account_queries = self.member_queries = self.wait_queries = self.membership_queries = 0
        self.stopped = False

        def query(_job, kind, output, _size, _result):
            result = output._obj
            if kind == 1:
                self.account_queries += 1
                result.total = 2 if mutation == "generation" and self.account_queries >= 2 else 1
                result.active = 0 if self.stopped or mutation == "active" else 1
            else:
                self.member_queries += 1
                result.assigned = 65 if mutation == "oversized" else 2 if mutation in ("partial", "duplicate") else 1
                result.listed = 65 if mutation == "oversized" else 2 if mutation == "duplicate" else 1
                result.pids[0] = (0 if mutation == "zero-pid" else 0x100000000 if mutation == "large-pid"
                                  else 102 if mutation == "replaced" and self.member_queries >= 2 else 101)
                result.pids[1] = result.pids[0]
            return 1

        def opened(rights, inherit, pid):
            assert rights == 0x00101000 and inherit is False and pid == 101
            self.opened.append(pid)
            return 0 if mutation == "open" else 201

        def membership(handle, job, result):
            assert handle == 201 and job == 100
            self.membership_queries += 1
            result._obj.value = not (mutation == "foreign" or mutation == "late-foreign" and self.membership_queries >= 2)
            return 1

        def wait(handle, milliseconds):
            assert handle == 201 and (milliseconds == 0 or self.stopped and 0 <= milliseconds <= 5000)
            self.wait_queries += 1
            return 0xffffffff if mutation == "wait" else 0 if self.stopped or mutation == "exited" or mutation == "late-exit" and self.wait_queries >= 2 else 258

        def image_name(handle, flags, output, size):
            assert handle == 201 and flags == 0
            output.value = self.image
            size._obj.value = 32768 if mutation == "image-size" else len(self.image)
            return mutation != "image"

        def close(handle):
            self.closed.append(handle)
            return mutation != "close"

        def terminate(handle, code):
            self.terminations.append((handle, code))
            self.stopped = True
            return 1

        self.api = SimpleNamespace(QueryInformationJobObject=query, OpenProcess=opened,
                                   IsProcessInJob=membership, WaitForSingleObject=wait,
                                   QueryFullProcessImageNameW=image_name, CloseHandle=close,
                                   TerminateJobObject=terminate)

    @staticmethod
    def check(value):
        if not value:
            raise OSError("PRIVATE API diagnostic C:\\private\\secret --token=private")


class WindowsOwnedTimeoutDiagnosticTests(unittest.TestCase):
    def observe(self, job, *, started=None, deadline=None, bootstrap=None):
        now = time.monotonic()
        return OBSERVER(job, bootstrap or SimpleNamespace(poll=lambda: None),
                        r"C:\private\agentplugins-source.exe", now - 180 if started is None else started,
                        now + 5 if deadline is None else deadline)

    def test_fixed_schema_categories_and_no_private_identity_export(self):
        for image, expected in ((r"C:\private\agentplugins-source.exe", "command"),
                                (sys.executable, "python"), (r"C:\private\Claude.exe", "claude"),
                                (r"C:\private\node.exe", "node"), (r"C:\private\cmd.exe", "cmd"),
                                (r"C:\private\conhost.exe", "console"),
                                (r"C:\private\secret-token.exe", "other")):
            with self.subTest(expected=expected):
                job = FakeJob(image=image)
                result = self.observe(job)
                self.assertEqual(set(result), {"status", "qualified", "stage", "elapsed_ms",
                                               "bootstrap_state", "total_started", "active", "members"})
                self.assertEqual(result["status"], "observed-unqualified")
                self.assertIs(result["qualified"], False)
                self.assertEqual(result["members"], [{"image": expected, "state": "running"}])
                self.assertEqual((result["total_started"], result["active"]), (1, 1))
                self.assertLessEqual(abs(result["elapsed_ms"] - 180000), 1000)
                self.assertEqual(job.closed, [201])
                self.assertEqual(job.terminations, [])
                for private in ("private", "secret-token", "101", "201", "argv", "env", "pid"):
                    self.assertNotIn(private, json.dumps(result))

    def test_incomplete_racy_foreign_or_unreadable_observations_refuse_all_facts(self):
        for mutation in ("generation", "active", "partial", "oversized", "duplicate", "zero-pid",
                         "large-pid", "replaced", "open", "foreign", "late-foreign", "wait",
                         "late-exit", "exited", "image", "image-size", "close"):
            with self.subTest(mutation=mutation):
                job = FakeJob(mutation)
                result = self.observe(job)
                self.assertEqual(result, {"status": "refused", "qualified": False})
                self.assertEqual(job.closed, [] if not job.opened or mutation == "open" else [201])
                self.assertEqual(job.terminations, [])

    def test_deadline_and_bootstrap_failure_do_not_export_partial_facts(self):
        now = time.monotonic()
        job = FakeJob()
        self.assertEqual(self.observe(job, deadline=now - 1), {"status": "refused", "qualified": False})
        self.assertEqual(job.opened, [])
        def refused_poll():
            raise OSError("PRIVATE")
        job = FakeJob()
        self.assertEqual(self.observe(job, bootstrap=SimpleNamespace(poll=refused_poll)),
                         {"status": "refused", "qualified": False})
        self.assertEqual(job.closed, [201])

    def test_timeout_hook_preserves_termination_and_original_cleanup_deadline(self):
        for mutation in ("success", "failure", "exhausted"):
            with self.subTest(mutation=mutation):
                audit = load_audit()
                job, clock, waits = FakeJob(), [100.0], []
                child = SimpleNamespace(stdin=io.BytesIO(), returncode=None)
                def wait(timeout):
                    waits.append(timeout)
                    if len(waits) == 1:
                        clock[0] += 180
                        raise subprocess.TimeoutExpired(["private-argv"], timeout)
                    child.returncode = 97
                    return 97
                child.wait = wait
                expected = {"status": "observed-unqualified", "qualified": False}
                def observe(owned, bootstrap, executable, started, deadline):
                    self.assertIs(owned, job)
                    self.assertIs(bootstrap, child)
                    self.assertEqual((started, deadline), (100.0, 285.0))
                    if mutation == "failure":
                        raise OSError("PRIVATE SECRET")
                    if mutation == "exhausted":
                        clock[0] += 6
                        return {"status": "refused", "qualified": False}
                    return expected
                def helper(path):
                    return {"snapshot": observe} if str(path).endswith("windows_owned_timeout.py") else {"WindowsJob": lambda _child: job}
                with patch.object(audit, "run_path", side_effect=helper), \
                     patch.object(audit.subprocess, "Popen", return_value=child), \
                     patch.object(audit.subprocess, "CREATE_NEW_PROCESS_GROUP", 0, create=True), \
                     patch.object(time, "monotonic", side_effect=lambda: clock[0]):
                    if mutation == "exhausted":
                        with self.assertRaisesRegex(OSError, "cleanup deadline"):
                            audit._windows_file_call(["private-argv"], cwd="private", env={},
                                                     stdout=None, stderr=None, stdin=None, timeout=180, check=False)
                    else:
                        with self.assertRaises(subprocess.TimeoutExpired) as caught:
                            audit._windows_file_call(["private-argv"], cwd="private", env={},
                                                     stdout=None, stderr=None, stdin=None, timeout=180, check=False)
                        self.assertEqual(caught.exception.owned_timeout_diagnostic,
                                         expected if mutation == "success" else {"status": "refused", "qualified": False})
                self.assertEqual(job.terminations, [(100, 97)])
                self.assertIn(100, job.closed)
                self.assertEqual(len(waits), 2)
                self.assertLessEqual(waits[-1], 5)

    def test_timeout_report_retains_snapshot_and_never_parses_success(self):
        from tempfile import TemporaryDirectory
        audit = load_audit()
        error = subprocess.TimeoutExpired(["private-argv"], 180)
        error.owned_timeout_diagnostic = {"status": "observed-unqualified", "qualified": False}
        with TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            def timed_out(*_args, **kwargs):
                kwargs["stdout"].write(b'{"result":"success"}')
                raise error
            with patch.object(audit.sys, "platform", "win32"), \
                 patch.object(audit, "_windows_file_call", side_effect=timed_out):
                result = audit.call(["private-argv"], {"TEMP": str(root)}, root,
                                    timeout=180, capture_files=True)
        self.assertTrue(result["timeout"])
        self.assertIsNone(result["exit_code"])
        self.assertIsNone(result["_json"])
        self.assertEqual(result["owned_timeout_diagnostic"], error.owned_timeout_diagnostic)


if __name__ == "__main__":
    unittest.main()
