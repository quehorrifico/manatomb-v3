#!/usr/bin/env python3
"""Restart only the isolated QA engine; never touches the owner's review instance."""
import argparse,json,subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--directory',type=Path,required=True);p.add_argument('--fixture',default='verification',choices=['verification','realistic']);p.add_argument('--classes',type=Path);a=p.parse_args()
def d(*args):return subprocess.check_output(['docker',*args],text=True).strip()
assert json.loads(d('context','inspect'))[0]['Endpoints']['docker']['Host'].startswith('unix://')
name='manatomb-qa-forge';info=json.loads(d('inspect',name))[0]
assert info['NetworkSettings']['Networks'].get('manatomb-v2-qa') is not None
assert 'java' not in d('exec',name,'ps','-eo','comm').split(), 'End QA game explicitly first'
d('stop',name);d('rm',name)
args=['run','-d','--name',name,'--platform','linux/amd64','--network','manatomb-v2-qa','--network-alias','qa-forge','--memory','2g','--memory-swap','2g','--cpus','1','--pids-limit','256','--read-only','--tmpfs','/tmp:rw,noexec,nosuid,size=128m,mode=1777','--env-file',str(a.directory/'engine.env'),'-p','127.0.0.1:18891:8081']
if a.fixture!='realistic':args+=['--mount',f'type=bind,src={Path(__file__).resolve().with_name("fixture-launch")},dst=/opt/qa-launch,readonly','-e','FORGE_LAUNCHER=/opt/qa-launch','-e','MANATOMB_TEST_FIXTURE='+a.fixture]
if a.classes:args+=['--mount',f'type=bind,src={a.classes.resolve()},dst=/opt/forge/classes,readonly']
d(*args,'manatomb-forge:qa');print('Restarted isolated QA only:',a.fixture)
