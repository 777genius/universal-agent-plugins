#!/usr/bin/env python3
"""Ordinary Devin metadata in an authored local-policy profile; no UAP mutation."""
import argparse
import hashlib
from pathlib import Path

import devin_structured_inventory_admission as base

ORIGINAL = b'{"read_config_from":{"cloud":false}}\n'


class OrdinaryEffects(base.BootstrapEffects):
    metadata_first = True
    config_path = "home/config/devin/config.json"
    metadata_route = "ordinary-original-user-config; authored local-policy profile only"

    @property
    def config_mode(self):
        # Measured native first migration makes the original0644 file private.
        # Only the first mcp-a postcondition can enter migrating=True. Once
        # admitted, the complete after identity/hash stays frozen at0600.
        return 0o600 if self.migrating or self.after is not None else 0o644

    @classmethod
    def prepare(cls, ctx):
        path = ctx.root / base.METADATA_CONFIG
        base.require(not path.exists() and not path.is_symlink(), "ordinary route has a private config")
        return []

    def __init__(self, ctx, baseline):
        super().__init__(ctx, baseline)
        self.step, self.migrating = 0, False
        self.before = self.fingerprint(ORIGINAL, baseline[self.config_path])
        held = ctx.known_file_diagnostics.before["user-config"]
        original = held["identity"]
        base.require(self.before["identity"] == original[:3] + [original[5]] + original[3:5] + original[6:]
                     and self.before["sha256"] == held["sha256"] and self.before["bytes"] == held["bytes"],
                     "ordinary original User authority differs")
        self.after = None
        # Observed effect evidence also survives a settled nonzero command; it is
        # not a success receipt. The inherited runner still refuses that result.
        self.report = {"phase": "mcp-a", "before": self.before, "after": None}
        ctx.summary["ordinary_user_config_migration"] = self.report
        ctx.ordinary_effects = self
        self.verify(baseline)

    @staticmethod
    def fingerprint(body, info):
        return {"identity": list(info), "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}

    def verify(self, current):
        self.prepare(self.ctx)
        self.ctx.known_file_diagnostics.verify()
        root = self.ctx.original_output_identity
        base.require(base.root_identity(self.ctx.root) == root
                     and all(key in current and (key == self.config_path or current[key] == value)
                             for key, value in self.baseline.items()), "ordinary original metadata changed")
        info = current[self.config_path]
        base.require(len(info) == 9 and info[0] == root[0] and info[2] == base.stat.S_IFREG | self.config_mode
                     and info[3] == 1 and tuple(info[4:6]) == root[3:] and 0 <= info[6] <= 256,
                     "ordinary config pre-read authority refused")
        if not self.migrating:
            base.require(list(info) == (self.after or self.before)["identity"], "ordinary config pre-read identity changed")
        super().verify(current)
        self.ctx.known_file_diagnostics.verify()

    def verify_config(self, body, info):
        base.require(len(body) <= 256, "ordinary User config byte bound")
        observed = self.fingerprint(body, info)
        if not self.migrating:
            base.require(observed == (self.after or self.before), "ordinary User config changed outside first inventory")
            return
        value = base.strict_json(body)
        base.require(self.after is None and type(value) is dict
                     and value == {"read_config_from": {"cloud": False}, "version": 1}
                     and type(value["version"]) is int and value["read_config_from"]["cloud"] is False,
                     "ordinary User migration changed policy or version")
        self.after = observed
        self.report["after"] = observed

    def verify_command(self, before, after):
        base.require(self.step < len(base.METADATA_PROBES), "ordinary metadata command count")
        self.migrating = base.METADATA_PROBES[self.step] == "mcp-a"
        try:
            super().verify_command(before, after)
        finally:
            self.migrating = False
        self.step += 1


def require_ordinary_commands(ctx, *, recipe="ordinary"):
    expected = [(label, arguments, "a") for label, arguments in base.HELP]
    expected += [(kind + "-" + label, [kind, "list", "--json"], scope)
                 for label, scope in (("a", "a"), ("b", "b"), ("user", "user"), ("a-repeat", "a"))
                 for kind in ("mcp", "skills")]
    records = [row for row in ctx.summary["commands"] if row.get("probe") in base.METADATA_PROBES]
    base.require(recipe in ("ordinary", "ordinary-global") and ctx.summary.get("metadata_recipe") == recipe and
                 ctx.summary.get("metadata_route") == OrdinaryEffects.metadata_route and len(records) == len(expected),
                 "ordinary metadata recipe or sequence differs")
    for record, (label, arguments, scope) in zip(records, expected):
        base.require(record["probe"] == label and record.get("argv") == [*ctx.argv, *arguments]
                     and record.get("cwd") == str(ctx.projects[scope]), "ordinary metadata argv or scope differs")


def collect(ctx, probes, readout_state=None):
    ctx.summary["metadata_recipe"] = "ordinary"
    ctx.summary["metadata_route"] = OrdinaryEffects.metadata_route
    base.collect(ctx, probes, readout_state, effects_type=OrdinaryEffects)
    effects = ctx.ordinary_effects
    effects.verify(base.metadata_inventory(ctx.root, ctx.budget))
    require_ordinary_commands(ctx)
    base.require(effects.step == len(base.METADATA_PROBES) and effects.after is not None,
                 "ordinary metadata gate incomplete")
    ctx.summary["ordinary_metadata_capture_complete"] = True


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True, type=Path)
    raise SystemExit(base.run(parser.parse_args().output, collector=collect, recipe="ordinary"))
