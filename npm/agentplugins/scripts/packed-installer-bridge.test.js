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
test("SYNTHETIC intake control and fail-closed mutations (not packed-native acceptance)", t => {
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
test("snapshot rejects links and captures empty files/directories and modes", t => {
 const root=fs.mkdtempSync(path.join(os.tmpdir(),"packed-snapshot-SYNTHETIC-"));
 fs.mkdirSync(path.join(root,"empty")); fs.writeFileSync(path.join(root,"file"),"");
 const first=bridge.snapshot(root); assert.equal(first.entries.length,3);
 fs.chmodSync(path.join(root,"empty"),(fs.statSync(path.join(root,"empty")).mode & 0o777) ^ 0o020); assert.notEqual(bridge.snapshot(root).sha256,first.sha256);
 fs.symlinkSync("file",path.join(root,"link")); assert.throws(()=>bridge.snapshot(root));
 assert.equal(bridge.snapshot(root,true).entries.length,4);
 fs.unlinkSync(path.join(root,"link")); fs.symlinkSync("/etc/passwd",path.join(root,"link")); assert.throws(()=>bridge.snapshot(root,true));
});
