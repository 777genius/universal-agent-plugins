import json, os, pathlib, subprocess
root=pathlib.Path.cwd()
private=pathlib.Path(os.environ['RUNNER_TEMP'])/'authoring-old-red'
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
subprocess.run(['python','scripts/authoring-old-red-overlay.py',str(root),str(evidence/'overlay')],env=env,check=True)
pkg='./install/integrationctl/agentplugins/adapters/packageview'
subprocess.run(['go','list','-p','2','-deps','-test',pkg],env=env,check=True,stdout=subprocess.DEVNULL)
env.update(GOPROXY='off',GOSUMDB='off')
with (evidence/'old-red.json').open('w') as out,(evidence/'stderr.txt').open('w') as err:
 result=subprocess.run(['go','test','-p','2','-count=1','-timeout=120s','-json','-overlay='+str(evidence/'overlay'/'overlay.json'),'-run=^TestWindowsAncestorEpochBoundary$',pkg],env=env,stdout=out,stderr=err)
(evidence/'exit.txt').write_text(str(result.returncode))
assert result.returncode==1,result.returncode
events=[json.loads(line) for line in (evidence/'old-red.json').read_text().splitlines()]
terminals={e.get('Test'):e['Action'] for e in events if e['Action'] in ('fail','pass','skip') and e.get('Test')}
for boundary in ('outside','root','descendant'):
 for stage in ('stat','protect'):
  name='TestWindowsAncestorEpochBoundary/'+boundary+'/'+stage
  assert terminals.get(name)==('fail' if boundary=='outside' else 'pass'),terminals
  logs=''.join(e.get('Output','') for e in events if e.get('Test')==name)
  assert 'stage='+stage+' boundary='+boundary+' before=' in logs,logs
  assert 'mutation fired=' not in logs and 'did not isolate' not in logs,logs
  if boundary=='outside': assert 'outside-root sibling must not invalidate acquisition' in logs,logs
(evidence/'verdict.json').write_text(json.dumps({'baseline':'c622c0fae13248be0b7fdf826ffdcdf896e7dc33','old_red_reproduced':True,'release_proof':False,'terminals':terminals},indent=2))
