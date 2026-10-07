#!/usr/bin/env python3
"""Local capped AMD64 measurements; never supplies a Forge gameplay choice."""
import argparse,hashlib,json,os,shutil,subprocess,time,fcntl,shlex,xml.etree.ElementTree as ET
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
PIN='26d8aff87509dde8a9d5017852339382d7ab3e83'
DEFAULT_IMAGE='eclipse-temurin@sha256:24a8854594eea72c16822953e6cb96c78d10fc3c77b7b8a60ce8e5ac440a2337'
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('operation',choices=['prepare','run','stop'])
p.add_argument('--cache',type=Path,default=Path('/Users/zeusborrego/Developer/forge/v2-010'))
p.add_argument('--relocate-from',default='/Users/zeusborrego/Developer/forge/v2-010',help='Original absolute root in the retained Maven classpath file')
p.add_argument('--artifacts',type=Path,default=Path('/Users/zeusborrego/Developer/forge/v2-012'))
p.add_argument('--javac',default=shutil.which('javac'))
p.add_argument('--image',default=DEFAULT_IMAGE)
p.add_argument('--timing',action='store_true',help='Measure existing AI chooseSpellAbilityToPlay calls with a local Java agent')
p.add_argument('--memory',type=int,choices=[512,1024,2048],default=512)
p.add_argument('--mode',choices=['human','ai'],default='human')
p.add_argument('--scenario',choices=['realistic','focused','blocking','ordering'],default='realistic')
p.add_argument('--name',default='screen-512')
p.add_argument('--seconds',type=float,default=180)
p.add_argument('--idle-seconds',type=float,help='Cancel this many seconds after readiness; no gameplay answers')
a=p.parse_args();a.artifacts=a.artifacts.resolve();a.cache=a.cache.resolve();a.artifacts.mkdir(parents=True,exist_ok=True)
def call(argv,**kw):return subprocess.run(argv,check=True,text=True,**kw)
def capture(argv):return subprocess.check_output(argv,text=True).strip()
def digest(path):return hashlib.sha256(path.read_bytes()).hexdigest()
def write(path,obj):path.write_text(json.dumps(obj,indent=2)+'\n')
if a.operation=='stop':
    # Only this experiment's labeled containers; preserve all other Docker work.
    ids=capture(['docker','ps','-aq','--filter','label=manatomb.benchmark=V2-012']).split()
    if ids:call(['docker','rm','-f',*ids])
    raise SystemExit()
manifest=json.loads((ROOT/'docs/v2/evidence/V2-010/manifest.json').read_text())
for name,row in manifest['configuration'].items():assert digest(a.cache/name)==row['sha256'],name
desktop=a.cache/f'forge-{PIN}'/'forge-gui-desktop'
retained=(desktop/'target/runtime-classpath.txt').read_text().strip()
if str(a.cache)==a.relocate_from:
    base=json.loads(capture(['python3',str(ROOT/'docs/v2/evidence/V2-010/run.py'),str(a.cache),'sim','--java','java','--dry-run']))
    original=base['argv']
else:
    # Portable equivalent of the retained helper's command construction. Only paths change.
    rebased=':'.join(str(a.cache)+x[len(a.relocate_from):] if x.startswith(a.relocate_from+'/') else x for x in retained.split(':'))
    cp=str(desktop/'target/classes')+':'+rebased
    assert all(Path(x).exists() for x in cp.split(':')),'Rebased cache is incomplete; no automatic rebuild'
    pom=ET.parse(desktop/'pom.xml');opens=shlex.split(pom.find('m:properties/m:addopen.java.args',{'m':'http://maven.apache.org/POM/4.0.0'}).text)
    original=['java','-Djava.awt.headless=true','-Dsentry.enabled=false','-Xmx2g',*opens,'-cp',cp,'forge.view.Main','sim','-d','feline.dck','hostility.dck','-n','1','-f','Commander','-s','20260912','-a','Default','Default','-c','120']
cp=original[original.index('-cp')+1]
stage=a.artifacts/'support';stage.mkdir(exist_ok=True)
if a.operation=='prepare':
    assert a.javac,'Specify --javac for the matching JDK'
    classes=stage/'classes';classes.mkdir(exist_ok=True)
    call([a.javac,'--release','17','-cp',cp,'-d',str(classes),str(ROOT/'tools/forge-benchmark/Supervisor.java'),str(ROOT/'tools/forge-browser-spike/src/BrowserSpike.java')])
    agent=stage/'agent';agent.mkdir(exist_ok=True)
    call([a.javac,'-source','17','-target','17','--add-exports','java.base/jdk.internal.org.objectweb.asm=ALL-UNNAMED','-d',str(agent),str(ROOT/'tools/forge-benchmark/DecisionTimer.java')])
    (agent/'MANIFEST.MF').write_text('Premain-Class: DecisionTimer\n')
    call([str(Path(a.javac).with_name('jar')),'cfm',str(stage/'timer.jar'),str(agent/'MANIFEST.MF'),'-C',str(agent),'.'])
    write(stage/'identity.json',{'adapter_sha256':digest(ROOT/'tools/forge-browser-spike/src/BrowserSpike.java'),'supervisor_sha256':digest(ROOT/'tools/forge-benchmark/Supervisor.java'),'javac':capture([a.javac,'-version']),'timer_sha256':digest(stage/'timer.jar'),'timer_source_sha256':digest(ROOT/'tools/forge-benchmark/DecisionTimer.java'),'files':{str(x.relative_to(classes)):digest(x) for x in classes.rglob('*.class')}})
    print('Prepared Java 17 bytecode; Forge engine classes/resources not rebuilt.');raise SystemExit()
identity=json.loads((stage/'identity.json').read_text())
assert identity['adapter_sha256']==digest(ROOT/'tools/forge-browser-spike/src/BrowserSpike.java')
assert identity['supervisor_sha256']==digest(ROOT/'tools/forge-benchmark/Supervisor.java')
lock=(a.artifacts/'admission.lock').open('a+')
try:fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
except BlockingIOError:p.error('Local benchmark capacity occupied')
assert not capture(['docker','ps','-q','--filter','label=manatomb.benchmark=V2-012']),'Existing experiment container occupies capacity'
run=a.artifacts/a.name
assert not run.exists(),'Use a fresh run name to preserve evidence'
run.mkdir();out=run/'out';out.mkdir();config=run/'config';config.mkdir()
profile='userDir=/runprofile/user\ncacheDir=/runprofile/cache\ndecksDir=/runprofile/decks\n'
(config/'forge.profile.properties').write_text(profile)
runtime=run/'runtime'
for sub in ['user/preferences','cache','decks/commander']:(runtime/sub).mkdir(parents=True,exist_ok=True)
shutil.copyfile(a.cache/'runtime/user/preferences/forge.preferences',runtime/'user/preferences/forge.preferences')
for deck in ['feline.dck','hostility.dck']:shutil.copyfile(a.cache/'runtime/decks/commander'/deck,runtime/'decks/commander'/deck)
entries=cp.split(':');assert all(str(a.cache) in x for x in entries)
linuxcp='/support/classes:'+cp.replace(str(a.cache),'/forge')
argv=['/opt/java/openjdk/bin/java',*original[1:original.index('-cp')],'-XX:ActiveProcessorCount=1','-XX:NativeMemoryTracking=summary','-cp',linuxcp]
argv[argv.index('-Xmx2g')]=f'-Xmx{a.memory//2}m'
if a.timing:argv[1:1]=['--add-exports=java.base/jdk.internal.org.objectweb.asm=ALL-UNNAMED','-javaagent:/support/timer.jar']
if a.mode=='ai':argv+=original[original.index('forge.view.Main'):]
else:
    decks=['/runprofile/decks/commander/feline.dck','/runprofile/decks/commander/hostility.dck'] if a.scenario in ('realistic','ordering') else ['/repo/tools/forge-browser-spike/fixtures/human.dck','/repo/tools/forge-browser-spike/fixtures/'+('blocking-cpu.dck' if a.scenario=='blocking' else 'cpu.dck')]
    argv+=['BrowserSpike',*decks,'18711','/repo/tools/forge-browser-spike/index.html','/out',a.scenario]
(config/'engine.argv').write_text('\n'.join(argv)+'\n')
container='manatomb-v2012-'+a.name
command=['docker','run','-d','--name',container,'--label','manatomb.benchmark=V2-012','--platform','linux/amd64','--memory',f'{a.memory}m','--memory-swap',f'{a.memory}m','--cpus','1','--pids-limit','256','--network','bridge','-p','127.0.0.1:18711:18712','--entrypoint','java']
for host,target,mode in [(a.cache,'/forge','ro'),(ROOT/'tools/forge-browser-spike','/repo/tools/forge-browser-spike','ro'),(stage,'/support','ro'),(out,'/out','rw'),(config,'/config','ro'),(runtime,'/runprofile','rw'),(config/'forge.profile.properties',f'/forge/forge-{PIN}/forge-gui/forge.profile.properties','ro')]:command+=['-v',f'{host}:{target}:{mode}']
command+=[a.image,'-Xms8m','-Xmx32m','-XX:+UseSerialGC','-XX:ActiveProcessorCount=1','-cp',linuxcp,'Supervisor']
write(run/'command.json',{'container_argv':command,'engine_argv':argv,'identity':identity,'memory_mib':a.memory,'mode':a.mode,'scenario':a.scenario,'profile_sha256':digest(config/'forge.profile.properties'),'preferences_sha256':digest(runtime/'user/preferences/forge.preferences')})
started=time.monotonic();cid=capture(command);write(run/'container-initial.json',json.loads(capture(['docker','inspect',cid])))
print(json.dumps({'container':container,'browser':'http://127.0.0.1:18711/','output':str(run)}),flush=True)
reason='deadline';ready=None
try:
    while time.monotonic()-started<a.seconds:
        if (out/'worker-exit.json').exists():reason='worker-exit';break
        if (out/'events.jsonl').exists():
            lines=(out/'events.jsonl').read_text().splitlines()
            events=[]
            for line in lines:
                try:events.append(json.loads(line))
                except json.JSONDecodeError:pass
            if ready is None and any(x['event']=='ready' for x in events):ready=time.monotonic();write(run/'readiness.json',{'host_observed_seconds':ready-started,'sampling_detection_interval_seconds':1});print('Human prompt ready',flush=True)
            if any(x['event']=='view' and x['value'].get('status') in ['blocked','finished'] for x in events):reason='terminal-view';break
            if ready and a.idle_seconds is not None and time.monotonic()-ready>=a.idle_seconds:reason='idle-complete';break
        if json.loads(capture(['docker','inspect',cid]))[0]['State']['Running'] is False:reason='container-exit';break
        time.sleep(1)
except KeyboardInterrupt:reason='interrupted'
finally:
    cleanup=time.monotonic()
    write(run/'container-before-cleanup.json',json.loads(capture(['docker','inspect',cid])))
    call(['docker','stop','-t','5',cid],stdout=subprocess.DEVNULL)
    write(run/'container-final.json',json.loads(capture(['docker','inspect',cid])))
    call(['docker','rm',cid],stdout=subprocess.DEVNULL)
    write(run/'supervision.json',{'reason':reason,'elapsed_seconds':time.monotonic()-started,'cleanup_seconds':time.monotonic()-cleanup,'container_removed':not capture(['docker','ps','-aq','--filter',f'name=^{container}$']),'retained_files':sum(1 for x in run.rglob('*') if x.is_file()),'retained_bytes':sum(x.stat().st_size for x in run.rglob('*') if x.is_file())})
    print(json.dumps(json.loads((run/'supervision.json').read_text())),flush=True)
