"""Independent result, durable state and native-effect oracles."""
import hashlib
import json
from pathlib import Path
import tomllib

from harness import check, hashes


def envelope(body, exit_code, command='skills add'):
    try:
        value = json.loads(body)
    except (json.JSONDecodeError, UnicodeDecodeError) as error:
        raise AssertionError('stdout must contain exactly one JSON envelope') from error
    check(isinstance(value, dict) and value.get('schema_version') == 1,
          'missing versioned JSON envelope')
    check(value.get('command') == command, 'wrong JSON command')
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


def installed(fixture, targets, *, retained_failed=(), prepared_targets=(), result=None):
    """Prove successful deliveries and explicitly selected incomplete ownership.

    The installer commits a managed directory before native activation. A failed
    activation retains that directory and its receipts for repair; it is not a
    successful native installation. An interrupted native add can also leave
    committed Prepared ownership despite an installed native registry entry.
    Callers must name these distinct expectations; neither counts as healthy.
    """
    successful, failed, prepared = set(targets), set(retained_failed), set(prepared_targets)
    check(len(successful) == len(targets) and len(failed) == len(retained_failed) and
          len(prepared) == len(prepared_targets) and
          len(successful | failed | prepared) == len(successful) + len(failed) + len(prepared),
          'ambiguous expected binding outcomes')
    check(not failed or failed == {'codex'}, 'unsupported retained failure expectation')
    check(not prepared or prepared == {'codex'}, 'unsupported prepared ownership expectation')
    check(not prepared or result is None, 'prepared ownership has no completed command result')
    actual = bindings(fixture)
    state = json.loads((fixture.data / 'state-v2.json').read_text(encoding='utf-8'))
    expected = successful | failed | prepared
    check(set(actual) == expected, f'durable targets {set(actual)}, expected {expected}')
    try:
        lock = json.loads((fixture.data / 'uap.lock').read_text(encoding='utf-8'))
    except json.JSONDecodeError as error:
        raise AssertionError('saved immutable lock is not valid JSON') from error
    check(lock.get('lock_version') == 1 and isinstance(lock.get('packages'), list),
          'saved immutable lock has no versioned package inventory')
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
            resources = {name: hashlib.sha256(data).hexdigest()
                         for name, data in fixture.publisher.files.items()}
            check(hashes(skill.parent) == resources, 'Skill resource tree differs from complete source')
        owners = [receipt for receipt in state.get('environment_receipts', [])
                  if receipt.get('client_binding_id') == binding['client_binding_id']]
        check(len(owners) == 1 and owners[0].get('client_id') == target,
              'native binding has no matching saved environment ownership receipt')
        owner = owners[0]
        installations = [item for item in state.get('installations', [])
                         if item.get('clients', {}).get(binding['client_binding_id']) == binding]
        check(len(installations) == 1 and
              owner.get('installation_id') == installations[0].get('installation_id') and
              bool(owner.get('installation_id')) and owner.get('phase') == 'committed' and
              owner.get('environment_id') == 'user-default',
              'saved ownership receipt does not identify this committed installation/environment')
        revision = binding.get('package_revision', {})
        check(owner.get('package_digest') == revision.get('tree_digest'),
              'receipt and installed immutable package digest disagree')
        committed = [receipt for receipt in binding.get('receipts', [])
                     if receipt.get('phase') == 'committed']
        check(any(receipt.get('environment_authority', {}).get('source_key') ==
                  owner.get('source_key') and
                  receipt.get('environment_authority', {}).get('client_id') == target and
                  receipt.get('environment_authority', {}).get('alias') == owner.get('alias') and
                  receipt.get('environment_authority', {}).get('environment_id') ==
                  owner.get('environment_id') and
                  receipt.get('client_binding_id') == binding['client_binding_id'] and
                  receipt.get('active_path') == binding['target_locator']
                  for receipt in committed),
              'committed native receipt lost its reviewed source/client authority or alias')
        check(owner.get('alias') and owner.get('source_key'),
              'saved ownership receipt lost its package alias/source identity')
        packages = [package for package in lock['packages']
                    if package.get('alias') == owner['alias'] and
                    package.get('source_key') == owner['source_key'] and
                    isinstance(package.get('targets'), list) and target in package['targets']]
        check(len(packages) == 1, 'lock lost the exact binding alias/source/target')
        check(bool(revision.get('tree_digest')) and
              packages[0].get('tree_digest') == revision['tree_digest'],
              'matching locked binding and installed immutable package digest disagree')
        saved_binding_intent(fixture, packages[0], owner, expected)
        if target in failed:
            retained_native_failure(fixture, binding)
        elif target == 'cursor':
            check(binding.get('activation') == 'manual_activation_required' and
                  binding.get('verification') == 'package_validated',
                  'Cursor delivery did not retain its successful prepared boundary')
        elif target == 'codex':
            if target in prepared:
                check(binding.get('scope') == 'user' and
                      binding.get('activation') == 'prepared' and
                      binding.get('verification') == 'package_validated' and
                      binding.get('authentication') == 'not_required' and
                      binding.get('policy') == 'allowed' and
                      binding.get('native_activation_attempt') in ('', None) and
                      binding.get('pending_native_intent') is None,
                      'interrupted Codex ownership is not the exact committed Prepared boundary')
            else:
                check(binding.get('activation') == 'active' and
                      binding.get('verification') == 'installation_verified' and
                      binding.get('authentication') == 'not_required',
                      'synthetic Codex protocol did not prove native Skill completion')
            registry = json.loads((fixture.root / 'native-registry.json').read_text(encoding='utf-8'))
            native_registration(path, registry)
            cache = Path(registry['cache']).resolve()
            check(cache.is_relative_to(fixture.root.resolve()), 'native cache escaped fixture')
            cached = cache / 'skills' / fixture.skill_name / 'SKILL.md'
            check(cached.read_bytes() == body, 'native discovered cache has different Skill bytes')
            if target in prepared:
                check(hashes(cached.parent) == hashes(skill.parent),
                      'interrupted Codex cache has different complete Skill resources')
        if result is not None:
            reported_binding(result, expected, target, owner, binding, target in failed)
    check((fixture.data / 'uap.toml').is_file(), 'saved desired intent missing')
    check((fixture.data / 'uap.lock').is_file(), 'saved immutable lock missing')
    return {target: actual[target] for target in targets}


def saved_binding_intent(fixture, package, owner, targets):
    """Join this single-Skill qualification's saved portable declaration to its pin."""
    manifest_path = fixture.data / 'uap.toml'
    check(manifest_path.is_file(), 'saved desired intent missing')
    try:
        manifest = tomllib.loads(manifest_path.read_text(encoding='utf-8'))
    except (tomllib.TOMLDecodeError, UnicodeDecodeError) as error:
        raise AssertionError('saved desired intent is not valid TOML') from error
    alias = owner['alias']
    check(type(manifest.get('schema_version')) is int and manifest['schema_version'] == 1 and
          isinstance(manifest.get('dependencies'), dict) and
          set(manifest['dependencies']) == {alias},
          'saved desired intent lost its versioned selected alias')
    check(manifest.get('targets') == {target: [alias] for target in targets} and
          isinstance(package.get('targets'), list) and
          len(package['targets']) == len(targets) and set(package['targets']) == targets,
          'saved desired intent targets disagree with reviewed lock/ownership')
    source = package.get('source', {})
    check(isinstance(source, dict) and len(source) == 1,
          'saved Skill lock lost its unambiguous source')
    wanted = {'kind': 'skill', 'skill': fixture.skill_name}
    if 'web' in source:
        web = source['web']
        check(isinstance(web, dict) and web.get('skill') == fixture.skill_name and
              isinstance(web.get('url'), str) and bool(web['url']),
              'saved Skill lock lost its website selection')
        wanted['website'] = web['url']
        source_key = 'https-skill:' + web['url'] + ':' + fixture.skill_name
    else:
        git = source.get('git', {})
        check(isinstance(git, dict) and git.get('kind') == 'skill' and
              git.get('skill') == fixture.skill_name and bool(git.get('repository')),
              'saved Skill lock lost its GitHub selection')
        repository = git['repository'].lower()
        wanted['git'] = repository
        if git.get('path'):
            wanted['path'] = git['path']
        selector = git.get('requested', {})
        check(selector.get('kind') in ('commit', 'ref', 'default'),
              'saved Skill lock lost its requested Git selector')
        if selector['kind'] != 'default':
            check(isinstance(selector.get('value'), str) and bool(selector['value']),
                  'saved Skill lock has no requested Git revision')
            wanted[selector['kind']] = selector['value']
        if selector['kind'] == 'commit':
            check(git.get('commit') == selector['value'],
                  'saved Skill lock changed the exact requested Git commit')
        source_key = 'github-skill:' + repository + ':' + fixture.skill_name + ':' + git.get('path', '')
    declaration = manifest['dependencies'][alias]
    if isinstance(declaration, dict) and 'git' in wanted:
        declaration = dict(declaration)
        endpoint = declaration.get('git')
        # The production encoder may use its portable repository or canonical
        # HTTPS endpoint; both have the same SourceKey in environment/model.go.
        if isinstance(endpoint, str) and endpoint.lower() == 'https://github.com/' + repository + '.git':
            declaration['git'] = repository
        elif isinstance(endpoint, str):
            declaration['git'] = endpoint.lower()
    check(declaration == wanted and owner['source_key'] == source_key and
          package['source_key'] == source_key,
          'saved desired intent source disagrees with selected lock/ownership')
    expected_source = getattr(fixture, 'expected_source', None)
    check(expected_source is not None, 'fixture has no independent expected source descriptor')
    check(wanted == expected_source, 'saved origin differs from requested fixture source')


def retained_native_failure(fixture, binding):
    # This qualification injects failure before Codex plugin add. Directory
    # commit survives; native marketplace compensation must leave no installed
    # registration. Do not generalize this to uncertain cleanup or recovery.
    check(binding.get('activation') == 'failed' and
          binding.get('verification') == 'failed' and
          binding.get('authentication') == 'not_required' and
          binding.get('policy') == 'allowed',
          'retained native binding does not record the expected activation failure')
    registry_path = fixture.root / 'native-registry.json'
    check(registry_path.is_file(), 'retained native failure lost its registry observation')
    registry = json.loads(registry_path.read_text(encoding='utf-8'))
    check(isinstance(registry, dict) and set(registry) == {'path'} and
          Path(registry['path']).resolve() == Path(binding['target_locator']).resolve(),
          'failed native binding retained registration, cache or unrelated source')
    native_home = Path(getattr(fixture, 'env', {}).get('CODEX_HOME', fixture.home / '.codex'))
    check(native_home.resolve().is_relative_to(fixture.root.resolve()),
          'failed native profile escaped fixture')
    cache = native_home / 'plugins' / 'cache'
    check(not cache.exists() and not cache.is_symlink(),
          'failed native add left an unexpected cache effect')


def reported_binding(result, expected, target, owner, binding, failed):
    rows = items_by_agent(result)
    check(set(rows) == expected, 'reported targets disagree with durable reviewed bindings')
    item = rows[target]
    check(item.get('status') in (('failed',) if failed else ('succeeded', 'unchanged')),
          'reported binding outcome disagrees with durable/native evidence')
    check(item.get('alias') == owner['alias'] and
          item.get('destinationPath') == binding['target_locator'],
          'reported binding lost its owned alias or destination')
    if failed:
        check(result.get('result') == 'failure' and result.get('status') == 'partial',
              'retained failed binding was not reported as partial failure')


def native_registration(path, registry):
    manifest = json.loads((path / 'plugin.json').read_text(encoding='utf-8'))
    name, version = manifest.get('name'), manifest.get('version') or '0.0.0'
    marketplace = registry.get('marketplace')
    check(name and marketplace, 'native registry lost its package/marketplace identity')
    spec = name + '@' + marketplace
    check(registry.get('name') == name and registry.get('spec') == spec and
          Path(registry.get('path', '')).resolve() == path.resolve(),
          'native registry source does not match the installed binding')
    registrations = registry.get('installed')
    check(isinstance(registrations, list), 'native registry has no installed inventory')
    matches = [row for row in registrations
               if row.get('pluginId') == spec and row.get('name') == name and
               row.get('marketplaceName') == marketplace and row.get('version') == version and
               row.get('installed') is True and row.get('enabled') is True and
               Path(row.get('source', {}).get('path', '')).resolve() == path.resolve()]
    check(len(matches) == 1, 'native package is not registered as installed and enabled')


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
    try:
        with file.open(encoding='utf-8', newline='\n') as stream:
            entries = [json.loads(line) for line in stream]
    except json.JSONDecodeError as error:
        raise AssertionError('diagnostic log is not valid JSON Lines') from error
    check(entries and isinstance(entries[0], dict), 'failed binding log has no run header')
    header = entries[0]
    check(header.get('type') == 'run' and header.get('schema_version') == 1 and
          header.get('command') == 'skills add' and
          isinstance(header.get('attempt'), int) and header['attempt'] >= 1,
          'diagnostic run header does not identify this Skills attempt')
    failed_statuses = {'failed', 'not_attempted', 'incomplete', 'conflict',
                       'recovery_required', 'cancelled', 'canceled'}
    expected = [row for row in items_by_agent(result).values()
                if row.get('status') in ('failed', 'skipped')]
    check(expected, 'diagnostic oracle requires an actual failed result')
    records = [row for row in entries[1:] if isinstance(row, dict) and
               row.get('type') == 'binding' and row.get('status') in failed_statuses]
    for item in expected:
        check(isinstance(item.get('alias'), str) and bool(item['alias'].strip()),
              'failed result lost the expected package alias')
        check(any(row.get('client') == item['targetId'] and
                  row.get('alias') == item.get('alias') and
                  isinstance(row.get('code'), str) and bool(row['code'].strip()) and
                  isinstance(row.get('summary'), str) and bool(row['summary'].strip())
                  for row in records),
              'diagnostic log lost matching failed binding, classification or summary')
    return entries


def persist(path, value):
    Path(path).write_text(json.dumps(value, sort_keys=True, indent=2) + '\n', encoding='utf-8')
