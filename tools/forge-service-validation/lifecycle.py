#!/usr/bin/env python3
"""Bounded local supervisor fault tests. No browser/gameplay coverage claim."""
import argparse, concurrent.futures, http.client, json, subprocess, time, urllib.error, urllib.parse, urllib.request, uuid
from pathlib import Path

p = argparse.ArgumentParser()
p.add_argument('--url', default='http://127.0.0.1:18881')
p.add_argument('--container', required=True)
p.add_argument('--secret-file', type=Path, required=True)
p.add_argument('--output', type=Path, required=True)
a = p.parse_args()
u = urllib.parse.urlsplit(a.url)
assert u.scheme == 'http' and u.hostname in ('127.0.0.1', 'localhost') and not u.path
assert a.container.startswith('manatomb-') and not a.output.exists()
context = json.loads(subprocess.check_output(['docker', 'context', 'inspect']))[0]
assert context['Endpoints']['docker']['Host'].startswith('unix://'), 'Local Docker only'
secret = dict(line.split('=', 1) for line in a.secret_file.read_text().splitlines() if '=' in line)['FORGE_SERVICE_SECRET']
traffic = {'requests': 0, 'sent_body_bytes': 0, 'received_body_bytes': 0}
def request(method, path, body=None, owner='10001', expect=200):
    raw = None if body is None else json.dumps(body).encode()
    headers = {'Authorization': 'Bearer '+secret, 'X-ManaTomb-Owner': owner, 'Content-Type': 'application/json'}
    r = urllib.request.Request(a.url+'/v1'+path, data=raw, headers=headers, method=method)
    try: response = urllib.request.urlopen(r, timeout=15)
    except urllib.error.HTTPError as error: response = error
    data = response.read(); traffic['requests'] += 1; traffic['sent_body_bytes'] += len(raw or b''); traffic['received_body_bytes'] += len(data)
    assert response.status == expect, (method, path, response.status, data[:200])
    return json.loads(data)
def docker(*args):
    return subprocess.check_output(['docker', *args], text=True).strip()
def workers():
    return [int(row.split()[0]) for row in docker('exec', a.container, 'ps', '-eo', 'pid,comm').splitlines()[1:] if row.split()[1] == 'java']
def ready(sid):
    start = time.monotonic()
    while time.monotonic()-start < 180:
        v = request('GET', '/sessions/'+sid)
        assert v['status'] not in ('failed', 'expired'), v.get('error')
        if v.get('state', {}).get('prompt'): return v, round(time.monotonic()-start, 3)
        time.sleep(1)
    raise AssertionError('Worker readiness deadline')
def clean():
    start = time.monotonic()
    while time.monotonic()-start < 10:
        if not workers() and not docker('exec', a.container, 'ls', '-A', '/tmp/manatomb-games'): return round(time.monotonic()-start, 3)
        time.sleep(.2)
    raise AssertionError('Worker or session files remain')

report = {'kind': 'local API lifecycle/fault tests, not browser gameplay', 'cycles': [], 'traffic': traffic, 'passed': False}
a.output.parent.mkdir(parents=True, exist_ok=True)
previous = None
sid = None
try:
    for index, mode in enumerate(['startup-cancel', 'input-cancel', 'duplicate-create', 'forced-death', 'stopped-worker-cancel', 'lost-ack', 'service-restart', 'input-cancel', 'input-cancel', 'input-cancel']):
        started = time.monotonic()
        body = {'request': str(uuid.uuid4()), 'human': {'name': 'Lifecycle fixture', 'commanders': [{'name': 'Silvos, Rogue Elemental', 'quantity': 1}], 'main': [{'name': 'Forest', 'quantity': 99}]}, 'default': 'feline-ferocity', 'cpu': {'name': '', 'commanders': [], 'main': []}}
        v = request('POST', '/sessions', body); sid = v['id']; row = {'mode': mode, 'index': index, 'readiness_seconds': None}
        if previous:
            old_id, old_action = previous
            request('POST', '/sessions/'+old_id+'/actions', old_action, expect=404 if index == 7 else 410)
        request('GET', '/sessions/'+sid, owner='10002', expect=404)
        request('POST', '/sessions', {**body, 'request': str(uuid.uuid4())}, owner='10002', expect=409)
        if mode == 'duplicate-create':
            with concurrent.futures.ThreadPoolExecutor(max_workers=2) as pool:
                results = list(pool.map(lambda _: request('POST', '/sessions', body), range(2)))
            assert all(item['id'] == sid for item in results)
        action = {'request': str(uuid.uuid4()), 'session': 'old-worker', 'revision': 1, 'prompt': 'old-prompt', 'action': 'ok'}
        if mode != 'startup-cancel':
            v, row['readiness_seconds'] = ready(sid)
            s = v['state']; action.update(session=s['session'], revision=s['revision'], prompt=s['prompt']['id'])
            assert len(workers()) == 1
        if mode == 'lost-ack':
            s = v['state']; assert s['prompt']['kind'] == 'reveal' and s['prompt']['max'] == 0
            action.update(action='reply', selected=[])
            # Deliberately close before reading the reply; retry uses the SAME ID/body.
            conn = http.client.HTTPConnection(u.hostname, u.port, timeout=10)
            conn.request('POST', '/v1/sessions/'+sid+'/actions', body=json.dumps(action), headers={'Authorization':'Bearer '+secret, 'X-ManaTomb-Owner':'10001', 'Content-Type':'application/json'})
            conn.close(); time.sleep(.5)
            assert request('POST', '/sessions/'+sid+'/actions', action).get('accepted') is True
            changed = {**action, 'selected': [99]}; request('POST', '/sessions/'+sid+'/actions', changed, expect=409)
        cleanup_start = time.monotonic()
        if mode == 'forced-death':
            docker('exec', a.container, 'kill', '-KILL', str(workers()[0]))
            deadline = time.monotonic()+10
            while time.monotonic()<deadline:
                if request('GET', '/sessions/'+sid)['status'] == 'failed': break
                time.sleep(.2)
            else: raise AssertionError('Forced death not reported')
        elif mode == 'service-restart':
            docker('restart', '-t', '10', a.container)
            request('GET', '/sessions/'+sid, expect=404)
        else:
            if mode == 'stopped-worker-cancel': docker('exec', a.container, 'kill', '-STOP', str(workers()[0]))
            assert request('DELETE', '/sessions/'+sid)['status'] == 'cancelled'
        clean(); row['cleanup_seconds'] = round(time.monotonic()-cleanup_start, 3)
        # A fresh request, not an idempotent old receipt, must be rejected after exit.
        previous = (sid, {**action, 'request': str(uuid.uuid4())})
        row['elapsed_seconds'] = round(time.monotonic()-started, 3); row['no_worker_or_session_files'] = True
        report['cycles'].append(row); a.output.write_text(json.dumps(report, indent=2)+'\n')
        print(index, mode, 'PASS', flush=True)
    report['passed'] = True
finally:
    if not report['passed'] and sid:
        try:
            request('DELETE', '/sessions/'+sid)
            clean()
            report['failed_run_cleanup'] = 'reaped'
        except Exception as error:
            report['failed_run_cleanup'] = type(error).__name__
    # Preserve completed evidence even when an assertion exposes a real defect.
    a.output.write_text(json.dumps(report, indent=2)+'\n')
