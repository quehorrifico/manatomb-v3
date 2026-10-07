#!/usr/bin/env python3
"""Audit captured human DTOs; this does not operate a browser or replay a game."""
import argparse, hashlib, json
from pathlib import Path
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('work',type=Path,help='V2-011 external artifact directory')
args=parser.parse_args()
root=Path(__file__).resolve().parents[2]
all_inputs=set();all_choices=set();summaries={}
for name in ('realistic','focused','blocking','final-blocking'):
    path=args.work/name/'transcript.json'
    raw=path.read_text();records=json.loads(raw)
    projections=[r['details'] for r in records if r['event']=='projection']
    inputs={r['details']['kind'] for r in records if r['event']=='input'}
    choices={r['details']['kind'] for r in records if r['event']=='choice'}
    all_inputs.update(inputs);all_choices.update(choices)
    # Hidden-marker identity must never leave the worker, even in prompt prose.
    assert 'Benalish Hero' not in raw, name
    for d in projections:
        assert len(d['players'])==2
        assert all(not z['cards'] for z in d['players'][1]['zones'] if z['zone'] in ('Hand','Library')),name
        assert set(d)<=set('status session revision players stack turn phase turnPlayer prompt actionError'.split())
        for p in d['players']:
            assert set(p)==set('id name life zones'.split())
            for z in p['zones']:
                assert set(z)==set('zone count cards'.split())
                for c in z['cards']:
                    assert set(c)<=set('name tapped attacking blocking id mana type pt selected'.split())
        for s in d['stack']: assert set(s)=={'source'}
    scry_indices=[i for i,d in enumerate(projections) if d.get('prompt',{}).get('kind')=='manipulateCardList']
    for i in scry_indices:
        lib=lambda d:next(z['cards'] for z in d['players'][0]['zones'] if z['zone']=='Library')
        assert len(lib(projections[i]))==1
        assert not lib(projections[i+1]),'Reveal was not revoked'
        if name!='focused': assert '[hidden card]' not in projections[i]['prompt']['text']
    summaries[name]={'events':len(records),'human_projections':len(projections),'inputs':sorted(inputs),'choices':sorted(choices),'natural_results':sum(r['event']=='natural-result' for r in records),'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),'bytes':path.stat().st_size}
    if name=='focused':
        assert any('{2}{U}{R}' in d.get('prompt',{}).get('text','') for d in projections),'Commander tax missing'
        positions=[]
        for d in projections:
            pos=tuple(z['zone'] for z in d['players'][0]['zones'] if any(c['name']=='Balmor, Battlemage Captain' for c in z['cards']))
            if not positions or positions[-1]!=pos:positions.append(pos)
        assert positions.count(('Command',))>=2 and positions.count(('Battlefield',))>=2,positions
    if name=='final-blocking':
        assert any(not r['details']['accepted'] for r in records if r['event']=='browser-action')
        assert any(len(d['stack'])==4 for d in projections),'Stack response missing'
        assert any(any(c['blocking'] for z in d['players'][0]['zones'] for c in z['cards']) for d in projections)
        assert summaries[name]['natural_results']==1
        threads={r['details']['thread'] for r in records if r['event']=='choice-unblocked'}
        assert 'AWT-EventQueue-0' in threads and 'Game V2-011' in threads,threads
        cmd=json.loads((args.work/name/'command.json').read_text())
        assert cmd['adapter_sha256']==hashlib.sha256((root/'tools/forge-browser-spike/src/BrowserSpike.java').read_bytes()).hexdigest()
assert {'InputConfirmMulligan','InputLondonMulligan','InputSelectTargets','InputPayManaOfCostPayment','InputPassPriority','InputAttack','InputBlock','InputConfirm'}<=all_inputs
assert {'getAbilityToPlay','manipulateCardList','reveal'}<=all_choices
final=json.loads((args.work/'final-blocking/final-state.json').read_text())
assert final['status']=='finished' and final['result']=='Human' and final['turn']==2
# Forge removes the losing player from game.getPlayers() at the natural result.
assert final['players'][0]['name']=='Human' and final['players'][0]['life']==40
print(json.dumps({'result':'passed','scope':'captured DTO/protocol evidence invariants; not browser automation or exhaustive privacy proof','runs':summaries,'final_result':{k:final[k] for k in ('status','result','turn','phase')}},indent=2))
