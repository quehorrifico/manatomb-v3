#!/usr/bin/env python3
"""Verify retained cgroup/lifecycle evidence. Does not claim unrun gameplay coverage."""
import argparse,json,subprocess
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('artifacts',type=Path);a=p.parse_args()
summary=json.loads(subprocess.check_output(['python3',str(Path(__file__).with_name('summarize.py')),str(a.artifacts)],text=True));runs=summary['runs']
assert len(runs)>=10
for r in runs:
    path=a.artifacts/r['name'];initial=json.loads((path/'container-initial.json').read_text())[0]
    assert initial['HostConfig']['Memory']==r['cap_mib']*2**20
    assert initial['HostConfig']['MemorySwap']==initial['HostConfig']['Memory']
    assert initial['HostConfig']['NanoCpus']==1000000000
    env=next(json.loads(x)['value'] for x in (path/'out/events.jsonl').read_text().splitlines() if json.loads(x)['event']=='environment')
    assert env['arch']=='amd64' and env['cpu_max']=='100000 100000' and env['swap_max']=='0'
    assert r['supervision']['container_removed'] and not r['oom_killed']
    assert 'OutOfMemoryError' not in (path/'out/engine.log').read_text(errors='replace')
for prefix in ['idle-1024-','timed-ai-1024-']:
    assert len([r for r in runs if r['name'].startswith(prefix)])==5
for name in ['fault-cpu-cancel','fault-cpu-kill']:
    assert (a.artifacts/name/'out/decisions.csv').read_text().splitlines()[-1].startswith('begin,')
    assert json.loads((a.artifacts/name/('probe-'+name.removeprefix('fault-')+'.json')).read_text())['active_cpu_call_observed']
probes={name:json.loads((a.artifacts/'fault-stuck-human'/f'probe-{name}.json').read_text()) for name in ['replacement','interrupt','capacity','stuck']}
assert probes['replacement']['replacement_unchanged'] and probes['replacement']['old_token_status']==409 and probes['replacement']['new_token_old_session_status']==409
assert probes['interrupt']['same_session_prompt_revision'] and probes['capacity']['rejected']
assert probes['stuck']['worker_reap_response_seconds']<30
assert not subprocess.check_output(['docker','ps','-aq','--filter','label=manatomb.benchmark=V2-012'],text=True).strip()
print(json.dumps({'resource_and_lifecycle_invariants':'passed','cycles':len(runs),'natural_AI_games':sum(r['ai_natural_game_ms'] is not None for r in runs),'all_containers_removed':True,'fault_probes':probes,'representative_human_workload':'blocked; do not treat this audit as overall V2-012 acceptance'},indent=2))
