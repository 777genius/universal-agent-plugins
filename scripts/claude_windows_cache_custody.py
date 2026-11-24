"""Windows-only checkpoint custody for a disposable Claude cache retirement.

The NTFS selector is the documented FILE_ID_BOTH_DIR_INFO 64-bit ID joined to
BY_HANDLE_FILE_INFORMATION, independently of the full volume64/FileIdInfo128
identity. All cache objects remain held across the intentional directory rename.
This proves a bounded checkpoint; it never proves writer closure.

Primary API contracts:
https://learn.microsoft.com/windows/win32/api/winbase/ns-winbase-file_id_descriptor
https://learn.microsoft.com/windows/win32/api/winbase/ns-winbase-file_stream_info
https://learn.microsoft.com/windows/win32/api/winbase/nf-winbase-getfileinformationbyhandleex
https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-findfirststreamw
https://learn.microsoft.com/windows/win32/secauthz/security-descriptors-for-new-objects
"""
import hashlib
import os
from pathlib import Path
from runpy import run_path
import struct
import time

_BASE = run_path(str(Path(__file__).with_name("gemini_windows_custody.py")))
Refusal = _BASE["Refusal"]
identity = _BASE["identity"]
join_member = _BASE["join_member"]
MAX_FILES, MAX_FILE_BYTES, MAX_TREE_BYTES, MAX_DEPTH = 10000, 64 << 20, 256 << 20, 64


def require(value):
    if not value:
        raise Refusal("Claude Windows cache custody unresolved")


def parse_streams(body):
    """Strict FILE_STREAM_INFO parser; trailing buffer capacity is not a record."""
    require(type(body) is bytes and 24 <= len(body) <= 65536)
    result, offset = [], 0
    while True:
        require(offset % 8 == 0 and offset + 24 <= len(body))
        following, length, size, allocated = struct.unpack_from("<IIqq", body, offset)
        end = offset + following if following else len(body)
        require((not following or following >= 24 and following % 8 == 0)
                and end <= len(body) and 0 < length <= 510 and length % 2 == 0
                and offset + 24 + length <= end and size >= 0 and allocated >= 0)
        name = body[offset + 24:offset + 24 + length].decode("utf-16-le", "strict")
        require("\x00" not in name and len(result) < MAX_FILES)
        result.append((name, size, allocated))
        if not following:
            return tuple(result)
        offset = end


class Native(_BASE["Custody"]):
    def __init__(self, deadline):
        super().__init__(deadline)
        c = self.ctypes
        from ctypes import wintypes as w
        self.security = c.WinDLL("advapi32", use_last_error=True)
        p, d, h, b = c.c_void_p, c.c_uint32, w.HANDLE, w.BOOL
        declarations = {
            "GetSecurityInfo": (d, [h, c.c_int, d, c.POINTER(p), c.POINTER(p),
                                     c.POINTER(p), c.POINTER(p), c.POINTER(p)]),
            "GetSecurityDescriptorControl": (b, [p, c.POINTER(c.c_uint16), c.POINTER(d)]),
            "IsValidSecurityDescriptor": (b, [p]),
            "IsValidAcl": (b, [p]), "GetAce": (b, [p, d, c.POINTER(p)]),
            "IsValidSid": (b, [p]), "GetLengthSid": (d, [p]),
            "ConvertSidToStringSidW": (b, [p, c.POINTER(p)]),
            "ConvertStringSecurityDescriptorToSecurityDescriptorW":
                (b, [w.LPCWSTR, d, c.POINTER(p), c.POINTER(d)]),
            "OpenProcessToken": (b, [h, d, c.POINTER(h)]),
            "GetTokenInformation": (b, [h, c.c_int, p, d, c.POINTER(d)]),
        }
        for name, (result, arguments) in declarations.items():
            function = getattr(self.security, name)
            function.restype, function.argtypes = result, arguments
        extra = {
            "LocalFree": (p, [p]), "GetCurrentProcess": (h, []),
            "CreateDirectoryW": (b, [w.LPCWSTR, p]),
            "ReadFile": (b, [h, p, d, c.POINTER(d), p]),
            "SetFilePointerEx": (b, [h, c.c_int64, c.POINTER(c.c_int64), d]),
        }
        for name, (result, arguments) in extra.items():
            function = getattr(self.kernel, name)
            function.restype, function.argtypes = result, arguments
        token = h()
        require(self.security.OpenProcessToken(self.kernel.GetCurrentProcess(), 8, c.byref(token)))
        try:
            body, length = c.create_string_buffer(65536), d()
            require(self.security.GetTokenInformation(token, 1, body, len(body), c.byref(length))
                    and c.sizeof(p) <= length.value <= len(body))
            self.user = self.sid(c.c_void_p.from_buffer(body).value)
        finally:
            self._close_handle(token)
        self.allowed = {self.user, "S-1-5-18", "S-1-5-32-544"}

    def sid(self, pointer):
        c = self.ctypes
        require(pointer and self.security.IsValidSid(pointer))
        require(8 <= self.security.GetLengthSid(pointer) <= 68)
        text = c.c_void_p()
        require(self.security.ConvertSidToStringSidW(pointer, c.byref(text)))
        try:
            return c.wstring_at(text.value)
        finally:
            require(not self.kernel.LocalFree(text))

    def acl(self, handle, private=False, protected=False):
        self._tick()
        c = self.ctypes
        owner, group, dacl, descriptor = (c.c_void_p() for _ in range(4))
        require(self.security.GetSecurityInfo(handle, 1, 7, c.byref(owner), c.byref(group),
                                             c.byref(dacl), None, c.byref(descriptor)) == 0)
        try:
            require(descriptor.value and self.security.IsValidSecurityDescriptor(descriptor)
                    and dacl.value and self.security.IsValidAcl(dacl))
            control, revision = c.c_uint16(), c.c_uint32()
            require(self.security.GetSecurityDescriptorControl(
                descriptor, c.byref(control), c.byref(revision)))
            require(revision.value == 1 and control.value & 4)
            own, primary = self.sid(owner.value), self.sid(group.value)
            header = c.string_at(dacl.value, 8)
            acl_revision, reserved, size, count, reserved2 = struct.unpack("<BBHHH", header)
            require(acl_revision in (2, 4) and not reserved and not reserved2
                    and 8 <= size <= 65535 and 0 < count <= 1024)
            body = c.string_at(dacl.value, size)
            seen, cursor = set(), 8
            for index in range(count):
                require(cursor + 8 <= size)
                kind, flags, length, mask = struct.unpack_from("<BBHI", body, cursor)
                require(length >= 16 and length % 4 == 0 and cursor + length <= size)
                ace = c.c_void_p()
                require(self.security.GetAce(dacl, index, c.byref(ace))
                        and ace.value == dacl.value + cursor)
                require(kind in (0, 1) and (not private or kind == 0))
                trustee = self.sid(ace.value + 8)
                require(8 + self.security.GetLengthSid(ace.value + 8) == length)
                if private:
                    require(kind == 0 and not flags & ~0x13 and trustee in self.allowed
                            and mask != 0)
                    if not flags & 8 and mask & 0x1F01FF == 0x1F01FF:
                        seen.add(trustee)
                cursor += length
            require(cursor == size)
            if private:
                require(own in self.allowed and self.user in seen)
            if protected:
                require(control.value & 0x1000)
            return own, primary, control.value, body.hex()
        finally:
            require(not self.kernel.LocalFree(descriptor))

    def streams(self, handle, facts):
        self._tick()
        c = self.ctypes
        body = c.create_string_buffer(65536)
        require(c.addressof(body) % 8 == 0)
        c.set_last_error(0)
        success = self.kernel.GetFileInformationByHandleEx(handle, 7, body, len(body))
        error = c.get_last_error()
        # Directories cannot have an unnamed $DATA stream. EOF is accepted only
        # for an empty directory-stream result, never for a file or an API error.
        if error == 38:
            require(facts.directory and not any(body.raw))
            return ()
        require(success and error == 0)
        rows = parse_streams(body.raw)
        require(not facts.directory and len(rows) == 1
                and rows[0][:2] == ("::$DATA", facts.size))
        return rows

    def qualified(self, handle, directory, private=True):
        facts = self._facts(handle, directory)
        require(not facts.attributes & ~0x20B7)
        acl = self.acl(handle, private)
        streams = self.streams(handle, facts)
        require(identity(self._facts(handle, directory)) == identity(facts))
        return facts, acl, streams

    def _entries(self, handle):
        entries = {}
        for count in range(64):
            self._tick()
            body = self.ctypes.create_string_buffer(65536)
            require(self.ctypes.addressof(body) % 8 == 0)
            if not self.kernel.GetFileInformationByHandleEx(
                    handle, 11 if count == 0 else 10, body, len(body)):
                if self.ctypes.get_last_error() == 18:
                    return entries
                self._error("Claude directory enumeration")
            for name, attributes, index in _BASE["parse_directory"](body.raw):
                if name in (".", ".."):
                    continue
                require(name not in entries and len(entries) < MAX_FILES)
                entries[name] = (attributes, index)
        require(False)

    def member(self, parent, expected, path, directory):
        entry = self._entries(parent).get(path.name)
        require(entry is not None and identity(self._facts(parent, True)) == identity(expected))
        descriptor = self.Descriptor()
        descriptor.size, descriptor.kind = self.ctypes.sizeof(descriptor), 0
        descriptor.value.index = self.ctypes.c_int64(entry[1]).value
        handle = self._handle(self.kernel.OpenFileById(
            parent, self.ctypes.byref(descriptor), 0x80000000, 7, None,
            0x00200000 | (0x02000000 if directory else 0)), "Claude member ID open")
        try:
            facts = self._facts(handle, directory)
            join_member(expected, entry, facts)
            self._named(path, handle, facts)
            require(self._entries(parent).get(path.name) == entry
                    and identity(self._facts(parent, True)) == identity(expected))
            return handle, facts
        except BaseException:
            self._close_handle(handle)
            raise

    def digest(self, handle, size):
        require(0 <= size <= MAX_FILE_BYTES)
        c = self.ctypes
        require(self.kernel.SetFilePointerEx(handle, 0, None, 0))
        digest, consumed = hashlib.sha256(), 0
        while True:
            self._tick()
            body, count = c.create_string_buffer(65536), c.c_uint32()
            require(self.kernel.ReadFile(handle, body, min(len(body), size - consumed + 1),
                                         c.byref(count), None))
            if not count.value:
                require(consumed == size)
                return digest.hexdigest()
            consumed += count.value
            require(consumed <= size)
            digest.update(body.raw[:count.value])


def fresh_case(path):
    """Atomically create a new private case; never alter an existing directory."""
    deadline = time.monotonic() + 5
    native = Native(lambda: deadline)
    c, descriptor = native.ctypes, native.ctypes.c_void_p()
    try:
        path = native._path(path)
        native.hold(path.parent)
        text = "O:" + native.user + "D:P" + "".join(
            "(A;OICI;FA;;;" + sid + ")" for sid in sorted(native.allowed))
        require(native.security.ConvertStringSecurityDescriptorToSecurityDescriptorW(
            text, 1, c.byref(descriptor), None))
        class Attributes(c.Structure):
            _fields_ = [("length", c.c_uint32), ("descriptor", c.c_void_p), ("inherit", c.c_int)]
        attributes = Attributes(c.sizeof(Attributes), descriptor, 0)
        require(native.kernel.CreateDirectoryW(str(path), c.byref(attributes)))
        native.hold(path)
        native.acl(native.parents[str(path)][0], True, True)
    finally:
        if descriptor.value:
            require(not native.kernel.LocalFree(descriptor))
        native.close()


class Backend:
    def __init__(self, case, profile, parent, cache, bundle):
        self.deadline = time.monotonic() + 5
        self.native = Native(lambda: self.deadline)
        self.nodes, self.anchors, self.sibling_nodes = {}, {}, {}
        self.case, self.parent, self.cache = case, parent, cache
        try:
            self.native.hold(parent)
            for path, (handle, _facts) in self.native.parents.items():
                if Path(path).is_relative_to(case):
                    facts, acl, streams = self.native.qualified(handle, True)
                    if Path(path) == case:
                        require(acl[2] & 0x1000)
                    self.anchors[path] = (acl, streams)
            self.root, facts = self.native._child(cache, True)
            self.nodes[""] = (self.root, facts)
            self.before_siblings = self.siblings()
            self.before, self.file_count, self.byte_count = self.snapshot(cache)
            require(self.file_count > 0 and ".orphaned_at" not in self.before)
            other = Native(lambda: self.deadline)
            try:
                other.hold(bundle)
                handle, facts = other.parents[str(bundle)]
                comparison, _, _ = self.snapshot(bundle, (other, handle, facts))
                require(self.content(comparison) == self.content(self.before))
            finally:
                other.close()
            self.guard()
        except BaseException:
            self.close()
            raise

    @staticmethod
    def content(snapshot):
        return {path: ("file", row[-2], row[-1]) if row[0] == "file" else ("dir",)
                for path, row in snapshot.items()}

    def siblings(self):
        handle, facts = self.native.parents[str(self.parent)]
        entries = self.native._entries(handle)
        require(all(attributes & 0x10 for attributes, _index in entries.values()))
        result = {}
        for name in entries:
            child, current = self.native.member(handle, facts, self.parent / name, True)
            retained = False
            try:
                if name in self.sibling_nodes:
                    held, original = self.sibling_nodes[name]
                    require(identity(current) == identity(original))
                    self.native._named(self.parent / name, held, original)
                elif not hasattr(self, "before_siblings") and name != self.cache.name:
                    self.sibling_nodes[name] = (child, current)
                    retained = True
                result[name] = identity(current)
            finally:
                if not retained:
                    self.native._close_handle(child)
        require(self.native._entries(handle) == entries)
        return result

    def guard(self):
        self.deadline = time.monotonic() + 5
        self.native.guard()
        for path, expected in self.anchors.items():
            handle, _facts = self.native.parents[path]
            _facts, acl, streams = self.native.qualified(handle, True)
            require((acl, streams) == expected and (Path(path) != self.case or acl[2] & 0x1000))

    def snapshot(self, root_path, external=None):
        self.deadline = time.monotonic() + 5
        native, root, root_facts = external or (self.native, self.root, self.nodes[""][1])
        result, files, total = {}, 0, 0
        def visit(handle, expected, path, relative, depth):
            nonlocal files, total
            native._tick()
            require(depth <= MAX_DEPTH and len(result) < MAX_FILES)
            facts, acl, streams = native.qualified(handle, expected.directory)
            require(identity(facts) == identity(expected))
            native._named(path, handle, facts)
            full = (facts.volume, facts.file_id.hex(), facts.legacy_volume,
                    facts.file_index, facts.created, facts.attributes, acl, streams)
            if not facts.directory:
                files += 1
                total += facts.size
                require(files <= MAX_FILES and total <= MAX_TREE_BYTES and facts.links == 1)
                digest = native.digest(handle, facts.size)
                row = ("file", full, facts.written, facts.changed, facts.links, facts.size, digest)
            else:
                row = ("dir", full)
            result[relative] = row
            before = native._entries(handle) if facts.directory else {}
            for name, (attributes, _index) in sorted(before.items()):
                require(name not in (".git", ".uap-integrations.lock", ".plugin-kit-ai.lock")
                        and name not in ("", ".", "..") and not name.endswith((" ", "."))
                        and not any(character in name for character in "\x00/\\:"))
                child_path = path / name
                child_relative = relative + "/" + name if relative else name
                directory = bool(attributes & 0x10)
                child, child_facts = native.member(handle, facts, child_path, directory)
                retained = False
                try:
                    if external is None:
                        if child_relative in self.nodes:
                            held, original = self.nodes[child_relative]
                            require(identity(child_facts) == identity(original))
                            native._named(child_path, held, original)
                        else:
                            require(len(self.nodes) < MAX_FILES)
                            self.nodes[child_relative] = (child, child_facts)
                            retained = True
                    visit(child, child_facts, child_path, child_relative, depth + 1)
                finally:
                    if not retained:
                        native._close_handle(child)
            require((not facts.directory or native._entries(handle) == before)
                    and native.qualified(handle, facts.directory) == (facts, acl, streams))
            native._named(path, handle, facts)
        visit(root, root_facts, root_path, "", 0)
        return result, files, total

    def retired(self):
        self.guard()
        after_siblings = self.siblings()
        self.after_siblings = after_siblings
        retained = set(after_siblings) - set(self.before_siblings)
        require(self.cache.name not in after_siblings and len(retained) == 1)
        nonce = retained.pop()
        import re
        require(re.fullmatch(r"\.agentplugins-cache-retire-[0-9a-f]{32}", nonce)
                and {k: v for k, v in self.before_siblings.items() if k != self.cache.name}
                == {k: v for k, v in after_siblings.items() if k != nonce})
        quarantine = self.parent / nonce
        self.native.hold(quarantine)
        handle, facts = self.native.parents[str(quarantine)]
        qualified, self.quarantine_acl, self.quarantine_streams = self.native.qualified(handle, True)
        require(identity(qualified) == identity(facts))
        require(set(self.native._entries(handle)) == {"retired"})
        moved = quarantine / "retired"
        current, current_facts = self.native.member(handle, facts, moved, True)
        try:
            require(identity(current_facts) == identity(self.nodes[""][1]))
            self.native._named(moved, self.root, self.nodes[""][1])
        finally:
            self.native._close_handle(current)
        after, _, _ = self.snapshot(moved)
        marker = after.pop(".orphaned_at", None)
        if marker is not None:
            require(marker[0] == "file" and marker[-2] == 13)
            marker_handle, _facts = self.nodes[".orphaned_at"]
            # The already hashed, held default stream is re-read with a 14-byte bound.
            require(marker[-1] == self.native.digest(marker_handle, 13))
            c = self.native.ctypes
            require(self.native.kernel.SetFilePointerEx(marker_handle, 0, None, 0))
            body, count = c.create_string_buffer(14), c.c_uint32()
            require(self.native.kernel.ReadFile(marker_handle, body, 14, c.byref(count), None)
                    and re.fullmatch(rb"[1-9][0-9]{12}", body.raw[:count.value]))
        require(after == self.before)
        return quarantine, moved, after, marker

    def recheck(self, quarantine, moved, marker):
        stable, _, _ = self.snapshot(moved)
        if marker is not None:
            require(stable.pop(".orphaned_at", None) == marker)
        require(stable == self.before)
        self.guard()
        handle, _facts = self.native.parents[str(quarantine)]
        _facts, acl, streams = self.native.qualified(handle, True)
        require(acl == self.quarantine_acl and streams == self.quarantine_streams
                and set(self.native._entries(handle)) == {"retired"}
                and self.siblings() == self.after_siblings)
        self.native._named(moved, self.root, self.nodes[""][1])

    def proof(self):
        facts = self.nodes[""][1]
        return {"cache_volume_serial": facts.volume, "cache_file_id_128": facts.file_id.hex(),
                "cache_ntfs_selector_64": facts.file_index}

    def close(self):
        native = getattr(self, "native", None)
        if native is None:
            return
        error = None
        for handle, _facts in reversed(tuple(self.nodes.values())):
            try:
                native._close_handle(handle)
            except (OSError, Refusal) as failure:
                error = error or failure
        self.nodes.clear()
        for handle, _facts in self.sibling_nodes.values():
            try:
                native._close_handle(handle)
            except (OSError, Refusal) as failure:
                error = error or failure
        self.sibling_nodes.clear()
        try:
            native.close()
        except (OSError, Refusal) as failure:
            error = error or failure
        if error:
            raise error
