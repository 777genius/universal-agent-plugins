#!/usr/bin/env python3
"""Structural guard for this one workflow, not hosted required-check enforcement."""
import json
from pathlib import Path
import re
import sys


def require(ok, message):
    if not ok: raise ValueError(message)


def results(value):
    require(set(value) == {'native', 'packed'} and all(v.get('result') == 'success' for v in value.values()),
            'native and packed must both exist and succeed')


def check(text, runner):
    # Fixed indentation is intentional: reject unsupported rewrites for review.
    require('\non:\n  pull_request:\n  workflow_dispatch:\n' in text, 'unfiltered PR and dispatch required')
    require(not re.search(r'^\s*(?:paths(?:-ignore)?|branches(?:-ignore)?|continue-on-error):', text, re.M), 'success override/filter')
    jobs = dict(re.findall(r'^  (\w+):\n(.*?)(?=^  \w+:\n|\Z)', text.split('\njobs:\n', 1)[1], re.M | re.S))
    require(set(jobs) == {'native', 'packed', 'acceptance'}, 'required jobs missing/renamed')
    native, packed, aggregate = (jobs[n] for n in ('native', 'packed', 'acceptance'))
    for body in (native, packed):
        require(not re.search(r'^    (?:if|continue-on-error):', body, re.M), 'required job cannot be conditional')
    matrix = re.findall(r'- \{runner: ([\w.-]+), os: (\w+), arch: (\w+)\}', native)
    require(set(matrix) == {('ubuntu-24.04', 'linux', 'amd64'), ('ubuntu-24.04-arm', 'linux', 'arm64'),
        ('windows-2022', 'windows', 'amd64'), ('windows-11-arm', 'windows', 'arm64')} and len(matrix) == 4, 'four native lanes required')
    require('    needs: [native, packed]\n    if: always()\n' in aggregate, 'aggregate must always need native+packed')
    require('      REQUIRED_RESULTS: ${{ toJSON(needs) }}\n' in aggregate, 'actual needs results required')
    for body in (packed, aggregate):
        require('          ref: ${{ github.event.pull_request.head.sha || github.sha }}\n' in body and
                '          persist-credentials: false\n' in body, 'exact clean checkout')
        require(not re.search(r'\|\|\s*(?:true|:)|exit 0|set \+e', body), 'masked failure')
        require(all(line.strip() == 'if: always()' for line in body.splitlines() if re.match(r'\s*if:', line)), 'skipped step')
    require('          go-version: \'1.25.13\'' in packed and "          node-version: '22.23.2'" in packed, 'tool pins')
    require('        run: python3 -B scripts/run-packed-ci.py "$PACKED_ROOT" "$EXPECTED_HEAD"\n' in packed and
        '        run: python3 -B scripts/check-packed-ci.py "$PACKED_ROOT" "$EXPECTED_HEAD"\n' in packed, 'runner/checker required')
    require('          python3 -B scripts/check-packed-workflow.py\n' in aggregate and
        "          python3 -B -m unittest discover -s scripts -p 'test_packed_ci.py'\n" in aggregate and
        '        run: python3 -B scripts/check-packed-workflow.py --results "$REQUIRED_RESULTS"\n' in aggregate, 'aggregate controls/results required')
    for phase, flag in (('discovery', '-list'), ('planner', '-run')):
        pattern = rf"run\('{phase}', \[go, 'test', '-p=2', '-tags=packedci', '-json'.*?'{flag}', proof.REGEX, proof.PACKAGE_PATH\]"
        require(re.search(pattern, runner), 'explicit tagged exact ' + phase)
    require('proof.check(root, sha)' in runner, 'terminal checker invocation')
    require('go test -p 2 -json -count=1 -timeout=12m "${selected[@]}"' in native and not re.search(r'\s-run(?:[ =]|$)', native), 'ordinary native selection must remain unfiltered')
    require('-tags=packedci' not in native and 'GOFLAGS' not in packed, 'tag must remain opt-in')


if __name__ == '__main__':
    if len(sys.argv) == 3 and sys.argv[1] == '--results':
        results(json.loads(sys.argv[2]))
    else:
        repo = Path(__file__).resolve().parent.parent
        check((repo / '.github/workflows/authoring-native.yml').read_text(),
              (repo / 'scripts/run-packed-ci.py').read_text())
