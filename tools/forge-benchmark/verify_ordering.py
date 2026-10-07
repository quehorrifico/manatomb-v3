#!/usr/bin/env python3
"""Audit retained ordering/browser measurements; never submit gameplay or infer capacity."""
import argparse,csv,hashlib,json,math,statistics,subprocess
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('artifacts',type=Path);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
root=Path(__file__).resolve().parents[2];a.out.mkdir(parents=True,exist_ok=True)
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def save(name,data):(a.out/name).write_text(json.dumps(data,indent=2)+'\n')
def stats(v):
    v=sorted(v)
    return {'n':len(v),'p50':v[math.ceil(len(v)*.5)-1],'p95':v[math.ceil(len(v)*.95)-1],'max':max(v)}
names=[f'human-1024-{i}' for i in range(1,6)]+['human-2048-comparison']
summary=json.loads(subprocess.check_output(['python3',str(root/'tools/forge-benchmark/summarize.py'),str(a.artifacts)],text=True));save('measurements.json',summary)
checks={};active={};cpu=[];ack=[]
for name in names:
    run=a.artifacts/name;t=json.loads((run/'out/transcript.json').read_text());views=[x['details'] for x in t if x['event']=='projection'];endpoint=json.loads((run/'endpoint.json').read_text())
    assert endpoint['turn']==12 and endpoint['turnPlayer']=='Human' and endpoint['prompt']['kind']=='InputPassPriority'
    for d in views:
        assert set(d)<=set('status session revision players stack turn phase turnPlayer prompt actionError'.split())
        assert d['status']!='blocked'
        for i,player in enumerate(d['players']):
            for z in player['zones']:
                for c in z['cards']:assert set(c)<=set('name tapped attacking blocking id mana type pt selected'.split())
                if i==1 and z['zone']=='Library':assert not z['cards']
                if i==1 and z['zone']=='Hand' and z['cards']:
                    assert d['prompt']['kind']=='InputConfirm' and d['prompt']['text']=="Looking at cards in CPU's hand"
                    assert [c['name'] for c in z['cards']]==['Plains']
                if i==0 and z['zone']=='Library' and z['cards']:
                    assert d['prompt']['kind']=='InputSelectEntitiesFromList'
                    assert [c['name'] for c in z['cards']]==sorted(c['name'] for c in z['cards'])
        assert all(set(s)=={'source'} for s in d['stack'])
    orders=[(i,x['details']) for i,x in enumerate(t) if x['event']=='order-returned'];assert len(orders)==1
    i,order=orders[0];offered=next(x['details']['prompt'] for x in reversed(t[:i]) if x['event']=='projection' and x['details'].get('prompt',{}).get('ordering'))
    selected=[offered['options'][j]['label'].split(' — ')[0] for j in order['selected']]
    assert selected==['Sword of the Animist','Arahbo, Roar of the World'] and order['rememberDecision']==False
    resolved=[x['details']['source'] for x in t[i:] if x['event']=='resolved-public-source'][:2];assert resolved==selected
    assert all(x['details']['accepted'] for x in t if x['event']=='browser-action')
    assert any(d['turn']==8 and d['players'][1]['life']==34 for d in views)
    info=json.loads((run/'container-initial.json').read_text())[0]['HostConfig'];cap=2048 if '2048' in name else 1024
    assert info['Memory']==cap*2**20 and info['MemorySwap']==cap*2**20 and info['NanoCpus']==10**9
    assert json.loads((run/'supervision.json').read_text())['container_removed']
    meta=json.loads((run/'command.json').read_text());assert meta['identity']['adapter_sha256']==sha(root/'tools/forge-browser-spike/src/BrowserSpike.java')
    ev=[json.loads(l) for l in (run/'out/events.jsonl').read_text().splitlines()]
    boundary=next(e for e in ev if e['event']=='view' and e['value'].get('turn',0)>=3)
    start=boundary['seconds']
    end=next(e['seconds'] for e in ev if e['event']=='cancel')
    rows=[r for r in csv.DictReader((run/'out/samples.csv').open()) if start<=float(r['seconds'])<end and r['worker_alive']=='true'];recent=[r for r in rows if float(r['seconds'])>=end-30]
    active[name]={'first_observed_turn':boundary['value']['turn'],'exact_turn3_boundary_observed':boundary['value']['turn']==3,'start_seconds':start,'end_seconds':end,'sample_count':len(rows),'memory_mib':stats([int(r['memory_current'])/2**20 for r in rows]),'last_30_seconds_memory_mib':stats([int(r['memory_current'])/2**20 for r in recent]),'cgroup_cpu_seconds':(int(rows[-1]['cpu_usec'])-int(rows[0]['cpu_usec']))/1e6}
    checks[name]={'projected_views_checked':len(views),'turn12_endpoint':True,'selected_sources':selected,'resolved_sources':resolved,'original_option_indices':order['selected'],'transcript_sha256':sha(run/'out/transcript.json'),'command_sha256':sha(run/'command.json')}
    if cap==1024:
        cpu.extend(int(l.split(',')[3])/1e6 for l in (run/'out/decisions.csv').read_text().splitlines() if l.startswith('end,'))
        ack.extend(e['value']['milliseconds'] for e in ev if e['event']=='http' and e['value']['path']=='/action' and e['value']['code']==200)
repeats=[r for r in summary['runs'] if r['name'] in names[:5]]
# Audit all historical files, allowing only the specifically scoped current edits.
allowed={'docs/v2/STATUS.md','docs/v2/evidence/V2-012.md','tools/forge-browser-spike/src/BrowserSpike.java','tools/forge-browser-spike/index.html','tools/forge-browser-spike/README.md','tools/forge-browser-spike/regression.py','tools/forge-benchmark/README.md','tools/forge-benchmark/run.py'}
baseline=json.loads((a.artifacts/'baseline-files.json').read_text());changed=[]
for name,h in baseline.items():
    if sha(root/name)!=h:assert name in allowed,name;changed.append(name)
for source,target in [('focused-checks.json','focused-checks.json'),('protocol-final/checks.json','protocol.json'),('projection-final/checks.json','projection.json'),('workload.json','workload.json')]:
    value=json.loads((a.artifacts/source).read_text())
    if 'passed' in value:assert value['passed']
    save(target,value)
save('active-windows.json',{'scope':'First sampled view at/after CPU turn 3 through endpoint cancellation; coarse sampler misses turn 3 in some runs (flagged individually), so exact active-window boundary is unverified there. Includes human/tool thinking, not engine latency. Last 30 seconds is a live sample window, not proof of long-game plateau.','runs':active})
save('aggregate.json',{'one_gib_repetitions':5,'startup_seconds':stats([r['host_readiness']['host_observed_seconds'] for r in repeats]),'peak_memory_mib':stats([r['whole_cgroup_peak_mib'] for r in repeats]),'cpu_decision_ms':stats(cpu),'command_ack_ms':stats(ack),'cleanup_seconds':stats([r['supervision']['cleanup_seconds'] for r in repeats]),'headroom_passes':sum(r['headroom_fraction']>=.2 for r in repeats),'oom_count':sum(r['oom_killed'] for r in repeats)})
save('checks.json',{'artifact_audit':'passed','scope':'Fixed turn-12 automated real-browser workload and ordering boundary; not broad Commander compatibility or hosting acceptance','runs':checks,'preserved_preexisting_files':len(baseline)-len(changed),'authorized_changed_preexisting_files':changed,'source_hashes':{f:sha(root/f) for f in ['tools/forge-browser-spike/src/BrowserSpike.java','tools/forge-browser-spike/index.html','tools/forge-browser-spike/src/OrderingProtocolRegression.java','tools/forge-benchmark/verify_ordering.py','tools/forge-benchmark/finish_workload.py']}})
print('PASS artifact/privacy/ordering/endpoint/cleanup audit; assess performance targets separately')
