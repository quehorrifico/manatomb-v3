#!/usr/bin/env python3
"""Assemble retained pinned Forge runtime; compile adapter only, never rebuild Forge."""
import argparse,hashlib,json,shutil,subprocess,xml.etree.ElementTree as ET,shlex
from extract_combat import extract
from pathlib import Path
P=argparse.ArgumentParser();P.add_argument('--cache',type=Path,required=True);P.add_argument('--output',type=Path,required=True);P.add_argument('--javac',required=True);a=P.parse_args()
root=Path(__file__).resolve().parents[2];pin='26d8aff87509dde8a9d5017852339382d7ab3e83';source=a.cache/f'forge-{pin}';out=a.output.resolve();out.mkdir(parents=True,exist_ok=True)
manifest=json.loads((root/'docs/v2/evidence/V2-010/manifest.json').read_text())
for name,row in manifest['configuration'].items():
 # The old desktop probe profile contains absolute paths and is not shipped/used.
 if name.endswith('forge.profile.properties'):continue
 assert hashlib.sha256((a.cache/name).read_bytes()).hexdigest()==row['sha256'],name
def tree_digest(directory):
 files=sorted(p for p in directory.rglob('*') if p.is_file())
 return hashlib.sha256(''.join(hashlib.sha256(p.read_bytes()).hexdigest()+'  '+p.relative_to(directory).as_posix()+'\n' for p in files).encode()).hexdigest()
for name,row in manifest['compiled_modules'].items():assert tree_digest(source/name/'target/classes')==row['sha256_path_manifest'],name
assert tree_digest(source/'forge-gui/res')==manifest['resource_trees']['forge-gui/res']['sha256_path_manifest'],'resource tree changed'
for row in manifest['runtime_dependencies']:assert hashlib.sha256((a.cache/'m2'/row['maven_path']).read_bytes()).hexdigest()==row['sha256'],row['maven_path']
# Construct from pinned manifest inputs, never the build machine's absolute classpath.
cp=[source/name/'target/classes' for name in ['forge-gui-desktop','forge-core','forge-game','forge-ai','forge-gui']]
cp += [a.cache/'m2'/row['maven_path'] for row in manifest['runtime_dependencies']]
if (out/'lib').exists():shutil.rmtree(out/'lib')
(out/'lib').mkdir()
# Adapter inner classes can disappear between revisions; never ship stale bytecode.
if (out/'classes').exists():shutil.rmtree(out/'classes')
(out/'classes').mkdir()
for i,path in enumerate(cp):
 dest=out/'lib'/str(i)
 if path.is_dir():shutil.copytree(path,dest,dirs_exist_ok=True)
 else:shutil.copy2(path,dest.with_suffix('.jar'))
classpath=':'.join(str(x) for x in cp)
(out/'generated-source').mkdir(exist_ok=True)
combat=out/'generated-source/PinnedCombatAssignment.java'
(out/'combat-extraction.json').write_text(json.dumps(extract(source,combat),indent=2)+'\n')
subprocess.run([a.javac,'--release','17','-cp',classpath,'-d',str(out/'classes'),str(root/'services/forge/src/ForgeWorker.java'),str(combat)],check=True)
if (out/'res').exists():shutil.rmtree(out/'res')
shutil.copytree(source/'forge-gui/res',out/'res')
shutil.copy2(a.cache/'runtime/user/preferences/forge.preferences',out/'forge.preferences')
for src,dest in [(root/'services/forge/defaults',out/'defaults'),(root/'services/forge/src',out/'adapter-source')]:
 if dest.exists():shutil.rmtree(dest)
 shutil.copytree(src,dest)
shutil.copy2(root/'docs/v2/evidence/V2-010/manifest.json',out/'upstream-manifest.json')
(out/'source').mkdir(exist_ok=True)
archive=a.cache/'forge-source.tar.gz'
assert hashlib.sha256(archive.read_bytes()).hexdigest()==manifest['archives']['forge-source.tar.gz']['sha256'],'source archive changed'
shutil.copy2(archive,out/'source/forge-source.tar.gz')
for path in ['services/forge','internal/forge','cmd/forge-service']:
 dest=out/'source'/path
 if dest.exists():shutil.rmtree(dest)
 shutil.copytree(root/path,dest,ignore=shutil.ignore_patterns('__pycache__'))
for name in ['go.mod','go.sum']:shutil.copy2(root/name,out/'source'/name)
shutil.copy2(root/'docs/v2/evidence/V2-010.md',out/'source/BUILD-BASELINE.md')
for name in ['LICENSE','LICENSE.txt','README.txt','CHANGES.txt']:
 if (source/name).exists():shutil.copy2(source/name,out/name)
opens=shlex.split(ET.parse(source/'forge-gui-desktop/pom.xml').find('m:properties/m:addopen.java.args',{'m':'http://maven.apache.org/POM/4.0.0'}).text)
(out/'jvm-opens').write_text('\n'.join(opens)+'\n')
(out/'identity.json').write_text(json.dumps({'forge':pin,'compiler':subprocess.check_output([a.javac,'-version'],text=True).strip(),'files':{str(p.relative_to(out)):hashlib.sha256(p.read_bytes()).hexdigest() for p in sorted(out.rglob('*')) if p.is_file() and p.name!='identity.json'}},indent=2)+'\n')
print(out)
