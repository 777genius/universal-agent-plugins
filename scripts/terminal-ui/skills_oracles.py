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
    state = json.loads((fixture.data / 'state-v2.json').read_text(encoding='utf-8'))
    check(set(actual) == set(targets), f'durable targets {set(actual)}, expected {set(targets)}')
    for target, binding in actual.items():
        path = Path(binding['target_locator'])
        check(path.resolve().is_relative_to(fixture.root.resolve()), 'native target escaped fixture')
        check(binding.get('materialization') == 'materialized', 'binding is not materialized')
        check(any(receipt.get('phase') == 'committed' for receipt in binding.get('receipts', [])),
              'binding has no committed receipt')
        skill = path / 'skills' / fixture.skill_name / 'SKILL.md'
        check(skill.is_file(), 'selected Skill missing from native package')
        body = skill.read_bytes()
        if fixture.skill_bytes:
            check(body == fixture.skill_bytes, 'installed Skill bytes changed')
        if fixture.skill_blob:
            header = ('blob ' + str(len(body)) + '\0').encode()
            check(hashlib.sha1(header + body, usedforsecurity=False).hexdigest() == fixture.skill_blob,
                  'GitHub Skill bytes differ from the immutable pinned source blob')
        if fixture.publisher:
            expected = {name: hashlib.sha256(data).hexdigest()
                        for name, data in fixture.publisher.files.items()}
            check(hashes(skill.parent) == expected, 'Skill resource tree differs from complete source')
        owner = next((receipt for receipt in state.get('environment_receipts', [])
                      if receipt.get('client_binding_id') == binding['client_binding_id']), None)
        check(owner is not None and owner.get('client_id') == target,
              'native binding has no matching saved environment ownership receipt')
        revision = binding.get('package_revision', {})
        check(owner.get('package_digest') == revision.get('tree_digest'),
              'receipt and installed immutable package digest disagree')
        committed = [receipt for receipt in binding.get('receipts', [])
                     if receipt.get('phase') == 'committed']
        check(any(receipt.get('environment_authority', {}).get('source_key') ==
                  owner.get('source_key') and
                  receipt.get('environment_authority', {}).get('client_id') == target
                  for receipt in committed),
              'committed native receipt lost its reviewed source/client authority')
        lock = (fixture.data / 'uap.lock').read_text(encoding='utf-8')
        check(revision.get('tree_digest', 'missing-digest') in lock,
              'installed package digest does not occur in the saved immutable lock')
        if target == 'codex':
            check(binding.get('activation') == 'active' and
                  binding.get('verification') == 'installation_verified' and
                  binding.get('authentication') == 'not_required',
                  'synthetic Codex protocol did not prove native Skill completion')
            registry = json.loads((fixture.root / 'native-registry.json').read_text())
            cache = Path(registry['cache']).resolve()
            check(cache.is_relative_to(fixture.root.resolve()), 'native cache escaped fixture')
            cached = cache / 'skills' / fixture.skill_name / 'SKILL.md'
            check(cached.read_bytes() == body, 'native discovered cache has different Skill bytes')
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
