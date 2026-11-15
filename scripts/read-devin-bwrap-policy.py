#!/usr/bin/env python3
"""Read a fixed Noble AppArmor policy package and bounded host facts; never load policy or launch bwrap."""
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import tarfile
import tempfile
import time

import native_client_research as capture

PACKAGE = "apparmor-profiles"
VERSION = "4.0.1really4.0.1-0ubuntu0.24.04.9"
DEB_NAME = PACKAGE + "_" + VERSION + "_all.deb"
DEB_BYTES = 39592
DEB_SHA256 = "90b02aa006eea7702cd4e851343e469e41365dda42145a3cb035de1d6c773b8c"
PROFILE = "./usr/share/apparmor/extra-profiles/bwrap-userns-restrict"
DEADLINE = time.monotonic() + 150
ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C", "LANG": "C", "PYTHONDONTWRITEBYTECODE": "1"}
LIMIT = 1 << 20


def require(value, message):
    if not value:
        raise ValueError(message)


def remaining():
    value = DEADLINE - time.monotonic()
    require(value > 0, "policy readout deadline")
    return value


def sha(body):
    return hashlib.sha256(body).hexdigest()


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def ordinary(path, bound, expected_uid=None):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and before.st_size <= bound and not before.st_mode & 0o022, "policy input file shape")
    if expected_uid is not None:
        require(before.st_uid == expected_uid, "policy input file owner")
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        require(identity(os.fstat(fd)) == identity(before), "policy input changed before read")
        chunks, count = [], 0
        while True:
            remaining()
            part = os.read(fd, min(65536, bound + 1 - count))
            if not part:
                break
            chunks.append(part)
            count += len(part)
            require(count <= bound, "policy input exceeded bound")
        require(identity(os.fstat(fd)) == identity(before)
                and identity(path.lstat()) == identity(before), "policy input changed during read")
        return b"".join(chunks), before
    finally:
        os.close(fd)


def command(root, label, argv, limit=LIMIT, required=True):
    record, stdout, stderr = capture.run_probe(argv, root, ENV, root, label,
                                              timeout=min(30, remaining()), limit=limit, memory_only=True)
    require(record["error"] is None and type(record["exit"]) is int, "policy command did not settle")
    if required:
        require(record["exit"] == 0, "policy command refused: " + label)
    return record, stdout, stderr


def host_file(path, bound=16384):
    result = {"path": str(path)}
    try:
        info = path.lstat()
        result["mode"] = info.st_mode
        result["uid"], result["gid"] = info.st_uid, info.st_gid
        if stat.S_ISLNK(info.st_mode):
            target = os.readlink(path)
            require(len(os.fsencode(target)) <= 1024, "policy link text bound")
            result.update(kind="symlink", target=target)
            return result
        body, _ = ordinary(path, bound, expected_uid=0)
        result.update(kind="regular", bytes=len(body), sha256=sha(body), text=body.decode("utf-8"))
    except OSError as error:
        result.update(kind="unavailable", errno=error.errno)
    except ValueError:
        result.update(kind="guard-refused")
    return result


def kernel_file(path, bound=16384):
    try:
        with open(path, "rb", buffering=0) as stream:
            body = stream.read(bound + 1)
        require(len(body) <= bound, "kernel fact bound")
        return {"text": body.decode("ascii"), "bytes": len(body), "sha256": sha(body)}
    except OSError as error:
        return {"unavailable_errno": error.errno}


def apt_metadata(body):
    text = body.decode("utf-8")
    paragraphs = text.strip().split("\n\n")
    rows = []
    for paragraph in paragraphs:
        fields = {}
        for line in paragraph.splitlines():
            if line and not line[0].isspace() and ":" in line:
                key, value = line.split(":", 1)
                require(key not in fields, "duplicate apt field")
                fields[key] = value.strip()
        if fields.get("Package") == PACKAGE and fields.get("Version") == VERSION:
            rows.append({key: fields.get(key) for key in ("Package", "Version", "Architecture", "Filename", "Size", "SHA256")})
    require(rows and all(row == rows[0] for row in rows), "fixed apt metadata absent or ambiguous")
    row = rows[0]
    require(row == {"Package": PACKAGE, "Version": VERSION, "Architecture": "all",
                    "Filename": "pool/main/a/apparmor/" + DEB_NAME,
                    "Size": str(DEB_BYTES), "SHA256": DEB_SHA256}, "fixed apt metadata mismatch")
    return row


def package_profile(body):
    require(len(body) <= 2 * LIMIT, "package tar bound")
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:") as archive:
        members = archive.getmembers()
        require(len(members) <= 4096, "package member bound")
        selected = [member for member in members if member.name in (PROFILE, PROFILE[2:])]
        require(len(selected) == 1, "fixed profile member absent or duplicate")
        member = selected[0]
        require(member.isfile() and member.uid == member.gid == 0 and not member.mode & 0o022
                and 0 < member.size <= 16384, "fixed profile member shape")
        stream = archive.extractfile(member)
        require(stream is not None, "fixed profile data unavailable")
        with stream:
            value = stream.read(16385)
        require(len(value) == member.size, "fixed profile data bound")
        text = value.decode("utf-8")
        return {"member": member.name, "mode": member.mode, "uid": member.uid, "gid": member.gid,
                "bytes": len(value), "sha256": sha(value), "text": text, "abi4_declared": "abi <abi/4.0>," in text}


def loaded_profiles(root):
    code = ("import json\n"
            "p='/sys/kernel/security/apparmor/profiles'\n"
            "try:\n"
            " with open(p,'rb',buffering=0) as f: b=f.read(1048577)\n"
            " if len(b)>1048576: raise ValueError('loaded policy bound')\n"
            " rows=[x for x in b.decode('utf-8').splitlines() if 'bwrap' in x or 'unprivileged_userns' in x]\n"
            " if len(rows)>64 or any(len(x)>1024 for x in rows): raise ValueError('loaded policy row bound')\n"
            " print(json.dumps({'rows':rows,'complete':True},ensure_ascii=True))\n"
            "except OSError as e: print(json.dumps({'unavailable_errno':e.errno}))\n")
    record, out, _ = command(root, "loaded-profiles", ["/usr/bin/sudo", "-n", "/usr/bin/python3", "-I", "-S", "-c", code], 65536, False)
    result = {"record": record, "applies_to_original_843": False}
    if record["exit"] == 0:
        result["data"] = json.loads(out)
    return result


def recent_audit(root):
    record, out, _ = command(root, "kernel-audit", ["/usr/bin/sudo", "-n", "/usr/bin/journalctl",
                            "-k", "-b", "--no-pager", "--output=json", "-n", "256"], LIMIT, False)
    rows = []
    if record["exit"] == 0:
        for line in out.splitlines():
            item = json.loads(line)
            message = item.get("MESSAGE")
            if not isinstance(message, str) or 'apparmor="DENIED"' not in message:
                continue
            if 'comm="bwrap"' not in message and 'profile="unprivileged_userns"' not in message:
                continue
            safe = {}
            for key, quoted, plain in re.findall(r'\b(operation|profile|comm|capname|pid|capability|error)=(?:"([^"]*)"|([^ ]+))', message):
                value = quoted or plain
                require(len(value) <= 256 and value.isascii(), "audit field bound")
                safe[key] = value
            rows.append(safe)
        require(len(rows) <= 256, "audit row bound")
    return {"record": record, "rows": rows, "attributed_to_original_843": False,
            "absence_is_not_proof": True}


def main():
    require(os.getuid() > 0 and os.getgid() > 0, "ordinary runner required")
    with tempfile.TemporaryDirectory(prefix="devin-bwrap-policy-", dir=os.environ["RUNNER_TEMP"]) as temporary:
        root = Path(temporary)
        records = []
        record, out, _ = command(root, "apt-metadata", ["/usr/bin/apt-cache", "show", PACKAGE + "=" + VERSION], 65536)
        records.append(record)
        metadata = apt_metadata(out)
        record, _, _ = command(root, "apt-download", ["/usr/bin/apt-get",
                "-o", "APT::Get::AllowUnauthenticated=false", "-o", "Acquire::AllowInsecureRepositories=false",
                "download", PACKAGE + "=" + VERSION], 65536)
        records.append(record)
        require(sorted(path.name for path in root.iterdir()) == [DEB_NAME], "unexpected download member")
        deb, _ = ordinary(root / DEB_NAME, DEB_BYTES, expected_uid=os.getuid())
        require(len(deb) == DEB_BYTES and sha(deb) == DEB_SHA256, "fixed signed-apt package mismatch")
        record, tar, _ = command(root, "package-data", ["/usr/bin/dpkg-deb", "--fsys-tarfile", str(root / DEB_NAME)], 2 * LIMIT)
        records.append(record)
        require(ordinary(root / DEB_NAME, DEB_BYTES, expected_uid=os.getuid())[0] == deb, "package changed after parser")
        profile = package_profile(tar)
        fmt = chr(36) + "{binary:Package}\t" + chr(36) + "{Version}\t" + chr(36) + "{db:Status-Abbrev}\n"
        installed, out, _ = command(root, "installed-packages", ["/usr/bin/dpkg-query", "-W", "-f=" + fmt,
                                                "apparmor", "apparmor-profiles", "bubblewrap"], 16384, False)
        status = kernel_file("/proc/self/status")
        if "text" in status:
            status = {line.split(":", 1)[0]: line.split(":", 1)[1].strip()
                      for line in status["text"].splitlines()
                      if line.split(":", 1)[0] in ("Uid", "Gid", "CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb", "NoNewPrivs", "Seccomp")}
        report = {"schema": 1, "purpose": "fixed Noble policy and current-host facts only", "native_executed": False,
                  "policy_loaded_or_changed": False, "qualification": False, "original_843_cause_proved": False,
                  "uid": os.getuid(), "gid": os.getgid(), "kernel": os.uname().release,
                  "package_metadata": metadata, "package_profile": profile, "commands": records,
                  "installed": {"record": installed, "text": out.decode("utf-8")},
                  "self_label": kernel_file("/proc/self/attr/current"), "self_status": status,
                  "uid_map": kernel_file("/proc/self/uid_map"), "gid_map": kernel_file("/proc/self/gid_map"),
                  "restrict_userns": kernel_file("/proc/sys/kernel/apparmor_restrict_unprivileged_userns"),
                  "host_policy_files": [host_file(Path(path)) for path in (
                      "/etc/apparmor.d/bwrap", "/etc/apparmor.d/bwrap-userns-restrict",
                      "/usr/share/apparmor/extra-profiles/bwrap-userns-restrict", "/etc/apparmor.d/unprivileged_userns")],
                  "loaded_profiles": loaded_profiles(root), "recent_kernel_audit": recent_audit(root)}
        remaining()
        text = json.dumps(report, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
        require(len(text) <= 131072, "readout total bound")
        print("DEVIN_BWRAP_POLICY_READOUT=" + text)


if __name__ == "__main__":
    main()
