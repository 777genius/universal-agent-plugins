"""Closed Windsurf project skills lifecycle; no file, network or process access."""
import base64
import hashlib
import json
import posixpath
import re

from project_native_windsurf_components import LABELS as FIRST, NAME, _layout

MODE = "windsurf-complete-project-skills-cycle"
MEMBERS = ("windsurf-cycle-commands.jsonl", "windsurf-cycle-proofs.jsonl", "windsurf-cycle-probe.jsonl")
STAGES = FIRST + tuple("repeat-add-a unchanged-update-a intact-repair-a stable-a-skills stable-b-skills update-a updated-a-skills updated-a-get updated-b-skills damaged-a-skills missing-repair-a repaired-a-skills repaired-a-get remove-a removed-a-skills removed-a-mcp remaining-b-skills remaining-b-mcp repeat-remove-a remove-b final-a-skills final-a-mcp final-b-skills final-b-mcp".split())
PUBLIC = tuple("add-user add-a add-b repeat-add-a unchanged-update-a intact-repair-a update-a missing-repair-a remove-a repeat-remove-a remove-b".split())
COLD = frozenset(("add-user", "add-a", "add-b", "update-a"))
NOOPS = frozenset(("repeat-add-a", "unchanged-update-a", "repeat-remove-a"))
PROOFS = tuple("caller-marker-user add-user caller-marker-a add-a caller-marker-b add-b repeat-add-a unchanged-update-a intact-repair-a source-a-two update-a missing-owned-a-skill missing-repair-a remove-a repeat-remove-a remove-b".split())
FLAGS = tuple("project-lifecycle native-restart-removal mcp-initialize mcp-tools-list mcp-tools-call".split())
CLEANUP = "full descendant supervision; forced cleanup fails"
SCOPE = "Project skills metadata lifecycle and restart/removal; MCP configuration visibility only, no MCP initialize/list/call, authentication or model"
REFUSAL = b'agentplugins: plugin is not installed for target "windsurf" in project scope; no target was changed\n'
SCHEMA = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
POLICY = {"id":"agent-plugin-install","version":2,"digest":"sha256:9cf869e299d847d7078aeca01f5a182fcdb0144bbf513c83290459991c79e037"}
RECORD = "Kind Mode Device Inode UID GID Links Size Modified Changed SHA256"


def require(condition):
    if not condition:
        raise ValueError("Windsurf complete project lifecycle evidence differs")


def obj(value, required, optional=""):
    require(type(value) is dict and set(required.split()) <= set(value) <= set((required + " " + optional).split()))


def load(raw):
    from project_native_runtime import strict_json
    return strict_json(raw)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def compact(value):
    return json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=False).encode()


def same(a, b):
    if type(a) is not type(b):
        return False
    if type(a) is dict:
        return set(a) == set(b) and all(same(a[k],v) for k,v in b.items())
    if type(a) is list:
        return len(a)==len(b) and all(same(x,y) for x,y in zip(a,b))
    return a == b


def private(raw):
    from project_native_runtime import cline_cycle_private
    return cline_cycle_private(raw,b"uap-windsurf-private-")


def stream(value):
    obj(value,"bytes sha256 body")
    require(type(value["bytes"]) is int and 0 <= value["bytes"] <= 1 << 20 and type(value["body"]) is str and len(value["body"]) <= ((1<<20)+2)//3*4)
    raw=base64.b64decode(value["body"],validate=True)
    require(len(raw)==value["bytes"] and sha(raw)==value["sha256"] and base64.b64encode(raw).decode()==value["body"] and not private(raw))
    raw.decode("utf-8",errors="strict")
    return raw


def stamp(value):
    from project_native_copilot_events import instant
    require(type(value) is str and re.fullmatch(r"\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,9})?Z",value))
    return instant(value)


def command(value):
    obj(value,"argv cwd exit error deadline elapsed_ms cleanup started_at ended_at stdout stderr")
    require(type(value["argv"]) is list and value["argv"] and all(type(x) is str and "\0" not in x for x in value["argv"]) and type(value["cwd"]) is str and value["cwd"].startswith("/")
            and type(value["exit"]) is int and type(value["error"]) is bool and type(value["deadline"]) is bool and type(value["elapsed_ms"]) is int and value["cleanup"]==CLEANUP)
    start,end=stamp(value["started_at"]),stamp(value["ended_at"])
    require(0 <= end-start <= 35*10**9 and value["elapsed_ms"]==(end-start)//10**6)
    return stream(value["stdout"]),stream(value["stderr"]),start,end


def aggregate_private(raw):
    return len(raw)>1<<20 or any(private(line) for line in raw.splitlines())


def records(raw,order):
    require(type(raw) is bytes and len(raw)<=1<<20 and (not raw or raw.endswith(b"\n")) and not aggregate_private(raw))
    rows=[load(line) for line in raw.splitlines()]
    require(len(rows)<=len(order))
    for row,stage in zip(rows,order):
        obj(row,"stage value")
        require(row["stage"]==stage and type(row["value"]) is dict)
    return rows


def scope(stage):
    if stage in FIRST:
        from project_native_windsurf_components import _scope
        return _scope(stage)
    return "b" if "-b-" in stage or stage.endswith("-b") else "a"


def state_root(paths,selected):
    return paths["root"]+"/installer-state" if selected=="user" else paths[selected]+"/.uap"


def profile(paths,selected):
    return paths["home"]+"/.codeium/windsurf" if selected=="user" else paths[selected]+"/.devin"


def source(paths,selected):
    return paths["root"]+"/component-source-"+selected


def nonce(paths,selected,version):
    return sha((paths["nonce"]+("a-two" if version=="1.1.0" else selected)).encode())[:32]


def authored(paths,selected,version):
    revision="a-two" if version=="1.1.0" else selected+"-one"
    manifest={"$schema":SCHEMA,"name":NAME,"version":version,"description":"Disposable Windsurf component observation"}
    skill=("---\nname: "+NAME+"\ndescription: "+revision+"\n---\nUAP_WINDSURF_"+revision+"_"+nonce(paths,selected,version)+"\n").encode()
    return compact(manifest),skill


def entry(paths,selected):
    active,data=selected["active"],selected["data"]["locator"]
    tag="a-two" if selected["version"]=="1.1.0" else selected["scope"]
    return {"command":active+"/io.github.777genius.agentplugins/managed-stdio-v1/agentplugins", "args":["--internal-stdio-v1",active,data,active,"plugin","--","./bin/probe","Owned Windsurf "+tag,active+"/argument"],
            "env":{"PLUGIN_ROOT":active,"PLUGIN_DATA":data,"UAP_TEST_ROOT":paths["root"],"UAP_TEST_EVENTS":paths["root"]+"/events.jsonl","UAP_TEST_NONCE":selected["nonce"],"UAP_TEST_REVISION":selected["revision"]}}


def record(value,empty=False):
    obj(value,RECORD)
    require(value["Kind"] in (("",) if empty else ("file","directory")) and all(type(value[k]) is int and value[k]>=0 for k in ("Mode","Device","Inode","UID","GID","Links")) and type(value["Size"]) is int)
    for key in ("Modified","Changed"):
        require(type(value[key]) is list and len(value[key])==2 and all(type(x) is int for x in value[key]) and 0<=value[key][1]<10**9)
    if empty:
        require(same(value,{"Kind":"","Mode":0,"Device":0,"Inode":0,"UID":0,"GID":0,"Links":0,"Size":0,"Modified":[0,0],"Changed":[0,0],"SHA256":""}))
    if not empty:
        require(value["Inode"]>0 and value["Links"]>0 and 0<=value["Size"]<=64<<20 and value["Mode"]&0o170000==(0o100000 if value["Kind"]=="file" else 0o40000))
        require(value["SHA256"]=="" if value["Kind"]=="directory" else type(value["SHA256"]) is str and re.fullmatch(r"[0-9a-f]{64}",value["SHA256"]))
    return value


def authority(a,b):
    return all(same(a[k],b[k]) for k in ("Kind","Mode","Device","Inode","UID","GID","Links"))


def times(value,start,end):
    return all(start <= value[k][0]*10**9+value[k][1] <= end for k in ("Modified","Changed"))


def frame(value):
    require(type(value) is dict and 0<len(value)<=4096 and "." in value)
    for name,row in value.items():
        require(type(name) is str and name!="" and "\0" not in name and (name=="." or not name.startswith("/") and posixpath.normpath(name)==name and ".." not in name.split("/")))
        record(row)
        require(row["UID"]==value["."]["UID"] and row["GID"]==value["."]["GID"])
    return value


def receipt(row,selected,group,sequence,kind,before,after):
    base=posixpath.dirname(selected["active"])
    operation=group+"-001"
    expected={"operation_id":operation,"operation_group_id":group,"sequence":sequence,"mutation_type":kind,"client_binding_id":selected["binding_id"],"active_path":selected["active"],"backup_path":base+"/.agentplugins-backup-"+sha(operation.encode())[:16],"phase":"committed"}
    if before: expected["before_digest"]=before
    if after: expected["after_digest"]=after
    if kind=="directory_swap":expected["staging_path"]=base+"/.agentplugins-staging-"+sha(operation.encode())[:16]
    require(re.fullmatch(r"op-[0-9a-f]{32}",group) and same(row,expected))


def installation(value,paths,previous,stage,cmd):
    obj(value,"scope version revision nonce source source_digest removed state native_config cache data_marker frames mutation_lock data_owner payload_files")
    selected=scope(stage)
    version="1.1.0" if selected=="a" and STAGES.index(stage)>=STAGES.index("update-a") else "1.0.0"
    revision="a-two" if version=="1.1.0" else selected+"-one"
    require(value["scope"]==selected and value["version"]==version and value["revision"]==revision and value["nonce"]==nonce(paths,selected,version) and value["source"]==source(paths,selected)
            and type(value["source_digest"]) is str and re.fullmatch(r"sha256:[0-9a-f]{64}",value["source_digest"]))
    raw=stream(value["state"]);state=load(raw)
    obj(state,"schema_version installations","transaction_receipts")
    require(type(state["schema_version"]) is int and state["schema_version"]==5 and type(state["installations"]) is list and len(state["installations"])==1)
    i=state["installations"][0]
    obj(i,"installation_id declared_name origin_mode source package clients operation_group_id data_receipts created_at updated_at","data_retained")
    iid=i["installation_id"]
    require(type(iid) is str and re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}",iid) and i["declared_name"]==NAME and i["origin_mode"]=="direct")
    physical=NAME+"-"+sha(iid.encode())[:12]
    root=state_root(paths,selected);active=root+"/managed/clients/windsurf/"+physical
    scope_name="user" if selected=="user" else "project"
    data_id="data_"+sha("\0".join((iid,physical,scope_name)).encode())[:24]
    binding_id="client_"+sha("\0".join((iid,"windsurf",scope_name,active)).encode())[:24]
    manifest,skill=authored(paths,selected,version)
    src=source(paths,selected)
    require(same(i["source"],{"source_binding_id":"src_"+sha((src+"\0\0").encode())[:24],"requested_source":src,"canonical_source":src,"resolved_revision":"","tree_digest":value["source_digest"]}))
    package={"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":SCHEMA,"declared_name":NAME,"version":version,"manifest_digest":"sha256:"+sha(manifest),"inventory":{"skills":[NAME],"mcp_present":True,"mcp_enabled":True,"mcp_servers":[NAME]}}
    require(same(i["package"],package) and type(i["data_receipts"]) is dict and set(i["data_receipts"])=={data_id})
    data=i["data_receipts"][data_id]
    obj(data,"data_receipt_id physical_backend_id scope locator ownership_digest state","created_at updated_at")
    locator=root+"/plugin-data/"+physical
    require(all(data.get(k)==v for k,v in {"data_receipt_id":data_id,"physical_backend_id":physical,"scope":scope_name,"locator":locator,"state":"owned","ownership_digest":"sha256:"+sha(("agentplugins-plugin-data-v1\0"+data_id+"\0"+locator).encode())}.items()))
    current={**value,"installation":i,"data":data,"active":active,"binding_id":binding_id,"state_raw":raw}
    start,end=stamp(cmd["started_at"]),stamp(cmd["ended_at"])
    require(start<=stamp(i["updated_at"])<=end if stage not in NOOPS else True)
    require(stamp(i["created_at"])<=end and all(stamp(data[k])<=end for k in ("created_at","updated_at") if k in data))
    if previous is not None:
        require(iid==previous["installation"]["installation_id"] and same(data,previous["data"]) and i["created_at"]==previous["installation"]["created_at"])
    else:
        require(start<=stamp(i["created_at"]))
    removed=stage in ("remove-a","repeat-remove-a","remove-b")
    require(value["removed"] is removed)
    if removed:
        require(previous is not None and i.get("data_retained") is True and i["clients"]=={} and len(state.get("transaction_receipts",[]))==1
                and same(i["source"],previous["installation"]["source"]) and same(i["package"],previous["installation"]["package"]))
        b=previous["binding"]
        receipt(state["transaction_receipts"][0],current,i["operation_group_id"],len(b["receipts"])+1,"directory_remove",b["receipts"][-1]["after_digest"],None)
    else:
        require("data_retained" not in i and not state.get("transaction_receipts") and type(i["clients"]) is dict and set(i["clients"])=={binding_id})
        b=i["clients"][binding_id]
        obj(b,"client_binding_id client_id scope target_locator physical_artifact_id materialization activation authentication policy verification package_revision native_objects receipts data_receipt_id affected_surfaces updated_at","native_profile_root")
        expected={"client_binding_id":binding_id,"client_id":"windsurf","scope":scope_name,"target_locator":active,"physical_artifact_id":physical,"materialization":"materialized","activation":"active" if selected=="user" else "prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"version":version,"tree_digest":value["source_digest"],"manifest_digest":package["manifest_digest"]},"data_receipt_id":data_id,"affected_surfaces":["windsurf"]}
        require(all(same(b.get(k),v) for k,v in expected.items()) and b.get("native_profile_root","")==("" if selected=="user" else profile(paths,selected)))
        native=entry(paths,current);codec="windsurf-mcpServers" if selected=="user" else "devin-mcpServers"
        kind="windsurf_mcp_entry" if selected=="user" else "devin_project_mcp_server"
        objects=b["native_objects"]
        require(type(objects) is list and len(objects)==(2 if selected=="user" else 3))
        package_object=[row for row in objects if row.get("kind")=="managed_package_directory"]
        require(len(package_object)==1 and re.fullmatch(r"sha256:[0-9a-f]{64}",package_object[0].get("managed_digest","")))
        digest=package_object[0]["managed_digest"]
        expected_objects=[{"object_id":"package:windsurf:"+physical,"kind":"managed_package_directory","logical_name":NAME,"path":active,"managed_digest":digest,"protection_class":"managed"},
            {"object_id":("windsurf:mcp:" if selected=="user" else kind+":")+NAME,"kind":kind,"logical_name":NAME,"path":profile(paths,selected)+"/mcp_config.json","managed_digest":"sha256:"+sha(b"agentplugins-native-mcp-v1\0"+codec.encode()+b"\0"+NAME.encode()+b"\0"+compact(native)),"protection_class":"managed_entry" if selected=="user" else "managed",**({"source_relative":"mcp.json"} if selected=="user" else {})}]
        if selected!="user":
            kind="devin_project_skill_directory"
            expected_objects.append({"object_id":kind+":"+NAME,"kind":kind,"logical_name":NAME,"path":profile(paths,selected)+"/skills/"+NAME,"source_relative":"skills/"+NAME,"managed_digest":"sha256:"+sha(b"f\0SKILL.md\0-\0"+skill+b"\0"),"protection_class":"managed"})
        require(same(sorted(objects,key=lambda x:x["kind"]),sorted(expected_objects,key=lambda x:x["kind"])))
        receipts=b["receipts"]
        old=[] if previous is None else previous["binding"]["receipts"]
        require(type(receipts) is list)
        if stage in NOOPS:
            require(previous is not None and same(b,previous["binding"]) and same(i,previous["installation"]) and raw==previous["state_raw"])
        else:
            require(len(receipts)==len(old)+1 and same(receipts[:-1],old))
            receipt(receipts[-1],current,i["operation_group_id"],len(receipts),"directory_swap",old[-1]["after_digest"] if old else None,digest)
    if stage=="repeat-remove-a":
        require(previous is not None and same(i,previous["installation"]) and raw==previous["state_raw"])
    current["binding"]=b
    native_config=load(stream(value["native_config"]))
    foreign={"serverUrl":"http://127.0.0.1:9/foreign-user","disabled":True} if selected=="user" else {"url":"http://127.0.0.1:9/foreign-"+selected,"transport":"http","disabled":True}
    servers={"foreign-"+selected:foreign}
    if not removed:servers[NAME]=entry(paths,current)
    require(same(native_config,{"mcpServers":servers}))
    marker=stream(value["data_marker"])
    require(marker==("WINDSURF_CALLER_"+selected+"_"+paths["nonce"]+"\n").encode())
    roots={root,profile(paths,selected),paths["outside" if selected=="user" else selected],state_root(paths,"user")}
    require(type(value["frames"]) is dict and set(value["frames"])==roots)
    frames={root:frame(value) for root,value in value["frames"].items()}
    native_skill_frame(frames[profile(paths,selected)],selected,removed,skill)
    state_file=frames[root]["state-v2.json"]
    require(state_file["Kind"]=="file" and state_file["Mode"]==0o100600 and state_file["Links"]==1 and state_file["Size"]==len(raw) and state_file["SHA256"]==sha(raw))
    config_file=frames[profile(paths,selected)]["mcp_config.json"]
    config_raw=stream(value["native_config"])
    require(config_file["Mode"]==0o100600 and config_file["Links"]==1 and config_file["Size"]==len(config_raw) and config_file["SHA256"]==sha(config_raw))
    data_rel=posixpath.relpath(locator,root)
    require(frames[root][data_rel+"/caller-owned.txt"]["SHA256"]==sha(marker))
    owner=stream(value["data_owner"])
    require(same(load(owner),data) and frames[root][data_rel+"/.agentplugins-data-owner.json"]["Mode"]==0o100600 and frames[root][data_rel+"/.agentplugins-data-owner.json"]["Links"]==1 and frames[root][data_rel+"/.agentplugins-data-owner.json"]["Size"]==len(owner) and frames[root][data_rel+"/.agentplugins-data-owner.json"]["SHA256"]==sha(owner))
    lock_raw=stream(value["mutation_lock"]);lock=load(lock_raw)
    obj(lock,"pid acquired_at")
    require(type(lock["pid"]) is int and lock["pid"]>0 and lock_raw==json.dumps({"pid":lock["pid"],"acquired_at":lock["acquired_at"]},separators=(",",":")).encode()+b"\n")
    lock_record=frames[root]["mutation.lock"]
    require(lock_record["Mode"]==0o100600 and lock_record["Links"]==1 and lock_record["Size"]==len(lock_raw) and lock_record["SHA256"]==sha(lock_raw))
    if stage=="repeat-remove-a":
        require(previous is not None and lock_raw==stream(previous["mutation_lock"]) and same(previous["frames"][root]["mutation.lock"],lock_record) and same(frames,previous["frames"]))
    else:
        require(start<=stamp(lock["acquired_at"])<=end and times(lock_record,start,end))
    if previous is not None:
        require(authority(previous["frames"][root]["mutation.lock"],lock_record))
    payload(current,paths)
    return current


def assessment(value,paths,selected,stage,cmd,known,global_before):
    obj(value,"role scope version source_digest manifest_digest path key phase prior assessment before_file after_file before_parent after_parent")
    manifest,_=authored(paths,selected["scope"],selected["version"])
    subject={"tree_digest":selected["source_digest"],"manifest_digest":"sha256:"+sha(manifest)}
    key=sha("\0".join((subject["tree_digest"],subject["manifest_digest"],"lintai","0.1.3",POLICY["id"],"2",POLICY["digest"])).encode())
    root=state_root(paths,"user");name="security/assessments/"+key+".json"
    require(value["role"]=="assessment" and value["scope"]==selected["scope"] and value["version"]==selected["version"] and value["source_digest"]==selected["source_digest"] and value["manifest_digest"]==subject["manifest_digest"] and value["path"]==root+"/"+name and value["key"]==key and value["prior"]==sorted(known))
    raw=stream(value["assessment"]);a=load(raw)
    obj(a,"schema_version subject scanner policy outcome counts scanned_files report_digest")
    require(type(a["schema_version"]) is int and a["schema_version"]==1 and same(a["subject"],subject) and same(a["scanner"],{"id":"lintai","version":"0.1.3"}) and same(a["policy"],POLICY)
            and a["outcome"]=="no_blocking_findings" and same(a["counts"],{"blocking":0,"warnings":0,"total":0}) and type(a["scanned_files"]) is int and a["scanned_files"]==3 and re.fullmatch(r"sha256:[0-9a-f]{64}",a["report_digest"]))
    ordered={"schema_version":1,"subject":subject,"scanner":{"id":"lintai","version":"0.1.3"},"policy":POLICY,"outcome":"no_blocking_findings","counts":{"blocking":0,"warnings":0,"total":0},"scanned_files":3,"report_digest":a["report_digest"]}
    require(raw==json.dumps(ordered,separators=(",",":")).encode())
    cold=stage in COLD;require(value["phase"]==("cold" if cold else "existing"))
    before,after=record(value["before_parent"]),record(value["after_parent"])
    if global_before is not None:
        require(same(before,global_before["security/assessments"]))
        require(same(value["before_file"],global_before.get(name,{"Kind":"","Mode":0,"Device":0,"Inode":0,"UID":0,"GID":0,"Links":0,"Size":0,"Modified":[0,0],"Changed":[0,0],"SHA256":""})))
    file=record(value["after_file"])
    require(same(selected["frames"][root][name],file) and same(selected["frames"][root]["security/assessments"],after) and authority(before,after) and before["Mode"]==0o40700 and before["SHA256"]==after["SHA256"]=="" and 0<=before["Size"]<=1<<20 and 0<=after["Size"]<=1<<20)
    require(file["Mode"]==0o100600 and file["Links"]==1 and file["Device"]==after["Device"] and file["UID"]==after["UID"] and file["GID"]==after["GID"] and file["Size"]==len(raw) and file["SHA256"]==sha(raw))
    start,end=stamp(cmd["started_at"]),stamp(cmd["ended_at"])
    if cold:
        record(value["before_file"],True)
        require(name not in known and len(known)<4 and times(file,start,end) and times(after,start,end))
        known[name]=file
    else:
        require(name in known and same(value["before_file"],file) and same(known[name],file) and same(before,after))
    cache={p:r for p,r in selected["frames"][root].items() if p.startswith("security/assessments/")}
    require(same(cache,known))
    return a


def public_result(raw,stage,selected,assessment_body):
    d=load(raw);obj(d,"schema_version command result data")
    verb="repair" if "repair" in stage else "update" if "update" in stage else "remove" if stage.startswith("remove") else "add"
    require(type(d["schema_version"]) is int and d["schema_version"]==1 and d["command"]==verb and d["result"]=="success")
    data=d["data"];require(type(data) is dict)
    security=[]
    def walk(value):
        if type(value) is dict:
            for k,v in value.items():
                if k=="security":security.append(v)
                else:walk(v)
        elif type(value) is list:
            for v in value:walk(v)
    walk(data)
    if stage in ("add-user","add-a","add-b","repeat-add-a"):
        require(len(security)==1 and "security" in data)
        expected={**assessment_body,"evidence_source":"cache" if stage=="repeat-add-a" else "local_scan"}
        require(same(data["security"],expected))
    else:require(not security)
    output=data
    if stage!="repeat-add-a":
        obj(data,"operation_id batch status succeeded failed plugin dry_run targets","version source tree_digest manifest_digest security action_required environment plugin_data_preserved data_retained retained_data retained_data_action")
        require(data["batch"] is True and type(data["succeeded"]) is int and data["succeeded"]==1 and type(data["failed"]) is int and data["failed"]==0 and data["plugin"]==NAME and data["dry_run"] is False
                and data["status"]==("data_retained" if verb=="remove" else "completed") and type(data["targets"]) is list and len(data["targets"])==1)
        target=data["targets"][0];obj(target,"target status output","selected next_action")
        require(target["target"]=="windsurf" and target["status"]=="external_completed")
        if verb=="update":require(target.get("selected") is True)
        output=target["output"]
        require(output.get("operation_id")==data["operation_id"])
        if stage not in NOOPS:require(data["operation_id"]==selected["installation"]["operation_group_id"])
        if verb=="add":require(same(data.get("environment"),{"reason":"no_save_requested","saved":False}))
        if verb=="remove":require(data["data_retained"] is True and data["plugin_data_preserved"] is True)
    obj(output,"plugin version source tree_digest manifest_digest dry_run result","next_action security" if stage=="repeat-add-a" else "operation_id next_action")
    label="local" if verb in ("repair","remove") else "direct local source"
    require(output["plugin"]==NAME and output["version"]==selected["version"] and output["source"]==label and output["tree_digest"]==selected["source_digest"] and output["manifest_digest"]==selected["installation"]["package"]["manifest_digest"] and output["dry_run"] is False)
    row=output["result"]
    keys="installation_id requires_confirmation mutated"
    if verb=="remove":
        obj(row,keys+" plugin client_id deactivation affected_surfaces group_phase")
        require(row["plugin"]==NAME and row["client_id"]=="windsurf" and row["affected_surfaces"]==["windsurf"] and same(row["deactivation"],{"activation":"not_required","artifact_removal_allowed":True,"external_removal_complete":True}))
    else:
        keys+=" plan activation"
        if stage!="repeat-add-a":keys+=" group_phase"
        if stage in ("repeat-add-a","unchanged-update-a"):keys+=" no_change";require(row.get("no_change") is True)
        obj(row,keys)
        plan=row["plan"];activation=row["activation"]
        obj(plan,"client_id scope status package_mode activation authentication policy verification physical_artifact_id components","user_actions warnings")
        obj(activation,"activation authentication policy verification","user_actions")
        expected={"client_id":"windsurf","scope":"user" if selected["scope"]=="user" else "project","status":"ready","package_mode":"prepared_package","activation":"prepared","authentication":"not_checked","policy":"allowed","verification":"package_validated","physical_artifact_id":selected["binding"]["physical_artifact_id"]}
        require(all(same(plan[k],v) for k,v in expected.items()))
        require(same(plan["components"],[{"kind":"skill","name":NAME,"support":"prepared"},{"kind":"mcp_server","name":NAME,"support":"prepared"}]))
        expected={"activation":"active" if selected["scope"]=="user" else "prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified"}
        require(all(activation[k]==v for k,v in expected.items()))
    require(row["installation_id"]==selected["installation"]["installation_id"] and row["requires_confirmation"] is False and row["mutated"] is (stage not in NOOPS))
    if stage!="repeat-add-a":require(row["group_phase"]=="external_completed")


def argv(stage,paths,held):
    selected=scope(stage);cwd=paths["outside" if selected=="user" else selected]
    if stage in ("git-a","git-b"):return [paths["git"],"-c","core.hooksPath=/dev/null","init","--quiet"],cwd
    if stage=="version":return [paths["client"],"--version"],cwd
    if stage in PUBLIC:
        verb="repair" if "repair" in stage else "update" if "update" in stage else "remove" if "remove" in stage else "add"
        identifier=source(paths,selected) if verb=="add" else held[selected]["installation"]["installation_id"]
        args=[paths["installer"],verb,identifier]
        if verb=="add":args+=["--no-save"]
        args+=["--scope","user" if selected=="user" else "project","--target","windsurf","--format","json"]
        if selected!="user":args+=["--project-root",cwd]
        return args,cwd
    args=["skills","list","--json"] if stage.endswith("-skills") else ["mcp","get",NAME] if stage.endswith("-get") else ["mcp","list"]
    return [paths["client"],*args],cwd


def skillrow(name,description,provider,base,triggers=("model","user")):
    return {"name":name,"description":description,"provider":provider,"base_dir":base,"display_name":name,"triggers":list(triggers),"warnings":[],"errors":[]}


def native(raw,stage,paths,held):
    if stage=="version":require(raw==b"devin 3000.11.205 (6dea39440ae3)\n");return
    selected=scope(stage)
    current=held.get(selected)
    owned=current is not None and not current["removed"] and stage!="damaged-a-skills"
    if stage.endswith("-skills"):
        expected={"devin-cli":skillrow("devin-cli","Look up Devin CLI documentation (skills, extensibility, configuration, commands, models, troubleshooting)","Builtin",posixpath.dirname(posixpath.dirname(paths["client"]))+"/share/devin",("model",)),
            "declarative-repo-setup":skillrow("declarative-repo-setup","Generate and verify a working environment.yaml (Devin snapshot-setup blueprint) for a repo","Builtin",""),
            "upload-secrets":skillrow("upload-secrets","Securely upload local secrets (dotenv files, env vars, API keys) to the Devin Cloud secrets manager — values never enter the conversation","Builtin",""),
            "foreign-user":skillrow("foreign-user","Harmless foreign marker","Windsurf",profile(paths,"user")+"/skills/foreign-user")}
        if selected!="user":expected["foreign-"+selected]=skillrow("foreign-"+selected,"Harmless foreign marker","Devin",profile(paths,selected)+"/skills/foreign-"+selected)
        if owned and selected!="user":expected[NAME]=skillrow(NAME,current["revision"],"Devin",profile(paths,selected)+"/skills/"+NAME)
        rows=load(raw);require(type(rows) is list and len(rows)==len(expected))
        seen=set()
        for row in rows:
            obj(row,"name description provider base_dir display_name triggers warnings errors")
            require(type(row["name"]) is str and row["name"] not in seen and type(row["triggers"]) is list and all(type(x) is str for x in row["triggers"]))
            require(same({**row,"triggers":sorted(row["triggers"])},expected.get(row["name"])))
            seen.add(row["name"])
        return
    chosen=current if owned else held.get("user")
    source_path=profile(paths,selected)+"/mcp_config.json" if owned and selected!="user" else "~/.codeium/windsurf/mcp_config.json"
    rendered="" if chosen is None else entry(paths,chosen)["command"]+" "+" ".join(entry(paths,chosen)["args"])
    if stage.endswith("-get"):
        expected="Server: "+NAME+"\nSource: "+source_path+"\n    Command: "+rendered+"\n    Env: PLUGIN_DATA=<redacted>, PLUGIN_ROOT=<redacted>, UAP_TEST_EVENTS=<redacted>, UAP_TEST_NONCE=<redacted>, UAP_TEST_REVISION=<redacted>, UAP_TEST_ROOT=<redacted>\n"
        require(chosen is not None and raw==expected.encode());return
    blocks=set()
    if chosen is not None:blocks.add("  • "+NAME+"\n    Command: "+rendered+"\n    Source: "+source_path)
    for foreign in {"user",selected}:
        src="~/.codeium/windsurf/mcp_config.json" if foreign=="user" else profile(paths,foreign)+"/mcp_config.json"
        blocks.add("  ✗ foreign-"+foreign+"  (disabled)\n    URL: http://127.0.0.1:9/foreign-"+foreign+"\n    Source: "+src)
    body=raw.decode();require(body.startswith("Configured MCP servers\n\n") and body.endswith("\n\n"))
    actual=body[len("Configured MCP servers\n\n"):-2].split("\n\n")
    require(len(actual)==len(blocks) and set(actual)==blocks)


def cross_frames(paths,current,previous,global_before,cmd,stage):
    selected=current["scope"];root=state_root(paths,selected);frames=current["frames"]
    start,end=stamp(cmd["started_at"]),stamp(cmd["ended_at"])
    if selected!="user":
        project=frames[paths[selected]]
        for prefix,tree in ((".uap",frames[root]),(".devin",frames[profile(paths,selected)])):
            for key,row in tree.items():require(same(project[prefix+("" if key=="." else "/"+key)],row))
        global_now=frames[state_root(paths,"user")]
        require(global_before is not None)
        for name,old in global_before.items():
            now=global_now.get(name)
            if same(old,now):continue
            if stage in COLD and name=="security/assessments":continue
            if name=="." and "remove" not in stage:
                require(now is not None and authority(old,now) and old["Size"]==now["Size"] and old["SHA256"]==now["SHA256"]=="" and times(now,start,end));continue
            require(False)
        new=set(global_now)-set(global_before)
        require(new==({"security/assessments/"+current["cache"]["key"]+".json"} if stage in COLD else set()))
    if previous is None:return
    old_frames=previous["frames"]
    old_profile=old_frames[profile(paths,selected)];new_profile=frames[profile(paths,selected)]
    owned="skills/"+NAME
    for name,old in old_profile.items():
        now=new_profile.get(name)
        if same(old,now):continue
        require(stage not in NOOPS)
        if name in (".","skills"):
            require(now is not None and all(old[k]==now[k] for k in ("Kind","Mode","Device","Inode","UID","GID")) and times(now,start,end));continue
        require(name in ("mcp_config.json",owned,owned+"/SKILL.md"))
    require(set(new_profile)-set(old_profile)<= {owned,owned+"/SKILL.md"})
    old_state,new_state=old_frames[root],frames[root]
    active=posixpath.relpath(current["active"],root)
    data=posixpath.relpath(current["data"]["locator"],root)
    parents={".","operations-v2"}
    for leaf in (active,data):
        parent=posixpath.dirname(leaf)
        while parent!=".":parents.add(parent);parent=posixpath.dirname(parent) or "."
    for name,old in old_state.items():
        now=new_state.get(name)
        if same(old,now):continue
        if name=="mutation.lock":require(now is not None and authority(old,now));continue
        if name in parents:
            require(now is not None and all(old[k]==now[k] for k in ("Kind","Mode","Device","Inode","UID","GID")) and times(now,start,end));continue
        require(stage not in NOOPS and (name=="state-v2.json" or name==active or name.startswith(active+"/")))
    for name in set(new_state)-set(old_state):require(stage not in NOOPS and (name==active or name.startswith(active+"/")))
    require(all(same(row,new_state.get(name)) for name,row in old_state.items() if name==data or name.startswith(data+"/")))


def marker(value,paths,selected,cmd):
    obj(value,"scope path started_at ended_at marker before after")
    scope_name=selected["scope"];directory=selected["data"]["locator"]
    require(value["scope"]==scope_name and value["path"]==directory+"/caller-owned.txt")
    raw=stream(value["marker"])
    require(raw==stream(selected["data_marker"]))
    before,after=frame(value["before"]),frame(value["after"])
    require(set(before)=={".",".agentplugins-data-owner.json"} and set(after)=={*before,"caller-owned.txt"} and same(before[".agentplugins-data-owner.json"],after[".agentplugins-data-owner.json"]))
    a,b=before["."],after["."]
    start,end=stamp(value["started_at"]),stamp(value["ended_at"])
    require(stamp(cmd["ended_at"])<=start<=end and authority(a,b) and a["Mode"]==0o40700 and a["SHA256"]==b["SHA256"]=="" and 0<=b["Size"]<=1<<20 and times(b,start,end))
    file=after["caller-owned.txt"]
    require(file["Mode"]==0o100600 and file["Links"]==1 and file["UID"]==b["UID"] and file["GID"]==b["GID"] and file["Device"]==b["Device"] and file["Size"]==len(raw) and file["SHA256"]==sha(raw) and times(file,start,end))
    tree=selected["frames"][state_root(paths,scope_name)];relative=posixpath.relpath(directory,state_root(paths,scope_name))
    require(all(same(tree[relative+("" if key=="." else "/"+key)],row) for key,row in after.items()))
    return end


def revision(value,paths,current):
    obj(value,"source revision nonce source_digest manifest mcp skill")
    manifest,skill=authored(paths,"a","1.1.0")
    require(value["source"]==source(paths,"a") and value["revision"]=="a-two" and value["nonce"]==nonce(paths,"a","1.1.0") and value["source_digest"]==current["source_digest"] and stream(value["manifest"])==manifest and stream(value["skill"])==skill)
    expected={"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{NAME:{"type":"stdio","command":"./bin/probe","args":["Owned Windsurf a-two","${PLUGIN_ROOT}/argument"],"cwd":"${PLUGIN_ROOT}","env":{"UAP_TEST_ROOT":paths["root"],"UAP_TEST_EVENTS":paths["root"]+"/events.jsonl","UAP_TEST_NONCE":value["nonce"],"UAP_TEST_REVISION":"a-two"}}}}
    require(stream(value["mcp"])==compact(expected))


def pins(result,paths):
    a=result["acquisition"];obj(a,"client acquisition node runtime_files")
    require(a["client"]=="windsurf" and a["node"] is None)
    acq=a["acquisition"];obj(acq,"sha256 pin_verified members")
    require(acq["sha256"]=="2e65c8b93682258fc91f5e4cead92e3698338cebf1b31dbc32a3201cb63705c4" and acq["pin_verified"] is True and type(acq["members"]) is list and len(acq["members"])==250
            and sha(compact(acq["members"]))=="7942bfe5cb303d6b4ba5f2c87e869ad24c1b3400de323aa9615341896e3e6884")
    root=posixpath.dirname(posixpath.dirname(paths["client"]))
    files=[{"path":root+"/"+row["Name"],"sha256":row["sha256"]} for row in acq["members"] if row["Kind"]=="file"]
    require(paths["client"]==root+"/bin/devin" and len(files)==234 and same(a["runtime_files"],files))
    require(same(result["scanner"],{"archive_sha256":"2b3d176db752433b904a4b42375543ff398f4841d22e48f7d4f23ded925b72da","binary_sha256":"b6723451f155eb49e84acb34d1f06de929bddfcff6238cd4e38cdc6519396fba","platform":"linux-amd64","route":"prepopulated supported scanner cache; ordinary CLI security assessment","source_tag":"v0.1.3","version":"0.1.3"}))


def progress(stage):
    if stage.startswith("add-"):return ("Resolving and validating one Agent Plugin package for every selected target...\nInstalling "+NAME+" for 1 selected clients...\n").encode()
    if stage=="repeat-add-a":return b"Resolving and validating one Agent Plugin package for every selected target...\nApplying transactional client package...\n"
    if "update" in stage:return b"Resolving and validating one updated Agent Plugin package for every selected target...\n"
    if "repair" in stage:return b"Resolving and validating each unique exact installed package revision once...\n"
    return b""


def complete(commands,proof_rows,probes,result):
    paths={**_layout(result["component_layout"]),"nonce":result["cycle_nonce"],"tool_pins":result["cycle_tool_pins"]}
    obj(paths["tool_pins"],"installer probe")
    require(all(type(v) is str and re.fullmatch(r"[0-9a-f]{64}",v) for v in paths["tool_pins"].values()))
    require(type(paths["nonce"]) is str and re.fullmatch(r"[0-9a-f]{32}",paths["nonce"]))
    pins(result,paths)
    proof_map={row["stage"]:row["value"] for row in proof_rows}
    held,known,groups={},{},set()
    first,previous,total,global_frame=None,None,0,None
    marker_ends={}
    for frame_row,probe in zip(commands,probes):
        stage,cmd=frame_row["stage"],frame_row["value"]
        out,err,start,end=command(cmd)
        require(previous is None or previous<=start)
        if first is None:first=start
        previous_end=previous
        previous,total=end,total+cmd["elapsed_ms"]
        expected,cwd=argv(stage,paths,held)
        require(cmd["argv"]==expected and cmd["cwd"]==cwd and cmd["error"] is False and cmd["deadline"] is False and cmd["exit"]==(1 if stage=="repeat-remove-a" else 0))
        native_stage=stage not in PUBLIC and not stage.startswith("git-")
        require(end-start<=(15 if native_stage and stage!="version" else 35)*10**9)
        if stage in PUBLIC:
            if stage=="repeat-remove-a":require(out==b"" and err==REFUSAL)
            else:require(err==progress(stage))
            selected=scope(stage);old=held.get(selected)
            current=installation(proof_map[stage],paths,old,stage,cmd)
            a=assessment(current["cache"],paths,current,stage,cmd,known,global_frame)
            cross_frames(paths,current,old,global_frame,cmd,stage)
            if stage!="repeat-remove-a":public_result(out,stage,current,a)
            if old is None:marker_ends[selected]=marker(proof_map["caller-marker-"+selected],paths,current,cmd)
            if stage=="update-a":
                require(old["source_digest"]!=current["source_digest"]);revision(proof_map["source-a-two"],paths,current)
            if stage not in NOOPS:
                group=current["installation"]["operation_group_id"];require(group not in groups);groups.add(group)
            held[selected]=current
            global_frame=current["frames"][state_root(paths,"user")]
        elif stage.startswith("git-"):require(out==err==b"")
        else:
            require(err==b"");native(out,stage,paths,held)
            if stage=="damaged-a-skills":
                fault=proof_map["missing-owned-a-skill"];obj(fault,"scope path skill_sha256 started_at ended_at")
                require(previous_end is not None and previous_end<=stamp(fault["started_at"])<=stamp(fault["ended_at"])<=start)
                require(fault["scope"]=="a" and fault["path"]==profile(paths,"a")+"/skills/"+NAME and fault["skill_sha256"]==sha(authored(paths,"a","1.1.0")[1]))
        for value in marker_ends.values():require(value<=start if stage not in ("add-user","add-a","add-b") else True)
        obj(probe["value"],"events");require(probe["stage"]==stage and stream(probe["value"]["events"])==b"")
    require(previous-first<150*10**9 and previous-first<(result["elapsed_ms"]+1)*10**6 and total<=result["elapsed_ms"] and len(known)==4 and held["a"]["removed"] is True and held["b"]["removed"] is True and held["user"]["removed"] is False)
    identities=[(v["installation"]["installation_id"],v["binding_id"],v["binding"]["physical_artifact_id"],v["data"]["data_receipt_id"]) for v in held.values()]
    require(all(len({row[i] for row in identities})==3 for i in range(4)))


def members(bodies,source_identity):
    try:return admit(bodies,source_identity)
    except (KeyError,TypeError,AttributeError,IndexError,RecursionError,OverflowError,UnicodeError) as error:
        raise ValueError("Windsurf complete lifecycle structural refusal") from error


def admit(bodies,source_identity):
    require(set(bodies)=={"runtime-attempt.json",*MEMBERS} and all(type(raw) is bytes and len(raw)<=1<<20 and not (aggregate_private(raw) if name in MEMBERS else private(raw)) for name,raw in bodies.items()) and sum(map(len,bodies.values()))<=16<<20)
    r=load(bodies["runtime-attempt.json"])
    obj(r,"acquisition activation attempt_stage capture_mode capture_review_required client component_layout elapsed_ms fixture_authored_updater_config qualification real_model_qualified runtime_attempt_complete scanner source_identity test_failed vendor_authentication_qualified version qualification_scope cycle_command_stages cycle_proof_stages cycle_observed_complete cycle_nonce cycle_tool_pins")
    require(r["client"]=="windsurf" and r["version"]=="3000.11.205" and r["capture_mode"]==MODE and r["capture_review_required"] is False and r["activation"]=="prepared" and r["vendor_authentication_qualified"] is False and r["real_model_qualified"] is False and r["qualification_scope"]==SCOPE)
    require(same(r["source_identity"],{"installer_base_commit":source_identity["source_commit"],"installer_tree":source_identity["source_tree"],"installer_source_state":"committed","installer_patch_sha256":[]}))
    done=r["runtime_attempt_complete"]
    require(type(done) is bool and type(r["test_failed"]) is bool and type(r["cycle_observed_complete"]) is bool and type(r["elapsed_ms"]) is int and 0<=r["elapsed_ms"]<=180000 and same(r["qualification"],{key:done and key in FLAGS[:2] for key in FLAGS}))
    commands,proofs,probes=[records(bodies[name],order) for name,order in zip(MEMBERS,(STAGES,PROOFS,STAGES))]
    require(r["cycle_command_stages"]==[row["stage"] for row in commands] and r["cycle_proof_stages"]==[row["stage"] for row in proofs] and len(probes)<=len(commands))
    for row in proofs:proof_wire(row["stage"],row["value"])
    for row in commands:command(row["value"])
    for row in probes:obj(row["value"],"events");require(stream(row["value"]["events"])==b"")
    if not done:
        require(r["test_failed"] is True and r["cycle_observed_complete"] is False);return r
    require(r["test_failed"] is False and r["cycle_observed_complete"] is True and r["elapsed_ms"]<150000 and len(commands)==len(probes)==47 and len(proofs)==len(PROOFS))
    complete(commands,proofs,probes,r)
    return r


def payload(selected,paths):
    files=selected["payload_files"];require(type(files) is dict)
    state=selected["frames"][state_root(paths,selected["scope"])];root=posixpath.relpath(selected["active"],state_root(paths,selected["scope"]))
    rows={name[len(root)+1:]:row for name,row in state.items() if name.startswith(root+"/")}
    if selected["removed"]:
        require(files=={} and root not in state and not rows);return
    namespace="io.github.777genius.agentplugins/managed-stdio-v1/"
    expected_names={"plugin.json","mcp.json","skills/"+NAME+"/SKILL.md",namespace+"metadata.json"}
    require(set(files)==expected_names)
    manifest,skill=authored(paths,selected["scope"],selected["version"])
    require(stream(files["plugin.json"])==manifest and stream(files["skills/"+NAME+"/SKILL.md"])==skill)
    server=entry(paths,selected)
    require(same(load(stream(files["mcp.json"])),{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{NAME:{"type":"stdio",**server}}}))
    require(same(load(stream(files[namespace+"metadata.json"])),{"protocol":1,"sha256":paths["tool_pins"]["installer"],"cliVersion":"0.1.0-development"}))
    for name,wire in files.items():
        raw=stream(wire);require(rows[name]["Kind"]=="file" and rows[name]["Size"]==len(raw) and rows[name]["SHA256"]==sha(raw) and rows[name]["Mode"]&0o111==0)
    for name,pin in (("bin/probe","probe"),(namespace+"agentplugins","installer")):
        require(rows[name]["Kind"]=="file" and rows[name]["SHA256"]==paths["tool_pins"][pin] and rows[name]["Mode"]&0o111!=0)
    all_files=expected_names|{"bin/probe",namespace+"agentplugins"};dirs=set()
    for name in all_files:
        parent=posixpath.dirname(name)
        while parent:dirs.add(parent);parent=posixpath.dirname(parent)
    require(set(rows)==all_files|dirs and all(rows[name]["Kind"]=="directory" for name in dirs))


def proof_wire(stage,value):
    if stage.startswith("caller-marker-"):
        obj(value,"scope path started_at ended_at marker before after")
        stream(value["marker"]);frame(value["before"]);frame(value["after"])
        require(stamp(value["started_at"])<=stamp(value["ended_at"]))
    elif stage=="source-a-two":
        obj(value,"source revision nonce source_digest manifest mcp skill")
        for key in ("manifest","mcp","skill"):stream(value[key])
    elif stage=="missing-owned-a-skill":
        obj(value,"scope path skill_sha256 started_at ended_at")
        require(stamp(value["started_at"])<=stamp(value["ended_at"]))
    else:
        obj(value,"scope version revision nonce source source_digest removed state native_config cache data_marker frames mutation_lock data_owner payload_files")
        for key in ("state","native_config","data_marker","mutation_lock","data_owner"):stream(value[key])
        require(type(value["frames"]) is dict and len(value["frames"]) in (3,4) and type(value["payload_files"]) is dict and len(value["payload_files"]) in (0,4))
        for row in value["frames"].values():frame(row)
        for row in value["payload_files"].values():stream(row)
        cache=value["cache"]
        obj(cache,"role scope version source_digest manifest_digest path key phase prior assessment before_file after_file before_parent after_parent")
        stream(cache["assessment"])


def native_skill_frame(tree,scope_name,removed,skill):
    root="skills/"+NAME
    rows={name:row for name,row in tree.items() if name==root or name.startswith(root+"/")}
    if scope_name=="user" or removed:
        require(not rows);return
    require(set(rows)=={root,root+"/SKILL.md"})
    directory,file=rows[root],rows[root+"/SKILL.md"]
    require(directory["Kind"]=="directory" and directory["Mode"]==0o40755 and directory["Device"]==tree["."]["Device"] and directory["UID"]==tree["."]["UID"] and directory["GID"]==tree["."]["GID"])
    require(file["Kind"]=="file" and file["Mode"]==0o100644 and file["Links"]==1 and file["Device"]==directory["Device"] and file["UID"]==directory["UID"] and file["GID"]==directory["GID"] and file["Size"]==len(skill) and file["SHA256"]==sha(skill))
