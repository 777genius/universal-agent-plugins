"""Independent result, durable state and native-effect oracles."""
import hashlib
import json
from pathlib import Path

from harness import check, hashes


def envelope(body, exit_code):
    try:
        value = json.loads(body)
    except (json.JSONDecodeError, UnicodeDecodeError) as error:
        raise AssertionError('stdout must contain exactly one JSON envelope') from error
    check(isinstance(value, dict) and value.get('schema_version') == 1,
          'missing versioned JSON envelope')
    check(value.get('command') == 'skills add', 'wrong JSON command')
    check(value.get('result') in ('success', 'failure'), 'invalid JSON outcome')
    check((exit_code == 0) == (value['result'] == 'success'),
          'exit status disagrees with JSON result')
    return value


def items_by_agent(result):
    rows = result.get('items')
    check(isinstance(rows, list) and rows, 'result lost reviewed item outcomes')
    found = {}
    for row in rows:
        target = row.get('targetId')
        check(target and target not in found, 'unexpected duplicate/missing target result')
        found[target] = row
    return found


def durable_tree(fixture):
    # Caches and logs can grow; desired intent, receipts and native bytes cannot.
    data = fixture.data
    result = {'home': hashes(fixture.home), 'project': hashes(fixture.project)}
    for name in ('managed', 'operations-v2', 'state-v2.json', 'uap.toml', 'uap.lock',
                 'environments', 'plugin-data'):
        path = data / name
        result[name] = hashes(path) if path.is_dir() else (
            hashlib.sha256(path.read_bytes()).hexdigest() if path.is_file() else None)
    registry = fixture.root / 'native-registry.json'
    result['native-registry'] = registry.read_bytes().hex() if registry.exists() else None
    return result


def bindings(fixture):
    state = fixture.data / 'state-v2.json'
    check(state.is_file(), 'no durable installation state')
    value = json.loads(state.read_text(encoding='utf-8'))
    result = {}
    for installation in value.get('installations', []):
        for binding in installation.get('clients', {}).values():
            target = binding.get('client_id')
            check(target not in result, 'unexpected duplicate native binding')
            result[target] = binding
    return result


def installed(fixture, targets):
    actual = bindings(fixture)
    check(set(actual) == set(targets), f'durable targets {set(actual)}, expected {set(targets)}')
    for target, binding in actual.items():
        path = Path(binding['target_locator'])
        check(path.resolve().is_relative_to(fixture.root.resolve()), 'native target escaped fixture')
        check(binding.get('materialization') == 'materialized', 'binding is not materialized')
        check(any(receipt.get('phase') == 'committed' for receipt in binding.get('receipts', [])),
              'binding has no committed receipt')
        skill = path / 'skills' / fixture.skill_name / 'SKILL.md'
        check(skill.is_file(), 'selected Skill missing from native package')
        if fixture.skill_bytes:
            check(skill.read_bytes() == fixture.skill_bytes, 'installed Skill bytes changed')
    check((fixture.data / 'uap.toml').is_file(), 'saved desired intent missing')
    check((fixture.data / 'uap.lock').is_file(), 'saved immutable lock missing')
    return actual


def native_mutations(calls):
    return [args for args in calls if args[:2] in (
        ['plugin', 'add'], ['plugin', 'remove']) or args[:3] in (
            ['plugin', 'marketplace', 'add'], ['plugin', 'marketplace', 'remove'],
            ['plugin', 'marketplace', 'update'])]


def failure_log(result, fixture):
    path = result.get('log_file')
    check(path is not None, 'failed binding lost its diagnostic path')
    file = Path(path)
    check(file.resolve().is_relative_to(fixture.root.resolve()), 'log escaped fixture')
    check(file.is_file(), 'result points to a nonexistent log')
    entries = [json.loads(line) for line in file.read_text(encoding='utf-8').splitlines()]
    check(entries, 'failed binding log is empty')
    return entries


def persist(path, value):
    Path(path).write_text(json.dumps(value, sort_keys=True, indent=2) + '\n', encoding='utf-8')
