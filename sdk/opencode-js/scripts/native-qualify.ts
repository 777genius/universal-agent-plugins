/** Real-host qualification. Run in an isolated directory with installed packed SDK and hosts. */
import { createServer } from 'node:http';
import { spawn, spawnSync, type ChildProcess } from 'node:child_process';
import { mkdir, writeFile, readFile, appendFile } from 'node:fs/promises';
import { join, resolve, isAbsolute } from 'node:path';
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';

type Json = Record<string, any>;
assert(process.argv[2]&&isAbsolute(process.argv[2])&&process.argv[2].includes('TEST-'),'An explicit absolute TEST scratch directory is required');
const root = resolve(process.argv[2]);
const generation = process.argv[3] ?? 'v1';
const tools = join(root, 'tools/node_modules');
const run = join(root, `run-${generation}-${Date.now()}`);
await mkdir(run, { recursive: true });
const evidence = join(run, 'evidence.jsonl');
const record = (value: unknown) => appendFile(evidence, JSON.stringify(value) + '\n');
const provider = createServer(async (req, res) => {
  let raw = ''; for await (const chunk of req) raw += chunk;
  const body = JSON.parse(raw || '{}') as Json;
  const text = JSON.stringify(body.messages ?? body.input ?? []);
  const scenario = ['question','permission','error'].find((x) => text.includes(`QUALIFY_${x}`));
  await record({ provider: req.url, scenario, model: body.model, toolNames: body.tools?.map((x: Json) => x.function?.name ?? x.name) });
  if (scenario === 'error') { res.writeHead(400, { 'content-type': 'application/json' }); res.end(JSON.stringify({error:{message:'qualification permanent failure',type:'invalid_request_error'}})); return; }
  const tool = scenario === 'question' ? 'question' : body.tools?.some((t:Json)=>t.function?.name==='shell')?'shell':'bash';
  const already = (body.messages ?? []).some((m: Json) => m.role === 'tool');
  const call = scenario && !already;
  const args = tool === 'question' ? { questions: [{ question: 'Qualification choice?', header: 'TEST', options: [{label:'yes',description:'Test answer'}] }] } : { command: 'echo native-qualification', description: 'Qualification permission request' };
  res.writeHead(200, { 'content-type': 'text/event-stream', 'cache-control': 'no-cache' });
  const packet = (delta: Json, finish_reason: string | null = null) => res.write('data: ' + JSON.stringify({id:'chatcmpl-native',object:'chat.completion.chunk',created:Math.floor(Date.now()/1000),model:body.model,choices:[{index:0,delta,finish_reason}]}) + '\n\n');
  packet({role:'assistant'});
  if (call) packet({tool_calls:[{index:0,id:`call_${scenario}`,type:'function',function:{name:tool,arguments:JSON.stringify(args)}}]});
  else packet({content:'Qualification completed.'});
  packet({}, call ? 'tool_calls' : 'stop'); res.end('data: [DONE]\n\n');
});
await new Promise<void>((ok) => provider.listen(0,'127.0.0.1',ok));
const providerPort = (provider.address() as {port:number}).port;
const home = join(run,'home'), project = join(run,'TEST-project');
await Promise.all([mkdir(home,{recursive:true}),mkdir(project,{recursive:true})]);
const pluginDirectory = join(run,'native-plugin-package');
await mkdir(pluginDirectory);
const plugin = generation==='v2'?join(pluginDirectory,'index.ts'):join(run,'native-plugin.ts');
const pluginSource = await readFile(new URL(`native-plugin-${generation}.ts`, import.meta.url),'utf8');
await writeFile(plugin,pluginSource);
await writeFile(join(generation==='v2'?pluginDirectory:run,'native-support.ts'),await readFile(new URL('native-support.ts',import.meta.url)));
await writeFile(join(pluginDirectory,'package.json'),JSON.stringify({name:'uap-native-qualification',version:'1.0.0',type:'module',exports:'./index.ts'}));
const baseURL = `http://127.0.0.1:${providerPort}/v1`;
const config = generation === 'v1' ? {
  plugin:[plugin], model:'qualification/test', small_model:'qualification/test', share:'disabled', snapshot:false,
  permission:{'*':'allow',bash:'ask'},
  provider:{qualification:{npm:'@ai-sdk/openai-compatible',name:'Qualification',options:{baseURL,apiKey:'TEST-not-a-secret'},models:{test:{name:'Test',limit:{context:20000,output:2000}}}}}
} : {
  plugins:[pluginDirectory], model:'qualification/test', share:'disabled',snapshots:false,
  permissions:[{action:'*',resource:'*',effect:'allow'},{action:'shell',resource:'*',effect:'ask'}],
  providers:{qualification:{name:'Qualification',package:'@ai-sdk/openai-compatible',settings:{baseURL,apiKey:'TEST-not-a-secret'},models:{test:{name:'Test',capabilities:{tools:true},limit:{context:20000,output:2000}}}}}
};
await writeFile(join(project,'opencode.json'),JSON.stringify(config,null,2));
const port=await new Promise<number>((ok)=>{const reservation=createServer();reservation.listen(0,'127.0.0.1',()=>{const selected=(reservation.address() as {port:number}).port;reservation.close(()=>ok(selected));});});
const command = generation === 'v1' ? join(tools,'opencode-ai/bin/opencode.exe') : join(tools,'@opencode/cli/bin/opencode.exe');
const env = {PATH:process.env.PATH,HOME:home,XDG_CONFIG_HOME:join(home,'config'),XDG_DATA_HOME:join(home,'data'),XDG_CACHE_HOME:join(home,'cache'),XDG_STATE_HOME:join(home,'state'),TMPDIR:join(root,'tmp'),OPENCODE_SERVER_PASSWORD:'TEST-native-qualification',NATIVE_AUTHORITY:join(run,'authority.json'),NATIVE_CONTROL:join(run,'control.json'),NATIVE_EVIDENCE:evidence,NATIVE_SDK:join(tools,'universal-agent-plugins-opencode-events'),NATIVE_TOOLS:tools,OPENCODE_DISABLE_DEFAULT_PLUGINS:'true',OPENCODE_DISABLE_UPDATE_CHECK:'true',OPENCODE_EXPERIMENTAL_DISABLE_FILEWATCHER:'true'};
assert.equal(spawnSync('git',['init','--quiet',project],{env,encoding:'utf8'}).status,0);
const measured=spawnSync(command,['--version'],{cwd:project,env,encoding:'utf8'});
assert.equal(measured.status,0);
const measuredVersionText=measured.stdout.trim();
const nativeVersion=measuredVersionText.replace(/^opencode v/u,'');
assert.equal(nativeVersion,generation==='v1'?'1.19.3':'2.0.32');
const child: ChildProcess = spawn(command,['serve','--hostname','127.0.0.1','--port',String(port),'--print-logs'],{cwd:project,env,detached:true,stdio:['ignore','pipe','pipe']});
child.stdout?.on('data',(c) => { void appendFile(join(run,'host.log'),c); });
child.stderr?.on('data',(c) => { void appendFile(join(run,'host.log'),c); });
const api = async (path: string, body?: unknown, method = body === undefined ? 'GET':'POST'): Promise<any> => {
 const url = `http://127.0.0.1:${port}${path}`;
 const response = await fetch(url,{signal:AbortSignal.timeout(10000),method,headers:{'authorization':'Basic '+Buffer.from('opencode:TEST-native-qualification').toString('base64'),'content-type':'application/json','x-opencode-directory':project},...(body===undefined?{}:{body:JSON.stringify(body)})});
 const text = await response.text();
 if (!response.ok) throw new Error(`${response.status} ${path} ${text}`);
 return text ? JSON.parse(text) : undefined;
};
const rows=async():Promise<Json[]> => (await readFile(evidence,'utf8')).trim().split('\n').filter(Boolean).map((line)=>JSON.parse(line));
const facts=async(sid:string,kind:string)=>(await rows()).filter((r)=>r.fact?.sessionID===sid&&r.fact?.kind===kind);
const fenceProof=async(sid:string,rid:string,close:(row:Json)=>boolean) => {
 await wait(async()=> (await rows()).some((r)=>r.phase==='preparation-exit'&&r.requestID===rid));
 const all=await rows(),closeIndex=all.findIndex(close),exitIndex=all.findIndex((r)=>r.phase==='preparation-exit'&&r.requestID===rid);
 assert(closeIndex>=0&&exitIndex>closeIndex,'Native close/dispose ingress must precede preparation exit');
 const exit=all[exitIndex];assert.equal(exit.aborted,true);assert.equal(exit.current,false);assert(exit.remainingMs>0,'Expiry must not cause the fence');assert.equal(exit.reason,'aborted');
 assert(!all.some((r)=>r.fact?.sessionID===sid&&['question_asked','permission_asked'].includes(r.fact.kind)));
};
const wait = async (check:()=>Promise<boolean>, ms=30000) => {const end=Date.now()+ms; while(Date.now()<end){if(await check())return;await new Promise((r)=>setTimeout(r,100));}throw new Error('qualification timeout');};
try {
 console.log(JSON.stringify({phase:'waiting-server',run}));
 await wait(async()=>{try{await api(generation==='v1'?'/global/health':'/api/info');return true;}catch{return false;}},30000);
 const serverInfo=await api(generation==='v1'?'/global/health':'/api/info');
 assert.equal(serverInfo.version,nativeVersion);
 if(generation==='v2')assert.equal(serverInfo.pid,child.pid);
 await writeFile(join(run,'authority.json'),JSON.stringify({version:nativeVersion,directory:project,pid:child.pid,serverInfo}));
 const sdkFiles:Record<string,string>={};
 for(const name of ['index.js','v1.js','v2.js','v2-reducer.js','observer-core.js','checkpoint.js'])sdkFiles[name]=createHash('sha256').update(await readFile(join(env.NATIVE_SDK,name))).digest('hex');
 const tarball=process.argv[4]??join(root,'candidate/sdk/opencode-js/universal-agent-plugins-opencode-events-0.3.0.tgz');
 const tarballSha256=createHash('sha256').update(await readFile(tarball)).digest('hex');
 await record({phase:'server-ready',generation,project,command,nativeVersion,measuredVersionText,serverInfo,pid:child.pid,port,tarball,tarballSha256,sdkFiles});
 if(generation==='v2'){
  const clientModule=await import('file://'+join(tools,'@opencode/client/dist/promise/client.js')) as typeof import('@opencode/client/promise');
  const client=clientModule.OpenCode.make({baseUrl:`http://127.0.0.1:${port}`,headers:{authorization:'Basic '+Buffer.from('opencode:TEST-native-qualification').toString('base64')}});
  const session=await client.session.create({location:{directory:project},model:{providerID:'qualification',id:'test'}});
  await record({phase:'session-created',session});
  await client.session.prompt({sessionID:session.id,text:'QUALIFY_completion'});
  await wait(async()=> (await readFile(evidence,'utf8')).split('\n').some((line)=>{try{return JSON.parse(line).fact?.kind==='turn_idle_verified';}catch{return false;}}));
  await record({phase:'completion-pass',sessionID:session.id});
  for(const scenario of ['question','permission','question-reply-fence','question-reject-fence','permission-reply-fence','error']) {
   const test=await client.session.create({location:{directory:project},model:{providerID:'qualification',id:'test'},title:scenario});
   const fence=scenario.includes('fence');
   if(fence)await writeFile(join(run,'control.json'),JSON.stringify({sessionID:test.id}));
   await client.session.prompt({sessionID:test.id,text:`QUALIFY_${scenario.split('-')[0]}`});
   if(scenario==='error'){
    await wait(async()=> (await facts(test.id,'terminal_error')).length===1);
    await record({phase:'error-pass',sessionID:test.id});continue;
   }
   const kind=scenario.startsWith('question')?'question_asked':'permission_asked';
   const nativeType=scenario.startsWith('question')?'form.created':'permission.asked';
   await wait(async()=> (await rows()).some((r)=>r.native?.type===nativeType&&(r.native.data?.sessionID??r.native.data?.form?.sessionID)===test.id));
   const asked=(await rows()).find((r)=>r.native?.type===nativeType&&(r.native.data?.sessionID??r.native.data?.form?.sessionID)===test.id)!.native.data;
   const requestID=asked.form?.id??asked.id;
   if(fence)await wait(async()=> (await rows()).some((r)=>r.phase==='preparation-wait'&&r.sessionID===test.id));
   else {await wait(async()=> (await facts(test.id,kind)).length===1);await record({phase:'unanswered-pass',scenario,sessionID:test.id,requestID});}
   if(scenario.startsWith('question')){
    if(scenario.includes('reply'))await client.session.form.reply({sessionID:test.id,formID:requestID,answer:{q0:'yes'}});
    else await client.session.form.cancel({sessionID:test.id,formID:requestID});
   } else await client.permission.reply({sessionID:test.id,requestID,decision:'reject'});
   await wait(async()=> (await rows()).some((r)=>['form.replied','form.cancelled','permission.replied'].includes(r.native?.type)&&(r.native?.data?.requestID??r.native?.data?.id)===requestID));
   if(fence){await fenceProof(test.id,requestID,(r)=>['form.replied','form.cancelled','permission.replied'].includes(r.native?.type)&&(r.native?.data?.requestID??r.native?.data?.id)===requestID);await record({phase:'settlement-fence-pass',scenario,sessionID:test.id,requestID});}
   await client.session.interrupt({sessionID:test.id});
  }
  const before=(await rows()).filter((r)=>r.phase==='plugin-loaded').length;
  const disposal=await client.session.create({location:{directory:project},model:{providerID:'qualification',id:'test'}});
  await writeFile(join(run,'control.json'),JSON.stringify({sessionID:disposal.id}));
  await client.session.prompt({sessionID:disposal.id,text:'QUALIFY_question_dispose'});
  await wait(async()=> (await rows()).some((r)=>r.phase==='preparation-wait'&&r.sessionID===disposal.id));
  const requestID=(await rows()).find((r)=>r.phase==='preparation-wait'&&r.sessionID===disposal.id)!.requestID;
  await record({phase:'native-reload-request',sessionID:disposal.id,requestID});
  await client.location.reload();
  await fenceProof(disposal.id,requestID,(r)=>r.native?.type==='location.shutdown'&&r.native?.location?.directory===project);
  await wait(async()=> (await rows()).some((r)=>r.phase==='plugin-disposed'));
  const disposedRows=await rows();
  assert(disposedRows.findIndex((r)=>r.phase==='plugin-disposed')>disposedRows.findIndex((r)=>r.phase==='preparation-exit'&&r.requestID===requestID));
  await record({phase:'dispose-fence-pass',sessionID:disposal.id,requestID,mode:'native-location-reload'});
  await writeFile(join(run,'control.json'),'{}');
  const reloaded=await client.session.create({location:{directory:project},model:{providerID:'qualification',id:'test'}});
  await client.session.prompt({sessionID:reloaded.id,text:'QUALIFY_completion_reload'});
  await wait(async()=> (await facts(reloaded.id,'turn_idle_verified')).length===1);
  assert((await rows()).filter((r)=>r.phase==='plugin-loaded').length>before);
  await record({phase:'reload-pass',sessionID:reloaded.id});
 }
 else {
  const session = await api('/session',{});
  await api(`/session/${session.id}/prompt_async`,{model:{providerID:'qualification',modelID:'test'},parts:[{type:'text',text:'QUALIFY_completion'}]});
  await wait(async()=> (await facts(session.id,'turn_idle_verified')).length===1);
  await record({phase:'completion-pass',sessionID:session.id});
  for(const scenario of ['question','permission','question-reply-fence','question-reject-fence','permission-reply-fence','error']) {
   const test=await api('/session',{title:scenario});
   const fence=scenario.includes('fence');
   if(fence)await writeFile(join(run,'control.json'),JSON.stringify({sessionID:test.id}));
   await api(`/session/${test.id}/prompt_async`,{model:{providerID:'qualification',modelID:'test'},parts:[{type:'text',text:`QUALIFY_${scenario.split('-')[0]}`} ]});
   if(scenario==='error'){
    await wait(async()=> (await facts(test.id,'terminal_error')).length===1);
    await record({phase:'error-pass',sessionID:test.id});continue;
   }
   const kind=scenario.startsWith('question')?'question_asked':'permission_asked';
   const nativeType=scenario.startsWith('question')?'question.asked':'permission.asked';
   await wait(async()=> (await rows()).some((r)=>r.native?.type===nativeType&&r.native?.properties?.sessionID===test.id));
   const asked=(await rows()).find((r)=>r.native?.type===nativeType&&r.native?.properties?.sessionID===test.id)!.native.properties;
   if(fence)await wait(async()=> (await rows()).some((r)=>r.phase==='preparation-wait'&&r.sessionID===test.id));
   else {await wait(async()=> (await facts(test.id,kind)).length===1);await record({phase:'unanswered-pass',scenario,sessionID:test.id,requestID:asked.id});}
   if(scenario.startsWith('question'))await api(`/question/${asked.id}/${scenario.includes('reply')?'reply':'reject'}`,scenario.includes('reply')?{answers:[['yes']]}:{});
   else await api(`/permission/${asked.id}/reply`,{reply:'reject'});
   await wait(async()=> (await rows()).some((r)=>['question.replied','question.rejected','permission.replied'].includes(r.native?.type)&&r.native?.properties?.requestID===asked.id));
   if(fence){await fenceProof(test.id,asked.id,(r)=>['question.replied','question.rejected','permission.replied'].includes(r.native?.type)&&r.native?.properties?.requestID===asked.id);await record({phase:'settlement-fence-pass',scenario,sessionID:test.id,requestID:asked.id});}
   await api(`/session/${test.id}/abort`,{});
  }
  const loaded=(await rows()).filter((r)=>r.phase==='plugin-loaded');
  const before=loaded.length;
  const disposal=await api('/session',{});
  await writeFile(join(run,'control.json'),JSON.stringify({sessionID:disposal.id}));
  await api(`/session/${disposal.id}/prompt_async`,{model:{providerID:'qualification',modelID:'test'},parts:[{type:'text',text:'QUALIFY_question_dispose'}]});
  await wait(async()=> (await rows()).some((r)=>r.phase==='preparation-wait'&&r.sessionID===disposal.id));
  await writeFile(join(run,'control.json'),JSON.stringify({sessionID:disposal.id,disposeInstance:loaded.at(-1)!.instanceID}));
  const requestID=(await rows()).find((r)=>r.phase==='preparation-wait'&&r.sessionID===disposal.id)!.requestID;
  await fenceProof(disposal.id,requestID,(r)=>r.phase==='dispose-ingress'&&r.instanceID===loaded.at(-1)!.instanceID);
  await record({phase:'dispose-fence-pass',sessionID:disposal.id,requestID,instanceID:loaded.at(-1)!.instanceID,mode:'explicit-consumer-dispose'});
  await api('/instance/dispose',{});
  await wait(async()=> (await rows()).some((r)=>r.phase==='plugin-disposed'));
  await writeFile(join(run,'control.json'),'{}');
  const reloaded=await api('/session',{});
  await api(`/session/${reloaded.id}/prompt_async`,{model:{providerID:'qualification',modelID:'test'},parts:[{type:'text',text:'QUALIFY_completion_reload'}]});
  await wait(async()=> (await facts(reloaded.id,'turn_idle_verified')).length===1);
  assert((await rows()).filter((r)=>r.phase==='plugin-loaded').length>before);
  await record({phase:'reload-pass',sessionID:reloaded.id});
 }
 const all=await rows();
 const loaded=all.filter((r)=>r.phase==='plugin-loaded');
 assert(loaded.length>=2);
 for(const instance of loaded)assert.equal(generation==='v1'?instance.directory:instance.location?.directory,project);
 for(const fenced of all.filter((r)=>['settlement-fence-pass','dispose-fence-pass'].includes(r.phase)))assert(!all.some((r)=>r.fact?.requestID===fenced.requestID));
 for(const row of all.filter((r)=>r.fact)){assert.equal(row.current,true);assert.equal(row.fact.provenance.generation,generation);}
 await writeFile(join(run,'result.json'),JSON.stringify({status:'passed',generation,run},null,2));
 console.log(JSON.stringify({run,evidence,status:'passed'}));
} finally {
 if(child.pid){try{process.kill(-child.pid,'SIGTERM');}catch{}}
 await new Promise<void>((ok)=>{if(child.exitCode!==null||child.signalCode!==null)ok();else child.once('exit',()=>ok());});
 await new Promise<void>((ok)=>provider.close(()=>ok()));
 await record({phase:'owned-host-closed',pid:child.pid,exitCode:child.exitCode,signal:child.signalCode});
}
