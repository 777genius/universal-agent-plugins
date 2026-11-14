#!/usr/bin/env python3
"""Exact public-CLI source lifecycle evidence; never a signed target grant.

The receipt qualifies only its source/digests, client, scope, platform and tested
release. A consumer must separately check current active policy and denials.
Native activation, project trust, MCP runtime and account access are not proved.
"""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import re
import subprocess
import tarfile
import tempfile
import tomllib
import urllib.request
import zipfile
from datetime import datetime, timezone

ROOT = Path(__file__).resolve().parents[1]
RELEASE_TAG = "agentplugins-v0.1.108"
RELEASE_COMMIT = "8c3cfa28e332c533b0cc0845739c0275730a4285"
REPOSITORY = "777genius/universal-agent-plugins"
LEDGER = "617ebdec67a58af3d265cae49e90c43497299db0"
ORIGIN = f"https://raw.githubusercontent.com/777genius/universal-agent-plugins-registry/{LEDGER}/registry/schemas/1/"
SNAPSHOT_DIGEST = "6ab80575a51378f6643b387a8eefe332e84493bcad86183370c46801243ab277"
SNAPSHOT_SOURCE = "121633366dc5255aaf638845fb7cb0cb50013340"
CLIENTS = ("hermes", "antigravity", "pi")
MAX_NATIVE_ARCHIVE_BYTES = 200 << 20
# Digest-pinned AGY 1.5.3 payloads are 246837648 (Linux), 208900592 (macOS),
# and 225159320 (Windows) bytes. Their compressed archives are all below 200 MiB.
MAX_NATIVE_EXECUTABLE_BYTES = 256 << 20


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / "scripts" / filename)
    loaded = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(loaded)
    return loaded


live = module("source_live_audit", "live-install-audit.py")
native = module("source_native_matrix", "run-native-client-matrix.py")


def require(value, message):
    if not value:
        raise ValueError(message)


def digest(body):
    return "sha256:" + hashlib.sha256(body).hexdigest()


def authenticated_snapshot(directory):
    """Use the production reader through its existing bootstrap verifier."""
    paths = {}
    for kind, suffix, limit in (("snapshot", ".json", 4 << 20), ("envelope", ".envelope.json", 16 << 10)):
        path = directory / (kind + ".json")
        with urllib.request.urlopen(ORIGIN + "snapshots/00000000000000000047" + suffix, timeout=45) as response:
            body = response.read(limit + 1)
        require(len(body) <= limit, "oversize signed input")
        path.write_bytes(body)
        paths[kind] = path
    body = paths["snapshot"].read_bytes()
    require(digest(body) == "sha256:" + SNAPSHOT_DIGEST, "signed source snapshot changed")
    subprocess.run([
        "go", "run", "./cmd/agentplugins/bootstrapgen",
        "-snapshot", str(paths["snapshot"]), "-envelope", str(paths["envelope"]),
        "-trust", str(ROOT / "cli/cmd/agentplugins/directory_bootstrap_inputs/trusted-keys.json"),
        "-expected-key-id", "uap-directory-2026-01",
        "-expected-public-key", "HalXARjat+v3ylTPLMAnvuavRo4ZfrF+DbWwsjlp2bI=",
        "-release-at", datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "-output", str(directory / "verified-bootstrap.go"),
    ], cwd=ROOT / "cli", check=True, timeout=180)
    snapshot = json.loads(body)
    require(snapshot["sequence"] == 47 and snapshot["source_commit"] == SNAPSHOT_SOURCE, "wrong signed provenance")
    return snapshot


def source_case(snapshot, distribution, sequence, client, scope, refusal=False):
    require(client in CLIENTS and scope in ("user", "project"), "unknown qualification tuple")
    rows = [row for row in snapshot["distributions"] if row["id"] == distribution]
    require(len(rows) == 1 and rows[0]["status"] == "active", "source distribution is not active")
    row = rows[0]
    releases = [release for release in row["releases"] if release["sequence"] == sequence]
    policies = [policy for policy in row["release_policies"] if policy["release_sequence"] == sequence]
    require(len(releases) == len(policies) == 1 and policies[0]["status"] == "active", "source release is not active")
    require(not any(item["distribution_id"] == distribution and item["release_sequence"] == sequence
                    for item in snapshot["revocations"]), "source release is revoked")
    # An exact Git alternative must never reinterpret a signed target or denial.
    require(not any(target["client"] == client for policy in row["release_policies"] for target in policy["targets"]), "signed target needs its normal policy route")
    require(not any(item.get("client") == client and item.get("distribution_id") == distribution
                    for item in snapshot["evidence"]), "existing target evidence needs policy review")
    release = releases[0]
    require(release["components"] and set(release["components"]) <= {"skills", "mcp"}, "nonportable mandatory component")
    require(not any(target.get("app_binding") for target in policies[0]["targets"]), "app-bound release cannot use source qualification")
    source = release["package_source"]
    require(re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", source["repository"]), "invalid repository")
    require(re.fullmatch(r"[0-9a-f]{40}", source["revision"]), "mutable source")
    require(source["path"] and all(re.fullmatch(r"[A-Za-z0-9_.-]+", part) and part not in (".", "..")
                                  for part in source["path"].split("/")), "unsafe source subdirectory")
    for key in ("tree_digest", "manifest_digest"):
        require(re.fullmatch(r"sha256:[0-9a-f]{64}", release[key]), "missing signed digest")
    return {"distribution": distribution, "release_sequence": sequence,
            "name": release["manifest_name"], "source": source,
            "tree_digest": release["tree_digest"], "manifest_digest": release["manifest_digest"],
            "components": release["components"], "client": client, "scope": scope,
            "selector": f'github:{source["repository"]}@{source["revision"]}//{source["path"]}',
            "expected": "dependency-refusal" if refusal else "materialized-lifecycle"}


def state_file(root, scope):
    path = root / ("uap-data" if scope == "user" else "project/.uap") / "state-v2.json"
    require(live.path_inside_case(path, root), "managed state escaped case")
    return json.loads(path.read_text(encoding="utf-8")) if path.exists() else {"installations": []}


def protected_scope_snapshot(root, scope, native_root):
    """Observe the other native profile and its saved, mutable environment."""
    documents = root / ("project" if scope == "user" else "uap-data")
    state = documents / ".uap" if scope == "user" else documents
    files = [documents / "uap.toml", documents / "uap.lock"]
    files += [state / name for name in ("state-v2.json", "mutation.lock", "environment.lock")]
    directories = [native_root, *[state / name for name in
                   ("managed", "operations-v2", "plugin-data", "installer-tmp", "clients")]]
    for path in [*files, *directories]:
        in_case(path, root)
    # The content store, acquisition and signed-feed/security caches are shared.
    # They are deliberately absent from these scope-owned paths.
    observed = {"directories": live.scoped_snapshot(directories), "files": {}}
    for path in files:
        observed["files"][path.relative_to(root).as_posix()] = protected_saved_record(path)
    return observed


def protected_saved_record(path):
    """Read one saved record through a held, bounded, no-follow descriptor."""
    try:
        before = path.lstat()
    except FileNotFoundError:
        return None
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and 0 <= before.st_size <= 1 << 20,
            "protected saved record must be a bounded single-link regular file")
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_NONBLOCK", 0)
    fd = os.open(path, flags)
    try:
        held = os.fstat(fd)
        def same(left, right):
            return (stat.S_ISREG(right.st_mode) and right.st_nlink == 1
                    and os.path.samestat(left, right) and right.st_mode == left.st_mode
                    and right.st_size == left.st_size and right.st_mtime_ns == left.st_mtime_ns
                    and right.st_ctime_ns == left.st_ctime_ns)
        require(same(before, held), "protected saved record changed before read")
        body = bytearray()
        while len(body) <= before.st_size:
            chunk = os.read(fd, min(65536, before.st_size + 1 - len(body)))
            if not chunk:
                break
            body.extend(chunk)
        require(len(body) == before.st_size
                and same(before, os.fstat(fd)) and same(before, path.lstat()),
                "protected saved record changed during read")
        return {"digest": digest(body), "mtime_ns": before.st_mtime_ns,
                "inode": before.st_ino, "mode": before.st_mode}
    finally:
        os.close(fd)


def checked_saved_removal(documents, alias, case, root):
    for path in documents:
        in_case(path, root)
        require(path.is_file(), "saved removal lost the manifest or lock")
    manifest = tomllib.loads(documents[0].read_text(encoding="utf-8"))
    locked = json.loads(documents[1].read_text(encoding="utf-8"))
    dependencies, targets = manifest.get("dependencies", {}), manifest.get("targets", {})
    require(manifest.get("schema_version") == 1 and isinstance(dependencies, dict)
            and isinstance(targets, dict), "saved removal produced an invalid manifest")
    require(alias not in dependencies, "remove retained a saved dependency")
    require(all(isinstance(aliases, list) and alias not in aliases for aliases in targets.values()),
            "remove retained a saved target assignment")
    packages = locked.get("packages")
    require(locked.get("lock_version") == 1 and isinstance(packages, list), "saved removal produced an invalid lock")
    source_key = f'github:{case["source"]["repository"].lower()}:{case["source"]["path"]}'
    require(all(isinstance(package, dict) and package.get("alias") != alias
                and package.get("source_key") != source_key for package in packages),
            "remove retained a locked package")


def checked_binding(state, case):
    installations = state.get("installations", [])
    require(len(installations) == 1, "expected one saved installation")
    installation = installations[0]
    require(installation["declared_name"] == case["name"] and installation.get("origin_mode") == "direct", "wrong direct source identity")
    source = installation["source"]
    require(source.get("resolved_revision") == case["source"]["revision"], "wrong immutable source revision")
    require(source.get("repository", "").removesuffix(".git").removeprefix("https://github.com/").casefold() == case["source"]["repository"].casefold(), "wrong source repository")
    require(source.get("package_subpath") == case["source"]["path"], "wrong source subdirectory")
    bindings = list(installation.get("clients", {}).values())
    require(len(bindings) == 1, "unexpected sibling target")
    binding = bindings[0]
    require(binding["client_id"] == case["client"] and binding["scope"] == case["scope"], "wrong saved placement")
    require(binding["materialization"] == "materialized", "native package not materialized")
    revision = binding.get("package_revision", {})
    for field in ("tree_digest", "manifest_digest"):
        require(revision.get(field) == case[field], "materialized package differs from signed source digest")
    require(revision.get("resolved_revision") == case["source"]["revision"], "binding revision changed")
    receipts = state.get("environment_receipts", [])
    owned = [receipt for receipt in receipts if receipt["installation_id"] == installation["installation_id"]
             and receipt["client_id"] == case["client"] and receipt["client_binding_id"] == binding["client_binding_id"]]
    require(len(owned) == 1 and owned[0].get("phase") == "committed", "saved environment ownership is missing")
    owner = owned[0]
    require(owner.get("source_key") == f'github:{case["source"]["repository"].lower()}:{case["source"]["path"]}'
            and owner.get("package_digest") == case["tree_digest"], "saved owner belongs to another source")
    require(owner.get("environment_id") == case["scope"] + "-default"
            and owner.get("alias") and owner.get("receipt_id"), "saved owner has wrong environment or incomplete identity")
    return installation, binding, owned[0]


def read_native_executable(stream, size, magic):
    require(0 < size <= MAX_NATIVE_EXECUTABLE_BYTES, "native executable exceeds payload size bound")
    body = stream.read(MAX_NATIVE_EXECUTABLE_BYTES + 1)
    require(len(body) == size, "native executable size differs from archive metadata")
    require(any(body.startswith(prefix) for prefix in magic), "native executable has invalid native magic")
    return body


def extract_antigravity_executable(body, archive):
    # Select exactly one regular executable. Archive paths never become
    # filesystem paths, and symlinks are never extracted or executed.
    if archive.endswith(".zip"):
        with zipfile.ZipFile(io.BytesIO(body)) as package:
            candidates = [item for item in package.infolist() if not item.is_dir()
                          and item.filename.lower().endswith(".exe")
                          and (item.external_attr >> 16) & 0o170000 in (0, 0o100000)]
            require(len(candidates) == 1, "native archive does not contain exactly one executable")
            item = candidates[0]
            with package.open(item) as stream:
                return read_native_executable(stream, item.file_size, (b"MZ",))
    magic = (b"\x7fELF", b"\xcf\xfa\xed\xfe", b"\xfe\xed\xfa\xcf", b"\xca\xfe\xba\xbe")
    with tarfile.open(fileobj=io.BytesIO(body), mode="r:gz") as package:
        candidates = []
        for item in package.getmembers():
            if item.isfile():
                with package.extractfile(item) as stream:
                    if stream.read(4) in magic:
                        candidates.append(item)
        require(len(candidates) == 1, "native archive does not contain exactly one executable")
        item = candidates[0]
        with package.extractfile(item) as stream:
            return read_native_executable(stream, item.size, magic)


def provision_native(client, target, directory, selected):
    if client == "hermes":
        require(selected and selected.is_file(), "real Hermes bootstrap executable is required")
        return selected.absolute(), {"commit": "2bd378ed1b8d6bf2192cb26e0a55ff3091e6bd19", "acquisition": "workflow pinned official bootstrap", "binary_sha256": hashlib.sha256(selected.read_bytes()).hexdigest()}
    if client == "pi":
        return module("source_pi", "pi_project_native_e2e.py").checked_release(directory, target, dict(os.environ))
    proof = module("source_released_native", "e2e/released-native-proof.py")
    archive = {"linux-amd64": "agy_cli_linux_x64.tar.gz", "darwin-arm64": "agy_cli_mac_arm64.tar.gz", "windows-amd64": "agy_cli_windows_x64.zip"}[target]
    url = "https://github.com/google-antigravity/antigravity-cli/releases/download/1.5.3/" + archive
    with urllib.request.urlopen(url, timeout=90) as response:
        body = response.read(MAX_NATIVE_ARCHIVE_BYTES + 1)
    require(len(body) <= MAX_NATIVE_ARCHIVE_BYTES, "native archive exceeds bound")
    native.verify_digest(body, "sha256:" + proof.AGY_PINS[target])
    executable = extract_antigravity_executable(body, archive)
    path = directory / ("agy.exe" if target == "windows-amd64" else "agy")
    path.write_bytes(executable)
    path.chmod(0o700)
    return path, {"tag": "1.5.3", "archive": archive, "sha256": proof.AGY_PINS[target], "binary_sha256": hashlib.sha256(executable).hexdigest()}


def in_case(path, root):
    require(live.path_inside_case(path, root), "native path escaped disposable case")
    return path


def checked_dependency_refusal(output, case):
    require(type(output.get("exit_code")) is int and output["exit_code"] != 0
            and not output.get("timeout"), "mandatory closure was not refused")
    require(isinstance(output.get("stderr_full"), str) and isinstance(output.get("stdout_full"), str),
            "bounded complete refusal output is required")
    require(case["distribution"] == "777genius/marketing-kit" and case["name"] == "marketing-kit",
            "no reviewed dependency refusal for this source")
    # Reuse the existing immutable Marketing Kit closure. A root failure,
    # transport failure, or a different dependency/version is not this proof.
    diagnostic = output["stderr_full"].strip()
    for distribution, pin in live.MARKETING_KIT_SIGNED_CLOSURE.items():
        if distribution == case["distribution"]:
            continue
        expected = (f"agentplugins: dependency conflict for directory:{distribution}: "
                    f'marketing-kit version {pin["package_version"]}')
        if diagnostic == expected and not output["stdout_full"].strip():
            return {"code": "dependency_conflict", "distribution": distribution,
                    "version": pin["package_version"], "message": diagnostic}
    raise ValueError("refusal did not identify the exact mandatory dependency conflict")


def checked_native_cleanup(selected_root, case, binding, root):
    in_case(selected_root, root)
    names = []
    for item in binding.get("native_objects", []):
        if item.get("path"):
            in_case(Path(item["path"]), root)
        if item.get("logical_name"):
            names.append(item["logical_name"])
    # Observe all remaining files, including config created by add/remove;
    # looking only at the original paths can miss a newly retained MCP entry.
    residuals = live.native_identity_residuals(
        [selected_root], case["name"], live.snapshot([selected_root]), case["client"],
        component_names=names)
    require(not residuals, "owned native entries remain after remove: " + ", ".join(residuals))
    return {"owned_logical_names": sorted(set(names)), "remaining": residuals}


def run_case(binary, case, root, executable):
    root.mkdir()
    home, project, data = (root / name for name in ("home", "project", "uap-data"))
    for path in (home, project, data):
        path.mkdir()
    env = live.isolated_client_env(home, root, data)
    env["HERMES_HOME"] = str(home / ".hermes")
    if executable:
        env["PATH"] = str(executable.parent) + os.pathsep + env.get("PATH", "")
    native_root = home / {"hermes": ".hermes", "antigravity": ".gemini/config", "pi": ".pi/agent"}[case["client"]]
    native_root.mkdir(parents=True)
    project_native = project / {"hermes": ".hermes", "antigravity": ".agents", "pi": ".pi"}[case["client"]]
    project_native.mkdir()
    sentinels = [home / "foreign.txt", project / "foreign.txt", native_root / "foreign.txt", project_native / "foreign.txt"]
    for path in sentinels:
        path.write_text("unmanaged source audit neighbor\n", encoding="utf-8")
    live.initialize_project_skill_git_root(project, env, root)
    scope = ["--scope", case["scope"]]
    if case["scope"] == "project":
        scope += ["--project-root", str(project)]
    trace = []
    saved_root = data if case["scope"] == "user" else project
    documents = [saved_root / "uap.toml", saved_root / "uap.lock"]

    def run(operation, selector=None, success=True):
        argv = [str(binary), operation, *([selector] if selector else []), "--target", case["client"], *scope, "--format", "json"]
        if operation in ("add", "update", "repair", "remove"):
            argv += ["--yes"]
        for path in (root, home, project, data, native_root, project_native):
            in_case(path, root)
        result = live.call(argv, env, project, 180, capture_files=True, capture_full=not success)
        parsed = result.pop("_json", None)
        trace.append({"operation": operation, "argv": argv, **result})
        for path in sentinels:
            require(live.preserved_regular_text(path, "unmanaged source audit neighbor\n", root), "foreign state changed")
        require(protected_scope_snapshot(root, case["scope"], protected_root) == protected,
                operation + " changed the other scope")
        if success:
            require(result["exit_code"] == 0 and not result.get("timeout") and isinstance(parsed, dict), operation + " did not complete")
        return parsed, result

    selected_root = native_root if case["scope"] == "user" else project_native
    protected_root = project_native if case["scope"] == "user" else native_root
    protected = protected_scope_snapshot(root, case["scope"], protected_root)
    before = live.snapshot([selected_root])
    try:
        _, added = run("add", case["selector"], case["expected"] != "dependency-refusal")
        if case["expected"] == "dependency-refusal":
            refusal = checked_dependency_refusal(added, case)
            require(not state_file(root, case["scope"]).get("installations"), "refused closure left managed installation")
            require(not any(path.exists() for path in documents), "refused closure published saved state")
            require(live.snapshot([selected_root]) == before, "refused closure mutated native state")
            outcome = {"status": "expected-dependency-refusal", "refusal": refusal}
        else:
            installation, binding, receipt = checked_binding(state_file(root, case["scope"]), case)
            locator = in_case(Path(binding["target_locator"]), root)
            require(locator.is_dir() and bool(live.snapshot([locator])), "materialized artifact is missing")
            native_after = live.snapshot([selected_root])
            require(native_after != before, "native materialization was not independently observed")
            require(all(path.is_file() for path in documents), "saved manifest/lock are absent")
            run("info", installation["installation_id"])
            run("list")
            # Exercise the actual updater; repeating add is not update evidence.
            run("update", receipt["alias"])
            updated, updated_binding, _ = checked_binding(state_file(root, case["scope"]), case)
            require(updated["installation_id"] == installation["installation_id"], "update changed saved identity")
            require(live.snapshot([selected_root]) == native_after, "immutable no-change update changed native files")
            run("repair", receipt["alias"])
            repaired, repaired_binding, _ = checked_binding(state_file(root, case["scope"]), case)
            require(repaired["installation_id"] == installation["installation_id"], "repair changed saved identity")
            require(live.snapshot([selected_root]) == native_after, "healthy repair changed native files")
            run("remove", receipt["alias"])
            listed, _ = run("list")
            require(listed.get("data", {}).get("installations") == [], "removed package is still listed")
            checked_saved_removal(documents, receipt["alias"], case, root)
            require(not any(item.get("clients") for item in state_file(root, case["scope"]).get("installations", [])), "remove retained a native binding")
            require(not locator.exists(), "remove retained the managed package")
            cleanup = checked_native_cleanup(selected_root, case, binding, root)
            outcome = {"status": "materialized-lifecycle-passed", "installation_id": installation["installation_id"],
                       "activation_observed": [binding["activation"], updated_binding["activation"], repaired_binding["activation"]],
                       "native_files_after_add": len(native_after), "native_cleanup": cleanup,
                       "update": "immutable-no-change", "repair": "healthy-repair"}
        require(protected_scope_snapshot(root, case["scope"], protected_root) == protected, "other scope changed")
        return {**case, **outcome, "trace": trace, "activation_qualified": False, "runtime_qualified": False}
    except Exception as error:
        return {**case, "status": "failed", "reason": str(error), "trace": trace,
                "activation_qualified": False, "runtime_qualified": False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--client", choices=CLIENTS, required=True)
    parser.add_argument("--target", choices=("linux-amd64", "darwin-arm64", "windows-amd64"), required=True)
    parser.add_argument("--native-bin", type=Path)
    parser.add_argument("--report-dir", type=Path, required=True)
    args = parser.parse_args()
    native.require_hosted(args.target)
    if args.client == "hermes":
        require(args.native_bin and args.native_bin.is_file(), "real native client executable required for detection")
    args.report_dir.mkdir(parents=True, exist_ok=False)
    report = {"schema_version": 1, "kind": "released-exact-source-lifecycle",
              "generated_at": datetime.now(timezone.utc).isoformat(),
              "workflow_sha": os.environ.get("AUDIT_WORKFLOW_SHA"), "harness_sha": os.environ.get("EXPECTED_HEAD"), "run_id": os.environ.get("GITHUB_RUN_ID"),
              "run_attempt": os.environ.get("GITHUB_RUN_ATTEMPT"), "platform": args.target,
              "source_snapshot": {"ledger_commit": LEDGER, "sequence": 47, "digest": "sha256:" + SNAPSHOT_DIGEST},
              "claims": {"signed_grants": False, "native_activation": False, "mcp_runtime": False, "oauth": False},
              "status": "failed", "cases": []}
    try:
        with tempfile.TemporaryDirectory(prefix="released-source-", dir=os.environ["RUNNER_TEMP"]) as temp:
            directory = Path(temp).resolve()
            snapshot = authenticated_snapshot(directory)
            binary, report["installer"] = native.provision_release(ROOT, directory, args.target, RELEASE_TAG, RELEASE_COMMIT, REPOSITORY)
            executable, report["native_client"] = provision_native(args.client, args.target, directory, args.native_bin)
            scopes = ("user",) if args.client == "hermes" else ("user", "project")
            for scope in scopes:
                for distribution, refusal in (("777genius/notion", False), ("777genius/marketing-kit", True)):
                    case = source_case(snapshot, distribution, 1, args.client, scope, refusal)
                    report["cases"].append(run_case(binary, case, directory / f'{case["name"]}-{scope}', executable))
            require(all(case["status"] != "failed" for case in report["cases"]), "some exact-source lifecycle cases failed")
            report["status"] = "passed"
    except Exception as error:
        report["reason"] = str(error)
    finally:
        (args.report_dir / "receipt.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "cases": [{"client": row["client"], "scope": row["scope"], "name": row["name"], "status": row["status"]} for row in report["cases"]]}))
    return 0 if report["status"] == "passed" else 1


if __name__ == "__main__":
    raise SystemExit(main())
