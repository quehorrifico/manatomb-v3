#!/usr/bin/env python3
"""Audit the bounded V2-012 correction artifacts; does not play a game or claim full acceptance."""
import argparse,csv,hashlib,json,statistics,subprocess
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('artifacts',type=Path);p.add_argument('--out',type=Path,required=True);a=p.parse_args()
root=Path(__file__).resolve().parents[2];a.out.mkdir(parents=True,exist_ok=True)
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
def save(name,data):(a.out/name).write_text(json.dumps(data,indent=2)+'\n')
measurements=json.loads(subprocess.check_output(['python3',str(root/'tools/forge-benchmark/summarize.py'),str(a.artifacts)],text=True))
save('measurements.json',measurements)
run=a.artifacts/'human-pilot-2';events=[json.loads(l) for l in (run/'out/events.jsonl').read_text().splitlines()]
trace=json.loads((run/'out/transcript.json').read_text());views=[x['details'] for x in trace if x['event']=='projection']
assert len(views)==100
for d in views:
    assert set(d)<=set('status session revision players stack turn phase turnPlayer prompt actionError'.split())
    for i,player in enumerate(d['players']):
        for z in player['zones']:
            for c in z['cards']:assert set(c)<=set('name tapped attacking blocking id mana type pt selected'.split())
            if i==1 and z['zone']=='Library':assert not z['cards']
            if i==1 and z['zone']=='Hand' and z['cards']:
                assert d['prompt']['kind']=='InputConfirm' and d['prompt']['text']=='Looking at cards in CPU\'s hand'
                assert [c['name'] for c in z['cards']]==['Plains']
            if i==0 and z['zone']=='Library' and z['cards']:
                assert d['prompt']['kind']=='InputSelectEntitiesFromList'
                assert [c['name'] for c in z['cards']]==sorted(c['name'] for c in z['cards'])
    assert all(set(s)=={'source'} for s in d['stack'])
context=[x['details'] for x in trace if x['event']=='ability-context'];assert context
assert all(x['viewDescription']==' [Phase: ]' and not x['scrubChangedDescription'] and not x['metadataRemoved'] for x in context)
assert any(d['turn']==6 and any(c['name']=='Leonin Shikari' and c['pt']=='6/6' for z in d['players'][0]['zones'] for c in z['cards']) for d in views)
assert any(d['turn']==8 and d['players'][1]['life']==34 for d in views)
assert any(d['turn']==10 and d['prompt']['kind']=='InputAttack' for d in views)
assert 'Unmapped GUI method: order' in (run/'out/engine.log').read_text()
assert all(x['details']['accepted'] for x in trace if x['event']=='browser-action')
assert not any(x['event']=='natural-result' for x in trace)
# Derived window uses the phase boundary specified before the first coverage pilot.
start=next(e['seconds'] for e in events if e['event']=='view' and e['value'].get('turn')==3)
end=next(e['seconds'] for e in events if e['event']=='view' and e['value']['status']=='blocked')
rows=[r for r in csv.DictReader((run/'out/samples.csv').open()) if start<=float(r['seconds'])<=end]
recent=[r for r in rows if float(r['seconds'])>=end-60]
def memory(rows):
    values=[int(r['memory_current'])/2**20 for r in rows]
    return {'n':len(values),'min_mib':min(values),'median_mib':statistics.median(values),'max_mib':max(values)}
save('active-window.json',{'scope':'Turn 3 CPU UPKEEP through blocked view; sampled boundaries nominal 1 s, includes human thinking; not engine elapsed latency','start_seconds':start,'end_seconds':end,'memory':memory(rows),'last_60_seconds_live_memory':memory(recent),'cgroup_cpu_seconds_delta':(int(rows[-1]['cpu_usec'])-int(rows[0]['cpu_usec']))/1e6,'decision_scope':'Full process 109 instrumented calls, including earlier CPU turn; no timestamp alignment to supervisor invented'})
reg=json.loads((a.artifacts/'regression-complete/checks.json').read_text());assert reg['passed']
save('regression.json',reg)
save('browser-checks.json',json.loads((a.artifacts/'stage1-browser-checks.json').read_text()))
save('workload.json',json.loads((a.artifacts/'workload.json').read_text()))
allowed={'docs/v2/STATUS.md','docs/v2/evidence/V2-012.md','tools/forge-browser-spike/src/BrowserSpike.java','tools/forge-browser-spike/README.md','tools/forge-benchmark/README.md'}
baseline=json.loads((a.artifacts/'baseline-files.json').read_text());changed=[]
for name,h in baseline.items():
    if sha(root/name)!=h:assert name in allowed,name;changed.append(name)
for name in ['original-reproduction','corrected-browser','human-pilot-2']:
    info=json.loads((a.artifacts/name/'container-initial.json').read_text())[0]
    assert info['HostConfig']['Memory']==1073741824 and info['HostConfig']['MemorySwap']==1073741824 and info['HostConfig']['NanoCpus']==1000000000
    assert json.loads((a.artifacts/name/'supervision.json').read_text())['container_removed']
files=[root/'tools/forge-browser-spike/src/BrowserSpike.java',root/'tools/forge-browser-spike/src/AbilityProjectionRegression.java',root/'tools/forge-browser-spike/regression.py',root/'tools/forge-benchmark/verify_resume.py']
save('checks.json',{'artifact_audit':'passed','new_active_workload':'blocked, not five completed repetitions','projected_views_checked':len(views),'cpu_library_excluded':True,'authorized_search_views_sorted':True,'reveals_limited_to_actual_prompt_scopes':True,'all_recorded_actions_accepted_by_forge':True,'source_hashes':{str(f.relative_to(root)):sha(f) for f in files},'preserved_preexisting_files':len(baseline)-len(changed),'authorized_changed_preexisting_files':changed,'runs':{name:{'command_sha256':sha(a.artifacts/name/'command.json'),'transcript_sha256':sha(a.artifacts/name/'out/transcript.json')} for name in ['original-reproduction','corrected-browser','human-pilot-2']}})
print('PASS artifact/projection/boundary/regression audit; full V2-012 remains blocked by ordering prompt and incomplete human repetitions')
