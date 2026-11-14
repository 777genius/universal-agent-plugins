"""One complete, pinned Go distribution for the Devin Global build only.

This private input expires before the native fixture baseline. No shared Go
cache file is selected or changed. Unknown partial stages remain retained.
"""
from contextlib import ExitStack, contextmanager
import gzip
import hashlib
import json
import os
from pathlib import Path
import stat
import tarfile
import urllib.request

import project_native_research as shared
from devin_structured_inventory_admission import require

URL = "https://dl.google.com/go/go1.26.11.linux-amd64.tar.gz"
ARCHIVE_BYTES = 66999944
ARCHIVE_SHA256 = "9849f4e04ee35303714e34fbc3ac8ab44ea3e448a95aa182f91e3b9b79c710ec"
CATALOG_SHA256 = "d5e5ce38823a38bf7be2a4481d19431d4c8a9f5f6b328895239ffac385aa0c78"
GO_BYTES = 15548742
GO_SHA256 = "95499375c24b610cdc9e3684cec920174ec4c4f3f66d35b4332107f0f7d3d27d"
MEMBERS, DIRECTORIES, PAYLOAD = 16715, 1667, 232821816
MAX_ROOT_DEPTH, MAX_DEPTH, EXPANDED = 32, 15, 256 << 20
PAX_PATHS = {"go/test/fixedbugs/issue27836.dir/Þfoo.go", "go/test/fixedbugs/issue27836.dir/Þmain.go"}
ENVIRONMENT = "original-hosted-build; private GOROOT; GOTOOLCHAIN=local; GOENV=off"
NAME, ARCHIVE = "build-toolchain", "go.tar.gz"
DIRECTORY = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
REGULAR = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC


def identity(info):
    basic = (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid)
    return basic if stat.S_ISDIR(info.st_mode) else basic + (
        info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def catalog_digest(rows):
    body = json.dumps(sorted(rows, key=lambda row: row["name"]), sort_keys=True,
                      separators=(",", ":")).encode() + b"\n"
    return hashlib.sha256(body).hexdigest()


class ExpandedReader:
    def __init__(self, stream, budget):
        self.stream, self.budget, self.count = stream, budget, 0

    def read(self, count):
        self.budget.check()
        body = self.stream.read(min(count, EXPANDED + 1 - self.count))
        self.count += len(body)
        require(self.count <= EXPANDED, "Go expanded archive bound")
        return body


def download(budget):
    class FixedRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, fp, code, message, headers, newurl):
            require(newurl == URL, "Go archive redirect")
            return super().redirect_request(request, fp, code, message, headers, newurl)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), FixedRedirect())
    response = opener.open(URL, timeout=min(90, budget.check()))
    try:
        require(response.url == URL, "Go archive response origin")
        return response
    except BaseException:
        response.close()
        raise


class PrivateGo:
    def __init__(self, ctx):
        self.ctx, self.budget, self.stack = ctx, ctx.budget, ExitStack()
        self.path = ctx.root / NAME
        self.go = self.path / "go/bin/go"
        self.ancestors, self.entries, self.rows = [], {}, []
        self.root_fd, self.ready, self.receipt = None, False, None

    def hold(self, name, parent=None):
        fd = os.open(name, DIRECTORY, dir_fd=parent)
        self.stack.callback(os.close, fd)
        item = (fd, identity(os.fstat(fd)), parent, name)
        self.ancestors.append(item)
        return fd

    def roots(self):
        for fd, original, parent, name in self.ancestors:
            self.budget.check()
            require(identity(os.fstat(fd)) == original
                    and identity(os.stat(name, dir_fd=parent, follow_symlinks=False)) == original,
                    "Go original directory authority changed")

    def prepare(self):
        root = self.ctx.root
        require(root.is_absolute() and root.resolve(strict=True) == root
                and len(root.parts) <= MAX_ROOT_DEPTH
                and getattr(self.ctx, "global_build_cleanup_proved", None) is True,
                "Go private build root authority")
        parent = None
        for part in root.parts:
            parent = self.hold(part, parent)
        info = os.fstat(parent)
        require(identity(info) == tuple(self.ctx.original_output_identity)
                and stat.S_IMODE(info.st_mode) == 0o700
                and (info.st_uid, info.st_gid) == (os.getuid(), os.getgid()), "Go original build root")
        self.roots()
        # Set this before mkdir: a deadline may interrupt immediately after it.
        self.ctx.global_build_cleanup_proved = False
        try:
            os.mkdir(NAME, 0o700, dir_fd=parent)
        except BaseException:
            self.roots()
            try:
                os.stat(NAME, dir_fd=parent, follow_symlinks=False)
            except FileNotFoundError:
                self.ctx.global_build_cleanup_proved = True
            raise
        self.root_fd = self.hold(NAME, parent)
        require(os.fstat(self.root_fd).st_mode == stat.S_IFDIR | 0o700, "Go private stage mode")
        self.roots()
        with download(self.budget) as stream:
            self.write(ARCHIVE, stream, ARCHIVE_BYTES, 0o600, ARCHIVE_SHA256)
        self.extract()
        self.receipt = dict(schema_version=1, url=URL, archive_bytes=ARCHIVE_BYTES,
            archive_sha256=ARCHIVE_SHA256, catalog_sha256=CATALOG_SHA256, members=MEMBERS,
            directories=DIRECTORIES, payload_bytes=PAYLOAD, root=str(self.path),
            original_root=list(identity(os.fstat(self.root_fd))), verifications=0, cleanup_proved=False)
        self.verify()
        self.ready = True

    @contextmanager
    def parent(self, name):
        parts = name.split("/")
        require(len(parts) <= MAX_DEPTH, "Go member depth")
        with ExitStack() as opened:
            parent, prefix = self.root_fd, []
            for part in parts[:-1]:
                prefix.append(part)
                record = self.entries.get("/".join(prefix))
                require(record is not None and record[0]["kind"] == "directory", "Go member parent")
                fd = os.open(part, DIRECTORY, dir_fd=parent)
                opened.callback(os.close, fd)
                require(identity(os.fstat(fd)) == record[1]
                        and identity(os.stat(part, dir_fd=parent, follow_symlinks=False)) == record[1],
                        "Go member parent changed")
                parent = fd
            self.roots()
            yield parent, parts[-1]

    def write(self, name, stream, size, mode, expected=None):
        require(name not in self.entries and 0 <= size <= EXPANDED, "Go file bound")
        with self.parent(name) as (parent, leaf):
            fd = os.open(leaf, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC,
                         0o600, dir_fd=parent)
            try:
                digest, count = hashlib.sha256(), 0
                while count <= size:
                    self.budget.check()
                    chunk = stream.read(min(65536, size + 1 - count))
                    if not chunk: break
                    count += len(chunk)
                    require(count <= size, "Go member grew")
                    digest.update(chunk)
                    view = memoryview(chunk)
                    while view:
                        written = os.write(fd, view)
                        require(written > 0, "Go member write incomplete")
                        view = view[written:]
                require(count == size and (expected is None or digest.hexdigest() == expected), "Go member integrity")
                os.fchmod(fd, mode)
                original = identity(os.fstat(fd))
                require(original[:5] == (original[0], original[1], stat.S_IFREG | mode, os.getuid(), os.getgid())
                        and original[5:7] == (1, size)
                        and identity(os.stat(leaf, dir_fd=parent, follow_symlinks=False)) == original,
                        "Go created file authority")
                row = dict(name=name, kind="file", mode=mode, bytes=size, sha256=digest.hexdigest())
                self.entries[name] = (row, original)
                return row
            finally:
                os.close(fd)

    def extract(self):
        with self.parent(ARCHIVE) as (parent, leaf):
            fd = os.open(leaf, REGULAR, dir_fd=parent)
        with os.fdopen(fd, "rb") as archive:
            require(identity(os.fstat(fd)) == self.entries[ARCHIVE][1], "Go archive replaced")
            with gzip.GzipFile(fileobj=archive) as decoded:
                expanded = ExpandedReader(decoded, self.budget)
                with tarfile.open(fileobj=expanded, mode="r|") as tree:
                    for member in tree:
                        self.budget.check()
                        name = shared.member_name(member.name)
                        require((name == "go" or name.startswith("go/")) and name not in self.entries
                                and len(self.rows) < MEMBERS and not member.issparse() and not member.linkname
                                and (not member.pax_headers or name in PAX_PATHS and member.pax_headers == {"path": name})
                                and member.type in (tarfile.REGTYPE, tarfile.DIRTYPE)
                                and member.mode in (0o644, 0o755), "Go archive closed member")
                        if member.isdir():
                            require(member.size == 0 and member.mode == 0o755, "Go directory format")
                            with self.parent(name) as (parent, leaf):
                                os.mkdir(leaf, 0o755, dir_fd=parent)
                                fd = os.open(leaf, DIRECTORY, dir_fd=parent)
                                try:
                                    info = os.fstat(fd)
                                    require(stat.S_ISDIR(info.st_mode) and (info.st_uid, info.st_gid) == (os.getuid(), os.getgid())
                                            and identity(os.stat(leaf, dir_fd=parent, follow_symlinks=False)) == identity(info),
                                            "Go created directory")
                                    os.fchmod(fd, 0o755)
                                    row = dict(name=name, kind="directory", mode=0o755, bytes=0)
                                    self.entries[name] = (row, identity(os.fstat(fd)))
                                finally:
                                    os.close(fd)
                        else:
                            with tree.extractfile(member) as stream:
                                row = self.write(name, stream, member.size, member.mode)
                        self.rows.append(row)
                while expanded.read(65536): pass
            require(identity(os.fstat(archive.fileno())) == self.entries[ARCHIVE][1], "Go archive changed")
        require(len(self.rows) == MEMBERS and sum(row["kind"] == "directory" for row in self.rows) == DIRECTORIES
                and sum(row["bytes"] for row in self.rows) == PAYLOAD
                and catalog_digest(self.rows) == CATALOG_SHA256, "Go complete distribution catalog")

    def file(self, parent, leaf, record):
        row, original = record
        require(identity(os.stat(leaf, dir_fd=parent, follow_symlinks=False)) == original, "Go member changed")
        fd = os.open(leaf, REGULAR, dir_fd=parent)
        try:
            require(identity(os.fstat(fd)) == original, "Go member changed at open")
            digest, count = hashlib.sha256(), 0
            while count <= row["bytes"]:
                self.budget.check()
                chunk = os.read(fd, min(65536, row["bytes"] + 1 - count))
                if not chunk: break
                count += len(chunk); digest.update(chunk)
            require(count == row["bytes"] and digest.hexdigest() == row["sha256"]
                    and identity(os.fstat(fd)) == original
                    and identity(os.stat(leaf, dir_fd=parent, follow_symlinks=False)) == original,
                    "Go held member integrity changed")
        finally:
            os.close(fd)

    def walk(self, parent, prefix="", *, remove=False):
        seen = set()
        # Close the directory iterator before recursion. Owned descriptors are
        # bounded by32 original ancestors, two depth15 traversals, and4 leaves.
        with os.scandir(parent) as iterator:
            names = []
            for entry in iterator:
                self.budget.check()
                require(len(names) <= MEMBERS, "Go directory member bound")
                names.append(entry.name)
        for entry in names:
            self.budget.check()
            name = prefix + entry
            record = self.entries.get(name)
            require(record is not None and len(seen) <= MEMBERS, "Go unknown stage member")
            seen.add(name)
            if record[0]["kind"] == "file":
                self.file(parent, entry, record)
                if remove:
                    with self.parent(name) as (current, leaf):
                        require(identity(os.fstat(current)) == identity(os.fstat(parent)), "Go removal parent changed")
                        require(identity(os.stat(leaf, dir_fd=current, follow_symlinks=False)) == record[1], "Go removal member changed")
                        os.unlink(leaf, dir_fd=current)
            else:
                fd = os.open(entry, DIRECTORY, dir_fd=parent)
                try:
                    require(identity(os.fstat(fd)) == record[1], "Go directory changed")
                    seen.update(self.walk(fd, name + "/", remove=remove))
                    with self.parent(name) as (current, leaf):
                        require(identity(os.fstat(current)) == identity(os.fstat(parent))
                                and identity(os.stat(leaf, dir_fd=current, follow_symlinks=False)) == record[1],
                                "Go directory name changed")
                        if remove: os.rmdir(leaf, dir_fd=current)
                finally:
                    os.close(fd)
        return seen

    def verify(self):
        self.roots()
        require(self.walk(self.root_fd) == set(self.entries), "Go incomplete stage inventory")
        self.roots()
        self.receipt["verifications"] += 1

    def cleanup(self):
        require(self.ready and self.ctx.process_cleanup_proved is True, "Go build process cleanup unproved")
        self.verify()
        require(self.walk(self.root_fd, remove=True) == set(self.entries), "Go incomplete stage removal")
        self.roots()
        parent = self.ancestors[-1][2]
        os.rmdir(NAME, dir_fd=parent)
        for fd, original, ancestor, name in self.ancestors[:-1]:
            require(identity(os.fstat(fd)) == original
                    and identity(os.stat(name, dir_fd=ancestor, follow_symlinks=False)) == original,
                    "Go cleanup parent changed")
        try:
            os.stat(NAME, dir_fd=parent, follow_symlinks=False)
        except FileNotFoundError:
            self.ctx.global_build_cleanup_proved = True
            self.receipt["cleanup_proved"] = True
            return
        raise ValueError("Go cleanup absence unproved")


@contextmanager
def prepared(ctx):
    compiler = PrivateGo(ctx)
    try:
        compiler.prepare()
        try:
            yield compiler
        except BaseException:
            try:
                compiler.cleanup()
            except Exception:
                pass  # First refusal wins; false cleanup proof retains every owned root.
            raise
        else:
            compiler.cleanup()
    finally:
        compiler.stack.close()


def validate_receipt(value, executable):
    required = {"schema_version", "url", "archive_bytes", "archive_sha256", "catalog_sha256", "members",
                "directories", "payload_bytes", "root", "original_root", "verifications", "cleanup_proved"}
    require(type(value) is dict and set(value) == required and type(value["schema_version"]) is int
            and value["schema_version"] == 1 and value["url"] == URL
            and value["archive_sha256"] == ARCHIVE_SHA256 and value["catalog_sha256"] == CATALOG_SHA256
            and all(type(value[key]) is int and value[key] == expected for key, expected in (
                ("archive_bytes", ARCHIVE_BYTES), ("members", MEMBERS), ("directories", DIRECTORIES),
                ("payload_bytes", PAYLOAD), ("verifications", 6)))
            and value["cleanup_proved"] is True, "Go closed distribution receipt")
    root, original = value["root"], value["original_root"]
    require(type(root) is str and "\x00" not in root and Path(root).is_absolute() and str(Path(root)) == root
            and ".." not in Path(root).parts and len(Path(root).parts) <= MAX_ROOT_DEPTH + 1
            and Path(root).name == NAME and type(original) is list and len(original) == 5
            and all(type(number) is int and 0 <= number < 1 << 64 for number in original)
            and all(number > 0 for number in original[:2]) and original[2] == stat.S_IFDIR | 0o700
            and original[3:] == [os.getuid(), os.getgid()]
            and executable["path"] == str(Path(root) / "go/bin/go")
            and executable["bytes"] == GO_BYTES and executable["sha256"] == GO_SHA256
            and (executable["uid"], executable["gid"]) == (os.getuid(), os.getgid()), "Go original private root receipt")
