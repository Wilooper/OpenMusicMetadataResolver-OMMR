#!/usr/bin/env python3
"""Public-only exact-edition, credit and lossless wire-compression checks."""
import argparse
import gzip
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.parse
import urllib.request

from live_album_coverage import PROVIDERS

CASES = [
    ('boyfriend', '/v1/resolve', {'apple_id': '1829591986', 'sources': 'applemusic'}, (1, 4)),
    ('p-pop-disc2', '/v1/resolve', {'apple_id': '1829591990', 'sources': 'applemusic'}, (2, 1)),
    ('proof-disc3', '/v1/resolve', {'apple_id': '1627577723', 'sources': 'applemusic'}, (3, 1)),
    ('melon-credits', '/v1/resolve', {'melon_id': '39664161', 'sources': 'melon'}, None),
    ('proof-album', '/v1/albums', {'provider': 'applemusic', 'id': '1627573936'}, None),
]


def fetch(url, encoding):
    req = urllib.request.Request(url, headers={'Accept-Encoding': encoding})
    with urllib.request.urlopen(req, timeout=30) as response:
        raw = response.read()
        actual = response.headers.get('Content-Encoding', '')
        decoded = gzip.decompress(raw) if actual == 'gzip' else raw
        return json.loads(decoded), decoded, len(raw), actual


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    binary, out = args.server.resolve(), args.output.resolve()
    out.mkdir(parents=True, exist_ok=True)
    results = []
    with tempfile.TemporaryDirectory(prefix='ommr-production-') as temp:
        root = Path(temp)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        config = root / 'server.yaml'
        config.write_text(
            f'port: {port}\ncredentials_dir: ""\ncache_provider: disk\n'
            f'cache_dir: {root / "cache"}\nprovider_timeout: 20s\n'
            'rate_limit_rps: 1000\nproviders:\n  enabled:\n'
            + ''.join(f'    {p}: {str(p in ["applemusic", "melon"]).lower()}\n' for p in PROVIDERS)
        )
        config.chmod(0o600)
        env = {k: v for k, v in os.environ.items() if not k.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''
        with (out / 'server.log').open('w') as log:
            proc = subprocess.Popen([str(binary), '--config', str(config)], cwd=root,
                                    env=env, stdout=log, stderr=log)
            try:
                base = f'http://127.0.0.1:{port}'
                for _ in range(100):
                    if proc.poll() is not None:
                        raise RuntimeError('server exited; inspect server.log')
                    try:
                        fetch(base + '/v1/health', 'identity')
                        break
                    except OSError:
                        time.sleep(.1)
                else:
                    raise RuntimeError('server startup timed out')
                for label, path, params, expected in CASES:
                    row = {'case': label}
                    try:
                        url = base + path + '?' + urllib.parse.urlencode(params)
                        body, plain, plain_size, _ = fetch(url, 'identity')
                        (out / (label + '.json')).write_text(json.dumps(body, ensure_ascii=False, indent=2) + '\n')
                        assert body.get('track') or body.get('album'), 'upstream returned no resolved entity'
                        compressed, decoded, wire_size, encoding = fetch(url, 'gzip')
                        if plain != decoded or body != compressed:
                            (out / (label + '-gzip.json')).write_text(json.dumps(compressed, ensure_ascii=False, indent=2) + '\n')
                            raise AssertionError('metadata changed on wire')
                        assert encoding == 'gzip' and wire_size < plain_size, 'compression missing'
                        entity = body.get('track') or body.get('album')
                        row.update(title=entity['title'], plain_bytes=plain_size,
                                   gzip_bytes=wire_size, reduction_percent=round(100 * (1 - wire_size / plain_size), 2),
                                   lossless=True)
                        if expected:
                            assert (entity.get('disc_number'), entity.get('track_number')) == expected, 'wrong position'
                            selected = [p for p in entity['album_placements'] if p.get('selected')]
                            assert len(selected) == 1, 'no unique selected placement'
                            assert selected[0]['track_id'] == params['apple_id'], 'wrong placed track'
                            assert selected[0]['album_id'] == entity['album']['ids']['applemusic'], 'wrong placed edition'
                            row.update(disc_number=entity['disc_number'], track_number=entity['track_number'],
                                       placement=selected[0])
                        if label == 'melon-credits':
                            composers = [c for c in entity.get('credits', []) if 'composer' in c['roles']]
                            assert len(composers) >= 3, 'expected live composer credits missing'
                            details = entity['credit_details']
                            subjects = {n['id']: n['type'] for n in entity['music_graph']['nodes']}
                            assert all(subjects[c['to']] == 'recording' for c in details['credits']), 'wrong credit scope'
                            row['composers'] = [c['name'] for c in composers]
                            row['source_roles'] = sorted({c.get('source_role') for c in details['credits']})
                        row['passed'] = True
                    except Exception as error:
                        row.update(passed=False, error=str(error))
                    results.append(row)
                    print(json.dumps(row, ensure_ascii=False), flush=True)
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()
    (out / 'live-summary.json').write_text(json.dumps(results, ensure_ascii=False, indent=2) + '\n')
    return 0 if all(r['passed'] for r in results) else 1


if __name__ == '__main__':
    raise SystemExit(main())
