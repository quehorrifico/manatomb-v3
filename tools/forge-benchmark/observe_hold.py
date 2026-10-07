#!/usr/bin/env python3
"""One fixed-state hold with read-only probes; never answers gameplay; always cancels worker."""
import argparse,csv,datetime,hashlib,json,subprocess,time,urllib.request
from pathlib import Path
p=argparse.ArgumentParser(description=__doc__);p.add_argument('run',type=Path);a=p.parse_args();run=a.run.resolve();out=run/'out';container='manatomb-v2012-'+run.name
info=json.loads(subprocess.check_output(['docker','inspect',container],text=True))[0]
assert info['Config']['Labels'].get('manatomb.benchmark')=='V2-012'
assert info['HostConfig']['Memory']==1024*2**20 and info['HostConfig']['NanoCpus']==10**9
probes=run/'hold-probes';probes.mkdir(exist_ok=False)
def write(p,v):p.write_text(json.dumps(v,indent=2)+'\n')
def state():return json.load(urllib.request.urlopen('http://127.0.0.1:18711/state',timeout=5))
def events():return [json.loads(l) for l in (out/'events.jsonl').read_text().splitlines() if l.endswith('}')]
def sample():return list(csv.DictReader((out/'samples.csv').open()))[-1]
def sizes():return {p.name:p.stat().st_size for p in out.iterdir() if p.is_file()}
def digest(v):return hashlib.sha256(json.dumps(v,sort_keys=True).encode()).hexdigest()
v=state();assert v['turn']==12 and v['turnPlayer']=='Human' and v['prompt']['kind']=='InputPassPriority'
worker=next(e['value'] for e in events() if e['event']=='worker-start');assert isinstance(worker,int)
start=time.monotonic();first=sample();records=[];signature=digest(v)
write(run/'hold-start.json',{'utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'sample':first,'view_sha256':signature,'turn':12,'duration_seconds':600,'files':sizes()})
def probe(second):
    began=time.monotonic();folder=probes/str(second);folder.mkdir();before=sample()
    commands={'cgroup':['cat','/sys/fs/cgroup/memory.stat','/sys/fs/cgroup/memory.events'],
      'worker-status':['cat',f'/proc/{worker}/status'], 'supervisor-status':['cat','/proc/1/status'],
      'processes':['ps','-eo','pid,ppid,rss,nlwp,comm'],
      'worker-nmt':['/opt/java/openjdk/bin/jcmd',str(worker),'VM.native_memory','summary','scale=KB'],
      'worker-heap':['/opt/java/openjdk/bin/jcmd',str(worker),'GC.heap_info'],
      'supervisor-heap':['/opt/java/openjdk/bin/jcmd','1','GC.heap_info']}
    codes={}
    for name,cmd in commands.items():
        result=subprocess.run(['docker','exec',container,*cmd],capture_output=True,text=True,timeout=20)
        (folder/(name+'.txt')).write_text(result.stdout+result.stderr);codes[name]=result.returncode
    write(folder/'probe.json',{'scheduled_seconds':second,'actual_start_seconds':began-start,'duration_seconds':time.monotonic()-began,'return_codes':codes,'before':before,'after':sample()})
try:
    for second in range(0,601,30):
        # Small waits keep interruption and the outer runner's independent deadline effective.
        while time.monotonic()-start<second:time.sleep(min(1,second-(time.monotonic()-start)))
        current=state();assert digest(current)==signature,'Human decision changed during hold'
        row={'scheduled_seconds':second,'elapsed_seconds':time.monotonic()-start,'sample':sample(),'files':sizes()};records.append(row);write(run/'hold-observations.json',records)
        if second in (0,300,600):probe(second)
        print(json.dumps({'hold_seconds':second,'memory_mib':int(row['sample']['memory_current'])/2**20,'view_unchanged':True}),flush=True)
    write(run/'hold-end.json',{'utc':datetime.datetime.now(datetime.timezone.utc).isoformat(),'elapsed_seconds':time.monotonic()-start,'sample':sample(),'view_sha256':digest(state()),'files':sizes(),'completed':True})
finally:
    subprocess.run(['python3',str(Path(__file__).with_name('faults.py')),'cancel','--run',str(run)],check=True,timeout=15)
