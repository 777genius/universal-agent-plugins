from pathlib import Path
import hashlib, json, os, signal, subprocess, time

BASE=Path(__file__).resolve().parent
BIN=BASE/'bin/package/claude'
USER=BASE/'fixture-user'
CONFIG=USER/'.claude'
A=BASE/'projects/A'
B=BASE/'projects/B'
OUT=BASE/'observations'
for p in [USER,CONFIG,A,B,A/'nested',B/'nested',OUT]: p.mkdir(parents=True,exist_ok=True)

ENV={
 'HOME':str(USER), 'USER':'spike-fixture', 'LOGNAME':'spike-fixture',
 'PATH':'/usr/local/bin:/usr/bin:/bin:/opt/codex/runtimes/codex-primary-runtime/dependencies/node/bin',
 'LANG':'C.UTF-8','TERM':'xterm-256color', 'TMPDIR':str(BASE/'tmp'),
 'XDG_CONFIG_HOME':str(USER/'.config'), 'XDG_CACHE_HOME':str(USER/'.cache'),
 'CLAUDE_CONFIG_DIR':str(CONFIG), 'DISABLE_AUTOUPDATER':'1',
 'CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1',
}
Path(ENV['TMPDIR']).mkdir(exist_ok=True)

def run(label,args,cwd=A,timeout=20):
 start=time.time()
 p=subprocess.Popen([str(BIN),*args],cwd=cwd,env=ENV,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
 timed_out=False
 try: stdout,stderr=p.communicate(timeout=timeout)
 except subprocess.TimeoutExpired:
  timed_out=True; os.killpg(p.pid,signal.SIGKILL); stdout,stderr=p.communicate()
 record={'label':label,'argv':[str(BIN),*args],'cwd':str(cwd),'env':ENV,'exit_code':p.returncode,'timed_out':timed_out,'elapsed_seconds':round(time.time()-start,3),'stdout':stdout.decode(errors='replace'),'stderr':stderr.decode(errors='replace')}
 (OUT/(label+'.json')).write_text(json.dumps(record,indent=2)+'\n')
 print(json.dumps({k:v for k,v in record.items() if k not in ['env','argv']},ensure_ascii=False),flush=True)
 return record

def plugin(root,folder,version,marker):
 p=root/'skills'/folder
 (p/'.claude-plugin').mkdir(parents=True,exist_ok=True)
 (p/'skills/marker').mkdir(parents=True,exist_ok=True)
 (p/'.claude-plugin/plugin.json').write_text(json.dumps({'name':'uap-scope-spike','version':version,'description':'Disposable inert scope discovery fixture','skills':'./skills/'},indent=2)+'\n')
 (p/'skills/marker/SKILL.md').write_text(f'---\nname: marker\ndescription: Inert scope marker {marker}.\n---\n{marker}\n')
 return p

if __name__=='__main__':
 run('00-version',['--version'],USER)
 run('01-list-help',['plugin','list','--help'],USER)
 run('02-empty-A',['plugin','list','--json'],A)
 plugin(CONFIG,'fixture-user','1.0.0','USER-V1')
 run('03-user-only-home',['plugin','list','--json'],USER)
 run('04-user-only-A',['plugin','list','--json'],A)
 plugin(A/'.claude','fixture-A','2.0.0','PROJECT-A-V2')
 run('05-user-and-A',['plugin','list','--json'],A)
 plugin(B/'.claude','fixture-B','3.0.0','PROJECT-B-V3')
 run('06-user-and-B',['plugin','list','--json'],B)
 run('07-user-and-A-nested',['plugin','list','--json'],A/'nested')
