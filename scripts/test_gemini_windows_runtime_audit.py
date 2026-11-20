"""Portable binary guards and genuine Windows Gemini custody controls.

The Windows class never spoofs a platform and never invokes a vendor client.
Required filesystem/API failures on Windows are failures, not successful skips.
"""

import ctypes
import json
import os
from pathlib import Path
from runpy import run_path
import struct
import subprocess
import sys
import time
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from scripts import test_live_install_audit_env as _contracts

_BACKEND = run_path(str(Path(__file__).with_name("gemini_windows_custody.py")))
load_audit = _contracts.load_audit


def directory_record(name, file_index=37, attributes=32, following=False):
    encoded = name.encode("utf-16-le")
    length = (104 + len(encoded) + 7) & ~7
    body = bytearray(length)
    struct.pack_into("<I", body, 0, length if following else 0)
    struct.pack_into("<II", body, 56, attributes, len(encoded))
    struct.pack_into("<Q", body, 96, file_index)
    body[104:104 + len(encoded)] = encoded
    return bytes(body)


class WindowsGeminiBinaryContracts(unittest.TestCase):
    def test_directory_offsets_names_and_full_legacy_selector_are_bounded(self):
        parse, refuse = _BACKEND["parse_directory"], _BACKEND["Refusal"]
        high = (1 << 63) + 37
        good = directory_record("session.jsonl", high)
        self.assertEqual(parse(good), (("session.jsonl", 32, high),))
        self.assertEqual(len(parse(directory_record("first", following=True) + good)), 2)
        invalid = [b"", good[:103], b"\0" * 65537]
        for offset, format_, value in (
                (0, "<I", 8), (0, "<I", 105), (0, "<I", 65536),
                (60, "<I", 0), (60, "<I", 1), (60, "<I", 512),
                (96, "<Q", 0)):
            body = bytearray(good)
            struct.pack_into(format_, body, offset, value)
            invalid.append(bytes(body))
        for name in ("nul\0name", "nested/name", "nested\\name"):
            invalid.append(directory_record(name))
        invalid_utf16 = bytearray(directory_record("a"))
        invalid_utf16[104:106] = b"\0\xd8"
        invalid.append(bytes(invalid_utf16))
        invalid.append(directory_record("last", following=True))
        for index, body in enumerate(invalid):
            with self.subTest(index=index), self.assertRaises(refuse):
                parse(body)

    def test_parent_join_never_derives_legacy_id_from_full_128_bits(self):
        facts = _BACKEND["Facts"]
        child = facts(0x100000007, b"\x11" * 8 + b"\x22" * 8,
                      7, (1 << 63) + 37, 10, 11, 12, 5, 1, 32, False)
        parent = child._replace(file_id=b"\x33" * 16, file_index=5,
                                size=0, attributes=16, directory=True)
        entry = (child.attributes, child.file_index)
        _BACKEND["join_member"](parent, entry, child)
        for changed_entry, changed_child in (
                (None, child), ((32, 37), child), ((16, child.file_index), child),
                (entry, child._replace(volume=0x200000007)),
                (entry, child._replace(legacy_volume=8))):
            with self.subTest(entry=changed_entry), self.assertRaises(_BACKEND["Refusal"]):
                _BACKEND["join_member"](parent, changed_entry, changed_child)
        self.assertNotEqual(
            _BACKEND["identity"](child),
            _BACKEND["identity"](child._replace(file_id=b"\x11" * 8 + b"\x44" * 8)))
        self.assertNotEqual(
            _BACKEND["identity"](child),
            _BACKEND["identity"](child._replace(volume=0x200000007)))

    def test_directory_api_port_refuses_partial_errors_duplicates_and_round_exhaustion(self):
        # An inert binary-output port exercises no Windows constructor or syscall.
        def read(chunks, terminal=18):
            remaining, calls, last_error = list(chunks), [], [0]
            def query(_handle, kind, body, length):
                calls.append(kind)
                if remaining:
                    data = remaining.pop(0)
                    ctypes.memmove(body, data, len(data))
                    return 1
                last_error[0] = terminal
                return 0
            port = SimpleNamespace(
                _tick=lambda: None,
                ctypes=SimpleNamespace(create_string_buffer=ctypes.create_string_buffer,
                                       addressof=ctypes.addressof,
                                       get_last_error=lambda: last_error[0]),
                kernel=SimpleNamespace(GetFileInformationByHandleEx=query))
            return port, calls
        port, calls = read([directory_record("one")])
        self.assertEqual(_BACKEND["Custody"]._entries(port, 1), {"one": (32, 37)})
        self.assertEqual(calls, [11, 10])
        duplicate = directory_record("one", following=True) + directory_record("one")
        for chunks, terminal, expected in (
                ([duplicate], 18, "duplicate directory name"),
                ([directory_record("one"), directory_record("one")], 18, "duplicate directory name"),
                ([directory_record("one")], 5, "directory enumeration error 5"),
                ([directory_record("n" + str(index)) for index in range(64)], 18,
                 "directory enumeration round bound")):
            port, calls = read(chunks, terminal)
            with self.subTest(expected=expected), self.assertRaisesRegex(_BACKEND["Refusal"], expected):
                _BACKEND["Custody"]._entries(port, 1)
            if expected == "directory enumeration round bound":
                self.assertEqual(len(calls), 64)


@unittest.skipUnless(sys.platform == "win32" and os.name == "nt",
                     "genuine Windows NTFS custody controls")
class WindowsGeminiRuntimeContracts(unittest.TestCase):
    VERSION = _contracts.GeminiEmptySessionRuntimeContracts.VERSION
    fixture = _contracts.GeminiEmptySessionRuntimeContracts.fixture
    record = _contracts.GeminiEmptySessionRuntimeContracts.record
    phase = _contracts.GeminiEmptySessionRuntimeContracts.phase
    test_protected_profile_file_replacement_and_empty_directory_changes_refuse = (
        _contracts.GeminiEmptySessionRuntimeContracts
        .test_protected_profile_file_replacement_and_empty_directory_changes_refuse)
    test_strict_existing_metadata_negatives = getattr(
        _contracts.GeminiEmptySessionRuntimeContracts,
        "test_conversation_foreign_duplicate_partial_and_unbounded_records_refuse")

    def test_windows_registry_and_marker_require_exact_producer_spelling(self):
        for mutation in ("registry", "marker"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                project = str(case["project"])
                self.assertNotEqual(project, project.lower())
                case["observer"].close()
                if mutation == "registry":
                    (case["profile"] / "projects.json").write_text(
                        json.dumps({"projects": {project: "project"}}))
                else:
                    (case["chats"].parent / ".project_root").write_text(project)
                before = case["audit"].snapshot([case["profile"]])
                observer = case["audit"].GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
                    case["home"], case["project"], before, self.VERSION,
                    profile_snapshot=case["audit"].scoped_snapshot)
                try:
                    self.assertIsNotNone(observer.windows)
                    self.assertEqual(observer.reason,
                        "Gemini runtime registry does not bind the exact project"
                        if mutation == "registry" else "Gemini runtime ownership marker differs")
                    self.assertFalse(observer.finish(before))
                    if mutation == "registry":
                        self.assertIs(observer.evidence()["failure_facts"]["exact_key_present"], False)
                finally:
                    observer.close()

    def saved(self, backend):
        return ([handle for handle, _facts in backend.parents.values()],
                [fd for fd, _facts in backend.leaves.values()])

    def assert_closed(self, backend, handles, descriptors):
        self.assertFalse(backend.parents)
        self.assertFalse(backend.leaves)
        for descriptor in descriptors:
            with self.assertRaises(OSError):
                os.fstat(descriptor)
        for handle in handles:
            flags = ctypes.c_uint32()
            self.assertFalse(backend.kernel.GetHandleInformation(handle, ctypes.byref(flags)))

    def test_six_records_preserve_raw_windows_keys_and_close_noninherited_handles(self):
        with self.fixture() as case:
            observer = case["observer"]
            self.assertIsNotNone(observer.windows)
            for index in range(1, 5):
                self.record(case, index)
            self.phase(case, "after-public-repeat-add")
            self.assertIsNone(observer.reason)
            for index in (5, 6):
                self.record(case, index, lambda body: body[:-2] + b',"kind":"main"}\n')
            final = self.phase(case, "after-public-repeat-list")
            self.assertTrue(observer.finish(final), observer.reason)
            evidence = observer.evidence()
            self.assertEqual(evidence["platform"], "win32")
            self.assertEqual(evidence["status"], "metadata-only-new-records")
            self.assertEqual(len(evidence["files"]), 6)
            self.assertFalse(evidence["producer_attribution"])
            self.assertFalse(evidence["writer_closure"])
            self.assertEqual(observer.prefix, ".gemini/tmp\\project\\chats\\")
            for entry in evidence["files"]:
                self.assertIn(entry["path"], final)
                self.assertTrue(entry["path"].startswith(observer.prefix))
                self.assertEqual(entry["records"], 1)
                self.assertEqual(entry["conversation_records"], 0)
            self.assertNotEqual(final, case["before"])
            self.assertEqual(case["old"].read_bytes(),
                             b"pre-existing conversation must stay byte-for-byte\n")
            native = observer.windows
            self.assertTrue(native.parents)
            self.assertEqual(len(native.leaves), 9)
            for handle, facts in native.parents.values():
                native._noninherit(handle)
                self.assertEqual(len(facts.file_id), 16)
            for fd, facts in native.leaves.values():
                self.assertFalse(os.get_inheritable(fd))
                native._noninherit(native.msvcrt.get_osfhandle(fd))
                self.assertEqual(len(facts.file_id), 16)
            held = self.saved(native)
            observer.close()
            self.assert_closed(native, *held)
            observer.close()

    def test_real_file_rename_hardlink_and_foreign_delta_refuse(self):
        for mutation in ("old", "new", "registry", "deleted", "hardlink", "foreign"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                new = self.record(case, 1)
                self.phase(case, "after-public-repeat-add")
                self.assertIsNone(case["observer"].reason)
                if mutation in ("old", "new", "registry"):
                    path = (case["old"] if mutation == "old" else new if mutation == "new"
                            else case["profile"] / "projects.json")
                    body = path.read_bytes()
                    path.rename(path.with_name(path.name + ".held"))
                    path.write_bytes(body)
                    self.assertEqual(path.read_bytes(), body)
                elif mutation == "deleted":
                    new.unlink()
                elif mutation == "hardlink":
                    os.link(new, case["chats"] / "another-link")
                else:
                    (case["profile"] / "foreign-settings.json").write_text("{}")
                final = self.phase(case, "after-public-repeat-list")
                self.assertFalse(case["observer"].finish(final))
                self.assertIsNotNone(case["observer"].reason)

    def test_real_parent_rename_checks_denial_or_changed_namespace(self):
        with self.fixture() as case:
            native = case["observer"].windows
            self.assertIn(str(case["old"]), native.leaves)
            moved = case["chats"].with_name("held-chats")
            held = self.saved(native)
            denial = None
            try:
                case["chats"].rename(moved)
            except OSError as refused:
                denial = refused
            if denial is not None:
                self.assertIn(denial.winerror, (5, 32))
                self.assertTrue(case["chats"].is_dir())
                self.assertFalse(moved.exists())
                self.assertEqual(case["audit"].snapshot([case["profile"]]), case["before"])
                native.guard()
                branch = "denied with winerror " + str(denial.winerror)
            else:
                # This is a real namespace change, distinct from injected facts.
                self.assertFalse(case["chats"].exists())
                self.assertTrue(moved.is_dir())
                case["chats"].mkdir()
                with self.assertRaisesRegex(ValueError,
                                            "^Gemini Windows custody: "
                                            "(held name changed|current name identity)$"):
                    native.guard()
                branch = "real rename succeeded; changed namespace refused"
            case["observer"].close()
            self.assert_closed(native, *held)
            if denial is not None:
                case["chats"].rename(moved)
                self.assertFalse(case["chats"].exists())
            else:
                self.assertTrue(case["chats"].is_dir())
                self.assertEqual(list(case["chats"].iterdir()), [])
            self.assertEqual((moved / case["old"].name).read_bytes(),
                             b"pre-existing conversation must stay byte-for-byte\n")
            print("Windows parent rename safety branch: " + branch, flush=True)

    def test_real_existing_junction_is_refused_before_descendant_custody(self):
        with TemporaryDirectory(prefix="gemini-junction-&-") as temporary:
            root = Path(temporary).resolve()
            target, junction = root / "target", root / "junction"
            target.mkdir()
            witness = target / "foreign.txt"
            witness.write_bytes(b"foreign target must survive\n")
            # Create the reparse point before the first native custody handle.
            command = Path(os.environ["SystemRoot"]) / "System32/cmd.exe"
            result = subprocess.run(
                [str(command), "/d", "/c", "mklink", "/J", "junction", "target"],
                cwd=root, capture_output=True, timeout=10, check=False)
            self.assertEqual(result.returncode, 0, result.stderr[:256])
            self.assertTrue(junction.lstat().st_file_attributes & 0x400)
            deadline = time.monotonic() + 5
            probe = _BACKEND["Custody"](lambda: deadline)
            try:
                self.assertFalse(probe.parents)
                with self.assertRaisesRegex(_BACKEND["Refusal"], "nonordinary object"):
                    probe.hold(junction)
                self.assertNotIn(str(junction), probe.parents)
                self.assertFalse(probe.leaves)
                self.assertEqual(witness.read_bytes(), b"foreign target must survive\n")
            finally:
                held = self.saved(probe)
                probe.close()
            self.assert_closed(probe, *held)
            junction.rmdir()
            self.assertEqual(witness.read_bytes(), b"foreign target must survive\n")

    def test_same_bytes_swapped_during_the_real_os_read_are_refused(self):
        with self.fixture() as case:
            new = self.record(case, 1)
            original, read = new.read_bytes(), os.read
            interval = (case["phase_started"], case["clock"].now())
            final = case["audit"].snapshot([case["profile"]])
            changed = []
            def swap(fd, count):
                block = read(fd, count)
                selected = case["observer"].windows.leaves.get(str(new))
                if not changed and selected is not None and fd == selected[0]:
                    new.rename(new.with_name("held-old-session"))
                    new.write_bytes(original)
                    changed.append(True)
                return block
            with patch.object(os, "read", side_effect=swap):
                case["observer"].checkpoint("after-public-repeat-add", final,
                                            interval=interval)
            self.assertEqual(changed, [True])
            self.assertFalse(case["observer"].finish(final))
            self.assertRegex(case["observer"].reason,
                             "held file changed|held name changed|current name identity")

    def test_native_identity_and_membership_disagreements_refuse(self):
        # Facts are injected over real held handles, not filesystem replacements.
        for mutation in ("high-id", "volume", "parent-entry", "parent-id"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                native = case["observer"].windows
                fd, _expected = native.leaves[str(case["old"])]
                handle = (native.parents[str(case["chats"])][0] if mutation == "parent-id"
                          else native.msvcrt.get_osfhandle(fd))
                real_facts, real_entries = native._facts, native._entries
                def facts(current, directory):
                    value = real_facts(current, directory)
                    if current == handle:
                        if mutation in ("high-id", "parent-id"):
                            changed = value.file_id[:15] + bytes([value.file_id[15] ^ 1])
                            return value._replace(file_id=changed)
                        if mutation == "volume":
                            return value._replace(volume=value.volume ^ (1 << 32))
                    return value
                def entries(current):
                    value = real_entries(current)
                    if (mutation == "parent-entry"
                            and current == native.parents[str(case["chats"])][0]):
                        attributes, index = value[case["old"].name]
                        value[case["old"].name] = (attributes, index ^ 1)
                    return value
                with (patch.object(native, "_facts", side_effect=facts),
                      patch.object(native, "_entries", side_effect=entries)):
                    with self.assertRaisesRegex(ValueError, "held (file|parent) changed|parent/member identity"):
                        native.guard()

    def test_descriptor_transfer_failure_closes_only_its_untransferred_handle(self):
        with self.fixture() as case:
            new = self.record(case, 1)
            interval = (case["phase_started"], case["clock"].now())
            final = case["audit"].snapshot([case["profile"]])
            native, opened = case["observer"].windows, []
            original = native.kernel.OpenFileById
            def capture(*args):
                handle = original(*args)
                opened.append(handle)
                return handle
            with (patch.object(native.kernel, "OpenFileById", side_effect=capture),
                  patch.object(native.msvcrt, "open_osfhandle", side_effect=OSError("transfer control"))):
                case["observer"].checkpoint("after-public-repeat-add", final,
                                            interval=interval)
            self.assertEqual(len(opened), 1)
            self.assertFalse(case["observer"].finish(final))
            flags = ctypes.c_uint32()
            self.assertFalse(native.kernel.GetHandleInformation(opened[0], ctypes.byref(flags)))
            self.assertNotIn(str(new), native.leaves)
            held = self.saved(native)
            case["observer"].close()
            self.assert_closed(native, *held)

    def test_count_phase_version_and_literal_raw_path_guards_remain_required(self):
        with self.fixture() as case:
            for index in range(1, 18):
                self.record(case, index)
            final = self.phase(case, "after-public-repeat-add")
            self.assertFalse(case["observer"].finish(final))
            self.assertIn("count exceeds", case["observer"].reason)
        with self.fixture() as case:
            self.record(case, 1)
            final = self.phase(case, "after-public-repeat-list")
            self.assertFalse(case["observer"].finish(final))
            self.assertIn("phase sequence", case["observer"].reason)
        with self.fixture() as case:
            check = case["audit"].GEMINI_RUNTIME_AUDIT["pinned_version"]
            for version in (None, {}, {**self.VERSION, "value": "0.67.0"},
                            {**self.VERSION, "exit_code": False}, {**self.VERSION, "exit_code": 1}):
                self.assertFalse(check(version))
            self.record(case, 1)
            snapshot = case["audit"].snapshot([case["profile"]])
            key = next(key for key in snapshot if key not in case["before"])
            changed = {name: value for name, value in snapshot.items() if name != key}
            changed[key.replace("\\", "/")] = snapshot[key]
            case["observer"].checkpoint(
                "after-public-repeat-add", changed,
                interval=(case["phase_started"], case['clock'].now()))
            self.assertFalse(case["observer"].finish(changed))
            self.assertIn("outside exact list-session files", case["observer"].reason)
        with self.fixture() as case:
            nested = case["chats"] / "nested"
            nested.mkdir()
            (nested / "foreign.jsonl").write_bytes(b"foreign\n")
            snapshot = case["audit"].snapshot([case["profile"]])
            observer = case["audit"].GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
                case["home"], case["project"], snapshot, self.VERSION,
                profile_snapshot=case["audit"].scoped_snapshot)
            try:
                self.assertIn("not flat", observer.reason)
                self.assertFalse(observer.windows.parents)
                self.assertFalse(observer.windows.leaves)
            finally:
                observer.close()

    def test_public_repeat_gate_preserves_raw_delta_and_refuses_simulated_parent_identity_change(self):
        for changed_parent in (False, True):
            with self.subTest(changed_parent=changed_parent), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary).resolve()
                project = root / 'skill-00/project'
                profile = root / 'skill-00/home/.gemini'
                chats = profile / 'tmp/project/chats'
                skill = project / '.gemini/skills/owned-test/SKILL.md'
                # An inert argv marker handled only by the mocked call port.
                # No vendor CLI or replacement executable is run by this test.
                gemini = str(root / 'fake-gemini.exe')
                item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                        'source': 'unused', 'skill_md_git_blob_sha1': '0' * 40}
                matched = [{'installation_id': 'owned-id', 'name': 'owned-test',
                            'clients': [{'client_id': 'gemini'}]}]
                operations, held, native_calls, public_lists = [], [], [], []
                parent_changed, injected_parent_facts = [False], []
                real_observer = audit.GEMINI_RUNTIME_AUDIT['GeminiRepeatAudit']
                clock = _contracts.GeminiFixtureClock()
                def observe(*args, **kwargs):
                    observer = real_observer(*args, **kwargs)
                    self.assertIsNone(observer.reason)
                    self.assertIsNotNone(observer.windows)
                    native = observer.windows
                    parent_handle = native.parents[str(chats)][0]
                    real_facts, close = native._facts, native.close
                    def facts(current, directory):
                        value = real_facts(current, directory)
                        if parent_changed[0] and current == parent_handle:
                            # Simulated guard disagreement over a real held handle.
                            injected_parent_facts.append(current)
                            changed = value.file_id[:15] + bytes([value.file_id[15] ^ 1])
                            return value._replace(file_id=changed)
                        return value
                    native._facts = facts
                    def capture_close():
                        held.append((native, *self.saved(native)))
                        native._facts = real_facts
                        close()
                    native.close = capture_close
                    return observer
                def fake_call(argv, env, cwd, timeout=105):
                    operation = ('native-list' if argv[0] == gemini else
                                 argv[2] if argv[1] == 'skills' else argv[1])
                    operations.append(operation)
                    if operation == 'native-list':
                        self.assertEqual(argv, [gemini, 'skills', 'list', '--all'])
                        self.assertEqual(cwd, project)
                        self.assertEqual(timeout, 30)
                        native_calls.append(tuple(argv))
                        # Existing fixture emits millisecond timestamps inside this call.
                        self.record({'project': project, 'chats': chats, 'clock': clock}, 100, startup=True)
                    elif operation == 'add':
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text('---\nname: owned-test\n---\n')
                        (project / 'uap.toml').write_text('owned-test')
                        (project / 'uap.lock').write_text('owned-test')
                        if operations.count('add') == 1:
                            chats.mkdir(parents=True)
                            registry_project = str(project).lower()
                            (profile / 'projects.json').write_text(json.dumps({'projects': {registry_project: 'project'}}))
                            (chats.parent / '.project_root').write_text(registry_project)
                        elif changed_parent:
                            parent_changed[0] = True
                        else:
                            self.record({'project': project, 'chats': chats, 'clock': clock}, 1, startup=True)
                    elif operation == 'list':
                        public_lists.append(tuple(argv))
                        if len(public_lists) == 2 and not changed_parent:
                            self.record({'project': project, 'chats': chats, 'clock': clock}, 2, startup=True)
                    elif operation == 'remove':
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / 'uap.lock').unlink()
                    response = {'data': {'installations': [] if 'remove' in operations else matched}}
                    if operation == 'add':
                        response.update(readiness='ready', next_actions=[])
                    return {'exit_code': 0, 'stdout_tail': '', 'stderr_tail': '', 'timeout': False,
                            '_json': response}
                with (patch.dict(audit.AGENT_BINARIES, {'gemini': gemini}, clear=True),
                      patch.dict(audit.GEMINI_RUNTIME_AUDIT, {'GeminiRepeatAudit': observe}),
                      patch.dict(real_observer.__init__.__globals__, {'_now': clock.now}),
                      patch.object(audit, 'datetime', clock),
                      patch.object(audit, 'call', side_effect=fake_call),
                      patch.object(audit, 'matching_installations', return_value=matched),
                      patch.object(audit, 'locked_skill_is_pinned',
                                   side_effect=lambda *args: (project / 'uap.lock').exists()),
                      patch.object(audit, 'matches_pinned_skill_bytes', return_value=True)):
                    row = audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {},
                                        client_version=self.VERSION)
                self.assertEqual(operations, ['add', 'list', 'native-list', 'add', 'list', 'remove', 'list'])
                self.assertEqual(native_calls, [(gemini, 'skills', 'list', '--all')])
                self.assertEqual(len(public_lists), 3)
                baseline = row['gemini_native_baseline']
                self.assertEqual(baseline['operation'], 'native-skills-list-baseline')
                self.assertEqual(baseline['observation']['exit_code'], 0)
                self.assertFalse(baseline['observation']['timeout'])
                self.assertFalse(baseline['selected_skill_discovery_claimed'])
                proof = row['gemini_runtime_proof']
                self.assertEqual(proof['baseline']['records'], 3)
                self.assertEqual(proof['baseline']['started'], baseline['started'])
                self.assertEqual(proof['baseline']['finished'], baseline['finished'])
                self.assertFalse(proof['producer_attribution'])
                self.assertFalse(proof['writer_closure'])
                self.assertEqual(bool(injected_parent_facts), changed_parent)
                if changed_parent:
                    self.assertIn('held parent changed', proof['reason'])
                else:
                    self.assertEqual(len(proof['files']), 2)
                self.assertTrue(row['removal_verified'])
                self.assertTrue(row['sentinels_preserved'])
                self.assertEqual(row['repeat_native_unchanged'], changed_parent)
                self.assertEqual(row['repeat_native_runtime_only'], not changed_parent)
                self.assertEqual(row['repeat_verified'], not changed_parent)
                self.assertEqual(row['status'], 'repeat-add-unqualified' if changed_parent else 'passed')
                self.assertEqual(len(held), 1)
                self.assert_closed(*held[0])


if __name__ == "__main__":
    unittest.main()
