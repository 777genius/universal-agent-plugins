#!/usr/bin/env python3
"""Structural guard for this one workflow, not hosted required-check enforcement."""
import json
from pathlib import Path
import re
import sys


def require(ok, message):
    if not ok: raise ValueError(message)


# The only permitted condition on the lanes: they run unless the shared pull
# request classifier succeeded and reported a documentation-only diff.
GATE = ("    needs: changes\n"
        "    if: ${{ !cancelled() && (needs.changes.result != 'success' || needs.changes.outputs.authoring != 'false') }}\n")
CLASSIFIER = ("    if: ${{ github.event_name == 'pull_request' }}\n"
              "    permissions:\n      pull-requests: read\n"
              "    uses: ./.github/workflows/pr-changes.yml\n")


def results(value):
    require(set(value) == {'changes', 'native', 'packed'}, 'changes, native and packed results required')
    changes = value['changes'] or {}
    lanes = [value[name].get('result') if isinstance(value[name], dict) else None for name in ('native', 'packed')]
    documentation_only = changes.get('result') == 'success' and (changes.get('outputs') or {}).get('authoring') == 'false'
    if documentation_only:
        require(lanes == ['skipped', 'skipped'], 'a documentation-only classification must skip both lanes')
    else:
        require(lanes == ['success', 'success'], 'native and packed must both exist and succeed')


def check(text, runner):
    # Fixed indentation is intentional: reject unsupported rewrites for review.
    require('\non:\n  pull_request:\n  workflow_dispatch:\n' in text, 'unfiltered PR and dispatch required')
    require(not re.search(r'^\s*(?:paths(?:-ignore)?|branches(?:-ignore)?|continue-on-error):', text, re.M), 'success override/filter')
    jobs = dict(re.findall(r'^  (\w+):\n(.*?)(?=^  \w+:\n|\Z)', text.split('\njobs:\n', 1)[1], re.M | re.S))
    require(set(jobs) == {'changes', 'native', 'packed', 'acceptance'}, 'required jobs missing/renamed')
    native, packed, aggregate = (jobs[n] for n in ('native', 'packed', 'acceptance'))
    require(jobs['changes'].strip('\n') + '\n' == CLASSIFIER, 'shared pull request classifier required')
    for body in (native, packed):
        require(body.count(GATE) == 1 and len(re.findall(r'^    (?:if|needs):', body, re.M)) == 2,
                'lanes may only be gated by the fail-open documentation classifier')
    # github/docs contexts.md: jobs.<job_id>.env excludes runner (steps.env allows it).
    for body in jobs.values():
        job_env = re.search(r'^    env:\n((?:[ \t]+.*\n|\n)*?)(?=^    \S|\Z)', body, re.M)
        if job_env:
            require(not re.search(r'\$\{\{[^}]*\brunner\s*(?:\.|\[)', job_env[1]),
                    'runner context is unavailable in job env')
    initialization = """      - name: Resolve disposable packed evidence path
        shell: bash
        run: |
          set -euo pipefail
          # Publish the path before checkout/setup so always() consumers retain it.
          # Leave creation (0700) and stale-root rejection to run-packed-ci.py.
          printf 'PACKED_ROOT=%s/authoring-packed-%s-%s\\n' "$RUNNER_TEMP" "$GITHUB_RUN_ID" "$GITHUB_RUN_ATTEMPT" >> "$GITHUB_ENV"
"""
    require('    steps:\n' + initialization in packed, 'publish packed path first without creating root')
    require('      PACKED_ROOT:' not in packed, 'packed path must come from runtime environment')
    require('      - name: Require terminal packed evidence\n        if: always()\n' in packed and
            '      - name: Preserve packed evidence including failure logs and sealed inputs\n        if: always()\n' in packed and
            '            ${{ env.PACKED_ROOT }}/**\n' in packed and
            '          if-no-files-found: error\n' in packed, 'always checker/upload required')
    matrix = re.findall(r'- \{runner: ([\w.-]+), os: (\w+), arch: (\w+)\}', native)
    require(set(matrix) == {('ubuntu-24.04', 'linux', 'amd64'), ('ubuntu-24.04-arm', 'linux', 'arm64'),
        ('windows-2022', 'windows', 'amd64'), ('windows-11-arm', 'windows', 'arm64')} and len(matrix) == 4, 'four native lanes required')
    require('    needs: [changes, native, packed]\n    if: always() && !cancelled()\n' in aggregate,
            'aggregate must need the classifier and both lanes without running after cancellation')
    require('      REQUIRED_RESULTS: ${{ toJSON(needs) }}\n' in aggregate, 'actual needs results required')
    for body in (packed, aggregate):
        require('          ref: ${{ github.event.pull_request.head.sha || github.sha }}\n' in body and
                '          persist-credentials: false\n' in body, 'exact clean checkout')
        require(not re.search(r'\|\|\s*(?:true|:)|exit 0|set \+e', body), 'masked failure')
        # The status-only aggregate skips cancelled runs; its individual steps
        # still run after failure, as do the packed evidence checker and upload.
        step_body = body.replace('    if: always() && !cancelled()\n', '', 1) if body == aggregate else body.replace(GATE, '', 1)
        require(all(line.strip() == 'if: always()' for line in step_body.splitlines() if re.match(r'\s*if:', line)), 'skipped step')
    require('          go-version: \'1.26.10\'' in packed and "          node-version: '22.23.2'" in packed, 'tool pins')
    require('        run: python3 -B scripts/run-packed-ci.py "$PACKED_ROOT" "$EXPECTED_HEAD"\n' in packed and
        '        run: python3 -B scripts/check-packed-ci.py "$PACKED_ROOT" "$EXPECTED_HEAD"\n' in packed, 'runner/checker required')
    require('          python3 -B scripts/check-packed-workflow.py\n' in aggregate and
        "          python3 -B -m unittest discover -s scripts -p 'test_packed_ci.py'\n" in aggregate and
        '        run: python3 -B scripts/check-packed-workflow.py --results "$REQUIRED_RESULTS"\n' in aggregate, 'aggregate controls/results required')
    for phase, flag in (('discovery', '-list'), ('planner', '-run')):
        pattern = rf"run\('{phase}', \[go, 'test', '-p=2', '-tags=packedci', '-json'.*?'{flag}', proof.REGEX, proof.PACKAGE_PATH\]"
        require(re.search(pattern, runner), 'explicit tagged exact ' + phase)
    require('proof.check(root, sha)' in runner, 'terminal checker invocation')
    supplemental = """          if [[ "$EXPECTED_OS" == windows ]]; then
            go test -p 2 -json -count=1 -run '^TestDigest(KnownFormatParityAndNoOutput|FreshBytesAndExecutableIntent)$' ./install/integrationctl/packagesnapshot > "$NATIVE_ROOT/evidence/digest-windows.json" 2> "$NATIVE_ROOT/evidence/digest-windows.stderr"
            digest_result=$?
            if [[ "$digest_result" != 0 ]]; then result=$digest_result; fi
          fi
"""
    terminal = """          printf '%s\\n' "$result" > "$NATIVE_ROOT/evidence/test-exit.txt"
          exit "$result"
"""
    require(native.count(supplemental) == 1 and supplemental + terminal in native,
            'exact Windows digest supplement and exit propagation required')
    ordinary_native = native.replace(supplemental, '', 1)
    require('go test -p 2 -json -count=1 -timeout=12m "${selected[@]}"' in ordinary_native and not re.search(r'\s-run(?:[ =]|$)', ordinary_native), 'ordinary native selection must remain unfiltered')
    require('-tags=packedci' not in ordinary_native and 'GOFLAGS' not in packed, 'tag must remain opt-in')


if __name__ == '__main__':
    if len(sys.argv) == 3 and sys.argv[1] == '--results':
        results(json.loads(sys.argv[2]))
    else:
        repo = Path(__file__).resolve().parent.parent
        check((repo / '.github/workflows/authoring-native.yml').read_text(),
              (repo / 'scripts/run-packed-ci.py').read_text())
