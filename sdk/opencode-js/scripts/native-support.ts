import { appendFileSync, readFileSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import type { ObservedEvent, PreparationHandoff } from '../common.js';
export const log=(value:unknown)=>appendFileSync(process.env.NATIVE_EVIDENCE!,JSON.stringify(value)+'\n');
export async function nativePreparation(event:ObservedEvent,handoff:PreparationHandoff):Promise<boolean> {
 if(event.kind!=='question_asked'&&event.kind!=='permission_asked')return true;
 let control:{sessionID:string;release?:boolean};
 try{control=JSON.parse(readFileSync(process.env.NATIVE_CONTROL!,'utf8'));}catch{return true;}
 if(control.sessionID!==event.sessionID)return true;
 log({phase:'preparation-wait',sessionID:event.sessionID,requestID:event.requestID});
 let reason='aborted';
 try {
  while(!handoff.signal.aborted){
   try{const value=JSON.parse(await readFile(process.env.NATIVE_CONTROL!,'utf8'));if(value.release){reason='released';return true;}}catch{}
   await new Promise((r)=>setTimeout(r,5));
  }
  return true;
 } finally {
  log({phase:'preparation-exit',sessionID:event.sessionID,requestID:event.requestID,aborted:handoff.signal.aborted,current:handoff.isCurrent(),remainingMs:handoff.metadataDeadline-performance.now(),reason});
 }
}
