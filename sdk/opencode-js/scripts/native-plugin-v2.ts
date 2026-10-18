import { nativePreparation } from './native-support.ts';
import { appendFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';
import type { Plugin } from '@opencode/plugin';
import type { V2Envelope, V2NativePorts, V2Run, V2SessionProof, V2CheckpointPort } from '../v2.js';
const log=(value:unknown)=>appendFileSync(process.env.NATIVE_EVIDENCE!,JSON.stringify(value)+'\n');
const plugin: Plugin.Plugin = { id:'uap-native-qualification', async setup(ctx) {
 const sdk = await import(pathToFileURL(process.env.NATIVE_SDK!+'/v2.js').href) as typeof import('../v2.js');
 const observed = new Map<string,V2Envelope[]>();
 const fields=(e:V2Envelope)=>e.data as Record<string,any>;
 const native: V2NativePorts = {
  correlate(e) {
   const p=fields(e), form=p.form;
   const sid = form?.sessionID ?? p.sessionID;
   if(!sid) return;
   const durable=(e as V2Envelope&{durable?:{aggregateID:string;seq:number;version:number}}).durable;
   if(['session.text.delta','session.usage.updated'].includes(e.type))return;
   const history=observed.get(sid)??[];history.push(e);observed.set(sid,history);
   const scope = e.location as {directory?:string;workspaceID?:string}|undefined;
   if(scope?.workspaceID!==undefined) return {sessionID:sid,location:'unsupported-workspace'};
   return {sessionID:sid,location:scope?.directory,messageID:p.assistantMessageID??form?.metadata?.tool?.messageID??p.source?.messageID,
    requestID:form?.id??p.requestID??(e.type==='permission.asked'||e.type.startsWith('form.')?p.id:undefined),callID:form?.metadata?.tool?.id??p.source?.id,
    ...(durable?{sequence:{aggregate:durable.aggregateID,seq:durable.seq,version:durable.version}}:{}),
    ...(p.type==='compaction'?{compaction:true}:{})};
  },
  async session(run,signal) {
   if(signal.aborted)return;
   const info=await ctx.session.get({sessionID:run.sessionID});
   const history=observed.get(run.sessionID)??[];
   const start=history.findLast((e)=>e.type==='session.execution.started');
   const user=history.findLast((e)=>e.type==='session.inbox.delivered');
   const scoped=history.findLast((e)=>(e.location as {directory?:string})?.directory===ctx.location.directory);
   if(signal.aborted||info.id!==run.sessionID||info.parentID!==undefined||info.location.directory!==ctx.location.directory||info.projectID!==ctx.location.project.id||!scoped||start?.id!==run.turnID||!user||fields(user).inboxID!==run.userID)return;
   return {...run,rootSession:true};
  },
  async assistant(run,signal) {
   const proof=await native.session(run,signal);if(!proof)return;
   const rows=await ctx.session.context({sessionID:run.sessionID});
   const answer=rows.findLast((m)=>m.type==='assistant');
   const terminal=(observed.get(run.sessionID)??[]).find((e)=>e.id===run.terminalEventID);
   const last=rows.at(-1),user=rows.findLast((m)=>m.type==='user');
   if(signal.aborted||!answer||answer.id!==run.messageID||!answer.time.completed||user?.id!==run.userID||last?.type!=='idle'||!terminal||last.outcome!==(terminal.type==='session.execution.succeeded'?'succeeded':'failed'))return;
   log({phase:'terminal-native-proof',terminalEventID:terminal.id,assistantID:answer.id,idleID:last.id,idleOutcome:last.outcome,idleCreated:last.time.created,assistantCompleted:answer.time.completed,terminalCreated:terminal.created});
   return {...proof,messageID:answer.id,role:'assistant',summary:false,final:true,outcome:answer.error?'error':'success'};
  },
  async questionSource(run,signal) {
   const proof=await native.session(run,signal);if(!proof)return;
   const rows=await ctx.session.context({sessionID:run.sessionID});
   const answer=rows.findLast((m)=>m.type==='assistant');
   const tool=answer?.content.find((p)=>p.type==='tool'&&p.id===run.callID&&p.name==='question');
   if(signal.aborted||answer?.id!==run.messageID||!tool)return;
   return {...proof,requestID:run.requestID,messageID:run.messageID,callID:run.callID,role:'assistant',tool:'question',summary:false};
  },
  async currentPermission(run,signal) {
   const proof=await native.session(run,signal);if(!proof)return;
   const pending=await ctx.permission.get({sessionID:run.sessionID,requestID:run.requestID});
   if(signal.aborted||pending.id!==run.requestID||pending.sessionID!==run.sessionID||pending.source?.type!=='tool')return;
   return {...proof,requestID:run.requestID,messageID:pending.source.messageID,callID:pending.source.id,pending:true};
  }
 };
 const checkpoint: V2CheckpointPort = {async register(signal,namespace) {
  const id='uap_native_'+namespace;
  const registered=await ctx.rpc.register({id,methods:{},events:{checkpoint:{schema:{type:'object',properties:{nonce:{type:'string',minLength:32,maxLength:32}},required:['nonce'],additionalProperties:false}}}},{});
  log({phase:'checkpoint-registered',id});
  return {type:`rpc.${id}.checkpoint`,async emit(nonce){if(signal.aborted)return;try{await registered.events.emit('checkpoint',{nonce});}catch(error){log({phase:'checkpoint-emit-error',error:String(error)});throw error;}},read(e){const p=fields(e);if(e.type===`rpc.${id}.checkpoint`&&Object.keys(p).length===1&&typeof p.nonce==='string'&&/^[0-9a-f]{32}$/.test(p.nonce)){log({phase:'checkpoint-consumed',eventID:e.id});return p.nonce;}},async dispose(){await registered.dispose();log({phase:'checkpoint-disposed',id});}};
 }};
 const context={app:ctx.app,event:{async *subscribe({signal}:{signal:AbortSignal}) {try{for await(const e of ctx.event.subscribe({signal})){log({native:e});yield e;}}catch(error){log({phase:'reader-error',error:String(error)});throw error;}}}};
 const observer=sdk.createV2Observer({context,location:ctx.location.directory,runtimeEligibility:(version)=>version==='2.0.32'?'supported':'unverified',native,checkpoint,beforeEmit:nativePreparation,emit(event,handoff){log({fact:event,current:handoff.isCurrent()});},onDiagnostic(reason){log({diagnostic:reason});}});
 observer.start();log({phase:'plugin-loaded',generation:'v2',app:ctx.app,location:ctx.location});
 return async()=>{observer.dispose();await observer.done();log({phase:'plugin-disposed'});};
}};
export default plugin;
