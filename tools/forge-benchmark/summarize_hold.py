#!/usr/bin/env python3
"""Summarize the single V2-013 hold, preserving total charges and diagnostic overhead."""
import argparse,csv,hashlib,json,re,statistics,subprocess
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('artifacts',type=Path);p.add_argument('--out',type=Path,required=True);a=p.parse_args();a.out.mkdir(parents=True,exist_ok=True)
root=Path(__file__).resolve().parents[2];run=a.artifacts/'duration-hold-1024';out=run/'out'
def load(p):return json.loads(p.read_text())
def save(n,v):(a.out/n).write_text(json.dumps(v,indent=2)+'\n')
def sha(p):return hashlib.sha256(p.read_bytes()).hexdigest()
start=load(run/'hold-start.json');end=load(run/'hold-end.json');assert end['completed'] and start['view_sha256']==end['view_sha256']
lo=float(start['sample']['seconds']);hi=float(end['sample']['seconds']);rows=list(csv.DictReader((out/'samples.csv').open()));held=[r for r in rows if lo<=float(r['seconds'])<=hi];ev=[json.loads(l) for l in (out/'events.jsonl').read_text().splitlines()]
windows=[]
for second in range(0,600,60):
    rs=[r for r in held if lo+second<=float(r['seconds'])<lo+second+60];http=[x['value'] for x in ev if lo+second<=x['seconds']<lo+second+60 and x['event']=='http']
    windows.append({'hold_minute':second//60+1,'sample_count':len(rs),'median_mib':{k:statistics.median(int(r[k]) for r in rs)/(1024 if k=='worker_rss_kib' else 2**20) for k in ['memory_current','anon','file','kernel','worker_rss_kib']},'min_mib':min(int(r['memory_current']) for r in rs)/2**20,'max_mib':max(int(r['memory_current']) for r in rs)/2**20,'proxy_state_requests':sum(x['path']=='/state' for x in http),'proxy_response_payload_bytes':sum(x['response_bytes'] for x in http)})
probes=[]
for second in [0,300,600]:
    d=run/'hold-probes'/str(second);record=load(d/'probe.json');assert all(v==0 for v in record['return_codes'].values());record['jvm']={}
    for kind in ['worker','supervisor']:
        status=(d/(kind+'-status.txt')).read_text();heap=(d/(kind+'-heap.txt')).read_text();generations=re.findall(r'(?:def new generation|tenured generation)\s+total (\d+)K, used (\d+)K',heap)
        record['jvm'][kind]={'rss_mib':int(re.search(r'VmRSS:\s+(\d+)',status)[1])/1024,'threads':int(re.search(r'Threads:\s+(\d+)',status)[1]),'heap_committed_mib':sum(int(x[0]) for x in generations)/1024,'heap_used_mib':sum(int(x[1]) for x in generations)/1024}
    nmt=(d/'worker-nmt.txt').read_text();m=re.search(r'Total: reserved=(\d+)KB, committed=(\d+)KB',nmt);h=re.search(r'Java Heap \(reserved=(\d+)KB, committed=(\d+)KB',nmt)
    record['worker_nmt_mib']={'reserved':int(m[1])/1024,'committed':int(m[2])/1024,'heap_committed':int(h[2])/1024,'tracked_nonheap_committed':(int(m[2])-int(h[2]))/1024};probes.append(record)
held_http=[e['value'] for e in ev if lo<=e['seconds']<=hi and e['event']=='http'];assert not any(x['path']=='/action' for x in held_http)
obs=load(run/'hold-observations.json');assert len(obs)==21
supervision=load(run/'supervision.json');assert supervision['container_removed']
final=load(run/'container-final.json')[0]
meta=load(run/'command.json');old=load(a.artifacts.parent/'v2-012-ordering/human-1024-3/command.json')
assert meta['engine_argv']==old['engine_argv'] and meta['identity']==old['identity'] and meta['preferences_sha256']==old['preferences_sha256']
initial=load(run/'container-initial.json')[0]['HostConfig'];assert initial['Memory']==1024*2**20 and initial['MemorySwap']==1024*2**20 and initial['NanoCpus']==10**9
baseline=load(a.artifacts/'baseline-files.json');changed=[f for f,h in baseline.items() if sha(root/f)!=h];assert changed==['docs/v2/STATUS.md'],changed
peak=max(int(r['memory_peak']) for r in rows);cpu=(int(held[-1]['cpu_usec'])-int(held[0]['cpu_usec']))/1e6
save('hold-summary.json',{'scope':'One fixed turn-12 state under AMD64 emulation; includes diagnostic processes, supervisor, cache and kernel. Window trends are descriptive, not causal attribution or proof of indefinite plateau.','requested_hold_seconds':600,'observed_including_final_probe_seconds':end['elapsed_seconds'],'sampled_interval_seconds':hi-lo,'whole_run_peak_mib':peak/2**20,'headroom_fraction':1-peak/(1024*2**20),'start_current_mib':int(start['sample']['memory_current'])/2**20,'pre_final_probe_current_mib':int(obs[-1]['sample']['memory_current'])/2**20,'worker_exit':load(out/'worker-exit.json'),'end_current_mib':int(end['sample']['memory_current'])/2**20,'hold_cgroup_cpu_seconds':cpu,'minute_windows':windows,'probes':probes,'http':{'state_get_count':sum(x['path']=='/state' for x in held_http),'successful_response_payload_bytes':sum(x['response_bytes'] for x in held_http),'request_payload_bytes':sum(x['request_bytes'] for x in held_http),'includes':'200ms browser polling plus explicit observer reads; excludes internal supervisor probes and wire framing'},'file_growth_bytes':{n:end['files'][n]-start['files'].get(n,0) for n in end['files']},'transcript_unchanged_size':end['files']['transcript.json']==start['files']['transcript.json'],'oom_killed':final['State']['OOMKilled'],'supervision':supervision})
save('checks.json',{'fixed_view_all_21_observations':True,'no_game_commands_during_hold':True,'same_engine_jvm_configuration_and_instrumentation_as_previous_1g_run':True,'same_enforced_1g_one_cpu_no_swap':True,'prior_files_preserved_except_authorized_status':len(baseline)-1,'container_removed':True,'command_sha256':sha(run/'command.json'),'source_hashes':{f:sha(root/f) for f in ['tools/forge-benchmark/observe_hold.py','tools/forge-benchmark/summarize_hold.py','tools/forge-browser-spike/src/BrowserSpike.java','tools/forge-browser-spike/index.html']}})
save('hold-plan.json',load(a.artifacts/'hold-plan.json'))
save('measurements.json',json.loads(subprocess.check_output(['python3',str(root/'tools/forge-benchmark/summarize.py'),str(a.artifacts)],text=True)))
print('PASS fixed-state, fixed-settings, retention and cleanup audit; resource targets assessed separately')
