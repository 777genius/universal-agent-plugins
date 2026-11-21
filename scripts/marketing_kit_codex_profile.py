"""Reuse the pinned Codex bootstrap and SQLite checks in the signed-kit lane."""

from pathlib import Path
import stat


class CodexProfile:
    def __init__(self, audit, root, environment, project):
        self.audit = audit
        self.case, self.home = project.parent, Path(environment["HOME"])
        data = Path(environment["AGENTPLUGINS_HOME"])
        binary = audit["AGENT_BINARIES"].get("codex")
        if (not binary or self.home != self.case / "home" or data != self.case / "data"
                or environment.get("CODEX_HOME") != str(self.home / ".codex")):
            raise ValueError("Marketing Codex profile is not the selected disposable context")
        paths = (root, self.case, self.home, data, project,
                 *(self.home / relative for relative in audit["NATIVE_ROOTS"]["codex"]))
        self.anchors = {}
        for path in paths:
            if not audit["path_inside_case"](path, root):
                raise ValueError("Marketing Codex authority escaped the disposable case")
            self.anchors[path] = self._directory(path)
        self.failed_phase = None
        self.final_verified = False
        self.runtime = None
        self.system_metadata = None
        self.runtime, self.system_metadata = audit["initialize_codex_plugin_profile"](
            binary, self.case, self.home, project, data, environment, self.guard)
        self.checkpoint("profile-initialized")

    @staticmethod
    def _directory(path):
        info = path.lstat()
        if (not stat.S_ISDIR(info.st_mode) or path.is_symlink()
                or path.resolve(strict=True) != path):
            raise ValueError("Marketing Codex authority directory is redirected")
        return info.st_mode, info.st_dev, info.st_ino

    def guard(self):
        for path, original in self.anchors.items():
            if self._directory(path) != original:
                raise ValueError("Marketing Codex authority directory changed")
        if self.runtime is not None:
            self.runtime.guard()
        if self.system_metadata is not None:
            self.audit["_codex_system_skill_checkpoint"](self.home, self.system_metadata)

    def checkpoint(self, phase):
        try:
            self.guard()
            self.runtime.checkpoint(phase)
            self.guard()
        except BaseException:
            self.failed_phase = phase
            raise
        if phase == "final-lifecycle":
            self.final_verified = True

    def evidence(self):
        self.guard()
        return {**self.runtime.evidence(),
                "runtime_semantics_unchanged": self.final_verified and self.failed_phase is None,
                "final_checkpoint": ("failed" if self.failed_phase is not None else
                                     "passed" if self.final_verified else "not_run"),
                "failed_phase": self.failed_phase}

    def system_files(self):
        self.guard()
        return self.audit["_codex_system_skill_checkpoint"](self.home, self.system_metadata)
