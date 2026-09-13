"""Dispatch-only public release proof. All mutations live in a temporary root."""
import hashlib
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import tempfile
import time
import urllib.request

REVISION = '05d1f19796b257b1f0544464145d2653f721e21a'
VERSIONS = {'agentplugins': '0.1.61', 'plugin-kit-ai': '2.0.1'}
COMMANDS = ('validate', 'inspect', 'pack', 'compat')


def retry(operation):
    for attempt in range(4):
        try:
            return operation()
        except Exception:
            if attempt == 3:
                raise
            time.sleep(10 * (attempt + 1))


def download(url):
    def fetch():
        with urllib.request.urlopen(url, timeout=60) as response:
            return response.read()
    return retry(fetch)


def identity(report, version):
    data = report.get('data', report)
    if report.get('revision', data.get('revision')) != REVISION:
        raise ValueError('wrong source revision')
    if data.get('product_version') != version:
        raise ValueError('wrong product version')


def main(channel):
    evidence = {'channel': channel, 'revision': REVISION, 'results': []}
    with tempfile.TemporaryDirectory(prefix='uap-public-channel-') as temporary:
        root = Path(temporary)
        env = {k: v for k, v in os.environ.items() if k in (
            'PATH', 'SystemRoot', 'SYSTEMROOT', 'WINDIR', 'COMSPEC', 'PATHEXT')}
        for key in ('HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'XDG_CONFIG_HOME',
                    'XDG_CACHE_HOME', 'XDG_DATA_HOME', 'TMP', 'TEMP', 'TMPDIR',
                    'npm_config_cache', 'PIP_CACHE_DIR', 'HOMEBREW_CACHE', 'HOMEBREW_LOGS'):
            directory = root / key
            directory.mkdir()
            env[key] = str(directory)
        for kind in ('user', 'global'):
            config = root / f'npm-{kind}.rc'
            config.touch()
            env[f'npm_config_{kind}config'] = str(config)
        env.update(CI='1', PIP_CONFIG_FILE=os.devnull, HOMEBREW_NO_AUTO_UPDATE='1',
                   HOMEBREW_NO_ANALYTICS='1', HOMEBREW_NO_INSTALL_CLEANUP='1')

        def run(args):
            result = subprocess.run(list(map(str, args)), cwd=root, env=env,
                                    capture_output=True, text=True, timeout=240)
            evidence['results'].append({'argv': list(map(str, args)), 'status': result.returncode,
                                        'stdout': result.stdout, 'stderr': result.stderr})
            result.check_returncode()
            return result.stdout

        try:
            products = VERSIONS if channel in ('npm', 'github') else ['plugin-kit-ai']
            command_failures = []
            for product in products:
                version = VERSIONS[product]
                install = root / product
                install.mkdir()
                suffix = '.exe' if sys.platform == 'win32' else ''
                if channel == 'npm':
                    package = 'universal-agent-plugins' if product == 'agentplugins' else product
                    # Invoke npm's JS CLI directly, including on Windows (no shell quoting).
                    import shutil
                    node = Path(shutil.which('node')).resolve()
                    npm = node.parent / 'node_modules/npm/bin/npm-cli.js'
                    if not npm.exists():
                        npm = Path(shutil.which('npm')).resolve()
                    retry(lambda: run([node, npm, 'install', '--prefix', install,
                                       '--registry=https://registry.npmjs.org', '--no-audit',
                                       '--no-fund', '--fetch-retries=0', f'{package}@{version}']))
                    executable = [node, install / 'node_modules' / package / 'bin' / f'{product}.js']
                elif channel == 'pypi':
                    run([sys.executable, '-m', 'venv', install / 'venv'])
                    bindir = install / 'venv' / ('Scripts' if suffix else 'bin')
                    retry(lambda: run([bindir / ('python' + suffix), '-m', 'pip', 'install',
                                       '--index-url=https://pypi.org/simple', '--retries=0',
                                       f'plugin-kit-ai=={version}']))
                    executable = [bindir / (product + suffix)]
                elif channel == 'brew':
                    # Private Homebrew checkout/prefix: never mutate the runner's installation.
                    run(['git', 'clone', '--depth=1', 'https://github.com/Homebrew/brew', install / 'brew'])
                    brew = install / 'brew/bin/brew'
                    formula = '777genius/plugin-kit-ai/plugin-kit-ai'
                    retry(lambda: run([brew, 'tap', '777genius/plugin-kit-ai']))
                    metadata = json.loads(run([brew, 'info', '--json=v2', formula]))
                    if metadata['formulae'][0]['versions']['stable'] != version:
                        raise ValueError('Homebrew formula is not exact requested version')
                    retry(lambda: run([brew, 'install', formula]))
                    executable = [install / 'brew/bin/plugin-kit-ai']
                elif channel == 'github':
                    tag = f'{product}-v{version}'
                    base = f'https://github.com/777genius/universal-agent-plugins/releases/download/{tag}/'
                    target = {'Linux': 'linux', 'Darwin': 'darwin', 'Windows': 'windows'}[platform.system()]
                    arch = 'arm64' if platform.machine().lower() in ('arm64', 'aarch64') else 'amd64'
                    name = f'{product}_{version}_{target}_{arch}' + (suffix if product == 'agentplugins' else '.tar.gz')
                    checksums = download(base + 'checksums.txt').decode()
                    expected = dict((line.split()[-1].lstrip('*'), line.split()[0])
                                    for line in checksums.splitlines() if line.strip())[name]
                    asset = download(base + name)
                    if hashlib.sha256(asset).hexdigest() != expected:
                        raise ValueError('release checksum mismatch')
                    binary = install / (product + suffix)
                    if product == 'agentplugins':
                        binary.write_bytes(asset)
                    else:
                        import io
                        import tarfile
                        with tarfile.open(fileobj=io.BytesIO(asset)) as archive:
                            members = [m for m in archive.getmembers() if Path(m.name).name == product + suffix and m.isfile()]
                            if len(members) != 1:
                                raise ValueError('ambiguous release executable')
                            binary.write_bytes(archive.extractfile(members[0]).read())
                    binary.chmod(0o700)
                    executable = [binary]
                else:
                    raise ValueError('unknown channel')
                author = executable + (['author'] if product == 'agentplugins' else [])
                identity(json.loads(run(author + ['version', '--format=json'])), version)
                project = install / 'fixture'
                initialized = json.loads(run(author + ['init', project, '--template=skill',
                              '--name=public-channel-fixture',
                              '--description=Disposable public channel fixture.', '--format=json']))
                if initialized.get('data', initialized).get('revision') != REVISION:
                    raise ValueError('init source revision mismatch')
                if not (project / 'plugin.json').is_file():
                    raise ValueError('init omitted plugin.json')
                for command in COMMANDS:
                    args = author + [command, project, '--format=json']
                    if command == 'compat':
                        args += ['--target=codex']
                    try:
                        report = json.loads(run(args))
                        if report.get('data', report).get('revision') != REVISION:
                            raise ValueError('command source revision mismatch')
                    except Exception as error:
                        command_failures.append(f'{product} {command}: {error}')
            if command_failures:
                raise ValueError('; '.join(command_failures))
            evidence['status'] = 'passed'
        except Exception as error:
            evidence.update(status='failed', error=str(error))
            raise
        finally:
            print(json.dumps(evidence, indent=2), flush=True)


if __name__ == '__main__':
    main(sys.argv[1])
