"use strict";
// SYNTHETIC INTAKE HARNESS ONLY. No native program, packed product or candidate
// is built/executed. The one stub is explicit and never used by the CLI verifier.
const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const c = require("./dual-authoring-candidate");
const bridge = require("./packed-installer-bridge");
const posixFixture = { skip: process.platform !== "linux" && "Linux POSIX fixture only" };
const hash = p => c.digest(fs.readFileSync(p));
const write = (p, v) => fs.writeFileSync(p,c.encode(v),{mode:0o600});
function fixture(t) {
 const root = fs.mkdtempSync(path.join(os.tmpdir(),"packed-bridge-SYNTHETIC-"));
 // Retained disposable fixtures support failure diagnosis; caller owns cleanup.
 t.diagnostic(`SYNTHETIC ONLY fixture: ${root}`);
 const dir = name => { const p=path.join(root,name); fs.mkdirSync(p,{mode:0o700}); return p; };
 const repo=dir("repo"), candidate=dir("candidate"), output=dir("pack"), evidence=dir("native"), work=dir("work");
 fs.writeFileSync(path.join(candidate,"SYNTHETIC-NOT-A-CANDIDATE"),"synthetic");
 const fixtureRoot=path.join(work,"dual-authoring-SYNTHETIC"); fs.mkdirSync(fixtureRoot,{mode:0o700});
 const identity={repository:"777genius/universal-agent-plugins",engine_revision:"1".repeat(40),commit:"1".repeat(40),versions:{agentplugins:"0.1.23","plugin-kit-ai":"2.0.0"}};
 const packs={},projects={},invocations=[];
 for (const product of bridge.PRODUCTS) {
  const bytes=Buffer.from("SYNTHETIC pack bytes "+product), file=`${product==="agentplugins"?"universal-agent-plugins":product}-${identity.versions[product]}.tgz`;
  fs.writeFileSync(path.join(output,file),bytes);
  packs[product]={file,sha256:c.digest(bytes),size:bytes.length,integrity:"sha512-"+crypto.createHash("sha512").update(bytes).digest("base64")};
  projects[product]=path.join(fixtureRoot,product+" disposable projects"); fs.mkdirSync(projects[product]);
  for (const lane of bridge.LANES) {
   const project=path.join(projects[product],lane); fs.mkdirSync(project); write(path.join(project,"plugin.json"),{synthetic:true}); fs.mkdirSync(path.join(project,"empty")); fs.writeFileSync(path.join(project,"empty-file"),"");
   invocations.push({product,argv:[...(product==="agentplugins"?["author"]:[]),"init",lane,"--format=json"],status:0,signal:null,stdout:JSON.stringify({result:"success",data:{committed:true,revision:identity.commit}}),stderr:""});
  }
 }
 const completion={schema:"dual-authoring-npm-completion/v1",status:"CANDIDATE",identity,candidate_sha256:"2".repeat(64),asset_scope:"linux-amd64-pair",authoring_mode:"release-cli-contract-v1",wrapper_blobs:{},generated:{},packs,tools:{},evidence:{},release_eligible:false,platform_acceptance:false,attested:false};
 const completionPath=path.join(output,"completion.json"); write(completionPath,completion);
 const native={kind:"actual-linux-private-npm-pair",identity,candidate_sha256:completion.candidate_sha256,completion_sha256:hash(completionPath),packs,tools:{},inventory_sha256:"3".repeat(64),invocations:invocations.length,projects,release_eligible:false,platform_acceptance:false,attested:false,installer_planner:"SYNTHETIC TEST DATA ONLY"};
 const nativePath=path.join(evidence,"native-completion.json"); write(nativePath,native); write(path.join(evidence,"invocations.json"),invocations);
 const nativeConfig=path.join(root,"native-config.json"); write(nativeConfig,{stage:{candidate:true,repo,root:candidate,identity,manifestDigest:completion.candidate_sha256,assetScope:completion.asset_scope,authoringMode:completion.authoring_mode,go:"/unused",workParent:work,output,node:"/unused",npm:"/unused"},completionDigest:hash(completionPath),evidenceOutput:evidence});
 return {root,candidate,projects,nativePath,nativeConfig,completionPath,fixtureRoot,request:{expectedCommit:identity.commit,nativeConfig,nativeConfigSha256:hash(nativeConfig),nativeCompletionSha256:hash(nativePath),fixtureRoot,disposableEvidence:true}};
}
test("SYNTHETIC intake control and fail-closed mutations (not packed-native acceptance)", posixFixture, t => {
 const original=c.frozenCandidate;
 c.frozenCandidate=()=>({synthetic:true}); // no fabricated binary metadata or execution
 try {
  const f=fixture(t), cfg=path.join(f.root,"bridge.json"); write(cfg,bridge.seal(f.request)); const digest=hash(cfg);
  const verify=()=>bridge.verify(cfg,digest,f.request.expectedCommit);
  assert.equal(verify().projects.length,10);
  const mutations=[
   ["wrong config digest",()=>assert.throws(()=>bridge.verify(cfg,"0".repeat(64),f.request.expectedCommit))],
   ["wrong intended commit",()=>assert.throws(()=>bridge.verify(cfg,digest,"0".repeat(40)))],
   ["missing completion",()=>{fs.renameSync(f.nativePath,f.nativePath+".held"); try{assert.throws(verify);}finally{fs.renameSync(f.nativePath+".held",f.nativePath);}}],
  ];
  for(const [name,run] of mutations){run();t.diagnostic(name);}
  function changed(file, mutate) { const bytes=fs.readFileSync(file),mode=fs.statSync(file).mode&0o777; try {mutate();assert.throws(verify);} finally {fs.chmodSync(file,mode);fs.writeFileSync(file,bytes);} }
  changed(cfg,()=>fs.appendFileSync(cfg," "));
  changed(f.nativeConfig,()=>fs.appendFileSync(f.nativeConfig," "));
  changed(f.nativePath,()=>write(f.nativePath,{...JSON.parse(fs.readFileSync(f.nativePath)),invocations:0}));
  changed(f.completionPath,()=>fs.appendFileSync(f.completionPath," "));
  const source=path.join(f.projects.agentplugins,"skill"), manifest=path.join(source,"plugin.json");
  changed(manifest,()=>fs.writeFileSync(manifest,"{}"));
  changed(manifest,()=>fs.chmodSync(manifest,0o400));
  const mode=fs.statSync(source).mode&0o777; fs.chmodSync(source,mode ^ 0o020); try{assert.throws(verify);}finally{fs.chmodSync(source,mode);}
  fs.mkdirSync(path.join(source,"new-empty")); assert.throws(verify); fs.rmdirSync(path.join(source,"new-empty"));
  fs.rmdirSync(path.join(source,"empty")); assert.throws(verify); fs.mkdirSync(path.join(source,"empty"));
  fs.renameSync(manifest,manifest+".held"); fs.symlinkSync(manifest+".held",manifest); assert.throws(verify); fs.unlinkSync(manifest); fs.renameSync(manifest+".held",manifest);
  fs.linkSync(manifest,path.join(source,"alias")); assert.throws(verify); fs.unlinkSync(path.join(source,"alias"));
  assert.throws(()=>bridge.seal({...f.request,fixtureRoot:f.root}));
  assert.throws(()=>bridge.seal({...f.request,disposableEvidence:false}));
  assert.throws(()=>bridge.seal({...f.request,extra:true}));
  const bytes=fs.readFileSync(f.nativeConfig); fs.writeFileSync(f.nativeConfig,'{"stage":{},"stage":{}}\n');
  assert.throws(()=>bridge.seal({...f.request,nativeConfigSha256:hash(f.nativeConfig)})); fs.writeFileSync(f.nativeConfig,bytes);
  const inv=path.join(path.dirname(f.nativePath),"invocations.json"); changed(inv,()=>write(inv,[]));
  const tar=path.join(path.dirname(f.completionPath),JSON.parse(fs.readFileSync(f.completionPath)).packs.agentplugins.file); changed(tar,()=>fs.appendFileSync(tar,"mutation"));
  changed(path.join(f.candidate,"SYNTHETIC-NOT-A-CANDIDATE"),()=>fs.appendFileSync(path.join(f.candidate,"SYNTHETIC-NOT-A-CANDIDATE"),"changed"));
  fs.renameSync(source,source+"-held"); assert.throws(verify); fs.renameSync(source+"-held",source);
  const nativeBytes=fs.readFileSync(f.nativePath);
  for (const change of [{identity:{...JSON.parse(nativeBytes).identity,commit:"0".repeat(40)}},{candidate_sha256:"0".repeat(64)},{completion_sha256:"0".repeat(64)},{release_eligible:true},{projects:{agentplugins:f.root,"plugin-kit-ai":f.projects["plugin-kit-ai"]}}]) {
   write(f.nativePath,{...JSON.parse(nativeBytes),...change});
   assert.throws(()=>bridge.seal({...f.request,nativeCompletionSha256:hash(f.nativePath)}));
  }
  fs.writeFileSync(f.nativePath,nativeBytes);
  assert.equal(verify().projects.length,10);
  c.frozenCandidate=original;
  assert.throws(()=>bridge.seal(f.request),"real verifier must reject these synthetic bytes");
  const requestPath=path.join(f.root,"request.json"), rejectedOutput=path.join(f.root,"must-not-exist.json"); write(requestPath,f.request);
  const cli=require("node:child_process").spawnSync(process.execPath,[require.resolve("./packed-installer-bridge"),"seal",requestPath,rejectedOutput],{encoding:"utf8",timeout:10000});
  assert.equal(cli.status,1); assert.equal(fs.existsSync(rejectedOutput),false);
 } finally {c.frozenCandidate=original;}
});
test("snapshot rejects links and captures empty files/directories and modes", posixFixture, t => {
 const root=fs.mkdtempSync(path.join(os.tmpdir(),"packed-snapshot-SYNTHETIC-"));
 fs.mkdirSync(path.join(root,"empty")); fs.writeFileSync(path.join(root,"file"),"");
 const first=bridge.snapshot(root); assert.equal(first.entries.length,3);
 fs.chmodSync(path.join(root,"empty"),(fs.statSync(path.join(root,"empty")).mode & 0o777) ^ 0o020); assert.notEqual(bridge.snapshot(root).sha256,first.sha256);
 fs.symlinkSync("file",path.join(root,"link")); assert.throws(()=>bridge.snapshot(root));
 assert.equal(bridge.snapshot(root,true).entries.length,4);
 fs.unlinkSync(path.join(root,"link")); fs.symlinkSync("/etc/passwd",path.join(root,"link")); assert.throws(()=>bridge.snapshot(root,true));
});

function publicFixture(t) {
 const f=fixture(t), cfg=JSON.parse(fs.readFileSync(f.nativeConfig)), old=cfg.stage;
 const dir=name=>{const p=path.join(f.root,name);fs.mkdirSync(p);return p;};
 const outputs=Object.fromEntries(bridge.PRODUCTS.map(p=>[p,dir('projection-'+p)])), projectionPins={}, assets={}, binaries={}, packs={};
 const native=JSON.parse(fs.readFileSync(f.nativePath)), evidence=cfg.evidenceOutput, invocations=[];
 for(const p of bridge.PRODUCTS) {
  const bytes=Buffer.from('synthetic binary '+p), asset={file:p+'.bin',sha256:c.digest(bytes),size:bytes.length,binary:{sha256:c.digest(bytes),size:bytes.length}};
  assets[p]={assets:{'linux-amd64':asset}};
  fs.writeFileSync(path.join(outputs[p],asset.file),bytes);
  write(path.join(outputs[p],'release-manifest.json'),{schema_version:3,status:'CANDIDATE',product:p,repository:old.identity.repository,tag:p==='agentplugins'?`agentplugins-v${old.identity.versions[p]}`:`v${old.identity.versions[p]}`,version:old.identity.versions[p],versions:old.identity.versions,authoring_mode:old.authoringMode,asset_scope:'six-platform-pair',assets:assets[p].assets,commit:old.identity.commit,engine_revision:old.identity.commit,candidate_sha256:old.manifestDigest,release_eligible:false,platform_acceptance:false,attested:false});
  fs.writeFileSync(path.join(outputs[p],'checksums.txt'),`${asset.sha256}  ${asset.file}\n${hash(path.join(outputs[p],'release-manifest.json'))}  release-manifest.json\n`);
  projectionPins[p]={manifest_sha256:hash(path.join(outputs[p],'release-manifest.json')),checksums_sha256:hash(path.join(outputs[p],'checksums.txt'))};
  const binary=path.join(f.fixtureRoot,p+'.bin');fs.writeFileSync(binary,bytes);binaries[p]={path:binary,sha256:asset.sha256,size:asset.size};
  const packdir=path.join(evidence,p);fs.mkdirSync(packdir);const pack=path.join(packdir,native.packs[p].file), body=Buffer.from('executed synthetic '+p);fs.writeFileSync(pack,body);
  packs[p]={file:pack,sha256:c.digest(body),size:body.length,integrity:'sha512-'+crypto.createHash('sha512').update(body).digest('base64')};
  const parent=path.join(f.fixtureRoot,`${p} projects ü`);fs.renameSync(f.projects[p],parent);f.projects[p]=parent;
  const add=(argv,author=false,status=0)=>invocations.push({product:p,argv:[...(author&&p==='agentplugins'?['author']:[]),...argv],status,signal:null,stderr:'',stdout:JSON.stringify({schema_version:1,result:'success',data:{engine:'standard-first-slice/1',revision:old.identity.commit,committed:true}})});
  add(['version','--format=json']);add(['--help']);add(['version','--format=json'],true);add(['--help','--format=json'],true);
  for(const lane of bridge.LANES){
   const source=path.join(parent,lane);fs.mkdirSync(path.join(source,'skills'));fs.mkdirSync(path.join(source,'skills/extra-skill'));fs.writeFileSync(path.join(source,'skills/extra-skill/SKILL.md'),'synthetic');
   for(const args of [bridge.publicInit(lane),['skills','init','extra-skill',source,'--description=Disposable fixture.'],['skills','validate',source],...['validate','inspect','test'].map(n=>[n,source])]) add([...args,'--format=json'],true);
  }
 }
 invocations.push({product:'plugin-kit-ai',argv:['update','--all','--format=json'],status:2,signal:null,stdout:'{}',stderr:''},
  {product:'agentplugins',argv:['add',path.join(f.projects.agentplugins,'skill'),'--target=codex','--dry-run','--format=json'],status:0,signal:null,stdout:'{}',stderr:''});
 const tool=path.join(f.root,'tool');fs.writeFileSync(tool,'synthetic tool');const toolpin={path:tool,sha256:hash(tool)};
 const candidate={candidate:true,root:f.candidate,identity:old.identity,manifestDigest:old.manifestDigest,go:tool,workParent:old.workParent,assetScope:'six-platform-pair',authoringMode:old.authoringMode,outputs,pairMarker:path.join(f.root,'pair.json')};
 write(candidate.pairMarker,{schema:'authoring-release-pair/v1',status:'CANDIDATE',identity:old.identity,candidate_sha256:old.manifestDigest,authoring_mode:old.authoringMode,asset_scope:candidate.assetScope,products:projectionPins,release_eligible:false,platform_acceptance:false,attested:false});
 const pairMarkerDigest=hash(candidate.pairMarker), prep={schema:'dual-authoring-public-preparation/v1',identity:old.identity,candidate_sha256:old.manifestDigest,projection_pins:projectionPins,pair_marker_sha256:pairMarkerDigest,wrapper_blobs:{'source.js':{sha256:c.digest(Buffer.from('synthetic source'))}},generated:{},packs:native.packs,tools:{node:toolpin,npm:toolpin},qualification:null,release_eligible:false,platform_acceptance:false,attested:false};
 fs.writeFileSync(path.join(old.repo,'source.js'),'synthetic source');write(f.completionPath,prep);
 const publicCfg={prepare:{candidate,repo:old.repo,node:tool,npm:tool,output:old.output,projectionPins,pairMarkerDigest},completionDigest:hash(f.completionPath),evidenceOutput:evidence};write(f.nativeConfig,publicCfg);
 write(path.join(evidence,'invocations.json'),invocations);fs.writeFileSync(path.join(evidence,'downloads.log'),'synthetic download');
 write(path.join(evidence,'result.json'),{source:old.identity.commit,fixture_acquisition_execution:true,signed_promotion:false,public_eligible:false,runtime_evidence:'not_evaluated'});
 const installations=[...bridge.PRODUCTS,...bridge.PRODUCTS,'agentplugins'].map((p,i)=>({product:p,argv:[tool,'install','--global','--prefix',path.join(f.fixtureRoot,i<2?`${p} independent prefix`:'shared prefix ü'),'--offline','--ignore-scripts','--no-audit','--no-fund',packs[p].file],status:0,signal:null,pack_sha256:packs[p].sha256}));
 const terminal={schema:'dual-authoring-public-native/v1',status:'completed',identity:old.identity,candidate_sha256:old.manifestDigest,completion_sha256:hash(f.completionPath),config_sha256:hash(f.nativeConfig),pair_marker_sha256:pairMarkerDigest,projection_pins:projectionPins,fixtureRoot:f.fixtureRoot,target:'linux-amd64',packs,binaries,installations,tools:{node:toolpin,npm:toolpin,go:toolpin,producer_node:{...toolpin,version:'v22.21.1'}},invocations:invocations.length,invocations_sha256:hash(path.join(evidence,'invocations.json')),downloads_sha256:hash(path.join(evidence,'downloads.log')),result_sha256:hash(path.join(evidence,'result.json')),projects:f.projects,trees:Object.fromEntries(bridge.PRODUCTS.map(p=>[p,bridge.snapshot(f.projects[p])])),fixture_acquisition_execution:true,qualification:null,signed_promotion:false,public_eligible:false,release_eligible:false,platform_acceptance:false,attested:false,runtime_evidence:'not_evaluated'};
 f.nativePath=path.join(evidence,'public-native-completion.json');write(f.nativePath,terminal);
 return {...f,terminal,assets,tool,request:{...f.request,intake:'public-fixture/v1',nativeConfigSha256:hash(f.nativeConfig),nativeCompletionSha256:hash(f.nativePath)}};
}
test('SYNTHETIC public intake: exact lanes, executed packs, pins and schema separation',posixFixture,t=>{
 const original=c.frozenCandidate, f=publicFixture(t);
 c.frozenCandidate=()=>({manifest:{products:f.assets,build:{go_sha256:hash(f.tool)}}});
 try {
  const sealed=path.join(f.root,'sealed.json');const digest=bridge.publishSeal(f.request,sealed);
  assert.throws(()=>bridge.publishSeal(f.request,sealed));
  assert.throws(()=>bridge.publishSeal(f.request,path.join(f.candidate,'overlap.json')));
  assert.throws(()=>bridge.publishSeal(f.request,path.join(f.root,'repo/overlap.json')));
  const verify=()=>bridge.verify(sealed,digest,f.request.expectedCommit);assert.equal(verify().projects.length,10);
  for(const field of ['release_eligible','platform_acceptance','attested','signed_promotion','public_eligible']) for(const value of [true,undefined]) {
   const bad={...f.terminal,[field]:value};write(f.nativePath,bad);
   assert.throws(()=>bridge.seal({...f.request,nativeCompletionSha256:hash(f.nativePath)}),field);
  }
  for(const change of [{schema:'dual-authoring-npm-completion/v1'},{status:'running'},{qualification:{}},{invocations:69},{fixtureRoot:f.root},{identity:{...f.terminal.identity,engine_revision:'0'.repeat(40)}},{packs:{...f.terminal.packs,agentplugins:{...f.terminal.packs.agentplugins,sha256:JSON.parse(fs.readFileSync(f.completionPath)).packs.agentplugins.sha256}}},{projects:{...f.projects,agentplugins:f.projects['plugin-kit-ai']}},{installations:f.terminal.installations.slice(1)}]){
   write(f.nativePath,{...f.terminal,...change});assert.throws(()=>bridge.seal({...f.request,nativeCompletionSha256:hash(f.nativePath)}));
  }
  write(f.nativePath,f.terminal);
  assert.throws(()=>bridge.seal({...f.request,expectedCommit:'0'.repeat(40)}));
  assert.throws(()=>bridge.seal({...f.request,nativeConfigSha256:'0'.repeat(64)}));
  const {intake,...privateRequest}=f.request;assert.throws(()=>bridge.seal(privateRequest));assert.throws(()=>bridge.seal({...f.request,intake:'private'}));
  for(const file of [f.nativeConfig,f.completionPath,path.join(f.root,'pair.json'),path.join(f.root,'projection-agentplugins/checksums.txt'),path.join(f.root,'projection-agentplugins/release-manifest.json'),f.tool,f.terminal.packs.agentplugins.file,f.terminal.binaries.agentplugins.path,path.join(path.dirname(f.nativePath),'invocations.json'),path.join(f.projects.agentplugins,'skill/plugin.json')]){
   const before=fs.readFileSync(file);fs.appendFileSync(file,'changed');assert.throws(verify);fs.writeFileSync(file,before);
  }
  const inv=path.join(path.dirname(f.nativePath),'invocations.json'), before=fs.readFileSync(inv), rows=JSON.parse(before);
  for(const bad of [rows.slice(1),[rows[0],...rows.slice(0,-1)],rows.map((r,i)=>i===5?{...r,status:1}:r)]){
   write(inv,bad);write(f.nativePath,{...f.terminal,invocations:bad.length,invocations_sha256:hash(inv)});assert.throws(()=>bridge.seal({...f.request,nativeCompletionSha256:hash(f.nativePath)}));
  }
  fs.writeFileSync(inv,before);write(f.nativePath,f.terminal);assert.equal(verify().projects.length,10);
 } finally {c.frozenCandidate=original;}
 assert.throws(()=>bridge.seal(f.request),'unstubbed candidate rejects synthetic bytes');
});

// v1 remains independently accepted; v2 must be explicitly selected.
test('SYNTHETIC v2 help/preflight boundary cannot stand for production add', posixFixture, t => {
 const original = c.frozenCandidate, f = publicFixture(t);
 c.frozenCandidate = () => ({manifest:{products:f.assets,build:{go_sha256:hash(f.tool)}}});
 const inv = path.join(path.dirname(f.nativePath), 'invocations.json');
 const oldRows = JSON.parse(fs.readFileSync(inv));
 const help = {product:'agentplugins', argv:['add','--help','--format=json'], status:0, signal:null, stderr:'',
  stdout:JSON.stringify({schema_version:1,command:'help',result:'success',data:{use:'agentplugins add',commands:null}})};
 const rejection = {...help, argv:['add',path.join(f.projects.agentplugins,'skill'),'--target=codex','--scope=project','--dry-run','--format=json'],
  status:1, stdout:'', stderr:'agentplugins: --scope project is not supported by the current client adapters; the public CLI supports user scope only\n'};
 const rows = [...oldRows.slice(0,69), help, rejection];
 const boundary = {executable_observation:'help-and-preflight-rejection',valid_add_dry_run:'not_evaluated',
  argv:oldRows[69].argv,reason:'production-security-inputs-not-offline'};
 const terminal = {...f.terminal,schema:'dual-authoring-public-native/v2',installer_boundary:boundary};
 function seal(changes = {}, observations = rows, intake = 'public-fixture/v2') {
  write(inv, observations);
  write(f.nativePath, {...terminal,invocations:observations.length,invocations_sha256:hash(inv),...changes});
  return bridge.seal({...f.request,intake,nativeCompletionSha256:hash(f.nativePath)});
 }
 try {
  const accepted = seal();
  assert.equal(rows.length,71);
  assert.deepEqual(rows.reduce((n,r)=>(n[r.status]=(n[r.status]||0)+1,n),{}),{0:69,1:1,2:1});
  assert.deepEqual(accepted.inputs.public_evidence.installer_boundary,boundary);
  assert.throws(()=>seal({},rows,'public-fixture/v1'));
  assert.throws(()=>seal({schema:'dual-authoring-public-native/v1'}));
  for (const installer_boundary of [undefined,{},true,{...boundary,valid_add_dry_run:true},
    {...boundary,valid_add_dry_run:'success'},{...boundary,reason:undefined},{...boundary,argv:rejection.argv},
    {...boundary,executable_observation:'successful-add'},{...boundary,accepted:true}]) {
   assert.throws(()=>seal({installer_boundary}));
  }
  for (const bad of [oldRows, rows.slice(0,70), [...rows.slice(0,69),rejection],
    [...rows.slice(0,69),rejection,help], [...rows.slice(0,69),help,help]]) assert.throws(()=>seal({},bad));
  for (const i of [69,70]) for (const change of [{status:0},{status:2},{signal:'SIGTERM'},
    {stdout:'{}'},{stdout:help.stdout+'{}'},{stderr:'arbitrary'},{stderr:''},
    {argv:oldRows[69].argv}]) {
   if (Object.entries(change).every(([k,v])=>JSON.stringify(rows[i][k])===JSON.stringify(v))) continue;
   assert.throws(()=>seal({},rows.map((r,j)=>j===i?{...r,...change}:r)));
  }
  for (const value of [{}, {schema_version:1,command:'help',result:'failure',data:{use:'agentplugins add',commands:null}},
    {schema_version:1,command:'help',result:'success',data:{use:'agentplugins',commands:null}}]) {
   assert.throws(()=>seal({},rows.map((r,i)=>i===69?{...r,stdout:JSON.stringify(value)}:r)));
  }
  assert.deepEqual(seal().inputs.public_evidence.installer_boundary,boundary);
 } finally { c.frozenCandidate = original; }
 assert.throws(()=>seal(), 'synthetic fixtures cannot publish native evidence');
});

// C3-only synthetic seam: bypass the unavailable journey result adapter ONLY
// inside these tests. The real reader and CLI cannot accept this local J.
test('C3 bridge authenticated intake preserves legacy dispatch', t => {
 const a = require('./public-authoring-acceptance');
 const {fixture, withReaders} = require('../test/public-authoring-acceptance.test');
 const f = fixture(t);
 withReaders(t, f, () => {
  assert.throws(() => bridge.seal(f.request), /C3b required/);
  t.mock.method(a, 'readJourney', request => a.readJourneyInputs(request));
  const result = bridge.seal(f.request);
  assert.equal(result.schema, 'packed-installer-bridge/public-authenticated/v1');
  assert.equal(result.inputs.projects.length, 10);
  assert.equal(result.inputs.public_inputs.qualification, null);
  assert.equal(result.attested, false);
  for (const intake of ['public-fixture/v1', 'public-fixture/v2', undefined]) {
   const request = {...f.request}; if (intake) request.intake = intake; else delete request.intake;
   assert.throws(() => bridge.seal(request));
  }
  for (const extra of [{authenticated:true}, {nativeTap:'fixture.tap'}, {disposableEvidence:true}]) assert.throws(() => bridge.seal({...f.request,...extra}));
 });
 assert.throws(() => a.readAcceptance(f.request), /completed remote E/);
});
test('C3 bridge seal binds original ten projects', t => {
 const a = require('./public-authoring-acceptance');
 const {fixture, withReaders} = require('../test/public-authoring-acceptance.test');
 const f = fixture(t);
 withReaders(t, f, () => {
  t.mock.method(a, 'readJourney', request => a.readJourneyInputs(request));
  const sealed = path.join(f.root, 'sealed.json'), pin = bridge.publishSeal(f.request, sealed);
  const verify = () => bridge.verify(sealed, pin, f.request.expectedCommit);
  assert.equal(verify().projects.length, 10);
  for (const output of [path.join(f.admission.work_parent, 'overlap.json'), f.request.admission, f.j.tools.go.path]) {
   assert.throws(() => bridge.publishSeal(f.request, output), /overlapping roots/);
  }
  const source = path.join(f.j.projects.agentplugins, 'skill'), manifest = path.join(source, 'plugin.json');
  const original = fs.readFileSync(manifest), mode = fs.statSync(manifest).mode & 0o777;
  fs.appendFileSync(manifest, 'changed'); assert.throws(verify); fs.writeFileSync(manifest, original);
  fs.chmodSync(manifest, mode ^ 0o020); assert.throws(verify); fs.chmodSync(manifest, mode);
  const extra = path.join(source, 'unexpected-empty'); fs.mkdirSync(extra); assert.throws(verify);
  // Retain the changed fixture; no cleanup and no claim that it still verifies.
  assert.throws(() => bridge.publishSeal(f.request, path.join(f.root, 'late-seal.json')));
  assert.equal(fs.existsSync(path.join(f.root, 'late-seal.json')), false);
 });
});
