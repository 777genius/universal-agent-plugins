'use strict';
if (process.env.AGENTPLUGINS_STAGED_TEST_CHILD === "1") {
  require("node:test")("source-checkout-only suite", { skip: "requires the complete repository source tree" }, () => {});
} else {
// Source contracts only. These tests never dispatch, sign, or qualify E.
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const childProcess = require('node:child_process');
const Module = require('node:module');
const a = require('../scripts/public-authoring-acceptance');
const text = fs.readFileSync(path.resolve(__dirname, '../../../.github/workflows/authoring-public-packed.yml'), 'utf8');
const jobs = Object.fromEntries([...text.matchAll(/^  (public_[a-z_]+):\n([\s\S]*?)(?=^  public_[a-z_]+:|$(?![\s\S]))/gm)]
  .map(m => [m[1], m[2]]));
const encode = value => JSON.stringify(value, null, 2) + '\n';
const digest = value => crypto.createHash('sha256').update(value).digest('hex');
const locator = n => ({ sha256: digest(`payload-${n}`), artifact: {
  run_id: n + 1, run_attempt: 1, artifact_id: n + 101, artifact_sha256: digest(`artifact-${n}`)
} });
const selected = () => ({ tag: 'agentplugins-v2.1.0', ref: 'refs/tags/agentplugins-v2.1.0', source: 'a'.repeat(40),
  versions: { agentplugins: '2.1.0', 'plugin-kit-ai': '2.0.0' } });
const dispatch = mode => {
  const values = {
    selected: selected(),
    input: locator(1), stage: locator(2), journeys: a.matrix.map((row, i) => ({ cell: row.key, ...locator(i + 10) })),
    bridge: locator(40), assembly: locator(41), acceptance: locator(42)
  };
  const required = { inputs: [], produce: [], assemble: ['journeys', 'bridge'], attest: ['assembly'], read: ['assembly', 'acceptance'] }[mode];
  const env = Object.fromEntries(['selected', 'input', 'stage', 'journeys', 'bridge', 'assembly', 'acceptance'].map(name => [`PUBLIC_${name.toUpperCase()}`, '']));
  for (const name of ['selected', 'input', 'stage', ...required]) env[`PUBLIC_${name.toUpperCase()}`] = encode(values[name]);
  return env;
};

test('C3 runtime rejects every open, missing, and malformed dispatch before effects', () => {
  const owned = ['selected', 'input', 'stage', 'journeys', 'bridge', 'assembly', 'acceptance'];
  const required = { inputs: ['selected', 'input', 'stage'], produce: ['selected', 'input', 'stage'],
    assemble: ['selected', 'input', 'stage', 'journeys', 'bridge'], attest: ['selected', 'input', 'stage', 'assembly'],
    read: ['selected', 'input', 'stage', 'assembly', 'acceptance'] };
  const savedEnv = { ...process.env }, load = Module._load;
  const originalFs = Object.fromEntries(['mkdirSync', 'mkdtempSync', 'writeFileSync', 'rmSync'].map(name => [name, fs[name]]));
  const originalChild = Object.fromEntries(['spawn', 'spawnSync', 'execFileSync'].map(name => [name, childProcess[name]]));
  const effects = { controller: 0, facade: 0, filesystem: 0, subprocess: 0 };
  Module._load = function(request, parent, isMain) {
    if (request.includes('public-authoring-tools')) effects.controller++;
    if (request.includes('public-authoring-custody')) effects.facade++;
    return load.call(this, request, parent, isMain);
  };
  for (const name of Object.keys(originalFs)) fs[name] = (...args) => { effects.filesystem++; return originalFs[name](...args); };
  for (const name of Object.keys(originalChild)) childProcess[name] = (...args) => { effects.subprocess++; return originalChild[name](...args); };
  try {
    for (const [mode, names] of Object.entries(required)) {
      const cases = [];
      process.env = { ...savedEnv, GITHUB_WORKFLOW_SHA: 'a'.repeat(40), ...dispatch(mode) };
      assert.doesNotThrow(() => a.dispatchInputs(mode), `${mode}: valid closed dispatch`);
      for (const foreign of owned.filter(name => !names.includes(name))) cases.push([`foreign ${foreign}`, env => { env[`PUBLIC_${foreign.toUpperCase()}`] = encode(locator(90)); }]);
      for (const missing of names) cases.push([`missing ${missing}`, env => { delete env[`PUBLIC_${missing.toUpperCase()}`]; }]);
      for (const malformed of names) cases.push([`malformed ${malformed}`, env => { env[`PUBLIC_${malformed.toUpperCase()}`] = encode({}); }]);
      const selectedCases = [
        ['versions null', s => { s.versions = null; }],
        ['missing version', s => { delete s.versions.agentplugins; }],
        ['extra version', s => { s.versions.extra = '1.0.0'; }],
        ['non-string version', s => { s.versions.agentplugins = 210; }],
        ['oversized version', s => { s.versions.agentplugins = '1'.repeat(33); }],
        ['wrong kit version', s => { s.versions['plugin-kit-ai'] = '2.0.1'; }],
        ['wrong tag', s => { s.tag = 'agentplugins-v9.9.9'; }],
        ['wrong ref', s => { s.ref = 'refs/tags/agentplugins-v9.9.9'; }],
        ['wrong workflow source', s => { s.source = 'b'.repeat(40); }]
      ];
      for (const [label, mutate] of selectedCases) cases.push([label, env => { const s = selected(); mutate(s); env.PUBLIC_SELECTED = encode(s); }]);
      cases.push(['duplicate selected key', env => { env.PUBLIC_SELECTED = env.PUBLIC_SELECTED.replace('  "tag":', '  "tag": "agentplugins-v2.1.0",\n  "tag":'); }]);
      cases.push(['non-canonical selected JSON', env => { env.PUBLIC_SELECTED = JSON.stringify(selected()); }]);
      for (const [label, mutate] of cases) {
        process.env = { ...savedEnv, GITHUB_WORKFLOW_SHA: 'a'.repeat(40), ...dispatch(mode) }; mutate(process.env);
        assert.throws(() => a.workflowRequest(mode, mode === 'produce' ? a.matrix[0].key : undefined), `${mode}: ${label}`);
        assert.deepEqual(effects, { controller: 0, facade: 0, filesystem: 0, subprocess: 0 }, `${mode}: ${label}`);
      }
    }
  } finally {
    process.env = savedEnv; Module._load = load;
    for (const [name, fn] of Object.entries(originalFs)) fs[name] = fn;
    for (const [name, fn] of Object.entries(originalChild)) childProcess[name] = fn;
  }
});

test('C3 workflow four invocations retain all cells and exact completed jobs', () => {
  assert.deepEqual(Object.keys(jobs), ['public_inputs', 'public_cell', 'public_producer_complete', 'public_assemble',
    'public_evidence_intake', 'public_evidence_attestation', 'public_check']);
  assert.match(text, /options: \[produce, assemble, attest, check\]/);
  assert.match(jobs.public_cell, /fail-fast: false/);
  assert.match(jobs.public_cell, /needs: public_inputs/);
  assert.match(jobs.public_cell, /name: public_cell \(\$\{\{ matrix.cell \}\}\)/);
  assert.match(jobs.public_cell, /matrix: \$\{\{ fromJSON\(needs.public_inputs.outputs.matrix\) \}\}/);
  assert.match(jobs.public_producer_complete, /needs: \[public_inputs, public_cell\]/);
  assert.match(jobs.public_producer_complete, /== os.environ\['CELLS_RESULT'\] == 'success'/);
  assert.equal(a.PUBLIC_JOBS.produce.length, 20);
  assert.equal(new Set(a.matrix.map(r => r.key)).size, 18);
  for (const [mode, names] of Object.entries(a.PUBLIC_JOBS)) {
    const producer = { workflow: a.WORKFLOW, source: 'a'.repeat(40), ref: 'refs/tags/agentplugins-v2.0.0', run_id: 50, run_attempt: 2 };
    const selected = { tag: 'agentplugins-v2.0.0', ref: producer.ref, source: producer.source, versions: {} };
    const attempt = { producer, status: 'completed', conclusion: 'success', jobs: names.map((name, i) => ({
      id: i + 1, name, run_id: 50, run_attempt: 2, source: producer.source, ref: producer.ref, status: 'completed', conclusion: 'success'
    })) };
    assert.deepEqual(a.completedAttempt(attempt, selected, mode), producer);
    for (let i = 0; i < names.length; i++) for (const field of ['run_attempt', 'source', 'status', 'conclusion']) {
      const bad = structuredClone(attempt);
      bad.jobs[i][field] = { run_attempt: 1, source: 'b'.repeat(40), status: 'in_progress', conclusion: 'skipped' }[field];
      assert.throws(() => a.completedAttempt(bad, selected, mode), `${mode}/${names[i]}/${field}`);
    }
  }
});

test('C3 workflow isolates E2 signing and rechecks closure after signing', () => {
  for (const [name, body] of Object.entries(jobs)) {
    if (name === 'public_evidence_attestation') continue;
    assert.doesNotMatch(body, /id-token: write|attestations: write|actions\/attest@/);
  }
  const signer = jobs.public_evidence_attestation;
  assert.match(signer, /needs: public_evidence_intake/);
  assert.match(signer, /runs-on: ubuntu-24.04/);
  assert.match(signer, /id-token: write\n      attestations: write/);
  assert.match(signer, /subject-path: \$\{\{ steps.evidence.outputs.subjects \}\}/);
  assert.ok(signer.indexOf("packed.public_workflow('attest')") < signer.indexOf('uses: actions/attest@'));
  assert.ok(signer.indexOf('uses: actions/attest@') < signer.indexOf("checked = packed.public_workflow('attest')"));
  assert.ok(signer.indexOf('assert closure(') < signer.indexOf('uses: actions/upload-artifact@'));
  assert.match(signer, /assert closure\(os.environ\['PUBLIC_SIGNED_ROOT'\]\) == closure\(checked\['root'\]\)/);
  assert.doesNotMatch(text, /contents: write|packages: write|gh release|npm publish|workflow_call:|push:/);
});

test('C3 workflow pins source actions and independent controller bootstrap', () => {
  const uses = [...text.matchAll(/uses: ([^\s]+)/g)].map(m => m[1]);
  assert.ok(uses.length > 0);
  for (const use of uses) assert.match(use, /^actions\/(checkout|upload-artifact|attest)@[0-9a-f]{40}$/);
  for (const [name, body] of Object.entries(jobs)) {
    if (name === 'public_producer_complete') continue;
    assert.match(body, /persist-credentials: false/);
    assert.match(body, /s\['source'\] == os.environ\['GITHUB_SHA'\] == os.environ\['GITHUB_WORKFLOW_SHA'\]/);
    assert.match(body, /shell: python -I -B \{0\}/);
    assert.match(body, /packed.public_workflow\(/);
    assert.doesNotMatch(body, /setup-node|node-version|node npm\//);
  }
  assert.doesNotMatch(jobs.public_assemble, /needs:/);
  assert.doesNotMatch(jobs.public_evidence_intake, /needs:/);
  assert.doesNotMatch(jobs.public_check, /needs:/);
});

test('C3 trusted bootstrap closes and validates dispatch inputs before checkout', () => {
  const admitted = Object.entries(jobs).filter(([name]) => name !== 'public_producer_complete');
  assert.equal(admitted.length, 6);
  for (const [name, body] of admitted) {
    const checkout = body.indexOf('uses: actions/checkout@');
    assert.ok(checkout > 0, name);
    const bootstrap = body.slice(0, checkout);
    assert.match(bootstrap, /bool\(body\) == \(name in required\)/, name);
    assert.match(bootstrap, /object_pairs_hook=unique/, name);
    assert.match(bootstrap, /raw\[name\] == json\.dumps\(parsed, ensure_ascii=False, indent=2, separators=/, name);
    assert.match(bootstrap, /def locator\(v\):/, name);
    assert.match(bootstrap, /locator\(value\('input'\)\); locator\(value\('stage'\)\)/, name);
    assert.match(bootstrap, /if mode == 'assemble':/, name);
    assert.match(bootstrap, /if mode in \('attest', 'check'\): locator\(value\('assembly'\)\)/, name);
    assert.match(bootstrap, /if mode == 'check': locator\(value\('acceptance'\)\)/, name);
    assert.match(bootstrap, /set\(s\['versions'\]\) == \{'agentplugins', 'plugin-kit-ai'\}/, name);
    assert.match(bootstrap, /s\['tag'\] == 'agentplugins-v' \+ s\['versions'\]\['agentplugins'\]/, name);
  }
});

test('C3 trusted pre-checkout bootstraps reject the selected-source semantic bypass', () => {
  const modes = { public_inputs: 'produce', public_cell: 'produce', public_assemble: 'assemble',
    public_evidence_intake: 'attest', public_evidence_attestation: 'attest', public_check: 'check' };
  const cases = [
    ['versions null', s => { s.versions = null; }],
    ['missing version', s => { delete s.versions.agentplugins; }],
    ['extra version', s => { s.versions.extra = '1.0.0'; }],
    ['non-string version', s => { s.versions.agentplugins = 210; }],
    ['oversized version', s => { s.versions.agentplugins = '1'.repeat(33); }],
    ['wrong kit value', s => { s.versions['plugin-kit-ai'] = '2.0.1'; }],
    ['wrong tag', s => { s.tag = 'agentplugins-v9.9.9'; }],
    ['wrong ref', s => { s.ref = 'refs/tags/agentplugins-v9.9.9'; }]
  ];
  for (const [job, mode] of Object.entries(modes)) {
    const bootstrap = jobs[job].slice(0, jobs[job].indexOf('uses: actions/checkout@'));
    const script = bootstrap.match(/run: \|\n([\s\S]*?)(?=^      - )/m)[1].replace(/^ {10}/gm, '');
    const base = { ...process.env, ...dispatch(mode === 'check' ? 'read' : mode), PUBLIC_MODE: mode,
      GITHUB_REPOSITORY: '777genius/universal-agent-plugins', GITHUB_EVENT_NAME: 'workflow_dispatch',
      GITHUB_SHA: 'a'.repeat(40), GITHUB_WORKFLOW_SHA: 'a'.repeat(40),
      GITHUB_REF: 'refs/tags/agentplugins-v2.1.0' };
    assert.equal(childProcess.spawnSync('python3', ['-I', '-B', '-'], { input: script, env: base }).status, 0, `${job}: valid`);
    for (const [label, mutate] of cases) {
      const s = selected(); mutate(s);
      const result = childProcess.spawnSync('python3', ['-I', '-B', '-'], { input: script, env: { ...base, PUBLIC_SELECTED: encode(s) } });
      assert.notEqual(result.status, 0, `${job}: ${label}`);
    }
    for (const [label, raw] of [
      ['duplicate key', encode(selected()).replace('  "tag":', '  "tag": "agentplugins-v2.1.0",\n  "tag":')],
      ['non-canonical JSON', JSON.stringify(selected())]
    ]) {
      const result = childProcess.spawnSync('python3', ['-I', '-B', '-'], { input: script, env: { ...base, PUBLIC_SELECTED: raw } });
      assert.notEqual(result.status, 0, `${job}: ${label}`);
    }
  }
});
}
