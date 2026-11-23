"""Observe public Copilot consent boundaries without authoring native trust."""

import hashlib
from pathlib import Path
import stat


TRUST_REFUSAL = ("copilot has not remembered the selected project folder; "
                 "rerun installation to restore folder trust")


class ProjectTrustObservation:
    """Pin a fresh disposable profile; only the real explicit add may grant trust."""

    def __init__(self, audit, environment, project):
        self.audit, self.project = audit, project
        self.case, self.home = project.parent, Path(environment["HOME"])
        if (self.home != self.case / "home"
                or environment.get("AGENTPLUGINS_HOME") != str(self.case / "data")):
            raise AssertionError("Copilot consent observation requires the selected disposable profile")
        self.native_guard = audit["copilot_native_roots_guard"](self.home, environment, project)
        self.paths = tuple(self.home / relative / "config.json" for relative in (".copilot", ".config/copilot"))
        self.anchors = {path: self.directory(path) for path in
                        (self.home / ".config", *(path.parent for path in self.paths))}
        self.initial = self.snapshot()
        if any(str(project) in self.document(value).get("trustedFolders", [])
               for value in self.initial.values()):
            raise AssertionError("cold Copilot Project already has remembered trust")
        self.expected = self.initial

    def directory(self, path):
        info = path.lstat()
        if (not stat.S_ISDIR(info.st_mode)
                or not self.audit["path_inside_case"](path, self.case)):
            raise AssertionError("Copilot trust profile directory is unsafe")
        return info.st_mode, info.st_dev, info.st_ino

    def guard(self):
        self.native_guard()
        if any(self.directory(path) != original for path, original in self.anchors.items()):
            raise AssertionError("Copilot trust profile directory changed")

    def document(self, value):
        if value is None:
            return {}
        document = self.audit["copilot_json_document"](value[0])
        if (type(document) is not dict
                or type(document.get("trustedFolders", [])) is not list
                or any(type(path) is not str for path in document.get("trustedFolders", []))):
            raise AssertionError("Copilot trust document is malformed")
        return document

    def snapshot(self):
        self.guard()
        result = {}
        for path in self.paths:
            body, error = self.audit["copilot_settings_read"](path, self.case)
            if error == "absent":
                if path.exists() or path.is_symlink():
                    raise AssertionError("Copilot trust file disappeared during observation")
                result[path] = None
            elif error is None and type(body) is str:
                info = path.lstat()
                if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                    raise AssertionError("Copilot trust file is unsafe")
                result[path] = (body, info.st_mode, info.st_dev, info.st_ino,
                                info.st_size, info.st_mtime_ns)
            else:
                raise AssertionError("Copilot trust file cannot be read safely")
            self.document(result[path])
        self.guard()
        return result

    def unchanged(self):
        if self.snapshot() != self.expected:
            raise AssertionError("Copilot Project trust changed outside explicit owner add")

    def owner_add_completed(self):
        """Observe one exact project grant produced by the successful public add."""
        after = self.snapshot()
        primary, alternate = self.paths
        before = self.document(self.initial[primary])
        expected = dict(before, trustedFolders=[*before.get("trustedFolders", []), str(self.project)])
        if self.document(after[primary]) != expected or after[alternate] != self.initial[alternate]:
            raise AssertionError("explicit Copilot add did not retain exactly the selected project trust")
        self.expected = after
        self.unchanged()
        return {"status": "passed", "trust_source": "successful_public_explicit_add",
                "fixture_authored_native_trust": False, "copied_native_trust": False,
                "selected_project_only": True}


def verify_cold_refusal(result, project, audit):
    """Require the exact public116 four-binding refusal, including immutable pins."""
    if (type(result.get("exit_code")) is not int or result["exit_code"] != 1
            or result.get("timeout") is not False or type(result.get("stdout_full")) is not str
            or len(result["stdout_full"].encode()) > 1 << 20):
        raise AssertionError("cold Copilot restore did not return a bounded native trust refusal")
    envelope = audit["copilot_json_document"](result["stdout_full"])
    selection = {"scope": "project", "scope_selection_reason": "explicit",
                 "project_root": str(project), "manifest_path": str(project / "uap.toml"),
                 "lock_path": str(project / "uap.lock")}
    if (type(envelope) is not dict or type(envelope.get("schema_version")) is not int
            or envelope["schema_version"] != 1 or envelope.get("command") != "agentplugins install"
            or envelope.get("scope") != "project" or envelope.get("scope_selection_reason") != "explicit"
            or envelope.get("selection") != selection
            or envelope.get("result") != "failure" or envelope.get("outcome") != "partial"):
        raise AssertionError("cold Copilot refusal has the wrong command, scope or outcome")
    aliases = audit["MARKETING_KIT_DISTRIBUTIONS"]
    actions, results, bindings = (envelope.get(key) for key in ("actions", "results", "bindings"))
    if (any(type(rows) is not list or len(rows) != 4 for rows in (actions, results, bindings))
            or any(type(row) is not dict for rows in (actions, results, bindings) for row in rows)
            or {row.get("alias") for row in actions} != set(aliases)
            or {row.get("alias") for row in bindings} != set(aliases)):
        raise AssertionError("cold Copilot refusal lacks the complete signed closure")
    ids = set()
    for action, receipt in zip(actions, results):
        pin = audit["MARKETING_KIT_SIGNED_CLOSURE"][aliases[action["alias"]]]
        if (action.get("client_id") != "copilot" or action.get("action") != "install"
                or any(action.get(key) != pin[field] for key, field in (
                    ("to_version", "package_version"), ("package_digest", "tree_digest"),
                    ("revision", "resolved_revision")))
                or receipt.get("client_id") != "copilot" or receipt.get("operation") != "install"
                or receipt.get("outcome") != "incomplete" or receipt.get("reason") != TRUST_REFUSAL
                or receipt.get("manual_actions") != ["retry client activation and verify client visibility"]
                or receipt.get("next_actions") != [{"kind": "activate", "reason": TRUST_REFUSAL}]
                or type(receipt.get("installation_id")) is not str
                or not receipt["installation_id"].strip() or receipt["installation_id"] in ids):
            raise AssertionError("cold Copilot refusal has an unrelated binding or source result")
        ids.add(receipt["installation_id"])
    if any(row != {"alias": row["alias"], "client_id": "copilot", "status": "incomplete", "code": "FAILED"}
           for row in bindings):
        raise AssertionError("cold Copilot refusal binding diagnostics differ")
    stderr = ("agentplugins: dependency sync stopped; inspect binding results and next actions: "
              "4 of 4 bindings failed: " + TRUST_REFUSAL + "\n")
    if result.get("stderr_full") != stderr:
        raise AssertionError("cold Copilot restore has an unrelated failure diagnostic")
    return {"status": "refused_as_expected", "exit_code": 1, "refused_bindings": 4,
            "reason": TRUST_REFUSAL,
            "materialization_absence_claimed": False,
            "receipt_sha256": hashlib.sha256(result["stdout_full"].encode()).hexdigest()}


def verified_unchanged_locked_result(result, project, expected, audit):
    """A post-consent locked no-op must retain every exact previously installed ID."""
    text = result.get("stdout_full")
    if type(text) is not str or len(text.encode()) > 1 << 20:
        raise AssertionError("post-consent Copilot locked restore has no bounded whole JSON")
    envelope = audit["copilot_json_document"](text)
    if (type(result.get("exit_code")) is not int or result["exit_code"] != 0
            or result.get("timeout") is not False or type(envelope) is not dict
            or type(envelope.get("schema_version")) is not int or envelope["schema_version"] != 1
            or envelope.get("command") != "agentplugins install"
            or envelope.get("scope") != "project" or envelope.get("scope_selection_reason") != "explicit"
            or envelope.get("result") != "success" or envelope.get("outcome") != "completed"
            or envelope.get("results") != []):
        raise AssertionError("post-consent Copilot locked restore is not a complete unchanged result")
    aliases = audit["MARKETING_KIT_DISTRIBUTIONS"]
    actions = envelope.get("actions")
    if (type(actions) is not list or len(actions) != 4 or any(type(row) is not dict for row in actions)
            or {row.get("alias") for row in actions} != set(aliases)
            or set(expected) != set(aliases.values())
            or any(type(value) is not str or not value.strip() for value in expected.values())
            or len(set(expected.values())) != 4):
        raise AssertionError("post-consent Copilot locked restore lost its installed closure")
    for row in actions:
        pin = audit["MARKETING_KIT_SIGNED_CLOSURE"][aliases[row["alias"]]]
        if (row.get("client_id") != "copilot" or row.get("action") != "unchanged"
                or row.get("installation_id") != expected[aliases[row["alias"]]]
                or any(row.get(key) != pin[field] for key, field in (
                    ("to_version", "package_version"), ("package_digest", "tree_digest"),
                    ("revision", "resolved_revision")))):
            raise AssertionError("post-consent Copilot locked restore changed a source or installation")
    selection = envelope.get("selection")
    if selection != {"scope": "project", "scope_selection_reason": "explicit",
                     "project_root": str(project), "manifest_path": str(project / "uap.toml"),
                     "lock_path": str(project / "uap.lock")}:
        raise AssertionError("post-consent Copilot locked restore selected another project")
    return envelope
