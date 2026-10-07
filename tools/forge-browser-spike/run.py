#!/usr/bin/env python3
"""Compile only the disposable adapter, using the retained V2-010 runtime."""
from pathlib import Path
import argparse,json,subprocess,hashlib
root=Path(__file__).resolve().parents[2]
parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--work',type=Path,default=Path('/Users/zeusborrego/Developer/forge/v2-010'))
parser.add_argument('--output',type=Path,default=Path('/Users/zeusborrego/Developer/forge/v2-011/realistic'))
parser.add_argument('--scenario',choices=('realistic','focused','blocking'),default='realistic')
parser.add_argument('--port',type=int,default=18711)
args=parser.parse_args()
java='/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home/bin/java'
invocation=json.loads(subprocess.check_output(['python3',str(root/'docs/v2/evidence/V2-010/run.py'),str(args.work),'sim','--java',java,'--dry-run'],text=True))
manifest=json.loads((root/'docs/v2/evidence/V2-010/manifest.json').read_text())
for name,record in manifest['configuration'].items():
    assert hashlib.sha256((args.work/name).read_bytes()).hexdigest()==record['sha256'],f'V2-010 configuration changed: {name}'
command=invocation['argv'][:invocation['argv'].index('forge.view.Main')]
classpath=command[command.index('-cp')+1]
args.output.mkdir(parents=True,exist_ok=True)
classes=args.output/'classes';classes.mkdir(exist_ok=True)
subprocess.run([str(Path(java).with_name('javac')),'--release','17','-cp',classpath,'-d',str(classes),str(Path(__file__).with_name('src')/'BrowserSpike.java')],check=True)
command[command.index('-cp')+1]=str(classes)+':'+classpath
decks=[args.work/'runtime/decks/commander/feline.dck',args.work/'runtime/decks/commander/hostility.dck'] if args.scenario=='realistic' else [Path(__file__).with_name('fixtures')/'human.dck',Path(__file__).with_name('fixtures')/('blocking-cpu.dck' if args.scenario=='blocking' else 'cpu.dck')]
command+=['BrowserSpike',str(decks[0]),str(decks[1]),str(args.port),str(Path(__file__).with_name('index.html')),str(args.output),args.scenario]
(args.output/'command.json').write_text(json.dumps({'cwd':invocation['cwd'],'argv':command,'adapter_sha256':hashlib.sha256((Path(__file__).with_name('src')/'BrowserSpike.java').read_bytes()).hexdigest(),'decks':{str(p):hashlib.sha256(p.read_bytes()).hexdigest() for p in decks}},indent=2)+'\n')
print(f'Local harness: http://127.0.0.1:{args.port}/',flush=True)
try: subprocess.run(command,cwd=invocation['cwd'],check=True)
except KeyboardInterrupt: pass
except subprocess.CalledProcessError as error: raise SystemExit(error.returncode)
