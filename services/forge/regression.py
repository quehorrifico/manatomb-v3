#!/usr/bin/env python3
"""Focused engine-backed adapter checks; no claims of browser/game completion."""
import argparse,os,subprocess,shutil,json,hashlib
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--bundle',type=Path,required=True);p.add_argument('--java-home',type=Path,required=True);p.add_argument('--cache',type=Path,required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--test',choices=['WorkerRegression','OrderingRegression','ConcedeRegression','QueuedConcedeRegression','ChoiceRegression','ControlsRegression','DamageTriggerRegression','ActionShortcutRegression'],default='WorkerRegression');a=p.parse_args();a.output.mkdir(parents=True,exist_ok=True)
assets=a.output/'assets';assets.mkdir(exist_ok=True);res=assets/'res'
if not res.exists():res.symlink_to(a.bundle/'res',target_is_directory=True)
for x in ['user/preferences','cache','decks']:(a.output/x).mkdir(parents=True,exist_ok=True)
(assets/'forge.profile.properties').write_text(''.join(f'{k}={a.output/v}\n' for k,v in [('userDir','user'),('cacheDir','cache'),('decksDir','decks')]))
shutil.copy2(a.bundle/'forge.preferences',a.output/'user/preferences/forge.preferences')
cp=':'.join(map(str,[a.bundle/'classes',*sorted((a.bundle/'lib').iterdir())]))
sources=[Path(__file__).parent/'src/ForgeWorker.java',a.bundle/'generated-source/PinnedCombatAssignment.java',Path(__file__).parent/'src/WorkerRegression.java',Path(__file__).parent/'src/OrderingRegression.java',Path(__file__).parent/'src/ConcedeRegression.java',Path(__file__).parent/'src/QueuedConcedeRegression.java',Path(__file__).parent/'src/ChoiceRegression.java',Path(__file__).parent/'src/ControlsRegression.java',Path(__file__).parent/'src/DamageTriggerRegression.java',Path(__file__).parent/'src/ActionShortcutRegression.java']
subprocess.run([str(a.java_home/'bin/javac'),'--release','17','-cp',cp,'-d',str(a.output),*map(str,sources)],check=True)
(a.output/'identity.json').write_text(json.dumps({'sources':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in sources},'runtime_bundle_identity_sha256':hashlib.sha256((a.bundle/'identity.json').read_bytes()).hexdigest()},indent=2)+'\n')
env=os.environ.copy();env['FORGE_WORKER_TOKEN']='local-regression-token';cmd=[str(a.java_home/'bin/java'),'-Djava.awt.headless=true',f'-Dmanatomb.assets={assets}','-Dsentry.enabled=false','-Xmx1024m',*(a.bundle/'jvm-opens').read_text().split(),'-cp',str(a.output)+':'+cp,a.test,str(a.cache/'runtime/decks/commander/feline.dck'),str(a.cache/'runtime/decks/commander/hostility.dck'),str(a.output)]
with (a.output/'regression.log').open('w') as log:r=subprocess.run(cmd,env=env,stdout=log,stderr=subprocess.STDOUT,timeout=120)
print('PASS' if r.returncode==0 else 'FAIL',a.output/'regression.log');raise SystemExit(r.returncode)
