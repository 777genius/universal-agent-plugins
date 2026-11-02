"""Authored, deliberately unqualified envelopes; no native or hosted execution."""
import base64
import copy
import hashlib
import json
import unittest

import project_native_gemini_cycle as cycle
import project_native_gemini_cycle_bytes as bounded
import project_native_gemini_cycle_proof as proof
import project_native_runtime as runtime


def raw(value): return json.dumps(value,sort_keys=True,separators=(",",":"),ensure_ascii=False).encode()
def sha(body): return hashlib.sha256(body).hexdigest()
def stream(body): return {"bytes":len(body),"sha256":sha(body),"body":base64.b64encode(body).decode()}
def rows(values): return b"".join(raw(row)+b"\n" for row in values)
def record(stage,value): return {"stage":stage,"value":value}


def recipe(selected,paths,revision):
    name={"user":"uapgeminiu","a":"uapgeminia","b":"uapgeminib"}[selected];nonce=str({"user":1,"a":2,"b":3}[selected])*32
    manifest={"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":proof.NAME,"version":"1.0.0","description":"Disposable Gemini lifecycle proof"}
    skill=("---\nname: "+name+"\ndescription: "+revision+"\n---\nUAP_GEMINI_"+revision+"_"+nonce+"\n").encode()
    env={"UAP_TEST_ROOT":paths["root"],"UAP_TEST_EVENTS":paths["root"]+"/events.jsonl","UAP_TEST_NONCE":nonce,"UAP_TEST_REVISION":revision,"UAP_TEST_LITERAL":"literal ${UNKNOWN} $HOME","UAP_TEST_ONCE":"${PLUGIN_ROOT}/${UNKNOWN}"}
    mcp={"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{name:{"type":"stdio","command":"./bin/probe","args":["argument with spaces","${PLUGIN_ROOT}/argument","${UNKNOWN}"],"cwd":"${PLUGIN_ROOT}","env":env}}}
    files={"plugin.json":raw(manifest),"skills/"+name+"/SKILL.md":skill,"mcp.json":raw(mcp),"bin/probe":b"inert never executed fixture"}
    return {"scope":selected,"source":paths["root"]+"/cycle-source-"+selected,"name":name,"revision":revision,"nonce":nonce,"digest":"sha256:"+sha((selected+revision).encode()),"files":{name:{"bytes":len(body),"sha256":sha(body),"executable":name=="bin/probe"} for name,body in files.items()},"skill":stream(skill),"manifest":stream(files["plugin.json"]),"mcp":stream(files["mcp.json"])}


def state_row(stage,r,paths,previous=None):
    selected=r["scope"];scope="user" if selected=="user" else "project";root=paths["root"]+"/installer-state" if selected=="user" else paths[selected]+"/.uap"
    profile=paths["home"]+"/.gemini" if selected=="user" else paths[selected]+"/.gemini"
    iid="01234567-89ab-4cde-8fab-0123456789a"+str({"user":1,"a":2,"b":3}[selected]);physical=proof.NAME+"-"+sha(iid.encode())[:12]
    active,data=root+"/managed/clients/gemini/"+physical,root+"/plugin-data/"+physical
    bid="client_"+sha("\0".join((iid,"gemini",scope,active)).encode())[:24];did="data_"+sha("\0".join((iid,physical,scope)).encode())[:24]
    d={"data_receipt_id":did,"physical_backend_id":physical,"scope":scope,"locator":data,"ownership_digest":"sha256:"+sha(("agentplugins-plugin-data-v1\0"+did+"\0"+data).encode()),"state":"owned"}
    receipts=[] if previous is None else copy.deepcopy(previous["binding"]["receipts"])
    removed=stage.startswith("remove-") or stage=="repeat-remove-a"
    mutate=stage.startswith("add-") or stage in ("intact-repair-a","update-a","missing-repair-a")
    payload="sha256:"+sha((selected+r["revision"]+"installed").encode())
    def receipt(index,remove=False):
        group="inert-"+selected+"-"+str(index);operation=group+"-001";base=root+"/managed/clients/gemini"
        value={"operation_id":operation,"operation_group_id":group,"sequence":index,"mutation_type":"directory_remove" if remove else "directory_swap","client_binding_id":bid,"active_path":active,"backup_path":base+"/.agentplugins-backup-"+sha(operation.encode())[:16],"phase":"committed"}
        if receipts: value["before_digest"]=receipts[-1]["after_digest"]
        if not remove: value.update(staging_path=base+"/.agentplugins-staging-inert-"+str(index),after_digest=payload)
        return value
    if mutate: receipts.append(receipt(len(receipts)+1))
    package={"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":proof.NAME,"version":"1.0.0","manifest_digest":"sha256:"+r["files"]["plugin.json"]["sha256"],"inventory":{"skills":[r["name"]],"mcp_present":True,"mcp_enabled":True,"mcp_servers":[r["name"]]}}
    env={"PLUGIN_ROOT":active,"PLUGIN_DATA":data,"UAP_TEST_ROOT":paths["root"],"UAP_TEST_EVENTS":paths["root"]+"/events.jsonl","UAP_TEST_NONCE":r["nonce"],"UAP_TEST_REVISION":r["revision"],"UAP_TEST_LITERAL":"literal ${UNKNOWN} $HOME","UAP_TEST_ONCE":active+"/${UNKNOWN}"}
    entry={"command":active+"/bin/probe","cwd":active,"args":["argument with spaces",active+"/argument","${UNKNOWN}"],"env":env};name=r["name"]
    objects=[{"object_id":"package:gemini:"+physical,"kind":"managed_package_directory","logical_name":proof.NAME,"path":active,"managed_digest":payload,"protection_class":"managed"},
             {"object_id":"gemini-mcp:"+name,"kind":"gemini_global_mcp_server","logical_name":name,"path":profile+"/settings.json","managed_digest":"sha256:"+sha(("agentplugins-native-mcp-v1\0gemini-mcpServers\0"+name+"\0").encode()+raw(entry)),"protection_class":"managed"},
             {"object_id":"gemini-skill:"+name,"kind":"gemini_global_skill_directory","logical_name":name,"path":profile+"/skills/"+name,"source_relative":"skills/"+name,"managed_digest":"sha256:"+sha(b"f\0SKILL.md\0-\0"+base64.b64decode(r["skill"]["body"])+b"\0"),"protection_class":"managed"}]
    b={"client_binding_id":bid,"client_id":"gemini","scope":scope,"target_locator":active,"physical_artifact_id":physical,"materialization":"materialized","activation":"active" if selected=="user" else "prepared","authentication":"not_checked","policy":"allowed","verification":"installation_verified","package_revision":{"version":"1.0.0","tree_digest":r["digest"],"manifest_digest":package["manifest_digest"]},"data_receipt_id":did,"affected_surfaces":["gemini"],"native_profile_root":profile,"native_objects":objects,"receipts":receipts,"updated_at":"2026-10-27T00:00:00Z"}
    item={"installation_id":iid,"declared_name":proof.NAME,"source":{"source_binding_id":"src_"+sha((r["source"]+"\0\0").encode())[:24],"requested_source":r["source"],"canonical_source":r["source"],"resolved_revision":"","tree_digest":r["digest"]},"package":package,"origin_mode":"direct","clients":{} if removed else {bid:b},"created_at":"2026-10-27T00:00:00Z","updated_at":"2026-10-27T00:00:00Z","data_receipts":{did:d}}
    state={"schema_version":5,"installations":[item]}
    if removed:item["data_retained"]=True;state["transaction_receipts"]=[receipt(len(receipts)+1,True)]
    return {"scope":selected,"revision":r["revision"],"removed":removed,"source_digest":r["digest"],"state":stream(raw(state)),"binding":b}


def fixture():
    source={"source_commit":"a"*40,"source_tree":"b"*40};root="/inert/gemini"
    paths={"root":root,"home":root+"/home","a":root+"/project","b":root+"/project-B","outside":root+"/outside","installer":"/inert/bin/agentplugins","client":"/inert/client/gemini.js","node":"/inert/node/bin/node","git":"/usr/bin/git"}
    result={"client":"gemini","provider_kind":"scripted_loopback","capture_mode":cycle.MODE,"cycle_capture_review_required":True,"cycle_observed_complete":True,"runtime_attempt_complete":False,"test_failed":True,"activation":"prepared","qualification":dict(cycle.QUAL),"vendor_authentication_qualified":False,"real_model_qualified":False,"source_identity":{"installer_base_commit":source["source_commit"],"installer_tree":source["source_tree"],"installer_source_state":"committed","installer_patch_sha256":[]},"cycle_layout":paths,"cycle_public_stages":list(cycle.PUBLIC),"cycle_listing_stages":list(cycle.LISTINGS),"cycle_native_stages":list(cycle.NATIVE),"cycle_provider_completed":list(cycle.NATIVE),"attempt_stage":"complete-cycle-capture-review","elapsed_ms":1000}
    bodies={"runtime-attempt.json":raw(result)}
    for label in cycle.PUBLIC:
        cwd,args=cycle.public_command(label,paths);stdout=b"";stderr=cycle.progress(label);exit_code=0
        if label=="version":stdout=b"0.65.0\n"
        elif label=="repeat-remove-a":exit_code=1;stderr=b'agentplugins: plugin is not installed for target "gemini" in project scope; no target was changed\n'
        elif not label.startswith("git-"):
            verb={"repeat-add-a":"add","unchanged-update-a":"update","intact-repair-a":"repair","missing-repair-a":"repair"}.get(label,label.split("-")[0]);stdout=raw({"schema_version":1,"command":verb,"result":"success","data":{"mutated":label not in ("repeat-add-a","unchanged-update-a")}})
        bodies[label+".command.json"]=raw({"argv":args,"cwd":cwd,"exit":exit_code,"error":False,"context_error":False,"elapsed_ms":1,"cleanup":cycle.CLEANUP});bodies[label+".stdout.log"]=stdout;bodies[label+".stderr.log"]=stderr
    values={name:[] for name in cycle.AGGREGATES}
    for stage in cycle.OBSERVATIONS:
        kind,label=stage.split(":");selected=cycle.scope(label);argv=[paths["node"],paths["client"]]+(["skills","list","--all"] if kind=="listing" else ["--prompt","inert prompt","--approval-mode","default","--output-format","stream-json","--model","gemini-2.5-flash"])
        value={"kind":kind,"scope":selected,"argv":argv,"cwd":paths["outside" if selected=="user" else selected],"exit":0,"error":False,"context_error":False,"elapsed_ms":1,"started_at":"2026-10-27T00:00:00Z","finished_at":"2026-10-27T00:00:01Z","cleanup":cycle.CLEANUP,"stdout":stream(b"authored inert capture; no native executed\n"),"stderr":stream(b"")}
        values["cycle-native-commands.jsonl"].append(record(stage,value))
        if kind=="native":values["cycle-native-events.jsonl"].append(record(label,{"scope":selected,"session":"inert-session-"+label,"stream":value["stdout"]}))
        else:values["cycle-discovery.jsonl"].append(record(label,{"scope":selected,"stdout":value["stdout"],"stderr":value["stderr"]}))
        values["cycle-probe-events.jsonl"].append(record(label if kind=="native" else stage,{"scope":selected,"stream":stream(b"")}))
    recipes={key:recipe(key,paths,{"user":"U-one","a":"A-one","b":"B-one"}[key]) for key in ("user","a","b")};states={}
    for stage in cycle.PROOFS:
        if stage.startswith("source-"):
            selected=stage.split("-")[1]
            if stage=="source-a-A-two":recipes["a"]=recipe("a",paths,"A-two")
            value=recipes[selected]
        else:selected="a" if stage in ("before-source-revision-a","before-missing-a") else cycle.scope(stage);value=state_row(stage,recipes[selected],paths,states.get(selected));states[selected]=value
        values["cycle-proof.jsonl"].append(record(stage,value))
    for stage in cycle.NATIVE:
        selected=cycle.scope(stage);r=recipes[selected];operation="write" if stage in ("installed-user","installed-a","installed-b") else "read" if stage in ("restarted-a","updated-a","repaired-a") else "none";count=1 if operation=="none" else 2
        tools=[] if stage in ("baseline-a","removed-user") else ["mcp_uapgeminiu_inspect_runtime"]
        if stage in ("installed-a","installed-b","restarted-a","updated-a","repaired-a"):tools.append("mcp_"+r["name"]+"_inspect_runtime")
        wire="" if operation=="none" else "uap_cycle_"+sha((stage+r["nonce"]).encode())[:24];native="" if operation=="none" else "mcp_"+r["name"]+"_inspect_runtime__"+wire
        value={"scope":selected,"operation":operation,"tools":tools,"native_call_id":native,"wire_call_id":wire,"prompt":"Disposable Gemini fixture stage "+stage+". Call only the preselected installed inspect_runtime tool when requested.","sentinel":"UAP_GEMINI_DONE_"+stage+"_"+r["nonce"],"requests":count,"responses":count,"issued":operation!="none","returned":operation!="none","error":False}
        values["cycle-provider-receipts.jsonl"].append(record(stage,value))
        for index in range(count):
            for name in ("cycle-provider-requests.jsonl","cycle-provider-responses.jsonl"):values[name].append(record(stage,{"index":index,"stream":stream(b"authored inert provider bytes")}))
    bodies.update({name:rows(value) for name,value in values.items()});return bodies,source


class GeminiCycleTests(unittest.TestCase):
    def test_every_public_boundary_retains_truthful_closed_failed_prefix(self):
        timeline=[("proof",v) for v in cycle.PROOFS[:3]]
        timeline += [("public",v) for v in cycle.PUBLIC[:4]]
        timeline += [("listing",v) for v in cycle.LISTINGS[:3]]+[("native","baseline-a")]
        for selected in ("user","a","b"):
            timeline += [("public","add-"+selected),("proof","add-"+selected),("listing","installed-"+selected),("native","installed-"+selected)]
        for label in ("repeat-add-a","unchanged-update-a","intact-repair-a"):
            timeline += [("public",label),("proof",label),("listing",label)]
        timeline += [("native","restarted-a"),("proof","before-source-revision-a"),("proof","source-a-A-two")]
        timeline += [("public","update-a"),("proof","update-a"),("listing","updated-a"),("native","updated-a"),("proof","before-missing-a")]
        timeline += [("public","missing-repair-a"),("proof","missing-repair-a"),("listing","repaired-a"),("native","repaired-a")]
        timeline += [("public","remove-a"),("proof","remove-a"),("listing","removed-a"),("native","removed-a")]
        timeline += [("public","repeat-remove-a"),("proof","repeat-remove-a")]
        for selected in ("b","user"):
            timeline += [("public","remove-"+selected),("proof","remove-"+selected),("listing","removed-"+selected),("native","removed-"+selected)]
        for count in range(17):
            bodies,source=fixture();summary=runtime.strict_json(bodies["runtime-attempt.json"])
            stop=next((i for i,event in enumerate(timeline) if event==("public",cycle.PUBLIC[count])),len(timeline)) if count<16 else len(timeline)
            included=timeline[:stop];selected={kind:[name for k,name in included if k==kind] for kind in ("public","proof","listing","native")}
            for label in cycle.PUBLIC[count:]:
                for suffix in (".command.json",".stdout.log",".stderr.log"):bodies.pop(label+suffix)
            summary.update(cycle_observed_complete=False,cycle_public_stages=selected["public"],cycle_listing_stages=selected["listing"],cycle_native_stages=selected["native"],cycle_provider_completed=selected["native"],attempt_stage=cycle.PUBLIC[count] if count<16 else "complete-cycle-capture-review")
            observed={"listing:"+v for v in selected["listing"]}|{"native:"+v for v in selected["native"]}
            for member in cycle.AGGREGATES:
                values=bounded.rows(bodies[member])
                if member=="cycle-native-commands.jsonl":values=[v for v in values if v["stage"] in observed]
                elif member=="cycle-proof.jsonl":values=[v for v in values if v["stage"] in selected["proof"]]
                elif member=="cycle-discovery.jsonl":values=[v for v in values if v["stage"] in selected["listing"]]
                elif member=="cycle-probe-events.jsonl":values=[v for v in values if ("native:"+v["stage"] in observed or v["stage"] in observed)]
                else:values=[v for v in values if v["stage"] in selected["native"]]
                bodies[member]=rows(values)
            bodies["runtime-attempt.json"]=raw(summary)
            with self.subTest(public_prefix=count):
                admitted=runtime.attempt_members(bodies,source,"gemini");self.assertFalse(admitted["cycle_observed_complete"]);self.assertFalse(any(admitted["qualification"].values()))

    def test_failed_first_native_process_keeps_its_raw_command_and_probe_without_success(self):
        bodies,source=fixture();summary=runtime.strict_json(bodies["runtime-attempt.json"])
        summary.update(cycle_observed_complete=False,cycle_public_stages=list(cycle.PUBLIC[:4]),cycle_listing_stages=list(cycle.LISTINGS[:3]),cycle_native_stages=[],cycle_provider_completed=[],attempt_stage="native:baseline-a")
        for label in cycle.PUBLIC[4:]:
            for suffix in (".command.json",".stdout.log",".stderr.log"):bodies.pop(label+suffix)
        for member in cycle.AGGREGATES:
            values=bounded.rows(bodies[member])
            if member=="cycle-native-commands.jsonl":values=values[:4];values[-1]["value"]["exit"]=1;values[-1]["value"]["error"]=True
            elif member=="cycle-discovery.jsonl":values=values[:3]
            elif member=="cycle-probe-events.jsonl":values=values[:4]
            elif member=="cycle-proof.jsonl":values=values[:3]
            elif member=="cycle-provider-receipts.jsonl":values=values[:1];values[0]["value"].update(requests=0,responses=0)
            else:values=[]
            bodies[member]=rows(values)
        bodies["runtime-attempt.json"]=raw(summary)
        result=runtime.attempt_members(bodies,source,"gemini");self.assertTrue(result["test_failed"]);self.assertFalse(result["runtime_attempt_complete"])

    def test_complete_authored57_member_first_capture_never_qualifies(self):
        bodies,source=fixture();self.assertEqual(len(bodies),57)
        result=runtime.attempt_members(bodies,source,"gemini")
        self.assertTrue(result["test_failed"]);self.assertFalse(result["runtime_attempt_complete"]);self.assertFalse(any(result["qualification"].values()))

    def test_closed_source_flags_stages_members_and_process_contradictions_refuse(self):
        for kind in ("qualification","flag-type","runtime","passed","auth","model","source","mode","public","listing","native","provider-prefix","stage","deadline","missing","orphan","argv","cleanup","native-error","provider-count","provider-id","observation-join","encoded-private"):
            bodies,source=fixture();summary=runtime.strict_json(bodies["runtime-attempt.json"])
            if kind=="qualification":summary["qualification"]["project-lifecycle"]=True
            elif kind=="flag-type":summary["qualification"]["project-lifecycle"]=0
            elif kind in ("runtime","passed","auth","model"):summary[{"runtime":"runtime_attempt_complete","passed":"test_failed","auth":"vendor_authentication_qualified","model":"real_model_qualified"}[kind]]=kind!="passed"
            elif kind=="source":summary["source_identity"]["installer_tree"]="c"*40
            elif kind=="mode":summary["capture_mode"]="unknown"
            elif kind in ("public","listing","native","provider-prefix"):summary[{"public":"cycle_public_stages","listing":"cycle_listing_stages","native":"cycle_native_stages","provider-prefix":"cycle_provider_completed"}[kind]].pop(1)
            elif kind=="stage":summary["attempt_stage"]="unknown"
            elif kind=="deadline":summary["elapsed_ms"]=180001
            elif kind=="missing":del bodies["add-a.stderr.log"]
            elif kind=="orphan":bodies["unknown.stdout.log"]=b""
            elif kind in ("argv","cleanup"):
                row=runtime.strict_json(bodies["git-a.command.json"]);row[kind]=["/unregistered"] if kind=="argv" else "assumed";bodies["git-a.command.json"]=raw(row)
            elif kind in ("native-error","observation-join","encoded-private"):
                name="cycle-native-commands.jsonl";values=bounded.rows(bodies[name]);value=values[-1]["value"]
                if kind=="native-error":value["error"]=True
                elif kind=="observation-join":value["stdout"]=stream(b"changed")
                else:value["stdout"]=stream(b'{"nested":"{\\"secret\\":\\"\\u0075ap-gemini-private-test\\"}"}')
                bodies[name]=rows(values)
            elif kind in ("provider-count","provider-id"):
                name="cycle-provider-receipts.jsonl";values=bounded.rows(bodies[name]);values[1]["value"]["requests" if kind=="provider-count" else "native_call_id"]=1 if kind=="provider-count" else "foreign";bodies[name]=rows(values)
            bodies["runtime-attempt.json"]=raw(summary)
            with self.subTest(kind=kind),self.assertRaises(ValueError):runtime.attempt_members(bodies,source,"gemini")

    def test_consistently_rehashed_state_and_native_authority_hostiles_refuse(self):
        for kind in ("schema","source","profile","data","extra-data","object-id","object-digest","extra-object","binding-map","intent","unknown-state","receipt-sequence","receipt-path"):
            bodies,source=fixture();values=bounded.rows(bodies["cycle-proof.jsonl"]);value=values[3]["value"];state=runtime.strict_json(bounded.stream(value["state"]));item=state["installations"][0];binding=value["binding"]
            if kind=="schema":state["schema_version"]=4
            elif kind=="source":item["source"]["canonical_source"]="/foreign"
            elif kind=="profile":binding["native_profile_root"]="/foreign"
            elif kind=="data":item["data_receipts"][binding["data_receipt_id"]]["locator"]="/foreign"
            elif kind=="extra-data":item["data_receipts"]["foreign"]={}
            elif kind=="object-id":binding["native_objects"][0]["object_id"]="foreign"
            elif kind=="object-digest":binding["native_objects"][0]["managed_digest"]="sha256:"+"0"*64
            elif kind=="extra-object":binding["native_objects"].append(copy.deepcopy(binding["native_objects"][0]))
            elif kind=="binding-map":item["clients"]={"foreign":binding}
            elif kind=="intent":binding["install_intent"]="automatic"
            elif kind=="unknown-state":state["unknown"]=True
            elif kind=="receipt-sequence":binding["receipts"][0]["sequence"]=True
            elif kind=="receipt-path":binding["receipts"][0]["staging_path"]="/foreign"
            if kind in ("profile","object-id","object-digest","extra-object","intent","receipt-sequence","receipt-path"):item["clients"]={binding["client_binding_id"]:binding}
            value["state"]=stream(raw(state));bodies["cycle-proof.jsonl"]=rows(values)
            with self.subTest(kind=kind),self.assertRaises(ValueError):runtime.attempt_members(bodies,source,"gemini")

    def test_semantic_privacy_numeric_framing_and_nested_canaries(self):
        for body in (b"1e999"+b"a"*59,b"1e999 1e999",b'{"ordinary":"1e999'+b"a"*59+b'"}'):
            self.assertFalse(bounded.private(body))
        for body in (b'{"number":1e999}',b'{"nested":"{\\"secret\\":\\"\\u0075ap-gemini-private-test\\"}"}',b'{"ok":true}\n'+b'{"secret":"\\u0075ap-gemini-private-test"}\n',b"\xff",b"x"*((1<<20)+1)):
            self.assertTrue(bounded.private(body))

    def test_actual_unchanged_constructor_key_namespace_cannot_escape_via_json(self):
        secret=b"uap-fixture-"+b"a"*48
        escaped=b'{"credential":"\\u0075ap-fixture-'+b"a"*48+b'"}'
        key=b'{"\\u0075ap-fixture-'+b"a"*48+b'":true}'
        for body in (escaped,raw({"nested":escaped.decode()}),key,raw({"nested":key.decode()}),b"{}\n"+escaped+b"\n"):
            redacted=body.replace(secret,b"[redacted]")
            self.assertNotIn(secret,body)
            self.assertTrue(bounded.private(redacted))
            with self.assertRaises(ValueError):bounded.stream(stream(redacted))
        literal=raw({"credential":secret.decode()}).replace(secret,b"[redacted]")
        self.assertFalse(bounded.private(literal))


if __name__=="__main__":unittest.main()
