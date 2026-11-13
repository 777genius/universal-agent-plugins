#!/usr/bin/env python3
"""Pinned trusted VS Code project lifecycle through the existing strict host.

The original 900-second clock includes acquisition and builds. This frontend
does not grant trust, register MCP servers or call a probe; the reviewed native
controller owns those public-install/UI/gateway observations and all cleanup.
"""
import argparse
import contextlib
import hashlib
import importlib.util
import io
import json
import math
import os
from pathlib import Path
import platform
import re
import shutil
import stat
import struct
import sys
import tempfile
import time
import zipfile


def adjacent(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


legacy = adjacent("vscode_project_legacy", "research-vscode-workspace.py")
toolchain = adjacent("vscode_project_toolchain", "project_native_vscode_toolchain.py")
shared, publisher = toolchain.shared, legacy.publisher
SOURCE = Path(__file__).resolve().parent.parent
MODE = "trusted-project-v1"
QUALIFICATIONS = ("project-lifecycle", "native-restart-removal", "mcp-initialize", "mcp-tools-list", "mcp-tools-call")
PUBLIC = ("git-a", "git-b", "add-a", "add-b", "repeat-add-a", "unchanged-update-a",
          "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a")
GUI = ("empty-a", "empty-b", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a",
       "restart-a", "removed-a", "remaining-b")
PREPARATION = ("source-commit", "source-tree", "source-clean", "source-final-clean", "go-version",
               "controller-build", "installer-build", "probe-build", "trusted-node-version", "trusted-npm-ci",
               "trusted-tsc-version", "trusted-tsc", "trusted-driver-diagnostics", "trusted-runner-readiness",
               "publisher-version", "publisher-help")
SCANNER_ARCHIVE = "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da"


def validate_host_image(image, release):
    fields = dict(line.split("=", 1) for line in release.splitlines() if "=" in line)
    if (image != "ubuntu24" or fields.get("ID", "").strip('"') != "ubuntu"
            or fields.get("VERSION_ID", "").strip('"') != "24.04"):
        raise ValueError("trusted project requires matching genuine Ubuntu 24.04 image/release")


def guard():
    legacy.guard()
    if platform.system() != "Linux" or platform.machine() != "x86_64":
        raise ValueError("trusted project host must be the actual Ubuntu 24.04 Linux x64 runner")
    release = toolchain.ordinary_bytes(Path("/etc/os-release").resolve(strict=True), 64 << 10).decode("utf-8", "strict")
    validate_host_image(os.environ.get("ImageOS"), release)


def remaining(origin_ns, seconds):
    value = seconds - (time.monotonic_ns() - origin_ns) / 1e9
    if value <= 0:
        raise ValueError("original trusted project deadline exceeded")
    return value


def native_names():
    names = {"vscode-workspace-controller.json", "vscode-project-research.json", "system-executables.json", "xauth.json",
             "Xvfb.merged.log", "probe-a-events.log", "probe-b-events.log", "trusted-host-policy.json"}
    for label in ("xauth", "root-cleanup", "controller", "cleanup"):
        names.update({label + ".stdout.log", label + ".stderr.log"})
    names.update(label + ".merged.log" for label in PUBLIC)
    for label in GUI:
        names.update({label + ".merged.log", label + ".observation.json"})
    return names


def frontend_names():
    return {"vscode-project-frontend.json"} | {label + suffix for label in PREPARATION for suffix in (".stdout.log", ".stderr.log")}


def write_owned(directory, expected, name, body):
    """Publish a closed native member through the originally held directory."""
    if name not in native_names() or len(body) > 1 << 20 or directory.resolve() != directory:
        raise ValueError("trusted output member name/size/parent refused")
    descriptor = os.open(directory, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
    try:
        held = os.fstat(descriptor)
        if not os.path.samestat(held, expected) or held.st_uid != os.getuid() or not stat.S_ISDIR(held.st_mode):
            raise ValueError("trusted output directory identity changed")
        leaf = os.open(name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600, dir_fd=descriptor)
        with os.fdopen(leaf, "wb") as stream:
            stream.write(body)
        if not os.path.samestat(os.stat(directory, follow_symlinks=False), expected):
            raise ValueError("trusted output directory replaced during publication")
    finally:
        os.close(descriptor)


HOST_POLICY_REFERENCE = "9fd4a3cc5f3bf64aefbed6ff4cab30fd60563d655ae2896197f152d07b726e9a"


def validate_host_policy(body):
    """Admit facts only; installed bytes and loaded membership are not equivalent."""
    if not isinstance(body, bytes) or len(body) > 16 << 10:
        raise ValueError("host policy artifact bound refused")
    value = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=shared.legacy.unique_object,
                       parse_constant=shared.legacy.invalid_constant)

    def keys(obj, expected):
        if type(obj) is not dict or set(obj) != set(expected.split()):
            raise ValueError("closed host policy fields refused")

    def integer(number, maximum, minimum=0):
        if type(number) is not int or not minimum <= number <= maximum:
            raise ValueError("host policy integer refused")

    def identity(obj, directory=False):
        keys(obj, "device inode mode uid gid links size mtime_ns ctime_ns")
        for key in ("device", "inode", "links"):
            integer(obj[key], (1 << 64) - 1, 1 if key != "device" else 0)
        for key in ("mode", "uid", "gid"):
            integer(obj[key], (1 << 32) - 1)
        integer(obj["size"], (1 << 63) - 1)
        for key in ("mtime_ns", "ctime_ns"):
            integer(obj[key], (1 << 63) - 1, -(1 << 63))
        if obj["uid"] != 0 or (stat.S_IFMT(obj["mode"]) != (stat.S_IFDIR if directory else stat.S_IFREG)):
            raise ValueError("host policy object type/owner refused")
        if not directory and (obj["links"] != 1 or obj["size"] > 16 << 10 or obj["mode"] & 0o22):
            raise ValueError("host policy ordinary file refused")

    keys(value, "schema_version status reason reference_profile_sha256 before after native_qualified loaded_equals_disk_qualified")
    reasons = {"none", "file-unsafe", "file-capture-refused", "directory-refused", "code-leaf-refused", "entry-refused",
               "grammar-refused", "loaded-profile-refused", "loaded-profile-absent", "apparmor-disabled", "enabled-fact-refused",
               "sysctl-refused", "held-identity-changed", "descriptor-close-refused", "measurement-changed"}
    if (type(value["schema_version"]) is not int or value["schema_version"] != 1
            or value["status"] not in ("observed", "refused") or type(value["reason"]) is not str or value["reason"] not in reasons
            or (value["status"] == "observed") != (value["reason"] == "none")
            or value["reference_profile_sha256"] != HOST_POLICY_REFERENCE
            or value["native_qualified"] is not False or value["loaded_equals_disk_qualified"] is not False):
        raise ValueError("host policy status/qualification refused")
    for name in ("before", "after"):
        snap = value[name]
        keys(snap, "files directories entries sysctls grammar known_profile_match loaded_vscode_mode apparmor_enabled code_absent")
        keys(snap["files"], "code local_code abi global alias")
        for record in snap["files"].values():
            keys(record, "state sha256 identity")
            if record["state"] == "present":
                if type(record["sha256"]) is not str or not re.fullmatch(r"[0-9a-f]{64}", record["sha256"]):
                    raise ValueError("host policy digest refused")
                identity(record["identity"])
            elif record["state"] not in ("absent", "unmeasured", "refused") or record["sha256"] != "" or record["identity"] is not None:
                raise ValueError("host policy file state refused")
        keys(snap["directories"], "root usr usr_share")
        for record in snap["directories"].values():
            keys(record, "identity mount_id")
            if record["identity"] is None:
                if type(record["mount_id"]) is not int or record["mount_id"] != 0:
                    raise ValueError("host policy absent mount refused")
            else:
                identity(record["identity"], directory=True)
                integer(record["mount_id"], (1 << 64) - 1, 1)
        keys(snap["entries"], "disable_code force_complain_code")
        if any(item not in ("absent", "symlink", "ordinary", "other", "unmeasured", "refused") for item in snap["entries"].values()):
            raise ValueError("host policy entry refused")
        keys(snap["sysctls"], "apparmor_userns apparmor_unconfined userns_clone max_user_namespaces")
        for key, number in snap["sysctls"].items():
            if number is not None:
                integer(number, 2147483647 if key == "max_user_namespaces" else 1)
        if (snap["grammar"] not in ("unmeasured", "refused", "known-ubuntu-vscode-userns")
                or type(snap["known_profile_match"]) is not bool
                or snap["known_profile_match"] != (snap["files"]["code"]["sha256"] == HOST_POLICY_REFERENCE)
                or snap["loaded_vscode_mode"] not in ("unmeasured", "", "absent", "unconfined", "enforce", "complain", "kill")
                or any(item is not None and type(item) is not bool for item in (snap["apparmor_enabled"], snap["code_absent"]))):
            raise ValueError("host policy projection refused")
        if value["status"] == "observed" and (snap["grammar"] != "known-ubuntu-vscode-userns"
                or snap["files"]["code"]["state"] != "present" or snap["loaded_vscode_mode"] in ("unmeasured", "", "absent")
                or snap["apparmor_enabled"] is not True or snap["code_absent"] is None
                or any(record["state"] not in ("present", "absent") for record in snap["files"].values())
                or any(record["identity"] is None for record in snap["directories"].values())
                or any(number is None for number in snap["sysctls"].values())
                or any(state in ("unmeasured", "refused") for state in snap["entries"].values())):
            raise ValueError("incomplete host policy marked observed")
    if value["status"] == "observed" and value["before"] != value["after"]:
        raise ValueError("changed host policy marked observed")
    return value


def admit_native_host_policy(directory, collection_complete):
    """Called inside publication's refusal boundary, including failed captures."""
    try:
        body = toolchain.ordinary_bytes(Path(directory) / "trusted-host-policy.json", 16 << 10)
    except FileNotFoundError:
        if collection_complete is False:
            return None  # controller may have refused before its trusted-only hook
        raise ValueError("successful capture omitted host policy measurement") from None
    value = validate_host_policy(body)
    if collection_complete is not False and value["status"] != "observed":
        raise ValueError("successful capture has refused host policy measurement")
    return value


def controller_record(body):
    if type(body) is not bytes or len(body) > 1 << 20:
        raise ValueError("trusted controller record bound refused")
    def finite(text):
        value = float(text)
        if not math.isfinite(value):
            raise ValueError("nonfinite trusted controller number")
        return value
    value = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=shared.legacy.unique_object,
                       parse_constant=shared.legacy.invalid_constant, parse_float=finite)
    if type(value) is not dict:
        raise ValueError("trusted controller record is not an object")
    return value


def controller_result(body, commit, tree):
    value = controller_record(body)
    if (not isinstance(value, dict) or value.get("evidence_kind") != "vscode-pinned-trusted-project-gateway-lifecycle"
            or value.get("mode") != MODE or value.get("source_commit") != commit or value.get("source_tree") != tree
            or value.get("version") != publisher.VERSION or value.get("publisher_commit") != publisher.COMMIT
            or value.get("collection_complete") is not True or value.get("activation") != "prepared"
            or value.get("user_public_state") != "not-installed" or value.get("user_native_install_qualified") is not False
            or value.get("vendor_authentication_qualified") is not False or value.get("real_model_qualified") is not False
            or value.get("qualification") != {name: True for name in QUALIFICATIONS}
            or value.get("cleanup", {}).get("complete") is not True or value.get("cleanup", {}).get("error") != ""):
        raise ValueError("trusted controller did not prove exact full lifecycle, source and cleanup")
    validate_publisher_view(value.get("publisher_view"), True)
    if any(type(value["qualification"][name]) is not bool for name in QUALIFICATIONS):
        raise ValueError("trusted qualification must be exact booleans")
    children = value.get("children")
    if not isinstance(children, list) or len(children) != len(PUBLIC) + len(GUI) + 1:
        raise ValueError("trusted controller required children are missing")
    labels = set()
    for child in children:
        if (not isinstance(child, dict) or child.get("label") in labels or child.get("tree_reaped") is not True
                or child.get("merged_overflow") is not False or child.get("error") != ""):
            raise ValueError("trusted child failed capture or cleanup")
        labels.add(child.get("label"))
    if labels != {*PUBLIC, *GUI, "Xvfb"} or value.get("completed_stages") != list(GUI):
        raise ValueError("trusted controller required stage identity/order differs")
    return value


def validate_publisher_view(view, complete, policy=None):
    if type(complete) is not bool:
        raise ValueError("private publisher view status refused")
    if view is None and complete is False:
        return
    closed_artifact(view, ('scope', 'mount_namespace', 'original_mount_namespace',
        'original_mount_id', 'original_share_identity', 'acquired_mounts', 'released',
        'remaining_mounts', 'mount_identity_uncertain', 'label_required',
        'sudo_parent_pid', 'sudo_parent_executable_sha256'))
    def number(value, minimum=0, maximum=(1 << 64) - 1):
        if type(value) is not int or not minimum <= value <= maximum:
            raise ValueError('private publisher view integer refused')
    if view['scope'] != 'private-publisher-view-v1' or view['label_required'] != 'vscode (unconfined)':
        raise ValueError('private publisher view scope refused')
    for key in ('mount_namespace', 'original_mount_namespace'):
        number(view[key], 1)
    if view['mount_namespace'] == view['original_mount_namespace']:
        raise ValueError('publisher namespace is not private')
    number(view['original_mount_id'])
    number(view['sudo_parent_pid'], maximum=(1 << 31) - 1)
    if type(view['sudo_parent_executable_sha256']) is not str or (view['sudo_parent_executable_sha256'] and not re.fullmatch(r'[0-9a-f]{64}', view['sudo_parent_executable_sha256'])):
        raise ValueError('publisher actual sudo identity digest refused')
    number(view['remaining_mounts'], maximum=3)
    if type(view['released']) is not bool or type(view['mount_identity_uncertain']) is not bool:
        raise ValueError('publisher release proof is not literal boolean')
    identity = view['original_share_identity']
    closed_artifact(identity, ('device', 'inode', 'mode', 'uid', 'gid', 'links', 'size', 'mtime_ns', 'ctime_ns'))
    for key in ('device', 'inode', 'links'):
        number(identity[key])
    for key in ('mode', 'uid', 'gid'):
        number(identity[key], maximum=(1 << 32) - 1)
    number(identity['size'], maximum=(1 << 63) - 1)
    for key in ('mtime_ns', 'ctime_ns'):
        number(identity[key], -(1 << 63), (1 << 63) - 1)
    mounts = view['acquired_mounts']
    if mounts is None and complete is False:
        mounts = []
    if type(mounts) is not list or len(mounts) > 3:
        raise ValueError('publisher acquired mount count refused')
    ids = set()
    for index, mount in enumerate(mounts):
        closed_artifact(mount, ('kind', 'mount_id', 'parent_mount_id'))
        number(mount['mount_id'], 1)
        number(mount['parent_mount_id'], 1)
        if (mount['kind'] != ('lower', 'overlay', 'publisher')[index]
                or mount['mount_id'] == mount['parent_mount_id'] or mount['mount_id'] in ids
                or mount['mount_id'] == view['original_mount_id']):
            raise ValueError('publisher acquired mount chain refused')
        ids.add(mount['mount_id'])
        if index == 1 and mount['parent_mount_id'] != view['original_mount_id']:
            raise ValueError('publisher overlay original mount differs')
        if index == 2 and mount['parent_mount_id'] != mounts[1]['mount_id']:
            raise ValueError('publisher canonical parent mount differs')
    if complete is True and (view['released'] is not True or view['remaining_mounts'] != 0
            or view['mount_identity_uncertain'] is not False or len(mounts) != 3
            or view['original_mount_id'] == 0 or view['sudo_parent_pid'] <= 1
            or not view['sudo_parent_executable_sha256'] or identity['uid'] != 0 or identity['gid'] != 0
            or identity['mode'] != stat.S_IFDIR | 0o777 or identity['inode'] == 0):
        raise ValueError('publisher full release/ownership not proved')
    if policy is not None and view['original_mount_id']:
        expected = policy['before']['directories']['usr_share']
        if identity != expected['identity'] or view['original_mount_id'] != expected['mount_id']:
            raise ValueError('publisher view and independent host receipt differ')

def admit_native_publisher_view(directory, complete, policy):
    try:
        body = toolchain.ordinary_bytes(Path(directory) / "vscode-workspace-controller.json", 1 << 20)
    except FileNotFoundError:
        if complete is False:
            return
        raise ValueError("successful capture omitted publisher controller") from None
    controller = controller_record(body)
    validate_publisher_view(controller.get("publisher_view"), complete, policy)


class Preparation:
    def __init__(self, root, output, origin_ns):
        self.root, self.output, self.origin_ns = root, output, origin_ns
        self.identities, self.records = {}, []
        self.env = {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8", "HOME": str(root / "build-home"),
                    "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_TERMINAL_PROMPT": "0"}
        (root / "build-home").mkdir(mode=0o700)

    def capture(self, label, argv, cwd, env, seconds):
        if label not in PREPARATION or any(row["probe"] == label for row in self.records):
            raise ValueError("unregistered or duplicate trusted preparation child")
        record, stdout, stderr = shared.run_probe([str(part) for part in argv], cwd, env, self.output, label,
                                                  timeout=min(seconds, remaining(self.origin_ns, 840)), limit=1 << 20)
        self.records.append(record)
        if record["error"] is not None or record["exit"] != 0:
            raise ValueError("bounded preparation child failed: " + label)
        return stdout, stderr

    def source(self):
        facts = {}
        for label, args in (("source-commit", ("rev-parse", "HEAD")), ("source-tree", ("rev-parse", "HEAD^{tree}")),
                            ("source-clean", ("status", "--porcelain", "--untracked-files=normal"))):
            stdout, stderr = self.capture(label, ["/usr/bin/git", *args], SOURCE, self.env, 10)
            if stderr:
                raise ValueError("source identity query emitted diagnostics")
            facts[label] = stdout.decode("ascii", "strict").strip()
        if (facts["source-commit"] != os.environ["EXPECTED_COMMIT"] or not legacy.SOURCE_RE.fullmatch(facts["source-tree"])
                or facts["source-clean"] or (os.environ.get("EXPECTED_TREE") and facts["source-tree"] != os.environ["EXPECTED_TREE"])):
            raise ValueError("exact clean trusted source commit/tree required")
        return facts["source-tree"]

    def build(self):
        selected = shutil.which("go")
        if selected is None:
            raise ValueError("pinned Go compiler missing")
        go = Path(selected).resolve(strict=True)
        before = shared.artifacts.identity(go.lstat())
        digest = hashlib.sha256(toolchain.ordinary_bytes(go, 64 << 20)).hexdigest()

        def unchanged():
            if (shared.artifacts.identity(go.lstat()) != before
                    or hashlib.sha256(toolchain.ordinary_bytes(go, 64 << 20)).hexdigest() != digest):
                raise ValueError("Go compiler changed during trusted builds")

        env = {**self.env, "PATH": str(go.parent) + ":/usr/bin:/bin", "GOTOOLCHAIN": "local", "GOENV": "off",
               "GOCACHE": str(self.root / "go-cache"), "GOMODCACHE": str(self.root / "go-modules"),
               "GOMAXPROCS": "2", "GOMEMLIMIT": "768MiB", "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org"}
        unchanged()
        stdout, stderr = self.capture("go-version", [go, "version"], SOURCE, env, 10)
        unchanged()
        if stdout != b"go version go1.26.11 linux/amd64\n" or stderr:
            raise ValueError("exact Go 1.26.11 Linux amd64 required")
        executable = self.root / "executables"
        executable.mkdir(mode=0o700)
        controller, installer, probe = (self.root / "controller", executable / "agentplugins", executable / "native-probe")
        builds = (("controller-build", SOURCE, controller, "./install/integrationctl/agentplugins/research/vscodeworkspace"),
                  ("installer-build", SOURCE / "cli", installer, "./cmd/agentplugins"),
                  ("probe-build", SOURCE, probe, "./repotests/testdata/agentplugins_native_probe"))
        for label, cwd, target, package in builds:
            argv = [go, "build", "-trimpath", "-p=1"]
            if label == "controller-build":
                argv.extend(["-ldflags", "-X main.buildCommit=" + os.environ["EXPECTED_COMMIT"]])
            unchanged()
            self.capture(label, [*argv, "-o", target, package], cwd, env, 240)
            unchanged()
            self.identities[target] = hashlib.sha256(toolchain.ordinary_bytes(target, 128 << 20)).hexdigest()
        return controller, installer, probe, {"version": "1.26.11", "sha256": digest}

    def preflight(self, materialized, budget):
        """Reuse the fixed ELF-plus-cli.js version/help isolation, never GUI flags."""
        before = legacy.verify_publisher(materialized, budget)
        (materialized / publisher.PREFIX / "code").chmod(0o700)
        parent_ns = os.stat("/proc/self/ns/net").st_ino
        probes = []
        for label, flag in publisher.PROBES:
            probe_root = self.root / label
            probe_root.mkdir(mode=0o700)
            env = publisher.probe_environment(probe_root)
            if legacy.verify_publisher(materialized, budget) != before:
                raise ValueError("publisher identity changed before preflight")
            command, argv, facts = publisher.isolated_command(self.root, label, env, flag, parent_ns)
            stdout, stderr = self.capture("publisher-" + label, command, probe_root / "cwd",
                                           {"PATH": "/usr/bin:/bin", "LANG": "C.UTF-8"}, 20)
            record = self.records[-1]
            if not publisher.validate_probe(record, stdout, stderr, label):
                raise ValueError("fixed publisher version/help preflight failed")
            body = toolchain.ordinary_bytes(facts, publisher.MAX_METADATA)
            observed = json.loads(body, object_pairs_hook=publisher.bounds.unique_object, parse_constant=publisher.bounds.invalid_constant)
            if (observed.get("network_namespace") == parent_ns or observed.get("uid") != os.getuid()
                    or observed.get("euid") != os.getuid() or observed.get("gid") != os.getgid()
                    or observed.get("egid") != os.getgid() or observed.get("groups") != []
                    or observed.get("interfaces") != ["lo"] or observed.get("capabilities_zero") is not True
                    or observed.get("no_new_privileges") is not True):
                raise ValueError("fixed publisher preflight isolation differs")
            probes.append({"probe": label, "native_argv": argv, "isolation": observed,
                           "environment_mode": "ELECTRON_RUN_AS_NODE=1 confined to fixed version/help preflight"})
            if legacy.verify_publisher(materialized, budget) != before:
                raise ValueError("publisher identity changed during preflight")
        return probes


def acquire_scanner(root, budget):
    kind, repository, version, archive, pin, basename = publisher.native.PINS["linux-amd64"]["lintai"]
    if kind != "github" or pin != "sha256:" + SCANNER_ARCHIVE:
        raise ValueError("reviewed scanner archive differs")
    spec = {"url": "https://github.com/" + repository + "/releases/download/" + version + "/" + archive,
            "integrity": pin, "format": "tar.gz", "max_bytes": 96 << 20, "max_expanded_bytes": 256 << 20,
            "runtime": "native", "entrypoint": basename}
    receipt, contents = shared.acquire(spec, budget)
    matches = [row["name"] for row in receipt["members"] if row["kind"] == "file" and Path(row["name"]).name == basename]
    if len(matches) != 1:
        raise ValueError("exact scanner member is absent or ambiguous")
    spec["entrypoint"] = matches[0]
    binary, identities = shared.materialize(root / "scanner", spec, receipt, contents, selected_only=True)
    return binary, identities[binary], receipt


# Artifact authority is held independently of the mutable output pathname.
HANDOFF_PREFIX = 'UAP_VSCODE_ARTIFACT_HANDOFF '
DIR_FIELDS = ('device', 'inode', 'mode', 'uid', 'gid')
ZIP_FIELDS = DIR_FIELDS + ('links', 'size', 'mtime_ns', 'ctime_ns')


def artifact_identity(info, archive=False):
    row = dict(device=info.st_dev, inode=info.st_ino, mode=info.st_mode, uid=info.st_uid, gid=info.st_gid)
    if archive:
        row.update(links=info.st_nlink, size=info.st_size, mtime_ns=info.st_mtime_ns, ctime_ns=info.st_ctime_ns)
    return row


class ArtifactHandoff:
    def __init__(self, output, stack):
        self.output, self.stack, self.fds, self.identities = output, stack, {}, {}
        self.bindings = dict(frontend=None, native=None)
        self.captures = []
        flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
        for label in ('root', 'frontend', 'native'):
            fd = os.open(output if label == 'root' else label, flags,
                         **({} if label == 'root' else {'dir_fd': self.fds['root']}))
            stack.callback(os.close, fd)
            self.fds[label] = fd
            self.identities[label] = artifact_identity(os.fstat(fd))
        self.verify()

    def verify(self):
        if self.output.resolve() != self.output:
            raise ValueError('artifact output alias refused')
        for label, fd in self.fds.items():
            current = self.output.lstat() if label == 'root' else os.stat(label, dir_fd=self.fds['root'], follow_symlinks=False)
            expected = self.identities[label]
            if (artifact_identity(current) != expected or artifact_identity(os.fstat(fd)) != expected
                    or expected['uid'] != os.getuid() or expected['gid'] != os.getgid()
                    or expected['mode'] != stat.S_IFDIR | 0o700):
                raise ValueError('original artifact directory changed')

        for label, upload, archive, binding in self.captures:
            if (artifact_identity(os.fstat(upload)) != binding['upload']
                    or artifact_identity(os.stat('upload', dir_fd=self.fds[label], follow_symlinks=False)) != binding['upload']
                    or artifact_identity(os.fstat(archive), True) != binding['zip']
                    or artifact_identity(os.stat('research.zip', dir_fd=upload, follow_symlinks=False), True) != binding['zip']):
                raise ValueError('held upload or ZIP changed')

    def clear(self, label):
        self.bindings[label] = None
        self.captures = [capture for capture in self.captures if capture[0] != label]

    def archive(self, label, deadline):
        self.verify()
        flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC
        upload = os.open('upload', flags, dir_fd=self.fds[label])
        self.stack.callback(os.close, upload)
        upload_id = artifact_identity(os.fstat(upload))
        if upload_id['uid'] != os.getuid() or not stat.S_ISDIR(upload_id['mode']):
            raise ValueError('archive parent owner refused')
        fd = os.open('research.zip', os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC, dir_fd=upload)
        self.stack.callback(os.close, fd)
        before = artifact_identity(os.fstat(fd), True)
        if (not stat.S_ISREG(before['mode']) or before['links'] != 1 or before['uid'] != os.getuid()
                or not 0 < before['size'] <= 32 << 20):
            raise ValueError('archive identity refused')
        chunks, size = [], 0
        while True:
            if time.monotonic() >= deadline:
                raise ValueError('artifact copy deadline')
            chunk = os.read(fd, min(1 << 20, (32 << 20) + 1 - size))
            if not chunk:
                break
            chunks.append(chunk); size += len(chunk)
            if size > 32 << 20:
                raise ValueError('artifact ZIP bound')
        if size != before['size']:
            raise ValueError('archive length changed')
        body = b''.join(chunks)
        binding = {'upload': upload_id, 'zip': before, 'bytes': size, 'sha256': hashlib.sha256(body).hexdigest()}
        self.captures.append((label, upload, fd, binding))
        self.verify()
        return binding, body

    def bind(self, label, deadline):
        binding, _ = self.archive(label, deadline)
        self.bindings[label] = binding

    def receipt(self, summary):
        self.verify()
        deadline = time.monotonic() + 5
        for label, binding in self.bindings.items():
            if binding is not None and self.archive(label, deadline)[0] != binding:
                raise ValueError('bound artifact changed')
        value = dict(schema_version=1, source_commit=summary['source_commit'], source_tree=summary['source_tree'],
                     collection_complete=summary['collection_complete'], output=self.identities, archives=self.bindings)
        body = json.dumps(value, sort_keys=True, separators=(',', ':')).encode()
        if len(body) > 16 << 10:
            raise ValueError('artifact receipt bound')
        return body


def closed_artifact(value, keys):
    if type(value) is not dict or set(value) != set(keys):
        raise ValueError('closed artifact fields refused')


def validate_handoff(value, commit, tree):
    closed_artifact(value, ('schema_version', 'source_commit', 'source_tree', 'collection_complete', 'output', 'archives'))
    if (type(value['schema_version']) is not int or value['schema_version'] != 1
            or value['source_commit'] != commit or value['source_tree'] != tree
            or not re.fullmatch('[0-9a-f]{40}', commit) or not re.fullmatch('[0-9a-f]{40}', tree)
            or type(value['collection_complete']) is not bool):
        raise ValueError('artifact source or status refused')
    closed_artifact(value['output'], ('root', 'frontend', 'native'))
    closed_artifact(value['archives'], ('frontend', 'native'))
    def identity(row, archive=False):
        closed_artifact(row, ZIP_FIELDS if archive else DIR_FIELDS)
        if any(type(number) is not int for number in row.values()):
            raise ValueError('artifact identity types refused')
        if row['device'] < 0 or row['inode'] <= 0 or row['uid'] != os.getuid() or row['gid'] != os.getgid():
            raise ValueError('artifact identity owner refused')
        if archive:
            if not stat.S_ISREG(row['mode']) or row['links'] != 1 or not 0 < row['size'] <= 32 << 20:
                raise ValueError('artifact ZIP identity refused')
        elif not stat.S_ISDIR(row['mode']):
            raise ValueError('artifact directory type refused')
    for row in value['output'].values():
        identity(row)
    for binding in value['archives'].values():
        if binding is None:
            continue
        closed_artifact(binding, ('upload', 'zip', 'bytes', 'sha256'))
        identity(binding['upload']); identity(binding['zip'], True)
        if (type(binding['bytes']) is not int or binding['bytes'] != binding['zip']['size']
                or type(binding['sha256']) is not str or not re.fullmatch('[0-9a-f]{64}', binding['sha256'])):
            raise ValueError('artifact digest refused')
    return value


READOUT_PHASES = frozenset(('publisher-startup native-devtools-admission workbench-focus workspace-trust native-mcp-start mcp-command-palette mcp-server-picker mcp-server-selection mcp-action-picker mcp-start-action mcp-trust mcp-state ui-trigger-publication observer-result publisher-shutdown driver-result-publication parent-planned-shutdown').split())
READOUT_REASONS = frozenset(('unclassified publisher-startup-failed publisher-spawn-failed publisher-stream-overflow publisher-proof-bound publisher-proof-missing publisher-proof-mismatch publisher-stream-malformed publisher-pre-verifier-stderr publisher-endpoint-ambiguous publisher-endpoint-refused publisher-port-refused stage-deadline operation-deadline browser-context-mismatch workbench-pages-ambiguous workbench-page-missing publisher-shutdown-incomplete publisher-terminal-result-mismatch observer-deadline driver-output-bound picker-input-deadline picker-input-ambiguous picker-label-missing picker-label-ambiguous picker-row-bound ui-selection-refused ui-dialog-unexpected ui-dialog-unplanned mcp-trust-origin mcp-state-refused mcp-start-deadline trigger-publication-refused').split())
READOUT_AWAIT_BOUNDARIES = frozenset(('unobserved server-catalog server-selection action-picker start-precheck start-selection dialog-count trust-dialog trust-origin trust-accept trust-hidden input-count input-placeholder launch-diagnostic catalog-reopen state-row state-description state-dismiss state-dialog').split())


def native_log_readout(archive, names, deadline, commit, tree):
    if 'vscode-workspace-controller.json' not in names:
        return {}
    member = archive.getinfo('vscode-workspace-controller.json')
    if member.is_dir() or member.flag_bits & 1 or not 0 < member.file_size <= 1 << 20:
        raise ValueError('native log controller bound')
    with archive.open(member) as stream:
        raw = stream.read((1 << 20) + 1)
    if len(raw) != member.file_size or time.monotonic() >= deadline:
        raise ValueError('native log controller incomplete')
    value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=publisher.bounds.unique_object,
                       parse_constant=publisher.bounds.invalid_constant)
    if type(value) is not dict:
        raise ValueError('native log controller type')
    rows, result, seen = value.get('observations'), {}, set()
    if type(rows) is not list or len(rows) > len(GUI):
        raise ValueError('native log observation count')
    if commit is not None and (value.get('source_commit') != commit or value.get('source_tree') != tree
            or value.get('mode') != 'trusted-project-v1'
            or value.get('evidence_kind') != 'vscode-pinned-trusted-project-gateway-lifecycle'):
        raise ValueError('native log controller source')
    for row in rows:
        if type(row) is not dict or row.get('stage') not in GUI or row['stage'] in seen:
            raise ValueError('native log stage join')
        seen.add(row['stage'])
        facts = row.get('native_log')
        if facts is None:
            continue
        if commit is None or type(row.get('nonce')) is not str or not 0 < len(row['nonce']) <= 128:
            raise ValueError('native log attribution missing')
        closed_artifact(facts, 'status transport_start last_state stopped starting running error'.split())
        if (facts['status'] not in ('unavailable', 'observed') or type(facts['transport_start']) is not bool
                or facts['last_state'] not in ('none', 'Stopped', 'Starting', 'Running', 'Error')
                or any(type(facts[key]) is not int or not 0 <= facts[key] <= 255 for key in ('stopped', 'starting', 'running', 'error'))
                or sum(facts[key] for key in ('stopped', 'starting', 'running', 'error')) > 255
                or (facts['last_state'] != 'none' and facts[facts['last_state'].lower()] == 0)
                or (facts['status'] == 'unavailable' and facts != dict(status='unavailable', transport_start=False, last_state='none', stopped=0, starting=0, running=0, error=0))):
            raise ValueError('native log finite facts')
        result[row['stage']] = (dict(facts), row['nonce'])
    return result


def closed_native_readout(body, deadline, commit=None, tree=None):
    """Projection of already receipt-bound native bytes, not a new native proof."""
    if not isinstance(body, bytes) or not 0 < len(body) <= 32 << 20:
        raise ValueError('readout ZIP bound refused')
    if len(body) < 22 or body[-22:-18] != b'PK\x05\x06':
        raise ValueError('readout ZIP end record refused')
    disk, central_disk, disk_count, total, central_size, offset, comment = struct.unpack_from('<4H2LH', body, len(body) - 18)
    if (disk or central_disk or comment or disk_count != total or total > len(native_names())
            or central_size > 16384 or offset + central_size != len(body) - 22):
        raise ValueError('readout ZIP metadata bound refused')
    rows = []
    with zipfile.ZipFile(io.BytesIO(body)) as archive:
        members = archive.infolist()
        names = [item.filename for item in members]
        if len(names) > len(native_names()) or len(set(names)) != len(names) or not set(names) <= native_names():
            raise ValueError('readout closed members refused')
        native_logs = native_log_readout(archive, names, deadline, commit, tree)
        for stage in GUI:
            if time.monotonic() >= deadline:
                raise ValueError('readout original staging deadline')
            name = stage + '.observation.json'
            if name not in names:
                continue
            member = archive.getinfo(name)
            if member.is_dir() or member.flag_bits & 1 or not 0 < member.file_size <= 1 << 20:
                raise ValueError('readout observation bound refused')
            with archive.open(member) as stream:
                raw = stream.read((1 << 20) + 1)
            if len(raw) != member.file_size or time.monotonic() >= deadline:
                raise ValueError('readout observation incomplete')
            value = json.loads(raw.decode('utf-8', 'strict'), object_pairs_hook=publisher.bounds.unique_object,
                               parse_constant=publisher.bounds.invalid_constant)
            if type(value) is not dict:
                raise ValueError('readout observation type refused')
            if value.get('type') in ('driver-result', 'observer-partial'):
                continue
            if (set(value) != set('type stage nonce phase code visibleNativeUI diagnostics'.split())
                    or value.get('type') != 'driver-failure' or value.get('stage') != stage
                    or value.get('code') != 'closed-stage-contract-refused'
                    or type(value.get('phase')) is not str or value['phase'] not in READOUT_PHASES):
                raise ValueError('readout failure context refused')
            facts = value['diagnostics']
            if type(facts) is not dict or type(facts.get('reason')) is not str or facts['reason'] not in READOUT_REASONS:
                raise ValueError('readout reason refused')
            mcp = facts.get('mcp')
            if (type(mcp) is not dict or set(mcp) != set('startSelected trustAccepted lastState stopped starting running other launchNotification outputVisible launchError postStartInput awaitBoundary elapsedMs'.split())
                    or any(type(mcp[key]) is not bool for key in ('startSelected', 'trustAccepted', 'launchNotification', 'outputVisible'))
                    or type(mcp['launchError']) is not str or mcp['launchError'] not in ('none', 'variable-resolution', 'native-start', 'command-not-found', 'other', 'diagnostic-refused')
                    or type(mcp['postStartInput']) is not str or mcp['postStartInput'] not in ('unobserved', 'none', 'action-picker', 'trust-picker', 'other', 'multiple')
                    or type(mcp['awaitBoundary']) is not str or mcp['awaitBoundary'] not in READOUT_AWAIT_BOUNDARIES
                    or type(mcp['elapsedMs']) is not int or not 0 <= mcp['elapsedMs'] <= 15001
                    or type(mcp['lastState']) is not str or mcp['lastState'] not in ('none', 'Stopped', 'Starting', 'Running', 'other')
                    or any(type(mcp[key]) is not int or not 0 <= mcp[key] <= 255 for key in ('stopped', 'starting', 'running', 'other'))):
                raise ValueError('readout MCP facts refused')
            row = dict(stage=stage, phase=value['phase'], reason=facts['reason'], mcp=dict(mcp))
            if stage in native_logs:
                native_log, nonce = native_logs[stage]
                if nonce != value.get('nonce'):
                    raise ValueError('native log failure nonce differs')
                row['native_log'] = native_log
            rows.append(row)
    if time.monotonic() >= deadline or len(json.dumps(rows).encode('ascii')) > 16384:
        raise ValueError('readout result bound refused')
    return rows


def stage_handoff(value, output, parent, commit, tree, *, readout=None):
    """Trusted parent only; the receipt never chooses source or target paths."""
    validate_handoff(value, commit, tree)
    deadline, paths = time.monotonic() + 5, {}
    projection, native_body = {'status': 'unavailable', 'failures': []}, None
    stage = Path(tempfile.mkdtemp(prefix='vscode-admitted-', dir=parent))
    stage_id = artifact_identity(stage.lstat())
    try:
        with contextlib.ExitStack() as stack:
            destination = os.open(stage, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_CLOEXEC)
            stack.callback(os.close, destination)
            held = ArtifactHandoff(output, stack)
            if held.identities != value['output']:
                raise ValueError('original output root replaced')
            for label in ('frontend', 'native'):
                binding = value['archives'][label]
                if binding is None:
                    continue
                observed, body = held.archive(label, deadline)
                if observed != binding:
                    raise ValueError('ZIP receipt differs')
                if label == 'native' and readout is not None:
                    native_body = body
                name = label + '.zip'
                fd = os.open(name, os.O_RDWR | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW | os.O_CLOEXEC, 0o600, dir_fd=destination)
                with os.fdopen(fd, 'w+b') as stream:
                    stream.write(body); stream.flush(); stream.seek(0)
                    if hashlib.sha256(stream.read((32 << 20) + 1)).hexdigest() != binding['sha256']:
                        raise ValueError('staged ZIP differs')
                    identity = artifact_identity(os.fstat(stream.fileno()), True)
                    if identity['size'] != binding['bytes'] or artifact_identity(os.stat(name, dir_fd=destination, follow_symlinks=False), True) != identity:
                        raise ValueError('staged ZIP identity differs')
                paths[label] = str(stage / name)
            held.verify()
            if artifact_identity(os.fstat(destination)) != stage_id:
                raise ValueError('staging descriptor changed')
        if artifact_identity(stage.lstat()) != stage_id or time.monotonic() >= deadline:
            raise ValueError('staging authority or deadline refused')
        if readout is not None:
            if native_body is not None:
                try:
                    projection = {'status': 'observed', 'failures': closed_native_readout(native_body, deadline, commit, tree)}
                except Exception:
                    # Diagnostic refusal cannot discard already admitted archives or expose raw text.
                    projection = {'status': 'refused', 'failures': []}
            readout.append(projection)
        return paths
    except BaseException:
        if artifact_identity(stage.lstat()) == stage_id:
            shutil.rmtree(stage)
        raise


def trusted_mount_launcher(stack):
    path = Path('/usr/bin/unshare').resolve(strict=True)
    for parent in path.parents:
        info = parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or stat.S_IMODE(info.st_mode) & 0o22:
            raise ValueError('fixed mount launcher ancestor refused')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_CLOEXEC)
    stack.callback(os.close, fd)
    before = artifact_identity(os.fstat(fd), True)
    if (not stat.S_ISREG(before['mode']) or before['uid'] != 0 or before['links'] != 1
            or before['mode'] & 0o22 or not before['mode'] & 0o111 or not 0 < before['size'] <= 128 << 20):
        raise ValueError('fixed mount launcher authority refused')
    body = os.read(fd, (128 << 20) + 1)
    if (len(body) != before['size'] or artifact_identity(os.fstat(fd), True) != before
            or artifact_identity(path.lstat(), True) != before):
        raise ValueError('fixed mount launcher changed')
    return str(path), {'path': str(path), 'sha256': hashlib.sha256(body).hexdigest()}


def quarantine_artifact(directory, identity, handoff=None):
    if handoff is not None:
        handoff.verify()
    legacy.quarantine_output(directory, identity)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path)
    parser.add_argument("--contracts", action="store_true")
    args = parser.parse_args()
    if args.contracts:
        return contracts()
    if args.output is None:
        parser.error("--output required for trusted native observation")
    try:
        guard()
    except (RuntimeError, ValueError, OSError) as error:
        print(type(error).__name__ + ": " + str(error)[:512], file=sys.stderr)
        return 2
    origin_ns = time.monotonic_ns()
    output = args.output.absolute()
    if output.resolve() != output or output.exists() or SOURCE == output or SOURCE in output.parents:
        raise ValueError("fresh canonical trusted output outside checkout required")
    output.mkdir(mode=0o700, parents=True)
    output_identity = output.lstat()
    frontend, native = output / "frontend", output / "native"
    frontend.mkdir(mode=0o700)
    native.mkdir(mode=0o700)
    native_identity, frontend_identity = native.lstat(), frontend.lstat()
    with contextlib.ExitStack() as stack:
        handoff = ArtifactHandoff(output, stack)
        return run_capture(output, origin_ns, handoff)


def run_capture(output, origin_ns, handoff):
    frontend, native = output / 'frontend', output / 'native'
    output_identity, frontend_identity, native_identity = output.lstat(), frontend.lstat(), native.lstat()
    summary = {"evidence_kind": "vscode-trusted-project-frontend", "mode": MODE, "source_commit": os.environ["EXPECTED_COMMIT"],
               "source_tree": os.environ["EXPECTED_TREE"],
               "version": publisher.VERSION, "publisher_commit": publisher.COMMIT, "collection_complete": False,
               "host": {"ImageOS": os.environ.get("ImageOS"), "ImageVersion": os.environ.get("ImageVersion")},
               "qualification": {name: False for name in QUALIFICATIONS}, "vendor_authentication_qualified": False,
               "real_model_qualified": False, "user_public_state": "not-installed", "user_native_install_qualified": False}
    root, prep, frontend_hash = None, None, None
    frontend_publication_attempted = False
    try:
        root = Path(tempfile.mkdtemp(prefix="vscode-workspace-input-", dir="/tmp"))
        root_identity = root.lstat()
        prep = Preparation(root, frontend, origin_ns)
        with publisher.bounds.Budget(seconds=remaining(origin_ns, 840)) as budget:
            tree = prep.source()
            summary["source_tree"] = tree
            if shutil.disk_usage(root).free < publisher.MIN_FREE:
                raise ValueError("at least four GiB free required")
            archive = publisher.acquire_archive(root, budget)
            materialized = root / "publisher"
            materialized.mkdir(mode=0o700)
            summary["archive"] = {"url": publisher.URL, "bytes": publisher.ARCHIVE_BYTES, "sha256": publisher.ARCHIVE_SHA256}
            summary["materialization"] = publisher.materialize_archive(archive, materialized, budget)
            summary["publisher_files"] = legacy.verify_publisher(materialized, budget)
            summary["publisher_preflight"] = prep.preflight(materialized, budget)
            controller, installer, probe, summary["compiler"] = prep.build()
            tools, summary["toolchain"] = toolchain.prepare(root / "compiler", SOURCE / "scripts/vscode-project-runtime-observer", budget, prep.capture)
            scanner, scanner_sha, summary["scanner"] = acquire_scanner(root, budget)
            prep.identities[scanner] = scanner_sha
            shared.verify_files(prep.identities, budget)
            stdout, stderr = prep.capture("source-final-clean", ["/usr/bin/git", "status", "--porcelain", "--untracked-files=normal"], SOURCE, prep.env, 10)
            if stdout or stderr:
                raise ValueError("trusted preparation modified source checkout")
            trusted = {"tools_root": str(tools), "tools_manifest_sha256": summary["toolchain"]["tools_manifest_sha256"],
                       "installer_path": str(installer), "installer_sha256": prep.identities[installer],
                       "probe_path": str(probe), "probe_sha256": prep.identities[probe],
                       "scanner_path": str(scanner), "scanner_sha256": scanner_sha, "scanner_archive_sha256": SCANNER_ARCHIVE}
            request = {"mode": MODE, "uid": os.getuid(), "gid": os.getgid(), "euid": os.geteuid(), "egid": os.getegid(),
                       "start_monotonic_ns": origin_ns, "source_commit": os.environ["EXPECTED_COMMIT"], "source_tree": tree,
                       "observer_root": "", "output_root": str(native),
                       "original_mount_namespace": os.stat('/proc/self/ns/mnt').st_ino, "helper_sha256": prep.identities[controller], "trusted": trusted}
            toolchain.write_new(root / "request.json", json.dumps(request, sort_keys=True).encode() + b"\n")
            frontend_summary = {**summary, "evidence_kind": "vscode-trusted-project-prepared-inputs", "build_complete": True,
                                "collection_complete": True, "commands": prep.records, "trusted_inputs": trusted}
            toolchain.write_new(frontend / "vscode-project-frontend.json", json.dumps(frontend_summary, sort_keys=True, indent=2).encode() + b"\n")
            frontend_publication_attempted = True
            if not publish_frontend_capture(frontend, frontend_identity, frontend_summary, handoff):
                raise ValueError("trusted source/build input artifact admission failed")
            frontend_hash = hashlib.sha256(toolchain.ordinary_bytes(frontend / "upload/research.zip", 32 << 20)).hexdigest()
            if remaining(origin_ns, 840) < 75:
                raise ValueError("trusted controller cannot start inside its cleanup reserve")
        launcher, summary['mount_launcher'] = trusted_mount_launcher(handoff.stack)
        argv = ["/usr/bin/sudo", "-n", "--preserve-env=" + ",".join(legacy.GUARD_KEYS), launcher, "--mount", "--propagation", "private", "--", str(controller), "--input-root", str(root)]
        summary["controller"] = legacy.capture_controller(argv, SOURCE, native, origin_ns, trusted_project=True, memory_only=True)
        for name, body in summary["controller"].pop("_streams").items():
            write_owned(native, native_identity, "controller." + name + ".log", body)
        if summary["controller"]["error"] is not None:
            raise ValueError("trusted fixed controller failed; bounded evidence retained")
        result = controller_result(toolchain.ordinary_bytes(native / "vscode-workspace-controller.json", 1 << 20), os.environ["EXPECTED_COMMIT"], tree)
        if not native_names() - {"vscode-project-research.json", "cleanup.stdout.log", "cleanup.stderr.log"} <= {path.name for path in native.iterdir()}:
            raise ValueError("successful trusted controller omitted required raw observations")
        summary["qualification"], summary["collection_complete"] = result["qualification"], True
    except Exception as error:
        summary["collection_complete"] = False
        summary["error"] = type(error).__name__ + ": " + str(error)[:2048]
    finally:
        cleanup_streams = {}
        if root is not None:
            try:
                summary["frontend_cleanup"] = legacy.cleanup_frontend(root, root_identity, native, memory_only=True)
                cleanup_streams = summary["frontend_cleanup"].pop("_streams")
            except Exception as error:
                summary["frontend_cleanup"] = {"complete": False, "error": type(error).__name__}
            if summary["frontend_cleanup"].get("complete") is not True:
                summary["collection_complete"] = False
        # Preparation cleanup owns an independent identity. Artifact refusal
        # cannot suppress it, and still forbids every subsequent output write.
        handoff.verify()
        try:
            for name, body in cleanup_streams.items():
                write_owned(native, native_identity, "cleanup." + name + ".log", body)
        except Exception as error:
            summary["frontend_cleanup"] = {"complete": False, "error": type(error).__name__}
            summary["collection_complete"] = False
        if frontend_hash is not None:
            try:
                if hashlib.sha256(toolchain.ordinary_bytes(frontend / "upload/research.zip", 32 << 20)).hexdigest() != frontend_hash:
                    raise ValueError("prepared input ZIP changed after native execution")
            except Exception as error:
                summary["collection_complete"] = False
                summary["frontend_archive_error"] = type(error).__name__
                handoff.clear('frontend')
                try:
                    quarantine_artifact(frontend, frontend_identity, handoff)
                except (ValueError, OSError):
                    quarantine_artifact(output, output_identity, handoff)
                    return 2
        elif prep is not None and not frontend_publication_attempted:
            failure = {**summary, "commands": prep.records, "collection_complete": False}
            path = frontend / "vscode-project-frontend.json"
            if not path.exists():
                toolchain.write_new(path, json.dumps(failure, sort_keys=True, indent=2).encode() + b"\n")
            frontend_publication_attempted = True
            publish_frontend_capture(frontend, frontend_identity, failure, handoff)
        summary["elapsed_seconds"] = (time.monotonic_ns() - origin_ns) / 1e9
        summary["prepared_input_zip_sha256"] = frontend_hash
        if summary["elapsed_seconds"] > 900:
            summary["collection_complete"] = False
        if not summary["collection_complete"]:
            summary["qualification"] = {name: False for name in QUALIFICATIONS}
        try:
            write_owned(native, native_identity, "vscode-project-research.json", json.dumps(summary, sort_keys=True, indent=2).encode() + b"\n")
        except (ValueError, OSError):
            quarantine_artifact(output, output_identity, handoff)
            return 2
        publish_native_capture(native, native_identity, summary, handoff)
    print(handoff.receipt(summary).decode('ascii'))
    return 0 if summary["collection_complete"] else 2


def publish_frontend_capture(frontend, identity, summary, handoff=None):
    return publish_capture(frontend, identity, summary, frontend_names(), 'frontend', handoff)


def publish_native_capture(native, identity, summary, handoff=None):
    return publish_capture(native, identity, summary, native_names(), 'native', handoff)


def publish_capture(directory, identity, summary, names, label, handoff):
    if handoff is not None:
        handoff.clear(label)
    deadline = time.monotonic() + 5
    try:
        if handoff is not None:
            handoff.verify()
        if label == 'native':
            policy = admit_native_host_policy(directory, summary.get('collection_complete'))
            admit_native_publisher_view(directory, summary.get('collection_complete'), policy)
        admission = legacy.publish_from_memory(directory, summary, allowed_names=names)
        complete = admission.get('complete') is True
        retain = complete or (summary.get('collection_complete') is False
                              and admission.get('child_reaped') is True
                              and admission.get('archive_failure_allowed') is True)
        if retain and handoff is not None:
            handoff.bind(label, deadline)
    except BaseException:
        if handoff is not None:
            handoff.clear(label)
        summary['collection_complete'] = False
        summary['qualification'] = {name: False for name in QUALIFICATIONS}
        quarantine_artifact(directory, identity, handoff)
        raise
    if not retain:
        if label == 'native':
            summary['collection_complete'] = False
            summary['qualification'] = {name: False for name in QUALIFICATIONS}
        quarantine_artifact(directory, identity, handoff)
    return complete



def contracts():
    """Pure admission checks; no native process, host spoofing or acquisition."""
    if len(native_names()) != 47 or len(frontend_names()) > 64 or set(PUBLIC) & set(GUI):
        raise AssertionError("closed trusted artifact table differs")
    positive = {"evidence_kind": "vscode-pinned-trusted-project-gateway-lifecycle", "mode": MODE,
                "source_commit": "a" * 40, "source_tree": "b" * 40, "version": publisher.VERSION,
                "publisher_commit": publisher.COMMIT, "collection_complete": True, "activation": "prepared",
                "user_public_state": "not-installed", "user_native_install_qualified": False,
                "vendor_authentication_qualified": False, "real_model_qualified": False,
                "qualification": {name: True for name in QUALIFICATIONS}, "cleanup": {"complete": True, "error": ""},
                "children": [{"label": name, "tree_reaped": True, "merged_overflow": False, "error": ""}
                             for name in (*PUBLIC, *GUI, "Xvfb")], "completed_stages": list(GUI)}
    positive["publisher_view"] = {
        "scope": "private-publisher-view-v1", "mount_namespace": 2, "original_mount_namespace": 1,
        "original_mount_id": 1, "original_share_identity": {"device": 1, "inode": 2,
            "mode": stat.S_IFDIR | 0o777, "uid": 0, "gid": 0, "links": 1, "size": 0,
            "mtime_ns": 1, "ctime_ns": 1}, "acquired_mounts": [
            {"kind": "lower", "mount_id": 2, "parent_mount_id": 1},
            {"kind": "overlay", "mount_id": 3, "parent_mount_id": 1},
            {"kind": "publisher", "mount_id": 4, "parent_mount_id": 3}],
        "released": True, "remaining_mounts": 0, "mount_identity_uncertain": False,
        "label_required": "vscode (unconfined)", "sudo_parent_pid": 2,
        "sudo_parent_executable_sha256": "a" * 64}
    controller_result(json.dumps(positive).encode(), "a" * 40, "b" * 40)
    for change in ("wrong-source", "bad-cleanup", "missing-child", "wrong-order", "numeric-flag", "stale-user-prepared", "stale-add-user"):
        value = json.loads(json.dumps(positive))
        if change == "wrong-source": value["source_tree"] = "c" * 40
        elif change == "bad-cleanup": value["children"][0]["tree_reaped"] = False
        elif change == "missing-child": value["children"].pop()
        elif change == "wrong-order": value["completed_stages"].reverse()
        elif change == "stale-user-prepared": value["user_public_state"] = "prepared"
        elif change == "stale-add-user": value["children"].append({"label": "add-user", "tree_reaped": True, "merged_overflow": False, "error": ""})
        else: value["qualification"][QUALIFICATIONS[0]] = 1
        try:
            controller_result(json.dumps(value).encode(), "a" * 40, "b" * 40)
        except ValueError:
            continue
        raise AssertionError("malformed trusted proof admitted: " + change)
    for value in ({}, {"mode": MODE, "collection_complete": True}, {"mode": "unknown"}):
        try:
            controller_result(json.dumps(value).encode(), "a" * 40, "b" * 40)
        except ValueError:
            continue
        raise AssertionError("incomplete controller success admitted")
    if (legacy.ROOT_TIMEOUT, legacy.ROOT_KILL) != (460, 470):
        raise AssertionError("legacy controller bounds changed")
    if SCANNER_ARCHIVE != publisher.native.PINS["linux-amd64"]["lintai"][4].removeprefix("sha256:"):
        raise AssertionError("scanner archive pin differs")
    with tempfile.TemporaryDirectory() as temporary:
        root = Path(temporary)
        original, outside = root / "original", root / "outside"
        original.mkdir()
        outside.mkdir()
        identity = original.lstat()
        write_owned(original, identity, "controller.stdout.log", b"inert capture")
        original.rename(root / "held-original")
        original.symlink_to(outside, target_is_directory=True)
        try:
            write_owned(original, identity, "controller.stderr.log", b"must not write")
        except ValueError:
            pass
        else:
            raise AssertionError("trusted output parent alias admitted")
        if list(outside.iterdir()):
            raise AssertionError("trusted output alias wrote outside original authority")
    try:
        remaining(time.monotonic_ns() - 901_000_000_000, 900)
    except ValueError:
        return 0
    raise AssertionError("original trusted task deadline reset")


if __name__ == "__main__":
    raise SystemExit(main())
