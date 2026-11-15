#!/usr/bin/env python3
"""Install only the pinned Noble Bubblewrap profile on a disposable CI runner."""
import hashlib
import io
import json
import os
from pathlib import Path
import stat
import sys
import tarfile
import tempfile
import time

from native_client_research import run_probe

PACKAGE = "apparmor-profiles"
VERSION = "4.0.1really4.0.1-0ubuntu0.24.04.9"
DEB_NAME = PACKAGE + "_" + VERSION + "_all.deb"
DEB_BYTES = 39592
DEB_SHA256 = "90b02aa006eea7702cd4e851343e469e41365dda42145a3cb035de1d6c773b8c"
PROFILE_MEMBER = "usr/share/apparmor/extra-profiles/bwrap-userns-restrict"
PROFILE_NAME = "bwrap-userns-restrict"
PROFILE_BYTES = 1936
PROFILE_SHA256 = "11d39094f044f0cda0febb3ad517b830301da6b2ce929664af09ee9e4dd264f9"
POLICY_DIRECTORY = Path("/etc/apparmor.d")
KERNEL_FEATURE_DIRECTORY = Path("/sys/kernel/security/apparmor/features")
KERNEL_REQUIRED_FEATURES = {
    "io_uring/mask": frozenset((b"sqpoll", b"override_creds")),
    "namespaces/mask": frozenset((b"userns_create",)),
    "domain/stack": None,
    "domain/post_nnp_subset": None,
}
OVERRIDES = ("local/bwrap-userns-restrict", "local/unpriv_bwrap")
EXPECTED_LOADED = {"bwrap (enforce)", "unpriv_bwrap (enforce)"}
LIMIT = 1 << 20
ENV = {"PATH": "/usr/sbin:/usr/bin:/sbin:/bin", "LC_ALL": "C", "LANG": "C",
       "PYTHONDONTWRITEBYTECODE": "1"}


def require(value, message):
    if not value:
        raise ValueError(message)


def sha(body):
    return hashlib.sha256(body).hexdigest()


def identity(info):
    return (info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid,
            info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns)


def remaining(deadline):
    value = deadline - time.monotonic()
    require(value > 0, "Bubblewrap policy setup deadline")
    return value


def command(root, label, argv, deadline, records, limit=LIMIT):
    record, stdout, stderr = run_probe(argv, root, ENV, root, label,
        timeout=min(30, remaining(deadline)), limit=limit, memory_only=True)
    records.append(record)
    if label == "policy-add" and (record["error"] is not None or record["exit"] != 0):
        try:
            packet = {"stage": "policy-add", "exit": record["exit"],
                      "capture_error_present": record["error"] is not None,
                      "stderr_captured_bytes": record["stderr_bytes"],
                      "stderr_captured_sha256": record["stderr_sha256"],
                      "stderr_tail": stderr[-2048:].decode("utf-8", "backslashreplace"),
                      "stderr_tail_truncated": len(stderr) > 2048}
            text = json.dumps(packet, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
            require(len(text) <= 16384, "policy failure diagnostic bound")
            print("DEVIN_BWRAP_POLICY_FAILURE=" + text, file=sys.stderr, flush=True)
        except Exception:
            # Optional output never substitutes for the original refusal below.
            pass
    require(record["error"] is None and record["exit"] == 0, "Bubblewrap policy setup command refused: " + label)
    remaining(deadline)
    return stdout


def read_regular(path, bound, deadline):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and (before.st_uid, before.st_gid) == (os.geteuid(), os.getegid())
            and not before.st_mode & 0o6022 and 0 < before.st_size <= bound,
            "Bubblewrap policy input shape")
    fd = os.open(path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    try:
        require(identity(os.fstat(fd)) == identity(before), "Bubblewrap policy input replaced")
        chunks, count = [], 0
        while True:
            remaining(deadline)
            part = os.read(fd, min(65536, bound + 1 - count))
            if not part:
                break
            chunks.append(part)
            count += len(part)
            require(count <= bound, "Bubblewrap policy input bound")
        require(identity(os.fstat(fd)) == identity(before)
                and identity(path.lstat()) == identity(before), "Bubblewrap policy input changed")
        remaining(deadline)
        return b"".join(chunks)
    finally:
        os.close(fd)


def apt_metadata(body):
    rows = []
    for paragraph in body.decode("utf-8").strip().split("\n\n"):
        fields = {}
        for line in paragraph.splitlines():
            if line and not line[0].isspace() and ":" in line:
                key, value = line.split(":", 1)
                require(key not in fields, "duplicate policy package field")
                fields[key] = value.strip()
        if fields.get("Package") == PACKAGE and fields.get("Version") == VERSION:
            rows.append({key: fields.get(key) for key in ("Package", "Version", "Architecture", "Filename", "Size", "SHA256")})
    wanted = {"Package": PACKAGE, "Version": VERSION, "Architecture": "all",
              "Filename": "pool/main/a/apparmor/" + DEB_NAME,
              "Size": str(DEB_BYTES), "SHA256": DEB_SHA256}
    require(rows and all(row == wanted for row in rows), "pinned signed-APT policy metadata mismatch")
    return wanted


def profile_from_tar(body):
    require(len(body) <= 2 * LIMIT, "policy package tar bound")
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:") as archive:
        members = archive.getmembers()
        require(len(members) <= 4096, "policy package member bound")
        selected = [row for row in members if row.name in (PROFILE_MEMBER, "./" + PROFILE_MEMBER)]
        require(len(selected) == 1, "policy package member absent or duplicated")
        row = selected[0]
        require(row.isfile() and (row.uid, row.gid, row.mode, row.size) == (0, 0, 0o644, PROFILE_BYTES),
                "policy package member shape")
        with archive.extractfile(row) as stream:
            profile = stream.read(PROFILE_BYTES + 1)
    require(len(profile) == PROFILE_BYTES and sha(profile) == PROFILE_SHA256, "pinned policy body mismatch")
    return profile


def kernel_read(path, bound):
    with open(path, "rb", buffering=0) as stream:
        body = stream.read(bound + 1)
    require(len(body) <= bound, "policy kernel fact bound")
    return body


def read_kernel_feature(fd, deadline):
    # securityfs reports st_size=0 for nonempty feature files.
    os.lseek(fd, 0, os.SEEK_SET)
    body = b""
    while True:
        remaining(deadline)
        part = os.read(fd, 4097 - len(body))
        if not part:
            break
        body += part
        require(len(body) <= 4096, "policy kernel feature bound")
    remaining(deadline)
    require(body and body.isascii() and b"\x00" not in body, "policy kernel feature content")
    return body


def open_kernel_features(deadline):
    held = {}
    try:
        for relative in (".", "io_uring", "namespaces", "domain", *KERNEL_REQUIRED_FEATURES):
            remaining(deadline)
            path = Path(relative)
            parent_fd = None if relative == "." else held[str(path.parent)][0]
            name = KERNEL_FEATURE_DIRECTORY if relative == "." else path.name
            original = os.stat(name, dir_fd=parent_fd, follow_symlinks=False)
            regular = relative in KERNEL_REQUIRED_FEATURES
            require((stat.S_ISREG(original.st_mode) if regular else stat.S_ISDIR(original.st_mode))
                    and (original.st_uid, original.st_gid) == (os.geteuid(), os.getegid())
                    and not original.st_mode & 0o6022
                    and (not regular or (original.st_nlink == 1 and 0 <= original.st_size <= 4096)),
                    "policy kernel feature authority")
            fd = os.open(name, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW
                         | (0 if regular else os.O_DIRECTORY), dir_fd=parent_fd)
            held[relative] = (fd, original, None)
            require(identity(os.fstat(fd)) == identity(original), "policy kernel feature replaced")
            if regular:
                body = read_kernel_feature(fd, deadline)
                wanted = KERNEL_REQUIRED_FEATURES[relative]
                require((body == b"yes\n" if wanted is None else wanted <= set(body.split())),
                        "required AppArmor kernel mediation is absent")
                held[relative] = (fd, original, body)
        verify_kernel_features(held, deadline)
        return held
    except BaseException:
        close_kernel_features(held)
        raise


def verify_kernel_features(held, deadline):
    for relative, (fd, original, body) in held.items():
        remaining(deadline)
        path = Path(relative)
        parent_fd = None if relative == "." else held[str(path.parent)][0]
        name = KERNEL_FEATURE_DIRECTORY if relative == "." else path.name
        require(identity(os.fstat(fd)) == identity(original)
                == identity(os.stat(name, dir_fd=parent_fd, follow_symlinks=False)),
                "original policy kernel feature changed")
        if body is not None:
            require(read_kernel_feature(fd, deadline) == body, "policy kernel feature bytes changed")
            require(identity(os.fstat(fd)) == identity(original)
                    == identity(os.stat(name, dir_fd=parent_fd, follow_symlinks=False)),
                    "original policy kernel feature changed")
    remaining(deadline)


def close_kernel_features(held):
    for fd, _, _ in reversed(tuple(held.values())):
        os.close(fd)


def capture_kernel_abi(root, held, deadline, records, original=None):
    verify_kernel_features(held, deadline)
    stage = "policy-kernel-abi-before" if original is None else "policy-kernel-abi-after"
    body = command(root, stage, ["/usr/bin/aa-features-abi", "--extract", "--stdout"],
                   deadline, records, 65536)
    require(0 < len(body) <= 65536 and all(byte in (9, 10, 13) or 32 <= byte <= 126 for byte in body),
            "actual kernel ABI export shape")
    verify_kernel_features(held, deadline)
    require(original is None or body == original, "actual kernel ABI changed")
    return body


def loaded_profiles():
    body = kernel_read("/sys/kernel/security/apparmor/profiles", LIMIT)
    rows = body.decode("utf-8").splitlines()
    require(len(rows) <= 4096 and len(set(rows)) == len(rows)
            and all(0 < len(row) <= 1024 and "\x00" not in row for row in rows), "loaded policy list bound")
    return set(rows)


def require_loaded(before, after=None):
    require(not any("bwrap" in row for row in before), "a Bubblewrap policy already exists")
    if after is not None:
        require(after == before | EXPECTED_LOADED, "pinned Bubblewrap policies were not added in enforce mode")


def open_policy_directory():
    before = POLICY_DIRECTORY.lstat()
    require(stat.S_ISDIR(before.st_mode)
            and (before.st_uid, before.st_gid) == (os.geteuid(), os.getegid())
            and not before.st_mode & 0o022, "policy directory authority")
    fd = os.open(POLICY_DIRECTORY, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        require(identity(os.fstat(fd)) == identity(before), "policy directory replaced")
    except BaseException:
        os.close(fd)
        raise
    return fd, before


def verify_policy_directory(fd, original):
    require(identity(os.fstat(fd))[:5] == identity(POLICY_DIRECTORY.lstat())[:5] == identity(original)[:5],
            "policy directory authority changed")


def require_absent(fd, name):
    try:
        os.stat(name, dir_fd=fd, follow_symlinks=False)
    except FileNotFoundError:
        return
    raise ValueError("pre-existing Bubblewrap policy or local override")


def check_policy_names(fd, *, installed=False):
    require_absent(fd, "bwrap")
    if not installed:
        require_absent(fd, PROFILE_NAME)
    try:
        local = os.stat("local", dir_fd=fd, follow_symlinks=False)
    except FileNotFoundError:
        local = None
    if local is not None:
        require(stat.S_ISDIR(local.st_mode)
                and (local.st_uid, local.st_gid) == (os.geteuid(), os.getegid())
                and not local.st_mode & 0o022, "local policy directory authority")
    for name in OVERRIDES:
        require_absent(fd, name)


def write_profile(fd, body):
    require(len(body) == PROFILE_BYTES and sha(body) == PROFILE_SHA256, "pinned policy body mismatch")
    target = os.open(PROFILE_NAME, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    try:
        offset = 0
        while offset < len(body):
            count = os.write(target, body[offset:])
            require(count > 0, "policy write incomplete")
            offset += count
        os.fchmod(target, 0o644)
        os.fsync(target)
        info = os.fstat(target)
        require(stat.S_ISREG(info.st_mode) and stat.S_IMODE(info.st_mode) == 0o644
                and (info.st_uid, info.st_gid, info.st_nlink, info.st_size)
                == (os.geteuid(), os.getegid(), 1, len(body)), "installed policy shape")
    finally:
        os.close(target)


def setup(root, deadline):
    records = []
    metadata = apt_metadata(command(root, "policy-metadata",
        ["/usr/bin/apt-cache", "show", PACKAGE + "=" + VERSION], deadline, records, 65536))
    command(root, "policy-download", ["/usr/bin/apt-get",
        "-o", "APT::Get::AllowUnauthenticated=false", "-o", "Acquire::AllowInsecureRepositories=false",
        "download", PACKAGE + "=" + VERSION], deadline, records, 65536)
    require(sorted(path.name for path in root.iterdir()) == [DEB_NAME], "unexpected policy download member")
    deb = read_regular(root / DEB_NAME, DEB_BYTES, deadline)
    require(len(deb) == DEB_BYTES and sha(deb) == DEB_SHA256, "pinned signed-APT archive mismatch")
    tar = command(root, "policy-package-data", ["/usr/bin/dpkg-deb", "--fsys-tarfile", str(root / DEB_NAME)],
                  deadline, records, 2 * LIMIT)
    require(read_regular(root / DEB_NAME, DEB_BYTES, deadline) == deb, "policy archive changed")
    profile = profile_from_tar(tar)
    restriction = kernel_read("/proc/sys/kernel/apparmor_restrict_unprivileged_userns", 16)
    require(restriction == b"1\n", "ordinary Ubuntu user-namespace restriction required")
    before = loaded_profiles()
    require_loaded(before)
    features = open_kernel_features(deadline)
    fd = -1
    try:
        kernel_abi = capture_kernel_abi(root, features, deadline, records)
        fd, original_directory = open_policy_directory()
        check_policy_names(fd)
        remaining(deadline)
        write_profile(fd, profile)
        path = POLICY_DIRECTORY / PROFILE_NAME
        require(read_regular(path, PROFILE_BYTES, deadline) == profile, "installed policy differs from signed package")
        verify_kernel_features(features, deadline)
        command(root, "policy-add", ["/usr/sbin/apparmor_parser", "--config-file=/dev/null",
                "--add", "--skip-cache", "--jobs=1", "--override-policy-abi=<kernel>",
                "--warn=rule-not-enforced", "--Werror=rule-not-enforced", str(path)], deadline, records)
        capture_kernel_abi(root, features, deadline, records, kernel_abi)
        check_policy_names(fd, installed=True)
        require(read_regular(path, PROFILE_BYTES, deadline) == profile, "installed policy changed")
        after = loaded_profiles()
        require_loaded(before, after)
        require(kernel_read("/proc/sys/kernel/apparmor_restrict_unprivileged_userns", 16) == restriction,
                "user-namespace restriction changed")
        verify_policy_directory(fd, original_directory)
        remaining(deadline)
    finally:
        try:
            if fd >= 0:
                os.close(fd)
        finally:
            close_kernel_features(features)
    remaining(deadline)
    return {"schema": 1, "package": metadata, "profile": {"path": str(path),
            "bytes": len(profile), "sha256": sha(profile)}, "added_profiles": sorted(EXPECTED_LOADED),
            "prior_profiles_sha256": sha("\n".join(sorted(before)).encode()),
            "restrict_userns_unchanged": True, "local_overrides_absent": True,
            "commands": records, "native_executed": False, "qualification": False}


def main():
    require(os.getuid() == os.geteuid() == os.getgid() == os.getegid() == 0,
            "policy installation requires the explicit CI setup step")
    require(os.environ.get("SUDO_UID", "").isdigit() and int(os.environ["SUDO_UID"]) > 0,
            "policy setup must return to the ordinary CI runner")
    # Hosted-runner disposal owns final cleanup; unsettled setup never removes inputs.
    temporary = Path(tempfile.mkdtemp(prefix="devin-bwrap-policy-"))
    result = setup(temporary, time.monotonic() + 150)
    text = json.dumps(result, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
    require(len(text) <= 65536, "policy setup report bound")
    print("DEVIN_BWRAP_POLICY_SETUP=" + text)


if __name__ == "__main__":
    main()
