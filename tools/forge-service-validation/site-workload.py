#!/usr/bin/env python3
"""30 low-rate requests to the named disposable local site. Never production.

Ten search/deck/save requests each, at most one request/second. Saves write the
existing fixture overview unchanged and verify it afterward. No gameplay input.
"""
import argparse, hashlib, http.cookiejar, json, math, subprocess, time, urllib.request
from pathlib import Path

p=argparse.ArgumentParser()
p.add_argument('--mode',choices=['baseline','human-prompt'],required=True)
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
assert not a.output.exists(), 'Fresh output required'
base='http://127.0.0.1:18880'
def docker(*args): return subprocess.check_output(['docker',*args],text=True).strip()
assert json.loads(docker('context','inspect'))[0]['Endpoints']['docker']['Host'].startswith('unix://')
def fixture():
    value=json.loads(docker('exec','manatomb-bug-regression','psql','-U','postgres','-d','manatomb_v2_local','-Atc',"select row_to_json(d) from (select d.id,d.name,d.description,d.tags,d.format from decks d join users u on u.id=d.user_id where d.id=1 and u.email='v2-local@example.invalid') d"))
    assert value['name']=='V2 local recovery fixture' and value['format']=='Commander'
    return value
before=fixture()
engine='manatomb-forge-release-local'
workers=lambda: [r for r in docker('exec',engine,'ps','-eo','comm').splitlines() if r.strip()=='java']
assert len(workers())==(0 if a.mode=='baseline' else 1), 'Expected worker condition not present'
identities={name:json.loads(docker('inspect',name))[0]['Image'] for name in ['manatomb-web-release-local',engine,'manatomb-bug-regression']}
cookies=http.cookiejar.CookieJar()
opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(cookies))
from urllib.parse import urlencode
login=urlencode({'email':'v2-local@example.invalid','password':'Local-v2-test-only-2026','stay_signed_in':'0','next':'/cpu'}).encode()
with opener.open(urllib.request.Request(base+'/login',data=login),timeout=10) as r:
    r.read()
assert any(c.name for c in cookies), 'Local authentication failed'
def cpu():
    with opener.open(base+'/api/cpu/sessions',timeout=10) as r:return json.load(r)
fixed=None
if a.mode=='human-prompt':
    fixed=cpu();assert fixed['state'].get('prompt') and fixed['status'] in ('input','choice')
body=urlencode({'action':'save_overview','description':before['description'],'tags':before['tags'],'format':before['format']}).encode()
rows=[];start=time.monotonic()
for i in range(30):
    time.sleep(max(0,start+i-time.monotonic()))
    kind=['search','deck','save'][i%3]
    path='/cards?q=Forest' if kind=='search' else '/decks/1'
    raw=body if kind=='save' else None
    headers={'Origin':base,'Accept':'application/json' if kind=='save' else 'text/html'}
    begin=time.monotonic()
    with opener.open(urllib.request.Request(base+path,data=raw,headers=headers),timeout=10) as r:
        data=r.read();status=r.status;url=r.url
    assert status==200 and '/login' not in url,(kind,status)
    if kind=='save':assert isinstance(json.loads(data),dict)
    else:assert b'ManaTomb' in data and (b'Forest' if kind=='search' else b'V2 local recovery fixture') in data
    rows.append({'kind':kind,'status':status,'milliseconds':round((time.monotonic()-begin)*1000,3),'response_body_bytes':len(data),'request_body_bytes':len(raw or b'')})
assert fixture()==before,'Fixture overview changed'
assert len(workers())==(0 if a.mode=='baseline' else 1)
if fixed:
    after=cpu();assert after['id']==fixed['id'] and after['state']['prompt']['id']==fixed['state']['prompt']['id'],'Human decision changed during hold'
summary={}
for kind in ['search','deck','save']:
    values=sorted(r['milliseconds'] for r in rows if r['kind']==kind)
    summary[kind]={'n':len(values),'p50_ms':values[math.ceil(len(values)*.5)-1],'p95_ms':values[math.ceil(len(values)*.95)-1],'max_ms':max(values)}
report={'mode':a.mode,'passed':True,'scope':'Local Linux AMD64 containers under Rosetta on ARM64; separate web/engine/database. Fixed idle human prompt when present. Warm/unspecified cache; small synthetic catalog; not production capacity or active CPU decision load. At most one site request/second; browser polling is additional and not included in these body byte totals.', 'images':identities,'samples':rows,'summary':summary,'duration_seconds':round(time.monotonic()-start,3),'fixture_overview_preserved':True,'prompt_kind':fixed['state']['prompt']['kind'] if fixed else None}
a.output.parent.mkdir(parents=True,exist_ok=True);a.output.write_text(json.dumps(report,indent=2)+'\n')
print(json.dumps(summary))
