#!/usr/bin/env python3
"""Restart only the named disposable packaged validation services on local Docker.

Preserves the existing local test database and network. Never terminates a live game.
The optional fixture launcher is a read-only local mount, absent from production.
"""
import argparse, json, subprocess
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--env-dir', type=Path, required=True)
p.add_argument('--fixture', choices=['focused', 'blocking', 'ordering', 'choices'])
p.add_argument('--web', action='store_true', help='Also recreate packaged web using its existing local env file')
a = p.parse_args()
def run(*args):
    return subprocess.check_output(['docker', *args], text=True).strip()
context = json.loads(run('context', 'inspect'))[0]
assert context['Endpoints']['docker']['Host'].startswith('unix://'), 'Local Docker only'
network = 'manatomb-v2-release-local'
run('network', 'inspect', network)
engine = 'manatomb-forge-release-local'
exists = run('ps', '-aq', '--filter', 'name=^'+engine+'$')
if exists:
    info = json.loads(run('inspect', engine))[0]
    if info['State']['Running']:
        assert not any(row.split()[-1] == 'java' for row in run('exec', engine, 'ps', '-eo', 'comm').splitlines()), 'End the active local game first'
    run('stop', '-t', '10', engine)
    run('rm', engine)
args = ['run', '-d', '--name', engine, '--platform', 'linux/amd64', '--network', network,
        '--network-alias', 'forge', '--memory', '2g', '--memory-swap', '2g', '--cpus', '1',
        '--pids-limit', '256', '--read-only', '--tmpfs', '/tmp:rw,noexec,nosuid,size=128m,mode=1777',
        '--env-file', str(a.env_dir/'local-engine.env'), '-p', '127.0.0.1:18881:8081']
if a.fixture:
    launcher = Path(__file__).resolve().with_name('fixture-launch')
    args += ['--mount', f'type=bind,src={launcher},dst=/opt/test-fixture-launch,readonly',
             '-e', 'FORGE_LAUNCHER=/opt/test-fixture-launch', '-e', 'MANATOMB_TEST_FIXTURE='+a.fixture]
run(*args, 'manatomb-forge:local')
if a.web:
    web = 'manatomb-web-release-local'
    if run('ps', '-aq', '--filter', 'name=^'+web+'$'):
        run('stop', '-t', '10', web)
        run('rm', web)
    run('run', '-d', '--name', web, '--platform', 'linux/amd64', '--network', network,
        '--memory', '512m', '--memory-swap', '512m', '--cpus', '1', '--read-only',
        '--tmpfs', '/tmp:rw,noexec,nosuid,size=32m,mode=1777', '--env-file', str(a.env_dir/'local-web.env'),
        '-p', '127.0.0.1:18880:8080', 'manatomb-web:v2-local')
print('Local packaged services ready; fixture:', a.fixture or 'none (normal Forge shuffle)')
