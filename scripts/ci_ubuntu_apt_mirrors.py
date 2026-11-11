#!/usr/bin/env python3
"""Prefer already configured canonical HTTPS Ubuntu mirrors, without adding sources.

APT mirror priorities: https://manpages.debian.org/testing/apt/apt-transport-mirror.1.en.html
Only priority metadata changes; repository trust and Playwright dependencies stay intact.
"""
from contextlib import ExitStack
import os
from pathlib import Path
import re
import stat
import sys

ALLOWED = {
    "http://azure.archive.ubuntu.com/ubuntu",
    "http://archive.ubuntu.com/ubuntu",
    "https://archive.ubuntu.com/ubuntu",
    "http://security.ubuntu.com/ubuntu",
    "https://security.ubuntu.com/ubuntu",
}
LISTS = (
    (Path("/etc/apt/apt-mirrors.txt"), "https://archive.ubuntu.com/ubuntu", True),
    (Path("/etc/apt/apt-security-mirrors.txt"), "https://security.ubuntu.com/ubuntu", False),
)
LIMIT = 65536


class MirrorError(ValueError):
    """Refuse an unexpected mirror configuration without disclosing its content."""


def prioritize(text, preferred):
    """Preserve nonpreferred groups and all bytes other than priority metadata."""
    lines = text.splitlines(keepends=True)
    entries = []
    seen = set()
    for index, line in enumerate(lines):
        body = line.rstrip("\r\n")
        if not body.strip() or body.lstrip().startswith("#"):
            continue
        head, separator, metadata = body.partition("\t")
        uri = head.strip().rstrip("/")
        if uri not in ALLOWED:
            raise MirrorError("unrecognized mirror URI")
        if uri in seen:
            raise MirrorError("duplicate mirror URI")
        seen.add(uri)
        matches = list(re.finditer(r"(?<!\S)priority:(\S*)", metadata))
        if len(matches) > 1:
            raise MirrorError("duplicate priority key")
        priority = None
        if matches:
            value = matches[0].group(1)
            if not value.isascii() or not value.isdecimal():
                raise MirrorError("invalid mirror priority")
            try:
                priority = int(value)
            except ValueError:
                raise MirrorError("invalid mirror priority") from None
        entries.append((index, body, line[len(body):], head, separator, uri, priority, matches))
    if preferred not in seen:
        raise MirrorError("configured canonical HTTPS mirror is missing")
    groups = sorted({entry[6] for entry in entries if entry[5] != preferred and entry[6] is not None})
    ranks = {value: index + 1 for index, value in enumerate(groups)}
    for index, body, ending, head, separator, uri, priority, matches in entries:
        if uri == preferred:
            selected = 0
        elif priority is None:
            continue
        else:
            selected = ranks[priority]
        if matches:
            match = matches[0]
            offset = len(head) + len(separator)
            body = body[:offset + match.start(1)] + str(selected) + body[offset + match.end(1):]
        else:
            body += (" " if separator else "\t") + "priority:" + str(selected)
        lines[index] = body + ending
    return "".join(lines)


def check_file(handle):
    info = os.fstat(handle.fileno())
    if not stat.S_ISREG(info.st_mode) or info.st_uid != 0 or info.st_nlink != 1 or info.st_mode & 0o022:
        raise MirrorError("mirror list must be a protected root-owned regular file")


def main():
    if os.geteuid() != 0:
        raise MirrorError("root is required to prepare system mirror lists")
    with ExitStack() as stack:
        plans = []
        for path, preferred, required in LISTS:
            try:
                descriptor = os.open(path, os.O_RDWR | os.O_NOFOLLOW | os.O_CLOEXEC)
            except FileNotFoundError:
                if required:
                    raise MirrorError("required Ubuntu mirror list is missing") from None
                continue
            handle = stack.enter_context(os.fdopen(descriptor, "r+", encoding="utf-8", newline=""))
            check_file(handle)
            before = handle.read(LIMIT + 1)
            if len(before) > LIMIT:
                raise MirrorError("mirror list exceeds the preparation limit")
            plans.append((path, preferred, handle, before, prioritize(before, preferred)))
        # Validate every present list and held input before changing either file.
        for _, _, handle, before, _ in plans:
            check_file(handle)
            handle.seek(0)
            if handle.read(LIMIT + 1) != before:
                raise MirrorError("mirror list changed during preparation")
        for path, preferred, handle, before, after in plans:
            if before != after:
                handle.seek(0)
                handle.write(after)
                handle.truncate()
                handle.flush()
            print(path.name + ": existing " + preferred + " preferred at priority 0")


if __name__ == "__main__":
    try:
        main()
    except MirrorError as error:
        print("Ubuntu mirror preparation refused: " + str(error), file=sys.stderr)
        sys.exit(1)
    except (OSError, UnicodeError):
        print("Ubuntu mirror preparation refused: system mirror list I/O failed", file=sys.stderr)
        sys.exit(1)
