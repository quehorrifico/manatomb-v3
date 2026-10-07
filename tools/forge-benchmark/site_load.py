#!/usr/bin/env python3
"""Exploratory read-only local ManaTomb load; refuses non-local targets."""
import argparse,json,time,urllib.request,urllib.error,urllib.parse,math
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('--output',type=Path,required=True);p.add_argument('--seconds',type=int,default=30);p.add_argument('--base',default='http://localhost:18080');a=p.parse_args()
assert urllib.parse.urlparse(a.base).hostname in ('localhost','127.0.0.1')
paths=['/cards/search/autocomplete?q=forest','/cards/search/autocomplete?q=island','/cards/search/autocomplete?q=mountain','/healthz']
# Fixed offered load, sequential requests: record schedule lag rather than hide overload.
rows=[];start=time.monotonic();i=0
while time.monotonic()-start<a.seconds:
    target=start+i*.1;time.sleep(max(0,target-time.monotonic()));began=time.monotonic();code=0;size=0
    try:
        with urllib.request.urlopen(a.base+paths[i%len(paths)],timeout=5) as r:code=r.status;size=len(r.read())
    except urllib.error.HTTPError as e:code=e.code
    except Exception:pass
    rows.append({'ms':(time.monotonic()-began)*1000,'status':code,'response_bytes':size,'start_lag_ms':(began-target)*1000,'path':paths[i%len(paths)]});i+=1
values=sorted(r['ms'] for r in rows)
result={'scope':'Native Mac Go site and local PostgreSQL; separate-process exploratory test, not capped Linux cohosting','target':a.base,'offered_requests_per_second':10,'seconds':a.seconds,'n':len(rows),'p50_ms':values[math.ceil(len(values)*.5)-1],'p95_ms':values[math.ceil(len(values)*.95)-1],'max_ms':max(values),'errors':sum(r['status']!=200 for r in rows),'response_payload_bytes':sum(r['response_bytes'] for r in rows),'max_schedule_lag_ms':max(r['start_lag_ms'] for r in rows),'samples':rows}
a.output.write_text(json.dumps(result,indent=2)+'\n');print(json.dumps({k:v for k,v in result.items() if k!='samples'}))
