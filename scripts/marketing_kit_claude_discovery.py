"""Fresh native Claude listing joined to continuously held signed-cache owners."""

import hashlib
import json
from pathlib import Path
import re


class ClaudeProjectDiscovery:
    """Registration/absence evidence only; never model, MCP or writer closure."""

    def __init__(self, retirement, environment, invoke):
        self.retirement, self.environment, self.invoke = retirement, environment, invoke
        self.audit, self.home, self.project = retirement.audit, retirement.home, retirement.project
        self.binary = self.audit["AGENT_BINARIES"].get("claude")
        if (not self.binary or not Path(self.binary).is_absolute() or not Path(self.binary).is_file()
                or environment.get("HOME") != str(self.home)
                or environment.get("CLAUDE_CONFIG_DIR") != str(self.home / ".claude")):
            raise AssertionError("Claude native discovery requires the selected pinned disposable profile")
        retirement.guard()
        result = self.invoke([self.binary, "--version"], environment, self.project, 30, capture_full=True)
        self.output(result)
        if re.fullmatch(r"2\.1\.263 \(Claude Code\)\r?\n?", result["stdout_full"]) is None:
            raise AssertionError("Claude native discovery version differs from its fixed package pin")
        retirement.guard()
        before, digest = self.listing()
        self.before_digest = digest
        self.owned_ids = {owner["key"] for owner in retirement.owners}
        if len(self.owned_ids) != 4 or not self.owned_ids <= set(before):
            raise AssertionError("fresh Claude list lacks the four receipt-bound Project registrations")
        for owner in retirement.owners:
            row = before[owner["key"]]
            if (row.get("scope") != "project" or type(row.get("enabled")) is not bool
                    or row.get("version") != owner["signed_release"]["package_version"]
                    or row.get("installPath") != str(owner["cache"])
                    or row.get("projectPath", str(self.project)) != str(self.project)
                    or row.get("errors", []) != []):
                raise AssertionError("fresh Claude registration differs from the held signed owner")
        self.foreign_rows = {key: row for key, row in before.items() if key not in self.owned_ids}
        retirement.guard()

    @staticmethod
    def output(result):
        if (type(result.get("exit_code")) is not int or result["exit_code"] != 0
                or result.get("timeout") is not False
                or any(type(result.get(key)) is not str or len(result[key].encode()) > 1 << 20
                       for key in ("stdout_full", "stderr_full"))
                or result["stderr_full"].strip()):
            raise AssertionError("fresh Claude native output failed or exceeded its bounded capture")

    def listing(self):
        self.retirement.guard()
        result = self.invoke([self.binary, "plugin", "list", "--json"],
                             self.environment, self.project, 40, capture_full=True)
        self.retirement.guard()
        self.output(result)
        rows = self.audit["copilot_json_document"](result["stdout_full"])
        if type(rows) is not list or any(type(row) is not dict or type(row.get("id")) is not str
                                       or not row["id"].strip() for row in rows):
            raise AssertionError("fresh Claude native listing is not a complete typed JSON array")
        by_id = {row["id"]: row for row in rows}
        if len(by_id) != len(rows):
            raise AssertionError("fresh Claude native listing contains duplicate registration identities")
        return by_id, hashlib.sha256(result["stdout_full"].encode()).hexdigest()

    def removed(self):
        # The raw quarantine snapshot and native registries must still match
        # before and after this *new* native process, including later User A.
        self.retirement.checkpoint()
        try:
            after, digest = self.listing()
            if after != self.foreign_rows or self.owned_ids & set(after):
                raise AssertionError("fresh Claude native list retained an owned registration or changed a foreign row")
            return {"status": "fresh_native_registration_absence",
                    "native_version": "2.1.263", "registered_before": 4,
                    "owned_registrations_after": 0, "foreign_rows_unchanged": True,
                    "before_stdout_sha256": self.before_digest, "after_stdout_sha256": digest,
                    "native_activation_claimed": False, "mcp_execution_claimed": False,
                    "authentication_claimed": False, "writer_closure_claimed": False}
        finally:
            self.retirement.checkpoint()
