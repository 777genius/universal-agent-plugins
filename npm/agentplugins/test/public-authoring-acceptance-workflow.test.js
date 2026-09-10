'use strict';
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
const dispatch = mode => {
  const values = {
    selected: { tag: 'agentplugins-v2.0.0', ref: 'refs/tags/agentplugins-v2.0.0', source: 'a'.repeat(40), versions: {} },
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
      process.env = { ...savedEnv, ...dispatch(mode) };
      assert.doesNotThrow(() => a.dispatchInputs(mode), `${mode}: valid closed dispatch`);
      for (const foreign of owned.filter(name => !names.includes(name))) cases.push([`foreign ${foreign}`, env => { env[`PUBLIC_${foreign.toUpperCase()}`] = encode(locator(90)); }]);
      for (const missing of names) cases.push([`missing ${missing}`, env => { delete env[`PUBLIC_${missing.toUpperCase()}`]; }]);
      for (const malformed of names) cases.push([`malformed ${malformed}`, env => { env[`PUBLIC_${malformed.toUpperCase()}`] = encode({}); }]);
      for (const [label, mutate] of cases) {
        process.env = { ...savedEnv, ...dispatch(mode) }; mutate(process.env);
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
    assert.match(bootstrap, /def locator\(v\):/, name);
    assert.match(bootstrap, /locator\(value\('input'\)\); locator\(value\('stage'\)\)/, name);
    assert.match(bootstrap, /if mode == 'assemble':/, name);
    assert.match(bootstrap, /if mode in \('attest', 'check'\): locator\(value\('assembly'\)\)/, name);
    assert.match(bootstrap, /if mode == 'check': locator\(value\('acceptance'\)\)/, name);
  }
});
