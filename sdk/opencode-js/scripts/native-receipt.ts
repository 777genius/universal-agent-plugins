import { createHash } from 'node:crypto';
import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';
import assert from 'node:assert/strict';
type Row = Record<string, any>;
const [root,sourceSHA,v1,v2,packPath]=process.argv.slice(2);
assert(root&&sourceSHA&&v1&&v2&&packPath,'root sourceSHA v1Run v2Run packJSON required');
const hash=(data:Uint8Array|string)=>createHash('sha256').update(data).digest('hex');
const cells=[];
for(const [generation,run] of [['v1',v1],['v2',v2]]) {
 const bytes=await readFile(join(run,'evidence.jsonl'));
 const rows=bytes.toString().trim().split('\n').map((line)=>JSON.parse(line)) as Row[];
 const result=JSON.parse(await readFile(join(run,'result.json'),'utf8'));
 assert.equal(result.status,'passed');
 const ready=rows.find((r)=>r.phase==='server-ready')!;
 assert(ready&&rows.some((r)=>r.phase==='owned-host-closed'&&r.pid===ready.pid));
 const expected=['completion-pass','error-pass','dispose-fence-pass','reload-pass'];
 for(const phase of expected)assert(rows.some((r)=>r.phase===phase));
 for(const scenario of ['question','permission'])assert(rows.some((r)=>r.phase==='unanswered-pass'&&r.scenario===scenario));
 for(const scenario of ['question-reply-fence','question-reject-fence','permission-reply-fence'])assert(rows.some((r)=>r.phase==='settlement-fence-pass'&&r.scenario===scenario));
 for(const r of rows.filter((r)=>r.phase==='settlement-fence-pass'||r.phase==='dispose-fence-pass')){
  assert(!rows.some((x)=>x.fact?.sessionID===r.sessionID&&['question_asked','permission_asked'].includes(x.fact.kind)));
  const exitIndex=rows.findIndex((x)=>x.phase==='preparation-exit'&&x.requestID===r.requestID),exit=rows[exitIndex];
  assert(exit&&exit.aborted===true&&exit.current===false&&exit.remainingMs>0&&exit.reason==='aborted');
  const closeIndex=rows.findIndex((x)=>r.phase==='dispose-fence-pass'?(r.mode==='native-location-reload'?x.native?.type==='location.shutdown'&&x.native?.location?.directory===ready.project:x.phase==='dispose-ingress'&&x.instanceID===r.instanceID):['question.replied','question.rejected','permission.replied','form.replied','form.cancelled'].includes(x.native?.type)&&(x.native?.properties?.requestID??x.native?.data?.requestID??x.native?.data?.id)===r.requestID);
  assert(closeIndex>=0&&exitIndex>closeIndex);
  if(r.mode==='native-location-reload')assert(rows.findIndex((x)=>x.phase==='plugin-disposed')>exitIndex);
 }
 for(const [name,digest] of Object.entries(ready.sdkFiles))assert.equal(hash(await readFile(join(root,'tools/node_modules/universal-agent-plugins-opencode-events',name))),digest);
 const fenceProofs=rows.filter((r)=>['settlement-fence-pass','dispose-fence-pass'].includes(r.phase)).map((r)=>({scenario:r.scenario??'dispose',sessionID:r.sessionID,requestID:r.requestID,mode:r.mode,...rows.find((x)=>x.phase==='preparation-exit'&&x.requestID===r.requestID)}));
 const phases=rows.filter((r)=>expected.includes(r.phase)||['unanswered-pass','settlement-fence-pass'].includes(r.phase));
 cells.push({generation,entrypoint:`/${generation}`,mode:'strict-native',run,ready,scenarios:phases,fenceProofs,factCount:rows.filter((r)=>r.fact).length,checkpointConsumed:rows.filter((r)=>r.phase==='checkpoint-consumed').length,checkpointDisposed:rows.filter((r)=>r.phase==='checkpoint-disposed').length,evidenceSha256:hash(bytes),hostLogSha256:hash(await readFile(join(run,'host.log'))),resultSha256:hash(await readFile(join(run,'result.json')))});
}
assert.equal(cells[0].ready.tarballSha256,cells[1].ready.tarballSha256);
const ownedHostCommands=spawnSync('ps',['-eo','pid=,args='],{encoding:'utf8'}).stdout.split('\n').filter((line)=>line.trim().replace(/^\d+\s+/u,'').startsWith(join(root,'tools/node_modules/')));
assert.deepEqual(ownedHostCommands,[],'Owned native host still running');
const receipt={schemaVersion:1,status:'passed',sourceSHA,scope:'Linux current V1/V2 strict native packed SDK qualification',created:new Date().toISOString(),pack:JSON.parse(await readFile(packPath,'utf8')),tarballSha256:cells[0].ready.tarballSha256,cells,ownedHostCommands,uncovered:['older native version floors','AN packaged platform cells','child/fork/workspace variants','full transport replacement failure matrix','external AN consumer delivery']};
const output=join(root,'evidence/native-qualification.json');
await writeFile(output,JSON.stringify(receipt,null,2)+'\n');
console.log(JSON.stringify({output,sha256:hash(await readFile(output))}));
