from spike import *
import shutil

run('08-interactive-no-prompt',[],A,timeout=10)
run('09-disable-help',['plugin','disable','--help'],A)
run('10-disable-project',['plugin','disable','uap-scope-spike@skills-dir','--scope','project'],A)
run('11-project-disabled-A',['plugin','list','--json'],A)
run('12-project-disabled-B',['plugin','list','--json'],B)
run('13-enable-project',['plugin','enable','uap-scope-spike@skills-dir','--scope','project'],A)
run('14-project-reenabled-A',['plugin','list','--json'],A)
run('15-disable-user',['plugin','disable','uap-scope-spike@skills-dir','--scope','user'],USER)
run('16-user-disabled-home',['plugin','list','--json'],USER)
run('17-user-disabled-A',['plugin','list','--json'],A)
run('18-user-disabled-B',['plugin','list','--json'],B)
run('19-native-uninstall-project',['plugin','uninstall','uap-scope-spike@skills-dir','--scope','project'],A)
run('20-after-uninstall-project-A',['plugin','list','--json'],A)
run('21-after-uninstall-project-B',['plugin','list','--json'],B)

# Move the disposable user fixture outside its discovery directory, then inspect
# the project-only case. This is fixture mutation, not native activation/trust.
saved=BASE/'saved-user-fixture'
shutil.move(str(CONFIG/'skills/fixture-user'),str(saved))
run('22-project-only-A',['plugin','list','--json'],A)
run('23-project-only-B',['plugin','list','--json'],B)
run('24-project-only-A-nested',['plugin','list','--json'],A/'nested')
shutil.rmtree(A/'.claude/skills/fixture-A')
run('25-owned-A-directory-removed',['plugin','list','--json'],A)
run('26-B-after-A-removal',['plugin','list','--json'],B)

inventory=[]
for root in [USER,A,B]:
 for p in sorted(root.rglob('*')):
  if p.is_file():
   body=p.read_bytes()
   inventory.append({'path':str(p.relative_to(BASE)),'size':len(body),'sha256':hashlib.sha256(body).hexdigest()})
(OUT/'final-fixture-inventory.json').write_text(json.dumps(inventory,indent=2)+'\n')
