"""Read-only, bounded NTFS custody for the Gemini initial-record observer.

A directory entry's documented 64-bit NTFS ID selects OpenFileById. It is
joined to that same handle's legacy ID; the retained identity independently
includes the full 128-bit FileIdInfo and volume. No ID is truncated. This module
does not infer a writer, cleanup authority, ACL privacy or absence of streams.
"""

from collections import namedtuple
import os
from pathlib import Path
import struct
import sys
import time


class Refusal(ValueError):
    pass


Facts = namedtuple("Facts", "volume file_id legacy_volume file_index created "
                   "written changed size links attributes directory")
_BUFFER = 65536
_MAX_ENTRIES = 4096
_MAX_ROUNDS = 64


def _refuse(clause):
    raise Refusal("Gemini Windows custody: " + clause)


def parse_directory(body):
    """Decode FILE_ID_BOTH_DIR_INFO without interpreting platform ctypes."""
    if not isinstance(body, bytes) or not 104 <= len(body) <= _BUFFER:
        _refuse("directory buffer")
    rows, offset = [], 0
    while True:
        if offset % 8 or offset + 104 > len(body):
            _refuse("directory record boundary")
        following, = struct.unpack_from("<I", body, offset)
        attributes, length = struct.unpack_from("<II", body, offset + 56)
        file_index, = struct.unpack_from("<Q", body, offset + 96)
        end = offset + following if following else len(body)
        if (following and (following % 8 or following < 104)
                or end > len(body) or not 0 < length <= 510 or length % 2
                or offset + 104 + length > end or not file_index):
            _refuse("directory record fields")
        try:
            name = body[offset + 104:offset + 104 + length].decode("utf-16-le", "strict")
        except UnicodeError:
            _refuse("directory UTF-16")
        if any(character in name for character in "\x00/\\"):
            _refuse("directory component")
        rows.append((name, attributes, file_index))
        if len(rows) > _MAX_ENTRIES:
            _refuse("directory entry bound")
        if not following:
            return tuple(rows)
        offset = end


def identity(facts):
    # New children legitimately change a directory's size/write/change times.
    common = (facts.volume, facts.file_id, facts.legacy_volume, facts.file_index,
              facts.created, facts.attributes, facts.directory)
    return common if facts.directory else common + (
        facts.written, facts.changed, facts.size, facts.links)


def join_member(parent, entry, child):
    if (entry != (child.attributes, child.file_index)
            or child.volume != parent.volume
            or child.legacy_volume != parent.legacy_volume):
        _refuse("parent/member identity")


class Custody:
    """Keep native directory handles and CRT-owned file handles until close."""

    def __init__(self, deadline):
        if sys.platform != "win32" or os.name != "nt":
            _refuse("actual Windows required")
        import ctypes
        from ctypes import wintypes
        import msvcrt

        if (ctypes.sizeof(ctypes.c_void_p) != 8 or ctypes.sizeof(ctypes.c_wchar) != 2
                or ctypes.sizeof(wintypes.DWORD) != 4):
            _refuse("Windows ABI")
        if (type(getattr(os, "O_BINARY", None)) is not int or os.O_BINARY <= 0
                or not callable(getattr(msvcrt, "open_osfhandle", None))
                or not callable(getattr(msvcrt, "get_osfhandle", None))
                or not callable(getattr(os, "set_inheritable", None))):
            _refuse("CRT descriptor capability")
        self.ctypes, self.msvcrt, self.deadline = ctypes, msvcrt, deadline
        self.parents, self.leaves = {}, {}
        self.kernel = ctypes.WinDLL("kernel32", use_last_error=True)
        handle, dword, pointer = wintypes.HANDLE, ctypes.c_uint32, ctypes.c_void_p

        class FileIdentifier(ctypes.Union):
            _fields_ = [("index", ctypes.c_int64), ("storage", ctypes.c_ubyte * 16)]

        class Descriptor(ctypes.Structure):
            _fields_ = [("size", dword), ("kind", ctypes.c_int), ("value", FileIdentifier)]

        if (ctypes.sizeof(Descriptor) != 24 or ctypes.alignment(Descriptor) != 8
                or Descriptor.value.offset != 8):
            _refuse("file-ID descriptor ABI")
        self.Descriptor = Descriptor
        declarations = {
            "CreateFileW": (handle, [wintypes.LPCWSTR, dword, dword, pointer, dword, dword, handle]),
            "OpenFileById": (handle, [handle, ctypes.POINTER(Descriptor), dword, dword, pointer, dword]),
            "GetFileInformationByHandleEx": (wintypes.BOOL, [handle, ctypes.c_int, pointer, dword]),
            "GetFileInformationByHandle": (wintypes.BOOL, [handle, pointer]),
            "GetFinalPathNameByHandleW": (dword, [handle, wintypes.LPWSTR, dword, dword]),
            "GetFileType": (dword, [handle]),
            "GetDriveTypeW": (wintypes.UINT, [wintypes.LPCWSTR]),
            "GetVolumeInformationByHandleW": (
                wintypes.BOOL, [handle, wintypes.LPWSTR, dword, ctypes.POINTER(dword),
                                ctypes.POINTER(dword), ctypes.POINTER(dword),
                                wintypes.LPWSTR, dword]),
            "GetHandleInformation": (wintypes.BOOL, [handle, ctypes.POINTER(dword)]),
            "CloseHandle": (wintypes.BOOL, [handle]),
        }
        for name, (result, arguments) in declarations.items():
            try:
                function = getattr(self.kernel, name)
            except AttributeError:
                _refuse("required API " + name)
            function.restype, function.argtypes = result, arguments

    def _tick(self):
        if time.monotonic() > self.deadline():
            _refuse("checkpoint deadline")

    def _error(self, clause):
        error = self.ctypes.get_last_error()
        _refuse(clause + " error " + str(error))

    def _handle(self, value, clause):
        if value is None or value == self.ctypes.c_void_p(-1).value:
            self._error(clause)
        return value

    def _close_handle(self, handle):
        if not self.kernel.CloseHandle(handle):
            self._error("close")

    def _noninherit(self, handle):
        flags = self.ctypes.c_uint32()
        if not self.kernel.GetHandleInformation(handle, self.ctypes.byref(flags)):
            self._error("handle flags")
        if flags.value & 1:
            _refuse("inheritable handle")

    def _path(self, path):
        path = Path(path)
        try:
            path_bytes = len(str(path).encode("utf-16-le"))
        except UnicodeError:
            _refuse("path UTF-16")
        if (not path.is_absolute() or len(path.parts) > 64
                or len(path.drive) != 2 or not path.drive[0].isascii()
                or not path.drive[0].isalpha() or path.drive[1] != ":"
                or path.anchor != path.drive + "\\"
                or path_bytes > 8190):
            _refuse("local absolute path bound")
        for name in path.parts[1:]:
            if (not name or name in (".", "..") or name.endswith((" ", "."))
                    or any(character in name for character in "\x00/\\:")):
                _refuse("path component")
        return path

    def _name(self, handle):
        self._tick()
        body = self.ctypes.create_unicode_buffer(4096)
        length = self.kernel.GetFinalPathNameByHandleW(handle, body, len(body), 0)
        if length == 0:
            self._error("final name")
        if length >= len(body) or not body.value.startswith("\\\\?\\"):
            _refuse("final name bound")
        value = body.value[4:]
        self._path(value)
        return value

    def _info(self, handle, kind, length):
        self._tick()
        body = self.ctypes.create_string_buffer(length)
        if self.ctypes.addressof(body) % 8:
            _refuse("file information buffer alignment")
        if not self.kernel.GetFileInformationByHandleEx(handle, kind, body, length):
            self._error("file information " + str(kind))
        return body.raw

    def _facts(self, handle, directory):
        self._noninherit(handle)
        if self.kernel.GetFileType(handle) != 1:  # FILE_TYPE_DISK
            _refuse("non-disk handle")
        raw_id = self._info(handle, 18, 24)  # FileIdInfo
        volume, file_id = struct.unpack("<Q16s", raw_id)
        created, _accessed, written, changed, attributes = struct.unpack(
            "<qqqqI4x", self._info(handle, 0, 40))
        _allocated, size, links, pending, is_directory = struct.unpack(
            "<qqIBB2x", self._info(handle, 1, 24))
        tagged_attributes, reparse = struct.unpack("<II", self._info(handle, 9, 8))
        legacy = self.ctypes.create_string_buffer(52)
        if not self.kernel.GetFileInformationByHandle(handle, legacy):
            self._error("legacy file information")
        old = struct.unpack("<13I", legacy.raw)
        legacy_volume, file_index = old[7], (old[11] << 32) | old[12]
        if (raw_id != self._info(handle, 18, 24) or not volume or not any(file_id)
                or not legacy_volume or not file_index or created <= 0
                or written <= 0 or changed <= 0 or size < 0 or links < 1
                or pending != 0 or is_directory not in (0, 1)
                or bool(is_directory) != directory or bool(attributes & 0x10) != directory
                or attributes != tagged_attributes or attributes != old[0] or reparse
                or attributes & (0x400 | 0x40 | 0x1000 | 0x40000 | 0x400000)
                or created != ((old[2] << 32) | old[1])
                or not directory and (written != ((old[6] << 32) | old[5])
                                      or size != ((old[8] << 32) | old[9])
                                      or links != old[10] or links != 1)):
            _refuse("file information disagreement or nonordinary object")
        return Facts(volume, file_id, legacy_volume, file_index, created,
                     written, changed, size, links, attributes, directory)

    def _volume(self, handle, path, facts):
        self._tick()
        if self.kernel.GetDriveTypeW(path.anchor) != 3:  # DRIVE_FIXED
            _refuse("non-fixed volume")
        serial, maximum, flags = (self.ctypes.c_uint32() for _ in range(3))
        filesystem = self.ctypes.create_unicode_buffer(32)
        if not self.kernel.GetVolumeInformationByHandleW(
                handle, None, 0, self.ctypes.byref(serial), self.ctypes.byref(maximum),
                self.ctypes.byref(flags), filesystem, len(filesystem)):
            self._error("volume information")
        if (filesystem.value != "NTFS" or serial.value != facts.legacy_volume
                or not flags.value & 0x01000000):  # FILE_SUPPORTS_OPEN_BY_FILE_ID
            _refuse("non-NTFS volume or serial disagreement")

    def _named_handle(self, path, directory):
        self._tick()
        flags = 0x00200000 | (0x02000000 if directory else 0)
        return self._handle(self.kernel.CreateFileW(
            str(path), 0x80000000, 7, None, 3, flags, None), "named open")

    def _named(self, path, handle, expected):
        if self._name(handle) != str(path):
            _refuse("held name changed")
        opened = self._named_handle(path, expected.directory)
        try:
            if (self._name(opened) != str(path)
                    or identity(self._facts(opened, expected.directory)) != identity(expected)):
                _refuse("current name identity")
        finally:
            self._close_handle(opened)

    def _entries(self, handle):
        entries = {}
        for count in range(_MAX_ROUNDS):
            self._tick()
            body = self.ctypes.create_string_buffer(_BUFFER)
            if self.ctypes.addressof(body) % 8:
                _refuse("directory buffer alignment")
            success = self.kernel.GetFileInformationByHandleEx(
                handle, 11 if count == 0 else 10, body, len(body))
            if not success:
                error = self.ctypes.get_last_error()
                if error == 18:  # ERROR_NO_MORE_FILES
                    return entries
                _refuse("directory enumeration error " + str(error))
            for name, attributes, file_index in parse_directory(body.raw):
                if name in (".", ".."):
                    continue
                if name in entries:
                    _refuse("duplicate directory name")
                entries[name] = (attributes, file_index)
                if len(entries) > _MAX_ENTRIES:
                    _refuse("directory entry bound")
        _refuse("directory enumeration round bound")

    def _child(self, path, directory):
        parent_handle, parent_facts = self.parents[str(path.parent)]
        if identity(self._facts(parent_handle, True)) != identity(parent_facts):
            _refuse("parent changed before open")
        entry = self._entries(parent_handle).get(path.name)
        if entry is None:
            _refuse("child absent from held parent")
        descriptor = self.Descriptor()
        descriptor.size, descriptor.kind = self.ctypes.sizeof(descriptor), 0
        descriptor.value.index = self.ctypes.c_int64(entry[1]).value
        flags = 0x00200000 | (0x02000000 if directory else 0)
        handle = self._handle(self.kernel.OpenFileById(
            parent_handle, self.ctypes.byref(descriptor), 0x80000000, 7, None, flags), "ID open")
        try:
            facts = self._facts(handle, directory)
            join_member(parent_facts, entry, facts)
            self._named(path, handle, facts)
            if (self._entries(parent_handle).get(path.name) != entry
                    or identity(self._facts(parent_handle, True)) != identity(parent_facts)):
                _refuse("parent/member changed during open")
            return handle, facts
        except BaseException:
            self._close_handle(handle)
            raise

    def hold(self, path):
        path = self._path(path)
        for member in (*reversed(path.parents), path):
            key = str(member)
            if key in self.parents:
                continue
            if member == member.parent:
                handle = self._named_handle(member, True)
                try:
                    facts = self._facts(handle, True)
                    self._volume(handle, member, facts)
                    self._named(member, handle, facts)
                except BaseException:
                    self._close_handle(handle)
                    raise
            else:
                handle, facts = self._child(member, True)
            self.parents[key] = (handle, facts)
        self.guard()

    def guard(self):
        self._tick()
        entries = {}
        for path, (handle, expected) in self.parents.items():
            if identity(self._facts(handle, True)) != identity(expected):
                _refuse("held parent changed")
            self._named(Path(path), handle, expected)
            entries[path] = self._entries(handle)
        for path, (_handle, expected) in self.parents.items():
            member = Path(path)
            if member != member.parent:
                parent = str(member.parent)
                join_member(self.parents[parent][1], entries[parent].get(member.name), expected)
        for path, (fd, expected) in self.leaves.items():
            handle = self.msvcrt.get_osfhandle(fd)
            if identity(self._facts(handle, False)) != identity(expected):
                _refuse("held file changed")
            member = Path(path)
            self._named(member, handle, expected)
            parent = str(member.parent)
            join_member(self.parents[parent][1], entries[parent].get(member.name), expected)
        self._tick()

    def open_file(self, path, limit):
        path = self._path(path)
        self.guard()
        if (len(self.leaves) >= 82 or str(path) in self.leaves
                or str(path.parent) not in self.parents):
            _refuse("file count, duplicate or unheld parent")
        handle, facts = self._child(path, False)
        fd = None
        try:
            if not 0 < facts.size <= limit:
                _refuse("file size bound")
            fd = self.msvcrt.open_osfhandle(
                handle, os.O_RDONLY | os.O_BINARY)
            if fd < 0:
                fd = None
                _refuse("descriptor transfer")
            handle = None  # Only the CRT descriptor owns the native handle now.
            os.set_inheritable(fd, False)
            if os.get_inheritable(fd):
                _refuse("inheritable descriptor")
            self.leaves[str(path)] = (fd, facts)
        except BaseException:
            if fd is not None:
                os.close(fd)
            raise
        finally:
            if handle is not None:
                self._close_handle(handle)
        self.guard()
        return fd, facts.size

    def close(self):
        error = None
        for fd, _facts in self.leaves.values():
            try:
                os.close(fd)
            except OSError as failure:
                error = error or failure
        self.leaves.clear()
        for handle, _facts in reversed(tuple(self.parents.values())):
            try:
                self._close_handle(handle)
            except (OSError, Refusal) as failure:
                error = error or failure
        self.parents.clear()
        if error:
            raise error
