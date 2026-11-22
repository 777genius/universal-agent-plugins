import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { delimiter, join, resolve } from 'node:path';
import test from 'node:test';

// Exercise the actual evidence runner with a disposable process adapter, never
// npm, network, native clients or the user's HOME. These are harness tests only.
const adapter = `#!${process.execPath}
const fs = require('node:fs'), p = require('node:path');
const args = process.argv.slice(4), mode = process.env.MOCK_MODE;
const command = args[0], root = process.cwd(), marker = p.join(root, 'mock-identity');
const envelope = (command, data) => console.log(JSON.stringify({schema_version:1,command,result:'success',data}));
if (command === 'version') { envelope('version', {version:'mock-version'}); process.exit(0); }
if (command === 'skills') {
 const skill = args[args.indexOf('--skill')+1];
 if(skill==='frontend-design' && args[2]!=='https://github.com/anthropics/skills') throw Error('Expected captured root source');
 const native = p.join(root,'.agents','skills',skill);
 fs.mkdirSync(native,{recursive:true}); fs.writeFileSync(p.join(native,'SKILL.md'),'mock Skill');
 fs.writeFileSync(marker,skill); fs.writeFileSync(p.join(root,'uap.toml'),'mock requirements');
 const digest = 'sha256:'+'a'.repeat(64);
 const website = skill==='ui-radar', url='https://uizze.com';
 const source = website ? {web:{url,skill,index_url:url+'/.well-known/agent-skills/index.json',index_sha256:'a'.repeat(64),raw_tree_digest:digest,normalizer:'agent-skill-package/v1',files:[{path:'SKILL.md',url:url+'/SKILL.md',sha256:'a'.repeat(64)}]}} : {git:{repository:'anthropics/skills',kind:'skill',skill,path:'skills/frontend-design',resolved_skill_path:'skills/frontend-design',commit:'b'.repeat(40),raw_tree_digest:digest,normalizer:'agent-skill-package/v1'}};
 const sourceKey = website?'https-skill:'+url+':'+skill:'github-skill:anthropics/skills:'+skill+':skills/frontend-design';
 fs.writeFileSync(p.join(root,'uap.lock'),JSON.stringify({lock_version:1,packages:[{alias:mode==='identity'?'other':skill,source,source_key:mode==='wrong-source'?'wrong-source':sourceKey,tree_digest:digest,manifest_digest:digest,targets:['codex']}]}));
 if(mode==='json') console.log('not JSON');
 else console.log(JSON.stringify({schema_version:1,command:'skills add',result:'success',status:'succeeded',readiness:mode==='reported-ready'?'ready':'needs_action',items:[{alias:skill,targetId:'codex',status:'succeeded'}]}));
} else if(command === 'info') {
 const skill=fs.readFileSync(marker,'utf8');
 envelope('info',{name:skill,source:skill==='ui-radar'?'https://uizze.com':'anthropics/skills/skills/frontend-design',installation_id:'verified-id',clients:[{client_id:'codex',package_revision:{tree_digest:'sha256:'+'a'.repeat(64),manifest_digest:'sha256:'+'a'.repeat(64),resolved_revision:skill==='ui-radar'?undefined:'b'.repeat(40)},native_discovery_reconciled:mode==='reported-ready'}]});
} else if(command === 'remove') {
 if(args[1]!=='verified-id') throw Error('unverified selector');
 const skill=fs.readFileSync(marker,'utf8');
 if(mode==='remove-error'){console.error('refused');process.exit(2);}
 if(mode!=='leftover') fs.rmSync(p.join(root,'.agents','skills',skill),{recursive:true});
 if(mode==='foreign') fs.rmSync(p.join(root,'foreign.txt'));
 fs.writeFileSync(p.join(root,'uap.lock'),JSON.stringify({lock_version:1,packages:[]}));
 console.log(JSON.stringify({schema_version:1,command:'agentplugins remove',result:'success',outcome:'completed'}));
} else if(command === 'deps') console.log(JSON.stringify({facts:[],diagnostics:mode==='stale-requirement'?[{code:'manifest_mismatch'}]:[],manifest_digest:'sha256:'+'a'.repeat(64),lock_digest:'sha256:'+'a'.repeat(64),pending_recovery:false}));
else if(command === 'list') envelope('list',{installations:[]});
else throw Error('unexpected command '+args.join(' '));
`;
for (const mode of ['success', 'reported-ready', 'wrong-source', 'stale-requirement', 'json', 'identity', 'remove-error', 'leftover', 'foreign']) {
  test(`published lifecycle harness: ${mode}`, () => {
    const root = mkdtempSync(join(tmpdir(), 'uap-lifecycle-test-'));
    try {
      const bin = join(root, 'bin'); mkdirSync(bin);
      const npx = join(bin, 'npx'); writeFileSync(npx, adapter); chmodSync(npx, 0o700);
      const temp = join(root, 'tmp'); mkdirSync(temp);
      const home = join(root, 'home'); mkdirSync(home);
      const report = join(root, 'report');
      const output = spawnSync(process.execPath, ['--experimental-strip-types', resolve('landing/scripts/skills-install-native-live-e2e.ts')], {
        env: { ...process.env, PATH: bin + delimiter + process.env.PATH, HOME: home, CODEX_HOME: join(home, '.codex'),
          XDG_CONFIG_HOME: join(home, 'config'), XDG_CACHE_HOME: join(home, 'cache'), XDG_DATA_HOME: join(home, 'data'),
          TMPDIR: temp, TMP: temp, TEMP: temp, UAP_E2E_LIVE_SKILLS: '1', UAP_E2E_PUBLISHED_CLI: '1',
          UAP_SKILLS_INSTALL_REPORT_DIR: report, MOCK_MODE: mode }, encoding: 'utf8', timeout: 20000,
      });
      assert.ifError(output.error);
      const evidence = JSON.parse(readFileSync(join(report, 'npm-install.json'), 'utf8'));
      assert.equal(evidence.results.length, 2);
      assert.equal(evidence.results[0].source, 'https://github.com/anthropics/skills');
      assert.equal(evidence.results[0].id, 'skills-sh:anthropics/skills:frontend-design');
      assert.equal(evidence.results[0].command, 'npx -y universal-agent-plugins@latest skills add https://github.com/anthropics/skills --skill frontend-design');
      assert.deepEqual(evidence.historical_capture.commands[evidence.results[0].id], evidence.results[0].command);
      assert.equal(output.status === 0, ['success', 'reported-ready'].includes(mode), output.stderr);
      for (const result of evidence.results) {
        assert.equal(result.lifecycle.removed, ['success', 'reported-ready'].includes(mode));
        assert.match(result.lifecycle.version_before.stdout, /mock-version/);
        assert.equal(result.lifecycle.observed_version_before, 'mock-version');
        assert.equal(result.lifecycle.version_observations_match, true);
        if (['success', 'reported-ready'].includes(mode)) {
          assert.equal(result.lifecycle.native_qualified, false, 'files must not imply discovery');
          assert.equal(result.lifecycle.installation_id, 'verified-id');
          assert.equal(result.lifecycle.remove.exit_code, 0);
        } else assert.ok(result.lifecycle.error);
      }
    } finally { rmSync(root, { recursive: true, force: true }); }
  });
}
