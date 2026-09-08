"use strict";

// All executable responses below are test-local subprocess fixtures. No real
// frozen binary, installer, scanner, network service or OS observer is executed.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const os = require("node:os");
const vm = require("node:vm");
const cp = require("node:child_process");
const { createRequire } = require("node:module");
const c = require("../scripts/dual-authoring-candidate");
const release = require("../scripts/authoring-release");
const native = require("../scripts/authoring-native-qualification");
const promotion = require("../scripts/authoring-promotion");
const moduleFile = path.resolve(__dirname, "../scripts/authoring-native-qualification.js");
const hash = s => c.digest(Buffer.from(s));
const ID = { repository: c.REPOSITORY, commit: "a".repeat(40), engine_revision: "a".repeat(40),
  versions: { agentplugins: "0.1.54", "plugin-kit-ai": "2.0.0" } };
const invocation = workflow => ({ repository: c.REPOSITORY, workflow, source: ID.commit,
  workflow_sha: ID.commit, run_id: 42, run_attempt: 2 });

function fixture() {
  // Reuse only the established text/ustar fixture construction, never its tests
  // or provider seam. These bytes are intentionally not native executables.
  const source = fs.readFileSync(path.join(__dirname, "authoring-promotion.test.js"), "utf8");
  const prefix = source.slice(0, source.indexOf("function expected("));
  const Module = require("node:module"), original = path.join(__dirname, "authoring-promotion.test.js");
  const instance = new Module(original, module);
  instance.filename = original; instance.paths = Module._nodeModulePaths(__dirname);
  instance._compile(prefix + "\nmodule.exports = fixture;", original);
  const f = instance.exports();
  f.pins = { identity: structuredClone(ID), candidate_sha256: f.record.candidate_sha256,
    pair_marker_sha256: f.record.pair_marker_sha256, products: Object.fromEntries(c.PRODUCTS.map(p => [p, {
      manifest_sha256: f.record.products[p].manifest_sha256, checksums_sha256: f.record.products[p].checksums_sha256 }])) };
  const preparationProducer = invocation(".github/workflows/agentplugins-release.yml");
  const receipt = native.writePreparation(f.root, f.pins, preparationProducer);
  f.go = path.join(f.sandbox, "host-go"); fs.writeFileSync(f.go, "host Go fixture; not executed", { mode: 0o500 });
  f.options = { root: f.root, pins: f.pins, preparation: { sha256: receipt.sha256, producer: preparationProducer },
    producer: invocation(".github/workflows/authoring-frozen-native.yml"), go: f.go,
    go_sha256: c.digest(c.readFile(f.go)), output: path.join(f.sandbox, "evidence"), work: path.join(f.sandbox, "journey") };
  // Sibling outputs must not be inside the protected tool parent.
  const tools = path.join(f.sandbox, "tools"); fs.mkdirSync(tools);
  const moved = path.join(tools, "go"); fs.renameSync(f.go, moved); f.go = moved; f.options.go = moved;
  return f;
}
function internal(observation = false, fastTimeout = false) {
  let source = fs.readFileSync(moduleFile, "utf8");
  if (observation) source = source.replace('    observationGate(); //', '    /* Test-only synthetic observation; not a native pass. */ //');
  if (fastTimeout) source = source.replace('installer ? 120000 : 15000', '100');
  // Expose lexical contracts only in this test VM. Production exports no policy,
  // verifier, command inventory, child executable or success switch.
  source += '\nmodule.exports.test = { commands, installationCommands, verifyJourney, terminal, tree, canonical, context, subprocess, observationGate, jsonDocument, treeShape };';
  const Module = require("node:module");
  const instance = new Module(moduleFile, module);
  instance.filename = moduleFile; instance.paths = Module._nodeModulePaths(path.dirname(moduleFile));
  instance._compile(source, moduleFile);
  return instance.exports;
}
function noTerminal(f) {
  for (const p of c.PRODUCTS) assert.equal(fs.existsSync(path.join(f.options.output, `${p}-terminal.json`)), false);
}
function fixtureProgram() {
  // Deliberately independent response implementation with actual mkdir/write/
  // state mutations in each child. A zero status without those effects is tested.
  return String.raw`
const fs = require('node:fs'), path = require('node:path'), crypto = require('node:crypto');
const cfg = JSON.parse(fs.readFileSync(process.argv[2]));
const selected = process.argv[3], argv = process.argv.slice(4);
const sha = s => crypto.createHash('sha256').update(s).digest('hex');
const product = selected.includes('plugin-kit-ai') ? 'plugin-kit-ai' : 'agentplugins';
fs.appendFileSync(cfg.log, JSON.stringify({selected,argv,env:process.env})+'\n');
const scenario = cfg.scenario;
if (scenario === 'timeout') setInterval(()=>{},1000);
else if (scenario === 'flood') process.stdout.write('x'.repeat(2*1024*1024));
else if (scenario === 'cancel') setInterval(()=>{},1000);
else main();
function output(value, status=0) {
  if (scenario === 'invalid-utf8') process.stdout.write(Buffer.from([0xff]));
  else if (scenario === 'bad-json') process.stdout.write('{bad');
  else process.stdout.write(JSON.stringify(value)+'\n');
  process.exitCode=status;
}
function main() {
  if (selected === cfg.go) {
    if (argv[0]==='env') return output({GOVERSION:'go1.25.13',GOHOSTOS:'linux',GOHOSTARCH:'amd64'});
    const p = path.basename(argv[3]);
    const settings = {GOOS:'linux',GOARCH:'amd64',CGO_ENABLED:'0','-buildmode':'exe','-compiler':'gc',
      '-ldflags':cfg.linker[p]};
    if(scenario==='wrong-build-target') settings.GOARCH='arm64';
    if(scenario==='wrong-build-mode') settings['-buildmode']='pie';
    if(scenario==='wrong-build-source') settings['-ldflags']=settings['-ldflags'].replace(cfg.identity.commit,'b'.repeat(40));
    return output({GoVersion:'go1.25.13',Path:'github.com/777genius/plugin-kit-ai/cli/cmd/'+(scenario==='wrong-binary'?'peer':p),
      Settings:Object.entries(settings).map(([Key,Value])=>({Key,Value}))});
  }
  let args=argv.filter(x=>x!=='--format=json');
  if(args.length===1 && args[0]==='--help' && !argv.includes('--format=json')) { process.stdout.write('Usage: '+product+'\nAvailable commands: author init validate inspect test capabilities skills version doctor compat\n'); return; }
  const author=product==='plugin-kit-ai'||args[0]==='author';
  if(args[0]==='author') args.shift();
  if(!author) return installer(args);
  const verb=args[0], lane=args[1], op=verb==='skills'?'author.skills.'+args[1]:verb==='--help'?'author':'author.'+verb;
  const data={engine:'standard-first-slice/1',revision:cfg.identity.commit,authoring_schema_version:1,
    engine_version:'standard-first-slice/1',runtime_evidence:{status:'not_evaluated'},committed:false,effects:{committed:false},
    product,product_version:cfg.identity.versions[product]};
  const done=(status=0)=>output({schema_version:1,command:scenario==='wrong-command'&&verb==='validate'?'author.inspect':op,result:status?'failure':'success',data},status);
  if(scenario==='wrong-source') data.revision='b'.repeat(40);
  if(scenario==='runtime-claim') data.runtime_evidence.status='pass';
  if(verb==='version') {if(scenario==='wrong-version') data.product_version='9.9.9';return done();}
  if(verb==='update') {data.error={code:'v1_operation_unavailable'};return done(2);}
  if(verb==='--help'||verb==='capabilities') {
    data.commands=['author.capabilities','author.compat','author.doctor','author.init','author.inspect','author.skills.init','author.skills.validate','author.test','author.validate'];
    if(scenario==='missing-command') data.commands.pop();
    if(scenario==='deferred-command') data.commands.push('author.publish');
    if(verb==='--help')data.help={use:product==='agentplugins'?'agentplugins author':'plugin-kit-ai'};
    else data.capabilities={schemas:[{id:'fixture-only'}]};
    return done();
  }
  if(args.includes('--force')||args.includes('--scope=user')||lane==='missing-destination')return done(2);
  if(verb==='init'&&fs.existsSync(lane))return done(1);
  const root=verb==='skills'?args[1]==='init'?args[3]:args[2]:lane;
  if(verb==='init') {
    if(scenario==='missing-template'&&lane==='hybrid-stdio')return done();
    fs.mkdirSync(root,{mode:0o700});
    write(root,'plugin.json',{$schema:'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json',name:lane,version:'0.1.0',description:'Owned fixture'});
    write(root,'README.md','Owned fixture\n');write(root,'.gitignore','cache\n');
    if(lane==='skill'||lane.startsWith('hybrid-'))skill(root,lane);
    if(lane!=='skill')write(root,'mcp.json',{$schema:'https://agent-plugins.org/schemas/1.0.0/mcp.schema.json',mcpServers:{[lane]:{type:lane.endsWith('remote')?'streamable-http':'stdio'}}});
    if(lane.endsWith('stdio')) {
      write(root,'package.json',{dependencies:{'@modelcontextprotocol/sdk':'1.30.0'}});
      write(root,'package-lock.json',{packages:{'node_modules/@modelcontextprotocol/sdk':{version:'1.30.0',integrity:'sha512-xKd8OIzlqNzcqcNumGAa6g+PW2kjD5vrpcKOnfldAUPP3j7lnqMPwlTXQm8gF+UwH72z0lqaRbjr9hqGz0eITA=='}}});
    }
    if(scenario==='yaml')write(root,'plugin.yaml','legacy');
    data.committed=true;data.effects.committed=true;
  }
  if(verb==='skills'&&args[1]==='init'){skill(root,'extra-skill');data.committed=true;data.effects.committed=true;}
  const malformed=fs.existsSync(path.join(root,'skills/broken'));
  data.identity={read_profile:'packageview-local-linux-v1',tree_digest:'sha256:'+sha('tree '+root)};
  data.profiles=[{id:'agent-skills/2026-09-06',revision:'69ef37e9424c0a7ea9dd2293b559e43ec8176379',digest:'sha256:b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220'}];
  data.loadability={status:'pass'};data.normative_conformance={status:malformed?'fail':'pass'};
  data.authoring_readiness={status:malformed?'fail':'pass'};data.release_policy={status:'not_evaluated'};
  data.components=[{type:'skill',status:'pass'}];data.findings=malformed?[{severity:'error'}]:[];
  if(verb==='compat'){data.compatibility={status:'pass'};data.clients=[{client_id:'claude'},{client_id:'codex'}];}
  if(verb==='doctor'){data.toolchain={status:root==='skill'?'pass':'not_evaluated'};return done(root==='skill'?0:1);}
  if(verb==='inspect') {
    const count=root==='skill'||root.startsWith('hybrid-')?2:1;
    data.inspection={name:root,schema:'https://agent-plugins.org/schemas/1.0.0/plugin.schema.json',
      components:[...Array(count).fill({type:'skill'}),...(root==='skill'?[]:[{type:root.endsWith('remote')?'mcp_streamable-http':'mcp_stdio'}])]};
  }
  if(scenario==='changed-input'&&verb==='test')fs.appendFileSync(cfg.input,'changed');
  if(scenario==='changed-project'&&verb==='test')write(root,'unexpected','mutation');
  if(scenario==='wrong-result'&&verb==='validate')data.normative_conformance.status='fail';
  if(scenario==='parity'&&product==='plugin-kit-ai')data.findings.push({severity:'warning'});
  return done(malformed?1:0);
}
function write(root,name,value){fs.writeFileSync(path.join(root,name),typeof value==='string'?value:JSON.stringify(value)+'\n');}
function skill(root,name){fs.mkdirSync(path.join(root,'skills',name),{recursive:true,mode:0o700});write(root,'skills/'+name+'/SKILL.md','---\nname: '+name+'\ndescription: Owned fixture\n---\nInstructions\n');}
function security(source){return {scanner:{id:'lintai',version:'0.1.3'},scanned_files:4,evidence_source:source,outcome:'no_blocking_findings',subject:{tree_digest:'sha256:'+sha('tree'),manifest_digest:'sha256:'+sha('manifest')},report_digest:'sha256:'+sha('report')};}
function installer(args) {
  const verb=args[0], state=process.env.AGENTPLUGINS_HOME,client=path.join(process.env.HOME,'.codex');
  const data={}, result={schema_version:1,command:verb,result:'success',data};
  if(verb==='version'){data.version=scenario==='wrong-version'?'9.9.9':cfg.identity.versions.agentplugins;return output(result);}
  if(scenario==='scan-failure'){result.result='failure';return output(result,1);}
  if(verb==='add'&&args.includes('--dry-run')) {
    const lane=path.basename(args[1]), names=fs.readdirSync(path.join(args[1],'skills'));
    const components=names.map(name=>({kind:'skill',name,support:'native'}));
    if(lane!=='skill')components.push({kind:'mcp_server',name:lane,support:'native'});
    data.security=security('local_scan');data.tree_digest='sha256:'+sha('tree');data.manifest_digest='sha256:'+sha('manifest');
    data.dry_run=true;data.result={plan:{client_id:'codex',scope:'user',status:'manual_activation_required',components}};
    if(scenario==='wrong-plan')data.result.plan.components=[];
    if(scenario==='dry-run-effect')write(client,'unexpected','effect');
  } else if(verb==='add') {
    if(scenario==='add-failure'){result.result='failure';return output(result,1);}
    data.result={mutated:true,activation:{authentication:'not_checked'}};
    data.security=security('cache');data.tree_digest='sha256:'+sha('tree');data.manifest_digest='sha256:'+sha('manifest');
    if(scenario!=='no-state')write(state,'state-v2.json',{installed:true});
    if(scenario!=='no-effect')write(client,'installed','fixture projection');
    if(scenario==='auth-claim')data.result.activation.authentication_attested=true;
  } else if(verb==='info'){data.name='skill';data.version='0.1.0';data.clients=[{client_id:'codex',package_revision:{version:'0.1.0'}}];}
  else if(verb==='update')data.result={mutated:false,no_change:scenario!=='wrong-update'};
  else if(verb==='remove'){data.result={mutated:true};write(state,'state-v2.json',{installed:false});fs.unlinkSync(path.join(client,'installed'));}
  else if(verb==='list')data.installations=scenario==='remaining-installation'?[{name:'skill'}]:[];
  return output(result);
}
`;
}
function subprocessFixtures(t, f, scenario = "ok") {
  const script = path.join(f.sandbox, "child-fixture.js"), config = path.join(f.sandbox, "child-config.json");
  f.log = path.join(f.sandbox, "subprocesses.jsonl");
  fs.writeFileSync(script, fixtureProgram());
  fs.writeFileSync(config, JSON.stringify({ scenario, identity: ID, go: f.go, log: f.log,
    input: path.join(f.root, "candidate/candidate.json"), linker: Object.fromEntries(c.PRODUCTS.map(p => [p, c.linkerFlags(p, ID, "release-cli-contract-v1")])) }));
  const spawn = cp.spawn;
  t.mock.method(cp, "spawn", (file, args, options) => {
    assert.ok(file === f.go || c.PRODUCTS.some(p => file === path.join(f.options.work, p, p)), "only pinned tool or selected binary");
    assert.equal(options.shell, false); assert.equal(options.detached, true);
    for (const forbidden of ["GH_TOKEN", "GITHUB_TOKEN", "NODE_OPTIONS", "HTTPS_PROXY", "UAP_PROOF_MODE", "GOFLAGS"])
      assert.equal(options.env[forbidden], undefined, forbidden);
    return spawn(process.execPath, [script, config, file, ...args], options);
  });
}
function expectations(f) {
  return { producer: f.options.producer, preparation: f.options.preparation,
    tools: { go: { sha256: f.options.go_sha256, version: "go1.25.13" }, node: { sha256: c.digest(c.readFile(process.execPath)), version: process.version } } };
}

test("shared verifier consumes uploaded projections, not missing flat candidate assets", () => {
  const f = fixture(), before = release.verifyProjectedPair(f.root, f.pins);
  assert.equal(before.subjects.length, 18);
  assert.deepEqual(fs.readdirSync(path.join(f.root, "candidate")), ["candidate.json"]);
  for (const p of c.PRODUCTS) assert.equal(fs.readdirSync(path.join(f.root, p)).length, 8);
  assert.throws(() => c.frozenCandidate(path.join(f.root, "candidate"), ID, f.pins.candidate_sha256, "six-platform-pair", "release-cli-contract-v1"));
  assert.equal(native.readPreparation(f.root, f.pins, f.options.preparation).subjects.length, 18);
  assert.deepEqual(promotion.frozenSubjects(f.root, f.record), before.subjects);
});
for (const kind of ["candidate", "pair", "manifest", "checksum", "outer", "extra", "link", "hardlink", "identity", "mode"]) {
  test(`projected closure rejects ${kind}`, () => {
    const f = fixture(), p = "agentplugins";
    const file = kind === "candidate" ? path.join(f.root, "candidate/candidate.json") : kind === "pair" ? path.join(f.root, "pair-prepared.json") :
      path.join(f.root, p, kind === "manifest" ? "release-manifest.json" : kind === "checksum" ? "checksums.txt" : f.record.products[p].assets["linux-amd64"].file);
    if (kind === "identity") f.pins.identity.commit = "b".repeat(40);
    else if (kind === "mode") {const v=JSON.parse(c.readFile(path.join(f.root,"candidate/candidate.json")));v.build.authoring_mode="vertical-slice-v1";fs.writeFileSync(path.join(f.root,"candidate/candidate.json"),c.encode(v));f.pins.candidate_sha256=c.digest(c.encode(v));}
    else if (kind === "extra") fs.writeFileSync(path.join(f.root,p,"extra"),"extra");
    else if (kind === "link") {fs.renameSync(file,file+"-original");fs.symlinkSync(file+"-original",file);}
    else if (kind === "hardlink") fs.linkSync(file,path.join(f.sandbox,"alias"));
    else fs.appendFileSync(file,"changed");
    assert.throws(()=>release.verifyProjectedPair(f.root,f.pins)); noTerminal(f);
  });
}
for (const kind of ["missing", "attempt", "workflow", "source", "duplicate-key", "future-qualification", "subject"]) {
  test(`preparation receipt rejects ${kind}`, () => {
    const f=fixture(),file=path.join(f.root,"preparation-run.json");
    if(kind==="missing")fs.unlinkSync(file);
    else if(kind==="attempt")f.options.preparation.producer.run_attempt++;
    else if(kind==="workflow")f.options.preparation.producer.workflow=".github/workflows/other.yml";
    else if(kind==="source")f.options.preparation.producer.source="b".repeat(40);
    else {
      let v=JSON.parse(c.readFile(file));
      if(kind==="future-qualification")v.qualification_sha256=hash("future");
      if(kind==="subject")v.subjects.pop();
      let bytes=c.encode(v);if(kind==="duplicate-key")bytes=Buffer.from(bytes.toString().replace('{','{"schema":"duplicate",'));
      fs.chmodSync(file,0o600);fs.writeFileSync(file,bytes);f.options.preparation.sha256=c.digest(bytes);
    }
    assert.throws(()=>native.readPreparation(f.root,f.pins,f.options.preparation));noTerminal(f);
  });
}

test("fixed production orchestration and closed reader with subprocess fixtures only", async t => {
  const f=fixture();subprocessFixtures(t,f);
  const n=internal(true), result=await n.produce(f.options);
  assert.equal(result.length,2);
  const reread=native.readTerminals(f.options.output,f.root,f.pins,expectations(f));
  assert.equal(reread[0].lane,"agentplugins/linux-amd64"); assert.equal(reread[1].lane,"plugin-kit-ai/linux-amd64");
  assert.notDeepEqual(reread[0].subject,reread[1].subject);
  assert.deepEqual(reread[0].peer_subject,reread[1].subject);
  assert.equal(reread[0].assertions.installer.length,8);
  assert.equal(reread[0].assertions.commands.agentplugins.length,54);
  const calls=fs.readFileSync(f.log,"utf8").trim().split("\n").map(JSON.parse);
  assert.ok(calls.some(x=>x.argv.includes('--dry-run')));
  assert.ok(calls.some(x=>x.argv[0]==='remove'));
  assert.ok(calls.every(x=>!x.argv.includes('--auth-complete')&&!x.argv.includes('--accept-security-risk')));
  assert.throws(()=>promotion.requireNativeContracts(result),/unknown or duplicate terminal lane|NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
});
for (const scenario of ["wrong-binary","wrong-build-target","wrong-build-mode","wrong-build-source","wrong-version","wrong-source",
  "runtime-claim","bad-json","invalid-utf8","missing-command","deferred-command","missing-template","yaml","wrong-result","wrong-command","parity","changed-project",
  "changed-input","scan-failure","wrong-plan","dry-run-effect","add-failure","no-state","no-effect","auth-claim","wrong-update","remaining-installation"]) {
  test(`full subprocess journey fails without completion: ${scenario}`,async t=>{
    const f=fixture();subprocessFixtures(t,f,scenario);
    await assert.rejects(internal(true).produce(f.options), scenario === "wrong-command" ? /author command identifier/ : undefined);noTerminal(f);
    assert.ok(fs.existsSync(path.join(f.options.output,"diagnostic.json")));
  });
}
test("unavailable whole-OS observation cannot emit native acceptance after fixture installer",async t=>{
  const f=fixture();subprocessFixtures(t,f);
  await assert.rejects(native.produce(f.options),/WHOLE_OS_OBSERVATION_UNAVAILABLE/);noTerminal(f);
  assert.match(fs.readFileSync(f.log,"utf8"),/"remove"/);
});
for(const scenario of ["timeout","flood","cancel"]){
  test(`bounded child ${scenario} retains no terminal`,async t=>{
    const f=fixture();subprocessFixtures(t,f,scenario);const control=new AbortController();
    const promise=internal(true,true).produce(f.options,control.signal);
    if(scenario==='cancel')setTimeout(()=>control.abort(),40);
    await assert.rejects(promise,/timeout|flood|cancel/);noTerminal(f);
  });
}
test("already-cancelled invocation never starts child or emits terminal",async t=>{
  const f=fixture();subprocessFixtures(t,f);const control=new AbortController();control.abort();
  await assert.rejects(native.produce(f.options,control.signal),/cancel/);noTerminal(f);
  assert.equal(fs.existsSync(f.log),false);
});

test("closed reader rejects terminal and evidence mutations independently", async t => {
  const f=fixture();subprocessFixtures(t,f);await internal(true).produce(f.options);
  const expected=expectations(f),root=f.options.output;
  const original=Object.fromEntries(fs.readdirSync(root).map(name=>[name,fs.readFileSync(path.join(root,name))]));
  const put=(name,body)=>{const file=path.join(root,name);fs.chmodSync(file,0o600);fs.writeFileSync(file,body);};
  for(const scenario of ["lane","subject","peer","source","attempt","producer-workflow","tool","unknown-key","duplicate-key","missing-command",
    "missing-template","missing-installer","extra-evidence","missing-evidence","evidence-link","evidence-hardlink","evidence-digest","traversal","noncanonical"]){
    await t.test(scenario,()=>{
      const file='agentplugins-terminal.json',v=JSON.parse(original[file]);let special;
      if(scenario==='lane')v.lane='agentplugins/linux-arm64';
      if(scenario==='subject')v.subject.sha256=hash('wrong outer');
      if(scenario==='peer')v.peer_subject=v.subject;
      if(scenario==='source')v.identity.commit='b'.repeat(40);
      if(scenario==='attempt')v.producer.run_attempt++;
      if(scenario==='producer-workflow')v.producer.workflow='.github/workflows/other.yml';
      if(scenario==='tool')v.tools.go.sha256=hash('wrong host Go');
      if(scenario==='unknown-key')v.provider_attempt_verified=true;
      if(scenario==='missing-command')v.assertions.commands.agentplugins.pop();
      if(scenario==='missing-template')v.assertions.commands.agentplugins=v.assertions.commands.agentplugins.filter(x=>!x.startsWith('hybrid-stdio/'));
      if(scenario==='missing-installer')v.assertions.installer=['add'];
      if(scenario==='evidence-digest')v.evidence[0].sha256=hash('wrong transcript');
      if(scenario==='traversal')v.evidence[0].file='../transcripts.json';
      let bytes=c.encode(v);
      if(scenario==='duplicate-key')bytes=Buffer.from(bytes.toString().replace('{','{"schema":"duplicate",'));
      if(scenario==='noncanonical')bytes=Buffer.from(JSON.stringify(v));
      put(file,bytes);
      if(scenario==='extra-evidence'){special=path.join(root,'extra.json');fs.writeFileSync(special,'{}');}
      if(scenario==='missing-evidence'){fs.renameSync(path.join(root,'host.json'),path.join(f.sandbox,'host-held.json'));}
      if(scenario==='evidence-link'){
        fs.renameSync(path.join(root,'host.json'),path.join(f.sandbox,'host-held.json'));
        fs.symlinkSync(path.join(f.sandbox,'host-held.json'),path.join(root,'host.json'));
      }
      if(scenario==='evidence-hardlink'){special=path.join(f.sandbox,'host-alias.json');fs.linkSync(path.join(root,'host.json'),special);}
      assert.throws(()=>native.readTerminals(root,f.root,f.pins,expected));
      if(special)fs.unlinkSync(special);
      if(scenario==='evidence-link')fs.unlinkSync(path.join(root,'host.json'));
      if(scenario==='evidence-link'||scenario==='missing-evidence')fs.renameSync(path.join(f.sandbox,'host-held.json'),path.join(root,'host.json'));
      for(const [name,body]of Object.entries(original))put(name,body);
    });
  }
  assert.equal(native.readTerminals(root,f.root,f.pins,expected).length,2);
});

test("rehashed evidence cannot hide omitted commands, bad plans or false preservation",async t=>{
  const f=fixture();subprocessFixtures(t,f);const n=internal(true);await n.produce(f.options);
  const root=f.options.output,expected=expectations(f);
  const originals=Object.fromEntries(fs.readdirSync(root).map(file=>[file,fs.readFileSync(path.join(root,file))]));
  function put(file,value){const out=path.join(root,file);fs.chmodSync(out,0o600);fs.writeFileSync(out,c.encode(value));}
  for(const scenario of ['omitted','duplicate','wrong-json','changed-state','empty-preservation','changed-tree','wrong-build-info']){
    await t.test(scenario,()=>{
      const e=Object.fromEntries(Object.entries(originals).filter(([file])=>!file.endsWith('-terminal.json')).map(([file,b])=>[file,JSON.parse(b)]));
      const rows=e['transcripts.json'].agentplugins;
      if(scenario==='omitted')rows.splice(8,1);
      if(scenario==='duplicate')rows[8]=rows[7];
      if(scenario==='wrong-json')rows[8].stdout='{"schema_version":1,"schema_version":1}';
      if(scenario==='changed-state')e['transcripts.json'].installer[0].after.state.push({path:'state-v2.json'});
      if(scenario==='empty-preservation'){e['preservation.json'].inputs_before=[];e['preservation.json'].inputs_after=[];}
      if(scenario==='changed-tree')e['trees.json'].agentplugins.skill.manifest.name='other';
      if(scenario==='wrong-build-info')e['build-info.json'].agentplugins.Path='other';
      for(const [file,v]of Object.entries(e))put(file,v);
      // Attacker updates outer evidence hashes too. Semantic replay must reject.
      for(const p of c.PRODUCTS){const v=JSON.parse(originals[p+'-terminal.json']);v.evidence=v.evidence.map(pin=>({file:pin.file,...c.metadata(c.encode(e[pin.file]))}));put(p+'-terminal.json',v);}
      assert.throws(()=>native.readTerminals(root,f.root,f.pins,expected));
      for(const [file,b]of Object.entries(originals)){fs.chmodSync(path.join(root,file),0o600);fs.writeFileSync(path.join(root,file),b);}
    });
  }
});

test("closed configuration has no success, observer, executable or provider boolean seam",async()=>{
  for(const key of ['provider_attempt_verified','observer','run','accept','binary','skip_installer']){
    const f=fixture();f.options[key]=true;await assert.rejects(native.produce(f.options),/unexpected or missing fields/);noTerminal(f);
  }
  assert.deepEqual(Object.keys(native).sort(),['produce','readPreparation','readTerminals','writePreparation']);
});

test("host Go pin is independent of builder executable digest and checked before effects",async t=>{
  const f=fixture();subprocessFixtures(t,f);
  const manifest=release.verifyProjectedPair(f.root,f.pins).manifest;
  assert.notEqual(manifest.build.go_sha256,f.options.go_sha256);
  f.options.go_sha256=hash('incorrect host pin');
  await assert.rejects(native.produce(f.options));noTerminal(f);assert.equal(fs.existsSync(f.log),false);
});

test("bounded JSON decoding rejects duplicate keys including escaped aliases",()=>{
  const parse=internal().test.jsonDocument;
  for(const text of ['{"a":1,"a":2}','{"nested":{"a":1,"\\u0061":2}}','{"x":[{"a":1,"a":2}]}',
    '[] trailing','{"a":','[1,]','["'+ 'x'.repeat(1024*1024)+'"]','['.repeat(65)+'0'+']'.repeat(65)])
    assert.throws(()=>parse(text));
  assert.deepEqual(parse(' { "x": [1, {"a":true}], "nested":{"a":false} } '),{x:[1,{a:true}],nested:{a:false}});
});

test("tree decoder rejects aliases, special modes, traversal and incomplete parents",()=>{
  const check=internal().test.treeShape;
  const root={path:'.',mode:448,kind:'directory'};
  const file={path:'plugin.json',mode:420,kind:'file',size:1,sha256:hash('x')};
  check([root,file]);
  for(const entries of [[],[file],[root,file,file],[root,{...file,path:'../outside'}],
    [root,{...file,path:'/absolute'}],[root,{...file,path:'missing/child'}],[root,{...file,kind:'symlink'}],
    [root,{...file,mode:0o4777}],[root,{...file,size:-1}],[root,{...file,sha256:null}],[root,{...file,extra:true}]])
    assert.throws(()=>check(entries));
});

test("owned child cleanup denial rejects without any terminal",async t=>{
  const f=fixture();subprocessFixtures(t,f);
  const kill=process.kill;
  // This only models the post-exit check of our own fixture group. It never
  // touches another worker or probes a real observation service.
  t.mock.method(process,'kill',(pid,signal)=>{
    if(signal===0){const error=new Error('fixture cleanup denial');error.code='EPERM';throw error;}
    return kill(pid,signal);
  });
  await assert.rejects(internal(true).produce(f.options),/cleanup uncertain/);noTerminal(f);
});

test("failed second exclusive terminal write removes both owned terminal names",async t=>{
  const f=fixture();subprocessFixtures(t,f);
  const open=fs.openSync,write=fs.writeFileSync;let fd;
  t.mock.method(fs,'openSync',(file,...args)=>{
    const opened=open(file,...args);
    if(typeof file==='string'&&file===path.join(f.options.output,'plugin-kit-ai-terminal.json'))fd=opened;
    return opened;
  });
  t.mock.method(fs,'writeFileSync',(file,...args)=>{
    if(fd!==undefined&&file===fd){fd=undefined;const error=new Error('fixture terminal capacity failure');error.code='ENOSPC';throw error;}
    return write(file,...args);
  });
  await assert.rejects(internal(true).produce(f.options),/terminal capacity/);noTerminal(f);
});

test("input modes and candidate/projection bytes remain unchanged by receipt creation",()=>{
  const f=fixture(),n=internal(),before=n.test.tree(f.root);
  assert.throws(()=>native.writePreparation(f.root,f.pins,f.options.preparation.producer),/EEXIST/);
  assert.deepEqual(n.test.tree(f.root),before);
  assert.equal(JSON.parse(c.readFile(path.join(f.root,'pair-prepared.json'))).platform_acceptance,false);
  assert.equal(JSON.parse(c.readFile(path.join(f.root,'agentplugins/release-manifest.json'))).attested,false);
});

test("changed host/source/attempt expectations cannot read a matching local terminal",async t=>{
  const f=fixture();subprocessFixtures(t,f);await internal(true).produce(f.options);
  for(const kind of ['source','workflow-sha','run','attempt','preparation-attempt','tool']){
    const expect=structuredClone(expectations(f));
    if(kind==='source')expect.producer.source='b'.repeat(40);
    if(kind==='workflow-sha')expect.producer.workflow_sha='b'.repeat(40);
    if(kind==='run')expect.producer.run_id++;
    if(kind==='attempt')expect.producer.run_attempt++;
    if(kind==='preparation-attempt')expect.preparation.producer.run_attempt++;
    if(kind==='tool')expect.tools.go.sha256=hash('different host Go');
    assert.throws(()=>native.readTerminals(f.options.output,f.root,f.pins,expect));
  }
});

test("all thirteen promotion lanes still reject independently of N1 registration",()=>{
  const f=fixture();
  for(const lane of f.record.qualification.lanes)lane.schema='authoring-frozen-native/v1';
  assert.equal(promotion.LANES.length,13);
  assert.throws(()=>promotion.requireNativeContracts(f.record.qualification.lanes),/NATIVE_EVIDENCE_INTEGRATION_REQUIRED/);
  assert.equal(f.record.qualification.lanes.at(-1).lane,'public-packed-pair');
});

test("input and output placement reject overlap before subprocess effects",async t=>{
  for(const kind of ['input','peer-output','tool-directory','existing']){
    const f=fixture();
    if(kind==='input')f.options.output=path.join(f.root,'nested-output');
    if(kind==='peer-output')f.options.output=f.options.work;
    if(kind==='tool-directory')f.options.output=path.join(path.dirname(f.go),'nested-output');
    if(kind==='existing')fs.mkdirSync(f.options.output);
    await assert.rejects(native.produce(f.options));noTerminal(f);
    assert.equal(fs.existsSync(f.options.work),false);
  }
});
