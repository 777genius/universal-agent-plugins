#!/usr/bin/env python3
"""Counterexample model for research, NOT UAP production or native-client tests.

Exact equality below is only a conservative candidate for reuse. It does not
prove runtime identity, secret equality, filesystem authority or service health.
"""
from __future__ import annotations

from dataclasses import dataclass, replace
import unittest


@dataclass(frozen=True)
class Binding:
    source: str = "directory:acme/shared-mcp"
    pin: str = "tree-sha256:fixture-a"
    client: str = "codex"
    profile: str = "profile-a"
    scope: str = "project-a"
    transport: str = "streamable-http"
    endpoint: str = "https://mcp.example.test/api"
    command: tuple[str, ...] = ()
    cwd: str = "project-a"
    data: str = "owned-data-a"
    environment: tuple[tuple[str, str], ...] = (("TENANT", "a"),)
    credential_reference: str = "account-a"
    permissions: tuple[str, ...] = ("read",)
    owned: bool = True


def candidate_for_reuse(first: Binding, second: Binding) -> bool:
    return first.owned and second.owned and first == second


def reachable(graph: dict[str, tuple[str, ...]], roots: tuple[str, ...]) -> set[str]:
    seen: set[str] = set()
    todo = list(roots)
    while todo:
        node = todo.pop()
        if node not in graph:
            raise ValueError("unknown fixture node")
        if node not in seen:
            seen.add(node)
            todo.extend(graph[node])
    return seen


class Boundaries(unittest.TestCase):
    def test_exact_owned_binding_is_only_a_candidate(self) -> None:
        self.assertTrue(candidate_for_reuse(Binding(), Binding()))

    def test_same_url_does_not_establish_shared_authority(self) -> None:
        first = Binding()
        changes = {
            "source": "git:acme/mirror", "pin": "tree-sha256:fixture-b",
            "client": "claude", "profile": "profile-b", "scope": "project-b",
            "transport": "sse", "command": ("node", "server.mjs"),
            "cwd": "project-b", "data": "owned-data-b",
            "environment": (("TENANT", "b"),), "credential_reference": "account-b",
            "permissions": ("read", "write"), "owned": False,
        }
        for field, value in changes.items():
            with self.subTest(field=field):
                other = replace(first, **{field: value})
                self.assertEqual(first.endpoint, other.endpoint)
                self.assertFalse(candidate_for_reuse(first, other))

    def test_same_content_does_not_make_registry_and_git_one_source(self) -> None:
        first = Binding()
        other = replace(first, source="git:acme/mirror")
        self.assertEqual(first.pin, other.pin)
        self.assertFalse(candidate_for_reuse(first, other))

    def test_foreign_identical_entries_are_not_adopted(self) -> None:
        self.assertFalse(candidate_for_reuse(Binding(owned=False), Binding(owned=False)))

    def test_shared_package_survives_one_consumer_removal(self) -> None:
        graph = {"a": ("mcp",), "b": ("mcp",), "mcp": ("helper",), "helper": ("mcp",)}
        self.assertEqual(reachable(graph, ("a", "b")), {"a", "b", "mcp", "helper"})
        self.assertEqual(reachable(graph, ("b",)), {"b", "mcp", "helper"})

    def test_unreachable_cycle_does_not_keep_itself_installed(self) -> None:
        graph = {"mcp": ("helper",), "helper": ("mcp",)}
        self.assertEqual(reachable(graph, ()), set())
        # A naive inbound-reference counter would retain both cycle members.
        self.assertTrue(all(any(node in deps for deps in graph.values()) for node in graph))

    def test_same_stdio_program_has_separate_client_bindings(self) -> None:
        first = Binding(transport="stdio", endpoint="", command=("node", "server.mjs"))
        self.assertFalse(candidate_for_reuse(first, replace(first, client="claude")))


if __name__ == "__main__":
    unittest.main(verbosity=2)
