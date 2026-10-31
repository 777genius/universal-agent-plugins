import { nativePreparation } from './native-support.ts';
import { appendFileSync, readFileSync } from 'node:fs';
import { randomUUID } from 'node:crypto';
import { pathToFileURL } from 'node:url';
import type { ObserverOptions } from '../v1.js';
const log=(value:unknown)=>appendFileSync(process.env.NATIVE_EVIDENCE!,JSON.stringify(value)+'\n');
export default async function(input: ObserverOptions & {client: ObserverOptions['client'];directory:string}) {
 const sdk = await import(pathToFileURL(process.env.NATIVE_SDK!+'/v1.js').href) as typeof import('../v1.js');
 const observer=sdk.createObserver({client:input.client,location:input.directory,runtimeEligibility:()=> {const authority=JSON.parse(readFileSync(process.env.NATIVE_AUTHORITY!,'utf8'));return authority.version==='1.19.3'&&authority.directory===input.directory?'supported':'unverified';},callbackAuthority:'qualified_native_sync',beforeEmit:nativePreparation,emit(event,handoff){log({fact:event,current:handoff.isCurrent()});},onDiagnostic(reason){log({diagnostic:reason});}});
 const instanceID=randomUUID();
 const control=setInterval(()=>{try{if(JSON.parse(readFileSync(process.env.NATIVE_CONTROL!,'utf8')).disposeInstance===instanceID){log({phase:'dispose-ingress',instanceID});observer.dispose();clearInterval(control);log({phase:'plugin-disposed',instanceID,mode:'explicit-consumer-dispose'});}}catch{}},5);
 log({phase:'plugin-loaded',generation:'v1',directory:input.directory,instanceID});
 return { event({event}:{event:unknown}) { log({native:event});if((event as {type?:string}).type==='server.instance.disposed'){observer.dispose();log({phase:'plugin-disposed'});}void observer.observe(event);return Promise.resolve(); } };
}
