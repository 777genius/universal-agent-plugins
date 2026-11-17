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
    test_windows_registry_and_marker_require_exact_producer_spelling = (
        _contracts.GeminiEmptySessionRuntimeContracts
        .test_windows_registry_and_marker_require_exact_producer_spelling)
    phase = _contracts.GeminiEmptySessionRuntimeContracts.phase
    test_strict_existing_metadata_negatives = getattr(
        _contracts.GeminiEmptySessionRuntimeContracts,
        "test_conversation_foreign_duplicate_partial_and_unbounded_records_refuse")

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

    def test_real_rename_hardlink_junction_and_foreign_delta_refuse(self):
        for mutation in ("old", "new", "registry", "deleted", "parent",
                         "junction", "hardlink", "foreign"):
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
                elif mutation in ("parent", "junction"):
                    moved = case["chats"].with_name("held-chats")
                    case["chats"].rename(moved)
                    if mutation == "parent":
                        case["chats"].mkdir()
                    else:
                        command = Path(os.environ["SystemRoot"]) / "System32/cmd.exe"
                        result = subprocess.run(
                            [str(command), "/d", "/c", "mklink", "/J", str(case["chats"]), str(moved)],
                            capture_output=True, timeout=10, check=False)
                        self.assertEqual(result.returncode, 0, result.stderr[:256])
                        deadline = time.monotonic() + 5
                        probe = _BACKEND["Custody"](lambda: deadline)
                        try:
                            with self.assertRaisesRegex(ValueError, "nonordinary object"):
                                probe.hold(case["chats"])
                        finally:
                            probe.close()
                elif mutation == "hardlink":
                    os.link(new, case["chats"] / "another-link")
                else:
                    (case["profile"] / "foreign-settings.json").write_text("{}")
                final = self.phase(case, "after-public-repeat-list")
                self.assertFalse(case["observer"].finish(final))
                self.assertIsNotNone(case["observer"].reason)

    def test_same_bytes_swapped_during_the_real_os_read_are_refused(self):
        with self.fixture() as case:
            new = self.record(case, 1)
            original, read = new.read_bytes(), os.read
            changed = []
            def swap(fd, count):
                block = read(fd, count)
                if not changed:
                    new.rename(new.with_name("held-old-session"))
                    new.write_bytes(original)
                    changed.append(True)
                return block
            with patch.object(os, "read", side_effect=swap):
                final = self.phase(case, "after-public-repeat-add")
            self.assertEqual(changed, [True])
            self.assertFalse(case["observer"].finish(final))
            self.assertRegex(case["observer"].reason,
                             "held file changed|held name changed|current name identity")

    def test_native_identity_and_membership_disagreements_refuse(self):
        for mutation in ("high-id", "volume", "parent-entry"):
            with self.subTest(mutation=mutation), self.fixture() as case:
                native = case["observer"].windows
                fd, _expected = native.leaves[str(case["old"])]
                handle = native.msvcrt.get_osfhandle(fd)
                real_facts, real_entries = native._facts, native._entries
                def facts(current, directory):
                    value = real_facts(current, directory)
                    if current == handle:
                        if mutation == "high-id":
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
                    with self.assertRaisesRegex(ValueError, "held file changed|parent/member identity"):
                        native.guard()

    def test_descriptor_transfer_failure_closes_only_its_untransferred_handle(self):
        with self.fixture() as case:
            new = self.record(case, 1)
            native, opened = case["observer"].windows, []
            original = native.kernel.OpenFileById
            def capture(*args):
                handle = original(*args)
                opened.append(handle)
                return handle
            with (patch.object(native.kernel, "OpenFileById", side_effect=capture),
                  patch.object(native.msvcrt, "open_osfhandle", side_effect=OSError("transfer control"))):
                final = self.phase(case, "after-public-repeat-add")
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
            case["observer"].checkpoint("after-public-repeat-add", changed)
            self.assertFalse(case["observer"].finish(changed))
            self.assertIn("outside exact list-session files", case["observer"].reason)
        with self.fixture() as case:
            nested = case["chats"] / "nested"
            nested.mkdir()
            (nested / "foreign.jsonl").write_bytes(b"foreign\n")
            snapshot = case["audit"].snapshot([case["profile"]])
            observer = case["audit"].GEMINI_RUNTIME_AUDIT["GeminiRepeatAudit"](
                case["home"], case["project"], snapshot, self.VERSION)
            try:
                self.assertIn("not flat", observer.reason)
                self.assertFalse(observer.windows.parents)
                self.assertFalse(observer.windows.leaves)
            finally:
                observer.close()

    def test_public_repeat_gate_preserves_raw_delta_and_refuses_same_byte_parent_replacement(self):
        for replaced in (False, True):
            with self.subTest(replaced=replaced), TemporaryDirectory() as temporary:
                audit = load_audit()
                root = Path(temporary).resolve()
                project = root / 'skill-00/project'
                profile = root / 'skill-00/home/.gemini'
                chats = profile / 'tmp/project/chats'
                skill = project / '.gemini/skills/owned-test/SKILL.md'
                item = {'name': 'owned-test', 'repo': 'example/skills', 'folder': 'owned-test',
                        'source': 'unused', 'skill_md_git_blob_sha1': '0' * 40}
                matched = [{'installation_id': 'owned-id', 'name': 'owned-test',
                            'clients': [{'client_id': 'gemini'}]}]
                operations, held = [], []
                real_observer = audit.GEMINI_RUNTIME_AUDIT['GeminiRepeatAudit']
                def observe(*args):
                    observer = real_observer(*args)
                    self.assertIsNone(observer.reason)
                    self.assertIsNotNone(observer.windows)
                    native = observer.windows
                    close = native.close
                    def capture_close():
                        held.append((native, *self.saved(native)))
                        close()
                    native.close = capture_close
                    return observer
                def fake_call(argv, env, cwd, timeout=105):
                    operation = argv[2] if argv[1] == 'skills' else argv[1]
                    operations.append(operation)
                    if operation == 'add':
                        skill.parent.mkdir(parents=True, exist_ok=True)
                        skill.write_text('---\nname: owned-test\n---\n')
                        (project / 'uap.toml').write_text('owned-test')
                        (project / 'uap.lock').write_text('owned-test')
                        if operations.count('add') == 1:
                            chats.mkdir(parents=True)
                            (profile / 'projects.json').write_text(json.dumps({'projects': {str(project): 'project'}}))
                            (chats.parent / '.project_root').write_text(str(project))
                        elif replaced:
                            chats.rename(chats.with_name("held-chats"))
                            chats.mkdir()
                        else:
                            self.record({'project': project, 'chats': chats}, 1)
                    elif operation == 'list' and operations.count('list') == 2 and not replaced:
                        self.record({'project': project, 'chats': chats}, 2)
                    elif operation == 'remove':
                        skill.unlink()
                        skill.parent.rmdir()
                        (project / 'uap.lock').unlink()
                    return {'exit_code': 0, 'stdout_tail': '', 'stderr_tail': '', 'timeout': False,
                            '_json': {'data': {'installations': [] if 'remove' in operations else matched}}}
                with (patch.dict(audit.AGENT_BINARIES, {}, clear=True),
                      patch.dict(audit.GEMINI_RUNTIME_AUDIT, {'GeminiRepeatAudit': observe}),
                      patch.object(audit, 'call', side_effect=fake_call),
                      patch.object(audit, 'matching_installations', return_value=matched),
                      patch.object(audit, 'locked_skill_is_pinned',
                                   side_effect=lambda *args: (project / 'uap.lock').exists()),
                      patch.object(audit, 'matches_pinned_skill_bytes', return_value=True)):
                    row = audit.do_case('/unused/agentplugins', 'gemini', 'skill', item, 0, root, {},
                                        client_version=self.VERSION)
                self.assertEqual(operations, ['add', 'list', 'add', 'list', 'remove', 'list'])
                self.assertTrue(row['removal_verified'])
                self.assertTrue(row['sentinels_preserved'])
                self.assertEqual(row['repeat_native_unchanged'], replaced)
                self.assertEqual(row['repeat_native_runtime_only'], not replaced)
                self.assertEqual(row['repeat_verified'], not replaced)
                self.assertEqual(row['status'], 'repeat-add-unqualified' if replaced else 'passed')
                self.assertEqual(len(held), 1)
                self.assert_closed(*held[0])


if __name__ == "__main__":
    unittest.main()
