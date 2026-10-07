#!/usr/bin/env python3
"""Compile/run only the real-Forge projection regression against the retained V2-010 cache."""
import argparse,json,subprocess,hashlib
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
p=argparse.ArgumentParser(description=__doc__)
p.add_argument('--cache',type=Path,default=Path('/Users/zeusborrego/Developer/forge/v2-010'))
p.add_argument('--jdk',type=Path,default=Path('/Library/Java/JavaVirtualMachines/temurin-21.jdk/Contents/Home'))
p.add_argument('--out',type=Path,required=True)
p.add_argument('--test',choices=['projection','ordering'],default='projection')
a=p.parse_args();a.out=a.out.resolve();a.out.mkdir(parents=True,exist_ok=True)
base=json.loads(subprocess.check_output(['python3',str(ROOT/'docs/v2/evidence/V2-010/run.py'),str(a.cache),'sim','--java',str(a.jdk/'bin/java'),'--dry-run'],text=True))
argv=base['argv'];cp=argv[argv.index('-cp')+1];classes=a.out/'classes';classes.mkdir(exist_ok=True)
sources=[ROOT/'tools/forge-browser-spike/src'/f for f in ['BrowserSpike.java','AbilityProjectionRegression.java','OrderingProtocolRegression.java']]
compile=[str(a.jdk/'bin/javac'),'--release','17','-cp',cp,'-d',str(classes),*map(str,sources)]
subprocess.run(compile,check=True)
run=[*argv[:argv.index('-cp')],'-cp',str(classes)+':'+cp,('OrderingProtocolRegression' if a.test=='ordering' else 'AbilityProjectionRegression'),str(a.cache/'runtime/decks/commander/feline.dck'),str(a.cache/'runtime/decks/commander/hostility.dck'),str(a.out)]
(a.out/'command.json').write_text(json.dumps({'compile':compile,'run':run,'cwd':base['cwd'],'sources':{str(f.relative_to(ROOT)):hashlib.sha256(f.read_bytes()).hexdigest() for f in sources}},indent=2))
with (a.out/'local-test.log').open('w') as log:r=subprocess.run(run,cwd=base['cwd'],stdout=log,stderr=subprocess.STDOUT)
print((a.out/'local-test.log').read_text()[-2000:]);raise SystemExit(r.returncode)
