#!/usr/bin/env python3
"""Bounded operator API smoke on the named disposable local engine. No game starts."""
import argparse,json,subprocess,urllib.request,urllib.error,uuid
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--env-dir',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args()
assert not a.output.exists()
def docker(*args):return subprocess.check_output(['docker',*args],text=True).strip()
assert json.loads(docker('context','inspect'))[0]['Endpoints']['docker']['Host'].startswith('unix://')
secret=dict(line.split('=',1) for line in (a.env_dir/'local-engine.env').read_text().splitlines() if '=' in line)['FORGE_SERVICE_SECRET']
def req(path,method='GET',body=None,auth=True,expected=200):
 headers={'Content-Type':'application/json','X-ManaTomb-Owner':'10001'}
 if auth:headers['Authorization']='Bearer '+secret
 r=urllib.request.Request('http://127.0.0.1:18881'+path,data=None if body is None else json.dumps(body).encode(),headers=headers,method=method)
 try:result=urllib.request.urlopen(r,timeout=10)
 except urllib.error.HTTPError as error:result=error
 data=json.load(result);assert result.status==expected,(path,result.status);return data
before=req('/v1/admin/status');assert before['activeWorkers']==0 and not before['draining']
req('/v1/admin/status',auth=False,expected=401)
try:
 req('/v1/admin/drain','POST',{'drain':True})
 req('/readyz',auth=False,expected=503);req('/healthz',auth=False)
 deck={'name':'Never started','commanders':[{'name':'Silvos, Rogue Elemental','quantity':1}],'main':[{'name':'Forest','quantity':99}]}
 req('/v1/sessions','POST',{'request':str(uuid.uuid4()),'human':deck,'cpu':deck},expected=503)
 assert req('/v1/admin/status')['activeWorkers']==0
finally: req('/v1/admin/drain','POST',{'drain':False})
req('/readyz',auth=False)
a.output.parent.mkdir(parents=True,exist_ok=True)
a.output.write_text(json.dumps({'passed':True,'image':json.loads(docker('inspect','manatomb-forge-release-local'))[0]['Image'],'build':before['build'],'checks':['unauthenticated operator rejected','drain rejects new session without worker','liveness remains 200 while readiness 503','resume readiness 200'],'activeWorkers':req('/v1/admin/status')['activeWorkers']},indent=2)+'\n')
print('PASS packaged local operator endpoints; no worker allocated')
