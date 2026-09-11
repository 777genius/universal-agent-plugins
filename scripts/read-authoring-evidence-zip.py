#!/usr/bin/env python3
"""Bounded intake of a single provider authoring artifact; no trust inference.

The caller independently acquires the provider digest and exact file closure.
One immutable byte snapshot supplies both the digest check and every ZIP read.
Successful extraction is not native admission, attempt binding or qualification.
"""
import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import struct
import sys
import zipfile
import zlib

MAX_ARCHIVE = 2 * 1024**3
MAX_FILE = 128 * 1024**2
MAX_TOTAL = 2 * 1024**3
MAX_ENTRIES = 64
MAX_RATIO = 200
NATIVE_FILES = {
    "transcripts.json", "trees.json", "build-info.json", "preservation.json",
    "preparation.json", "host.json", "scans.json", "acquisition.json",
    "agentplugins-terminal.json", "plugin-kit-ai-terminal.json",
}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def safe_name(name):
    require(isinstance(name, str) and 0 < len(name) <= 240, "bounded member path required")
    parts = name.split("/")
    require(len(parts) <= 3, "member path depth")
    for part in parts:
        require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,150}", part) is not None,
                "noncanonical member path")
        require(not part.endswith(".") and part not in (".", ".."), "member alias")
        require(part.split(".")[0].upper() not in {
            "CON", "PRN", "AUX", "NUL", *(f"COM{i}" for i in range(10)),
            *(f"LPT{i}" for i in range(10)),
        }, "reserved member path")
    return name


def closure(kind, names):
    require(isinstance(names, list) and len(names) <= MAX_ENTRIES, "bounded file closure")
    for name in names:
        safe_name(name)
    require(len(set(n.lower() for n in names)) == len(names), "duplicate closure or alias")
    if kind == "native":
        require(set(names) == NATIVE_FILES, "exact native artifact closure")
    else:
        require(kind == "preparation" and len(names) == 20, "exact preparation artifact closure")
        require(set(n for n in names if not n.startswith(("agentplugins/", "plugin-kit-ai/"))) == {
            "candidate/candidate.json", "candidate-identity.json", "pair-prepared.json", "preparation-run.json",
        }, "preparation metadata closure")
        for product in ("agentplugins", "plugin-kit-ai"):
            members = [n for n in names if n.startswith(product + "/")]
            require(len(members) == 8 and all(n.count("/") == 1 for n in members), "eight product subjects")
            require({product + "/checksums.txt", product + "/release-manifest.json"} <= set(members),
                    "product metadata closure")
    return set(names)


def safe_directory(directory):
    require(directory.is_absolute(), "absolute directory required")
    for entry in (*reversed(directory.parents), directory):
        require(stat.S_ISDIR(entry.lstat().st_mode), "real directory ancestors required")


def checked_bytes(file, digest, size):
    require(re.fullmatch(r"[0-9a-f]{64}", digest) is not None and digest != "0" * 64, "exact ZIP SHA256")
    require(type(size) is int and 0 < size <= MAX_ARCHIVE, "ZIP size bound")
    file = Path(file)
    safe_directory(file.parent)
    before = file.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1, "regular unaliased ZIP required")
    with os.fdopen(os.open(file, os.O_RDONLY | os.O_NOFOLLOW), "rb") as handle:
        opened = os.fstat(handle.fileno())
        require((opened.st_dev, opened.st_ino) == (before.st_dev, before.st_ino), "ZIP identity changed")
        require(opened.st_size == size, "ZIP size mismatch")
        body = handle.read(size + 1)
        identity = lambda s: (s.st_dev, s.st_ino, s.st_size, s.st_mode, s.st_nlink, s.st_mtime_ns, s.st_ctime_ns)
        require(identity(os.fstat(handle.fileno())) == identity(opened) and identity(file.lstat()) == identity(before),
                "ZIP changed during read")
    require(len(body) == size and hashlib.sha256(body).hexdigest() == digest, "ZIP digest/size mismatch")
    return body


def check_expansion(body, start, info):
    # ZipExtFile truncates reads to declared file_size. Independently count the
    # complete deflate stream so a forged size/CRC cannot conceal an expansion.
    if info.compress_type == zipfile.ZIP_STORED:
        require(info.compress_size == info.file_size, "stored ZIP size mismatch")
        return
    decoder = zlib.decompressobj(-15)
    end, cursor, total, crc, pending = start + info.compress_size, start, 0, 0, b""
    while cursor < end or pending:
        if not pending:
            next_cursor = min(cursor + 65536, end)
            pending = memoryview(body)[cursor:next_cursor]
            cursor = next_cursor
        chunk = decoder.decompress(pending, min(1024 * 1024, info.file_size - total + 1))
        total += len(chunk)
        require(total <= info.file_size, "actual ZIP expansion exceeds declared size")
        crc = zlib.crc32(chunk, crc)
        pending = decoder.unconsumed_tail
        require(not decoder.unused_data, "trailing compressed ZIP data")
    require(decoder.eof and total == info.file_size and crc == info.CRC, "complete ZIP expansion and CRC required")


def extract(file, digest, size, output, kind, names):
    expected = closure(kind, names)
    body = checked_bytes(file, digest, size)
    # Bound central-directory allocation BEFORE ZipFile constructs its entry list.
    # Provider artifacts here require ordinary single-disk ZIP, no ZIP64/comments.
    require(len(body) >= 22 and body[-22:-18] == b"PK\x05\x06", "ordinary complete ZIP terminator required")
    disk, start_disk, count_disk, count, cd_size, cd_offset, comment = struct.unpack_from("<4H2LH", body, len(body) - 18)
    require(disk == start_disk == comment == 0 and count_disk == count and 0 < count <= MAX_ENTRIES,
            "ZIP entry count or disk bound")
    require(cd_size <= 65536 and cd_offset + cd_size == len(body) - 22, "exact bounded ZIP central directory")
    output = Path(output)
    safe_directory(output.parent)
    require(not os.path.lexists(output), "exclusive extraction destination required")
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        entries = archive.infolist()
        require(len(entries) == count, "ZIP central entry count mismatch")
        seen, files, directories, intervals = set(), set(), set(), []
        total = 0
        for info in entries:
            directory = info.is_dir()
            raw_name = info.filename[:-1] if directory else info.filename
            name = safe_name(raw_name)
            require(info.orig_filename == info.filename and name.lower() not in seen, "duplicate ZIP entry or alias")
            seen.add(name.lower())
            require(not info.extra and not info.comment, "ZIP extra metadata or alias unsupported")
            require(info.flag_bits & ~0x808 == 0 and info.compress_type in (zipfile.ZIP_STORED, zipfile.ZIP_DEFLATED),
                    "encrypted or unsupported ZIP encoding")
            mode = info.external_attr >> 16
            require(not mode & 0o7000 and stat.S_IFMT(mode) in (0, stat.S_IFDIR if directory else stat.S_IFREG),
                    "ZIP links or special file type")
            require(info.external_attr & 0x400 == 0, "ZIP reparse point")
            require(not (info.external_attr & 0x10) or directory, "ZIP directory alias")
            require(0 <= info.file_size <= MAX_FILE and info.file_size <= MAX_RATIO * max(1, info.compress_size),
                    "ZIP member size or ratio bound")
            total += info.file_size
            require(total <= MAX_TOTAL, "ZIP total size bound")
            if directory:
                require(info.file_size == 0 and any(n.startswith(name + "/") for n in expected), "unexpected ZIP directory")
                directories.add(name)
            else:
                require(name in expected and info.file_size > 0, "unexpected or empty ZIP subject")
                files.add(name)
            offset = info.header_offset
            require(0 <= offset and body[offset:offset + 4] == b"PK\x03\x04", "ZIP local header")
            flags, method = struct.unpack_from("<HH", body, offset + 6)
            crc, compressed, expanded = struct.unpack_from("<LLL", body, offset + 14)
            require((flags, method) == (info.flag_bits, info.compress_type), "ZIP local encoding mismatch")
            require((crc, compressed, expanded) == (info.CRC, info.compress_size, info.file_size) or
                    flags & 8 and (crc, compressed, expanded) == (0, 0, 0), "ZIP local size/CRC mismatch")
            name_size, extra_size = struct.unpack_from("<HH", body, offset + 26)
            require(extra_size == 0, "ZIP local extra metadata unsupported")
            start = offset + 30 + name_size + extra_size
            end = start + info.compress_size
            require(end <= cd_offset, "ZIP member overlaps central directory")
            check_expansion(body, start, info)
            if flags & 8:
                if body[end:end + 4] == b"PK\x07\x08":
                    end += 4
                require(struct.unpack_from("<LLL", body, end) == (info.CRC, info.compress_size, info.file_size),
                        "ZIP data descriptor mismatch")
                end += 12
            require(end <= cd_offset, "ZIP member overlaps central directory")
            intervals.append((offset, end))
        require(files == expected, "mismatched artifact file closure")
        require(not files & directories, "file/directory alias")
        intervals.sort()
        require(intervals[0][0] == 0 and intervals[-1][1] == cd_offset and
                all(a[1] == b[0] for a, b in zip(intervals, intervals[1:])),
                "overlapping, hidden or prefixed ZIP members")
        # Check local names, decompression, CRC and actual sizes before writing.
        for info in entries:
            with archive.open(info) as member:
                read = 0
                while chunk := member.read(1024 * 1024):
                    read += len(chunk)
                    require(read <= info.file_size, "expanded ZIP member exceeds size")
                require(read == info.file_size, "incomplete ZIP member")
        output.mkdir(mode=0o700)
        for name in sorted({str(Path(n).parent) for n in files} - {"."}):
            (output / name).mkdir(mode=0o700)
        for info in entries:
            if info.is_dir():
                continue
            destination = output / info.filename
            safe_directory(destination.parent)
            with archive.open(info) as member, os.fdopen(os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600), "wb") as target:
                while chunk := member.read(1024 * 1024):
                    target.write(chunk)
        # A write failure retains this exclusive incomplete root as diagnostics;
        # it never gets reused, repaired in place, or treated as admitted evidence.
    return {"archive_sha256": digest, "files": sorted(files)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", required=True)
    parser.add_argument("--sha256", required=True)
    parser.add_argument("--size", required=True, type=int)
    parser.add_argument("--output", required=True)
    parser.add_argument("--kind", required=True, choices=("native", "preparation"))
    parser.add_argument("--files", required=True)
    args = parser.parse_args()
    require(len(args.files) <= 8192, "bounded file closure JSON")
    print(json.dumps(extract(args.archive, args.sha256, args.size, args.output, args.kind, json.loads(args.files))))


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, zipfile.BadZipFile, EOFError, struct.error, zlib.error) as error:
        print(f"authoring evidence ZIP rejected: {str(error)[:500]}", file=sys.stderr)
        sys.exit(1)
