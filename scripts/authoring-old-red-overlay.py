"""Baseline c622 source plus scheduling seams only; never release evidence."""
import json
import pathlib
import subprocess
import sys

root = pathlib.Path(sys.argv[1]).resolve()
out = pathlib.Path(sys.argv[2]).resolve()
out.mkdir(parents=True, exist_ok=True)
relative = 'install/integrationctl/agentplugins/adapters/packageview/source_windows.go'
base = 'c622c0fae13248be0b7fdf826ffdcdf896e7dc33'
s = subprocess.check_output(['git', '-C', str(root), 'show', base + ':' + relative], text=True)
original = s

def replace(old, new):
    global s
    assert s.count(old) == 1, old
    s = s.replace(old, new)

replace('records    map[winSnapshot]*winObservation', 'records    map[winSnapshot]*winObservation\n\tmetadataStage func(*os.File, string)')
replace('\tinfo, e := f.Stat()\n', '\tif s.metadataStage != nil { s.metadataStage(f, "stat") }\n\tinfo, e := f.Stat()\n')
replace('\theld, e := winReopen(f, access, share)', '\tif s.metadataStage != nil { s.metadataStage(f, "protect") }\n\theld, e := winReopen(f, access, share)')
replace('func openSource(name string) (_ *source, err error) {', 'func openSource(name string) (*source, error) { return openSourceWithMetadataStage(name, nil) }\nfunc openSourceWithMetadataStage(name string, stage func(*os.File, string)) (_ *source, err error) {')
replace('s := &source{records: make(map[winSnapshot]*winObservation)}', 's := &source{records: make(map[winSnapshot]*winObservation), metadataStage: stage}')
# Compile the new helper-only guard test too. This helper is NOT called by any
# baseline acquisition/comparison: those retain the original complete equality.
s += '\nfunc winUnchanged(a, b winSnapshot, _ bool) bool { return a == b }\n'
assert 'meta != after' in s and 'locked != meta' in s and 'm != o.meta' in s
path = out / 'source_windows.go'
path.write_text(s)
(out / 'original-source_windows.go.txt').write_text(original)
(out / 'overlay.json').write_text(json.dumps({'Replace': {str(root / relative): str(path)}}, indent=2) + '\n')
print(out / 'overlay.json')
