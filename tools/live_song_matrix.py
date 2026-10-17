#!/usr/bin/env python3
"""Run an isolated credential-free live catalog smoke test (Python standard library)."""
import argparse
import json
import os
from pathlib import Path
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

SONGS = [
    ('Karan Aujla', 'Boyfriend'), ('Talwiinder', 'Tu'),
    ('Daft Punk', 'Get Lucky'), ('The Weeknd', 'Blinding Lights'),
    ('Adele', 'Hello'), ('BTS', 'Dynamite'),
    ('Queen', 'Bohemian Rhapsody'), ('Arijit Singh', 'Tum Hi Ho'),
    ('Luis Fonsi', 'Despacito'), ('YOASOBI', 'Idol'),
    ('Taylor Swift', 'Anti-Hero'), ('Billie Eilish', 'bad guy'),
]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    binary = args.server.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix='ommr-live-') as directory:
        root = Path(directory)
        with socket.socket() as listener:
            listener.bind(('127.0.0.1', 0))
            port = listener.getsockname()[1]
        config = root / 'test.yaml'
        config.write_text(f'port: {port}\ncredentials_dir: ""\ncache_dir: {root / "cache"}\nprovider_timeout: 10s\napple_storefront: us\n')
        config.chmod(0o600)
        env = {key: value for key, value in os.environ.items() if not key.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''
        base = f'http://127.0.0.1:{port}'

        def request(path):
            start = time.monotonic()
            try:
                with urllib.request.urlopen(base + path, timeout=35) as response:
                    raw, status = response.read(), response.status
            except urllib.error.HTTPError as error:
                raw, status = error.read(), error.code
            return status, json.loads(raw), len(raw), round(time.monotonic() - start, 3)

        with (root / 'server.log').open('w') as log:
            process = subprocess.Popen([str(binary), '--config', str(config)], cwd=root, env=env, stdout=log, stderr=log)
            try:
                for _ in range(200):
                    if process.poll() is not None:
                        raise RuntimeError('isolated server exited before becoming ready')
                    try:
                        request('/v1/health')
                        break
                    except (OSError, ValueError):
                        time.sleep(0.1)
                else:
                    raise RuntimeError('isolated server did not become ready')
                startup = request('/v1/providers')[1]
                (args.output / 'providers-startup.json').write_text(json.dumps(startup, indent=2) + '\n')
                summaries = []
                for index, (artist, title) in enumerate(SONGS, 1):
                    path = '/v1/resolve?' + urllib.parse.urlencode({'artist': artist, 'title': title})
                    status, debug, size, elapsed = request(path + '&debug=true')
                    public_status, public, public_size, warm_elapsed = request(path)
                    track, public_track = debug.get('track') or {}, public.get('track') or {}
                    assert 'provider_status' not in public, 'production leaked provider diagnostics'
                    assert 'metadata_comparison' not in public_track.get('extensions', {}), 'production leaked comparison'
                    # Expiring partial results may be re-resolved; equality is checked against IDs.
                    same_record = track.get('ids') == public_track.get('ids')
                    debug_data = track.get('extensions', {}).get('provider_data', {})
                    public_data = public_track.get('provider_data', {})
                    row = {'query_artist': artist, 'query_title': title, 'http_status': status,
                           'public_http_status': public_status, 'title': track.get('title'),
                           'artists': [item['name'] for item in track.get('artists', [])],
                           'ids': track.get('ids'), 'duration_ms': track.get('duration_ms'),
                           'identity_status': track.get('identity_status'),
                           'cold_seconds': elapsed, 'warm_seconds': warm_elapsed,
                           'debug_bytes': size, 'public_bytes': public_size,
                           'same_record': same_record, 'same_provider_data': debug_data == public_data}
                    summaries.append(row)
                    (args.output / f'{index:02d}-debug.json').write_text(json.dumps(debug, indent=2, ensure_ascii=False) + '\n')
                    (args.output / f'{index:02d}-public.json').write_text(json.dumps(public, indent=2, ensure_ascii=False) + '\n')
                    print(json.dumps(row, ensure_ascii=False), flush=True)
                (args.output / 'summary.json').write_text(json.dumps(summaries, indent=2, ensure_ascii=False) + '\n')
                (args.output / 'providers-final.json').write_text(json.dumps(request('/v1/providers')[1], indent=2) + '\n')
                failures = [row for row in summaries if row['http_status'] != 200 or row['public_http_status'] != 200 or not row['ids'] or not row['same_record'] or not row['same_provider_data']]
                if failures:
                    raise RuntimeError(f'{len(failures)} song checks failed; see summary.json')
            finally:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait()


if __name__ == '__main__':
    main()
