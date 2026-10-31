"""Vendor-free literal lifecycle wire fixture for the complete ZIP boundary."""
import base64
import copy
import json
import posixpath
from datetime import datetime, timezone

import project_native_windsurf_cycle as cycle


def wire(raw):
    return {"bytes":len(raw),"sha256":cycle.sha(raw),"body":base64.b64encode(raw).decode()}


def build():
    p={"root":"/owned","home":"/owned/home","a":"/owned/project","b":"/owned/project-b","outside":"/owned/outside","client":"/pinned/client/bin/devin","installer":"/held/agentplugins","git":"/usr/bin/git","nonce":"1"*32,"tool_pins":{"installer":cycle.sha(b"installer"),"probe":cycle.sha(b"probe")}}
    order="git-a git-b version empty-user-skills empty-user-mcp empty-a-skills empty-a-mcp empty-b-skills empty-b-mcp add-user user-skills user-mcp user-get add-a a-skills a-mcp a-get b-before-skills b-before-mcp add-b b-skills b-mcp b-get repeat-add-a unchanged-update-a intact-repair-a stable-a-skills stable-b-skills update-a updated-a-skills updated-a-get updated-b-skills damaged-a-skills missing-repair-a repaired-a-skills repaired-a-get remove-a removed-a-skills removed-a-mcp remaining-b-skills remaining-b-mcp repeat-remove-a remove-b final-a-skills final-a-mcp final-b-skills final-b-mcp".split()
    commands,proofs,probes=[],[],[]
    held,trees,profiles,cache={}, {}, {}, {}
    epoch=int(datetime(2026,10,30,14,tzinfo=timezone.utc).timestamp())
    def when(index,fraction=""):
        return f"2026-10-30T14:00:{index:02d}"+fraction+"Z"
    def rec(path,body=None,mode=None,index=0,salt="",fraction=200000000):
        directory=body is None
        return {"Kind":"directory" if directory else "file","Mode":mode or (0o40700 if directory else 0o100600),"Device":1,"Inode":int(cycle.sha((path+salt).encode())[:10],16)+1,"UID":1000,"GID":1000,"Links":2 if directory else 1,"Size":4096 if directory else len(body),"Modified":[epoch+index,fraction],"Changed":[epoch+index,fraction],"SHA256":"" if directory else cycle.sha(body)}
    def empty():return {"Kind":"","Mode":0,"Device":0,"Inode":0,"UID":0,"GID":0,"Links":0,"Size":0,"Modified":[0,0],"Changed":[0,0],"SHA256":""}
    global_root=cycle.state_root(p,"user")
    global_tree={".":rec(global_root),"security":rec(global_root+"/security"),"security/assessments":rec(global_root+"/security/assessments")}
    def addproof(stage,value):proofs.append({"stage":stage,"value":copy.deepcopy(value)})
    for index,stage in enumerate(order):
        selected=cycle.scope(stage);root=cycle.state_root(p,selected);prof=cycle.profile(p,selected)
        start,end=when(index),when(index,".8")
        argv,cwd=cycle.argv(stage,p,held)
        out,err=b"",b""
        if stage in cycle.PUBLIC:
            old=held.get(selected);version="1.1.0" if selected=="a" and index>=28 else "1.0.0"
            revision="a-two" if version=="1.1.0" else selected+"-one"
            n=cycle.nonce(p,selected,version);manifest,skill=cycle.authored(p,selected,version)
            digest="sha256:"+cycle.sha((selected+version).encode())
            iid=("a" if selected=="a" else "b" if selected=="b" else "c")*8+"-1111-4222-8333-444444444444"
            physical=cycle.NAME+"-"+cycle.sha(iid.encode())[:12]
            active=root+"/managed/clients/windsurf/"+physical
            scope_name="user" if selected=="user" else "project"
            bid="client_"+cycle.sha("\0".join((iid,"windsurf",scope_name,active)).encode())[:24]
            did="data_"+cycle.sha("\0".join((iid,physical,scope_name)).encode())[:24]
            locator=root+"/plugin-data/"+physical
            data=old["data"] if old else {"data_receipt_id":did,"physical_backend_id":physical,"scope":scope_name,"locator":locator,"ownership_digest":"sha256:"+cycle.sha(("agentplugins-plugin-data-v1\0"+did+"\0"+locator).encode()),"state":"owned","created_at":start,"updated_at":start}
            changed=stage not in cycle.NOOPS;removed=stage in ("remove-a","remove-b","repeat-remove-a")
            current={"scope":selected,"version":version,"revision":revision,"nonce":n,"source":cycle.source(p,selected),"source_digest":digest,"removed":removed,"active":active,"binding_id":bid,"data":data}
            group="op-"+f"{index:032x}";operation=group+"-001"
            package={"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":cycle.SCHEMA,"declared_name":cycle.NAME,"version":version,"manifest_digest":"sha256:"+cycle.sha(manifest),"inventory":{"skills":[cycle.NAME],"mcp_present":True,"mcp_enabled":True,"mcp_servers":[cycle.NAME]}}
            i={"installation_id":iid,"declared_name":cycle.NAME,"origin_mode":"direct","source":{"source_binding_id":"src_"+cycle.sha((current["source"]+"\0\0").encode())[:24],"requested_source":current["source"],"canonical_source":current["source"],"resolved_revision":"","tree_digest":digest},"package":package,"clients":{},"operation_group_id":group,"data_receipts":{did:data},"created_at":old["installation"]["created_at"] if old else start,"updated_at":start}
            previous_binding=old["binding"] if old else None
            before=previous_binding["receipts"][-1]["after_digest"] if old else None
            receipt={"operation_id":operation,"operation_group_id":group,"sequence":len(previous_binding["receipts"])+1 if old else 1,"mutation_type":"directory_remove" if removed else "directory_swap","client_binding_id":bid,"active_path":active,"backup_path":posixpath.dirname(active)+"/.agentplugins-backup-"+cycle.sha(operation.encode())[:16],"phase":"committed"}
            if before:receipt["before_digest"]=before
            state={"schema_version":5,"installations":[i]}
            payload={}
            if removed:
                i["data_retained"]=True;state["transaction_receipts"]=[receipt];binding=previous_binding
            else:
                server=cycle.entry(p,current);codec="windsurf-mcpServers" if selected=="user" else "devin-mcpServers"
                kind="windsurf_mcp_entry" if selected=="user" else "devin_project_mcp_server"
                payload_digest="sha256:"+cycle.sha(("payload"+selected+version).encode())
                objects=[{"object_id":"package:windsurf:"+physical,"kind":"managed_package_directory","logical_name":cycle.NAME,"path":active,"managed_digest":payload_digest,"protection_class":"managed"},
                    {"object_id":("windsurf:mcp:" if selected=="user" else kind+":")+cycle.NAME,"kind":kind,"logical_name":cycle.NAME,"path":prof+"/mcp_config.json","managed_digest":"sha256:"+cycle.sha(b"agentplugins-native-mcp-v1\0"+codec.encode()+b"\0"+cycle.NAME.encode()+b"\0"+cycle.compact(server)),"protection_class":"managed_entry" if selected=="user" else "managed",**({"source_relative":"mcp.json"} if selected=="user" else {})}]
                if selected!="user":objects.append({"object_id":"devin_project_skill_directory:"+cycle.NAME,"kind":"devin_project_skill_directory","logical_name":cycle.NAME,"path":prof+"/skills/"+cycle.NAME,"source_relative":"skills/"+cycle.NAME,"managed_digest":"sha256:"+cycle.sha(b"f\0SKILL.md\0-\0"+skill+b"\0"),"protection_class":"managed"})
                receipt.update(staging_path=posixpath.dirname(active)+"/.agentplugins-staging-"+cycle.sha(operation.encode())[:16],after_digest=payload_digest)
                binding={"client_binding_id":bid,"client_id":"windsurf","scope":scope_name,"target_locator":active,"physical_artifact_id":physical,"materialization":"materialized","activation":"active" if selected=="user" else "prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"version":version,"tree_digest":digest,"manifest_digest":package["manifest_digest"]},"native_objects":objects,"receipts":([] if old is None else previous_binding["receipts"])+[receipt],"data_receipt_id":did,"affected_surfaces":["windsurf"],"updated_at":start}
                if selected!="user":binding["native_profile_root"]=prof
                i["clients"]={bid:binding}
            if not changed:state=copy.deepcopy(old["state"]);i=state["installations"][0];binding=copy.deepcopy(old["binding"])
            current.update(installation=i,binding=binding,state=state)
            state_raw=cycle.compact(state)
            foreign={"serverUrl":"http://127.0.0.1:9/foreign-user","disabled":True} if selected=="user" else {"url":"http://127.0.0.1:9/foreign-"+selected,"transport":"http","disabled":True}
            native={"mcpServers":{"foreign-"+selected:foreign,**({} if removed else {cycle.NAME:cycle.entry(p,current)})}}
            config_raw=cycle.compact(native)
            marker_raw=("WINDSURF_CALLER_"+selected+"_"+p["nonce"]+"\n").encode();owner_raw=cycle.compact(data)
            tree=copy.deepcopy(global_tree if selected=="user" else trees.get(selected,{".":rec(root)}))
            # Fixed parents are retained; only source-defined parent time fields move.
            for leaf in (posixpath.relpath(active,root),posixpath.relpath(locator,root),"operations-v2"):
                parent=posixpath.dirname(leaf) if leaf!="operations-v2" else leaf
                while parent:
                    tree.setdefault(parent,rec(root+"/"+parent,index=index));parent=posixpath.dirname(parent)
            if stage!="repeat-remove-a":tree["."]["Modified"]=tree["."]["Changed"]=[epoch+index,200000000]
            tree["state-v2.json"]=rec(root+"/state-v2.json",state_raw,index=index,salt=group) if changed else tree["state-v2.json"]
            lock_raw=base64.b64decode(old["mutation_lock"]["body"]) if stage=="repeat-remove-a" else json.dumps({"pid":4321,"acquired_at":start},separators=(",",":")).encode()+b"\n"
            if stage!="repeat-remove-a":tree["mutation.lock"]=rec(root+"/mutation.lock",lock_raw,index=index)
            data_rel=posixpath.relpath(locator,root)
            tree.setdefault(data_rel,rec(locator,index=index))
            tree.setdefault(data_rel+"/.agentplugins-data-owner.json",rec(locator+"/.agentplugins-data-owner.json",owner_raw,index=index))
            if old is None:
                before_data={".":copy.deepcopy(tree[data_rel]),".agentplugins-data-owner.json":copy.deepcopy(tree[data_rel+"/.agentplugins-data-owner.json"])}
                tree[data_rel]["Modified"]=tree[data_rel]["Changed"]=[epoch+index,850000000]
                tree[data_rel+"/caller-owned.txt"]=rec(locator+"/caller-owned.txt",marker_raw,index=index,fraction=850000000)
                after_data={".":tree[data_rel],".agentplugins-data-owner.json":tree[data_rel+"/.agentplugins-data-owner.json"],"caller-owned.txt":tree[data_rel+"/caller-owned.txt"]}
                addproof("caller-marker-"+selected,{"scope":selected,"path":locator+"/caller-owned.txt","started_at":when(index,".81"),"ended_at":when(index,".9"),"marker":wire(marker_raw),"before":before_data,"after":after_data})
            active_rel=posixpath.relpath(active,root)
            for name in list(tree):
                if name==active_rel or name.startswith(active_rel+"/"):del tree[name]
            if not removed:
                namespace="io.github.777genius.agentplugins/managed-stdio-v1/"
                fixed={"plugin.json":manifest,"skills/"+cycle.NAME+"/SKILL.md":skill,"mcp.json":cycle.compact({"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{cycle.NAME:{"type":"stdio",**cycle.entry(p,current)}}}),namespace+"metadata.json":cycle.compact({"protocol":1,"sha256":p["tool_pins"]["installer"],"cliVersion":"0.1.0-development"})}
                payload={name:wire(body) for name,body in fixed.items()}
                fixed.update({"bin/probe":b"probe",namespace+"agentplugins":b"installer"})
                tree[active_rel]=rec(active,index=index)
                for name,body in fixed.items():
                    key=active_rel+"/"+name;tree[key]=rec(root+"/"+key,body,0o100700 if name in ("bin/probe",namespace+"agentplugins") else 0o100600,index=index)
                    parent=posixpath.dirname(key)
                    while parent!=active_rel:tree.setdefault(parent,rec(root+"/"+parent,index=index));parent=posixpath.dirname(parent)
                if not changed:
                    for name,row in trees[selected].items():
                        if name==active_rel or name.startswith(active_rel+"/"):tree[name]=copy.deepcopy(row)
            if selected not in profiles:
                profiles[selected]={".":rec(prof),"skills":rec(prof+"/skills"),"mcp_config.json.agentplugins.lock":rec(prof+"/mcp_config.json.agentplugins.lock",b"")}
            profile_tree=copy.deepcopy(profiles[selected])
            if changed:
                profile_tree["mcp_config.json"]=rec(prof+"/mcp_config.json",config_raw,index=index,salt=group)
                for key in (".","skills"):profile_tree[key]["Modified"]=profile_tree[key]["Changed"]=[epoch+index,200000000]
                for key in list(profile_tree):
                    if key=="skills/"+cycle.NAME or key.startswith("skills/"+cycle.NAME+"/"):del profile_tree[key]
                if selected!="user" and not removed:
                    profile_tree["skills/"+cycle.NAME]=rec(prof+"/skills/"+cycle.NAME,mode=0o40755,index=index)
                    profile_tree["skills/"+cycle.NAME+"/SKILL.md"]=rec(prof+"/skills/"+cycle.NAME+"/SKILL.md",skill,0o100644,index=index)
            subject={"tree_digest":digest,"manifest_digest":package["manifest_digest"]}
            key=cycle.sha("\0".join((digest,subject["manifest_digest"],"lintai","0.1.3",cycle.POLICY["id"],"2",cycle.POLICY["digest"])).encode());name="security/assessments/"+key+".json"
            assessment={"schema_version":1,"subject":subject,"scanner":{"id":"lintai","version":"0.1.3"},"policy":cycle.POLICY,"outcome":"no_blocking_findings","counts":{"blocking":0,"warnings":0,"total":0},"scanned_files":3,"report_digest":"sha256:"+cycle.sha(("report"+version).encode())}
            assessment_raw=json.dumps(assessment,separators=(",",":")).encode()
            prior=sorted(cache);before_cache=copy.deepcopy(global_tree["security/assessments"])
            before_file=copy.deepcopy(global_tree.get(name,empty()))
            if stage in cycle.COLD:
                cache[name]=rec(global_root+"/"+name,assessment_raw,index=index)
                global_tree[name]=cache[name]
                global_tree["security/assessments"]["Modified"]=global_tree["security/assessments"]["Changed"]=[epoch+index,200000000]
            if "remove" not in stage:global_tree["."]["Modified"]=global_tree["."]["Changed"]=[epoch+index,200000000]
            cache_proof={"role":"assessment","scope":selected,"version":version,"source_digest":digest,"manifest_digest":subject["manifest_digest"],"path":global_root+"/"+name,"key":key,"phase":"cold" if stage in cycle.COLD else "existing","prior":prior,"assessment":wire(assessment_raw),"before_file":before_file,"after_file":cache[name],"before_parent":before_cache,"after_parent":global_tree["security/assessments"]}
            if selected=="user":
                for k,v in global_tree.items():
                    if k=="security" or k.startswith("security/"):tree[k]=copy.deepcopy(v)
                global_tree=copy.deepcopy(tree)
            trees[selected]=copy.deepcopy(tree);profiles[selected]=profile_tree
            project={".":rec(p["outside" if selected=="user" else selected])}
            if selected!="user":
                for prefix,frame in ((".uap",tree),(".devin",profile_tree)):
                    for k,v in frame.items():project[prefix+("" if k=="." else "/"+k)]=v
            frames={root:tree,prof:profile_tree,p["outside" if selected=="user" else selected]:project,global_root:copy.deepcopy(global_tree)}
            value={k:current[k] for k in ("scope","version","revision","nonce","source","source_digest","removed")}
            value.update(state=wire(state_raw),native_config=wire(config_raw),cache=cache_proof,data_marker=wire(marker_raw),frames=frames,mutation_lock=wire(lock_raw),data_owner=wire(owner_raw),payload_files=payload)
            if stage=="update-a":
                authored_mcp={"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{cycle.NAME:{"type":"stdio","command":"./bin/probe","args":["Owned Windsurf a-two","${PLUGIN_ROOT}/argument"],"cwd":"${PLUGIN_ROOT}","env":{"UAP_TEST_ROOT":p["root"],"UAP_TEST_EVENTS":p["root"]+"/events.jsonl","UAP_TEST_NONCE":n,"UAP_TEST_REVISION":revision}}}}
                addproof("source-a-two",{"source":current["source"],"revision":revision,"nonce":n,"source_digest":digest,"manifest":wire(manifest),"skill":wire(skill),"mcp":wire(cycle.compact(authored_mcp))})
            addproof(stage,value)
            held[selected]={**current,**value,"state":state,"installation":i,"binding":binding,"data":data}
            if stage=="repeat-remove-a":err=cycle.REFUSAL
            else:
                verb=argv[1]
                outcome={"installation_id":iid,"requires_confirmation":False,"mutated":changed}
                if removed:outcome.update(plugin=cycle.NAME,client_id="windsurf",affected_surfaces=["windsurf"],group_phase="external_completed",deactivation={"activation":"not_required","artifact_removal_allowed":True,"external_removal_complete":True})
                else:
                    outcome.update(plan={"client_id":"windsurf","scope":scope_name,"status":"ready","package_mode":"prepared_package","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated","physical_artifact_id":physical,"components":[{"kind":"skill","name":cycle.NAME,"support":"prepared"},{"kind":"mcp_server","name":cycle.NAME,"support":"prepared"}]},activation={"activation":"active" if selected=="user" else "prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified"})
                    if stage!="repeat-add-a":outcome["group_phase"]="external_completed"
                    if stage in ("repeat-add-a","unchanged-update-a"):outcome["no_change"]=True
                output={"plugin":cycle.NAME,"version":version,"source":"local" if verb in ("repair","remove") else "direct local source","tree_digest":digest,"manifest_digest":package["manifest_digest"],"dry_run":False,"result":outcome}
                data_out=output
                if stage!="repeat-add-a":
                    output["operation_id"]=group;target={"target":"windsurf","status":"external_completed","output":output}
                    if verb=="update":target["selected"]=True
                    data_out={"operation_id":group,"batch":True,"status":"data_retained" if removed else "completed","succeeded":1,"failed":0,"plugin":cycle.NAME,"dry_run":False,"targets":[target]}
                    if verb=="add":data_out["environment"]={"reason":"no_save_requested","saved":False}
                    if removed:data_out.update(data_retained=True,plugin_data_preserved=True)
                if stage in ("add-user","add-a","add-b","repeat-add-a"):data_out["security"]={**assessment,"evidence_source":"cache" if stage=="repeat-add-a" else "local_scan"}
                out=cycle.compact({"schema_version":1,"command":verb,"result":"success","data":data_out});err=cycle.progress(stage)
        elif stage=="version":out=b"devin 3000.11.205 (6dea39440ae3)\n"
        elif not stage.startswith("git-"):
            if stage=="damaged-a-skills":addproof("missing-owned-a-skill",{"scope":"a","path":cycle.profile(p,"a")+"/skills/"+cycle.NAME,"skill_sha256":cycle.sha(cycle.authored(p,"a","1.1.0")[1]),"started_at":when(index-1,".9"),"ended_at":when(index-1,".95")})
            out=native_body(p,held,stage)
        commands.append({"stage":stage,"value":{"argv":argv,"cwd":cwd,"exit":1 if stage=="repeat-remove-a" else 0,"error":False,"deadline":False,"elapsed_ms":800,"started_at":start,"ended_at":end,"cleanup":cycle.CLEANUP,"stdout":wire(out),"stderr":wire(err)}})
        probes.append({"stage":stage,"value":{"events":wire(b"")}})
    result={"acquisition":{},"activation":"prepared","attempt_stage":"final-b-mcp","capture_mode":cycle.MODE,"capture_review_required":False,"client":"windsurf","component_layout":{k:v for k,v in p.items() if k not in ("nonce","tool_pins")},"elapsed_ms":48000,"fixture_authored_updater_config":False,"qualification":{key:key in cycle.FLAGS[:2] for key in cycle.FLAGS},"real_model_qualified":False,"runtime_attempt_complete":True,"scanner":{},"source_identity":{"installer_base_commit":"a"*40,"installer_tree":"b"*40,"installer_source_state":"committed","installer_patch_sha256":[]},"test_failed":False,"vendor_authentication_qualified":False,"version":"3000.11.205","qualification_scope":cycle.SCOPE,"cycle_command_stages":order,"cycle_proof_stages":[v["stage"] for v in proofs],"cycle_observed_complete":True,"cycle_nonce":p["nonce"],"cycle_tool_pins":p["tool_pins"]}
    return commands,proofs,probes,result


def native_body(p,held,stage):
    s=cycle.scope(stage);selected=held.get(s);owned=selected is not None and not selected["removed"] and stage!="damaged-a-skills"
    if stage.endswith("-skills"):
        rows=[cycle.skillrow("foreign-user","Harmless foreign marker","Windsurf",cycle.profile(p,"user")+"/skills/foreign-user"),cycle.skillrow("devin-cli","Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)","Builtin","/pinned/client/share/devin",("model",)),cycle.skillrow("declarative-repo-setup","Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo","Builtin",""),cycle.skillrow("upload-secrets","Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation","Builtin","")]
        if s!="user":rows.append(cycle.skillrow("foreign-"+s,"Harmless foreign marker","Devin",cycle.profile(p,s)+"/skills/foreign-"+s))
        if owned and s!="user":rows.append(cycle.skillrow(cycle.NAME,selected["revision"],"Devin",cycle.profile(p,s)+"/skills/"+cycle.NAME))
        return cycle.compact(rows)
    chosen=selected if owned else held.get("user")
    source=cycle.profile(p,s)+"/mcp_config.json" if owned and s!="user" else "~/.codeium/windsurf/mcp_config.json"
    rendered="" if chosen is None else cycle.entry(p,chosen)["command"]+" "+" ".join(cycle.entry(p,chosen)["args"])
    if stage.endswith("-get"):return ("Server: "+cycle.NAME+"\nSource: "+source+"\n    Command: "+rendered+"\n    Env: PLUGIN_DATA=<redacted>, PLUGIN_ROOT=<redacted>, UAP_TEST_EVENTS=<redacted>, UAP_TEST_NONCE=<redacted>, UAP_TEST_REVISION=<redacted>, UAP_TEST_ROOT=<redacted>\n").encode()
    blocks=[]
    if chosen is not None:blocks.append("  • "+cycle.NAME+"\n    Command: "+rendered+"\n    Source: "+source)
    for foreign in sorted({"user",s}):blocks.append("  ✗ foreign-"+foreign+"  (disabled)\n    URL: http://127.0.0.1:9/foreign-"+foreign+"\n    Source: "+("~/.codeium/windsurf/mcp_config.json" if foreign=="user" else cycle.profile(p,foreign)+"/mcp_config.json"))
    return ("Configured MCP servers\n\n"+"\n\n".join(blocks)+"\n\n").encode()
