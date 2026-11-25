"""Strict Marketing Kit joins. No policy interpretation and no exit-code oracle.

Authority must be the parsed stdout of the successful frozen-source Go adapter;
it is never constructed from local state. The caller owns subprocess provenance.
"""
import json
import base64
from itertools import islice
import os
import re
import subprocess
import stat
import tempfile
import tomllib
from datetime import datetime, timezone
from pathlib import Path

PRODUCER = "4858d973afcf6cf31b49bba98bdf06482dce6226"
ROOT_ALIAS = "marketing-kit"
# Match directoryv1.Cache.Load's encoded-artifact plus JSON framing bound.
MAX_CACHE_BYTES = 4 * (((4 << 20) + 2) // 3) + 4 * (((16 << 10) + 2) // 3) + 4096


def require(value, message):
    if not value:
        raise ValueError(message)


def reject_constant(_):
    raise ValueError("non-finite JSON value in closure authority")


def held_read(audit, path, root, *, max_bytes=1 << 20):
    """Reuse the existing ancestor/handle reader and join its exact bytes."""
    path = audit.in_case(path, root)
    try:
        before = path.lstat()
    except OSError as error:
        raise ValueError("closure held document unavailable") from error
    bounds = {} if max_bytes == 1 << 20 else {"max_bytes": max_bytes}
    first, status = audit.live.copilot_settings_read(path, root, **bounds)
    require(status is None and isinstance(first, str), "closure held ancestor read unavailable")
    body, observed = audit.read_saved_record(path, **bounds)
    repeated, status = audit.live.copilot_settings_read(path, root, **bounds)
    after = path.lstat()
    require(body is not None and observed is not None and status is None
            and body == first.encode("utf-8") and repeated == first,
            "closure document changed during held read")
    require(os.path.samestat(before, observed) and os.path.samestat(before, after)
            and all(getattr(before, field) == getattr(observed, field) == getattr(after, field)
                    for field in ("st_mode", "st_nlink", "st_size", "st_mtime_ns", "st_ctime_ns")),
            "closure document identity changed during held read")
    return body


def pins_for(audit):
    return {alias: {"distribution": distribution, **audit.live.MARKETING_KIT_SIGNED_CLOSURE[distribution]}
            for alias, distribution in audit.live.MARKETING_KIT_DISTRIBUTIONS.items()}


def checked_authority(audit, case, authority):
    pins = pins_for(audit)
    checked_policy_output(audit, case, authority)
    require(case["name"] == ROOT_ALIAS and case["client"] in audit.CLIENTS
            and case["scope"] in ("user", "project"), "unreviewed closure tuple")
    require(isinstance(authority, dict) and authority.get("producer_commit") == PRODUCER
            and type(authority.get("snapshot_schema")) is int and authority["snapshot_schema"] == 1
            and type(authority.get("snapshot_sequence")) is int and authority["snapshot_sequence"] > 0
            and isinstance(authority.get("snapshot_digest"), str)
            and re.fullmatch(r"sha256:[0-9a-f]{64}", authority["snapshot_digest"])
            and authority.get("client") == case["client"] and authority.get("scope") == case["scope"]
            and authority.get("policy_status") == "eligible"
            and authority.get("ineligible_distributions") == []
            and authority.get("signed_project_grant") is False, "missing frozen runtime authority")
    selected = authority.get("selections")
    expected = {p["distribution"]: p for alias, p in pins.items() if alias != ROOT_ALIAS}
    require(isinstance(selected, dict) and selected == expected
            and all(isinstance(p, dict) and type(p.get("release_sequence")) is int for p in selected.values()),
            "runtime policy selected a different immutable closure")
    root = pins[ROOT_ALIAS]
    require(case["source"] == {"repository": root["repository"], "revision": root["resolved_revision"],
                               "path": root["package_path"]}
            and all(case[k] == root[k] for k in ("tree_digest", "manifest_digest")), "root differs from reviewed direct source")
    return pins


def checked_policy_output(audit, case, authority):
    """Validate every typed adapter field before either acceptance branch."""
    require(isinstance(authority, dict) and authority.get("producer_commit") == PRODUCER
            and type(authority.get("snapshot_schema")) is int and authority["snapshot_schema"] == 1
            and type(authority.get("snapshot_sequence")) is int and authority["snapshot_sequence"] >= 43
            and isinstance(authority.get("snapshot_digest"), str)
            and re.fullmatch(r"sha256:[0-9a-f]{64}", authority["snapshot_digest"])
            and authority.get("key_id") in ("uap-directory-2026-01", "uap-directory-2026-11")
            and authority.get("client") == case["client"] and authority.get("scope") == case["scope"]
            and authority.get("target") in ("linux-amd64", "darwin-arm64", "windows-amd64")
            and authority.get("signed_project_grant") is False, "invalid typed runtime authority")
    observed = authority.get("observed_at")
    require(isinstance(observed, str) and re.fullmatch(r"\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z", observed),
            "runtime authority observation time absent")
    datetime.strptime(observed, "%Y-%m-%dT%H:%M:%SZ")
    pins = {p["distribution"]: p for alias, p in pins_for(audit).items() if alias != ROOT_ALIAS}
    selected, denied = authority.get("selections"), authority.get("ineligible_distributions")
    require(isinstance(selected, dict) and isinstance(denied, list)
            and all(isinstance(v, str) and v in pins for v in denied)
            and len(set(denied)) == len(denied) and set(selected).isdisjoint(denied)
            and set(selected) | set(denied) == set(pins), "runtime authority dependency partition differs")
    require(all(isinstance(p, dict) and type(p.get("release_sequence")) is int and p == pins[d]
                for d, p in selected.items()), "runtime authority selected a different source/release")
    require(authority.get("policy_status") == ("ineligible" if denied else "eligible"),
            "runtime authority eligibility partition inconsistent")


def checked_refusal_authority(audit, case, authority, refusal):
    checked_policy_output(audit, case, authority)
    root = pins_for(audit)[ROOT_ALIAS]
    require(case.get("name") == ROOT_ALIAS and case.get("source") == {
        "repository": root["repository"], "revision": root["resolved_revision"], "path": root["package_path"]}
        and all(case.get(k) == root[k] for k in ("tree_digest", "manifest_digest")), "refusal root source differs")
    require(authority["policy_status"] == "ineligible" and isinstance(refusal, dict)
            and refusal.get("code") == "dependency_conflict"
            and refusal.get("distribution") in authority["ineligible_distributions"],
            "dependency refusal is not backed by authenticated ineligibility")
    pin = audit.live.MARKETING_KIT_SIGNED_CLOSURE[refusal["distribution"]]
    require(refusal.get("version") == pin["package_version"], "dependency refusal version differs")
    return authority


def prepare(audit, directory, evidence_dir):
    """Build only this adapter against detached, unmodified released Go sources.

    Caller calls cleanup before ending directory lifetime. No product rebuild
    or released binary substitution occurs; only the policy adapter is built.
    """
    require(audit.RELEASE_COMMIT == PRODUCER, "policy adapter is not bound to the selected released CLI")
    frozen = directory / "policy-producer"
    helper = directory / ("source-policy.exe" if os.name == "nt" else "source-policy")
    git = audit.live.GIT_EXECUTABLE
    require(git, "Git prerequisite unavailable for frozen policy adapter")
    subprocess.run([git, "fetch", "--no-tags", "--depth=1", "origin", PRODUCER], cwd=audit.ROOT,
                   check=True, timeout=180)
    subprocess.run([git, "worktree", "add", "--detach", str(frozen), PRODUCER], cwd=audit.ROOT,
                   check=True, timeout=60)
    context = {"helper": helper, "frozen": frozen, "evidence_dir": evidence_dir, "git": git}
    try:
        head = subprocess.check_output([git, "rev-parse", "HEAD"], cwd=frozen, text=True).strip()
        status = subprocess.check_output([git, "status", "--porcelain", "--untracked-files=all", "--ignored"], cwd=frozen, text=True)
        require(head == PRODUCER and not status.strip(), "frozen policy source differs")
        env = dict(os.environ)
        env.update({"GOWORK": str(frozen / "go.work"), "GOTOOLCHAIN": "local", "GOFLAGS": "", "CGO_ENABLED": "0"})
        subprocess.run(["go", "build", "-o", str(helper), str(audit.ROOT / "scripts/released-source-policy.go")],
                       cwd=frozen / "cli", env=env, check=True, timeout=240)
        return context
    except BaseException as error:
        try:
            cleanup(audit, context)
        except BaseException as cleanup_error:
            error.add_note("frozen policy worktree cleanup failed: " + str(cleanup_error))
        raise


def cleanup(audit, context):
    """Remove the disposable linked worktree before its temporary parent exits."""
    subprocess.run([context["git"], "worktree", "remove", str(context["frozen"])],
                   cwd=audit.ROOT, check=True, timeout=60)


def authorize(audit, context, root, case, stage):
    """Capture actual held runtime bytes, then let frozen Go authenticate/select."""
    require(context is not None and stage in ("add", "update", "repair", "remove"), "runtime policy context unavailable")
    cache_path = audit.in_case(root / "uap-data/directory-v1-cache.json", root)
    body = held_read(audit, cache_path, root, max_bytes=MAX_CACHE_BYTES)
    cache = json.loads(body, object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant)
    require(isinstance(cache, dict) and type(cache.get("schema_version")) is int and cache["schema_version"] == 1
            and type(cache.get("sequence")) is int and cache["sequence"] > 0, "runtime cache identity invalid")
    snapshot = base64.b64decode(cache["snapshot"], validate=True)
    envelope = base64.b64decode(cache["envelope"], validate=True)
    require(0 < len(snapshot) <= 4 << 20 and 0 < len(envelope) <= 16 << 10, "runtime signed artifacts exceed bounds")
    require(re.fullmatch(r"[A-Za-z0-9_-]+", root.name), "unsafe evidence case name")
    observed = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    # Unverified cache wrappers never enter retained evidence.
    fd, name = tempfile.mkstemp(prefix="source-policy-cache-", dir=context["helper"].parent)
    try:
        with os.fdopen(fd, "wb") as stream:
            stream.write(body)
        output = subprocess.check_output([str(context["helper"]), "-producer-root", str(context["frozen"]),
            "-cache", name, "-client", case["client"], "-scope", case["scope"],
            "-target", context["target"], "-sequence", str(cache["sequence"]), "-digest", audit.digest(snapshot), "-at", observed],
            cwd=context["frozen"] / "cli", text=True, timeout=60)
    finally:
        Path(name).unlink(missing_ok=True)
    authority = json.loads(output, object_pairs_hook=audit.live._unique_json_object)
    require(len(output.encode("utf-8")) <= 64 << 10, "policy adapter output exceeds bound")
    checked_policy_output(audit, case, authority)
    require(authority.get("producer_commit") == PRODUCER and authority.get("snapshot_sequence") == cache["sequence"]
            and authority.get("snapshot_digest") == audit.digest(snapshot)
            and authority.get("target") == context["target"] and authority.get("observed_at") == observed
            and authority.get("policy_status") in ("eligible", "ineligible"), "policy adapter output differs from held runtime bytes")
    evidence = context["evidence_dir"] / (root.name + "-" + stage)
    evidence.mkdir(parents=True, exist_ok=False)
    (evidence / "snapshot.json").write_bytes(snapshot)
    (evidence / "envelope.json").write_bytes(envelope)
    (evidence / "policy.json").write_text(output, encoding="utf-8")
    authority["evidence_path"] = str(evidence.relative_to(context["evidence_dir"]))
    return authority


def source_key(alias, pin):
    return (f'github:{pin["repository"].lower()}:{pin["package_path"]}' if alias == ROOT_ALIAS
            else "directory:" + pin["distribution"])


def checked_closure(audit, case, root, documents, authority):
    """Join held saved intent/lock to all four live owners and signed children."""
    pins = checked_authority(audit, case, authority)
    bodies = []
    for path in documents:
        body = held_read(audit, path, root)
        require(body is not None, "closure saved documents absent")
        bodies.append(body)
    require(len(bodies) == 2, "closure requires manifest and lock")
    manifest = tomllib.loads(bodies[0].decode("utf-8"))
    p = pins[ROOT_ALIAS]
    expected_manifest = {"schema_version": 1, "dependencies": {ROOT_ALIAS: {
        "git": f'https://github.com/{p["repository"]}.git', "path": p["package_path"],
        "commit": p["resolved_revision"]}}, "targets": {case["client"]: [ROOT_ALIAS]}}
    require(type(manifest.get("schema_version")) is int and manifest == expected_manifest, "closure root intent changed")
    intent = {"schema_version": 1, "dependencies": [{"alias": ROOT_ALIAS,
        "source_key": source_key(ROOT_ALIAS, p), "path": p["package_path"],
        "selector": {"kind": "commit", "value": p["resolved_revision"]}}],
        "targets": [{"id": case["client"], "aliases": [ROOT_ALIAS]}]}
    locked = json.loads(bodies[1], object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant)
    require(isinstance(locked, dict) and type(locked.get("lock_version")) is int and locked["lock_version"] == 1 and locked.get("manifest_digest") ==
            audit.digest(json.dumps(intent, separators=(",", ":")).encode()), "closure lock intent mismatch")
    packages = locked.get("packages")
    require(isinstance(packages, list) and len(packages) == 4
            and all(isinstance(p, dict) for p in packages)
            and {p.get("alias") for p in packages} == set(pins), "closure lock inventory differs")
    # Use held reads for state too; do not let pathname JSON become authority.
    state_path = root / ("uap-data" if case["scope"] == "user" else "project/.uap") / "state-v2.json"
    body = held_read(audit, state_path, root)
    require(body is not None, "closure state absent")
    state = json.loads(body, object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant)
    require(isinstance(state, dict), "closure state must be an object")
    records, receipts = state.get("installations"), state.get("environment_receipts")
    require(type(state.get("schema_version")) is int and state["schema_version"] == 5 and isinstance(records, list) and len(records) == 4
            and all(isinstance(r, dict) for r in records)
            and {r.get("declared_name") for r in records} == set(pins)
            and isinstance(receipts, list) and len(receipts) == 4
            and all(isinstance(r, dict) for r in receipts), "closure owner inventory differs")
    owners, ids, binding_ids, receipt_ids, physical_ids = {}, set(), set(), set(), set()
    locators, directory_ids = set(), set()
    for alias, pin in pins.items():
        package = next(p for p in packages if p["alias"] == alias)
        record = next(r for r in records if r["declared_name"] == alias)
        checked_package(package, alias, pin, case)
        checked_record(record, alias, pin, authority)
        clients = record.get("clients")
        require(isinstance(clients, dict) and len(clients) == 1, "closure sibling binding")
        binding_id, binding = next(iter(clients.items()))
        require(binding.get("client_binding_id") == binding_id and binding.get("client_id") == case["client"]
                and binding.get("scope") == case["scope"] and binding.get("materialization") == "materialized",
                "closure placement mismatch")
        revision = binding.get("package_revision", {})
        require(all(revision.get(k) == pin[k] for k in ("tree_digest", "manifest_digest", "resolved_revision")),
                "closure binding digest mismatch")
        if alias != ROOT_ALIAS:
            require(type(revision.get("release_sequence")) is int and revision.get("distribution_id") == pin["distribution"]
                    and revision.get("release_sequence") == pin["release_sequence"], "closure binding release mismatch")
        owned = [r for r in receipts if r.get("installation_id") == record.get("installation_id")
                 and r.get("client_binding_id") == binding_id]
        require(len(owned) == 1, "closure receipt join is ambiguous")
        receipt = owned[0]
        require(receipt.get("phase") == "committed" and receipt.get("client_id") == case["client"]
                and receipt.get("alias") == alias and receipt.get("environment_id") == case["scope"] + "-default"
                and receipt.get("source_key") == source_key(alias, pin)
                and receipt.get("package_digest") == pin["tree_digest"], "closure receipt ownership mismatch")
        for value, seen in ((record.get("installation_id"), ids), (binding_id, binding_ids),
                            (receipt.get("receipt_id"), receipt_ids), (binding.get("physical_artifact_id"), physical_ids)):
            require(isinstance(value, str) and value.strip() == value and value and value not in seen,
                    "closure owner identity absent or duplicated")
            seen.add(value)
        locator = audit.in_case(Path(binding.get("target_locator", "")), root)
        info = locator.lstat()
        require(locator.is_absolute() and stat.S_ISDIR(info.st_mode) and not locator.is_symlink(),
                "closure managed package missing")
        resolved = audit.in_case(locator.resolve(strict=True), root)
        directory_id = (info.st_dev, info.st_ino)
        require(resolved not in locators and directory_id not in directory_ids,
                "closure native package locator or directory identity duplicated")
        locators.add(resolved)
        directory_ids.add(directory_id)
        owners[alias] = {"installation": record, "binding": binding, "receipt": receipt, "locator": locator}
    return owners


def checked_package(package, alias, pin, case):
    require(package.get("targets") == [case["client"]] and package.get("source_key") == source_key(alias, pin)
            and package.get("digest_algorithm") == "agentplugins-tree-sha256-v1"
            and all(package.get(k) == pin[k] for k in ("tree_digest", "manifest_digest"))
            and package.get("version") == pin["package_version"], "closure locked package mismatch")
    source = package.get("source", {})
    if alias == ROOT_ALIAS:
        require(source == {"git": {"repository": pin["repository"], "path": pin["package_path"],
            "requested": {"kind": "commit", "value": pin["resolved_revision"]}, "commit": pin["resolved_revision"]}}
            and not package.get("required_by"), "closure direct root source mismatch")
    else:
        require(set(source) == {"registry"}, "closure child changed source kind")
        registry = source["registry"]
        require(isinstance(registry, dict) and type(registry.get("release_sequence")) is int
            and all(registry.get(k) == v for k, v in {"distribution": pin["distribution"],
            "distribution_id": pin["distribution"], "release_sequence": pin["release_sequence"],
            "repository": pin["repository"], "path": pin["package_path"], "commit": pin["resolved_revision"],
            "requested": {"kind": "release", "value": str(pin["release_sequence"])}}.items())
            and package.get("required_by") == [{"alias": ROOT_ALIAS,
                "selector": {"kind": "version", "value": pin["package_version"]}}], "closure dependency edge/source mismatch")


def checked_record(record, alias, pin, authority):
    require(record.get("origin_mode") == ("direct" if alias == ROOT_ALIAS else "directory"), "closure origin mismatch")
    source = record.get("source", {})
    require(all(source.get(k) == pin[v] for k, v in (("repository", "repository"),
        ("package_subpath", "package_path"), ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest"))),
        "closure state source mismatch")
    require(record.get("package", {}).get("manifest_digest") == pin["manifest_digest"]
            and record.get("package", {}).get("version") == pin["package_version"], "closure state manifest mismatch")
    directory = record.get("directory")
    if alias == ROOT_ALIAS:
        require(directory is None, "direct root acquired Directory authority")
    else:
        require(isinstance(directory, dict) and directory.get("distribution_id") == pin["distribution"]
                and directory.get("desired_release_sequence") == pin["release_sequence"]
                and type(directory.get("snapshot_schema")) is int and type(directory.get("snapshot_sequence")) is int
                and type(directory.get("desired_release_sequence")) is int
                and all(directory.get(k) == authority[k] for k in ("snapshot_schema", "snapshot_sequence", "snapshot_digest")),
                "closure state is not joined to authenticated runtime bytes")


def checked_native(audit, owners, verifier):
    """Require a client-specific content proof for each already joined owner.

    verifier must inspect exact skill/MCP content and native ownership, not an
    existence check. Until all four oracles are supplied this fails closed.
    """
    require(callable(verifier), "closure native content oracle unavailable")
    require(set(owners) == set(audit.live.MARKETING_KIT_DISTRIBUTIONS), "closure native inventory differs")
    proofs = {alias: verifier(alias, owner) for alias, owner in owners.items()}
    require(all(isinstance(p, dict) and p.get("content_verified") is True for p in proofs.values()),
            "closure native content incomplete")
    return proofs


def checked_native_inventory(audit, locator, root, alias, client="antigravity"):
    """Closed AGY/Pi native projection of the four immutable source pins above.

    Frozen antigravity/project.go rewrites plugin.json, removes mcp.json and
    adds mcp_config.json for MCP members. Locked npm dependencies live under
    PLUGIN_DATA, never in this native plugin tree.
    """
    require(client in ("antigravity", "pi"), "unreviewed native inventory client")
    mcp = "mcp_config.json" if client == "antigravity" else "mcp.json"
    source_files = {
        # registry c5bcedd4, plugins/marketing-kit
        "marketing-kit": {"README.md", "SOURCES.md", "plugin.json", "uap.toml",
                          "skills/campaign-planning/SKILL.md"},
        # product 2ddbb99d, plugins/notion and plugins/hubspot-crm
        "notion": {"README.md", "plugin.json", mcp},
        "hubspot-crm": {"README.md", "plugin.json", mcp},
        # registry d9c8d778, plugins/playwright
        "playwright": {"NOTICE", "README.md", "plugin.json", mcp,
            *("io.github.777genius.agentplugins/runtime/" + name for name in
              ("launcher.mjs", "package.json", "package-lock.json", "runtime.json"))},
    }
    require(alias in source_files, "unreviewed native closure member")
    files = {Path(name) for name in source_files[alias]}
    directories = {Path(".")}
    for path in files:
        directories.update(path.parents)
    expected = files | directories
    held = {}
    for relative in directories:
        directory = audit.in_case(locator / relative, root)
        info = directory.lstat()
        require(stat.S_ISDIR(info.st_mode) and not directory.is_symlink(),
                "closure native inventory directory replaced")
        held[relative] = (info.st_mode, info.st_dev, info.st_ino)
        children = {locator / path for path in expected if path != Path(".") and path.parent == relative}
        require(set(islice(directory.iterdir(), len(children) + 1)) == children,
                "closure native inventory differs from immutable projection")
    for relative in files:
        held_read(audit, locator / relative, root)
    for relative, identity in held.items():
        directory = audit.in_case(locator / relative, root)
        info = directory.lstat()
        require((info.st_mode, info.st_dev, info.st_ino) == identity,
                "closure native inventory directory replaced")
        children = {locator / path for path in expected if path != Path(".") and path.parent == relative}
        require(set(islice(directory.iterdir(), len(children) + 1)) == children,
                "closure native inventory changed during held reads")


def checked_owned_data(audit, case, root, alias, owner, *, bound=True):
    """Join frozen identity/layout, held state receipt and private owner marker."""
    installation, binding = owner["installation"], owner["binding"]
    iid = installation.get("installation_id")
    require(alias in audit.live.MARKETING_KIT_DISTRIBUTIONS and type(iid) is str and iid,
            "closure data installation identity differs")
    physical = alias + "-" + audit.hashlib.sha256(iid.encode()).hexdigest()[:12]
    require(binding.get("physical_artifact_id") == physical,
            "closure data physical identity differs")
    scope = case["scope"]
    require(scope in ("user", "project"), "closure data scope differs")
    state_root = root / ("uap-data" if scope == "user" else "project/.uap")
    path = audit.in_case(state_root / "plugin-data" / physical, root)
    receipt_id = "data_" + audit.hashlib.sha256("\0".join((iid, physical, scope)).encode()).hexdigest()[:24]
    core = {"data_receipt_id": receipt_id, "physical_backend_id": physical, "scope": scope,
            "locator": str(path), "state": "owned",
            "ownership_digest": audit.digest(("agentplugins-plugin-data-v1\0" + receipt_id + "\0" + str(path)).encode())}
    recorded = installation.get("data_receipts")
    require(type(recorded) is dict and set(recorded) == {receipt_id},
            "closure data receipt inventory differs")
    allowed = set(core) | {"created_at", "updated_at"}
    def exact(value):
        return (type(value) is dict and set(core) <= set(value) <= allowed
                and all(value.get(key) == expected for key, expected in core.items())
                and all(type(value[key]) is str for key in set(value) - set(core)))
    require(exact(recorded[receipt_id]), "closure data state receipt differs")
    binding_id = binding.get("data_receipt_id")
    require(binding_id == receipt_id if bound else binding_id in (None, "", receipt_id),
            "closure data binding receipt differs")
    # in_case and held_read retain the existing no-follow ancestor/handle proof.
    directory = path.lstat()
    require(stat.S_ISDIR(directory.st_mode) and not path.is_symlink()
            and (os.name == "nt" or stat.S_IMODE(directory.st_mode) == 0o700)
            and (not hasattr(directory, "st_uid") or directory.st_uid == root.lstat().st_uid),
            "closure data directory is not private and owned")
    marker = path / ".agentplugins-data-owner.json"
    before = marker.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_nlink == 1
            and (os.name == "nt" or stat.S_IMODE(before.st_mode) == 0o600)
            and (not hasattr(before, "st_uid") or before.st_uid == directory.st_uid),
            "closure data marker is not private and owned")
    value = json.loads(held_read(audit, marker, root), object_pairs_hook=audit.live._unique_json_object,
                       parse_constant=reject_constant)
    require(exact(value), "closure data marker differs")
    final_marker = marker.lstat()
    require(os.path.samestat(before, final_marker) and all(getattr(before, key) == getattr(final_marker, key)
            for key in ("st_mode", "st_nlink", "st_uid", "st_gid", "st_size", "st_mtime_ns", "st_ctime_ns")),
            "closure data marker changed during held read")
    after = path.lstat()
    require(os.path.samestat(directory, after) and all(getattr(directory, key) == getattr(after, key)
            for key in ("st_mode", "st_uid", "st_gid", "st_mtime_ns", "st_ctime_ns")),
            "closure data directory changed during held read")
    return {"path": path, "receipt": core}


def checked_playwright_transport(audit, case, root, owner):
    """Exact frozen producer expansion; native values never supply authority."""
    locator = audit.in_case(owner["locator"], root)
    checked_playwright_runtime(audit, locator, root)
    data = checked_owned_data(audit, case, root, "playwright", owner)["path"]
    require(case["client"] in ("pi", "antigravity", "hermes")
            and (case["client"] != "hermes" or case["scope"] == "user"),
            "closure stdio transport tuple differs")
    # Frozen strings.NewReplacer preserves the authored slash suffix on Windows.
    entry = {"command": "node", "args": [str(locator) + "/io.github.777genius.agentplugins/runtime/launcher.mjs"],
             "env": {"PLUGIN_ROOT": str(locator), "PLUGIN_DATA": str(data)}, "cwd": str(locator)}
    if case["client"] == "antigravity":
        entry["env"] = {}
    return entry


def checked_member_native(audit, case, root, selected_root, alias, owner):
    """Exact Hermes/Antigravity physical content; Pi needs its trust oracle too."""
    client = case["client"]
    require(client in ("hermes", "antigravity"), "closure native client oracle unavailable")
    binding, locator = owner["binding"], owner["locator"]
    pin = pins_for(audit)[alias]
    objects = binding.get("native_objects")
    require(isinstance(objects, list) and all(isinstance(o, dict) for o in objects), "closure native ownership malformed")
    packages = [o for o in objects if o.get("kind") == "managed_package_directory"]
    members = [o for o in objects if o.get("kind") != "managed_package_directory"]
    require(len(packages) == 1 and packages[0].get("logical_name") == alias
            and packages[0].get("object_id") == f'package:{client}:{binding["physical_artifact_id"]}'
            and Path(packages[0].get("path", "")) == locator
            and packages[0].get("protection_class") == "managed"
            and isinstance(packages[0].get("managed_digest"), str)
            and re.fullmatch(r"sha256:[0-9a-f]{64}", packages[0]["managed_digest"])
            and not packages[0].get("user_modified"), "closure physical package ownership mismatch")
    if client == "antigravity":
        require(not members and locator.parent == selected_root / "plugins", "closure plugin escaped selected scope")
        checked_native_inventory(audit, locator, root, alias)
        body = held_read(audit, locator / "plugin.json", root)
        require(body is not None, "closure native manifest absent")
        manifest = json.loads(body, object_pairs_hook=audit.live._unique_json_object)
        require(manifest.get("$schema") == "https://antigravity.google/schemas/v1/plugin.json"
                and manifest.get("name") == alias and manifest.get("version") == pin["package_version"]
                and set(manifest) <= {"$schema", "name", "displayName", "version", "description"}, "closure native manifest mismatch")
    if alias == ROOT_ALIAS:
        skill = locator / "skills/campaign-planning/SKILL.md"
        if client == "hermes":
            require(len(members) == 1 and members[0].get("logical_name") == "campaign-planning"
                    and members[0].get("kind") == "hermes_user_skill"
                    and members[0].get("object_id") == "hermes_user_skill:campaign-planning"
                    and members[0].get("source_relative") == "skills/campaign-planning"
                    and Path(members[0].get("path", "")) == selected_root / "skills/campaign-planning"
                    and not members[0].get("user_modified")
                    and members[0].get("protection_class") == "managed", "closure native skill ownership mismatch")
            skill_root = audit.in_case(Path(members[0].get("path", "")), root)
            require(skill_root.is_absolute() and skill_root.is_relative_to(selected_root), "closure native skill escaped scope")
            skill = skill_root / "SKILL.md" if skill_root.is_dir() else skill_root
        body = held_read(audit, skill, root)
        require(body is not None and audit.digest(body) == "sha256:42ffb5ea182abd0c710735a28e6f6989cc8a4e125642f1d954dabd2e10ca76b1",
                "closure campaign skill differs from immutable source")
        require({p.name for p in skill.parent.iterdir()} == {"SKILL.md"}, "closure campaign skill contains unreviewed files")
        if client == "antigravity":
            require({p.name for p in (locator / "skills").iterdir()} == {"campaign-planning"}, "closure plugin contains unreviewed Skills")
        return {"content_verified": True, "path": str(skill.relative_to(root))}
    if alias == "notion":
        entry = {"url": "https://mcp.notion.com/mcp"}
    elif alias == "hubspot-crm":
        entry = {"url": "https://mcp.hubspot.com"}
    else:
        require(alias == "playwright", "unknown closure MCP member")
        entry = checked_playwright_transport(audit, case, root, owner)
    if client == "antigravity":
        path, collection = locator / "mcp_config.json", "mcpServers"
        if "url" in entry:
            entry = {"serverUrl": entry["url"]}
    else:
        path, collection = selected_root / "config.yaml", "mcp_servers"
        require(len(members) == 1 and members[0].get("kind") == "hermes_user_mcp"
                and members[0].get("logical_name") == alias
                and members[0].get("object_id") == "hermes_user_mcp:" + alias
                and Path(members[0].get("path", "")) == path
                and members[0].get("protection_class") == "managed", "closure native MCP ownership mismatch")
    body = held_read(audit, path, root)
    require(body is not None, "closure native MCP absent")
    document = (audit.strict_native_yaml(body) if client == "hermes"
                else json.loads(body, object_pairs_hook=audit.live._unique_json_object))
    require(isinstance(document, dict) and isinstance(document.get(collection), dict)
            and document[collection].get(alias) == entry, "closure native MCP content mismatch: " + alias)
    if client == "antigravity":
        require(document[collection] == {alias: entry}, "closure plugin contains unreviewed MCP entries")
    return {"content_verified": True, "path": str(path.relative_to(root)), "config_sha256": audit.digest(body)}


def checked_playwright_runtime(audit, locator, root):
    """Exact authored executable/config/lock bytes from reviewed d9c8d778 source."""
    expected = {"launcher.mjs": "3f24087a8798bc0db35accf806c39f5078008385bd8c442644fdc778e0b3031c",
                "package.json": "0e18420a5108f3df6f760cda0053cffbf45a8062fafd34e50ce7f762a5e13063",
                "package-lock.json": "1c10c6b0e0c7009fdc88fff6d4e295293d1f4f99c4b38bb7412a37504bf4ab7c",
                "runtime.json": "119856f0ee7ed7371a53c22677ca379d8499c74b8d381415193fd2bd4269aec4"}
    for name, sha in expected.items():
        path = audit.in_case(locator / "io.github.777genius.agentplugins/runtime" / name, root)
        body = held_read(audit, path, root)
        require(body is not None and audit.digest(body) == "sha256:" + sha, "closure Playwright executable/config differs from frozen source")


PI_STARTUP_STORES = ("auth.json", "models-store.json")


def pi_user_startup_scope(case):
    return (case.get("client"), case.get("scope"), case.get("name"), case.get("expected")) == (
        "pi", "user", ROOT_ALIAS, "dependency-closure") and case.get("components") == ["skills"]


def require_fresh_pi_startup_stores(audit, case, root, selected_root):
    require(pi_user_startup_scope(case), "Pi startup stores require the user Skill closure")
    audit.in_case(selected_root, root)
    require(not any((selected_root / name).exists() or (selected_root / name).is_symlink()
                    for name in PI_STARTUP_STORES), "Pi startup stores were not absent before add")


def pi_startup_store_custody(audit, case, root, selected_root):
    """Pi1.4 offline Skill RPC creates two empty private stores, never credentials.

    Pinned upstream 4a080252 auth-storage.ts FileAuthStorageBackend initializes
    both AuthStorage and FileModelsStore with literal {} on their first read.
    Keep their identities and metadata after add; names alone grant no authority.
    """
    require(pi_user_startup_scope(case), "Pi startup stores require the user Skill closure")
    result = {}
    for name in PI_STARTUP_STORES:
        path = selected_root / name
        before = path.lstat()
        require(stat.S_ISREG(before.st_mode) and not path.is_symlink() and before.st_nlink == 1
                and stat.S_IMODE(before.st_mode) == (0o666 if os.name == "nt" else 0o600)
                and (not hasattr(before, "st_uid") or before.st_uid == root.lstat().st_uid),
                "Pi startup store is not a private owned regular file")
        require(held_read(audit, path, root) == b"{}", "Pi startup store is not empty")
        after = path.lstat()
        fields = ("st_dev", "st_ino", "st_mode", "st_nlink", "st_uid", "st_gid",
                  "st_size", "st_mtime_ns", "st_ctime_ns", "st_birthtime_ns",
                  "st_file_attributes", "st_reparse_tag")
        stamp = tuple(getattr(before, field, None) for field in fields)
        require(stamp == tuple(getattr(after, field, None) for field in fields),
                "Pi startup store changed during custody read")
        result[name] = stamp
    return result


def checked_namespace(audit, case, root, selected_root, owners, removed=False, startup_stores=None):
    """Exact fresh-case namespaces; new unowned siblings cannot become baseline."""
    require(set(owners) == set(pins_for(audit)), "namespace owner inventory differs")
    client = case["client"]
    require(client in ("hermes", "antigravity", "pi"), "unreviewed closure namespace")
    locators = {owner["locator"] for owner in owners.values()}
    parents = {locator.parent for locator in locators}
    require(len(parents) == 1, "closure package storage is split across unreviewed namespaces")
    package_parent = next(iter(parents))
    checked_directory_namespace(audit, package_parent, root, set() if removed else locators)
    expected_top = {"foreign.txt"}
    if startup_stores is not None:
        require(pi_startup_store_custody(audit, case, root, selected_root) == startup_stores,
                "Pi startup stores changed after add")
        expected_top.update(PI_STARTUP_STORES)
    if client == "antigravity":
        require(package_parent == selected_root / "plugins", "Antigravity package namespace differs")
        expected_top.add("plugins")
    else:
        config = "config.yaml" if client == "hermes" else "mcp.json"
        expected_top.update({config, "skills", config + ".agentplugins.lock"})
        if client == "pi":
            require(package_parent == selected_root / "extensions", "Pi package namespace differs")
            expected_top.add("extensions")
        elif package_parent.is_relative_to(selected_root):
            expected_top.add(package_parent.relative_to(selected_root).parts[0])
        skills = selected_root / "skills"
        checked_directory_namespace(audit, skills, root, set() if removed else {skills / "campaign-planning"})
        path = selected_root / config
        body = held_read(audit, path, root)
        document = (audit.strict_native_yaml(body) if client == "hermes"
                    else json.loads(body, object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant))
        collection = "mcp_servers" if client == "hermes" else "mcpServers"
        expected_names = {audit.FOREIGN_NATIVE_MCP}
        if not removed:
            expected_names.update({"notion", "hubspot-crm", "playwright"})
        require(isinstance(document, dict) and isinstance(document.get(collection), dict)
                and set(document[collection]) == expected_names, "closure added or retained an unowned MCP entry")
        if client == "pi":
            require(set(document) == {collection}, "Pi closure retained unreviewed config fields")
        seed = ({"command": "echo", "args": ["unrelated-native-audit"]} if client == "hermes"
                else {"url": "https://foreign.invalid/mcp", "enabled": False})
        require(document[collection][audit.FOREIGN_NATIVE_MCP] == seed, "closure foreign MCP seed changed")
        checked_inert_lock(audit, selected_root / (config + ".agentplugins.lock"), root)
    require(not selected_root.is_symlink() and selected_root.is_dir(), "selected native namespace replaced")
    require({p.name for p in selected_root.iterdir()} <= expected_top, "closure created an unowned native sibling")
    require(held_read(audit, selected_root / "foreign.txt", root) == b"unmanaged source audit neighbor\n",
            "closure foreign sentinel changed")


def checked_directory_namespace(audit, path, root, expected):
    audit.in_case(path, root)
    require(not path.is_symlink(), "closure namespace is a symlink")
    if not path.exists():
        require(not expected, "closure native namespace is absent")
        return
    require(path.is_dir() and set(path.iterdir()) == expected, "closure native namespace has unowned entries")
    require(all(p.is_dir() and not p.is_symlink() for p in expected), "closure native member is not a real directory")


def checked_inert_lock(audit, path, root):
    if not path.exists() and not path.is_symlink():
        return
    body = held_read(audit, path, root)
    mode = stat.S_IMODE(path.lstat().st_mode)
    require(body == b"" and mode == (0o666 if os.name == "nt" else 0o600), "native config lock is not inert")


def data_custody(audit, case, root, owners):
    """Preserve existing data bytes and identities through the whole lifecycle."""
    result = {}
    for alias, owner in owners.items():
        if owner["installation"].get("data_receipts"):
            data = checked_owned_data(audit, case, root, alias, owner)
            result[alias] = {"receipt": data["receipt"],
                             "snapshot": audit.live.scoped_snapshot([data["path"]])}
    return result


def checked_cleanup(audit, owners, selected_root, case, root, documents=None, startup_stores=None):
    for alias, owner in owners.items():
        require(not owner["locator"].exists() and not owner["locator"].is_symlink(), "closure retained managed package")
        member_case = {**case, "name": alias}
        audit.checked_native_cleanup(selected_root, member_case, owner["binding"], root)
    state_path = root / ("uap-data" if case["scope"] == "user" else "project/.uap") / "state-v2.json"
    state = json.loads(held_read(audit, state_path, root), object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant)
    require(isinstance(state, dict) and type(state.get("schema_version")) is int and state["schema_version"] == 5
            and state.get("environment_receipts", []) == [], "closure retained saved owners")
    # Ordinary remove preserves persistent data. Frozen remove_prepare.go also
    # creates a retained receipt for a removed package that had no stdio data.
    records = state.get("installations")
    require(type(records) is list and len(records) == len(owners)
            and all(type(record) is dict for record in records)
            and {record.get("installation_id") for record in records}
                == {owner["installation"]["installation_id"] for owner in owners.values()},
            "closure retained data owner inventory differs")
    retained, data_paths = {}, set()
    for alias, owner in owners.items():
        original = owner["installation"]
        record = next(record for record in records if record["installation_id"] == original["installation_id"])
        require(record.get("clients") == {} and record.get("data_retained") is True
                and all(record.get(key) == original.get(key) for key in
                        ("declared_name", "source", "package", "origin_mode", "directory")),
                "closure retained data identity or binding differs")
        data = checked_owned_data(audit, case, root, alias,
                                  {**owner, "installation": record}, bound=False)
        data_paths.add(data["path"])
        if not original.get("data_receipts"):
            require(set(islice(data["path"].iterdir(), 2)) == {data["path"] / ".agentplugins-data-owner.json"},
                    "closure newly retained data contains unreviewed entries")
        retained[alias] = {"data_receipt_id": data["receipt"]["data_receipt_id"],
                           "path": data["path"].relative_to(root).as_posix(),
                           "ownership_digest": data["receipt"]["ownership_digest"], "state": "owned"}
    checked_directory_namespace(audit, state_path.parent / "plugin-data", root, data_paths)
    if documents is None:
        saved = root / ("uap-data" if case["scope"] == "user" else "project")
        documents = [saved / "uap.toml", saved / "uap.lock"]
    require(len(documents) == 2, "closure cleanup requires both saved documents")
    manifest = tomllib.loads(held_read(audit, documents[0], root).decode("utf-8"))
    locked = json.loads(held_read(audit, documents[1], root), object_pairs_hook=audit.live._unique_json_object, parse_constant=reject_constant)
    require(type(manifest.get("schema_version")) is int and manifest["schema_version"] == 1
            and manifest.get("dependencies", {}) == {} and manifest.get("targets", {}) == {}, "closure retained saved dependency or target")
    require(type(locked.get("lock_version")) is int and locked["lock_version"] == 1
            and locked.get("packages") == [], "closure retained locked child or dependency edge")
    checked_namespace(audit, case, root, selected_root, owners, removed=True, startup_stores=startup_stores)
    return retained
