#!/usr/bin/env python3
"""Install and check the pinned per-bwrap policy for the clean Grok CI runtime."""
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
PROFILE_NAME = "uap-grok-bwrap"
PARENT_LABEL = "uap_grok_bwrap"
CHILD_LABEL = "uap_grok_bwrap_child"
CONTROL_NAME = "grok-bwrap-policy-control.py"
PROFILE_BYTES = 1936
PROFILE_SHA256 = "11d39094f044f0cda0febb3ad517b830301da6b2ce929664af09ee9e4dd264f9"
POLICY_DIRECTORY = Path("/etc/apparmor.d")
OVERRIDES = ("local/bwrap-userns-restrict", "local/unpriv_bwrap")
EXPECTED_LOADED = {PARENT_LABEL + " (enforce)", CHILD_LABEL + " (enforce)"}
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
    record.update(stdout_preview=stdout[:512].decode("utf-8", "backslashreplace"),
                  stderr_preview=stderr[:512].decode("utf-8", "backslashreplace"),
                  stdout_preview_truncated=len(stdout) > 512, stderr_preview_truncated=len(stderr) > 512)
    records.append(record)
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


def open_policy_directory(path=POLICY_DIRECTORY):
    before = path.lstat()
    require(stat.S_ISDIR(before.st_mode)
            and (before.st_uid, before.st_gid) == (os.geteuid(), os.getegid())
            and not before.st_mode & 0o022, "policy directory authority")
    fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        require(identity(os.fstat(fd)) == identity(before), "policy directory replaced")
    except BaseException:
        os.close(fd)
        raise
    return fd


def require_absent(fd, name):
    try:
        os.stat(name, dir_fd=fd, follow_symlinks=False)
    except FileNotFoundError:
        return
    raise ValueError("pre-existing Bubblewrap policy or local override")


def check_policy_names(fd, *, installed=False):
    require_absent(fd, "bwrap")
    require_absent(fd, "bwrap-userns-restrict")
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


def write_owned(fd, name, body, mode=0o644):
    require(0 < len(body) <= 65536, "policy authored input bound")
    target = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=fd)
    try:
        offset = 0
        while offset < len(body):
            count = os.write(target, body[offset:])
            require(count > 0, "policy write incomplete")
            offset += count
        os.fchmod(target, mode)
        os.fsync(target)
        info = os.fstat(target)
        require(stat.S_ISREG(info.st_mode) and stat.S_IMODE(info.st_mode) == mode
                and (info.st_uid, info.st_gid, info.st_nlink, info.st_size)
                == (os.geteuid(), os.getegid(), 1, len(body)), "installed policy shape")
    finally:
        os.close(target)



def derive_profile(vendor):
    require(len(vendor) == PROFILE_BYTES and sha(vendor) == PROFILE_SHA256,
            "pinned policy body mismatch")
    replacements = (
        (b"profile bwrap /usr/bin/bwrap flags=", b"profile uap_grok_bwrap /usr/bin/bwrap flags="),
        (b"allow px /** -> bwrap//&unpriv_bwrap,", b"allow px /** -> uap_grok_bwrap//&uap_grok_bwrap_child,"),
        (b"profile unpriv_bwrap flags=", b"profile uap_grok_bwrap_child flags="),
        (b"allow pix /** -> &unpriv_bwrap,", b"allow pix /** -> &uap_grok_bwrap_child,"),
    )
    body = vendor
    for old, new in replacements:
        require(body.count(old) == 1, "pinned policy replacement differs")
        body = body.replace(old, new, 1)
    restored = body
    for old, new in reversed(replacements):
        restored = restored.replace(new, old, 1)
    require(restored == vendor and len(body) <= 4096, "derived policy changed unrelated rules")
    return body


# systemd's private RootDirectory mount is the namespace root; AppArmor matches
# /usr/bin/bwrap there. Never assign this capability-bearing profile to Python.
# Only this fixed control is installed in the existing root-owned input bind.
CONTROL = r'''import fcntl, hashlib, json, os, socket, stat, struct, subprocess, sys

def require(value, reason):
    if not value:
        raise ValueError(reason)

def read(path, bound):
    with open(path, "rb", buffering=0) as stream:
        body = stream.read(bound + 1)
    require(len(body) <= bound, "control-input-bound")
    return body

def physical(info):
    return [info.st_dev, info.st_ino, info.st_mode, info.st_uid, info.st_gid]

def identity(info):
    return physical(info) + [info.st_nlink, info.st_size, info.st_mtime_ns, info.st_ctime_ns]

def facts():
    wanted = {"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb", "NoNewPrivs", "Uid", "Gid"}
    fields = {}
    for line in read("/proc/self/status", 65536).decode("ascii").splitlines():
        key, sep, value = line.partition(":")
        if sep and key in wanted:
            require(key not in fields, "duplicate-status-field")
            fields[key] = value.split()
    require(set(fields) == wanted, "status-fields")
    caps = {}
    for key in ("CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"):
        require(len(fields[key]) == 1 and len(fields[key][0]) == 16
                and all(c in "0123456789abcdef" for c in fields[key][0]), "capability-shape")
        caps[key] = int(fields[key][0], 16)
    require(fields["NoNewPrivs"] == ["1"], "no-new-privileges")
    require(fields["Uid"] == [str(os.getuid())] * 4 and fields["Gid"] == [str(os.getgid())] * 4,
            "ordinary-identity")
    groups = os.getgroups()
    require(len(groups) <= 64 and all(type(value) is int and 0 <= value < 1 << 32 for value in groups),
            "original-group-bound")
    overflow = read("/proc/sys/kernel/overflowgid", 16).decode("ascii")
    require(overflow.endswith("\n") and overflow[:-1].isdigit()
            and str(int(overflow)) + "\n" == overflow and 0 <= int(overflow) < 1 << 32,
            "overflow-gid-shape")
    return dict(uid=os.getuid(), gid=os.getgid(), groups=groups, overflow_gid=int(overflow), capabilities=caps,
                no_new_privileges=1, label=read("/proc/self/attr/current", 256).decode("ascii").strip(),
                user_namespace=os.stat("/proc/self/ns/user").st_ino,
                network_namespace=os.stat("/proc/self/ns/net").st_ino)

def check_child(value, expected):
    require(value["uid"] == expected["uid"] and value["gid"] == expected["gid"]
            and value["groups"] == expected["groups"] and value["overflow_gid"] == expected["overflow_gid"]
            and set(value["capabilities"]) == {"CapInh", "CapPrm", "CapEff", "CapBnd", "CapAmb"}
            and all(v == 0 for v in value["capabilities"].values())
            and value["no_new_privileges"] == 1 and value["label"] == expected["child_label"],
            "child-policy-or-privilege")
    require(value["user_namespace"] != expected["parent_user_namespace"]
            and value["network_namespace"] != expected["parent_network_namespace"],
            "child-namespace")
    require(value["interfaces"] == ["lo"] and value["loopback_up"], "child-loopback")

def main():
    require(len(sys.argv) == 3 and sys.argv[1] in ("parent", "child"), "control-arguments")
    expected = json.loads(sys.argv[2])
    current = facts()
    if sys.argv[1] == "child":
        current["interfaces"] = sorted(name for _, name in socket.if_nameindex())
        with socket.socket(socket.AF_INET, socket.SOCK_DGRAM) as channel:
            flags = fcntl.ioctl(channel.fileno(), 0x8913, b"lo" + bytes(30))
        current["loopback_up"] = bool(struct.unpack_from("H", flags, 16)[0] & 1)
        check_child(current, expected)
        print(json.dumps(current, sort_keys=True, separators=(",", ":")))
        return
    require(current["uid"] == expected["uid"] and current["gid"] == expected["gid"]
            and current["label"] == "unconfined" and current["user_namespace"] == expected["host_user_namespace"]
            and all(current["capabilities"][key] == 0 for key in ("CapInh", "CapPrm", "CapEff", "CapAmb")),
            "parent-policy-or-privilege")
    require(physical(os.stat("/")) == expected["root"], "runtime-root-identity")
    expected["parent_user_namespace"] = current["user_namespace"]
    expected["parent_network_namespace"] = current["network_namespace"]
    expected["overflow_gid"] = current["overflow_gid"]
    expected["groups"] = [gid if gid == expected["gid"] else current["overflow_gid"] for gid in current["groups"]]
    held = []
    try:
        binary = os.open("/usr/bin/bwrap", os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
        held.append(binary)
        before = os.fstat(binary)
        require(identity(before) == expected["binary"], "bwrap-identity")
        with os.fdopen(os.dup(binary), "rb", buffering=0) as stream:
            body = stream.read((4 << 20) + 1)
        require(len(body) == before.st_size and hashlib.sha256(body).hexdigest() == expected["binary_sha256"],
                "bwrap-bytes")
        os.lseek(binary, 0, os.SEEK_SET)
        argv = ["/usr/bin/bwrap", "--unshare-user", "--uid", str(expected["uid"]),
                "--gid", str(expected["gid"]), "--unshare-pid", "--unshare-net", "--unshare-ipc",
                "--unshare-uts", "--disable-userns", "--die-with-parent", "--new-session",
                "--cap-drop", "ALL", "--tmpfs", "/"]
        for visible in ("/usr", "/bin", "/lib", "/lib64"):
            path = os.path.realpath(visible, strict=True)
            require(path == "/usr" or path.startswith("/usr/"), "runtime-alias")
            fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
            held.append(fd)
            info = os.fstat(fd)
            require((info.st_uid, info.st_gid) == (0, 0) and not info.st_mode & 0o022, "runtime-owner")
            argv.extend(("--ro-bind-fd", str(fd), visible))
        script = os.open(__file__, os.O_RDONLY | os.O_NOFOLLOW)
        held.append(script)
        info = os.fstat(script)
        require(stat.S_ISREG(info.st_mode) and (info.st_uid, info.st_gid, info.st_nlink) == (0, 0, 1)
                and not info.st_mode & 0o222, "control-owner")
        argv.extend(("--ro-bind-fd", str(script), "/grok-bwrap-policy-control.py",
            "--proc", "/proc", "--remount-ro", "/proc", "--dev", "/dev", "--remount-ro", "/dev",
            "--remount-ro", "/", "--chdir", "/", "--", "/usr/bin/python3.12", "-I", "-B",
            "/grok-bwrap-policy-control.py", "child", json.dumps(expected, separators=(",", ":"))))
        result = subprocess.run(argv, executable="/proc/self/fd/" + str(binary), pass_fds=tuple(held),
            stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            cwd="/", env={"PATH": "/usr/bin:/bin", "LANG": "C"}, timeout=10, check=False)
        if result.returncode != 0 or result.stderr or len(result.stdout) > 4096:
            print(json.dumps(dict(control="bwrap-control-refused", exit=result.returncode,
                stderr_preview=result.stderr[:512].decode("utf-8", "backslashreplace"),
                stdout_preview=result.stdout[:512].decode("utf-8", "backslashreplace"),
                stderr_bytes=len(result.stderr), stdout_bytes=len(result.stdout)), ensure_ascii=True), file=sys.stderr)
        require(result.returncode == 0 and not result.stderr and len(result.stdout) <= 4096, "bwrap-control-refused")
        child = json.loads(result.stdout)
        check_child(child, expected)
        require(identity(os.fstat(binary)) == identity(before) == identity(os.stat("/usr/bin/bwrap")),
                "bwrap-changed")
        require(facts() == current and physical(os.stat("/")) == expected["root"], "parent-context-changed")
    finally:
        for fd in reversed(held):
            os.close(fd)
    print(json.dumps(dict(schema=1, parent=current, child=child, fd_exec=True, native_client_executed=False,
                          qualification=False), sort_keys=True, separators=(",", ":")))

main()
'''


def prove_runtime(root, deadline, runtime, inputs, output, uid, gid, records):
    require(remaining(deadline) >= 30, "insufficient original policy-control budget")
    runtime_fd, input_fd, binary_fd = -1, -1, -1
    try:
        runtime_fd = open_policy_directory(runtime)
        input_fd = open_policy_directory(inputs)
        root_info, input_info = os.fstat(runtime_fd), os.fstat(input_fd)
        out_info = output.lstat()
        require(stat.S_ISDIR(out_info.st_mode) and (out_info.st_uid, out_info.st_gid) == (uid, gid)
                and stat.S_IMODE(out_info.st_mode) == 0o700, "control output authority")
        binary_path = runtime / "usr/bin/bwrap"
        original = binary_path.lstat()
        body = read_regular(binary_path, 4 << 20, deadline)
        require(stat.S_IMODE(original.st_mode) == 0o555, "selected bwrap is not read-only")
        binary_fd = os.open(binary_path, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
        require(identity(os.fstat(binary_fd)) == identity(original), "selected bwrap replaced")
        require_absent(input_fd, CONTROL_NAME)
        write_owned(input_fd, CONTROL_NAME, CONTROL.encode("ascii"), mode=0o444)
        control_path = inputs / CONTROL_NAME
        require(read_regular(control_path, 65536, deadline) == CONTROL.encode("ascii"), "control bytes changed")
        expected = dict(uid=uid, gid=gid, root=list(identity(root_info)[:5]),
                        binary=list(identity(original)), binary_sha256=sha(body),
                        host_user_namespace=os.stat("/proc/self/ns/user").st_ino,
                        child_label=PARENT_LABEL + "//&" + CHILD_LABEL + " (enforce)")
        # identity() starts dev/ino/mode/uid/gid, then link/size/times, as in the control.
        unit = "uap-grok-policy-" + root.name.rsplit("-", 1)[-1]
        argv = ["/usr/bin/systemd-run", "--unit=" + unit, "--wait", "--pipe", "--collect", "--quiet",
                "--service-type=exec", "--expand-environment=no", "--uid=" + str(uid), "--gid=" + str(gid),
                "--property=Delegate=yes", "--property=NoNewPrivileges=yes", "--property=KillMode=control-group",
                "--property=RootDirectory=" + str(runtime), "--property=MountAPIVFS=yes",
                "--property=BindReadOnlyPaths=" + str(inputs) + ":/opt/uap-input",
                "--property=BindPaths=" + str(output) + ":/work /sys/fs/cgroup",
                "--property=RuntimeMaxSec=20s", "--property=TimeoutStopSec=5s",
                "/usr/bin/env", "-i", "PATH=/usr/bin:/bin", "/usr/bin/python3.12", "-I", "-B",
                "/opt/uap-input/" + CONTROL_NAME, "parent", json.dumps(expected, separators=(",", ":"))]
        report = json.loads(command(root, "runtime-policy-control", argv, deadline, records, 8192))
        require(type(report) is dict and set(report) == {"schema", "parent", "child", "fd_exec", "native_client_executed", "qualification"}
                and report["schema"] == 1 and report["fd_exec"] is True
                and report["native_client_executed"] is False and report["qualification"] is False,
                "control report shape")
        require(identity(os.fstat(binary_fd)) == identity(original)
                and read_regular(binary_path, 4 << 20, deadline) == body, "selected bwrap changed")
        for path, fd, original_directory in ((runtime, runtime_fd, root_info), (inputs, input_fd, input_info)):
            require(identity(os.fstat(fd))[:5] == identity(path.lstat())[:5] == identity(original_directory)[:5],
                    "original runtime directory changed")
        require(read_regular(control_path, 65536, deadline) == CONTROL.encode("ascii"), "control bytes changed")
        remaining(deadline)
        return report
    finally:
        for fd in (binary_fd, input_fd, runtime_fd):
            if fd >= 0:
                os.close(fd)


def setup(root, deadline, runtime, inputs, output, uid, gid, records):
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
    vendor_profile = profile_from_tar(tar)
    profile = derive_profile(vendor_profile)
    restriction = kernel_read("/proc/sys/kernel/apparmor_restrict_unprivileged_userns", 16)
    require(restriction == b"1\n", "ordinary Ubuntu user-namespace restriction required")
    before = loaded_profiles()
    require_loaded(before)
    fd = open_policy_directory()
    try:
        check_policy_names(fd)
        remaining(deadline)
        write_owned(fd, PROFILE_NAME, profile)
        path = POLICY_DIRECTORY / PROFILE_NAME
        require(read_regular(path, 4096, deadline) == profile, "installed derived policy differs")
        command(root, "policy-add", ["/usr/sbin/apparmor_parser", "--config-file=/dev/null",
                "--add", "--skip-cache", "--jobs=1", "--Werror=rule-not-enforced", str(path)], deadline, records)
        check_policy_names(fd, installed=True)
        require(read_regular(path, 4096, deadline) == profile, "installed policy changed")
        control = prove_runtime(root, deadline, runtime, inputs, output, uid, gid, records)
        after = loaded_profiles()
        require_loaded(before, after)
        require(kernel_read("/proc/sys/kernel/apparmor_restrict_unprivileged_userns", 16) == restriction,
                "user-namespace restriction changed")
        held, named = os.fstat(fd), POLICY_DIRECTORY.lstat()
        require((held.st_dev, held.st_ino, held.st_mode, held.st_uid, held.st_gid)
                == (named.st_dev, named.st_ino, named.st_mode, named.st_uid, named.st_gid),
                "policy directory authority changed")
        remaining(deadline)
    finally:
        os.close(fd)
    return {"schema": 1, "package": metadata, "vendor_profile_sha256": sha(vendor_profile),
            "control": control, "profile": {"path": str(path),
            "bytes": len(profile), "sha256": sha(profile)}, "added_profiles": sorted(EXPECTED_LOADED),
            "prior_profiles_sha256": sha("\n".join(sorted(before)).encode()),
            "restrict_userns_unchanged": True, "local_overrides_absent": True,
            "commands": records, "bwrap_control_executed": True, "native_client_executed": False, "qualification": False}


def main():
    require(os.getuid() == os.geteuid() == os.getgid() == os.getegid() == 0,
            "policy installation requires the explicit CI setup step")
    require(len(sys.argv) == 2, "exact RUNNER_TEMP argument required")
    runner = Path(sys.argv[1])
    require(runner.is_absolute() and str(runner) == str(runner.resolve(strict=True))
            and all(c.isascii() and (c.isalnum() or c in "/_.-") for c in str(runner)),
            "canonical fixed runtime parent required")
    ids = [os.environ.get(name, "") for name in ("SUDO_UID", "SUDO_GID")]
    require(all(value.isascii() and value.isdecimal() and 0 < int(value) < 1 << 31 for value in ids),
            "policy setup must return to the ordinary CI runner")
    uid, gid = map(int, ids)
    # Hosted-runner disposal owns final cleanup; unsettled setup never removes inputs.
    temporary = Path(tempfile.mkdtemp(prefix="grok-bwrap-policy-"))
    records = []
    result = {"schema": 1, "qualification": False, "setup_completed": False, "commands": records}
    try:
        result = setup(temporary, time.monotonic() + 150, runner / "grok-runtime-root",
                       runner / "grok-runtime-input", runner / "grok-readonly-evidence", uid, gid, records)
        result["setup_completed"] = True
    except Exception as error:
        result["error"] = type(error).__name__ + ": " + str(error)[:512]
        raise
    finally:
        text = json.dumps(result, ensure_ascii=True, sort_keys=True, separators=(",", ":"))
        require(len(text) <= 65536, "policy setup report bound")
        print(text)


if __name__ == "__main__":
    main()
