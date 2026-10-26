#!/usr/bin/env python3
"""Pinned trusted VS Code project lifecycle through the existing strict host.

The original 900-second clock includes acquisition and builds. This frontend
does not grant trust, register MCP servers or call a probe; the reviewed native
controller owns those public-install/UI/gateway observations and all cleanup.
"""
import argparse
import hashlib
import importlib.util
import json
import math
import os
from pathlib import Path
import platform
import re
import shutil
import stat
import sys
import tempfile
import time


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
PUBLIC = ("git-a", "git-b", "add-user", "add-a", "add-b", "repeat-add-a", "unchanged-update-a",
          "intact-repair-a", "update-a", "missing-repair-a", "remove-a", "repeat-remove-a")
GUI = ("empty-a", "empty-b", "installed-a", "installed-b", "stable-a", "updated-a", "repaired-a",
       "restart-a", "removed-a", "remaining-b")
PREPARATION = ("source-commit", "source-tree", "source-clean", "source-final-clean", "go-version",
               "controller-build", "installer-build", "probe-build", "trusted-node-version", "trusted-npm-ci",
               "trusted-tsc-version", "trusted-tsc", "publisher-version", "publisher-help")
SCANNER_ARCHIVE = "3da60f749c61e2caca029a44a9ce422d570aef8c57f82ce51c411c8cec12f61b"


def guard():
    legacy.guard()
    if platform.system() != "Linux" or platform.machine() != "x86_64" or os.environ.get("ImageOS") != "ubuntu22":
        raise ValueError("trusted project host must be the actual Ubuntu 22.04 Linux x64 runner")
    release = toolchain.ordinary_bytes(Path("/etc/os-release").resolve(strict=True), 64 << 10).decode("utf-8", "strict")
    fields = dict(line.split("=", 1) for line in release.splitlines() if "=" in line)
    if fields.get("ID", "").strip('"') != "ubuntu" or fields.get("VERSION_ID", "").strip('"') != "22.04":
        raise ValueError("trusted project host release differs from Ubuntu 22.04")


def remaining(origin_ns, seconds):
    value = seconds - (time.monotonic_ns() - origin_ns) / 1e9
    if value <= 0:
        raise ValueError("original trusted project deadline exceeded")
    return value


def native_names():
    names = {"vscode-workspace-controller.json", "vscode-project-research.json", "system-executables.json", "xauth.json",
             "Xvfb.merged.log", "probe-a-events.log", "probe-b-events.log"}
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


def controller_result(body, commit, tree):
    def finite(text):
        value = float(text)
        if not math.isfinite(value):
            raise ValueError("nonfinite trusted controller number")
        return value
    value = json.loads(body.decode("utf-8", "strict"), object_pairs_hook=shared.legacy.unique_object,
                       parse_constant=shared.legacy.invalid_constant, parse_float=finite)
    if (not isinstance(value, dict) or value.get("evidence_kind") != "vscode-pinned-trusted-project-gateway-lifecycle"
            or value.get("mode") != MODE or value.get("source_commit") != commit or value.get("source_tree") != tree
            or value.get("version") != publisher.VERSION or value.get("publisher_commit") != publisher.COMMIT
            or value.get("collection_complete") is not True or value.get("activation") != "prepared"
            or value.get("user_public_state") != "prepared" or value.get("user_native_install_qualified") is not False
            or value.get("vendor_authentication_qualified") is not False or value.get("real_model_qualified") is not False
            or value.get("qualification") != {name: True for name in QUALIFICATIONS}
            or value.get("cleanup", {}).get("complete") is not True or value.get("cleanup", {}).get("error") != ""):
        raise ValueError("trusted controller did not prove exact full lifecycle, source and cleanup")
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
        if stdout != b"go version go1.26.10 linux/amd64\n" or stderr:
            raise ValueError("exact Go 1.26.10 Linux amd64 required")
        executable = self.root / "executables"
        executable.mkdir(mode=0o700)
        controller, installer, probe = (self.root / "controller", executable / "agentplugins", executable / "native-probe")
        builds = (("controller-build", SOURCE, controller, "./install/integrationctl/research/vscodeworkspace"),
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
        return controller, installer, probe, {"version": "1.26.10", "sha256": digest}

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
    kind, repository, version, archive, pin, basename = publisher.native.current_source_scanner_pin("linux-amd64")
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
    summary = {"evidence_kind": "vscode-trusted-project-frontend", "mode": MODE, "source_commit": os.environ["EXPECTED_COMMIT"],
               "version": publisher.VERSION, "publisher_commit": publisher.COMMIT, "collection_complete": False,
               "host": {"ImageOS": os.environ.get("ImageOS"), "ImageVersion": os.environ.get("ImageVersion")},
               "qualification": {name: False for name in QUALIFICATIONS}, "vendor_authentication_qualified": False,
               "real_model_qualified": False, "user_public_state": "prepared", "user_native_install_qualified": False}
    root, prep, frontend_hash = None, None, None
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
                       "observer_root": "", "output_root": str(native), "helper_sha256": prep.identities[controller], "trusted": trusted}
            toolchain.write_new(root / "request.json", json.dumps(request, sort_keys=True).encode() + b"\n")
            frontend_summary = {**summary, "evidence_kind": "vscode-trusted-project-prepared-inputs", "build_complete": True,
                                "collection_complete": True, "commands": prep.records, "trusted_inputs": trusted}
            toolchain.write_new(frontend / "vscode-project-frontend.json", json.dumps(frontend_summary, sort_keys=True, indent=2).encode() + b"\n")
            admitted = legacy.publish_from_memory(frontend, frontend_summary, allowed_names=frontend_names())
            if admitted["complete"] is not True:
                raise ValueError("trusted source/build input artifact admission failed")
            frontend_hash = hashlib.sha256(toolchain.ordinary_bytes(frontend / "upload/research.zip", 32 << 20)).hexdigest()
            if remaining(origin_ns, 840) < 75:
                raise ValueError("trusted controller cannot start inside its cleanup reserve")
        argv = ["/usr/bin/sudo", "-n", "--preserve-env=" + ",".join(legacy.GUARD_KEYS), str(controller), "--input-root", str(root)]
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
        if root is not None:
            try:
                summary["frontend_cleanup"] = legacy.cleanup_frontend(root, root_identity, native, memory_only=True)
                for name, body in summary["frontend_cleanup"].pop("_streams").items():
                    write_owned(native, native_identity, "cleanup." + name + ".log", body)
            except Exception as error:
                summary["frontend_cleanup"] = {"complete": False, "error": type(error).__name__}
            if summary["frontend_cleanup"].get("complete") is not True:
                summary["collection_complete"] = False
        if frontend_hash is not None:
            try:
                if hashlib.sha256(toolchain.ordinary_bytes(frontend / "upload/research.zip", 32 << 20)).hexdigest() != frontend_hash:
                    raise ValueError("prepared input ZIP changed after native execution")
            except Exception as error:
                summary["collection_complete"] = False
                summary["frontend_archive_error"] = type(error).__name__
                try:
                    legacy.quarantine_output(frontend, frontend_identity)
                except (ValueError, OSError):
                    legacy.quarantine_output(output, output_identity)
                    return 2
        elif prep is not None:
            failure = {**summary, "commands": prep.records, "collection_complete": False}
            path = frontend / "vscode-project-frontend.json"
            if not path.exists():
                toolchain.write_new(path, json.dumps(failure, sort_keys=True, indent=2).encode() + b"\n")
            legacy.publish_from_memory(frontend, failure, allowed_names=frontend_names())
        summary["elapsed_seconds"] = (time.monotonic_ns() - origin_ns) / 1e9
        summary["prepared_input_zip_sha256"] = frontend_hash
        if summary["elapsed_seconds"] > 900:
            summary["collection_complete"] = False
        if not summary["collection_complete"]:
            summary["qualification"] = {name: False for name in QUALIFICATIONS}
        try:
            write_owned(native, native_identity, "vscode-project-research.json", json.dumps(summary, sort_keys=True, indent=2).encode() + b"\n")
        except (ValueError, OSError):
            legacy.quarantine_output(output, output_identity)
            return 2
        admission = legacy.publish_from_memory(native, summary, allowed_names=native_names())
        if admission["complete"] is not True:
            summary["collection_complete"] = False
            if not (admission.get("child_reaped") and admission.get("archive_failure_allowed")):
                legacy.quarantine_output(native, native_identity)
    print(json.dumps({key: summary.get(key) for key in ("source_commit", "source_tree", "collection_complete", "qualification", "elapsed_seconds")}))
    return 0 if summary["collection_complete"] else 2


def contracts():
    """Pure admission checks; no native process, host spoofing or acquisition."""
    if len(native_names()) != 47 or len(frontend_names()) > 64 or set(PUBLIC) & set(GUI):
        raise AssertionError("closed trusted artifact table differs")
    positive = {"evidence_kind": "vscode-pinned-trusted-project-gateway-lifecycle", "mode": MODE,
                "source_commit": "a" * 40, "source_tree": "b" * 40, "version": publisher.VERSION,
                "publisher_commit": publisher.COMMIT, "collection_complete": True, "activation": "prepared",
                "user_public_state": "prepared", "user_native_install_qualified": False,
                "vendor_authentication_qualified": False, "real_model_qualified": False,
                "qualification": {name: True for name in QUALIFICATIONS}, "cleanup": {"complete": True, "error": ""},
                "children": [{"label": name, "tree_reaped": True, "merged_overflow": False, "error": ""}
                             for name in (*PUBLIC, *GUI, "Xvfb")], "completed_stages": list(GUI)}
    controller_result(json.dumps(positive).encode(), "a" * 40, "b" * 40)
    for change in ("wrong-source", "bad-cleanup", "missing-child", "wrong-order", "numeric-flag"):
        value = json.loads(json.dumps(positive))
        if change == "wrong-source": value["source_tree"] = "c" * 40
        elif change == "bad-cleanup": value["children"][0]["tree_reaped"] = False
        elif change == "missing-child": value["children"].pop()
        elif change == "wrong-order": value["completed_stages"].reverse()
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
    if SCANNER_ARCHIVE != publisher.native.current_source_scanner_pin("linux-amd64")[4].removeprefix("sha256:"):
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
