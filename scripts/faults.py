#!/usr/bin/env python3
"""Destructive only to this Compose project's processes; never deletes volumes."""
import concurrent.futures, http.client, json, socket, subprocess, threading, time, uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

KEY = 'local-development-key'
PREFIX = 'fault-' + uuid.uuid4().hex[:12]
manifest = []

def post(body, port=8080, timeout=2):
    c = http.client.HTTPConnection('localhost', port, timeout=timeout)
    try:
        c.request('POST', '/transfers', json.dumps(body), {'Authorization': 'Bearer ' + KEY, 'Content-Type': 'application/json'})
        r = c.getresponse()
        data = r.read()
        return r.status, json.loads(data)
    except (OSError, http.client.HTTPException, ValueError) as e:
        return 0, {'error': type(e).__name__}
    finally:
        c.close()

def request(n, **extra):
    r = dict(transaction_id=f'{PREFIX}-{n}', source_id=1, destination_id=2, amount_minor=1)
    r.update(extra)
    manifest.append(r)
    return r

def docker(*args):
    subprocess.run(['docker', 'compose', *args], check=True, stdout=subprocess.DEVNULL)

def ready(port):
    for _ in range(100):
        c = http.client.HTTPConnection('localhost', port, timeout=1)
        try:
            c.request('GET', '/readyz')
            r = c.getresponse(); r.read()
            if r.status == 200: return
        except (OSError, http.client.HTTPException): pass
        finally: c.close()
        time.sleep(.1)
    raise RuntimeError('service not ready')

# Forward to the real API, discard its committed response, and break the client connection.
class LoseResponse(BaseHTTPRequestHandler):
    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers['Content-Length'])))
        status, result = post(body)
        self.server.confirmed = status == 200 and result['decision'] == 'APPROVED'
        self.connection.shutdown(socket.SHUT_RDWR)
        self.connection.close()
    def log_message(self, *args): pass

report = {'run': PREFIX, 'checks': {}}
ready(8080); ready(8081)
proxy = ThreadingHTTPServer(('localhost', 0), LoseResponse)
thread = threading.Thread(target=proxy.serve_forever, daemon=True); thread.start()
r = request('lost-response')
status, _ = post(r, proxy.server_port)
proxy.shutdown(); proxy.server_close(); thread.join()
assert status == 0 and proxy.confirmed
assert post(r, 8081)[1]['decision'] == 'APPROVED'
report['checks']['committed_response_lost_then_replayed'] = True

# Concurrent requests crossing SIGKILL; all unknown results are replayed on the survivor.
requests = [request(f'kill-{i}') for i in range(200)]
with concurrent.futures.ThreadPoolExecutor(max_workers=40) as pool:
    futures = [pool.submit(post, r) for r in requests]
    docker('kill', '-s', 'SIGKILL', 'api')
    initial = [f.result()[0] for f in futures]
for r in requests:
    assert post(r, 8081)[1].get('decision') == 'APPROVED'
docker('up', '-d', 'api'); ready(8080)
report['checks']['application_sigkill_recovered'] = True
report['kill_initial_unknown'] = sum(s != 200 for s in initial)

# A paused database models seconds of unresponsiveness, not a financial decline.
r = request('paused-db')
docker('pause', 'db')
try:
    start = time.monotonic()
    status, result = post(r)
    report['database_pause_response_ms'] = round((time.monotonic()-start)*1000, 2)
    assert status in (0, 503), (status, result)
    time.sleep(2)
finally:
    docker('unpause', 'db')
ready(8080)
assert post(r)[1].get('decision') == 'APPROVED'
report['checks']['database_unresponsive_retry'] = True

# Process outage exercises broken connections and durable recovery.
docker('stop', '-t', '0', 'db')
r = request('stopped-db')
status, _ = post(r)
assert status in (0, 503)
docker('up', '-d', 'db'); ready(8080); ready(8081)
assert post(r)[1].get('decision') == 'APPROVED'
report['checks']['database_process_restart'] = True

# Retry every known request twice and retain its exact payload for independent SQL reconciliation.
for r in manifest:
    for port in (8080,8081):
        status, result = post(r, port)
        assert status == 200 and result['decision'] == 'APPROVED', result
Path('reports').mkdir(exist_ok=True)
Path('reports/fault-manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
report['expected_unique_operations'] = len(manifest)
report['passed'] = all(report['checks'].values())
Path('reports/faults.json').write_text(json.dumps(report, indent=2)+'\n')
print(json.dumps(report, indent=2))
