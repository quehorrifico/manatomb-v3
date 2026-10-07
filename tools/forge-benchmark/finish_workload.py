#!/usr/bin/env python3
"""Capture the real turn-12 endpoint, then cancel/reclaim the local benchmark (no gameplay replies)."""
import argparse,json,subprocess,time,urllib.request,datetime
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('run',type=Path);a=p.parse_args()
view=json.load(urllib.request.urlopen('http://127.0.0.1:18711/state',timeout=5))
assert view['turn']==12 and view['turnPlayer']=='Human' and view['prompt']['kind']=='InputPassPriority', 'Workload endpoint not reached'
(a.run/'endpoint.json').write_text(json.dumps(view,indent=2)+'\n')
(a.run/'endpoint-captured.json').write_text(json.dumps({'utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'scope':'Observed actual turn-12 human priority; normal benchmark abort, not natural game completion'},indent=2))
subprocess.run(['python3',str(Path(__file__).with_name('faults.py')),'cancel','--run',str(a.run)],check=True)
for _ in range(40):
    if (a.run/'supervision.json').exists():break
    time.sleep(.25)
final=json.loads((a.run/'supervision.json').read_text());assert final['container_removed']
print(json.dumps({'endpoint':'passed','cleanup_seconds':final['cleanup_seconds'],'container_removed':True}))
