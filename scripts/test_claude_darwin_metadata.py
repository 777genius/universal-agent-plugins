"""Darwin metadata controls: portable API refusals and genuine macOS filesystem."""
import ctypes
import os
from pathlib import Path
import platform
from runpy import run_path
import subprocess
import sys
from tempfile import TemporaryDirectory
from types import SimpleNamespace
import unittest
from unittest.mock import patch

METADATA = run_path(str(Path(__file__).with_name('claude_darwin_metadata.py')))


class DarwinMetadataAPIContracts(unittest.TestCase):
    def subject(self, properties=None, list_size=0, fork_size=-1, error=93,
                stat_result=0, corrupt=False, query_result=0):
        # Pure API port: never construct a Darwin CDLL or spoof the host.
        subject = object.__new__(METADATA['Metadata'])
        subject.freed = []
        subject.init = lambda: 1
        subject.free = subject.freed.append
        def stat(fd, storage, section):
            if corrupt:
                ctypes.cast(storage, ctypes.POINTER(ctypes.c_uint64))[18] = 0
            return stat_result
        subject.stat = stat
        expected = {3: 4, 6: 8, 5: 0} if properties is None else properties
        def query(section, key, valid):
            ctypes.cast(valid, ctypes.POINTER(ctypes.c_int))[0] = expected[key]
            return query_result
        subject.query = query
        subject.list = lambda *args: list_size
        def get(*args):
            ctypes.set_errno(error)
            return fork_size
        subject.get = get
        return subject

    def check(self, subject):
        info = SimpleNamespace(st_dev=1, st_ino=2, st_mode=0o100600, st_uid=3,
                               st_gid=4, st_nlink=1, st_size=7, st_mtime_ns=8, st_ctime_ns=9)
        with patch.object(os, 'fstat', return_value=info):
            subject.check(99)

    def test_absent_header_and_explicit_noacl_keep_xattr_and_abi_guards(self):
        # XNU permits no extended security header on ordinary APFS files.
        # The actual macOS ACL-injection fixture below ensures a real ACL
        # cannot be misclassified as the all-absent case.
        for properties in (None, {3: 0, 6: 0, 5: 0}):
            with self.subTest(properties=properties):
                subject = self.subject(properties=properties)
                self.check(subject)
                self.assertEqual(subject.freed, [1])
        for properties in ({3: 0, 6: 1, 5: 0}, {3: 1, 6: 0, 5: 0},
                           {3: 1, 6: 1, 5: 1}, {3: 0, 6: 0, 5: 1},
                           {3: -1, 6: 1, 5: 0}):
            with self.subTest(properties=properties):
                subject = self.subject(properties=properties)
                with self.assertRaises(ValueError):
                    self.check(subject)
                self.assertEqual(subject.freed, [1])

    def test_api_errors_xattrs_and_abi_overwrite_refuse(self):
        variants = [dict(stat_result=-1), dict(query_result=-1), dict(corrupt=True),
                    dict(list_size=-1), dict(list_size=1), dict(fork_size=0),
                    dict(fork_size=1)]
        variants += [dict(error=error) for error in (0, 1, 5, 13, 45)]
        for variant in variants:
            with self.subTest(variant=variant):
                subject = self.subject(**variant)
                with self.assertRaises(ValueError):
                    self.check(subject)
                self.assertEqual(subject.freed, [1])


@unittest.skipUnless(sys.platform == 'darwin', 'genuine Darwin filesystem only')
class DarwinMetadataFilesystemContracts(unittest.TestCase):
    def setUp(self):
        self.temporary = TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name).resolve()
        self.subject = METADATA['Metadata']()

    def descriptor(self, path, directory=False):
        fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | (os.O_DIRECTORY if directory else 0))
        self.addCleanup(os.close, fd)
        return fd

    def test_fresh_file_directory_symbol_and_opaque_abi_canary(self):
        self.assertEqual(self.subject.symbol, 'fstatx_np' if platform.machine() == 'arm64'
                         else 'fstatx_np$INODE64')
        path = self.root / 'owned'
        path.write_bytes(b'ordinary owned bytes')
        self.subject.check(self.descriptor(path))
        self.subject.check(self.descriptor(self.root, True))

    def test_real_nonempty_acl_is_refused(self):
        path = self.root / 'owned'
        path.write_bytes(b'ordinary owned bytes')
        fd = self.descriptor(path)
        self.subject.check(fd)
        subprocess.run(['/bin/chmod', '+a', 'everyone allow read', str(path)],
                       check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=15)
        section = self.subject.init()
        self.assertTrue(section)
        try:
            storage = (ctypes.c_uint64 * METADATA['STAT_WORDS'])()
            self.assertEqual(self.subject.stat(fd, ctypes.byref(storage), section), 0)
            present = ctypes.c_int(-1)
            self.assertEqual(self.subject.query(section, METADATA['FILESEC_ACL'],
                                                ctypes.byref(present)), 0)
            self.assertGreater(present.value, 0)
        finally:
            self.subject.free(section)
        with self.assertRaises(ValueError):
            self.subject.check(fd)

    def test_real_resource_fork_and_other_xattr_are_refused(self):
        setter = self.subject.lib.fsetxattr
        setter.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_void_p,
                           ctypes.c_size_t, ctypes.c_uint32, ctypes.c_int]
        setter.restype = ctypes.c_int
        for index, (name, value) in enumerate(((b'com.apple.ResourceFork', b'fork'),
                                             (b'com.example.uap-custody', b'x'))):
            with self.subTest(name=name):
                path = self.root / ('owned-' + str(index))
                path.write_bytes(b'ordinary owned bytes')
                fd = self.descriptor(path)
                self.subject.check(fd)
                body = ctypes.create_string_buffer(value)
                self.assertEqual(setter(fd, name, body, len(value), 0, 0), 0)
                with self.assertRaises(ValueError):
                    self.subject.check(fd)


if __name__ == '__main__':
    unittest.main()
