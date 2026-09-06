import json, os, pathlib, subprocess
root=pathlib.Path.cwd()
private=pathlib.Path(os.environ['RUNNER_TEMP'])/'authoring-concurrent'
private.mkdir()
evidence=private/'evidence'
evidence.mkdir()
env=dict(os.environ)
for key, folder in {'HOME':'home','USERPROFILE':'home','APPDATA':'appdata','LOCALAPPDATA':'localappdata','TMP':'tmp','TEMP':'tmp','TMPDIR':'tmp','GOCACHE':'cache','GOMODCACHE':'modules'}.items():
 p=private/folder
 p.mkdir(exist_ok=True)
 env[key]=str(p)
env.update(GOTOOLCHAIN='local',GOENV='off',GOMAXPROCS='2',GOWORK=str(root/'go.work'))
meta=json.loads(subprocess.check_output(['go','env','-json','GOHOSTOS','GOHOSTARCH','GOVERSION'],env=env))
assert meta=={'GOHOSTOS':'windows','GOHOSTARCH':os.environ['EXPECTED_ARCH'],'GOVERSION':'go1.25.13'},meta
(evidence/'host.json').write_text(json.dumps(meta))
(evidence/'head.txt').write_bytes(subprocess.check_output(['git','rev-parse','HEAD']))
subprocess.run(['go','run','./cli/plugin-kit-ai/internal/authoring/commands/testdata/native_diagnostics.go',str(root),str(evidence/'diagnostics')],env=env,check=True)
pkg='./cli/plugin-kit-ai/internal/authoring/commands'
subprocess.run(['go','list','-p','2','-deps','-test',pkg],env=env,check=True,stdout=subprocess.DEVNULL)
env.update(GOPROXY='off',GOSUMDB='off')
with (evidence/'concurrent.json').open('w') as out,(evidence/'stderr.txt').open('w') as err:
 result=subprocess.run(['go','test','-p','2','-count=32','-timeout=180s','-json','-overlay='+str(evidence/'diagnostics'/'overlay.json'),'-run=^TestConcurrentInitAndCanceledInvocation$',pkg],env=env,stdout=out,stderr=err)
(evidence/'exit.txt').write_text(str(result.returncode))
assert result.returncode in (0,1),result.returncode
events=[json.loads(line) for line in (evidence/'concurrent.json').read_text().splitlines()]
terminals=[e for e in events if e.get('Test')=='TestConcurrentInitAndCanceledInvocation' and e['Action'] in ('fail','pass','skip')]
assert len(terminals)==32 and all(e['Action']!='skip' for e in terminals),terminals
(evidence/'verdict.json').write_text(json.dumps({'planned_scenarios':32,'failed_scenarios':sum(e['Action']=='fail' for e in terminals),'release_proof':False,'terminals':terminals},indent=2))
