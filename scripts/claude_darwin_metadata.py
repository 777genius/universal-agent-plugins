"""Darwin held-FD metadata refusal; no writer-closure or ACL repair claim."""
import ctypes
import os
import platform
import sys

# Apple sys/fcntl.h public filesec_property_t, never private ACL structures.
FILESEC_UUID, FILESEC_ACL, FILESEC_GRPUUID = 3, 5, 6
# sys/stat.h and architecture typedefs: Darwin64 stat is 144 bytes/alignment 8.
# Reserve opaque aligned storage; no returned stat bytes are interpreted.
STAT_WORDS, STAT_BYTES = 128, 1024
ENOATTR = 93


def require(value):
    if not value:
        raise ValueError('Darwin cache metadata authority is unresolved')



class PresenceRefusal(ValueError):
    def __init__(self, presence):
        super().__init__('Darwin cache metadata authority is unresolved')
        self.presence = presence


def presence_diagnostic(error):
    """Project only the exact authored presence refusal, never foreign attributes."""
    if type(error) is not PresenceRefusal:
        return None
    current, terminal = error.__traceback__, None
    for _ in range(64):
        if current is None:
            break
        terminal = current.tb_frame.f_code
        current = current.tb_next
    else:
        return None
    if terminal is not Metadata.check.__code__:
        return None
    presence = error.presence
    if (type(presence) is not tuple or len(presence) != 3
            or any(type(value) is not bool for value in presence)
            or presence in ((True, True, False), (False, False, False))):
        return None
    return dict(zip(('uuid_present', 'group_uuid_present', 'acl_present'), presence))


class Metadata:
    def __init__(self):
        require(sys.platform == 'darwin' and platform.machine() in ('arm64', 'x86_64'))
        require(ctypes.sizeof(ctypes.c_void_p) == ctypes.sizeof(ctypes.c_long)
                == ctypes.sizeof(ctypes.c_longlong) == 8
                and ctypes.sizeof(ctypes.c_int) == 4 and ctypes.sizeof(ctypes.c_short) == 2
                and ctypes.alignment(ctypes.c_uint64) == 8)
        self.symbol = ('fstatx_np' if platform.machine() == 'arm64'
                       else 'fstatx_np$INODE64')
        self.lib = ctypes.CDLL('/usr/lib/libSystem.B.dylib', use_errno=True)
        self.init = self.lib.filesec_init
        self.init.argtypes, self.init.restype = [], ctypes.c_void_p
        self.free = self.lib.filesec_free
        self.free.argtypes, self.free.restype = [ctypes.c_void_p], None
        self.query = self.lib.filesec_query_property
        self.query.argtypes = [ctypes.c_void_p, ctypes.c_int, ctypes.POINTER(ctypes.c_int)]
        self.query.restype = ctypes.c_int
        try:
            self.stat = getattr(self.lib, self.symbol)  # No plain-symbol fallback.
        except AttributeError as error:
            raise ValueError('Darwin public stat symbol unavailable') from error
        self.stat.argtypes, self.stat.restype = [ctypes.c_int, ctypes.c_void_p, ctypes.c_void_p], ctypes.c_int
        self.list = self.lib.flistxattr
        self.list.argtypes, self.list.restype = [ctypes.c_int, ctypes.c_void_p, ctypes.c_size_t, ctypes.c_int], ctypes.c_ssize_t
        self.get = self.lib.fgetxattr
        self.get.argtypes = [ctypes.c_int, ctypes.c_char_p, ctypes.c_void_p,
                             ctypes.c_size_t, ctypes.c_uint32, ctypes.c_int]
        self.get.restype = ctypes.c_ssize_t

    def check(self, fd):
        # os.fstat supplies identity independently of the opaque public-stat ABI.
        before = os.fstat(fd)
        storage = (ctypes.c_uint64 * STAT_WORDS)()
        # Beyond the largest supported public struct, detect unexpected writes.
        for index in range(18, STAT_WORDS):
            storage[index] = 0xA51CE55ED15CA11E
        section = self.init()
        require(section)
        try:
            require(self.stat(fd, ctypes.byref(storage), section) == 0)
            require(all(storage[index] == 0xA51CE55ED15CA11E
                        for index in range(18, STAT_WORDS)))
            present = {}
            for property_id in (FILESEC_UUID, FILESEC_GRPUUID, FILESEC_ACL):
                valid = ctypes.c_int(-1)
                require(self.query(section, property_id, ctypes.byref(valid)) == 0
                        and valid.value >= 0)
                present[property_id] = bool(valid.value)
            # Apple XNU vn_stat_noauth returns KAUTH_FILESEC_NONE when the
            # filesystem exposes none of the three extended security fields.
            # Fresh ordinary APFS files/directories use this all-absent case.
            # The alternative has both UUID fields and an explicit NOACL;
            # a partial header or any reported ACL remains a refusal.
            accepted = ({FILESEC_UUID: False, FILESEC_GRPUUID: False, FILESEC_ACL: False},
                        {FILESEC_UUID: True, FILESEC_GRPUUID: True, FILESEC_ACL: False})
            if present not in accepted:
                presence = tuple(present[key] for key in (FILESEC_UUID, FILESEC_GRPUUID, FILESEC_ACL))
                try:
                    print('DARWIN_CACHE_PRESENCE=' + '{"uuid_present":'
                          + str(presence[0]).lower() + ',"group_uuid_present":'
                          + str(presence[1]).lower() + ',"acl_present":'
                          + str(presence[2]).lower() + '}', file=sys.stderr, flush=True)
                except Exception:
                    pass
                raise PresenceRefusal(presence)
            require(self.list(fd, None, 0, 0) == 0)
            ctypes.set_errno(0)
            size = self.get(fd, b'com.apple.ResourceFork', None, 0, 0, 0)
            error = ctypes.get_errno()
            require(size == -1 and error == ENOATTR)
            after = os.fstat(fd)
            require((before.st_dev, before.st_ino, before.st_mode, before.st_uid,
                     before.st_gid, before.st_nlink, before.st_size, before.st_mtime_ns,
                     before.st_ctime_ns)
                    == (after.st_dev, after.st_ino, after.st_mode, after.st_uid,
                        after.st_gid, after.st_nlink, after.st_size, after.st_mtime_ns,
                        after.st_ctime_ns))
        finally:
            self.free(section)


_METADATA = None


def check(fd):
    global _METADATA
    if _METADATA is None:
        _METADATA = Metadata()
    _METADATA.check(fd)
