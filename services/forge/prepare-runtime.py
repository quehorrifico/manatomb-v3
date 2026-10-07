#!/usr/bin/env python3
"""Prepare the exact Forge cache layout from public pinned sources, once.

No engine changes. Run package.py afterwards; it verifies all compiled modules,
resources and 89 runtime dependencies against the retained baseline manifest.
"""
import argparse, hashlib, os, shutil, subprocess, tarfile, urllib.request
from pathlib import Path

PIN = '26d8aff87509dde8a9d5017852339382d7ab3e83'
p = argparse.ArgumentParser(description=__doc__)
p.add_argument('--cache', type=Path, required=True, help='New dedicated cache directory')
p.add_argument('--java-home', type=Path, required=True, help='Temurin 21.0.6+7 JDK directory')
p.add_argument('--plan', action='store_true', help='Show operations without downloading/building')
a = p.parse_args()
cache = a.cache.resolve(); jdk = a.java_home.resolve()
archives = [
 ('forge-source.tar.gz', f'https://codeload.github.com/Card-Forge/forge/tar.gz/{PIN}', 'b82f437c3933c96e473fcd4b090e0cc7542c78352a391b0cf23aaee5188c804e'),
 ('apache-maven-3.9.9-bin.tar.gz', 'https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/3.9.9/apache-maven-3.9.9-bin.tar.gz', '7a9cdf674fc1703d6382f5f330b3d110ea1b512b51f1652846d9e4e8a588d766'),
]
source = cache / ('forge-' + PIN)
command = [str(cache/'apache-maven-3.9.9/bin/mvn'), '-B', '-ntp', f'-Dmaven.repo.local={cache / "m2"}', '-pl', 'forge-gui-desktop', '-am', '-DskipTests', '-Dcheckstyle.skip=true', 'compile', 'org.apache.maven.plugins:maven-dependency-plugin:3.8.1:build-classpath', '-Dmdep.outputFile=target/runtime-classpath.txt', '-DincludeScope=runtime']
if a.plan:
 print('Requires Python 3.12+, network to public source/Maven repositories, and Temurin 21.0.6+7 JDK.')
 for name, url, digest in archives: print(name, url, 'SHA256='+digest)
 print('Build once in', source, '\n', command)
 print('Initialize exact preferences and two baseline deck files; then run package.py.')
 raise SystemExit(0)
assert not cache.exists() or not any(cache.iterdir()), 'Use an empty dedicated cache; never overwrite a verified cache'
version = subprocess.check_output([str(jdk/'bin/java'), '-version'], stderr=subprocess.STDOUT, text=True)
assert '21.0.6' in version and 'Temurin-21.0.6+7' in version, 'Use pinned Temurin 21.0.6+7, not an arbitrary current JDK'
cache.mkdir(parents=True, exist_ok=True)
for name, url, digest in archives:
 path = cache/name
 with urllib.request.urlopen(url, timeout=60) as response, path.open('wb') as output: shutil.copyfileobj(response, output)
 assert hashlib.sha256(path.read_bytes()).hexdigest() == digest, f'Archive identity mismatch: {name}'
 with tarfile.open(path) as archive: archive.extractall(cache, filter='data')
env = os.environ.copy(); env['JAVA_HOME'] = str(jdk); env['PATH'] = str(jdk/'bin') + os.pathsep + env.get('PATH', '')
with (cache/'build.log').open('w') as log:
 subprocess.run(command, cwd=source, env=env, stdout=log, stderr=subprocess.STDOUT, timeout=1800, check=True)
prefs = cache/'runtime/user/preferences/forge.preferences'; prefs.parent.mkdir(parents=True)
prefs.write_text('LOAD_CARD_SCRIPTS_LAZILY=true\nDECKGEN_CARDBASED=false\nMULLIGAN_RULE=London\nUI_ENABLE_SOUNDS=false\nUI_ENABLE_MUSIC=false\n')
decks = cache/'runtime/decks/commander'; decks.mkdir(parents=True)
for original, dest in [('Feline Ferocity.dck','feline.dck'), ('Open Hostility.dck','hostility.dck')]: shutil.copy2(source/'forge-gui/res/quest/precons'/original, decks/dest)
print('Prepared', cache, '; package.py must verify all identities before use.')
