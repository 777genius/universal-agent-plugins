"""Actual inert process/capture contracts. These do not execute a Hermes bundle."""
import copy
import hashlib
import json
import os
from pathlib import Path
import runpy
import subprocess
import sys
import tempfile
import time
import unittest
from types import SimpleNamespace
from unittest import mock

import hermes_passive_history_control as control
import hermes_passive_history_processes as processes


CASE = runpy.run_path(str(Path(__file__).parent / "e2e/hermes-profile-admission.py"), run_name="history_contracts")
NONCE = "a" * 64


def child(directory, *, code=processes.SENTINEL, deadline=None):
    return processes.HeldChild([sys.executable, "-I", "-B", "-c", code, NONCE], directory,
                               dict(os.environ), deadline or time.monotonic() + 15)


def ledger_api():
    names = ("open_inventory_directory", "directory_identity", "same_opened_capture", "stable_file",
             "verify_inventory_directory", "check_inventory_directory")
    return SimpleNamespace(**{name: CASE[name] for name in names})


class HeldHistoryProcesses(unittest.TestCase):
    def test_real_original_sentinel_nonce_response_and_intentional_death(self):
        with tempfile.TemporaryDirectory() as temporary:
            held = child(temporary)
            try:
                for phase in control.CHALLENGES:
                    held.challenge(NONCE, phase)
                self.assertFalse(held.dead())
                report = held.close()
                self.assertTrue(held.reaped and held.closed)
                self.assertNotEqual(report["exit"], 0)
                rows = [processes.document(line) for line in processes.decoded(report["stdout"]).splitlines()]
                self.assertEqual([row["phase"] for row in rows], list(control.CHALLENGES))
                with self.assertRaisesRegex(RuntimeError, "authority lost"):
                    held.signal()
            finally:
                if not held.closed:
                    held.close()

    def test_exited_original_registrar_is_reserved_until_sole_final_wait(self):
        with tempfile.TemporaryDirectory() as temporary:
            held = child(temporary, code='print("{}", flush=True)')
            try:
                self.assertEqual(held.response(), {})
                self.assertTrue(held.wait_dead())
                self.assertIsNone(held.child.returncode)
                self.assertTrue(held.dead())
                self.assertEqual(held.close()["exit"], 0)
                self.assertTrue(held.child.stdout.closed and held.child.stderr.closed)
            finally:
                if not held.closed:
                    held.close()

    @unittest.skipIf(os.name == "nt", "waitpid external-reap control is POSIX only")
    def test_external_reap_refuses_without_any_later_numeric_signal(self):
        with tempfile.TemporaryDirectory() as temporary:
            held = child(temporary, code='print("{}", flush=True)')
            held.response()
            self.assertTrue(held.wait_dead())
            self.assertEqual(os.waitpid(held.child.pid, 0)[0], held.child.pid)
            with mock.patch.object(processes.os, "kill", wraps=os.kill) as signal:
                with self.assertRaisesRegex(RuntimeError, "wait authority lost"):
                    held.close()
                signal.assert_not_called()
            held.child.returncode = 0  # Test owns the already observed external reap.
            held.child.stdin.close()
            held.out.close(time.monotonic() + 1)
            held.err.close(time.monotonic() + 1)

    def test_poststart_constructor_error_closes_both_original_pipes_and_child(self):
        original_popen, original_capture, children = processes.subprocess.Popen, processes.PipeCapture, []
        def start(*args, **kwargs):
            result = original_popen(*args, **kwargs)
            children.append(result)
            return result
        count = 0
        def capture(*args):
            nonlocal count
            count += 1
            if count == 2:
                raise RuntimeError("actual second capture constructor fault")
            return original_capture(*args)
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(processes.subprocess, "Popen", start), \
                mock.patch.object(processes, "PipeCapture", capture):
            with self.assertRaisesRegex(RuntimeError, "second capture constructor"):
                child(temporary)
        self.assertEqual(len(children), 1)
        self.assertIsNotNone(children[0].returncode)
        self.assertTrue(all(stream.closed for stream in (children[0].stdin, children[0].stdout, children[0].stderr)))

    def test_first_pipe_error_still_settles_second_pipe_and_never_signals_reaped_child(self):
        with tempfile.TemporaryDirectory() as temporary:
            held = child(temporary, code='import os, time; os.write(1, b"x"*8193); time.sleep(30)')
            with self.assertRaisesRegex(RuntimeError, "overflow"):
                held.out.line(0, time.monotonic() + 3)
            with self.assertRaisesRegex(RuntimeError, "pipe closure incomplete"):
                held.close()
            self.assertTrue(held.reaped)
            self.assertTrue(held.child.stdout.closed and held.child.stderr.closed and held.child.stdin.closed)
            self.assertFalse(held.out.thread.is_alive() or held.err.thread.is_alive())
            with mock.patch.object(processes.os, "kill", wraps=os.kill) as signal:
                with self.assertRaisesRegex(RuntimeError, "pipe closure incomplete"):
                    held.close()
                signal.assert_not_called()

    def test_expired_active_cutoff_uses_only_original_reserved_cleanup_time(self):
        with tempfile.TemporaryDirectory() as temporary:
            deadline = time.monotonic() + 10.15
            held = child(temporary, deadline=deadline)
            time.sleep(.2)
            with self.assertRaisesRegex(RuntimeError, "original deadline expired"):
                held.challenge(NONCE, "ready")
            report = held.close()
            self.assertLess(time.monotonic(), deadline)
            self.assertEqual(held.deadline, deadline)
            self.assertTrue(report["waited"] and report["pipes_closed"])

    def test_real_open_pipe_cannot_publish_eof_until_original_writer_closes(self):
        read, write = os.pipe()
        capture = processes.PipeCapture(os.fdopen(read, "rb", buffering=0), 8)
        try:
            os.write(write, b"x")
            with self.assertRaisesRegex(RuntimeError, "EOF unproved"):
                capture.close(time.monotonic() + .03)
        finally:
            os.close(write)
        self.assertEqual(capture.close(time.monotonic() + 1), b"x")

    @unittest.skipUnless(os.name == "nt", "original Windows handle acquisition fault")
    def test_windows_identity_fault_after_start_uses_original_handle_cleanup(self):
        real, children = processes.windows_state, []
        def fault(original, original_identity=None, *, identity=True):
            if identity:
                children.append(original)
                raise RuntimeError("actual handle identity fault")
            return real(original, original_identity, identity=False)
        with tempfile.TemporaryDirectory() as temporary, mock.patch.object(processes, "windows_state", fault):
            with self.assertRaisesRegex(RuntimeError, "handle identity fault"):
                child(temporary)
        self.assertEqual(len(children), 1)
        self.assertIsNotNone(children[0].returncode)
        self.assertTrue(children[0]._handle.closed)


class OriginalLedgerCapture(unittest.TestCase):
    def test_prepare_holds_original_parent_before_add_without_recapture(self):
        with tempfile.TemporaryDirectory() as temporary:
            owner = object.__new__(control.HistoryControl)
            owner.private = Path(temporary).resolve() / "public-history"
            owner.project, owner.source = owner.private / "public-a", owner.private / "public-source-a"
            owner.evidence, owner.selected = owner.private / "evidence", owner.project / ".hermes/uap-session"
            owner.api, owner.held, owner.ledger = ledger_api(), [], None
            owner.api.ordinary_bytes, owner.api.public_stdio_recipe = CASE["ordinary_bytes"], lambda *_: {"command": "inert-probe"}
            owner.pins = {"uap": {"path": "inert-uap"}, "probe": {}}
            owner.probes = {"a": SimpleNamespace(url="http://127.0.0.1:9/mcp")}
            owner.nonce, owner.deadline = NONCE, time.monotonic() + 5
            proof, commands, original = None, [], []
            owner.phase, owner.snapshot = lambda *_: None, lambda: ({}, {"a": proof})

            def command(label, argv, **_):
                nonlocal proof
                commands.append(label)
                if label == "git":
                    return
                self.assertEqual(label, "add")
                self.assertTrue(owner.selected.parent.is_dir())
                self.assertFalse(owner.selected.exists())
                parents = [row for row in owner.held if row[0] == owner.selected.parent]
                self.assertEqual(len(parents), 1)
                original.append(parents[0])
                self.assertEqual(control.directory_facts(owner.selected.parent.lstat()), parents[0][2])
                self.assertEqual(control.directory_facts(os.fstat(parents[0][1])), parents[0][3])
                owner.selected.mkdir(mode=0o700)
                data = owner.project / "data"
                data.mkdir(mode=0o700)
                servers = {"hermes-public-a-http": {"url": owner.probes["a"].url},
                           "hermes-public-a-stdio": {"command": "inert-probe"}}
                (owner.selected / "config.yaml").write_bytes(json.dumps({"mcp_servers": servers}).encode())
                proof = {"data": data}

            owner.run_command = command
            try:
                with mock.patch.dict(sys.modules, {"hermes_yaml": SimpleNamespace(safe_load=json.loads)}):
                    owner.prepare()
                self.assertEqual(commands, ["git", "add"])
                self.assertEqual([row for row in owner.held if row[0] == owner.selected.parent], original)
                self.assertEqual(control.directory_facts(owner.selected.parent.lstat()), original[0][2])
                self.assertEqual(control.directory_facts(os.fstat(original[0][1])), original[0][3])
            finally:
                owner.close_handles()
            self.assertEqual(owner.held, [])

    def test_post_open_fstat_fault_closes_original_directory_descriptor(self):
        import errno
        real_fstat = os.fstat
        for role in ("directory", "ledger"):
            with self.subTest(role=role), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve(); path = root / "spawn-ledger.json"; path.write_bytes(b"[]")
                api, opened = ledger_api(), []
                original_open = api.open_inventory_directory
                def acquire(*args):
                    descriptor = original_open(*args); opened.append(descriptor); return descriptor
                def fault(descriptor):
                    if descriptor in opened: raise OSError("actual fstat fault after open")
                    return real_fstat(descriptor)
                api.open_inventory_directory = acquire
                owner = object.__new__(control.HistoryControl); owner.api = api; owner.held = []
                with mock.patch.object(os, "fstat", fault), self.assertRaisesRegex(OSError, "actual fstat fault"):
                    if role == "directory": owner.hold(root)
                    else: processes.LedgerHandle(api, path, b"[]", time.monotonic() + 2)
                self.assertEqual(len(opened), 1)
                with self.assertRaises(OSError) as error: real_fstat(opened[0])
                self.assertEqual(error.exception.errno, errno.EBADF)

    def test_earlier_status_mutation_during_later_stream_read_fails_final_sweep(self):
        for later in ("inert", "second"):
            with self.subTest(later=later), tempfile.TemporaryDirectory() as temporary:
                root = Path(temporary).resolve()
                names = ("bounded", "directory_identity", "held_census_evidence", "ordinary_bytes", "stable_file")
                api = SimpleNamespace(**{name: CASE[name] for name in names}, pins={},
                                      public_runtime_pins=lambda **_: {}, evidence_identity=CASE["directory_identity"](root))
                captures = {}
                for label in ("inert", "second"):
                    _, captures[label] = processes.capture_command(api, [sys.executable, "-I", "-B", "-c", "print('owned')"],
                        root, dict(os.environ), root, label, time.monotonic() + 25)
                original, changed = api.ordinary_bytes, []
                def read(path, *args, **kwargs):
                    result = original(path, *args, **kwargs)
                    if path.name == later + ".stdout":
                        status = root / "inert.status.json"
                        status.write_bytes(status.read_bytes() + b" "); changed.append(True)
                    return result
                api.ordinary_bytes = read
                with self.assertRaisesRegex(RuntimeError, "changed during receipt rejoin"):
                    processes.rejoin_commands(api, captures, root, time.monotonic() + 3)
                self.assertEqual(changed, [True])

    def test_cleanup_uses_first_held_private_identity_before_any_deletion(self):
        with tempfile.TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve(); private = outer / "public-history"; private.mkdir()
            (private / "original").write_bytes(b"original")
            owner = object.__new__(control.HistoryControl)
            owner.api, owner.private, owner.held, owner.ledger = ledger_api(), private, [], None
            owner.deadline = time.monotonic() + 5
            owner.hold(outer); owner.hold(private)
            displaced = outer / "original-private"
            try:
                try:
                    os.rename(private, displaced)
                except OSError as error:
                    self.assertEqual(os.name, "nt"); self.assertEqual(error.winerror, 32)
                    self.assertFalse(displaced.exists()); self.assertEqual((private / "original").read_bytes(), b"original")
                    self.assertEqual(control.directory_facts(private.lstat()), owner.held[-1][2])
                    self.assertEqual(control.directory_facts(os.fstat(owner.held[-1][1])), owner.held[-1][3])
                    owner.close_handles(); os.rename(private, displaced)
                    self.assertEqual((displaced / "original").read_bytes(), b"original")
                    return  # Actual Windows prevention and successful release, not substitution detection.
                os.rename(displaced, private)
                def replace():
                    os.rename(private, displaced); private.mkdir()
                    (private / "foreign").write_bytes(b"preserve substitute")
                owner.join = replace; owner.preserved = lambda **_: None; owner.report = lambda _: {}
                with mock.patch.object(control.shutil, "rmtree", side_effect=AssertionError("unowned delete attempted")) as delete:
                    with self.assertRaisesRegex(RuntimeError, "original owned cleanup root differs"):
                        owner.complete({"registrar": {"exit": 0}, "sentinel": {"exit": 1}})
                    delete.assert_not_called()
                self.assertEqual((private / "foreign").read_bytes(), b"preserve substitute")
                self.assertEqual((displaced / "original").read_bytes(), b"original")
            finally:
                owner.close_handles()

    def test_all_directory_close_attempts_survive_first_close_error(self):
        with tempfile.TemporaryDirectory() as temporary:
            owner = object.__new__(control.HistoryControl)
            owner.api, owner.held, owner.ledger = ledger_api(), [], None
            owner.hold(Path(temporary).resolve()); owner.hold(Path(temporary).resolve())
            descriptors = [row[1] for row in owner.held]
            close, seen = os.close, []
            def fault(descriptor):
                close(descriptor); seen.append(descriptor)
                if len(seen) == 1: raise OSError("actual first close fault")
            with mock.patch.object(os, "close", fault), self.assertRaisesRegex(RuntimeError, "handle closure incomplete"):
                owner.close_handles()
            self.assertEqual(set(seen), set(descriptors))
            for descriptor in descriptors:
                with self.assertRaises(OSError): os.fstat(descriptor)

    def test_real_held_ledger_rewrites_only_purpose_and_refuses_same_bytes_replacement(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            path = root / "spawn-ledger.json"
            body = b'[{"purpose":"mcp-helper","pid":23}]'
            path.write_bytes(body)
            replacement = root / "replacement"
            replacement.write_bytes(b'[{"pid":23,"purpose":"cli"}]')
            api = ledger_api()
            lease = processes.LedgerHandle(api, path, body, time.monotonic() + 5)
            try:
                original = path.stat()
                self.assertEqual(processes.document(lease.purpose("cli")), [{"purpose": "cli", "pid": 23}])
                self.assertEqual(path.stat().st_ino, original.st_ino)
                lease.verify()
                self.assertEqual(replacement.read_bytes(), lease.body)
                named, held = api.stable_file(path.lstat()), api.stable_file(os.fstat(lease.fd))
                candidate = replacement.lstat()
                try:
                    os.replace(replacement, path)
                except PermissionError as error:
                    self.assertEqual(os.name, "nt")
                    self.assertEqual(error.winerror, 5)
                    self.assertEqual(error.filename, str(replacement))
                    self.assertEqual(error.filename2, str(path))
                    self.assertEqual(api.stable_file(path.lstat()), named)
                    self.assertEqual(api.stable_file(os.fstat(lease.fd)), held)
                    self.assertEqual(api.stable_file(replacement.lstat()), api.stable_file(candidate))
                    self.assertEqual(path.read_bytes(), lease.body)
                    self.assertEqual(replacement.read_bytes(), lease.body)
                    lease.verify()
                    lease.close()
                    self.assertIsNone(lease.fd)
                    self.assertIsNone(lease.parent)
                    os.replace(replacement, path)
                    self.assertFalse(replacement.exists())
                    self.assertEqual(processes.physical(path.lstat()), processes.physical(candidate))
                    self.assertEqual(path.read_bytes(), lease.body)
                    return  # Kernel prevention followed by the same mutation after release.
                with self.assertRaisesRegex(RuntimeError, "inventory directory changed|authored ledger replaced"):
                    lease.verify()
            finally:
                lease.close()

    def test_named_hardlink_never_supplies_write_authority(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            source = root / "original"
            source.write_bytes(b"[]")
            path = root / "spawn-ledger.json"
            os.link(source, path)
            with self.assertRaisesRegex(RuntimeError, "ordinary authored ledger"):
                processes.LedgerHandle(ledger_api(), path, b"[]", time.monotonic() + 5)
            self.assertEqual(source.read_bytes(), b"[]")

    def test_actual_command_raw_status_and_streams_are_joined_before_return(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            names = ("bounded", "directory_identity", "held_census_evidence", "ordinary_bytes", "stable_file")
            api = SimpleNamespace(**{name: CASE[name] for name in names}, pins={},
                                  public_runtime_pins=lambda **_: {}, evidence_identity=CASE["directory_identity"](root))
            record, capture = processes.capture_command(api, [sys.executable, "-I", "-B", "-c", "print('original')"],
                root, dict(os.environ), root, "inert", time.monotonic() + 25)
            self.assertEqual(processes.decoded(record["stdout"]).strip(), b"original")
            processes.rejoin_commands(api, {"inert": capture}, root, time.monotonic() + 3)
            altered = copy.deepcopy(record)
            altered["status"]["pid"] = True
            with self.assertRaisesRegex(RuntimeError, "status bytes differ|status differs"):
                processes.validate_command(altered)
            (root / "inert.stdout").write_bytes(b"different")
            with self.assertRaisesRegex(RuntimeError, "receipt changed"):
                processes.rejoin_commands(api, {"inert": capture}, root, time.monotonic() + 3)

    def test_dry_run_controls_use_supported_run_arguments(self):
        with tempfile.TemporaryDirectory() as temporary:
            owner = object.__new__(control.HistoryControl)
            owner.outer = Path(temporary).resolve()
            owner.code = owner.outer / "inert-code"
            owner.project = owner.outer / "public-history/public-a"
            owner.selected = owner.project / ".hermes/uap-session"
            owner.pins, owner.nonce = {"uap": {"path": "inert-uap"}}, NONCE
            key = (owner.selected / control.LEDGER).relative_to(owner.outer).as_posix()
            owner.current, after = {key: "passive"}, {key: "active"}
            owner.baseline, owner.phases, owner.config = {}, [], {"owned-server": {}}
            receipt, calls, challenges, purposes = {"original": "receipt"}, [], [], []
            owner.forged_digest = control.digest(receipt)
            owner.history = lambda _image: (receipt, {})
            owner.snapshot = lambda: (after, {})
            owner.preserved = lambda: None
            owner.observer = lambda *_args: ["inert-selector"]
            owner.sentinel = SimpleNamespace(child=SimpleNamespace(pid=23),
                challenge=lambda nonce, phase: challenges.append((nonce, phase)))
            def purpose(value):
                purposes.append(value)
                return b"authored active purpose"
            owner.ledger = SimpleNamespace(purpose=purpose)
            expected = {"noninspect": ["run", "--dry-run", "--format", "json"],
                        "active": ["run", "--inspect", "--dry-run", "--format", "json"]}
            def command(label, argv, **options):
                calls.append(label)
                if label in expected:
                    self.assertEqual(argv, ["inert-uap", *expected[label], "--target", "hermes",
                                           "--scope", "project", "--project-root", str(owner.project)])
                    self.assertEqual(options, {"refused": True})
                    return {}
                value = owner.config
                if label == "selection":
                    value = dict(nonce=NONCE, pid=29, recorded=[23], selected=[23])
                return dict(status={"pid": 29}, stdout=processes.encoded(json.dumps(value).encode()))
            owner.run_command = command
            owner.exercise()
            self.assertEqual(calls, ["selection", "inspect", "noninspect", "active"])
            self.assertEqual(purposes, ["cli"])
            self.assertEqual(challenges, [(NONCE, phase) for phase in (
                "before-select", "after-select", "before-inspect", "after-inspect", "after-noninspect", "after-active")])
            self.assertEqual(owner.phases, [dict(label="active-authored", original_sha256=control.digest({}))])
            self.assertEqual(owner.current, after)

    def test_private_permission_policy_precedes_capture_and_preserves_original_history(self):
        with tempfile.TemporaryDirectory() as temporary:
            outer = Path(temporary).resolve()
            code = outer / "inert-code"
            code.mkdir()
            env = dict(HERMES_CALL_NONCE=NONCE, HERMES_BIN="inert-hermes", HERMES_PROFILE_PYTHON="inert-python",
                       HERMES_PROFILE_RUNTIME_ROOT=str(code))
            original = dict(env)
            helpers = {name: CASE[name] for name in control.HELPERS}
            helpers["public_spawn_install"] = lambda *_: (CASE["directory_identity"](code), "c" * 64)
            deadline = time.monotonic() + 180
            owner = control.HistoryControl(helpers, outer, {}, {}, env, {}, deadline)
            native_environments, permissions = [], []
            try:
                owner.selected.mkdir(parents=True, mode=0o700)
                owner.pins = {"uap": {"path": "inert-uap"}}
                owner.config = {"inert": {"url": "http://127.0.0.1:9/mcp"}}
                owner.accepted = {"data": outer / "inert-data"}
                (owner.selected / "config.yaml").write_bytes(b"mcp_servers: {}\n")
                phases = []
                def cold_command(label, argv, *, inspect=False):
                    self.assertEqual(label, "bootstrap")
                    self.assertEqual(argv, ["inert-uap", "run", "--inspect", "--confirm-client-start",
                        "--target", "hermes", "--scope", "project", "--project-root", str(owner.project)])
                    self.assertIs(inspect, True)
                    logs = owner.selected / "logs"
                    # Pinned make_user_dir defaults to mkdir(0777); the ordinary
                    # secure policy must run before this first directory capture.
                    previous = os.umask(0o022)
                    try:
                        logs.mkdir()
                    finally:
                        os.umask(previous)
                    if not owner.env.get("HERMES_SKIP_CHMOD"):
                        logs.chmod(0o700)
                    if os.name != "nt":
                        self.assertEqual(logs.stat().st_mode & 0o777, 0o700)
                    self.assertNotIn("HERMES_SKIP_CHMOD", owner.env)
                    phases.append("bootstrap")
                    owner.current = CASE["inventory"](outer)
                    return {"stdout": processes.encoded(json.dumps(owner.config).encode())}
                def profile(*_args, **_options):
                    self.assertNotIn("HERMES_SKIP_CHMOD", owner.env)
                    phases.append("profile")
                with mock.patch.object(owner, "run_command", side_effect=cold_command), \
                     mock.patch.object(owner.api, "public_profile", side_effect=profile):
                    owner.bootstrap()
                self.assertEqual(phases, ["bootstrap", "profile"])
                self.assertEqual(owner.env["HERMES_SKIP_CHMOD"], "1")
                self.assertEqual(env, original)
                path = owner.selected / control.LEDGER
                path.write_bytes(b"[]")
                owner.ledger = processes.LedgerHandle(ledger_api(), path, b"[]", deadline)
                owner.snapshot = lambda: (owner.current, {})
                def capture(_api, _argv, _cwd, environment, *_args, **_options):
                    native_environments.append(dict(environment))
                    # The pinned native directory policy performs this same-mode
                    # write unless its ordinary operator chmod-skip is selected.
                    if not environment.get("HERMES_SKIP_CHMOD"):
                        permissions.append("chmod")
                        os.chmod(owner.selected, 0o700)
                    return {}, None
                with mock.patch.object(control, "capture_command", capture):
                    owner.run_command("selection", ["inert-native-metadata"], native=True)
                owner.ledger.verify()
                self.assertEqual(permissions, [])
                self.assertEqual(native_environments, [dict(original, HERMES_SKIP_CHMOD="1",
                    AGENTPLUGINS_HOME=str(owner.private / "public-uap-state"), HERMES_HOME=str(owner.selected))])
                self.assertEqual(env, original)
                self.assertEqual(path.read_bytes(), b"[]")
            finally:
                owner.close_handles()
            for name in ("HERMES_HOME_MODE", "HERMES_UID", "HERMES_GID", "HERMES_SKIP_CHMOD"):
                for value in ("700", " ", None):
                    with self.subTest(name=name, value=value):
                        conflict = dict(env, **{name: value})
                        untouched = dict(conflict)
                        with mock.patch.object(control.HistoryControl, "hold", side_effect=AssertionError("captured before policy")), \
                             mock.patch.dict(helpers, public_spawn_install=mock.Mock(side_effect=AssertionError("runtime read before policy"))), \
                             self.assertRaisesRegex(RuntimeError, "private inspection permission policy"):
                            control.HistoryControl(helpers, outer, {}, {}, conflict, {}, deadline)
                        self.assertEqual(conflict, untouched)

    def test_original_live_capture_succeeds_but_oversized_retained_stream_refuses(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary).resolve()
            names = ("bounded", "directory_identity", "held_census_evidence", "ordinary_bytes", "stable_file")
            api = SimpleNamespace(**{name: CASE[name] for name in names}, pins={},
                                  public_runtime_pins=lambda **_: {}, evidence_identity=CASE["directory_identity"](root))
            argv = [sys.executable, "-I", "-B", "-c", "import sys; sys.stdout.buffer.write(b'x'*65537); sys.stdout.buffer.flush()"]
            with self.assertRaisesRegex(RuntimeError, "bounded ordinary single-link file required"):
                processes.capture_command(api, argv, root, dict(os.environ), root, "oversized", time.monotonic() + 25)
            status = json.loads((root / "oversized.status.json").read_bytes())
            self.assertEqual(status["exit"], 0)
            self.assertIsNone(status["error"])
            self.assertEqual(status["streams"]["stdout"]["bytes"], 65537)
            self.assertEqual((root / "oversized.stdout").stat().st_size, 65537)


if __name__ == "__main__":
    unittest.main()
