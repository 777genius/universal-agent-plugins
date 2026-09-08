"""Native-only, isolated controls for Windows promptio; no installer or scanner."""
import argparse
import json
import os
from pathlib import Path
import sys
import tempfile
import time
import uuid

p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--repo', type=Path, required=True)
p.add_argument('--binary', type=Path, required=True)
p.add_argument('--artifacts', type=Path, required=True)
p.add_argument('--control', action='append', choices=['consecutive', 'pre-canceled', 'timed-cancel', 'native-inherited', 'native-duplicate'])
a = p.parse_args()
if os.name != 'nt':
    raise SystemExit('Native Windows required')
sys.path.insert(0, str(a.repo.resolve() / 'scripts/terminal-ui'))
from harness import check, clean
from windows_conpty import ConPTY, finish_console

a.artifacts = a.artifacts.resolve()
a.artifacts.mkdir(parents=True, exist_ok=True)
results = []
for control in a.control or ['consecutive', 'pre-canceled', 'timed-cancel', 'native-inherited', 'native-duplicate']:
    evidence = a.artifacts / control
    evidence.mkdir(exist_ok=False)
    result = dict(control=control, passed=False)
    try:
        with tempfile.TemporaryDirectory(prefix='uap-console-diagnostic-') as temp:
            root = Path(temp)
            nonce = uuid.uuid4().hex
            env = {key: temp for key in ('HOME', 'USERPROFILE', 'APPDATA', 'LOCALAPPDATA', 'TMP', 'TEMP')}
            env.update(SystemRoot=os.environ['SystemRoot'], WINDIR=os.environ['SystemRoot'], PATH=temp,
                       UAP_CONSOLE_DIAGNOSTIC=control, UAP_CONSOLE_TRACE=str(evidence / 'reads.log'))
            status_path = evidence / 'status.json'
            config = evidence / 'job.json'
            config.write_text(json.dumps(dict(argv=[str(a.binary.resolve()), '-test.run=^TestDiagnosticConsoleReads$',
                              '-test.v', '-test.timeout=45s'], cwd=temp, nonce=nonce, status=str(status_path))), encoding='utf-8')
            session = ConPTY([sys.executable, '-I', str(a.repo.resolve() / 'scripts/terminal-ui/windows_job.py'), str(config)],
                             env, temp, 15, evidence=evidence)
            session.status_path = status_path
            try:
                session.wait('OWNER_READY_' + nonce)
                offset = 0
                for i in range(30):
                    offset = session.wait(r'DIAGNOSTIC_REUSE_READY ' + str(i) + r'\b', after=offset)
                    data = ('diagnostic-reuse-' + str(i) + '\r').encode()
                    with (evidence / 'sent.jsonl').open('a') as f:
                        f.write(json.dumps(dict(iteration=i, bytes_hex=data.hex(), monotonic=time.monotonic())) + '\n')
                    session.send(data)
                session.wait('DIAGNOSTIC_CONSOLE_OK', after=offset)
                session.wait('RESTORE_READY_' + nonce)
                status = json.loads(status_path.read_text(encoding='utf-8'))
                check(status['exit'] == 0, 'diagnostic helper failed')
                check(status['owner_before'] == status['owner_after'], 'owner state changed')
                offset = len(session.raw)
                session.send(('line_' + nonce + '\r').encode())
                session.wait('RESTORE_OK_' + nonce, after=offset)
                deadline = time.monotonic() + 15
                while session.poll() is None and time.monotonic() < deadline:
                    time.sleep(.02)
                check(session.poll() == 0, 'owner failed or hung')
                status = json.loads(status_path.read_text(encoding='utf-8'))
                check(status['line_read'] and status['owner_probe'] == status['owner_before'], 'owner reuse failed')
                check('line_' + nonce in clean(session.raw[offset:]), 'owner kernel echo missing')
            finally:
                finish_console(session, status_path, evidence)
            check(not session.forced, 'forced cleanup cannot pass')
            result['passed'] = True
    except Exception as exc:
        result['error'] = repr(exc)
    results.append(result)
    (a.artifacts / 'results.json').write_text(json.dumps(results, indent=2), encoding='utf-8')
    print(json.dumps(result), flush=True)
raise SystemExit(0 if all(r['passed'] for r in results) else 1)
