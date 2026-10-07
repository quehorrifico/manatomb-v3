#!/usr/bin/env python3
"""Summarize measured files without copying engine logs, tokens, or hidden game data."""
from pathlib import Path
import argparse,csv,json,math,re,statistics
p=argparse.ArgumentParser();p.add_argument('artifacts',type=Path);a=p.parse_args()
def percentile(v,p):return sorted(v)[max(0,math.ceil(len(v)*p)-1)] if v else None
def stats(v):return {'n':len(v),'p50':percentile(v,.5),'p95':percentile(v,.95),'max':max(v) if v else None}
results=[]
for run in sorted(a.artifacts.iterdir()):
    if not (run/'supervision.json').exists():continue
    meta=json.loads((run/'command.json').read_text());rows=list(csv.DictReader((run/'out/samples.csv').open()))
    events=[json.loads(l) for l in (run/'out/events.jsonl').read_text().splitlines()]
    ready=[x['seconds'] for x in events if x['event']=='ready']
    cpu=[]
    if (run/'out/decisions.csv').exists():
        for l in (run/'out/decisions.csv').read_text().splitlines():
            if l.startswith('end,'):cpu.append(int(l.split(',')[3])/1e6)
    acknowledgments=[x['value']['milliseconds'] for x in events if x['event']=='http' and x['value']['path']=='/action' and x['value']['code']==200]
    memory=[int(r['memory_current'])/2**20 for r in rows]
    idle=[int(r['memory_current'])/2**20 for r in rows if ready and float(r['seconds'])>=ready[0]+5 and r['worker_alive']=='true']
    log=(run/'out/engine.log').read_text(errors='replace');natural=re.search(r'Game Result: Game 1 ended in (\d+) ms\.',log)
    reaps=[x['seconds'] for x in events if x['event']=='worker-reaped'];cancel=[x['seconds'] for x in events if x['event'] in ('cancel','force-kill')]
    final=json.loads((run/'container-final.json').read_text())[0]
    record={'name':run.name,'mode':meta['mode'],'scenario':meta['scenario'],'cap_mib':meta['memory_mib'],'sample_count':len(rows),'nominal_sample_interval_ms':250,'supervisor_ready_seconds':ready[0] if ready else None,'host_readiness':json.loads((run/'readiness.json').read_text()) if (run/'readiness.json').exists() else None,'whole_cgroup_peak_mib':max(int(r['memory_peak']) for r in rows)/2**20,'sampled_memory_mib':stats(memory),'post_ready_memory_mib':stats(idle),'worker_peak_rss_mib':max(int(r['worker_rss_kib']) for r in rows)/1024,'worker_peak_threads':max(int(r['worker_threads']) for r in rows),'cpu_decision_ms':stats(cpu),'successful_command_ack_ms':stats(acknowledgments),'ai_natural_game_ms':int(natural[1]) if natural else None,'sampled_worker_gone':any(r['worker_alive']=='false' and r['worker_threads']=='-1' for r in rows),'cancel_to_reap_seconds':min(reaps)-min(cancel) if reaps and cancel else None,'oom_killed':final['State']['OOMKilled'],'supervision':json.loads((run/'supervision.json').read_text()),'http_request_payload_bytes':sum(x['value']['request_bytes'] for x in events if x['event']=='http'),'http_response_payload_bytes':sum(x['value']['response_bytes'] for x in events if x['event']=='http'),'file_bytes':{x.name:x.stat().st_size for x in (run/'out').iterdir() if x.is_file()}}
    record['headroom_fraction']=1-record['whole_cgroup_peak_mib']/record['cap_mib']
    results.append(record)
print(json.dumps({'runs':results,'scope':'Engine plus local supervisor in same capped cgroup; excludes browser and existing ManaTomb. Post-ready rows in interactive runs include gameplay/thinking, not a pure idle statistic.'},indent=2))
