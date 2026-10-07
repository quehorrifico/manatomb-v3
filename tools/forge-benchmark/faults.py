#!/usr/bin/env python3
"""Local lifecycle probes only. Never send a new valid gameplay choice."""
import argparse,json,re,socket,subprocess,time,urllib.request,urllib.error
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('operation',choices=['capture','replacement','interrupt','cancel','kill','stuck','cpu-cancel','cpu-kill'])
p.add_argument('--run',type=Path,required=True)
p.add_argument('--previous',type=Path)
a=p.parse_args();base='http://127.0.0.1:18711'
def state():return json.load(urllib.request.urlopen(base+'/state',timeout=5))
def token():return re.search("const token='([^']+)'",urllib.request.urlopen(base+'/',timeout=5).read().decode())[1]
def post(path,body):
    request=urllib.request.Request(base+path,data=json.dumps(body).encode(),headers={'Origin':base,'Content-Type':'application/json'},method='POST')
    try:
        with urllib.request.urlopen(request,timeout=10) as r:return r.status,r.read().decode()
    except urllib.error.HTTPError as e:return e.code,e.read().decode()
def same(x,y):return all(x.get(k)==y.get(k) for k in ('session','revision','prompt'))
result={};started=time.monotonic()
if a.operation=='capture':
    s=state();body={k:s[k] for k in ['session','revision']};body.update(csrf=token(),prompt=s['prompt']['id'],request='retired-session-probe',action='ok')
    private=a.run/'retired-control.private.json';private.write_text(json.dumps(body));private.chmod(0o600);result={'captured':True}
elif a.operation=='replacement':
    before=state();body=json.loads(a.previous.read_text());assert before['session']!=body['session']
    code,error=post('/action',body);assert code==409
    body['csrf']=token();code2,error2=post('/action',body);assert code2==409
    result={'old_token_status':code,'new_token_old_session_status':code2,'new_token_old_session_error':error2,'replacement_unchanged':same(before,state())};assert result['replacement_unchanged']
elif a.operation=='interrupt':
    before=state()
    with socket.create_connection(('127.0.0.1',18711)) as sock:
        sock.sendall(b'POST /action HTTP/1.1\r\nHost: 127.0.0.1:18711\r\nOrigin: http://127.0.0.1:18711\r\nContent-Length: 100\r\nContent-Type: application/json\r\n\r\n{')
    time.sleep(.3);result={'same_session_prompt_revision':same(before,state())};assert result['same_session_prompt_revision']
else:
    if a.operation.startswith('cpu-'):
        deadline=time.monotonic()+150
        while time.monotonic()<deadline:
            path=a.run/'out/decisions.csv'
            if path.exists() and path.read_text().splitlines()[-1].startswith('begin,'):break
            time.sleep(.01)
        else:raise RuntimeError('No active instrumented CPU call found')
        result['active_cpu_call_observed']=True
    if a.operation=='stuck':
        identity=json.loads((a.run/'container-initial.json').read_text())[0];events=[json.loads(l) for l in (a.run/'out/events.jsonl').read_text().splitlines()]
        pid=next(x['value'] for x in events if x['event']=='worker-start')
        subprocess.run(['docker','exec',identity['Id'],'kill','-STOP',str(pid)],check=True);result['worker_stopped']=True
    began=time.monotonic();code,body=post('/bench/kill' if a.operation in ('kill','cpu-kill') else '/bench/cancel',{})
    result.update(status=code,worker_reap_response_seconds=time.monotonic()-began);assert code==200
result['elapsed_seconds']=time.monotonic()-started
(a.run/('probe-'+a.operation+'.json')).write_text(json.dumps(result,indent=2)+'\n');print(json.dumps(result))
