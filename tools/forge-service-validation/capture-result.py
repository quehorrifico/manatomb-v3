#!/usr/bin/env python3
"""Capture only terminal result/configuration metadata from a disposable local game.

Never records a deck, hand, prompt prose, token, or raw engine object. Does not play.
"""
import argparse, hashlib, json, subprocess, urllib.request
from pathlib import Path

p=argparse.ArgumentParser()
p.add_argument('--env-dir',type=Path,required=True)
p.add_argument('--output',type=Path,required=True)
p.add_argument('--scenario',required=True)
a=p.parse_args()
assert not a.output.exists(), 'Preserve prior evidence'
def docker(*args): return subprocess.check_output(['docker',*args],text=True).strip()
assert json.loads(docker('context','inspect'))[0]['Endpoints']['docker']['Host'].startswith('unix://')
secret=dict(line.split('=',1) for line in (a.env_dir/'local-engine.env').read_text().splitlines() if '=' in line)['FORGE_SERVICE_SECRET']
req=urllib.request.Request('http://127.0.0.1:18881/v1/sessions',headers={'Authorization':'Bearer '+secret,'X-ManaTomb-Owner':'1'})
v=json.load(urllib.request.urlopen(req,timeout=10));s=v.get('state',{})
assert v['status'] in ('finished','failed','cancelled','expired'), 'Game is still active'
info=json.loads(docker('inspect','manatomb-forge-release-local'))[0]
data={'scenario':a.scenario,'coverage':'Agent-operated real browser; passive human pass/discard strategy unless separately described. No representative human-deck performance claim.',
      'status':v['status'],'turn':s.get('turn'),'result':s.get('result'),'conceded':s.get('conceded',False),'engine_image':info['Image'],
      'web_image':json.loads(docker('inspect','manatomb-web-release-local'))[0]['Image'],
      'runtime_identity_sha256':hashlib.sha256(subprocess.check_output(['docker','exec','manatomb-forge-release-local','cat','/opt/forge/identity.json'])).hexdigest(),
      'limits':{k:info['HostConfig'][k] for k in ('Memory','MemorySwap','NanoCpus')},
      'memory_current_and_container_lifetime_peak_bytes':list(map(int,docker('exec','manatomb-forge-release-local','sh','-c','cat /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.peak').split())),
      'processes_after_end':docker('exec','manatomb-forge-release-local','ps','-eo','comm').split(),
      'session_files_after_end':docker('exec','manatomb-forge-release-local','ls','-A','/tmp/manatomb-games').split(),
      'platform':'Linux AMD64 on Docker Desktop ARM64/Rosetta; warm or unspecified cache; not native provider capacity'}
assert 'java' not in data['processes_after_end'] and not data['session_files_after_end'], 'Worker not reclaimed yet'
a.output.parent.mkdir(parents=True,exist_ok=True);a.output.write_text(json.dumps(data,indent=2)+'\n')
print(data['scenario'],data['status'],data['turn'],data['result'])
