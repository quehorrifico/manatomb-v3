#!/usr/bin/env python3
"""Bounded local cgroup observation. No game decisions or hosting access."""
import argparse,json,subprocess,time
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--container',required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--seconds',type=int,default=1800);a=p.parse_args()
assert 1<=a.seconds<=3600
context=json.loads(subprocess.check_output(['docker','context','inspect']))[0]
assert context['Endpoints']['docker']['Host'].startswith('unix://'),'Local Docker only'
assert not a.output.exists(),'Use a fresh evidence path'
a.output.parent.mkdir(parents=True,exist_ok=True)
# Pin this exact container incarnation; a same-name replacement is a new experiment.
container_id=subprocess.check_output(['docker','inspect','--format','{{.Id}}',a.container],text=True).strip()
started=time.monotonic()
with a.output.open('x') as out:
 while time.monotonic()-started<a.seconds:
  command=['docker','exec',container_id,'sh','-c','cat /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.peak /sys/fs/cgroup/memory.stat /sys/fs/cgroup/cpu.stat; ps -eo pid,rss,comm']
  result=subprocess.run(command,capture_output=True,text=True)
  if result.returncode:break
  lines=result.stdout.splitlines();fields={};processes=[];in_processes=False
  for line in lines[2:]:
   values=line.split()
   if values and values[0]=='PID':in_processes=True;continue
   if in_processes and len(values)>=3:processes.append({'pid':int(values[0]),'rss_kib':int(values[1]),'name':values[2]})
   elif len(values)==2:fields[values[0]]=int(values[1])
  out.write(json.dumps({'container_id':container_id,'elapsed_seconds':round(time.monotonic()-started,3),'current_bytes':int(lines[0]),'peak_bytes':int(lines[1]),'components_and_cpu':fields,'processes':processes})+'\n');out.flush()
  time.sleep(5)
print('Observed',a.output,'; docker exec observation overhead remains included in whole-container accounting.')
