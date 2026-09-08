"""Run only on Windows, after the existing strict qualification has completed."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import uuid

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--repo', type=Path, required=True)
parser.add_argument('--binary', type=Path, required=True)
parser.add_argument('--artifacts', type=Path, required=True)
args = parser.parse_args()
assert sys.platform == 'win32', 'native Windows required'
repo = args.repo.resolve()
binary = args.binary.resolve()
artifacts = args.artifacts.resolve()
sys.path.insert(0, str(repo / 'scripts' / 'terminal-ui'))
from windows_conpty import ConPTY, finish_console

artifacts.mkdir(parents=True, exist_ok=True)
results = {'binary_sha256': hashlib.sha256(binary.read_bytes()).hexdigest(), 'cases': []}
for first in ('errno', 'format-message', 'wrap'):
    evidence = artifacts / first
    evidence.mkdir(exist_ok=True)
    row = {'first': first, 'passed': False}
    try:
        with tempfile.TemporaryDirectory(prefix='uap-error-resource-') as temp:
            root = Path(temp)
            nonce = uuid.uuid4().hex
            env = {key: str(root) for key in ('HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'TMP', 'TEMP')}
            env.update(SystemRoot=os.environ['SystemRoot'], WINDIR=os.environ['SystemRoot'],
                       PATH=str(root), UAP_QUALIFICATION_CASE='windows-console-error-resource',
                       UAP_QUALIFICATION_ERROR_FIRST=first)
            status_path = evidence / 'status.json'
            config_path = evidence / 'job.json'
            config_path.write_text(json.dumps(dict(
                argv=[str(binary), '-test.run=^TestQualificationConsoleErrorResourceProbe$',
                      '-test.v', '-test.timeout=45s'], cwd=str(root), nonce=nonce,
                status=str(status_path))), encoding='utf-8')
            session = ConPTY([sys.executable, '-I', str(repo / 'scripts' / 'terminal-ui' / 'windows_job.py'),
                              str(config_path)], env, str(root), 10, evidence=evidence)
            session.status_path = status_path
            try:
                session.wait('OWNER_READY_' + nonce)
                session.wait('RESTORE_READY_' + nonce)
                status = json.loads(status_path.read_text(encoding='utf-8'))
                assert status['exit'] == 0, 'probe failed; see transcript'
                assert status['before'] == status['after'], 'owner console changed'
                session.send(('line_' + nonce + '\r').encode())
                session.wait('REUSE_OK_' + nonce)
                status = json.loads(status_path.read_text(encoding='utf-8'))
                assert status.get('reused'), 'owner console reuse failed'
            finally:
                finish_console(session, status_path, evidence)
        row['passed'] = True
    except Exception as exc:
        row['error'] = repr(exc)
    results['cases'].append(row)
    print(json.dumps(row), flush=True)
    (artifacts / 'results.json').write_text(json.dumps(results, indent=2), encoding='utf-8')
sys.exit(0 if all(row['passed'] for row in results['cases']) else 1)
