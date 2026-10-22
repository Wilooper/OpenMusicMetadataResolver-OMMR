#!/usr/bin/env python3
"""Verify public Apple disc/collection coverage and REST/gateway cache parity."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request

PROVIDERS = [
    'spotify', 'ytmusic', 'applemusic', 'deezer', 'jiosaavn', 'musicbrainz',
    'soundcloud', 'qobuz', 'tidal', 'amazonmusic', 'pandora', 'bugs', 'genie', 'melon',
]
CASES = [
    ('damn', '1440881722', 1000),
    ('goldberg', '594521223', 1000),
    ('p-pop', '1829591921', 1000),
    ('proof', '1627573936', 1000),
    ('p-pop-limited', '1829591921', 2),
]


def request(url, payload=None):
    data = json.dumps(payload).encode() if payload is not None else None
    req = urllib.request.Request(url, data=data, headers={'Content-Type': 'application/json'})
    with urllib.request.urlopen(req, timeout=25) as response:
        return json.load(response)


def verify(base, out, label, album_id, limit):
    begin = time.monotonic()
    path = '/v1/albums?' + urllib.parse.urlencode({
        'provider': 'applemusic', 'id': album_id, 'limit': limit, 'debug': 'true',
    })
    body = request(base + path)
    (out / (label + '.json')).write_text(json.dumps(body, ensure_ascii=False, indent=2) + '\n')
    album = body['album']
    discs = [{'number': d['number'], 'coverage': d.get('track_list')} for d in album['discs']]
    assert album['id'] == album_id and len(album['tracks']) <= limit, 'identity/limit'
    assert album['track_list']['returned'] == len(album['tracks']), 'collection returned'
    assert all(d['coverage'] is not None for d in discs), 'missing disc coverage'
    assert sum(d['coverage']['returned'] for d in discs) == len(album['tracks']), 'disc returned'
    if label in ['damn', 'goldberg']:
        assert not album['track_list']['complete'], 'collection unexpectedly complete'
        assert all(d['coverage']['complete'] for d in discs), 'song disc incomplete'
    if label in ['p-pop', 'proof']:
        assert album['track_list']['complete'], 'collection incomplete'
        assert len(discs) == (2 if label == 'p-pop' else 3), 'disc count'
        assert all(d['coverage']['complete'] for d in discs), 'disc incomplete'
    if label == 'p-pop-limited':
        assert album['track_list']['reason'] == 'limit', 'collection limit'
        assert all(not d['coverage']['complete'] and d['coverage']['reason'] == 'limit'
                   for d in discs), 'disc limit'
    cached = request(base + '/v1/query', {
        'operation': 'albums', 'provider': 'applemusic', 'id': album_id,
        'limit': limit, 'debug': True,
    })
    assert cached['album'] == album and cached['source']['cached'], 'REST/gateway cache parity'
    return {
        'case': label, 'id': album_id, 'title': album['title'],
        'seconds': round(time.monotonic() - begin, 3), 'collection': album['track_list'],
        'discs': discs, 'rest_gateway_cache_equal': True, 'passed': True,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    binary = args.server.resolve()
    out = args.output.resolve()
    out.mkdir(parents=True, exist_ok=True)
    results = []
    with tempfile.TemporaryDirectory(prefix='ommr-coverage-') as temp:
        root = Path(temp)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        config = root / 'server.yaml'
        config.write_text(
            f'port: {port}\ncredentials_dir: ""\ncache_provider: disk\n'
            f'cache_dir: {root / "cache"}\nprovider_timeout: 20s\n'
            'rate_limit_rps: 1000\nproviders:\n  enabled:\n'
            + ''.join(f'    {p}: {str(p == "applemusic").lower()}\n' for p in PROVIDERS)
        )
        config.chmod(0o600)
        env = {k: v for k, v in os.environ.items() if not k.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''
        with (out / 'live-server.log').open('w') as log:
            proc = subprocess.Popen([str(binary), '--config', str(config)], cwd=root,
                                    env=env, stdout=log, stderr=log)
            try:
                base = f'http://127.0.0.1:{port}'
                for _ in range(100):
                    if proc.poll() is not None:
                        raise RuntimeError('server exited; inspect live-server.log')
                    try:
                        request(base + '/v1/health')
                        break
                    except OSError:
                        time.sleep(.1)
                else:
                    raise RuntimeError('server startup timed out')
                for label, album_id, limit in CASES:
                    try:
                        row = verify(base, out, label, album_id, limit)
                    except Exception as error:
                        row = {'case': label, 'passed': False, 'error': str(error)}
                    results.append(row)
                    print(json.dumps(row), flush=True)
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()
    (out / 'live-summary.json').write_text(json.dumps(results, ensure_ascii=False, indent=2) + '\n')
    return 0 if all(row['passed'] for row in results) else 1


if __name__ == '__main__':
    raise SystemExit(main())
