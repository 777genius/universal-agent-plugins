"""Receipt-bound signed Project cache retirement; never native writer closure."""
import hashlib
import json
import os
from pathlib import Path
import re
from runpy import run_path
import stat
import sys

_CUSTODY = run_path(str(Path(__file__).with_name("claude_cache_retirement_audit.py")))
require = _CUSTODY["require"]

# Exact published 0.1.115 producer contract for this immutable signed closure:
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


class SignedClosureRetirement:
    """Hold all four owned caches continuously across one Project removal."""

    def __init__(self, case_root, home, project, public_rows, pins, audit, checkpoint_diagnostics=None):
        self.fds, self.owners, self.anchors = [], [], {}
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
            self.guard()
        except BaseException:
            self.close()
            raise

    def document(self, path):
        return _checkpoint_document(self.audit, self.case, path)

    def capture(self, alias, distribution, pin, public, record):
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
        checkpoint = {"client_id": ("claude",), "scope": ("project",),
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
        active = self.project / ".claude/skills" / physical
        require(binding["target_locator"] == str(active)
                and self.audit["path_inside_case"](active, self.case))
        receipts = binding["receipts"]
        require(type(receipts) is list and bool(receipts)
                and all(row["phase"] == "committed" and row["client_binding_id"] == binding_id
                        and row["active_path"] == str(active) and type(row["operation_id"]) is str
                        and bool(row["operation_id"]) for row in receipts)
                and len({row["operation_id"] for row in receipts}) == len(receipts))
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

    def guard(self):
        require(not self.finished)
        for path, identity in self.anchors.items():
            info = _CUSTODY["ordinary"](path)
            require((info.st_mode, info.st_dev, info.st_ino, info.st_uid, info.st_gid) == identity)
        for owner in self.owners:
            _CUSTODY["named_directory"](owner["parent"], owner["parent_fd"])

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
                     moved=quarantine / "retired")
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

    def recheck(self):
        self.guard()
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
        self.guard()

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
            require(callable(rescan) and type(residuals) in (set, list, tuple) and bool(residuals)
                    and set(residuals) <= self.retired_allowed)
            for _ in range(2):
                require(set(rescan()) == set(residuals))
                self.checkpoint()
            return {"status": "verified-unchanged-signed-project-cache-retirement-at-user-removal",
                    "scope": "project", "writer_closure_claimed": False,
                    "matched_residual_count": len(set(residuals))}
        except BaseException:
            self.close()
            raise

    def finish(self, cleanup, residuals, rescan, *, keep_open=False):
        """Validate raw scanner findings without deleting or hiding retained bytes."""
        validated = False
        try:
            require(type(keep_open) is bool and self.retired_allowed is None)
            self.guard()
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
            require(set(residuals) <= allowed)
            self.registries_removed()
            require(set(rescan()) == set(residuals))
            self.recheck()
            require(set(rescan()) == set(residuals))
            self.recheck()
            proof = {"status": "verified-signed-closure-cache-retirement-at-checkpoint",
                    "scope": "project", "writer_closure_claimed": False,
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
