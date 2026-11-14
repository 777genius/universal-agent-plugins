'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const shape = require('./shape.cjs');
const runner = require('./runner.cjs');
function uri(p) {return {scheme:'file', authority:'', path:p, toString:()=>'file://'+p};}
function temporary(t) {
  const root=fs.mkdtempSync(path.join(os.tmpdir(),'uap-observer-'));
  t.after(()=>fs.rmSync(root,{recursive:true,force:true}));
  return root;
}
test('native order and duplicates are preserved without name collapsing',()=>{
 const rows=[{uri:uri('/A/SKILL.md'),name:'same',source:'local',description:'A',disableModelInvocation:true},
             {uri:uri('/USER/SKILL.md'),name:'same',source:'user',description:'USER',disableModelInvocation:true}];
 const actual=shape.skills(rows);
 assert.deepEqual(actual.map(r=>r.uri.path),['/A/SKILL.md','/USER/SKILL.md']);
 assert.deepEqual(actual.map(r=>r.name),['same','same']);
 const mcp=shape.mcp([{label:'first',uri:uri('/one'),headers:{}},{label:'first',command:'/inert',args:[],env:{}}]);
 assert.equal(mcp.length,2); assert.deepEqual(mcp.map(r=>r.kind),['http','stdio']);
});
test('missing malformed and oversized shapes fail closed',()=>{
 for(const value of [undefined,null,{},Array(129).fill({})]) assert.throws(()=>shape.skills(value));
 assert.throws(()=>shape.skills([{uri:uri('/A'),name:'x',source:'future',disableModelInvocation:true}]));
 assert.throws(()=>shape.skills([{uri:uri('/A'),name:'x',source:'local',disableModelInvocation:'true'}]));
 assert.throws(()=>shape.text('\ud800'));assert.throws(()=>shape.text('x'.repeat(8193)));
 assert.throws(()=>shape.mcp([{label:'x',command:'/inert',env:{invalid:true}}]));
});
test('metadata dictionaries do not mutate prototypes',()=>{
 const headers=JSON.parse('{"__proto__":"raw-marker","constructor":"raw-constructor"}');
 const row=shape.mcp([{label:'x',uri:uri('/A'),headers}])[0];
 assert.equal(Object.getPrototypeOf(row.headers),null);
 assert.equal(row.headers.__proto__,'raw-marker');
 assert.equal({}.polluted,undefined);
});
test('extension preserves actual activation context mode',()=>{
 const extension=require('./extension.cjs');
 assert.equal(extension.activate({extensionMode:3}).extensionMode,3);
 assert.equal(extension.activate({extensionMode:1}).extensionMode,1);
 assert.ok(Object.isFrozen(extension.activate({extensionMode:3})));
 const manifest=require('./package.json');
 assert.deepEqual(manifest.enabledApiProposals,['mcpServerDefinitions','chatPromptFiles']);
 assert.equal(manifest.capabilities.untrustedWorkspaces.supported,true);
});
test('observer remains read-only and deliberately pending',()=>{
 const source=fs.readFileSync(path.join(__dirname,'runner.cjs'),'utf8');
 assert.ok(source.includes('await extension.activate()'));
 assert.ok(source.includes('await new Promise(() => {})'));
 for(const forbidden of ['startMcpGateway','executeCommand(','sendChatRequest(','updateConfiguration(','isTrusted =']) assert.ok(!source.includes(forbidden));
});
test('pending writes and transient hardlinks are not final admission signals',t=>{
 const root=temporary(t),output=path.join(root,'observation.json'),ready=path.join(root,'observation.ready');
 const fileSystem={...fs,
  writeFileSync(fd,body){assert.equal(fs.existsSync(output),false);assert.equal(fs.existsSync(ready),false);fs.writeFileSync(fd,body);},
  linkSync(from,to){assert.equal(fs.existsSync(ready),false);fs.linkSync(from,to);assert.equal(fs.statSync(to).nlink,2);},
  unlinkSync(from){fs.unlinkSync(from);assert.equal(fs.statSync(output).nlink,1);assert.equal(fs.existsSync(ready),false);}
 };
 runner.storeResult(output,{complete:true},fileSystem);
 assert.deepEqual(JSON.parse(fs.readFileSync(output)),{complete:true});
 assert.equal(fs.statSync(ready).size,0);
 assert.equal(fs.existsSync(path.join(root,'.observation.pending')),false);
});
test('foreign protocol leaves and aliases are preserved on collision',t=>{
 for(const name of ['observation.json','.observation.pending','observation.ready']){
  const root=path.join(temporary(t),name.replaceAll('.','_'));fs.mkdirSync(root);
  const foreign=path.join(root,name);fs.writeFileSync(foreign,'foreign');
  assert.throws(()=>runner.storeResult(path.join(root,'observation.json'),{complete:true}));
  assert.equal(fs.readFileSync(foreign,'utf8'),'foreign');
 }
 const root=temporary(t),foreign=path.join(root,'foreign'),pending=path.join(root,'.observation.pending');
 fs.writeFileSync(foreign,'outside');fs.symlinkSync(foreign,pending);
 assert.throws(()=>runner.storeResult(path.join(root,'observation.json'),{complete:true}));
 assert.equal(fs.readFileSync(foreign,'utf8'),'outside');assert.ok(fs.lstatSync(pending).isSymbolicLink());
});


test('MCP environment preserves finite numbers and null without widening headers',()=>{
 const raw={label:'inert',command:'/never-run',env:{TEXT:'value',PORT:42,FRACTION:0.5,UNSET:null}};
 const row=shape.mcp([raw])[0];
 assert.equal(row.env.TEXT,'value'); assert.equal(row.env.PORT,42);
 assert.equal(row.env.FRACTION,0.5); assert.equal(row.env.UNSET,null);
 assert.equal(raw.env.PORT,42); assert.equal(Object.getPrototypeOf(row.env),null);
 for(const invalid of [NaN,Infinity,-Infinity,true,{},[]]) assert.throws(()=>shape.mcp([{label:'inert',command:'/never-run',env:{KEY:invalid}}]));
 for(const invalid of [42,null,NaN,Infinity]) assert.throws(()=>shape.mcp([{label:'inert',uri:uri('/A'),headers:{KEY:invalid}}]));
});
test('MCP optional containers default only when undefined and never normalize null',()=>{
 const stdio=shape.mcp([{label:'inert',command:'/never-run'}])[0];
 assert.deepEqual(stdio.args,[]); assert.equal(Object.keys(stdio.env).length,0);
 assert.equal(Object.keys(shape.mcp([{label:'inert',uri:uri('/A')}])[0].headers).length,0);
 for(const key of ['args','env']) assert.throws(()=>shape.mcp([{label:'inert',command:'/never-run',[key]:null}]));
 assert.throws(()=>shape.mcp([{label:'inert',uri:uri('/A'),headers:null}]));
});
test('MCP HTTP and stdio fields cannot silently collapse to one transport',()=>{
 for(const [key,value] of [['command','/never-run'],['args',[]],['env',{}],['cwd',uri('/inert')]]) {
  assert.throws(()=>shape.mcp([{label:'inert',uri:uri('/A'),[key]:value}]));
 }
 assert.throws(()=>shape.mcp([{label:'inert',command:'/never-run',headers:{}}]));
});
