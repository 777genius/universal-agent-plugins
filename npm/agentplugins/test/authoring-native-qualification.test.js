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
  if (process.env.N1_FIXTURE_LOG) fs.appendFileSync(process.env.N1_FIXTURE_LOG, f.sandbox + "\n");
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
const scannerFixture = Buffer.from("not executable: scanner acquisition fixture");
function scannerTar(binary) {
  const zlib = require("node:zlib"), tar = zlib.gunzipSync(c.archive(binary, "agentplugins"));
  tar.fill(0, 0, 100); tar.write("lintai", 0); tar.fill(32, 148, 156);
  const sum = tar.subarray(0, 512).reduce((n, b) => n + b, 0);
  tar.write(sum.toString(8).padStart(6, "0") + "\0 ", 148);
  return zlib.gzipSync(tar);
}
const scannerFixtureArchive = scannerTar(scannerFixture);
function fixtureReader(...args) { return internal(true).readTerminals(...args); }
function internal(observation = false, fastTimeout = false) {
  let source = fs.readFileSync(moduleFile, "utf8");
  if (observation) source = source.replace('    e["scans.json"] = observationGate(); //', '    e["scans.json"] = JSON.parse(fs.readFileSync(path.join(install.env.TMPDIR, "fixture-scans.json"))); // Test-only captured child fixtures.');
  // Only this isolated test module pins the tiny synthetic tar. Unmodified
  // production readers must reject it; no runtime pin parameter is introduced.
  if (observation) source = source.replace("2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", c.digest(scannerFixtureArchive));
  if (fastTimeout) source = source.replace('installer ? 120000 : 15000', '100');
  // Expose lexical contracts only in this test VM. Production exports no policy,
  // verifier, command inventory, child executable or success switch.
  source += '\nmodule.exports.test = { commands, installationCommands, verifyJourney, terminal, tree, canonical, context, subprocess, observationGate, jsonDocument, treeShape, packageIdentity, capabilities, PROFILES, POLICY, scannerArchive, SCANNER_RELEASES };';
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
const profiles = [{"id": "agent-plugins/1.0.0", "revision": "ff8ab5e392cc87bd88d87c060815a87490e51003", "digest": "sha256:97a658b7dca3ce1b4c2266b95da300fa51d9dc4ade59d73168e5f9104272da18"}, {"id": "agent-skills/2026-09-06", "revision": "69ef37e9424c0a7ea9dd2293b559e43ec8176379", "digest": "sha256:b9079c0c10b7930e8c6a20ff2bc10cda2a3343c55185120e3f1116a1a529b220"}, {"id": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "revision": "1.0.0", "digest": "sha256:0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883"}, {"id": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "revision": "1.0.0", "digest": "sha256:6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb"}, {"id": "author-document-bounds/v1", "revision": "1", "digest": "sha256:4b8ab8fd50481ccd1a0b777dcbbfa06cf89516a5ea61ce09d56d6dd6a2c43004"}];
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
    else {
      const kinds={chatgpt:['compatibility_projection','projected','unsupported','unsupported'],claude:['compatibility_projection','projected','projected','unsupported'],
        cline:['native','native','native','unsupported'],codex:['compatibility_projection','projected','projected','unsupported'],
        copilot:['native','native','native','native'],cursor:['native','native','native','native'],gemini:['native','native','native','unsupported'],kiro:['native','native','native','unsupported'],
        opencode:['prepared_package','prepared','prepared','unsupported'],vscode:['prepared_package','prepared','prepared','prepared'],windsurf:['prepared_package','prepared','prepared','prepared']};
      data.capabilities={schemas:profiles.slice(2,4).map(p=>({id:p.id,digest:p.digest})),profiles,commands:data.commands,
        evidence_limits:['static_only','no_path_lookup','no_executable_version_probe','no_runtime_or_oauth_evidence','native_files_metadata_only'],
        clients:Object.entries(kinds).map(([id,k])=>({client_id:id,package_mode:k[0],activation_mode:['claude','cline','opencode'].includes(id)?'automatic':'manual',
          scopes:['user'],skill_support:k[1],mcp_transports:{stdio:k[2],'streamable-http':k[2],sse:k[2]},app_support:id==='chatgpt'?'projected':'unsupported',extension_support:k[3]}))};
      if(scenario==='empty-capabilities')data.capabilities={};
      if(scenario==='missing-profile')data.capabilities.profiles=profiles.slice(1);
      if(scenario==='wrong-schema')data.capabilities.schemas[0].digest='sha256:'+sha('wrong schema');
    }
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
  data.identity={read_profile:'packageview-local-linux-v1',tree_digest:identity(root).tree_digest};
  data.profiles=profiles;data.schema_ids=(root==='skill'?[profiles[2].id]:profiles.slice(2,4).map(x=>x.id).sort());
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
// Independent implementation of the existing uint64-BE package framing on
// actual child-created bytes (not the producer's package identity helper).
function identity(root) {
  const parts=[];
  const number=n=>{const b=Buffer.alloc(8);b.writeBigUInt64BE(BigInt(n));return b;};
  const frame=x=>{const b=Buffer.from(x);parts.push(number(b.length),b);};
  frame('agentplugins.package-tree\0sha256\0v1');
  const entries=[];
  const walk=rel=>{for(const name of fs.readdirSync(path.join(root,rel)).sort()){
    const p=path.posix.join(rel,name),st=fs.statSync(path.join(root,p));entries.push([p,st]);if(st.isDirectory())walk(p);
  }};walk('');entries.sort((a,b)=>a[0]<b[0]?-1:1);
  for(const [p,st]of entries){const body=st.isDirectory()?Buffer.alloc(0):fs.readFileSync(path.join(root,p));
    for(const field of ['entry',p,st.isDirectory()?'directory':'file',st.isDirectory()?'040000':st.mode&0o111?'100755':'100644',''])frame(field);
    parts.push(number(body.length),body);
  }
  return {tree_digest:'sha256:'+sha(Buffer.concat(parts)),manifest_digest:'sha256:'+sha(fs.readFileSync(path.join(root,'plugin.json')))};
}
const policy={id:'agent-plugin-install',version:2,digest:'sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037'};
function security(source,root){
  const report=JSON.stringify({schema_version:1,tool:{name:'lintai',version:'0.1.3'},policy:{id:policy.id,version:scenario==='raw-report-policy'?999:2,presets:['agent-plugin']},
    stats:{scanned_files:scenario==='raw-report-counts'?-1:4,skipped_files:0},findings:scenario==='raw-report-findings'?[{rule_code:'SEC330'}]:[],diagnostics:[],runtime_errors:[]})+'\n';
  const value={schema_version:1,scanner:{id:'lintai',version:'0.1.3'},policy,counts:{blocking:0,warnings:0,total:0},scanned_files:4,
    evidence_source:source,outcome:'no_blocking_findings',subject:identity(root),report_digest:'sha256:'+sha(report)};
  if(scenario==='wrong-package')value.subject={tree_digest:'sha256:'+sha('unrelated tree'),manifest_digest:'sha256:'+sha('unrelated manifest')};
  if(scenario==='invalid-security-contract'){value.schema_version=999;value.policy={id:'wrong',version:-1,digest:'invalid'};value.counts={blocking:42};}
  if(scenario==='wrong-policy')value.policy={...policy,version:1};
  if(scenario==='wrong-counts')value.counts={blocking:42,warnings:0,total:42};
  if(scenario==='wrong-findings')value.findings=[{code:'SEC330',disposition:'blocking',message:'bad executable'}];
  if(scenario==='cache-mismatch'&&source==='cache')value.report_digest='sha256:'+sha('other report');
  const key=sha([value.subject.tree_digest,value.subject.manifest_digest,'lintai','0.1.3',policy.id,'2',policy.digest].join('\0'));
  const state=process.env.AGENTPLUGINS_HOME;
  if(source==='local_scan'){
    const scanner='security/lintai/0.1.3/linux-amd64/lintai',executable=scenario==='unrelated-scanner'?'arbitrary unrelated scanner bytes; never executed':'not executable: scanner acquisition fixture';
    fs.mkdirSync(path.dirname(path.join(state,scanner)),{recursive:true,mode:0o700});
    if(scenario!=='missing-scanner')fs.writeFileSync(path.join(state,scanner),executable,{mode:0o700});
    fs.mkdirSync(path.join(state,'security/assessments'),{recursive:true,mode:0o700});
    const cached={...value};delete cached.evidence_source;
    if(scenario!=='missing-cache')write(state,'security/assessments/'+key+'.json',cached);
    const file=path.join(process.env.TMPDIR,'fixture-scans.json'),scans=fs.existsSync(file)?JSON.parse(fs.readFileSync(file)):[];
    if(scenario!=='missing-scan')scans.push({id:'dry-run/'+path.basename(root),args:['scan-agent-plugin','<source>/'+path.basename(root)],subject:identity(root),
      executable:{path:scanner,sha256:sha(executable)},archive:{url:'https://github.com/777genius/lintai/releases/download/v0.1.3/lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz',bytes:cfg.scannerArchive},report:scenario==='missing-report'?'':report});
    fs.writeFileSync(file,JSON.stringify(scans));
  }
  return value;
}
const installationID='12345678-1234-4234-8234-123456789abc';
const physical='skill-'+sha(installationID).slice(0,12), projection='managed/clients/codex/'+physical;
function registration(state,root){
  const subject=identity(root),revision={version:'0.1.0',...subject};
  const target=path.join(scenario==='wrong-target-locator'?'/not-the-owned-installer-root':state,projection),binding='client_'+sha([installationID,'codex','user',target].join('\0')).slice(0,24);
  return {installation_id:installationID,declared_name:'skill',source:{tree_digest:subject.tree_digest},
    package:{declared_name:'skill',version:'0.1.0',manifest_digest:subject.manifest_digest},
    clients:{[binding]:{client_binding_id:binding,client_id:'codex',scope:'user',materialization:'materialized',physical_artifact_id:physical,
      target_locator:target,package_revision:revision}}};
}

function installer(args) {
  const verb=args[0], state=process.env.AGENTPLUGINS_HOME,client=path.join(process.env.HOME,'.codex');
  const data={}, result={schema_version:1,command:verb,result:'success',data};
  if(verb==='version'){data.version=scenario==='wrong-version'?'9.9.9':cfg.identity.versions.agentplugins;return output(result);}
  if(scenario==='scan-failure'){result.result='failure';return output(result,1);}
  if(verb==='add'&&args.includes('--dry-run')) {
    const lane=path.basename(args[1]), names=fs.readdirSync(path.join(args[1],'skills'));
    const components=names.map(name=>({kind:'skill',name,support:'projected'}));
    if(lane!=='skill')components.push({kind:'mcp_server',name:lane,support:'projected'});
    data.security=security('local_scan',args[1]);Object.assign(data,data.security.subject);
    data.dry_run=true;data.result={plan:{client_id:'codex',scope:'user',status:'manual_activation_required',components}};
    if(scenario==='wrong-plan')data.result.plan.components=[];
    if(scenario==='dry-run-effect')write(client,'unexpected','effect');
  } else if(verb==='add') {
    if(scenario==='add-failure'){result.result='failure';return output(result,1);}
    data.result={installation_id:installationID,mutated:true,activation:{authentication:'not_checked'}};
    data.security=security('cache',args[1]);Object.assign(data,data.security.subject);
    if(scenario!=='no-state')write(state,'state-v2.json',{schema_version:4,installations:[registration(state,args[1])]});
    if(scenario!=='no-effect'){
      for(const name of ['skill','extra-skill']){
        fs.mkdirSync(path.join(state,projection,'skills',name),{recursive:true,mode:0o700});
        fs.copyFileSync(path.join(args[1],'skills',name,'SKILL.md'),path.join(state,projection,'skills',name,'SKILL.md'));
      }
    }
    if(scenario!=='no-effect') {
      fs.mkdirSync(path.join(state,projection,'.codex-plugin'),{mode:0o700});
      fs.mkdirSync(path.join(state,projection,'.agents/plugins'),{recursive:true,mode:0o700});
      const manifest={name:'skill',version:'0.1.0',description:'Owned fixture',skills:'./skills/'};
      const marketplace={name:'agentplugins-'+sha(physical).slice(0,12),plugins:[{name:'skill',source:{source:'local',path:'./'},
        policy:{installation:'AVAILABLE',authentication:'ON_INSTALL'},category:'Productivity'}]};
      if(scenario==='wrong-codex-identity')manifest.name='unrelated';
      if(scenario==='wrong-codex-reference')manifest.skills='./elsewhere/';
      if(scenario==='wrong-marketplace-identity')marketplace.name='unrelated';
      if(scenario==='wrong-marketplace-reference')marketplace.plugins[0].source.path='../unrelated';
      if(scenario!=='missing-codex-projection')write(path.join(state,projection),'.codex-plugin/plugin.json',scenario==='malformed-codex-projection'?'{malformed':manifest);
      if(scenario!=='missing-marketplace')write(path.join(state,projection),'.agents/plugins/marketplace.json',scenario==='malformed-marketplace'?'{malformed':marketplace);
    }
    if(scenario==='alter-client')write(client,'config.toml','unexpected replacement');
    if(scenario==='alter-client-mode')fs.chmodSync(path.join(client,'config.toml'),0o644);
    if(scenario==='auth-claim')data.result.activation.authentication_attested=true;
    if(scenario==='wrong-installation-identity')data.result.installation_id='87654321-1234-4234-8234-123456789abc';
  } else if(verb==='info'){
    if(scenario==='info-mutates-state')write(state,'unexpected-info-write','x');
    data.installation_id=installationID;data.name='skill';data.version='0.1.0';data.clients=Object.values(JSON.parse(fs.readFileSync(path.join(state,'state-v2.json'))).installations[0].clients);
  }
  else if(verb==='update')data.result={installation_id:installationID,mutated:false,no_change:scenario!=='wrong-update'};
  else if(verb==='remove'){
    data.result={installation_id:installationID,mutated:true};
    if(scenario!=='remaining-registration')write(state,'state-v2.json',{schema_version:4,installations:[]});
    if(scenario!=='remove-leaves-client-projection'){
      for(const name of ['skill','extra-skill']){fs.unlinkSync(path.join(state,projection,'skills',name,'SKILL.md'));fs.rmdirSync(path.join(state,projection,'skills',name));}
      fs.unlinkSync(path.join(state,projection,'.codex-plugin/plugin.json'));fs.rmdirSync(path.join(state,projection,'.codex-plugin'));
      fs.unlinkSync(path.join(state,projection,'.agents/plugins/marketplace.json'));fs.rmdirSync(path.join(state,projection,'.agents/plugins'));fs.rmdirSync(path.join(state,projection,'.agents'));
      fs.rmdirSync(path.join(state,projection,'skills'));fs.rmdirSync(path.join(state,projection));
    }
  }
  else if(verb==='list'){data.installations=scenario==='remaining-installation'?[{name:'skill'}]:[];if(scenario==='list-mutates-state')write(state,'list-write','x');}
  return output(result);
}
if (scenario === 'timeout') setInterval(()=>{},1000);
else if (scenario === 'flood') process.stdout.write('x'.repeat(2*1024*1024));
else if (scenario === 'cancel') setInterval(()=>{},1000);
else main();

`;
}
function subprocessFixtures(t, f, scenario = "ok") {
  const script = path.join(f.sandbox, "child-fixture.js"), config = path.join(f.sandbox, "child-config.json");
  f.log = path.join(f.sandbox, "subprocesses.jsonl");
  fs.writeFileSync(script, fixtureProgram());
  fs.writeFileSync(config, JSON.stringify({ scenario, scannerArchive: scannerFixtureArchive.toString("base64"), identity: ID, go: f.go, log: f.log,
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
  // This synthetic archive is never genuine acquisition for the production reader.
  assert.throws(() => native.readTerminals(f.options.output,f.root,f.pins,expectations(f)), /independently pinned scanner release archive/);
  const reread=fixtureReader(f.options.output,f.root,f.pins,expectations(f));
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
  "changed-input","scan-failure","wrong-plan","dry-run-effect","add-failure","no-state","no-effect","auth-claim","wrong-update","remaining-installation","wrong-package","invalid-security-contract","wrong-policy","wrong-counts","wrong-findings","cache-mismatch",
  "missing-scanner","missing-cache","missing-scan","missing-report","alter-client","alter-client-mode","info-mutates-state",
  "remove-leaves-client-projection","remaining-registration","list-mutates-state","empty-capabilities","missing-profile","wrong-schema","raw-report-policy","raw-report-counts","raw-report-findings","wrong-installation-identity", "unrelated-scanner", "wrong-target-locator", "missing-codex-projection", "malformed-codex-projection", "wrong-codex-identity", "wrong-codex-reference", "missing-marketplace", "malformed-marketplace", "wrong-marketplace-identity", "wrong-marketplace-reference"]) {
  test(`full subprocess journey fails without completion: ${scenario}`,async t=>{
    const f=fixture();subprocessFixtures(t,f,scenario);
    const reasons = { "unrelated-scanner": /scanner executable is the pinned archive member/, "wrong-target-locator": /registration targets operation-owned installer root/, "wrong-command": /author command identifier/, "wrong-package": /independently captured package security subject/,
      "invalid-security-contract": /fixed production security schema/, "wrong-policy": /fixed production security policy/,
      "wrong-counts": /security counts match outcome/, "wrong-findings": /security findings match counts/,
      "remove-leaves-client-projection": /remove owned projection/, "info-mutates-state": /info is read only/,
      "list-mutates-state": /list is read only/, "raw-report-policy": /scanner report policy/,
      "raw-report-counts": /scanner report file counts/, "raw-report-findings": /scanner report findings/, "wrong-installation-identity": /lifecycle installed identity/ };
    await assert.rejects(internal(true).produce(f.options), reasons[scenario]);noTerminal(f);
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
      assert.throws(()=>fixtureReader(root,f.root,f.pins,expected));
      if(special)fs.unlinkSync(special);
      if(scenario==='evidence-link')fs.unlinkSync(path.join(root,'host.json'));
      if(scenario==='evidence-link'||scenario==='missing-evidence')fs.renameSync(path.join(f.sandbox,'host-held.json'),path.join(root,'host.json'));
      for(const [name,body]of Object.entries(original))put(name,body);
    });
  }
  assert.equal(fixtureReader(root,f.root,f.pins,expected).length,2);
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
      assert.throws(()=>fixtureReader(root,f.root,f.pins,expected));
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
    assert.throws(()=>fixtureReader(f.options.output,f.root,f.pins,expect));
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

// Rehash every outer pin as the independent reviewer did: each rejection must
// come from semantic replay, not a stale digest or a broken fixture baseline.
test("N1 reader rejects rehashed semantic omissions and contradictions", async t => {
  const f = fixture(); subprocessFixtures(t, f); await internal(true).produce(f.options);
  const root = f.options.output, expected = expectations(f);
  const originals = Object.fromEntries(fs.readdirSync(root).map(file => [file, fs.readFileSync(path.join(root, file))]));
  const rootOnly = [{ path: ".", mode: 448, kind: "directory" }];
  const editSecurity = (e, change) => {
    for (const row of e["transcripts.json"].installer.slice(0, 4)) {
      const v = JSON.parse(row.stdout); change(v.data, row); row.stdout = JSON.stringify(v) + "\n";
    }
  };
  function changeRawReport(e, change) {
    const scan = e["scans.json"][0], report = JSON.parse(scan.report); change(report);
    scan.report = JSON.stringify(report) + "\n";
    const digest = "sha256:" + hash(scan.report);
    editSecurity(e, (d, row) => { if (row.id === "dry-run/skill" || row.id === "add") d.security.report_digest = digest; });
    // Update the referenced cache bytes and every snapshot pin too. Only the
    // report's production semantics are invalid, not its integrity chain.
    const states = e["transcripts.json"].installer.flatMap(r => [r.before, r.after]);
    const files = states.flatMap(s => s.acquisition).filter(x => x.path.startsWith("security/assessments/") && x.kind === "file");
    for (const sha256 of new Set(files.map(x => x.sha256))) {
      const value = JSON.parse(Buffer.from(e["acquisition.json"][sha256], "base64"));
      if (value.subject.tree_digest !== scan.subject.tree_digest) continue;
      value.report_digest = digest; const body = Buffer.from(JSON.stringify(value) + "\n"), pin = c.metadata(body);
      delete e["acquisition.json"][sha256]; e["acquisition.json"][pin.sha256] = body.toString("base64");
      for (const item of files.filter(x => x.sha256 === sha256)) Object.assign(item, pin);
    }
  }
  const states = e => e["transcripts.json"].installer.flatMap(r => [r.before, r.after]);
  function projectionChange(e, leaf, change) {
    for (const state of states(e)) {
      const item = state.state.find(x => x.path.endsWith("/" + leaf)); if (!item) continue;
      if (!change) { state.state = state.state.filter(x => x !== item); delete state.projection_documents[item.path]; continue; }
      const old = Buffer.from(state.projection_documents[item.path], "base64");
      const body = Buffer.from(change(old.toString("utf8")));
      Object.assign(item, c.metadata(body)); state.projection_documents[item.path] = body.toString("base64");
    }
  }
  const cases = {
    "unrelated scanner with every acquisition digest rehashed": e => {
      const old = e["scans.json"][0].executable.sha256, body = Buffer.from("arbitrary unrelated scanner bytes; never executed"), pin = c.metadata(body);
      delete e["acquisition.json"][old]; e["acquisition.json"][pin.sha256] = body.toString("base64");
      for (const scan of e["scans.json"]) scan.executable.sha256 = pin.sha256;
      for (const state of states(e)) for (const item of state.acquisition) if (item.sha256 === old) Object.assign(item, pin);
    },
    "arbitrary root expectation in every snapshot": e => {
      for (const state of states(e)) state.root = "/not-the-owned-installer-root";
    },
    "raw state byte mutation hidden by path normalization": e => {
      const row = e["transcripts.json"].installer[4]; row.after.state_document_source.sha256 = hash("mutated raw bytes");
    },
    "missing release archive": e => { delete e["scans.json"][0].archive; },
    "substituted release archive": e => { for (const scan of e["scans.json"]) scan.archive.bytes = scannerTar(Buffer.from("unrelated")).toString("base64"); },
    "wrong release URL": e => { e["scans.json"][0].archive.url = "https://unrelated.invalid/scanner"; },
    "wrong root with coherent binding and info": e => {
      for (const state of states(e)) {
        if (!state.state_document) continue; const doc = JSON.parse(state.state_document);
        for (const r of doc.installations) {
          const cl = Object.values(r.clients)[0]; cl.target_locator = '/not-the-owned-installer-root/managed/clients/codex/' + cl.physical_artifact_id;
          cl.client_binding_id = 'client_' + hash([r.installation_id, 'codex', 'user', cl.target_locator].join('\0')).slice(0, 24);
          r.clients = { [cl.client_binding_id]: cl };
        }
        state.state_document = JSON.stringify(doc) + '\n'; Object.assign(state.state.find(x => x.path === 'state-v2.json'), c.metadata(Buffer.from(state.state_document)));
      }
      const row = e['transcripts.json'].installer[4], doc = JSON.parse(row.stdout);
      doc.data.clients = Object.values(JSON.parse(row.before.state_document).installations[0].clients); row.stdout = JSON.stringify(doc) + '\n';
    },
    "missing Codex manifest": e => projectionChange(e, '.codex-plugin/plugin.json'),
    "malformed Codex manifest": e => projectionChange(e, '.codex-plugin/plugin.json', () => '{malformed'),
    "wrong Codex identity": e => projectionChange(e, '.codex-plugin/plugin.json', s => { const v = JSON.parse(s); v.name = 'unrelated'; return JSON.stringify(v); }),
    "wrong Codex reference": e => projectionChange(e, '.codex-plugin/plugin.json', s => { const v = JSON.parse(s); v.skills = './elsewhere/'; return JSON.stringify(v); }),
    "missing marketplace": e => projectionChange(e, '.agents/plugins/marketplace.json'),
    "malformed marketplace": e => projectionChange(e, '.agents/plugins/marketplace.json', () => '{malformed'),
    "wrong marketplace identity": e => projectionChange(e, '.agents/plugins/marketplace.json', s => { const v = JSON.parse(s); v.name = 'unrelated'; return JSON.stringify(v); }),
    "wrong marketplace reference": e => projectionChange(e, '.agents/plugins/marketplace.json', s => { const v = JSON.parse(s); v.plugins[0].source.path = '../unrelated'; return JSON.stringify(v); }),
    "missing custody": e => { e["preservation.json"].custody_before = []; e["preservation.json"].custody_after = []; },
    "preparation custody omitted": e => { e["preservation.json"].custody_before.pop(); e["preservation.json"].custody_after.pop(); },
    "custody substituted subject": e => { for (const k of ["custody_before", "custody_after"]) e["preservation.json"][k][0].file = "invented"; },
    "custody invalid inode": e => { for (const k of ["custody_before", "custody_after"]) e["preservation.json"][k][0].ino = "not-an-inode"; },
    "remove leaves projection": e => {
      const rows = e["transcripts.json"].installer, r = rows[6];
      r.after.state.push(...r.before.state.filter(x => x.path.startsWith("managed/clients/codex/") && !r.after.state.some(a => a.path === x.path)));
      rows[7].before = structuredClone(r.after); rows[7].after = structuredClone(r.after);
    },
    "info mutates state": e => { e["transcripts.json"].installer[4].after.state.push({ path: "unauthorized", mode: 384, kind: "file", size: 1, sha256: hash("x") }); },
    "list mutates client": e => { e["transcripts.json"].installer[7].after.client[1].mode ^= 0o100; },
    "preexisting client changed continuously": e => {
      const rows = e["transcripts.json"].installer;
      for (let i = 3; i < rows.length; i++) for (const when of i === 3 ? ["after"] : ["before", "after"])
        rows[i][when].client.find(x => x.path === "config.toml").sha256 = hash("changed config");
    },
    "wrong add installation ID": e => { const row = e["transcripts.json"].installer[3], v = JSON.parse(row.stdout); v.data.result.installation_id = "other"; row.stdout = JSON.stringify(v); },
    "wrong info revision": e => { const row = e["transcripts.json"].installer[4], v = JSON.parse(row.stdout); v.data.clients[0].package_revision.tree_digest = "sha256:" + hash("other tree"); row.stdout = JSON.stringify(v); },
    "wrong state client binding": e => {
      for (const row of e["transcripts.json"].installer) for (const state of [row.before, row.after]) {
        if (!state.state_document) continue; const document = JSON.parse(state.state_document);
        if (!document.installations.length) continue;
        document.installations[0].clients = { wrong: Object.values(document.installations[0].clients)[0] };
        state.state_document = JSON.stringify(document); Object.assign(state.state.find(x => x.path === "state-v2.json"), c.metadata(Buffer.from(state.state_document)));
      }
    },
    "state discontinuity": e => { e["transcripts.json"].installer[4].before = structuredClone(e["transcripts.json"].installer[2].after); },
    "missing command effects": e => { for (const p of c.PRODUCTS) for (const r of e["transcripts.json"][p]) { r.before = rootOnly; r.after = rootOnly; } },
    "init never created": e => { for (const p of c.PRODUCTS) e["transcripts.json"][p].find(r => r.id === "skill/init").after = rootOnly; },
    "extra Skill overwrites existing file": e => { for (const p of c.PRODUCTS) e["transcripts.json"][p].find(r => r.id === "skill/extra-skill").after.find(x => x.path === "skill/README.md").mode ^= 0o100; },
    "malformed harness omitted": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "malformed-skill"); r.before = r.before.filter(x => !x.path.includes("/broken")); r.after = r.before; } },
    "malformed harness wrong bytes": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "malformed-skill"); for (const when of ["before", "after"]) r[when].find(x => x.path.endsWith("broken/SKILL.md")).sha256 = hash("other invalid Skill"); } },
    "invented acquisition path": e => { for (const r of e["transcripts.json"].installer) for (const state of [r.before, r.after]) state.acquisition = [{ path: "../../outside", kind: "symlink", sha256: "not-a-digest" }]; },
    "acquisition missing bytes": e => { e["acquisition.json"] = {}; },
    "acquisition altered bytes": e => { const a = e["acquisition.json"]; a[Object.keys(a)[0]] = Buffer.from("unrelated").toString("base64"); },
    "acquisition oversize": e => { e["transcripts.json"].installer[0].after.acquisition.find(x => x.kind === "file").size = 17 * 1024 * 1024; },
    "wrong scanned package and policy": e => editSecurity(e, d => {
      d.tree_digest = "sha256:" + "b".repeat(64); d.manifest_digest = "sha256:" + "c".repeat(64);
      Object.assign(d.security, { schema_version: 999, policy: { id: "wrong", version: -1, digest: "invalid" }, counts: { blocking: 42 },
        subject: { tree_digest: d.tree_digest, manifest_digest: d.manifest_digest } });
    }),
    "wrong package only": e => editSecurity(e, d => { d.tree_digest = "sha256:" + hash("other package"); d.security.subject.tree_digest = d.tree_digest; }),
    "wrong policy only": e => editSecurity(e, d => { d.security.policy.digest = "sha256:" + hash("stale policy"); }),
    "invalid counts only": e => editSecurity(e, d => { d.security.counts.blocking = 42; }),
    "invalid findings only": e => editSecurity(e, d => { d.security.findings = [{ code: "SEC330", disposition: "blocking", message: "blocking" }]; }),
    "missing scanner call": e => { e["scans.json"].pop(); },
    "missing raw report": e => { e["scans.json"][0].report = ""; },
    "changed raw report": e => { e["scans.json"][0].report += " "; },
    "raw report invalid counts with coherent pins": e => changeRawReport(e, r => { r.stats.scanned_files = -1; }),
    "raw report invalid policy with coherent pins": e => changeRawReport(e, r => { r.policy.version = 999; }),
    "raw report findings with coherent pins": e => changeRawReport(e, r => { r.findings = [{ rule_code: "SEC330" }]; }),
    "raw report runtime errors with coherent pins": e => changeRawReport(e, r => { r.runtime_errors = [{}]; }),
    "scanner unrelated executable": e => { e["scans.json"][0].executable.sha256 = hash("other scanner"); },
    "scan unrelated subject": e => { e["scans.json"][0].subject.tree_digest = "sha256:" + hash("other subject"); },
    "scan wrong command": e => { e["scans.json"][0].args = ["--version"]; },
    "cache report mismatch": e => editSecurity(e, (d, row) => { if (row.id === "add") d.security.report_digest = "sha256:" + hash("another report"); }),
    "empty capability contract": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "capabilities"), v = JSON.parse(r.stdout); v.data.capabilities = {}; r.stdout = JSON.stringify(v); } },
    "missing capabilities client": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "capabilities"), v = JSON.parse(r.stdout); v.data.capabilities.clients.pop(); r.stdout = JSON.stringify(v); } },
    "missing conformance profile": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "skill/validate"), v = JSON.parse(r.stdout); v.data.profiles.pop(); r.stdout = JSON.stringify(v); } },
    "wrong schema inventory": e => { for (const p of c.PRODUCTS) { const r = e["transcripts.json"][p].find(r => r.id === "skill/validate"), v = JSON.parse(r.stdout); v.data.schema_ids = []; r.stdout = JSON.stringify(v); } }
  };
  for (const [name, change] of Object.entries(cases)) await t.test(name, () => {
    const e = Object.fromEntries(Object.entries(originals).filter(([n]) => !n.endsWith("-terminal.json")).map(([n, b]) => [n, JSON.parse(b)]));
    change(e);
    const put = (n, b) => { fs.chmodSync(path.join(root, n), 0o600); fs.writeFileSync(path.join(root, n), b); };
    for (const [n, v] of Object.entries(e)) put(n, c.encode(v));
    for (const p of c.PRODUCTS) {
      const v = JSON.parse(originals[p + "-terminal.json"]);
      v.evidence = v.evidence.map(pin => ({ file: pin.file, ...c.metadata(c.encode(e[pin.file])) }));
      put(p + "-terminal.json", c.encode(v));
    }
    const precise = {
      "unrelated scanner with every acquisition digest rehashed": /scanner executable is the pinned archive member/,
      "substituted release archive": /independently pinned scanner release archive/,
      "wrong root with coherent binding and info": /registration targets operation-owned installer root/,
      "arbitrary root expectation in every snapshot": /fixed operation-owned installer root token/,
      "missing Codex manifest": /mandatory Codex projection/,
      "missing marketplace": /mandatory Codex projection/,
      "wrong Codex identity": /Codex plugin identity and component references/,
      "wrong Codex reference": /Codex plugin identity and component references/,
      "wrong marketplace identity": /Codex managed marketplace identity and reference/,
      "wrong marketplace reference": /Codex managed marketplace identity and reference/
    };
    assert.throws(() => fixtureReader(root, f.root, f.pins, expected), precise[name], name);
    for (const [n, b] of Object.entries(originals)) put(n, b);
  });
  assert.equal(fixtureReader(root, f.root, f.pins, expected).length, 2);
});

for (const scenario of ["pre-close kill denied", "close never arrives", "successful termination"]) {
  test(`N1 bounded cleanup: ${scenario}`, async t => {
    const { EventEmitter } = require("node:events"), child = new EventEmitter();
    child.pid = 123456789; child.stdout = new EventEmitter(); child.stderr = new EventEmitter();
    let kills = 0, probes = 0;
    t.mock.method(cp, "spawn", () => child);
    t.mock.method(process, "kill", (_pid, signal) => {
      if (signal === 0) { probes++; const e = new Error("gone"); e.code = "ESRCH"; throw e; }
      kills++;
      if (scenario === "pre-close kill denied") { const e = new Error("denied"); e.code = "EPERM"; throw e; }
      if (scenario === "successful termination") setImmediate(() => child.emit("close", null, "SIGKILL"));
      return true;
    });
    const begin = performance.now();
    const result = internal(false, true).test.subprocess("/never-executed", [], { root: os.tmpdir(), env: {} });
    const outcome = await Promise.race([
      result.then(() => "unexpected success", e => e.message),
      new Promise(resolve => { const timer = setTimeout(() => resolve("unbounded"), 1800); timer.unref(); })
    ]);
    assert.notEqual(outcome, "unbounded"); assert.match(outcome, /timeout/); assert.equal(kills, 1);
    if (scenario === "successful termination") { assert.ok(performance.now() - begin < 900); assert.doesNotMatch(outcome, /uncertain/); }
    else { assert.match(outcome, /cleanup uncertain: child close not observed/); assert.equal(probes, 0); }
    if (scenario === "pre-close kill denied") { assert.match(outcome, /cleanup denied: EPERM/); assert.ok(performance.now() - begin < 500); }
    child.emit("close", null, "SIGKILL"); assert.equal(kills, 1); // Late close cannot retry cleanup.
  });
}

test("denied pre-close cleanup reaches producer failure diagnostics without retries", async t => {
  const f = fixture(), { EventEmitter } = require("node:events"), child = new EventEmitter();
  child.pid = 123456789; child.stdout = new EventEmitter(); child.stderr = new EventEmitter();
  let calls = 0;
  t.mock.method(cp, "spawn", () => child);
  t.mock.method(process, "kill", () => { calls++; const e = new Error("synthetic denial"); e.code = "EPERM"; throw e; });
  const pending = internal(true, true).produce(f.options);
  await assert.rejects(pending, /cleanup denied: EPERM; cleanup uncertain: child close not observed/);
  noTerminal(f); assert.equal(calls, 1);
  const diagnostic = JSON.parse(fs.readFileSync(path.join(f.options.output, "diagnostic.json")));
  assert.equal(diagnostic.native_acceptance, false); assert.match(diagnostic.error, /cleanup uncertain/);
  child.emit("close", null, "SIGKILL"); assert.equal(calls, 1);
});

test("independent package identity uses content framing and portable executable modes", () => {
  const documents = { "plugin.json": Buffer.from('{"name":"skill"}\n'), "skills/demo/SKILL.md": Buffer.from([0, ...Buffer.from("fixture"), 255]) };
  const project = { files: [{ path: ".", kind: "directory", mode: 448 },
    { path: "plugin.json", kind: "file", mode: 420, ...c.metadata(documents["plugin.json"]) },
    { path: "skills", kind: "directory", mode: 448 }, { path: "skills/demo", kind: "directory", mode: 448 },
    { path: "skills/demo/SKILL.md", kind: "file", mode: 493, ...c.metadata(documents["skills/demo/SKILL.md"]) }],
    documents: Object.fromEntries(Object.entries(documents).map(([name, body]) => [name, body.toString("base64")])) };
  const digest = internal().test.packageIdentity;
  // Independently framed uint64-BE vector; includes non-UTF8 content.
  const expected = { tree_digest: "sha256:68fa0b7927e71c2ff3c74c3235085d8aed2b85968db22ee27fe02ca1abff9bae",
    manifest_digest: "sha256:2950ebbb0911376d0566dead6130d2b13b00608eac7119a6fe50ae65e6a0967d" };
  assert.deepEqual(digest(project), expected);
  project.files[1].mode = 384; assert.deepEqual(digest(project), expected); // Non-executable permission changes are not package identity.
  project.files[4].mode = 420; assert.notEqual(digest(project).tree_digest, expected.tree_digest);
  project.documents["plugin.json"] = Buffer.from("other bytes").toString("base64"); assert.throws(() => digest(project));
});

test("fixed profile and schema pins agree with preserved domain contracts", () => {
  const n = internal().test, root = path.resolve(__dirname, "../../..");
  for (const schema of n.capabilities().schemas) {
    const name = schema.id.endsWith("/plugin.schema.json") ? "plugin" : "mcp";
    const file = path.join(root, `install/integrationctl/agentplugins/adapters/specregistry/schemas/1.0.0/${name}.schema.json`);
    assert.equal("sha256:" + c.digest(fs.readFileSync(file)), schema.digest);
  }
  assert.equal(n.PROFILES.at(-1).digest, "sha256:" + c.digest(fs.readFileSync(path.join(root, "install/integrationctl/agentplugins/conformance/profiles/README.md"))));
  const policy = fs.readFileSync(path.join(root, "install/integrationctl/agentplugins/adapters/securityscan/policy_test.go"), "utf8");
  assert.ok(policy.includes(n.POLICY.digest));
});

// No genuine release archive is provisioned. These validators exercise only
// tiny test tar bytes; source-pinned production acceptance is checked separately.
test("scanner release pins match production source and synthetic acquisition is rejected", () => {
  const contract = internal().test, source = fs.readFileSync(path.resolve(__dirname,
    "../../../install/integrationctl/agentplugins/adapters/securityscan/release.go"), "utf8");
  for (const asset of Object.values(contract.SCANNER_RELEASES)) {
    assert.ok(source.includes(`{"${asset.name}", "${asset.sha256}", false}`));
    assert.notEqual(asset.sha256, c.digest(scannerFixtureArchive));
  }
  const scan = { executable: { path: "security/lintai/0.1.3/linux-amd64/lintai" }, archive: {
    url: "https://github.com/777genius/lintai/releases/download/v0.1.3/" + contract.SCANNER_RELEASES["linux-amd64"].name,
    bytes: scannerFixtureArchive.toString("base64") } };
  assert.throws(() => contract.scannerArchive(scan, scannerFixture), /independently pinned scanner release archive/);
  assert.doesNotThrow(() => internal(true).test.scannerArchive(scan, scannerFixture));
  assert.throws(() => internal(true).test.scannerArchive(scan, Buffer.from("unrelated")), /pinned archive member/);
});
test("scanner archive parsing fails closed on bounded malformed test archives", async t => {
  const zlib = require("node:zlib");
  const base = zlib.gunzipSync(scannerFixtureArchive);
  const checksum = tar => {
    tar.fill(32, 148, 156);
    tar.write(tar.subarray(0, 512).reduce((n, b) => n + b, 0).toString(8).padStart(6, "0") + "\0 ", 148);
  };
  const cases = {
    "checksum": tar => { tar[0] ^= 1; return tar; },
    "traversal": tar => { tar.fill(0, 0, 100); tar.write("../lintai"); checksum(tar); return tar; },
    "link": tar => { tar[156] = 50; checksum(tar); return tar; },
    "unsupported extension": tar => { tar[156] = 120; checksum(tar); return tar; },
    "missing binary": tar => { tar.fill(0, 0, 100); tar.write("other"); checksum(tar); return tar; },
    "oversized member": tar => { tar.write((33 * 1024 * 1024).toString(8).padStart(11, "0") + "\0", 124); checksum(tar); return tar; },
    "truncated entry": tar => tar.subarray(0, 520),
    "missing terminator": tar => tar.subarray(0, 1024),
    "duplicate member": tar => Buffer.concat([tar.subarray(0, 1024), tar]),
    "trailing nonzero bytes": tar => { tar[tar.length - 1] = 1; return tar; }
  };
  for (const [name, change] of Object.entries(cases)) await t.test(name, () => {
    const body = zlib.gzipSync(change(Buffer.from(base)));
    // Test-local pin substitution reaches the tar validator, never production
    // acquisition or a caller-selectable expected digest in shipped code.
    const Module = require("node:module"), m = new Module(moduleFile, module);
    m.filename = moduleFile; m.paths = Module._nodeModulePaths(path.dirname(moduleFile));
    m._compile(fs.readFileSync(moduleFile, "utf8").replace(
      "2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da", c.digest(body)) +
      "\nmodule.exports.probe = scannerArchive;", moduleFile);
    assert.throws(() => m.exports.probe({ executable: { path: "security/lintai/0.1.3/linux-amd64/lintai" }, archive: {
      url: "https://github.com/777genius/lintai/releases/download/v0.1.3/lintai-v0.1.3-x86_64-unknown-linux-gnu.tar.gz",
      bytes: body.toString("base64") } }, scannerFixture), name);
  });
});
