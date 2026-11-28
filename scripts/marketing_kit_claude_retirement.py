"""Receipt-bound signed Project cache retirement; never native writer closure.

The caller must supply the genuine successful published remove receipt. That
adapter validates the managed digest before and after retirement using the
existing packagesnapshot.Builder authority. This helper joins its held byte
image to that receipt; it does not independently recompute the Go tree digest.
"""
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import stat
import sys
import time

_CUSTODY = run_path(str(Path(__file__).with_name("claude_cache_retirement_audit.py")))
require = _CUSTODY["require"]

# Reused from reviewed donor 9385bab57af7733090f08d2d6f1f62d44060cd0e,
# blob 1e1b939db2ff67617658bd92712cf94ae7051097. The Claude adapter subtree
# is byte-identical between immutable published tags 0.1.115 and 0.1.116.
# Exact published 0.1.115/0.1.116 contract for this immutable signed closure:
# marketing-kit contains only Skills; Playwright contains a local stdio MCP.
# With absent catalog evidence, the active/installed/allowed native qualifiers
# produce not_required for these two aliases. Remote Notion/HubSpot remain
# not_checked. Fresh prepared outcomes do not enter those active-only qualifiers.
# This pinned Project fixture's mutating SDK reload has absent catalog evidence;
# explicit catalog objects, including schema-1 unknown, need a different contract
# and refuse here. This is installation setup, never future model/tool credentials;
# no observed authentication value selects the oracle.


def expected_project_authentication(alias, binding):
    if binding["package_revision"].get("catalog_evidence") is not None:
        raise ValueError(f"Claude cache retirement {alias} authentication catalog evidence "
                         "differs from pinned Project checkpoint: expected=absent_or_null")
    if alias in ("marketing-kit", "playwright") and binding.get("activation") == "active":
        return "not_required"
    return "not_checked"


def authentication_checkpoint_label(row):
    """Report only public domain states; do not echo arbitrary state-file data."""
    if "authentication" not in row:
        return "missing"
    value = row["authentication"]
    known = ("not_checked", "not_required", "auth_pending", "authenticated", "failed")
    return value if type(value) is str and value in known else "invalid"


def _checkpoint_label(row, key, allowed):
    if type(row) is not dict:
        return "invalid"
    if key not in row:
        return "missing"
    value = row[key]
    return value if type(value) is str and value in allowed else "invalid"


def _checkpoint_readiness(binding):
    return {key: _checkpoint_label(binding, key, allowed) for key, allowed in (
        ("authentication", ("not_checked", "not_required", "auth_pending", "authenticated", "failed")),
        ("activation", ("active", "prepared", "manual_activation_required", "failed", "not_required")),
        ("verification", ("installation_verified", "package_validated", "runtime_verified", "not_run", "failed")),
        ("policy", ("allowed", "blocked", "approval_required")))}


def _checkpoint_catalog(binding, record):
    revision = binding.get("package_revision") if type(binding) is dict else None
    if type(revision) is not dict:
        return {"presence": "unavailable"}
    if "catalog_evidence" not in revision:
        return {"presence": "absent"}
    evidence = revision["catalog_evidence"]
    if evidence is None:
        return {"presence": "null"}
    if type(evidence) is not dict:
        return {"presence": "invalid"}
    schema = evidence.get("schema_version")
    compatibility = evidence.get("compatibility")
    selected = compatibility.get("claude") if type(compatibility) is dict else None
    directory = record.get("directory") if type(record) is dict else None
    digest = evidence.get("digest")
    digest_valid = type(digest) is str and re.fullmatch(r"sha256:[0-9a-f]{64}", digest) is not None
    recorded_digest = directory.get("snapshot_digest") if type(directory) is dict else None
    return {"presence": "object", "schema_version": schema if type(schema) is int and schema in (1, 2) else "invalid",
        "catalog_version": _checkpoint_label(evidence, "catalog_version", ("directory-snapshot-1", "directory-snapshot-2")),
        "digest_shape": "sha256" if digest_valid else "invalid",
        "digest_matches_recorded_snapshot": digest == recorded_digest if digest_valid and type(recorded_digest) is str else None,
        "claude_compatibility": "object" if type(selected) is dict else "absent" if type(compatibility) is dict and "claude" not in compatibility else "invalid",
        "authentication": _checkpoint_label(selected, "authentication", ("unknown", "required", "not_required")),
        "package": _checkpoint_label(selected, "package", ("native", "projected", "prepared", "unsupported")),
        "verification": _checkpoint_label(selected, "verification", ("tested", "schema_only", "not_tested"))}


def project_checkpoint_summary(state, public_rows, audit):
    """Finite diagnostic projection only; never installation or custody authority."""
    try:
        records = state.get("installations") if type(state) is dict else None
        if type(records) is not list or type(public_rows) is not list:
            return {"status": "unavailable"}
        members = {}
        for alias in ("marketing-kit", "notion", "hubspot-crm", "playwright"):
            distribution = audit["MARKETING_KIT_DISTRIBUTIONS"][alias]
            raw = [row for row in records if type(row) is dict and row.get("clients")
                   and type(row.get("directory")) is dict and row["directory"].get("distribution_id") == distribution]
            public = [row for row in public_rows if type(row) is dict and type(row.get("directory")) is dict
                      and row["directory"].get("recorded_distribution") == distribution]
            def select(rows):
                return rows[0] if len(rows) == 1 else {}, "one" if len(rows) == 1 else "missing" if not rows else "multiple"
            record, raw_status = select(raw)
            view, public_status = select(public)
            def binding(row):
                values = row.get("clients")
                if type(values) is dict:
                    values = list(values.values())
                if type(values) is not list:
                    return {}, "missing" if "clients" not in row else "invalid"
                if len(values) != 1:
                    return {}, "missing" if not values else "multiple"
                return (values[0], "one") if type(values[0]) is dict else ({}, "invalid")
            (raw_binding, raw_binding_status), (public_binding, public_binding_status) = binding(record), binding(view)
            members[alias] = {"recorded_selection": raw_status, "public_selection": public_status,
                "recorded_binding_selection": raw_binding_status, "public_binding_selection": public_binding_status,
                "same_installation_id": type(record.get("installation_id")) is str
                    and record["installation_id"] == view.get("installation_id") if raw_status == public_status == "one" else None,
                "recorded": _checkpoint_readiness(raw_binding), "public": _checkpoint_readiness(public_binding),
                "recorded_catalog_evidence": _checkpoint_catalog(raw_binding, record),
                "public_catalog_evidence": "not_exposed_by_public_schema"}
        return {"status": "observed", "diagnostic_only": True, "members": members}
    except Exception:
        return {"status": "unavailable"}


def _checkpoint_document(audit, case, path):
    text, error = audit["copilot_settings_read"](path, case)
    require(error is None and type(text) is str)
    return json.loads(text, object_pairs_hook=audit["_unique_json_object"],
                      parse_constant=lambda _: require(False))


def observe_project_checkpoint(case, project, public_rows, audit):
    """Read through the custody reader; a diagnostic failure never qualifies state."""
    try:
        return project_checkpoint_summary(_checkpoint_document(audit, case, project / ".uap/state-v2.json"), public_rows, audit)
    except Exception:
        return {"status": "unavailable"}


def require_owned_residuals(audit, residuals, allowed):
    """Keep the refusal actionable without exposing unsafe names or file data."""
    unexpected = set(residuals) - allowed
    if unexpected:
        # A literal filename can end in the same text as a scanner reason.
        # Project complete observations: unsafe names and reason-bearing rows
        # remain hash-only rather than publishing a guessed safe prefix.
        diagnostic = audit["copilot_snapshot_delta"]({}, {value: True for value in unexpected})
        diagnostic["unexpected_residual_count"] = len(unexpected)
        raise ValueError("Claude cache retirement has unowned native residuals: "
                         + json.dumps(diagnostic, sort_keys=True))



# Fixed private-fixture labels only. These observations establish no User ownership.
_DIAGNOSTIC_USER_KEYS = tuple(alias + "@skills-dir" for alias in
    ("marketing-kit", "notion", "hubspot-crm", "playwright"))


def _diagnostic_digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(",", ":"),
                                     allow_nan=False).encode()).hexdigest()


def _diagnostic_image(fd, audit):
    """Bounded old-FD read uses pread, leaving the authority's offset untouched."""
    before = os.fstat(fd)
    require(stat.S_ISREG(before.st_mode) and before.st_nlink in (0, 1)
            and before.st_uid == os.geteuid() and stat.S_IMODE(before.st_mode) == 0o600
            and 0 < before.st_size <= 1 << 20)
    body = os.pread(fd, (1 << 20) + 1, 0)
    require(type(body) is bytes and len(body) == before.st_size
            and _CUSTODY["_FILES"]["identity"](before)
            == _CUSTODY["_FILES"]["identity"](os.fstat(fd)))
    document = json.loads(body.decode("utf-8"), object_pairs_hook=audit["_unique_json_object"],
                          parse_constant=lambda _: require(False))
    require(type(document) is dict)
    return {"identity": _CUSTODY["_FILES"]["identity"](before),
            "uid": before.st_uid, "gid": before.st_gid,
            "sha256": hashlib.sha256(body).hexdigest(), "document": document}


def _diagnostic_named_image(path, case, audit, *, held_fd=None):
    """Use the existing no-follow member walk and read its exact held file."""
    before = path.lstat()
    identity = _CUSTODY["_FILES"]["identity"]
    case_fd = _CUSTODY["open_directory"](case)
    try:
        with audit["WINDSURF_GLOBAL_AUDIT"]["member"](case_fd, path.relative_to(case).as_posix()) as fd:
            _CUSTODY["named_directory"](case, case_fd)
            require(before.st_nlink == 1 and identity(before) == identity(os.fstat(fd))
                    == identity(path.lstat()))
            if held_fd is not None:
                require(identity(before) == identity(os.fstat(held_fd)))
            image = _diagnostic_image(fd if held_fd is None else held_fd, audit)
            require(image["identity"] == identity(before) == identity(path.lstat()))
            _CUSTODY["named_directory"](case, case_fd)
        return image
    finally:
        os.close(case_fd)


def _diagnostic_capture(subject):
    result = {"status": "unavailable", "code": "bounded_config_observation_unavailable",
              "named": None, "held": None, "expected_image_equal": None}
    try:
        require(not subject.finished and subject.retired_allowed is not None)
    except Exception:
        return result
    path = subject.profile / ".claude.json"
    try:
        result["named"] = _diagnostic_named_image(path, subject.case, subject.audit)
        result["status"] = "observed"
    except Exception:
        pass
    try:
        rows = [row for row in subject.passive_usage if row["path"] == path]
        require(len(rows) == 1)
        row = rows[0]
        held = _diagnostic_image(row["fd"], subject.audit)
        current = os.fstat(row["fd"])
        require(held["identity"] == _CUSTODY["_FILES"]["identity"](current)
                and (held["uid"], held["gid"]) == (current.st_uid, current.st_gid))
        result["held"] = held
        result["expected_image_equal"] = (
            held["identity"], held["uid"], held["gid"], held["sha256"]) == row["image"]
    except Exception:
        pass
    return result


def _diagnostic_foreign(document):
    foreign = dict(document)
    if type(document.get("pluginUsage")) is dict:
        foreign["pluginUsage"] = {key: value for key, value in document["pluginUsage"].items()
                                  if key not in _DIAGNOSTIC_USER_KEYS}
    return foreign


def _diagnostic_document(image, start_ms, end_ms, previous=None):
    document = image["document"]
    usage = document.get("pluginUsage")
    valid_usage = type(usage) is dict
    foreign = _diagnostic_foreign(document)
    fixed = {}
    for key in _DIAGNOSTIC_USER_KEYS:
        present = key in usage if valid_usage else None
        row = usage[key] if present else None
        object_row = type(row) is dict
        comparable = (previous is not None and type(previous.get("pluginUsage")) is dict
                      and key in previous["pluginUsage"] and present)
        unchanged = _CUSTODY["same_json"](previous["pluginUsage"][key], row) if comparable else None
        stamp = row.get("lastUsedAt") if object_row else None
        fixed[key] = {"present": present,
            "closed_row_shape": object_row and set(row) == {"usageCount", "lastUsedAt", "lastUsedNumStartups"},
            "usage_count_exact_zero": object_row and type(row.get("usageCount")) is int and row["usageCount"] == 0,
            "startups_exact_zero": object_row and type(row.get("lastUsedNumStartups")) is int and row["lastUsedNumStartups"] == 0,
            "timestamp_in_window": None if start_ms is None else type(stamp) is int and start_ms <= stamp <= end_ms,
            "row_unchanged": unchanged,
            "row_sha256": _diagnostic_digest(row) if present else None}
    others = sorted(set(usage) - set(_DIAGNOSTIC_USER_KEYS)) if valid_usage else []
    return {"raw_sha256": image["sha256"], "json_sha256": _diagnostic_digest(document),
            "foreign_json_sha256": _diagnostic_digest(foreign),
            "metadata_sha256": _diagnostic_digest((image["identity"], image["uid"], image["gid"])),
            "first_start_version_matches": document.get("firstStartVersion") == "2.1.263",
            "usage_object": valid_usage, "fixed_fixture_keys": fixed,
            "other_usage_key_count": len(others),
            "other_usage": [{"name_json_sha256": _diagnostic_digest(key),
                             "row_sha256": _diagnostic_digest(usage[key])} for key in others[:8]],
            "omitted_other_usage_keys": max(0, len(others) - 8)}


def observe_user_initialize_diagnostic(subject, action, sink):
    """Measure one native capture; never adopt bytes or change retirement state."""
    before = _diagnostic_capture(subject)
    returned, checked = False, False
    action_start_ms = None
    try:
        subject.checkpoint()
        checked = True
        action_start_ms = time.time_ns() // 1_000_000
        result = action()
        returned = True  # capture itself still checks exit/error/stderr/binary.
        return result
    finally:
        end_ms = time.time_ns() // 1_000_000
        after = _diagnostic_capture(subject)
        observation = {"diagnostic_only": True, "ownership": "ownership_unqualified",
                       "native_capture_returned": returned, "pre_initialize_checkpoint_passed": checked,
                       "initialization_qualified": False,
                       "clock_window_valid": action_start_ms is not None and action_start_ms <= end_ms,
                       "old_fd_same_inode": None, "old_fd_bytes_equal": None,
                       "before": {"status": before["status"]}, "after": {"status": after["status"]}}
        try:
            for label, image in (("before", before), ("after", after)):
                observation[label].update(retirement_fd_raw_sha256=None, retirement_fd_link_count=None,
                                          named_joins_retirement_fd=None, retirement_image_equal=None)
                if image["status"] == "observed":
                    previous = before["named"]["document"] if label == "after" and before["status"] == "observed" else None
                    observation[label].update(_diagnostic_document(
                        image["named"], action_start_ms, end_ms, previous))
                else:
                    observation[label]["code"] = image["code"]
                if image["held"] is not None:
                    observation[label]["retirement_fd_raw_sha256"] = image["held"]["sha256"]
                    observation[label]["retirement_fd_link_count"] = image["held"]["identity"][3]
                    observation[label]["named_joins_retirement_fd"] = (
                        image["named"]["identity"] == image["held"]["identity"] if image["named"] is not None else None)
                    observation[label]["retirement_image_equal"] = image["expected_image_equal"]
            if before["status"] == after["status"] == "observed":
                a, b = before["named"], after["named"]
                observation.update(
                    same_named_inode=a["identity"][:2] == b["identity"][:2],
                    same_named_metadata=(a["identity"], a["uid"], a["gid"]) == (b["identity"], b["uid"], b["gid"]),
                    full_json_equal=_CUSTODY["same_json"](a["document"], b["document"]),
                    foreign_json_equal=_CUSTODY["same_json"](_diagnostic_foreign(a["document"]), _diagnostic_foreign(b["document"])))
            if before["held"] is not None and after["held"] is not None:
                observation.update(
                    old_fd_same_inode=before["held"]["identity"][:2] == after["held"]["identity"][:2],
                    old_fd_bytes_equal=before["held"]["sha256"] == after["held"]["sha256"])
        except Exception:
            observation = {"diagnostic_only": True, "ownership": "ownership_unqualified",
                           "status": "unavailable", "code": "projection_unavailable",
                           "native_capture_returned": returned, "pre_initialize_checkpoint_passed": checked,
                           "initialization_qualified": False}
        sink.append(observation)


class SignedClosureRetirement:
    """Hold all four owned caches continuously across one Project removal."""

    def __init__(self, case_root, home, project, public_rows, pins, audit, checkpoint_diagnostics=None):
        self.fds, self.owners, self.anchors = [], [], {}
        self.passive_usage = []
        self.usage_contract = None
        self.detached_usage = []
        self.user_usage = None
        self.backup_namespace = None
        self.retained_backups = []
        self.finished = False
        self.retired_allowed = None
        if checkpoint_diagnostics is not None:
            checkpoint_diagnostics.append({"status": "unavailable"})
        try:
            require(sys.platform == "linux")
            self.case, self.home, self.project = map(Path, (case_root, home, project))
            self.audit, self.profile = audit, self.home / ".claude"
            for path in (self.case, self.home, self.project, self.profile):
                _CUSTODY["ordinary"](path)
            require(self.home.is_relative_to(self.case) and self.project.is_relative_to(self.case)
                    and self.home != self.project)
            require(_CUSTODY["same_json"](pins, audit["MARKETING_KIT_SIGNED_CLOSURE"]))
            aliases = audit["MARKETING_KIT_DISTRIBUTIONS"]
            require(set(aliases) == {"marketing-kit", "notion", "hubspot-crm", "playwright"}
                    and set(aliases.values()) == set(pins)
                    and type(public_rows) is list and len(public_rows) == 4)
            state = self.document(self.project / ".uap/state-v2.json")
            if checkpoint_diagnostics is not None:
                checkpoint_diagnostics[-1] = project_checkpoint_summary(state, public_rows, audit)
            require(type(state.get("schema_version")) is int and state["schema_version"] == 5)
            records = state["installations"]
            require(type(records) is list)
            active = [row for row in records if row.get("clients")]
            require(len(active) == 4)
            ids, physicals, bindings = set(), set(), set()
            for alias, distribution in aliases.items():
                pin = pins[distribution]
                selected = [row for row in public_rows
                            if row.get("directory", {}).get("recorded_distribution") == distribution]
                require(len(selected) == 1)
                public = selected[0]
                selected = [row for row in active if row.get("installation_id") == public.get("installation_id")]
                require(len(selected) == 1)
                owner = self.capture(alias, distribution, pin, public, selected[0])
                require(owner["installation_id"] not in ids and owner["physical"] not in physicals
                        and owner["binding_id"] not in bindings)
                ids.add(owner["installation_id"]); physicals.add(owner["physical"])
                bindings.add(owner["binding_id"])
                self.owners.append(owner)
            self.installed = self.document(self.profile / "plugins/installed_plugins.json")
            self.markets = self.document(self.profile / "plugins/known_marketplaces.json")
            require(type(self.installed.get("version")) is int and self.installed["version"] == 2)
            for owner in self.owners:
                selected = {k: v for k, v in self.installed["plugins"].items()
                            if k.endswith("@" + owner["market"])}
                require(set(selected) == {owner["key"]} and len(selected[owner["key"]]) == 1)
                row = selected[owner["key"]][0]
                require(row["scope"] == "project" and row["projectPath"] == str(self.project)
                        and row["installPath"] == str(owner["cache"]))
                market = self.markets[owner["market"]]
                require(market["source"] == {"source": "directory", "path": str(owner["active"])}
                        and market["installLocation"] == str(owner["active"]))
            self.capture_passive_usage()
            self.guard()
        except BaseException:
            self.close()
            raise

    def document(self, path):
        return _checkpoint_document(self.audit, self.case, path)

    def passive_image(self, path, fd):
        """Join the original FD to the no-follow walk and hash that exact FD."""
        image = _diagnostic_named_image(path, self.case, self.audit, held_fd=fd)
        return (image["identity"], image["uid"], image["gid"], image["sha256"])

    def held_passive_document(self, path):
        require(stat.S_ISREG(path.lstat().st_mode))
        fd = _CUSTODY["_FILES"]["open_member"](path); self.fds.append(fd)
        observed = _diagnostic_named_image(path, self.case, self.audit, held_fd=fd)
        image = (observed["identity"], observed["uid"], observed["gid"], observed["sha256"])
        document = observed["document"]
        require(self.passive_image(path, fd) == image)
        return document, {"path": path, "fd": fd, "image": image}

    def capture_passive_usage(self):
        """Bind only pinned Claude's zero-count usage history to these receipts."""
        path = self.profile / ".claude.json"
        if not path.exists() and not path.is_symlink():
            return
        document, held = self.held_passive_document(path)
        require(type(document) is dict)
        usage = document.get("pluginUsage")
        keys = {owner["key"] for owner in self.owners}
        if type(usage) is not dict or not keys.intersection(usage):
            return
        require(document.get("firstStartVersion") == "2.1.263" and keys <= set(usage))
        for key in keys:
            row = usage[key]
            require(type(row) is dict and set(row) == {"usageCount", "lastUsedAt", "lastUsedNumStartups"}
                    and type(row["usageCount"]) is int and row["usageCount"] == 0
                    and type(row["lastUsedNumStartups"]) is int and row["lastUsedNumStartups"] == 0
                    and type(row["lastUsedAt"]) is int and 10 ** 12 <= row["lastUsedAt"] < 10 ** 13)
        foreign = dict(document, pluginUsage={key: row for key, row in usage.items() if key not in keys})
        body = json.dumps(foreign, sort_keys=True)
        aliases = self.audit["MARKETING_KIT_DISTRIBUTIONS"]
        for alias in set(aliases) | {distribution.rsplit("/", 1)[-1] for distribution in aliases.values()}:
            require(not self.audit["exact_native_identity"](body, alias)
                    and re.search(r"(?<![A-Za-z0-9_./:-])" + re.escape(alias)
                        + r"@agentplugins-[0-9a-f]{12}(?![A-Za-z0-9_./-])", body, re.IGNORECASE) is None)
        self.passive_usage.append(held)
        self.usage_contract = {"rows": {key: usage[key] for key in keys}, "foreign": foreign}
        # Claude 2.1.263's concurrent usage writes can leave one immutable
        # atomic-write temporary file. Bind its exact pre-removal name/inode,
        # complete foreign projection and subset of the same four usage rows.
        temporary = [path for path in self.profile.iterdir()
                     if re.fullmatch(r"\.claude\.json\.tmp\.[1-9][0-9]{0,9}\.[0-9a-f]{12}", path.name)]
        self.require_temporary_bound(temporary, "capture")
        for path in temporary:
            pending, held = self.held_passive_document(path)
            require(type(pending) is dict and type(pending.get("pluginUsage")) is dict)
            pending_usage = pending["pluginUsage"]
            selected = keys.intersection(pending_usage)
            require(bool(selected) and all(_CUSTODY["same_json"](pending_usage[key], usage[key]) for key in selected)
                    and held["image"][1:3] == self.passive_usage[0]["image"][1:3]
                    and _CUSTODY["same_json"](dict(pending, pluginUsage={
                        key: row for key, row in pending_usage.items() if key not in keys}), foreign))
            self.passive_usage.append(held)

    def require_temporary_bound(self, temporary, phase):
        """Bound the phase before strict member validation; diagnostics grant no authority."""
        require(phase in ("capture", "finish"))
        limit = 1 if phase == "capture" else 2
        if len(temporary) <= limit:
            return
        prebound = {row["path"] for row in self.passive_usage[1:]} if phase == "finish" else set()
        observed = set(temporary)
        detail = {"diagnostic_only": True, "ownership": "ownership_unqualified",
                  "phase": phase, "temporary_count": len(temporary),
                  "prebound_temporary_count": len(prebound),
                  "observed_prebound_temporary_count": len(observed & prebound),
                  "new_temporary_count": len(observed - prebound),
                  "limit": limit, "members": [], "omitted_count": max(0, len(temporary) - 8)}
        for path in sorted(temporary[:8], key=lambda p: p.name):
            member = {"status": "unavailable"}
            try:
                member["name_sha256"] = _diagnostic_digest(path.name)
                before = path.lstat()
                member["regular_0600_owner_single_link"] = (
                    stat.S_ISREG(before.st_mode) and stat.S_IMODE(before.st_mode) == 0o600
                    and before.st_uid == os.geteuid() and before.st_nlink == 1)
                require(member["regular_0600_owner_single_link"])
                image = _diagnostic_named_image(path, self.case, self.audit)
                joined = (image["identity"] == _CUSTODY["_FILES"]["identity"](before)
                          == _CUSTODY["_FILES"]["identity"](path.lstat()))
                member["named_held_identity_equal"] = joined
                require(joined)
                member["raw_sha256"] = image["sha256"]
                member["bytes"] = image["identity"][4]
                member["same_original_owner"] = (image["uid"], image["gid"]) == self.passive_usage[0]["image"][1:3]
                document, contract = image["document"], self.usage_contract
                usage = document.get("pluginUsage")
                member["usage_object"] = type(usage) is dict
                require(member["usage_object"])
                keys = set(contract["rows"])
                selected = keys.intersection(usage)
                member["selected_original_row_count"] = len(selected)
                member["original_rows_unchanged"] = all(
                    _CUSTODY["same_json"](usage[key], contract["rows"][key]) for key in selected)
                member["foreign_json_equal"] = _CUSTODY["same_json"](
                    dict(document, pluginUsage={key: value for key, value in usage.items() if key not in keys}),
                    contract["foreign"])
                old = next((row for row in self.passive_usage[1:] if row["path"] == path), None)
                member["previously_held"] = old is not None
                member["nonempty_original_subset"] = bool(selected)
                member["new_temporary_strict_subset"] = None if old else 0 < len(selected) < len(keys)
                if old:
                    held = _diagnostic_image(old["fd"], self.audit)
                    member["old_temporary_image_unchanged"] = (
                        (image["identity"], image["uid"], image["gid"], image["sha256"])
                        == (held["identity"], held["uid"], held["gid"], held["sha256"]) == old["image"])
                member["status"] = "observed"
            except Exception:
                member["code"] = "bounded_temporary_observation_unavailable"
            detail["members"].append(member)
        error = ValueError("passive usage temporary bound: " + json.dumps(detail, sort_keys=True))
        error.claude_temporary_bound_diagnostic = detail
        raise error

    def validate_usage_deletion(self, document):
        """Only receipt-owned zero-count rows may disappear; all other JSON stays exact."""
        require(type(document) is dict and type(document.get("pluginUsage")) is dict)
        usage, contract = document["pluginUsage"], self.usage_contract
        keys = set(contract["rows"])
        require(all(_CUSTODY["same_json"](usage[key], contract["rows"][key]) for key in keys.intersection(usage))
                and _CUSTODY["same_json"](dict(document, pluginUsage={
                    key: row for key, row in usage.items() if key not in keys}), contract["foreign"]))

    def finish_passive_usage(self):
        """Admit the pinned uninstall's exact deletion once, after its verified receipt."""
        if self.usage_contract is None:
            return
        old_main, *old_temporary = self.passive_usage
        # Atomic replacement can unlink the old main while its descriptor stays
        # ours. Its held bytes must still have the same foreign JSON and may
        # only reflect the same permitted deletion. No pathname rediscovery is
        # used to recover this original authority.
        before = os.fstat(old_main["fd"])
        require((before.st_dev, before.st_ino, before.st_mode) == old_main["image"][0][:3]
                and before.st_nlink in (0, 1) and before.st_uid == old_main["image"][1]
                and before.st_gid == old_main["image"][2] and before.st_size <= 1 << 20)
        held_main = _diagnostic_image(old_main["fd"], self.audit)
        require(held_main["identity"] == _CUSTODY["_FILES"]["identity"](before))
        self.validate_usage_deletion(held_main["document"])
        document, main = self.held_passive_document(old_main["path"])
        self.validate_usage_deletion(document)
        same_main = main["image"][0][:2] == old_main["image"][0][:2]
        remaining = len(set(document["pluginUsage"]).intersection(self.usage_contract["rows"]))
        require(before.st_nlink == (1 if same_main else 0)
                and (remaining < 4 or main["image"] == old_main["image"])
                and main["image"][1:3] == old_main["image"][1:3])
        temporary = [path for path in self.profile.iterdir()
                     if re.fullmatch(r"\.claude\.json\.tmp\.[1-9][0-9]{0,9}\.[0-9a-f]{12}", path.name)]
        self.require_temporary_bound(temporary, "finish")
        original_temporary = {row["path"]: row for row in old_temporary}
        for row in old_temporary:
            require(row["path"] in temporary and self.passive_image(row["path"], row["fd"]) == row["image"])
        following = [main]
        for path in temporary:
            pending, held = self.held_passive_document(path)
            original = original_temporary.get(path)
            require(original is None or held["image"] == original["image"])
            self.validate_usage_deletion(pending)
            selected = set(pending["pluginUsage"]).intersection(self.usage_contract["rows"])
            require(bool(selected) and (original is not None or len(selected) < 4)
                    and held["image"][1:3] == main["image"][1:3])
            following.append(held)
        # Reopening cannot refresh old custody, including a replacement after
        # the reopened read. Close this window through the original held FDs.
        for row in old_temporary:
            require(self.passive_image(row["path"], row["fd"]) == row["image"])
        # After the one receipt-authorized transition, every future checkpoint
        # again requires exact post-removal inode, metadata and byte equality.
        self.passive_usage = following

    def passive_usage_paths(self):
        return {".claude/" + row["path"].relative_to(self.profile).as_posix() for row in self.passive_usage}

    def passive_usage_proof(self):
        proof = {"native_version": "2.1.263", "receipt_bound_key_count": 4 if self.passive_usage else 0,
                "preserved_foreign_document": True,
                "foreign_document_equality": "strict-decoded-json",
                "post_removal_bytes_and_identity_held": True,
                "files": [{"name": row["path"].name, "sha256": row["image"][-1]} for row in self.passive_usage]}
        if self.user_usage is not None:
            proof["user_initialize_transition"] = {
                "authority": "same signed public/recorded User binding and committed receipt checks",
                "phase": self.user_usage["phase"], "receipt_bound_user_key_count": 4,
                "original_project_usage_contract_preserved": True,
                "main_image_authority": "explicit bounded User initialize, not original Project bytes",
                "exact_four_user_remove_receipts_qualified": self.user_usage["phase"] in ("removed", "finished"),
                "detached_original_images_held": len(self.detached_usage),
                "user_owners": {alias: {"installation_id_sha256": _diagnostic_digest(owner["installation_id"]),
                    "binding_id_sha256": _diagnostic_digest(owner["binding_id"])}
                    for alias, owner in self.user_usage["owners"].items()},
                "full_native_removal_qualified": False, "writer_closure_qualified": False}
        if self.retained_backups:
            proof["user_initialize_backups"] = {
                "authority": "one bounded initialize copy of a continuously held authorized main image",
                "full_bytes_equal_to_authorized_image": True,
                "post_action_namespace_and_original_descriptor_held": True,
                "files": [{"name": row["path"].name, "sha256": row["image"][-1]}
                          for row in self.retained_backups],
                "writer_closure_qualified": False, "physical_absence_qualified": False}
        return proof

    def user_initialize_authority(self, data, public_rows):
        """Bind this one User cohort through the existing signed receipt checker."""
        self.checkpoint()
        require(self.user_usage is None and Path(data) == self.home.parent / "data"
                and type(public_rows) is list and len(public_rows) == 4)
        state_path = Path(data) / "state-v2.json"
        state = self.document(state_path)
        require(type(state.get("schema_version")) is int and state["schema_version"] == 5)
        records = [row for row in state["installations"] if row.get("clients")]
        require(len(records) == 4)
        owners = {}
        for alias, distribution in self.audit["MARKETING_KIT_DISTRIBUTIONS"].items():
            selected = [row for row in public_rows if row.get("directory", {}).get("recorded_distribution") == distribution]
            require(len(selected) == 1)
            public = selected[0]
            selected = [row for row in records if row.get("installation_id") == public.get("installation_id")]
            require(len(selected) == 1)
            pin = self.audit["MARKETING_KIT_SIGNED_CLOSURE"][distribution]
            binding, _, active = self.selected_binding(alias, distribution, pin, public, selected[0], "user")
            owners[alias] = {"id": alias + "@skills-dir", "version": pin["package_version"],
                "scope": "user", "enabled": True, "installPath": str(active),
                "installation_id": public["installation_id"], "binding_id": binding["client_binding_id"]}
        for field in ("installation_id", "binding_id", "installPath"):
            require(len({row[field] for row in owners.values()}) == 4)
        return {"owners": owners, "state_path": state_path, "state": state,
                "phase": "before", "pending": None}

    def backup_inventory(self, original=None):
        """Reuse the rooted member walk and bounded tree for this one namespace."""
        path = self.profile / "backups"
        try:
            before = path.lstat()
        except FileNotFoundError:
            before = None
        entered = False
        case_fd = _CUSTODY["open_directory"](self.case)
        try:
            try:
                with self.audit["WINDSURF_GLOBAL_AUDIT"]["member"](
                        case_fd, path.relative_to(self.case).as_posix()) as fd:
                    _CUSTODY["named_directory"](self.case, case_fd)
                    entered = True
                    info = os.fstat(fd)
                    require(before is not None and os.path.samestat(before, info)
                            and stat.S_ISDIR(info.st_mode) and not info.st_mode & 0o022
                            and info.st_uid == os.geteuid()
                            and info.st_gid == self.passive_usage[0]["image"][2])
                    held_fd = original["fd"] if original is not None else None
                    if held_fd is None:
                        held_fd = os.dup(fd)
                        self.fds.append(held_fd)
                    require(os.path.samestat(info, os.fstat(held_fd)))
                    tree, _, _ = _CUSTODY["tree"](held_fd)
                    _CUSTODY["named_directory"](path, held_fd)
                    _CUSTODY["named_directory"](self.case, case_fd)
                    return {"path": path, "fd": held_fd, "tree": tree}
            except FileNotFoundError:
                require(not entered and before is None
                        and (original is None or original["fd"] is None))
                self.guard(check_usage=False)
                return {"path": path, "fd": None, "tree": {}}
        finally:
            os.close(case_fd)

    def backup_guard(self, namespace=None, following=()):
        namespace = self.backup_namespace if namespace is None else namespace
        if namespace is None:
            return
        require(self.backup_inventory(namespace)["tree"] == namespace["tree"])
        for row in (*self.retained_backups, *following):
            require(self.passive_image(row["path"], row["fd"]) == row["image"])

    def stage_user_backups(self, before, start, end, old, following):
        """Only one new exact copy of an already held main image may be pending."""
        after = self.backup_inventory(before)
        prior, current = before["tree"], after["tree"]
        require(all(current.get(key) == value for key, value in prior.items()))
        added = set(current) - set(prior)
        if not prior and current:
            require(added and current[""][0] == "dir")
            added.remove("")
        require(len(self.retained_backups) + len(added) <= 1
                and (bool(prior) or not current or bool(added)))
        candidates = []
        references = [(os.fstat(row["fd"]).st_size, row["image"][-1])
                      for row in (old, following)]
        for name in added:
            matched = re.fullmatch(r"\.claude\.json\.backup\.([1-9][0-9]{12})", name)
            require(matched is not None and start <= int(matched[1]) <= end
                    and current[name][0] == "file")
            _, held = self.held_passive_document(after["path"] / name)
            require(held["image"][1:3] == old["image"][1:3]
                    and (os.fstat(held["fd"]).st_size, held["image"][-1]) in references)
            candidates.append(held)
        self.backup_guard(after, candidates)
        return {"namespace": after, "files": candidates}

    def user_initialize_guards(self):
        """Keep every original authority except the explicitly pending main name."""
        self.recheck(check_usage=False)
        temporary = {path for path in self.profile.iterdir()
                     if re.fullmatch(r"\.claude\.json\.tmp\.[1-9][0-9]{0,9}\.[0-9a-f]{12}", path.name)}
        require(temporary == {row["path"] for row in self.passive_usage[1:]})
        for row in self.passive_usage[1:]:
            require(self.passive_image(row["path"], row["fd"]) == row["image"])
        for row in self.detached_usage:
            image = _diagnostic_image(row["fd"], self.audit)
            require((image["identity"], image["uid"], image["gid"], image["sha256"]) == row["image"])

    def stage_user_initialize(self, authority, action, native_rows, *, removed=False, observer=None):
        """Hold a candidate; complete native and caller guards must precede commit."""
        self.checkpoint()
        self.user_initialize_guards()
        require(type(removed) is bool and authority["pending"] is None
                and authority["phase"] == ("removed" if removed else "before")
                and type(native_rows) is list and (observer is None or callable(observer)))
        owners = authority["owners"]
        selected = [row for row in native_rows if row["id"].split("@", 1)[0].casefold() in owners]
        if removed:
            require(not selected and self.user_usage is authority)
        else:
            require(len(selected) == 4 and {row["id"] for row in selected} == {row["id"] for row in owners.values()}
                    and _CUSTODY["same_json"](self.document(authority["state_path"]), authority["state"]))
            for row in selected:
                owner = owners[row["id"].split("@", 1)[0]]
                require(all(type(row.get(key)) is type(owner[key]) and row[key] == owner[key]
                            for key in ("id", "version", "scope", "enabled", "installPath")))
        if not self.passive_usage:
            raise ValueError("Claude User initialization requires a held passive .claude.json baseline")
        if self.backup_namespace is None:
            self.backup_namespace = self.backup_inventory()
        backup_before = self.backup_namespace
        old = self.passive_usage[0]
        before = _diagnostic_image(old["fd"], self.audit)
        require((before["identity"], before["uid"], before["gid"], before["sha256"]) == old["image"])
        start, end = None, None
        def capture():
            nonlocal start, end
            require(start is None)
            start = time.time_ns() // 1_000_000
            try:
                return action()
            finally:
                end = time.time_ns() // 1_000_000
        result = capture() if observer is None else observer(capture)
        require(type(start) is int and type(end) is int and start <= end
                and type(result) is tuple and len(result) == 2
                and type(result[1]) is dict and result[1].get("exit") == 0
                and result[1].get("error") is None and result[1].get("stderr_bytes") == 0)
        document, following = self.held_passive_document(old["path"])
        require(following["image"][1:3] == old["image"][1:3])
        if removed:
            require(_CUSTODY["same_json"](document, before["document"]))
        else:
            keys = {row["id"] for row in owners.values()}
            usage = document.get("pluginUsage")
            prior = before["document"].get("pluginUsage")
            require(type(usage) is dict and type(prior) is dict and not keys.intersection(prior)
                    and keys <= set(usage))
            for key in keys:
                row = usage[key]
                require(type(row) is dict and set(row) == {"usageCount", "lastUsedAt", "lastUsedNumStartups"}
                        and type(row["usageCount"]) is int and row["usageCount"] == 0
                        and type(row["lastUsedNumStartups"]) is int and row["lastUsedNumStartups"] == 0
                        and type(row["lastUsedAt"]) is int and start <= row["lastUsedAt"] <= end)
            require(_CUSTODY["same_json"](dict(document, pluginUsage={
                key: row for key, row in usage.items() if key not in keys}), before["document"]))
        held = _diagnostic_image(old["fd"], self.audit)
        same = following["image"][0][:2] == old["image"][0][:2]
        require((removed or not same) and held["sha256"] == old["image"][-1]
                and held["identity"][:3] == old["image"][0][:3]
                and held["identity"][4:6] == old["image"][0][4:6]
                and (held["uid"], held["gid"]) == old["image"][1:3]
                and held["identity"][3] == (1 if same else 0)
                and (not same or following["image"] == old["image"]))
        backups = self.stage_user_backups(backup_before, start, end, old, following)
        authority["pending"] = {"old": old, "following": following, "backups": backups,
            "detached_image": (held["identity"], held["uid"], held["gid"], held["sha256"]),
            "same_inode": same, "removed": removed, "native_rows": native_rows}
        # Do not replace passive_usage here: before/after_remove still has to
        # validate the complete native Skill response and all outer guards.
        return result

    def commit_user_initialize(self, authority, native_proof):
        pending = authority["pending"]
        require(type(pending) is dict and native_proof["scope"] == "user"
                and native_proof["home"] == str(self.home) and native_proof["project"] == str(self.project)
                and _CUSTODY["same_json"](native_proof["native_rows"], pending["native_rows"]))
        self.user_initialize_guards()
        old, following = pending["old"], pending["following"]
        require(self.passive_usage[0] is old
                and self.passive_image(following["path"], following["fd"]) == following["image"])
        held = _diagnostic_image(old["fd"], self.audit)
        require((held["identity"], held["uid"], held["gid"], held["sha256"]) == pending["detached_image"])
        self.backup_guard(pending["backups"]["namespace"], pending["backups"]["files"])
        if not pending["same_inode"]:
            self.detached_usage.append(dict(old, image=pending["detached_image"]))
            self.passive_usage = [following, *self.passive_usage[1:]]
        self.backup_namespace = pending["backups"]["namespace"]
        self.retained_backups.extend(pending["backups"]["files"])
        self.user_usage = authority
        authority["phase"] = "finished" if pending["removed"] else "initialized"
        authority["pending"] = None
        self.checkpoint()

    def user_remove_receipt(self, authority, cleanup):
        """Only the complete four-member User receipt may authorize the next read."""
        self.checkpoint()
        require(self.user_usage is authority and authority["phase"] == "initialized"
                and authority["pending"] is None and cleanup["result"] == "success"
                and cleanup["outcome"] == "completed" and cleanup["scope"] == "user"
                and type(cleanup["actions"]) is list and type(cleanup["results"]) is list
                and len(cleanup["actions"]) == len(cleanup["results"]) == 4)
        observed = set()
        for action, result in zip(cleanup["actions"], cleanup["results"]):
            alias = action["alias"]
            require(alias in authority["owners"] and alias not in observed)
            owner = authority["owners"][alias]
            require(action["action"] == result["operation"] == "remove"
                    and action["client_id"] == result["client_id"] == "claude"
                    and action["installation_id"] == result["installation_id"] == owner["installation_id"]
                    and action["binding_id"] == owner["binding_id"] and result["outcome"] == "completed")
            observed.add(alias)
        require(observed == set(authority["owners"]))
        authority["phase"] = "removed"
        self.checkpoint()

    def selected_binding(self, alias, distribution, pin, public, record, scope):
        """Share the same signed/public/recorded/path/receipt authority for both scopes."""
        require(scope in ("project", "user"))
        require(record["declared_name"] == record["package"]["declared_name"] == public["name"] == alias
                and record.get("origin_mode") == "directory"
                and type(public.get("installation_id")) is str and bool(public["installation_id"]))
        directory, source, package = (record[key] for key in ("directory", "source", "package"))
        require(directory["distribution_id"] == distribution
                and type(directory["desired_release_sequence"]) is int
                and directory["desired_release_sequence"] == pin["release_sequence"]
                and package["version"] == pin["package_version"]
                and package["manifest_digest"] == pin["manifest_digest"]
                and all(source[key] == pin[field] for key, field in (
                    ("repository", "repository"), ("package_subpath", "package_path"),
                    ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest"))))
        require(public["source"] == pin["repository"].strip("/") + "/" + pin["package_path"].strip("/")
                and public["directory"]["recorded_revision"] == pin["resolved_revision"]
                and type(public["directory"]["recorded_release_sequence"]) is int
                and public["directory"]["recorded_release_sequence"] == pin["release_sequence"])
        clients = public["clients"]
        if type(clients) is dict:
            clients = list(clients.values())
        require(type(clients) is list and len(clients) == 1
                and type(record["clients"]) is dict and len(record["clients"]) == 1)
        binding_id, binding = next(iter(record["clients"].items()))
        require(type(binding_id) is str and bool(binding_id) and binding["client_binding_id"] == binding_id)
        expected_authentication = expected_project_authentication(alias, binding)
        checkpoint = {"client_id": ("claude",), "scope": (scope,),
                # Installed but unready MCPs and untrusted/suppressed plugins
                # can remain prepared in the published adapter. Exact cache,
                # registry and receipt ownership below is still mandatory.
                "activation": ("active", "prepared"), "materialization": ("materialized",),
                "verification": ("installation_verified",),
                "authentication": (expected_authentication,), "policy": ("allowed",)}
        for key, allowed in checkpoint.items():
            if binding.get(key) != clients[0].get(key) or binding.get(key) not in allowed:
                if key == "authentication":
                    raise ValueError(f"Claude cache retirement {alias} binding field authentication "
                        f"differs from checkpoint contract: recorded={authentication_checkpoint_label(binding)}, "
                        f"public={authentication_checkpoint_label(clients[0])}; expected={expected_authentication}")
                raise ValueError(f"Claude cache retirement {alias} binding field {key} differs from checkpoint contract")
        readiness = {key: binding[key] for key in checkpoint}
        for revision in (binding["package_revision"], clients[0]["package_revision"]):
            require(type(revision["release_sequence"]) is int
                    and revision["distribution_id"] == distribution
                    and all(revision[key] == pin[field] for key, field in (
                        ("version", "package_version"), ("release_sequence", "release_sequence"),
                        ("resolved_revision", "resolved_revision"), ("tree_digest", "tree_digest"),
                        ("manifest_digest", "manifest_digest"))))
        physical = binding["physical_artifact_id"]
        require(type(physical) is str and physical == physical.strip() and bool(physical)
                and physical not in (".", "..") and Path(physical).name == physical)
        active = ((self.project / ".claude/skills") if scope == "project"
                  else self.profile / "skills") / physical
        require(binding["target_locator"] == str(active)
                and self.audit["path_inside_case"](active, self.case))
        receipts = binding["receipts"]
        require(type(receipts) is list and bool(receipts)
                and all(row["phase"] == "committed" and row["client_binding_id"] == binding_id
                        and row["active_path"] == str(active) and type(row["operation_id"]) is str
                        and bool(row["operation_id"]) for row in receipts)
                and len({row["operation_id"] for row in receipts}) == len(receipts))
        return binding, readiness, active

    def capture(self, alias, distribution, pin, public, record):
        binding, readiness, active = self.selected_binding(alias, distribution, pin, public, record, "project")
        binding_id, physical = binding["client_binding_id"], binding["physical_artifact_id"]
        objects = [row for row in binding["native_objects"] if row["kind"] == "claude_project_plugin_cache"]
        require(len(objects) == 1)
        receipt = objects[0]
        market = "agentplugins-" + hashlib.sha256((physical + "\0" + str(self.project)).encode()).hexdigest()[:12]
        key = alias + "@" + market
        relative = f"plugins/cache/{market}/{alias}/{pin['package_version']}"
        require(receipt["source_relative"] == relative and receipt["logical_name"] == alias
                and receipt["object_id"] == "claude-project-cache:" + key
                and receipt["path"] == str(active / ".agentplugins-plugin")
                and receipt["protection_class"] == "managed" and receipt.get("user_modified", False) is False
                and type(receipt["managed_digest"]) is str
                and re.fullmatch(r"sha256:[0-9a-f]{64}", receipt["managed_digest"]))
        cache = self.profile / relative
        parent = cache.parent
        for path in (self.case, self.home, self.project, self.profile, *list(parent.parents)[:3], parent):
            info = _CUSTODY["ordinary"](path)
            identity = (info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid)
            require(path not in self.anchors or self.anchors[path] == identity)
            self.anchors[path] = identity
        parent_fd = _CUSTODY["open_directory"](parent); self.fds.append(parent_fd)
        cache_fd = _CUSTODY["open_directory"](cache); self.fds.append(cache_fd)
        before, files, size = _CUSTODY["tree"](cache_fd)
        require(files > 0 and ".orphaned_at" not in before)
        bundle_fd = _CUSTODY["open_directory"](active / ".agentplugins-plugin")
        try:
            bundle, _, _ = _CUSTODY["tree"](bundle_fd)
            require(_CUSTODY["content"](bundle) == _CUSTODY["content"](before))
        finally:
            os.close(bundle_fd)
        return {"alias": alias, "distribution": distribution, "installation_id": public["installation_id"],
                "binding_id": binding_id, "physical": physical, "active": active, "market": market,
                "key": key, "cache": cache, "parent": parent, "parent_fd": parent_fd, "cache_fd": cache_fd,
                "before": before, "siblings": _CUSTODY["entries"](parent_fd), "files": files, "bytes": size,
                "signed_release": dict(pin), "binding_at_capture": readiness,
                "receipt": {key: receipt[key] for key in ("object_id", "source_relative", "managed_digest")}}

    def guard(self, *, check_usage=True):
        require(not self.finished)
        for path, identity in self.anchors.items():
            info = _CUSTODY["ordinary"](path)
            require((info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid) == identity)
        for owner in self.owners:
            _CUSTODY["named_directory"](owner["parent"], owner["parent_fd"])
            if "retired_parent_entries" in owner:
                require(_CUSTODY["entries"](owner["parent_fd"]) == owner["retired_parent_entries"])
        if check_usage:
            self.backup_guard()
            if self.user_usage is not None:
                require({path for path in self.profile.iterdir() if re.fullmatch(
                    r"\.claude\.json\.tmp\.[1-9][0-9]{0,9}\.[0-9a-f]{12}", path.name)}
                    == {row["path"] for row in self.passive_usage[1:]})
            for row in self.passive_usage:
                require(self.passive_image(row["path"], row["fd"]) == row["image"])
            for row in self.detached_usage:
                image = _diagnostic_image(row["fd"], self.audit)
                require((image["identity"], image["uid"], image["gid"], image["sha256"]) == row["image"]
                        and image["identity"][3] == 0)

    def close(self):
        while self.fds:
            os.close(self.fds.pop())
        self.finished = True

    def registries_removed(self):
        keys = {owner["key"] for owner in self.owners}
        markets = {owner["market"] for owner in self.owners}
        installed = self.document(self.profile / "plugins/installed_plugins.json")
        registered = self.document(self.profile / "plugins/known_marketplaces.json")
        require(_CUSTODY["same_json"](installed, dict(self.installed, plugins={
            key: value for key, value in self.installed["plugins"].items() if key not in keys})))
        require(_CUSTODY["same_json"](registered, {
            key: value for key, value in self.markets.items() if key not in markets}))

    def retired(self, owner):
        cache, parent_fd = owner["cache"], owner["parent_fd"]
        require(not cache.exists() and not cache.is_symlink()
                and not owner["active"].exists() and not owner["active"].is_symlink())
        after = _CUSTODY["entries"](parent_fd)
        new = set(after) - set(owner["siblings"])
        require(len(new) == 1)
        nonce = new.pop()
        require(re.fullmatch(r"\.agentplugins-cache-retire-[0-9a-f]{32}", nonce)
                and {k: v for k, v in owner["siblings"].items() if k != cache.name}
                == {k: v for k, v in after.items() if k != nonce})
        quarantine = owner["parent"] / nonce
        fd = _CUSTODY["open_directory"](quarantine); self.fds.append(fd)
        info = os.fstat(fd)
        require(stat.S_IMODE(info.st_mode) == 0o700 and info.st_uid == os.geteuid()
                and set(_CUSTODY["entries"](fd)) == {"retired"})
        owner.update(quarantine=quarantine, quarantine_fd=fd, quarantine_identity=(info.st_mode, info.st_uid),
                     moved=quarantine / "retired", retired_parent_entries=dict(after))
        _CUSTODY["named_directory"](owner["moved"], owner["cache_fd"])
        after, _, _ = _CUSTODY["tree"](owner["cache_fd"])
        marker = after.pop(".orphaned_at", None)
        if marker is not None:
            require(marker[0] == "file" and marker[5] == 13)
            fd = os.open(".orphaned_at", os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
                         dir_fd=owner["cache_fd"])
            try:
                held = os.fstat(fd)
                body = os.read(fd, 14)
                require(_CUSTODY["_FILES"]["identity"](held)
                        == _CUSTODY["_FILES"]["identity"](os.fstat(fd))
                        == _CUSTODY["_FILES"]["identity"](os.stat(
                            ".orphaned_at", dir_fd=owner["cache_fd"], follow_symlinks=False))
                        and re.fullmatch(rb"[1-9][0-9]{12}", body)
                        and hashlib.sha256(body).hexdigest() == marker[-1])
            finally:
                os.close(fd)
        require(after == owner["before"])
        owner["marker"] = marker
        allowed = {".claude/" + owner["parent"].relative_to(self.profile).as_posix() + ": directory"}
        prefix = ".claude/" + owner["moved"].relative_to(self.profile).as_posix()
        for path, row in dict(after, **({".orphaned_at": marker} if marker else {})).items():
            relative = prefix + ("/" + path if path else "")
            if row[0] == "file":
                allowed.add(relative)
                if row[5] > 1 << 20:
                    allowed.add(relative + ": unscanned-too-large")
            else:
                allowed.add(relative + ": directory")
        return allowed

    def recheck(self, *, check_usage=True):
        require(type(check_usage) is bool)
        self.guard(check_usage=check_usage)
        for owner in self.owners:
            _CUSTODY["named_directory"](owner["quarantine"], owner["quarantine_fd"])
            _CUSTODY["named_directory"](owner["moved"], owner["cache_fd"])
            current, _, _ = _CUSTODY["tree"](owner["cache_fd"])
            require(current.pop(".orphaned_at", None) == owner["marker"] and current == owner["before"])
            info = os.fstat(owner["quarantine_fd"])
            require((info.st_mode, info.st_uid) == owner["quarantine_identity"]
                    and set(_CUSTODY["entries"](owner["quarantine_fd"])) == {"retired"}
                    and not owner["cache"].exists() and not owner["cache"].is_symlink()
                    and not owner["active"].exists() and not owner["active"].is_symlink())
        self.registries_removed()
        self.guard(check_usage=check_usage)

    def checkpoint(self):
        """Recheck the same continuously held, successfully retired authority."""
        try:
            require(self.retired_allowed is not None and not self.finished)
            self.recheck()
        except BaseException:
            self.close()
            raise

    def verify_retained(self, home, case_root, residuals, rescan):
        """After User removal, permit only the earlier Project's held bytes."""
        try:
            require(Path(home) == self.home and Path(case_root) == self.case)
            self.checkpoint()
            require(callable(rescan) and type(residuals) in (set, list, tuple) and bool(residuals))
            allowed = self.retired_allowed | {".claude/" + row["path"].relative_to(self.profile).as_posix()
                                              for row in self.retained_backups}
            require_owned_residuals(self.audit, residuals, allowed)
            for _ in range(2):
                require(set(rescan()) == set(residuals))
                self.checkpoint()
            return {"status": "verified-unchanged-signed-project-cache-retirement-at-user-removal",
                    "scope": "project", "writer_closure_claimed": False,
                    "retired_bytes_present": True, "physical_absence_qualified": False,
                    "writer_closure_qualified": False, "full_native_removal_qualified": False,
                    "managed_digest_independently_recomputed_by_harness": False,
                    "managed_digest_validation_authority": "genuine-published-cli-remove",
                    "custody_validation_authority": "held-descriptor-image-and-namespace-equality",
                    "raw_native_residuals": sorted(set(residuals)),
                    "passive_plugin_usage": self.passive_usage_proof(),
                    "matched_residual_count": len(set(residuals))}
        except BaseException:
            self.close()
            raise

    def finish(self, cleanup, residuals, rescan, *, keep_open=False):
        """Validate raw scanner findings without deleting or hiding retained bytes."""
        validated = False
        try:
            require(type(keep_open) is bool and self.retired_allowed is None)
            self.guard(check_usage=False)
            require(callable(rescan) and type(residuals) in (set, list, tuple) and bool(residuals)
                    and cleanup["result"] == "success" and cleanup["outcome"] == "completed"
                    and cleanup["scope"] == "project")
            actions, results = cleanup["actions"], cleanup["results"]
            require(type(actions) is list and type(results) is list and len(actions) == len(results) == 4)
            observed = set()
            for action, result in zip(actions, results):
                owner = next((v for v in self.owners if v["alias"] == action.get("alias")), None)
                require(owner is not None and owner["alias"] not in observed
                        and action["action"] == result["operation"] == "remove"
                        and action["client_id"] == result["client_id"] == "claude"
                        and action["installation_id"] == result["installation_id"] == owner["installation_id"]
                        and action["binding_id"] == owner["binding_id"]
                        and result["outcome"] == "completed")
                observed.add(owner["alias"])
            allowed = set().union(*(self.retired(owner) for owner in self.owners))
            self.registries_removed()
            self.finish_passive_usage()
            allowed.update(self.passive_usage_paths())
            require_owned_residuals(self.audit, residuals, allowed)
            require(set(rescan()) == set(residuals))
            self.recheck()
            require(set(rescan()) == set(residuals))
            self.recheck()
            proof = {"status": "verified-signed-closure-cache-retirement-at-checkpoint",
                    "scope": "project", "writer_closure_claimed": False,
                    "retired_bytes_present": True, "physical_absence_qualified": False,
                    "writer_closure_qualified": False, "full_native_removal_qualified": False,
                    "managed_digest_independently_recomputed_by_harness": False,
                    "managed_digest_validation_authority": "genuine-published-cli-remove",
                    "custody_validation_authority": "held-descriptor-image-and-namespace-equality",
                    "raw_native_residuals": sorted(set(residuals)),
                    "passive_plugin_usage": self.passive_usage_proof(),
                    "matched_residual_count": len(set(residuals)),
                    "members": [{"distribution": owner["distribution"], "receipt": owner["receipt"],
                        "binding_at_capture": owner["binding_at_capture"],
                        "signed_release": owner["signed_release"],
                        "file_count": owner["files"], "bytes": owner["bytes"],
                        "original": owner["cache"].relative_to(self.case).as_posix(),
                        "retired": owner["moved"].relative_to(self.case).as_posix(),
                        "snapshot_equality_sha256": hashlib.sha256(json.dumps(
                            owner["before"], sort_keys=True).encode()).hexdigest(),
                        "orphaned_marker_present": owner["marker"] is not None} for owner in self.owners]}
            # Initialize retained authority only after the complete original
            # proof, never from a report or a later namespace rediscovery.
            self.retired_allowed = frozenset(allowed)
            validated = True
            return proof
        finally:
            if not (keep_open and validated):
                self.close()
