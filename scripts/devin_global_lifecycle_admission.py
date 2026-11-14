"""Real public UAP Global lifecycle after the pinned ordinary Devin gate.

Native JSON observation is separate from production's file-only verifier.
This research consumer never attests authentication, model or MCP execution.
"""
import argparse
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import stat

import devin_ordinary_metadata_admission as ordinary
import devin_global_toolchain as private_go

base = ordinary.base
NAME, SKILL = "uap-devin-global", "uap-global-marker"
SERVERS = ("uap_global_stdio", "uap_global_http")
NS = "io.github.777genius.agentplugins/managed-stdio-v1"
SCANNER = (9989584, "b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba")
POLICY = {"id": "agent-plugin-install", "version": 2,
          "digest": "sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"}
PUBLIC = ("uap-add", "uap-doctor-installed", "uap-repair", "uap-doctor-repaired", "uap-remove")
NATIVE = tuple(kind + "-" + phase + "-" + scope for phase in ("installed", "repaired", "removed")
               for scope in ("a", "b") for kind in ("mcp", "skills"))


def sha(body):
    return hashlib.sha256(body).hexdigest()



def auxiliary(name):
    return (name in base.BOOTSTRAP_DIRS or name in base.BOOTSTRAP_FIXED
            or name in (base.INSTALLATION_ID, base.MAINTENANCE) or base.BOOTSTRAP_LOG.fullmatch(name))


def source_digest(bodies):
    """The existing packagedigest v1 framing, applied only to four authored files."""
    digest = hashlib.sha256()
    def framed(value):
        value = value.encode()
        digest.update(len(value).to_bytes(8, "big")); digest.update(value)
    framed("agentplugins.package-tree\0sha256\0v1")
    directories = {parent.as_posix() for name in bodies for parent in Path(name).parents if str(parent) != "."}
    for name in sorted(set(bodies) | directories):
        directory = name in directories
        for value in ("entry", name, "directory" if directory else "file", "040000" if directory else "100644", ""):
            framed(value)
        body = b"" if directory else bodies[name]
        digest.update(len(body).to_bytes(8, "big")); digest.update(body)
    return "sha256:" + digest.hexdigest()


def entry_digest(name, entry):
    canonical = json.dumps(entry, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    for literal, escaped in (("<", "\\u003c"), (">", "\\u003e"), ("&", "\\u0026"),
                             ("\u2028", "\\u2028"), ("\u2029", "\\u2029")):
        canonical = canonical.replace(literal, escaped)
    return "sha256:" + sha(("agentplugins-native-mcp-v1\0devin-mcpServers\0" + name + "\0" + canonical).encode())


def owned_skill_row(ctx, profile, scope):
    return dict(name=str(ctx.projects[scope]) + ":" + SKILL, description="Owned Global source.",
                triggers=["user", "model"], provider="Devin", base_dir=str(profile / "skills" / SKILL),
                display_name=SKILL, warnings=[], errors=[])


def doctor_result(data, identifier, binding):
    base.require(type(data) is dict and data.get("read_only") is True
                 and type(data.get("installation_count")) is int and data["installation_count"] == 1
                 and type(data.get("open_operation_count")) is int and data["open_operation_count"] == 0,
                 "Global doctor readonly/counts")
    item = data.get("installation")
    base.require(type(item) is dict, "Global doctor installation shape")
    clients, findings = item.get("clients"), data.get("findings")
    base.require(item.get("installation_id") == identifier and item.get("name") == NAME
                 and item.get("version") == "1.0.0" and item.get("source") == "local"
                 and item.get("mixed_version") is False and item.get("needs_rebind", False) is False
                 and data.get("project_readiness") == []
                 and type(clients) is list and len(clients) == 1 and type(clients[0]) is dict
                 and clients[0].get("package_revision") == binding["package_revision"]
                 and all(clients[0].get(key) == value for key, value in {
                     "client_id": "windsurf", "scope": "user", "activation": "prepared",
                     "materialization": "materialized", "authentication": "not_checked",
                     "policy": "allowed", "verification": "installation_verified"}.items()),
                 "Global doctor selected lifecycle")
    # Ordinary Detect never executes a version probe. Its VerifyOnly therefore
    # refuses the exact version gate as indeterminate, without claiming drift.
    wanted = {("degraded", "activation_pending"), ("unknown", "authentication_not_checked"),
              ("unknown", "native_projection_not_checked")}
    base.require(type(findings) is list and len(findings) == len(wanted)
                 and all(type(row) is dict and row.get("installation_id") == identifier
                         and row.get("installation_name") == NAME and row.get("client_id") == "windsurf"
                         for row in findings)
                 and {(row.get("status"), row.get("code")) for row in findings} == wanted,
                 "Global doctor truthful findings")


def receipt_join(row, binding, group, sequence, before, *, removed=False):
    base.require(type(group) is str and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", group)
                 and type(row) is dict and row.get("operation_group_id") == group
                 and row.get("operation_id") == group + "-001"
                 and type(row.get("sequence")) is int and row["sequence"] == sequence
                 and row.get("mutation_type") == ("directory_remove" if removed else "directory_swap")
                 and row.get("phase") == "committed"
                 and row.get("client_binding_id") == binding["client_binding_id"]
                 and row.get("active_path") == binding["target_locator"]
                 and row.get("before_digest", "") == before
                 and (row.get("after_digest", "") == "" if removed else
                      type(row.get("after_digest")) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", row["after_digest"])),
                 "Global operation receipt join")


def public_result(data, verb, item, binding):
    """Join the actual public group/outcome to the independently read state."""
    group = item.get("operation_group_id")
    targets = data.get("targets") if type(data) is dict else None
    base.require(type(targets) is list and len(targets) == 1 and type(targets[0]) is dict
                 and data.get("batch") is True and type(data.get("succeeded")) is int and data["succeeded"] == 1
                 and type(data.get("failed")) is int and data["failed"] == 0 and data.get("operation_id") == group
                 and data.get("plugin") == NAME and data.get("dry_run") is False
                 and data.get("status") == ("data_retained" if verb == "remove" else "completed")
                 and targets[0].get("target") == "windsurf" and targets[0].get("status") == "external_completed"
                 and not targets[0].get("error"), "Global public batch outcome")
    output = targets[0].get("output")
    base.require(type(output) is dict and output.get("operation_id") == group
                 and output.get("plugin") == NAME and output.get("version") == "1.0.0"
                 and output.get("source") == ("direct local source" if verb == "add" else "local")
                 and output.get("tree_digest") == item["source"]["tree_digest"]
                 and output.get("manifest_digest") == item["package"]["manifest_digest"]
                 and output.get("dry_run") is False, "Global public source/revision")
    outcome = output.get("result")
    common = {"installation_id", "requires_confirmation", "mutated", "group_phase"}
    base.require(type(outcome) is dict and outcome.get("installation_id") == item["installation_id"]
                 and outcome.get("requires_confirmation") is False and outcome.get("mutated") is True
                 and outcome.get("group_phase") == "external_completed", "Global public selected mutation")
    if verb == "remove":
        receipt = item["data_receipts"][binding["data_receipt_id"]]
        base.require(set(outcome) == common | {"plugin", "client_id", "affected_surfaces", "deactivation"}
                     and outcome.get("plugin") == NAME and outcome.get("client_id") == "windsurf"
                     and outcome.get("affected_surfaces") == ["windsurf"]
                     and outcome.get("deactivation") == {"activation": "not_required", "artifact_removal_allowed": True,
                                                          "external_removal_complete": True}
                     and all(outcome["deactivation"].get(key) is True for key in ("artifact_removal_allowed", "external_removal_complete"))
                     and data.get("status") == "data_retained" and data.get("data_retained") is True
                     and data.get("plugin_data_preserved") is True and item.get("data_retained") is True,
                     "Global public retained removal")
        base.require(data.get("retained_data") == [{key: receipt[key] for key in
                     ("data_receipt_id", "physical_backend_id", "scope", "state")}], "Global public retained data receipt")
        return
    if verb == "add":
        base.require(all(data.get(key) == output[key] for key in ("version", "source", "tree_digest", "manifest_digest"))
                     and data.get("revision", "") == "" and output.get("revision", "") == "", "Global public add source join")
    base.require(verb in ("add", "repair") and set(outcome) == common | {"plan", "activation"},
                 "Global public install outcome shape")
    plan, activation = outcome.get("plan"), outcome.get("activation")
    base.require(type(plan) is dict and type(activation) is dict
                 and plan.get("client_id") == "windsurf" and plan.get("scope") == "user"
                 and plan.get("physical_artifact_id") == binding["physical_artifact_id"]
                 and plan.get("activation") == "prepared" and plan.get("verification") == "package_validated"
                 and activation.get("activation") == "prepared" and activation.get("authentication") == "not_checked"
                 and activation.get("policy") == "allowed" and activation.get("verification") == "installation_verified",
                 "Global public Prepared lifecycle")
    components = [{"kind": "skill", "name": SKILL, "support": "prepared"}]
    components.extend({"kind": "mcp_server", "name": name, "support": "prepared"} for name in SERVERS)
    observed = plan.get("components")
    base.require(type(observed) is list and len(observed) == 3 and all(type(row) is dict for row in observed)
                 and sorted(observed, key=lambda row: (row.get("kind", ""), row.get("name", "")))
                 == sorted(components, key=lambda row: (row["kind"], row["name"])), "Global public exact components")


def public_command_join(record, label, arguments, cwd, stdout, stderr):
    base.require(record.get("probe") == label and record.get("argv") == arguments
                 and record.get("cwd") == str(cwd), "Global public command identity")
    for kind, body in (("stdout", stdout), ("stderr", stderr)):
        base.require(record.get(kind + "_file") == label + "." + kind + ".log"
                     and type(record.get(kind + "_bytes")) is int and record[kind + "_bytes"] == len(body)
                     and record.get(kind + "_sha256") == sha(body), "Global public original stream join")


def authority_failure(value):
    checks = (("regular", value["regular"]), ("single-link", value["nlink"] == 1),
              ("owner", value["owner_matches"]), ("mode", value["mode"] == 0o755),
              ("size", 0 < value["bytes"] <= 128 << 20))
    return next((name for name, accepted in checks if not accepted), None)


class ExecutableAuthorityRefusal(ValueError):
    def __init__(self, info, owner):
        super().__init__("prebuilt installer authority")
        self.metadata = dict(regular=stat.S_ISREG(info.st_mode), nlink=info.st_nlink,
                             owner_matches=(info.st_uid, info.st_gid) == owner,
                             mode=stat.S_IMODE(info.st_mode), bytes=info.st_size)
        self.metadata["predicate"] = authority_failure(self.metadata)


def validate_authority_refusal(value):
    base.require(type(value) is dict and set(value) == {"role", "predicate", "regular", "nlink", "owner_matches", "mode", "bytes"}
                 and type(value["role"]) is str and value["role"] in ("go", "installer")
                 and type(value["predicate"]) is str and value["predicate"] in ("regular", "single-link", "owner", "mode", "size")
                 and all(type(value[key]) is bool for key in ("regular", "owner_matches"))
                 and type(value["mode"]) is int and 0 <= value["mode"] <= 0o7777
                 and all(type(value[key]) is int and 0 <= value[key] < 1 << 63 for key in ("nlink", "bytes"))
                 and value["predicate"] == authority_failure(value), "Global build authority diagnostic")
    return dict(value)


def build_executable_pin(ctx, path, role, *, owner=None, metadata=None):
    base.require(role in ("go", "installer"), "Global build executable role")
    state = getattr(ctx, "global_readout_state", None)
    if state is not None: state["phase"] = "build-" + role + "-authority"
    try:
        return installer_pin(path, ctx.budget, owner=owner, metadata=metadata)
    except ExecutableAuthorityRefusal as error:
        ctx.summary["global_build_authority_refusal"] = validate_authority_refusal(dict(role=role, **error.metadata))
        raise


def closed_build_authority_refusal(summary, state):
    present = "global_build_authority_refusal" in summary
    missing = state["phase"] in ("build-go-authority", "build-installer-authority") and state["reason"] == "executable-authority-refused"
    base.require(present or not missing, "Global build authority diagnostic missing")
    if not present: return None
    value = validate_authority_refusal(summary["global_build_authority_refusal"])
    base.require(summary.get("collection_complete") is False and summary.get("metadata_capture_complete") is False
                 and all(summary.get(key, False) is False for key in ("ordinary_metadata_capture_complete", "global_lifecycle_capture_complete"))
                 and summary.get("admission_qualified") is False
                 and state["phase"] == "build-" + value["role"] + "-authority"
                 and state["reason"] == "executable-authority-refused", "Global build authority diagnostic state")
    return value


def build_installer(ctx, installer):
    """Two unchanged bounded commands with one complete private pinned GOROOT."""
    parent = installer.parent
    base.require(installer.is_absolute() and parent.resolve(strict=True) == parent
                 and not installer.exists() and not installer.is_symlink(), "fresh installer output required")
    parent_identity = base.root_identity(parent)
    with private_go.prepared(ctx) as compiler:
        pin = build_with_compiler(ctx, installer, parent_identity, compiler)
    ctx.identities[installer] = pin[1]
    validate_build_receipt(ctx.summary["global_build_receipt"], ctx.summary)
    return pin


def build_with_compiler(ctx, installer, parent_identity, compiler):
    go, parent = compiler.go, installer.parent
    go_info = go.lstat()
    go_owner = (os.getuid(), os.getgid())
    authority = {"schema_version": 1, "role": "verified-private-go", "selected_lstat": installer_metadata(go_info),
                 "expected_owner": {"uid": go_owner[0], "gid": go_owner[1]}, "initial_pin_lstat": {}}
    ctx.summary["global_build_authority"] = authority
    go_pin = build_executable_pin(ctx, go, "go", owner=go_owner, metadata=authority["initial_pin_lstat"])
    receipt = {"schema_version": 1, "source_commit": ctx.summary["source_commit"],
               "source_tree": ctx.summary["source_tree"], "environment": private_go.ENVIRONMENT,
               "go": {"path": str(go), "bytes": go_pin[0], "sha256": go_pin[1], "uid": go_owner[0], "gid": go_owner[1], "mode": 0o755},
               "installer": None, "commands": [], "streams": {}, "toolchain": compiler.receipt}
    ctx.summary["global_build_receipt"] = receipt
    environment = dict(os.environ, GOTOOLCHAIN="local", GOENV="off", GOROOT=str(compiler.path / "go"),
                       PATH=str(go.parent) + ":/usr/bin:/bin")
    for label, arguments, timeout in (("build-go-version", ["version"], 10),
                                     ("build-uap", ["build", "-o", str(installer), "./cli/cmd/agentplugins"], 240)):
        compiler.verify()
        base.require(base.root_identity(parent) == parent_identity
                     and build_executable_pin(ctx, go, "go", owner=go_owner) == go_pin, "build authority changed")
        if label == "build-uap":
            base.require(not installer.exists() and not installer.is_symlink(), "fresh installer output required")
        state = getattr(ctx, "global_readout_state", None)
        if state is not None: state["phase"] = label
        ctx.process_cleanup_proved = False
        record, stdout, stderr = base.shared.run_probe([str(go), *arguments], base.shared.SOURCE, environment,
            ctx.private, label, timeout=min(timeout, ctx.budget.check()), limit=1 << 20, memory_only=True)
        ctx.process_cleanup_proved = True
        receipt["commands"].append(record)
        for kind, body in (("stdout", stdout), ("stderr", stderr)):
            receipt["streams"][label + "." + kind + ".log"] = base64.b64encode(body).decode("ascii")
        base.require(type(record.get("exit")) is int and record["exit"] == 0 and record.get("error") is None,
                     "Global bounded build command failed")
        if label == "build-go-version":
            base.require(stdout == b"go version go1.26.11 linux/amd64\n" and not stderr, "Global exact Go version")
        compiler.verify()
    base.require(base.root_identity(parent) == parent_identity and build_executable_pin(ctx, go, "go", owner=go_owner) == go_pin,
                 "build authority changed")
    pin = build_executable_pin(ctx, installer, "installer")
    receipt["installer"] = {"path": str(installer), "bytes": pin[0], "sha256": pin[1],
                            "uid": os.getuid(), "gid": os.getgid(), "mode": 0o755}
    return pin


def validate_build_receipt(value, summary):
    base.require(type(value) is dict and set(value) == {"schema_version", "source_commit", "source_tree", "environment",
                 "go", "installer", "commands", "streams", "toolchain"} and type(value["schema_version"]) is int
                 and value["schema_version"] == 1 and value["environment"] == private_go.ENVIRONMENT
                 and all(type(value[key]) is str and re.fullmatch("[0-9a-f]{40}", value[key])
                         and value[key] == summary.get(key) for key in ("source_commit", "source_tree")),
                 "Global build source witness")
    for role in ("go", "installer"):
        row = value[role]
        base.require(type(row) is dict and set(row) == {"path", "bytes", "sha256", "uid", "gid", "mode"}
                     and type(row["path"]) is str and Path(row["path"]).is_absolute()
                     and type(row["bytes"]) is int and 0 < row["bytes"] <= 128 << 20
                     and all(type(row[key]) is int and 0 <= row[key] < 1 << 32 for key in ("uid", "gid"))
                     and type(row["mode"]) is int and row["mode"] == 0o755
                     and type(row["sha256"]) is str and re.fullmatch("[0-9a-f]{64}", row["sha256"]),
                     "Global build executable witness")
    base.require((value["installer"]["uid"], value["installer"]["gid"]) == (os.getuid(), os.getgid()),
                 "Global build installer owner")
    private_go.validate_receipt(value["toolchain"], value["go"])
    labels = ("build-go-version", "build-uap")
    records, streams = value["commands"], value["streams"]
    base.require(type(records) is list and len(records) == 2 and type(streams) is dict
                 and set(streams) == {label + "." + kind + ".log" for label in labels for kind in ("stdout", "stderr")},
                 "Global build finite captures")
    fields = {"probe", "argv", "cwd", "error", "exit", "elapsed_seconds", "stdout_file", "stderr_file",
              "stdout_bytes", "stderr_bytes", "stdout_sha256", "stderr_sha256"}
    for row, label in zip(records, labels):
        arguments = ["version"] if label == labels[0] else ["build", "-o", value["installer"]["path"], "./cli/cmd/agentplugins"]
        base.require(type(row) is dict and set(row) == fields and row.get("probe") == label
                     and type(row.get("exit")) is int and row["exit"] == 0 and row.get("error") is None
                     and row.get("argv") == [value["go"]["path"], *arguments] and row.get("cwd") == str(base.shared.SOURCE)
                     and type(row["elapsed_seconds"]) in (int, float) and base.math.isfinite(row["elapsed_seconds"])
                     and 0 <= row["elapsed_seconds"] <= (20 if label == labels[0] else 250),
                     "Global build command witness")
        for kind in ("stdout", "stderr"):
            name = label + "." + kind + ".log"
            encoded = streams[name]
            base.require(type(encoded) is str and len(encoded) <= 4 * (((1 << 20) + 2) // 3),
                         "Global build stream bound")
            body = base64.b64decode(encoded, validate=True)
            base.require(len(body) <= 1 << 20 and row.get(kind + "_file") == name
                         and type(row.get(kind + "_bytes")) is int and row[kind + "_bytes"] == len(body)
                         and row.get(kind + "_sha256") == sha(body), "Global build original stream differs")
            if label == labels[0]:
                base.require(body == (b"go version go1.26.11 linux/amd64\n" if kind == "stdout" else b""),
                             "Global exact Go version")


def security_role(value):
    required = {"schema_version", "subject", "scanner", "policy", "outcome", "counts", "scanned_files", "report_digest"}
    base.require(type(value) is dict and required <= set(value) <= required | {"findings", "evidence_source"}
                 and type(value["schema_version"]) is int and value["schema_version"] == 1
                 and value["scanner"] == {"id": "lintai", "version": "0.1.3"} and value["policy"] == POLICY,
                 "Global actual security assessment")
    subject, counts = value["subject"], value["counts"]
    base.require(type(subject) is dict and set(subject) == {"tree_digest", "manifest_digest"}
                 and all(type(part) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", part) for part in subject.values())
                 and type(counts) is dict and set(counts) == {"blocking", "warnings", "total"}
                 and all(type(count) is int and 0 <= count <= 4096 for count in counts.values())
                 and counts["blocking"] == 0 and counts["warnings"] == counts["total"]
                 and value["outcome"] == ("warnings" if counts["warnings"] else "no_blocking_findings")
                 and type(value["scanned_files"]) is int and 0 <= value["scanned_files"] <= 512
                 and type(value["report_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", value["report_digest"]),
                 "Global scan subject/counts")
    key = sha("\0".join((subject["tree_digest"], subject["manifest_digest"], "lintai", "0.1.3",
                          POLICY["id"], "2", POLICY["digest"])).encode())
    return "security/assessments/" + key + ".json"


def installer_metadata(info):
    """Numeric facts from an existing observation; no path or content capture."""
    return {name: getattr(info, name) for name in (
        "st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")}


def installer_pin(path, budget, *, owner=None, metadata=None):
    base.require(path.is_absolute() and path.resolve(strict=True) == path, "prebuilt installer alias")
    before = path.lstat()
    owner = (os.getuid(), os.getgid()) if owner is None else owner
    if metadata is not None:
        metadata.update(installer_metadata(before))
    try:
        base.require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
                     and (before.st_uid, before.st_gid) == owner and stat.S_IMODE(before.st_mode) == 0o755
                     and 0 < before.st_size <= 128 << 20, "prebuilt installer authority")
    except ValueError as error:
        raise ExecutableAuthorityRefusal(before, owner) from error
    fd = base.shared.artifacts.open_member(path)
    try:
        held, digest, count = os.fstat(fd), hashlib.sha256(), 0
        base.require(base.shared.artifacts.same_opened(before, held), "prebuilt installer changed at open")
        while count <= before.st_size:
            budget.check()
            chunk = os.read(fd, min(65536, before.st_size + 1 - count))
            if not chunk: break
            count += len(chunk)
            base.require(count <= before.st_size, "prebuilt installer grew")
            digest.update(chunk)
        identity = lambda value: base.shared.artifacts.identity(value) + (value.st_uid, value.st_gid)
        base.require(count == before.st_size and identity(os.fstat(fd)) == identity(held)
                     and identity(path.lstat()) == identity(before),
                     "prebuilt installer changed")
        return count, digest.hexdigest()
    finally:
        os.close(fd)


def regular(ctx, relative, inventory, *, large=None, package_digest=None):
    info = inventory[relative]
    maximum = large[0] if large else 1 << 20
    base.require(len(info) == 9 and stat.S_ISREG(info[2]) and info[3] == 1
                 and info[0] == ctx.original_output_identity[0]
                 and tuple(info[4:6]) == ctx.original_output_identity[3:]
                 and 0 <= info[6] <= maximum, "Global ordinary file authority")
    if large:
        # materialize(), the managed launcher and pinned scanner all use0700.
        base.require(info[6] == large[0] and info[2] == stat.S_IFREG | 0o700, "Global exact large role metadata")
    value = base.read_bootstrap_file(ctx, relative, info, inventory,
                                     maximum=maximum, digest_only=large is not None, package_digest=package_digest)
    if large:
        base.require(info[6] == large[0] and value == large[1] and info[2] & 0o111,
                     "Global exact large role differs")
    return value


def capture(ctx, installer_pin, helper=None, *, authorities=None, inventory=None):
    """Admit the complete ordinary byte budget before reading any fixture file."""
    if inventory is None:
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
    base.require(all(inventory.get(name) == identity for name, identity in (authorities or {}).items()),
                 "Global original directory authority changed")
    roles = {name: ((213734376, base.BINARY) if ctx.root / name == Path(ctx.argv[0]) else
                    installer_pin if helper is not None and name == helper else
                    SCANNER if name == "home/uap-state/security/lintai/0.1.3/linux-amd64/lintai" else None)
             for name, info in inventory.items() if len(info) == 9}
    sizes = [inventory[name][6] for name, role in roles.items() if role is None]
    base.require(all(type(size) is int and 0 <= size <= 1 << 20 for size in sizes)
                 and sum(sizes) <= 16 << 20, "Global ordinary aggregate")
    result = {}
    for name, info in inventory.items():
        row = {"identity": info}
        if len(info) == 9:
            role = roles[name]
            value = regular(ctx, name, inventory, large=role)
            row["sha256"] = value if role else sha(value)
        result[name] = row
    base.require(base.metadata_inventory(ctx.root, ctx.budget) == inventory, "Global capture metadata changed")
    return inventory, result


def capture_package(ctx, active, inventory, helper_pin):
    """Current packagesnapshot framing; the exact helper is streamed once."""
    prefix = active.relative_to(ctx.root).as_posix() + "/"
    selected = {name[len(prefix):]: info for name, info in inventory.items() if name.startswith(prefix)}
    digest, bodies = hashlib.sha256(), {}
    for name, info in sorted(selected.items()):
        directory = len(info) == 5
        executable = not directory and bool(info[2] & 0o111)
        header = "{}\0{}\0{}\0{}\0".format("dir" if directory else "file", name,
                                          "true" if executable else "false", 0 if directory else info[6])
        digest.update(header.encode())
        if not directory:
            bodies[name] = regular(ctx, prefix + name, inventory,
                large=helper_pin if name == NS + "/agentplugins" else None, package_digest=digest)
    base.require(base.metadata_inventory(ctx.root, ctx.budget) == inventory, "Global package metadata changed")
    return bodies, "sha256:" + digest.hexdigest()


class Journey:
    def __init__(self, ctx, probes, expected, nonce, effects, installer, source):
        self.ctx, self.probes, self.expected = ctx, probes, copy.deepcopy(expected)
        self.nonce, self.effects, self.installer, self.source = nonce, effects, installer, source
        self.home = ctx.home / "uap-state"
        self.profile = Path(expected["user"]["root"])
        self.authorities = {path.relative_to(ctx.root).as_posix(): base.root_identity(path)
                            for path in (self.home, self.profile, source)}
        effects.verify(base.metadata_inventory(ctx.root, ctx.budget))
        self.cli_pin = installer_pin(installer, ctx.budget)
        ctx.identities[installer] = self.cli_pin[1]
        self.binding, self.identifier, self.helper = None, None, None
        self.data_receipt, self.package_members = None, set()
        self.installation_basis, self.binding_basis, self.data_authority = None, None, None
        self.security, self.assessment_path = None, None
        self.source_bodies = dict(ctx.global_source_bodies)
        self.source_sha = source_digest(self.source_bodies)
        self.verified_inventory = base.metadata_inventory(ctx.root, ctx.budget)
        self.state_history, self.native_labels, self.public_labels = [], [], []
        self.log_births = {}
        self.effects.max_logs, self.effects.max_members = 20, 36
        for proof in effects.proofs:
            for name, value in proof.items():
                if base.BOOTSTRAP_LOG.fullmatch(name):
                    self.log_births.setdefault(name, {"command": "ordinary-gate", "sha256": value["sha256"]})

    def phase(self, label):
        state = getattr(self.ctx, "global_readout_state", None)
        if state is not None: state["phase"] = label

    def identity_paths(self, identifier):
        base.require(type(identifier) is str and re.fullmatch(
            r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}", identifier),
            "Global installation identity")
        physical = NAME + "-" + sha(identifier.encode())[:12]
        active = self.home / "managed/clients/windsurf" / physical
        data = self.home / "plugin-data" / physical
        return physical, active, data

    def home_metadata(self, inventory, *, identifier=None, removed=False):
        """Close every output name before reading even the known state leaf."""
        identifier = identifier or self.identifier
        prefix = self.home.relative_to(self.ctx.root).as_posix() + "/"
        files = {name[len(prefix):] for name, info in inventory.items() if name.startswith(prefix) and len(info) == 9}
        if identifier is None:
            base.require(not any(name.startswith(prefix) for name in inventory), "Global unknown UAP state member")
            return
        _physical, active, data = self.identity_paths(identifier)
        marker = (data / ".agentplugins-data-owner.json").relative_to(self.home).as_posix()
        allowed = {"state-v2.json", "mutation.lock", marker, "security/lintai/0.1.3/linux-amd64/lintai"}
        if self.assessment_path is not None: allowed.add(self.assessment_path)
        if not removed:
            allowed.update((active / name).relative_to(self.home).as_posix()
                           for name in set(self.source_bodies) | {NS + "/agentplugins", NS + "/metadata.json"})
        base.require(files <= allowed and {"state-v2.json", "mutation.lock", marker} <= files,
                     "Global unknown UAP state member")
        dirs = {"operations-v2", "plugin-data/.agentplugins-data-operations",
                "managed", "managed/clients", "managed/clients/windsurf"}
        for name in allowed:
            dirs.update(parent.as_posix() for parent in Path(name).parents if str(parent) != ".")
        base.require(all(name[len(prefix):] in dirs for name, info in inventory.items()
                         if name.startswith(prefix) and len(info) == 5), "Global unknown UAP directory")
        for name in files:
            info = inventory[prefix + name]
            mode = 0o700 if name.endswith("/agentplugins") or name.endswith("/lintai") else (
                0o644 if name in {(active / leaf).relative_to(self.home).as_posix() for leaf in self.source_bodies} else 0o600)
            maximum = 128 << 20 if name.endswith("/agentplugins") else SCANNER[0] if name.endswith("/lintai") else 1 << 20
            base.require(info[2] == stat.S_IFREG | mode and info[3] == 1
                         and info[0] == self.ctx.original_output_identity[0]
                         and tuple(info[4:6]) == self.ctx.original_output_identity[3:] and 0 <= info[6] <= maximum,
                         "Global UAP member metadata")
        if self.data_authority is not None:
            base.require(all(inventory.get(name) == info for name, info in self.data_authority.items()),
                         "Global retained data authority changed")

    def guard_frame(self, inventory):
        base.require(all(inventory.get(name) == identity for name, identity in self.authorities.items()),
                     "Global original directory authority changed")
        self.home_metadata(inventory, removed=getattr(self, "removed", False))
        self.native_skill_metadata(inventory, absent=self.identifier is None or getattr(self, "removed", False)
                                   or getattr(self, "skill_quarantined", False))
        for name in self.verified_inventory.keys() | inventory.keys():
            if self.verified_inventory.get(name) != inventory.get(name):
                base.require(auxiliary(name), "Global unacknowledged fixture member")
        # Every original public delta is already joined to state, source and
        # receipts, and its complete metadata must still match that observation.
        projected = dict(self.effects.baseline)
        projected.update({name: info for name, info in inventory.items()
                          if auxiliary(name) or name == self.effects.config_path})
        self.effects.verify(projected)

    def frame(self, inventory=None):
        if inventory is None: inventory = base.metadata_inventory(self.ctx.root, self.ctx.budget)
        self.guard_frame(inventory)
        return capture(self.ctx, self.cli_pin, self.helper, authorities=self.authorities, inventory=inventory)

    def read(self, path, inventory):
        return regular(self.ctx, path.relative_to(self.ctx.root).as_posix(), inventory)

    def native_skill_metadata(self, inventory, *, absent=False):
        root = (self.profile / "skills" / SKILL).relative_to(self.ctx.root).as_posix()
        selected = {name for name in inventory if name == root or name.startswith(root + "/")}
        base.require(selected == (set() if absent else {root, root + "/SKILL.md"}),
                     "Global native Skill closed membership")
        if absent: return
        directory, leaf = inventory[root], inventory[root + "/SKILL.md"]
        original = self.ctx.original_output_identity
        base.require(len(directory) == 5 and directory[2] == stat.S_IFDIR | 0o755
                     and directory[0] == original[0] and tuple(directory[3:]) == original[3:]
                     and len(leaf) == 9 and leaf[2] == stat.S_IFREG | 0o644 and leaf[3] == 1
                     and leaf[0] == original[0] and tuple(leaf[4:6]) == original[3:]
                     and leaf[6] == len(self.source_bodies["skills/" + SKILL + "/SKILL.md"]),
                     "Global native Skill metadata")

    def state(self, inventory, *, removed=False, identifier=None):
        self.home_metadata(inventory, identifier=identifier, removed=removed)
        self.native_skill_metadata(inventory, absent=removed)
        value = base.strict_json(self.read(self.home / "state-v2.json", inventory))
        base.require(type(value) is dict and type(value.get("schema_version")) is int and value["schema_version"] == 5
                     and type(value.get("installations")) is list and len(value["installations"]) == 1
                     and not value.get("environment_receipts"), "Global state schema")
        item = value["installations"][0]
        current = item.get("installation_id")
        physical, active, data = self.identity_paths(current)
        base.require(current == (identifier or self.identifier) and item.get("declared_name") == NAME
                     and type(item.get("clients")) is dict, "Global installation identity")
        basis = {key: part for key, part in item.items()
                 if key not in ("clients", "data_retained", "operation_group_id", "updated_at")}
        if self.installation_basis is not None:
            base.require(basis == self.installation_basis, "Global source/package/data authority changed")
        if removed:
            base.require(self.installation_basis is not None and item["clients"] == {} and item.get("data_retained") is True,
                         "Global removal did not retain owned data")
            receipts = value.get("transaction_receipts")
            base.require(type(receipts) is list and len(receipts) == 1 and self.binding is not None
                         and item.get("operation_group_id") != self.binding["receipts"][-1]["operation_group_id"],
                         "Global state-level removal receipt")
            receipt_join(receipts[0], self.binding, item.get("operation_group_id"), len(self.binding["receipts"]) + 1,
                         self.binding["receipts"][-1]["after_digest"], removed=True)
            self.closed_home(inventory, removed=True)
            return value
        base.require(len(item["clients"]) == 1 and item.get("data_retained", False) is False
                     and value.get("transaction_receipts", []) == [], "Global single binding")
        binding = next(iter(item["clients"].values()))
        binding_id = "client_" + sha("\0".join((current, "windsurf", "user", str(active))).encode())[:24]
        data_id = "data_" + sha("\0".join((current, physical, "user")).encode())[:24]
        base.require(type(binding) is dict and binding.get("client_id") == "windsurf" and binding.get("scope") == "user"
                     and set(item["clients"]) == {binding_id} and binding.get("client_binding_id") == binding_id
                     and binding.get("data_receipt_id") == data_id and binding.get("target_locator") == str(active)
                     and binding.get("physical_artifact_id") == physical and binding.get("native_profile_root") == str(self.profile)
                     and binding.get("activation") == "prepared" and binding.get("materialization") == "materialized"
                     and binding.get("policy") == "allowed" and binding.get("activation_attested", False) is False
                     and binding.get("authentication_attested", False) is False
                     and binding.get("native_activation_attempt", "") == "" and binding.get("pending_native_intent") is None
                     and binding.get("verification") == "installation_verified"
                     and binding.get("authentication") == "not_checked", "Global binding authority")
        source = item.get("source", {})
        manifest = "sha256:" + sha(self.source_bodies["plugin.json"])
        base.require(source == {"source_binding_id": "src_" + sha((str(self.source) + "\0\0").encode())[:24],
                                "requested_source": str(self.source), "canonical_source": str(self.source),
                                "resolved_revision": "", "tree_digest": self.source_sha},
                     "Global source identity")
        package = item.get("package", {})
        base.require(package == {"loader_kind": "agent_plugins", "format_id": "agent-plugins/1.0.0",
                                  "schema_uri": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                  "declared_name": NAME, "version": "1.0.0", "manifest_digest": manifest,
                                  "inventory": {"mcp_present": True, "mcp_enabled": True,
                                                "mcp_servers": sorted(SERVERS), "skills": [SKILL]}},
                     "Global package identity")
        base.require(binding.get("package_revision") == {"version": "1.0.0", "tree_digest": self.source_sha,
                                                         "manifest_digest": manifest},
                     "Global binding package revision")
        base.require(self.security is not None and self.security["subject"] == {
            "tree_digest": self.source_sha, "manifest_digest": manifest}, "Global scan does not bind installed source")
        receipt = item.get("data_receipts", {}).get(data_id)
        base.require(type(item.get("data_receipts")) is dict and set(item["data_receipts"]) == {data_id}
                     and type(receipt) is dict and receipt.get("scope") == "user" and receipt.get("state") == "owned"
                     and receipt.get("physical_backend_id") == physical and receipt.get("locator") == str(data)
                     and receipt.get("data_receipt_id") == data_id and receipt.get("ownership_digest")
                     == "sha256:" + sha(("agentplugins-plugin-data-v1\0" + data_id + "\0" + str(data)).encode()),
                     "Global data ownership")
        base.require(self.data_receipt is None or receipt == self.data_receipt, "Global data ownership changed")
        self.data_receipt = copy.deepcopy(receipt)
        self.helper = (active / NS / "agentplugins").relative_to(self.ctx.root).as_posix()
        package_digest = self.verify_payload(active, inventory)
        receipts = binding.get("receipts")
        prior = self.binding["receipts"] if self.binding is not None else []
        base.require(type(receipts) is list and 1 <= len(receipts) <= 2
                     and all(type(row) is dict and row.get("phase") == "committed" and row.get("mutation_type") == "directory_swap"
                             and row.get("client_binding_id") == binding_id and row.get("active_path") == str(active)
                             and type(row.get("sequence")) is int and row["sequence"] == index + 1
                             and type(row.get("after_digest")) is str
                             and re.fullmatch(r"sha256:[0-9a-f]{64}", row["after_digest"]) for index, row in enumerate(receipts))
                     and receipts[:len(prior)] == prior and len(receipts) <= len(prior) + 1,
                     "Global committed public receipts")
        groups = []
        for index, row in enumerate(receipts):
            group = row.get("operation_group_id")
            receipt_join(row, binding, group, index + 1, receipts[index - 1]["after_digest"] if index else "")
            groups.append(group)
        base.require(len(set(groups)) == len(groups) and groups[-1] == item.get("operation_group_id"),
                     "Global installation operation group")
        digest = receipts[-1]["after_digest"]
        base.require(digest == package_digest, "Global committed package digest differs")
        entries = base.strict_json(self.read(self.profile / "mcp_config.json", inventory))["mcpServers"]
        wanted = [{"object_id": "package:windsurf:" + physical, "kind": "managed_package_directory",
                   "logical_name": NAME, "path": str(active), "managed_digest": digest, "protection_class": "managed"}]
        skill = self.source_bodies["skills/" + SKILL + "/SKILL.md"]
        wanted.append({"object_id": "devin_global_skill_directory:" + SKILL, "kind": "devin_global_skill_directory",
                       "logical_name": SKILL, "path": str(self.profile / "skills" / SKILL), "source_relative": "skills/" + SKILL,
                       "managed_digest": "sha256:" + sha(b"f\0SKILL.md\0-\0" + skill + b"\0"), "protection_class": "managed"})
        wanted.extend({"object_id": "devin_global_mcp_server:" + name, "kind": "devin_global_mcp_server",
                       "logical_name": name, "path": str(self.profile / "mcp_config.json"),
                       "managed_digest": entry_digest(name, entries[name]), "protection_class": "managed"} for name in SERVERS)
        objects = binding.get("native_objects")
        base.require(type(objects) is list and len(objects) == 4 and all(type(row) is dict for row in objects)
                     and sorted(objects, key=lambda row: row.get("object_id", "")) == sorted(wanted, key=lambda row: row["object_id"]),
                     "Global exact native receipt digest joins")
        stable_binding = {key: part for key, part in binding.items() if key not in ("updated_at", "receipts")}
        base.require(self.binding_basis is None or stable_binding == self.binding_basis, "Global binding authority changed")
        self.identifier, self.binding = current, binding
        self.installation_basis, self.binding_basis = copy.deepcopy(basis), copy.deepcopy(stable_binding)
        keys = [data.relative_to(self.ctx.root).as_posix(),
                (data / ".agentplugins-data-owner.json").relative_to(self.ctx.root).as_posix()]
        self.data_authority = {key: inventory[key] for key in keys}
        self.closed_home(inventory)
        return value

    def verify_payload(self, active, inventory):
        source_files = self.source_bodies
        for name, body in source_files.items():
            base.require(self.read(self.source / name, inventory) == body, "Global authored source changed")
        selected = {name[len(str(active.relative_to(self.ctx.root))) + 1:]: info
                    for name, info in inventory.items() if name.startswith(active.relative_to(self.ctx.root).as_posix() + "/")}
        files = {name for name, info in selected.items() if len(info) == 9}
        base.require(files == set(source_files) | {NS + "/agentplugins", NS + "/metadata.json"},
                     "Global package closed membership")
        self.package_members = {str((active / name).relative_to(self.home)) for name in files}
        bodies, package_digest = capture_package(self.ctx, active, inventory, self.cli_pin)
        for name, body in source_files.items():
            if name != "mcp.json":
                base.require(bodies[name] == body, "Global installed source bytes differ")
        metadata = base.strict_json(bodies[NS + "/metadata.json"])
        base.require(metadata == {"protocol": 1, "sha256": self.cli_pin[1], "cliVersion": "0.1.0-development"},
                     "Global managed helper metadata")
        document = base.strict_json(bodies["mcp.json"])
        base.require(type(document) is dict and set(document) == {"$schema", "mcpServers"}
                     and document["$schema"]
                     == "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "Global projected document settings")
        projected = document["mcpServers"]
        base.require(type(projected) is dict and set(projected) == set(SERVERS), "Global MCP projection membership")
        stdio, http = projected[SERVERS[0]], projected[SERVERS[1]]
        data = self.data_receipt["locator"]
        base.require(stdio == {"type": "stdio", "command": str(active / NS / "agentplugins"),
                               "args": ["--internal-stdio-v1", str(active), data, str(active), "plugin", "--",
                                        "python3", str(active / "inert.py")],
                               "env": {"PLUGIN_ROOT": str(active), "PLUGIN_DATA": data}}
                     and http == {"type": "streamable-http", "url": self.probes["user"].url},
                     "Global projected transport")
        native = base.strict_json(self.read(self.profile / "mcp_config.json", inventory))
        base.require(set(native) == {"mcpServers"}, "Global native unexpected root settings")
        entries = native.get("mcpServers", {})
        base.require(set(entries) == set(self.expected["user"]["mcp"]) | set(SERVERS)
                     and all(entries[name] == entry for name, entry in self.expected["user"]["mcp"].items()),
                     "Global foreign MCP changed")
        for name in SERVERS:
            entry = dict(projected[name]); entry.pop("type", None)
            if name == SERVERS[1]: entry["transport"] = "http"
            base.require(entries[name] == entry, "Global native entry differs from installed projection")
        skill = self.profile / "skills" / SKILL / "SKILL.md"
        base.require(self.read(skill, inventory) == source_files["skills/" + SKILL + "/SKILL.md"],
                     "Global projected Skill differs")
        return package_digest

    def closed_home(self, inventory, *, removed=False):
        self.home_metadata(inventory, removed=removed)
        marker = Path(self.data_receipt["locator"]) / ".agentplugins-data-owner.json"
        base.require(base.strict_json(self.read(marker, inventory)) == self.data_receipt, "Global data marker differs")
        lock = base.strict_json(self.read(self.home / "mutation.lock", inventory))
        base.require(set(lock) == {"pid", "acquired_at"} and type(lock["pid"]) is int and lock["pid"] > 0
                     and type(lock["acquired_at"]) is str and len(lock["acquired_at"]) <= 40,
                     "Global mutation lock shape")
        if self.assessment_path is not None:
            cached = base.strict_json(self.read(self.home / self.assessment_path, inventory))
            expected = dict(self.security); expected.pop("evidence_source", None)
            base.require(cached == expected, "Global source-bound scan cache differs")

    def record_effect(self, label, before, after, old, now):
        self.ctx.summary.setdefault("command_effects", []).append({
            "label": label,
            "changes": {name: {"before": old.get(name), "after": now.get(name)}
                        for name in old.keys() | now.keys() if old.get(name) != now.get(name)},
            "outer_changes": {name: {"before": before.get(name), "after": after.get(name)}
                              for name in before.keys() | after.keys() if before.get(name) != after.get(name)}})

    def public(self, label, argv, *, removed=False):
        self.phase(label)
        before, old = self.frame()
        base.require(installer_pin(self.installer, self.ctx.budget) == self.cli_pin, "prebuilt installer changed before public command")
        env = {**self.ctx.env, "AGENTPLUGINS_HOME": str(self.home),
               "PATH": str(Path(self.ctx.argv[0]).parent) + os.pathsep + self.ctx.env["PATH"]}
        confirmation = [] if label.startswith("uap-doctor") else ["--yes"]
        arguments = [str(self.installer), *argv, *confirmation, "--target", "windsurf", "--scope", "user", "--format", "json"]
        record, out, err = self.ctx.command(label, arguments,
                                           self.ctx.projects["user"], env, 90)
        base.require(type(record.get("exit")) is int and record["exit"] == 0 and record.get("error") is None,
                     "public Global lifecycle refused")
        base.require(installer_pin(self.installer, self.ctx.budget) == self.cli_pin, "prebuilt installer changed during public command")
        public_command_join(record, label, arguments, self.ctx.projects["user"], out, err)
        response = base.strict_json(out)
        base.require(set(response) == {"schema_version", "command", "result", "data"}
                     and type(response["schema_version"]) is int and response["schema_version"] == 1
                     and response["command"] == argv[0] and response["result"] == "success",
                     "public Global envelope refused")
        candidate = self.identifier
        if label == "uap-add":
            data = response["data"]
            security = data.get("security")
            self.assessment_path = security_role(security)
            base.require(security["subject"] == {"tree_digest": self.source_sha,
                         "manifest_digest": "sha256:" + sha(self.source_bodies["plugin.json"])},
                         "Global public scan source differs")
            self.security = security
            targets = data.get("targets")
            base.require(type(targets) is list and len(targets) == 1 and targets[0].get("target") == "windsurf"
                         and not targets[0].get("error"), "Global public selected target")
            candidate = targets[0].get("output", {}).get("result", {}).get("installation_id")
            self.identity_paths(candidate)  # Finite prospective names; not yet acknowledged ownership.
        after = base.metadata_inventory(self.ctx.root, self.ctx.budget)
        base.require(all(after.get(name) == identity for name, identity in self.authorities.items()),
                     "Global original directory authority changed")
        self.verify_public_metadata(before, after, doctor=label.startswith("uap-doctor"))
        state = self.state(after, removed=removed, identifier=candidate)
        if not removed:
            sequence = 2 if label == "uap-repair" or "uap-repair" in self.public_labels else 1
            base.require(len(self.binding["receipts"]) == sequence, "Global public receipt sequence")
        if label.startswith("uap-doctor"):
            doctor_result(response["data"], self.identifier, self.binding)
        else:
            public_result(response["data"], argv[0], state["installations"][0], self.binding)
            self.verify_public_delta(before, after, removed)
        self.removed, self.skill_quarantined = removed, False
        self.verified_inventory = after  # Only after the complete source/state/receipt/byte joins.
        _, now = self.frame(after)
        if label.startswith("uap-doctor"):
            base.require(now == old, "Global doctor wrote fixture")
        self.record_effect(label, before, after, old, now)
        self.ctx.summary.setdefault("global_public_joins", []).append({"label": label,
            "response_sha256": sha(out), "state_sha256": now[(self.home / "state-v2.json").relative_to(self.ctx.root).as_posix()]["sha256"],
            "installation_id": self.identifier, "operation_group_id": state["installations"][0]["operation_group_id"]})
        self.state_history.append(state)
        self.public_labels.append(label)
        base.zero_effects(self.probes, self.ctx.root / "stdio-started")

    def verify_public_metadata(self, before, after, *, doctor=False):
        allowed = {str(self.home.relative_to(self.ctx.root)),
                   str((self.profile / "mcp_config.json").relative_to(self.ctx.root)),
                   str((self.profile / "skills" / SKILL).relative_to(self.ctx.root))}
        for name in before.keys() | after.keys():
            if before.get(name) == after.get(name): continue
            base.require(not doctor and any(name == prefix or name.startswith(prefix + "/") for prefix in allowed),
                         "public Global changed foreign file or doctor metadata")

    def verify_public_delta(self, before, after, removed):
        self.verify_public_metadata(before, after)
        if removed:
            native = base.strict_json(self.read(self.profile / "mcp_config.json", after))
            base.require(native == {"mcpServers": self.expected["user"]["mcp"]}
                         and not (self.profile / "skills" / SKILL).exists(), "Global exact removal differs")

    def native(self, phase):
        for scope in ("a", "b"):
            for kind in ("mcp", "skills"):
                label = kind + "-" + phase + "-" + scope
                self.phase(label)
                before, old = self.frame()
                record, out, _ = self.ctx.probe(label, [kind, "list", "--json"], project=scope)
                base.require(type(record.get("exit")) is int and record["exit"] == 0 and record.get("error") is None,
                             "Global native inventory refused")
                after = base.metadata_inventory(self.ctx.root, self.ctx.budget)
                self.verify_native_metadata(before, after)
                after, now = self.frame(after)
                self.verify_native_delta(before, after, old, now, label)
                rows = base.strict_json(out)
                base.require(type(rows) is list and all(type(row) is dict for row in rows), "Global native row container")
                if kind == "mcp":
                    owned = [row for row in rows if row.get("name") in SERVERS]
                    base.require(len(owned) == (0 if phase == "removed" else 2), "Global native MCP membership")
                    if owned:
                        inventory = base.metadata_inventory(self.ctx.root, self.ctx.budget)
                        entries = base.strict_json(self.read(self.profile / "mcp_config.json", inventory))["mcpServers"]
                        expected = copy.deepcopy(self.expected)
                        expected["user"]["mcp"].update({name: entries[name] for name in SERVERS})
                    else: expected = self.expected
                    base.verify_metadata_sources(out, self.ctx, expected, scope)
                else:
                    selected = [row for row in rows if row.get("base_dir") == str(self.profile / "skills" / SKILL)]
                    base.require(len(selected) == (0 if phase == "removed" else 1), "Global native Skill membership")
                    for row in selected:
                        base.require(row == owned_skill_row(self.ctx, self.profile, scope), "Global native Skill identity")
                    foreign = [row for row in rows if row not in selected]
                    base.verify_skill_metadata_sources(json.dumps(foreign).encode(), self.ctx, self.expected, scope, self.nonce)
                self.record_effect(label, before, after, old, now)
                self.verified_inventory = after
                self.native_labels.append(label)
                base.zero_effects(self.probes, self.ctx.root / "stdio-started")

    def verify_native_metadata(self, before, after):
        # Compare every original/receipt-derived path before any post-native read.
        for name in before.keys() | after.keys():
            if before.get(name) != after.get(name):
                base.require(auxiliary(name), "native Global changed nonauxiliary authority")
                if name in before and base.BOOTSTRAP_LOG.fullmatch(name):
                    base.require(False, "existing task-born log changed")

    def verify_native_delta(self, before, after, old, now, label):
        for name in old.keys() | now.keys():
            if old.get(name) == now.get(name): continue
            base.require(auxiliary(name), "native Global changed nonauxiliary authority")
            if base.BOOTSTRAP_LOG.fullmatch(name) and name in before:
                base.require(False, "existing task-born log changed")
        born = {name for name in after.keys() - before.keys() if base.BOOTSTRAP_LOG.fullmatch(name)}
        base.require(len(born) <= 1, "Global native multiple new logs")
        for name in born:
            base.require(name not in self.log_births, "task-born log name reused")
            self.log_births[name] = {"command": label, "sha256": now[name]["sha256"]}

    def quarantine(self):
        self.phase("global-skill-quarantine")
        before = base.metadata_inventory(self.ctx.root, self.ctx.budget)
        self.guard_frame(before)
        quarantine_skill(self.ctx, self.profile, self.source_bodies["skills/" + SKILL + "/SKILL.md"])
        after = base.metadata_inventory(self.ctx.root, self.ctx.budget)
        old = (self.profile / "skills" / SKILL).relative_to(self.ctx.root).as_posix()
        new = (self.ctx.private / "deliberately-quarantined-skill").relative_to(self.ctx.root).as_posix()
        expected = {name: info for name, info in before.items() if name not in (old, old + "/SKILL.md")}
        expected.update({new + suffix: before[old + suffix] for suffix in ("", "/SKILL.md")})
        base.require(after == expected, "Global quarantine changed unrelated authority")
        self.skill_quarantined = True
        self.verified_inventory = after

    def execute(self):
        self.public("uap-add", ["add", str(self.source), "--no-save"])
        self.native("installed")
        self.public("uap-doctor-installed", ["doctor", self.identifier])
        self.quarantine()
        self.public("uap-repair", ["repair", self.identifier])
        self.native("repaired")
        self.public("uap-doctor-repaired", ["doctor", self.identifier])
        self.public("uap-remove", ["remove", self.identifier], removed=True)
        self.native("removed")
        base.require(self.public_labels == list(PUBLIC) and self.native_labels == list(NATIVE), "Global lifecycle sequence")
        self.ctx.summary.update(global_lifecycle_capture_complete=True, public_global_steps=self.public_labels,
                                native_global_steps=self.native_labels, task_born_logs=self.log_births,
                                global_state_history=self.state_history)


def quarantine_skill(ctx, profile, expected):
    parent, target = profile / "skills", ctx.private / "deliberately-quarantined-skill"
    base.require(not target.exists() and not target.is_symlink(), "Skill quarantine preexists")
    original = base.root_identity(parent / SKILL)
    held_parents = {path: base.root_identity(path) for path in (parent, ctx.private)}
    descriptors = []
    try:
        for path in (parent, ctx.private):
            fd = os.open(path, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NONBLOCK)
            descriptors.append(fd)
            base.require(base.KnownFileDiagnostics.identity(os.fstat(fd)) == held_parents[path], "quarantine parent changed")
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
        member = (parent / SKILL / "SKILL.md").relative_to(ctx.root).as_posix()
        base.require(regular(ctx, member, inventory) == expected, "quarantined Skill source differs")
        ctx.budget.check()
        base.require(base.root_identity(parent / SKILL) == original, "quarantine Skill changed")
        os.rename(SKILL, target.name, src_dir_fd=descriptors[0], dst_dir_fd=descriptors[1])
        base.require(base.root_identity(target) == original and not (parent / SKILL).exists(), "Skill quarantine inode differs")
        base.require(all(base.root_identity(path) == identity for path, identity in held_parents.items()), "quarantine parent replaced")
        inventory = base.metadata_inventory(ctx.root, ctx.budget)
        base.require(regular(ctx, (target / "SKILL.md").relative_to(ctx.root).as_posix(), inventory) == expected,
                     "quarantine source bytes changed")
        ctx.summary["deliberate_skill_quarantine"] = {"identity": original, "sha256": sha(expected)}
    finally:
        for fd in reversed(descriptors): os.close(fd)


def author_source(ctx, probes):
    source = ctx.private / "component-source"  # Covered by the original held private-root cleanup.
    skill = source / "skills" / SKILL / "SKILL.md"
    skill.parent.mkdir(parents=True)
    (source / "plugin.json").write_text(json.dumps({"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json",
                                                  "name": NAME, "version": "1.0.0"}))
    skill.write_text("---\nname: " + SKILL + "\ndescription: Owned Global source.\n---\nInert marker.\n")
    (source / "inert.py").write_text("from pathlib import Path\nPath(" + repr(str(ctx.root / "stdio-started")) + ").write_text('unexpected')\n")
    (source / "mcp.json").write_text(json.dumps({"mcpServers": {
        SERVERS[0]: {"type": "stdio", "command": "python3", "args": ["${PLUGIN_ROOT}/inert.py"]},
        SERVERS[1]: {"type": "streamable-http", "url": probes["user"].url}}}))
    (ctx.home / "uap-state").mkdir(mode=0o700)
    profile = Path(ctx.env["XDG_CONFIG_HOME"]) / "devin"
    profile.mkdir(mode=0o700, parents=True, exist_ok=True)
    lock = profile / "mcp_config.json.agentplugins.lock"
    with lock.open("xb"):
        pass
    lock.chmod(0o600)  # Exact UAP writer-lock leaf exists before the sole baseline.
    ctx.global_source_bodies = {path.relative_to(source).as_posix(): path.read_bytes()
                               for path in source.rglob("*") if path.is_file()}
    return source


def completion(summary):
    expected = ("source-commit", "source-tree", "source-clean", "git-init-a", "git-init-b", "git-init-c") + base.METADATA_PROBES
    tail = ("uap-add", *NATIVE[:4], "uap-doctor-installed", "uap-repair", *NATIVE[4:8],
            "uap-doctor-repaired", "uap-remove", *NATIVE[8:], "source-final-clean")
    commands, effects = summary.get("commands"), summary.get("command_effects")
    base.require(type(commands) is list and len(commands) == 37
                 and all(type(row) is dict and row.get("probe") == label and type(row.get("exit")) is int
                         and row["exit"] == 0 and "error" in row and row["error"] is None
                         for row, label in zip(commands, expected + tail)), "Global command completion refused")
    labels = base.METADATA_PROBES + tail[:-1]
    base.require(type(effects) is list and len(effects) == 30
                 and all(type(row) is dict and set(row) == {"label", "changes", "outer_changes"}
                         and row["label"] == label and type(row["changes"]) is dict and type(row["outer_changes"]) is dict
                         for row, label in zip(effects, labels)), "Global effect completion refused")
    base.require(summary.get("metadata_recipe") == "ordinary-global"
                 and summary.get("metadata_route") == ordinary.OrdinaryEffects.metadata_route
                 and summary.get("ordinary_metadata_capture_complete") is True
                 and summary.get("global_lifecycle_capture_complete") is True
                 and summary.get("metadata_capture_complete") is True and summary.get("admission_qualified") is False
                 and summary.get("native_json_schema") == summary.get("exact_source_admission") == base.METADATA_SCHEMA
                 and "error" not in summary and "global_build_authority_refusal" not in summary
                 and type(summary.get("qualification")) is dict
                 and set(summary["qualification"]) == set(base.shared.QUALIFICATION_CASES)
                 and all(value is False for value in summary["qualification"].values())
                 and summary.get("native_global_steps") == list(NATIVE)
                 and summary.get("public_global_steps") == list(PUBLIC), "Global completion refused")
    for field, empty in (("probe_methods", []), ("accepted_connections", 0)):
        values = summary.get(field)
        base.require(type(values) is dict and set(values) == {"a", "b", "user", "local", "legacy"}
                     and all(type(value) is type(empty) and value == empty for value in values.values()),
                     "Global endpoint effects")
    for row in effects:
        for field in ("changes", "outer_changes"):
            changes = row[field]
            base.require(len(changes) <= 4096 and all(type(delta) is dict and set(delta) == {"before", "after"}
                         and (delta["before"] is not None or delta["after"] is not None) for delta in changes.values()),
                         "Global effect delta shape")
    joins, history = summary.get("global_public_joins"), summary.get("global_state_history")
    base.require(type(joins) is list and type(history) is list and len(joins) == len(history) == len(PUBLIC),
                 "Global public state/command join count")
    previous_hash = None
    for joined, state, label in zip(joins, history, PUBLIC):
        base.require(type(joined) is dict and set(joined) == {"label", "response_sha256", "state_sha256", "installation_id", "operation_group_id"}
                     and joined["label"] == label, "Global public state/command join shape")
        command = next(row for row in commands if row["probe"] == label)
        effect = next(row for row in effects if row["label"] == label)
        delta = effect["changes"].get("home/uap-state/state-v2.json")
        if delta is not None:
            base.require(type(delta.get("after")) is dict, "Global public state effect missing")
            previous_hash = delta["after"].get("sha256")
        base.require(type(previous_hash) is str and re.fullmatch(r"[0-9a-f]{64}", previous_hash)
                     and joined["state_sha256"] == previous_hash and joined["response_sha256"] == command.get("stdout_sha256")
                     and type(state) is dict and len(state.get("installations", [])) == 1
                     and joined["installation_id"] == state["installations"][0].get("installation_id")
                     and joined["operation_group_id"] == state["installations"][0].get("operation_group_id"),
                     "Global public state/effect/command join")
    validate_build_receipt(summary.get("global_build_receipt"), summary)


def complete_ordinary_gate(ctx, effects):
    effects.verify(base.metadata_inventory(ctx.root, ctx.budget))
    ordinary.require_ordinary_commands(ctx, recipe="ordinary-global")
    base.require(type(effects.step) is int and effects.step == len(base.METADATA_PROBES)
                 and effects.after is not None and ctx.summary.get("metadata_capture_complete") is True
                 and len(ctx.summary.get("command_effects", [])) == len(base.METADATA_PROBES),
                 "ordinary metadata gate incomplete")
    ctx.summary["ordinary_metadata_capture_complete"] = True


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--installer", type=Path, required=True)
    args = parser.parse_args()
    def collect(ctx, probes, readout_state):
        ctx.global_readout_state = readout_state
        installer = args.installer
        build_installer(ctx, installer)
        source = author_source(ctx, probes)
        def continue_global(ctx, probes, expected, nonce, effects):
            complete_ordinary_gate(ctx, effects)
            Journey(ctx, probes, expected, nonce, effects, installer, source).execute()
        base.collect(ctx, probes, readout_state, effects_type=ordinary.OrdinaryEffects,
                     continuation=continue_global)
    return base.run(args.output, collector=collect, recipe="ordinary-global")


if __name__ == "__main__":
    raise SystemExit(main())
