#!/usr/bin/env python3
"""Public-only SponsorBlock POI checks through the running OMMR API."""
import argparse
from contextlib import contextmanager
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

VIDEO = 'kJQP7kiw5Fk'


def fetch(url, payload=None, encoding='identity'):
    data = json.dumps(payload).encode() if payload is not None else None
    req = urllib.request.Request(url, data=data, headers={
        'Content-Type': 'application/json', 'Accept-Encoding': encoding,
    })
    with urllib.request.urlopen(req, timeout=30) as response:
        raw = response.read()
        body = gzip.decompress(raw) if response.headers.get('Content-Encoding') == 'gzip' else raw
        return json.loads(body), len(raw)


@contextmanager
def server(binary, output, enabled):
    with tempfile.TemporaryDirectory(prefix='ommr-sponsorblock-') as temp:
        root = Path(temp)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        config = root / 'config.yaml'
        config.write_text(
            f'port: {port}\ncredentials_dir: ""\ncache_provider: disk\n'
            f'cache_dir: {root / "cache"}\nprovider_timeout: 25s\n'
            'sponsorblock_timeout: 10s\nenable_enrichment: true\nrate_limit_rps: 1000\n'
            'api:\n  default_sources: [ytmusic]\nproviders:\n  enabled:\n'
            + ''.join(f'    {p}: {str(p in ["ytmusic", "applemusic"]).lower()}\n' for p in PROVIDERS)
            + '    coverartarchive: false\n    musicbrainz_release: false\n'
            + '    lastfm: false\n    discogs: false\n'
            + f'    sponsorblock: {str(enabled).lower()}\n'
        )
        config.chmod(0o600)
        env = {k: v for k, v in os.environ.items() if not k.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''
        with (output / f'server-{enabled}.log').open('w') as log:
            proc = subprocess.Popen([str(binary), '--config', str(config)], cwd=root,
                                    env=env, stdout=log, stderr=log)
            try:
                base = f'http://127.0.0.1:{port}'
                for _ in range(100):
                    if proc.poll() is not None:
                        raise RuntimeError('server exited; inspect server log')
                    try:
                        fetch(base + '/v1/health')
                        break
                    except OSError:
                        time.sleep(.1)
                else:
                    raise RuntimeError('server startup timed out')
                yield base
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    binary, output = args.server.resolve(), args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    rows = []

    def save(name, body):
        (output / f'{name}.json').write_text(json.dumps(body, ensure_ascii=False, indent=2) + '\n')

    def check(name, fn):
        begin = time.monotonic()
        try:
            row = {'case': name, **fn(), 'passed': True}
        except Exception as error:
            row = {'case': name, 'passed': False, 'error': str(error)}
        row['seconds'] = round(time.monotonic() - begin, 3)
        rows.append(row)
        print(json.dumps(row), flush=True)

    with server(binary, output, True) as base:
        def positive():
            body, _ = fetch(base + '/v1/resolve?' + urllib.parse.urlencode({
                'youtube_id': VIDEO, 'sources': 'ytmusic', 'debug': 'true',
            }))
            save('positive-debug', body)
            assert body.get('track'), 'YouTube metadata did not resolve'
            data = body['track']['extensions'].get('video_annotations', {}).get('sponsorblock')
            assert data and data['video_id'] == VIDEO, 'SponsorBlock highlight unavailable'
            assert data['points_of_interest'], 'no compatible points'
            assert all(p['category'] == 'poi_highlight' and p['action_type'] == 'poi'
                       for p in data['points_of_interest'])
            return {'title': body['track']['title'], 'video_id': VIDEO,
                    'source_duration_ms': data.get('video_duration_ms'),
                    'points': data['points_of_interest'],
                    'enrichment_status': body['enrichment_status']['sponsorblock']}
        check('positive_exact_video', positive)

        def parity():
            path = '/v1/resolve?' + urllib.parse.urlencode({'youtube_id': VIDEO, 'sources': 'ytmusic'})
            body, plain_size = fetch(base + path)
            compressed, gzip_size = fetch(base + path, encoding='gzip')
            gateway, _ = fetch(base + '/v1/query', {'operation': 'resolve', 'youtube_id': VIDEO})
            save('production', body)
            assert body == compressed == gateway, 'production/compression/gateway changed metadata'
            assert body.get('track', {}).get('video_annotations'), 'annotation absent'
            return {'plain_bytes': plain_size, 'gzip_bytes': gzip_size, 'metadata_equal': True}
        check('production_gzip_gateway_cache', parity)

        def non_youtube():
            body, _ = fetch(base + '/v1/resolve?apple_id=1829591986&sources=applemusic&debug=true')
            save('non-youtube', body)
            assert body.get('track'), 'Apple metadata did not resolve'
            assert 'ytmusic' not in body['track']['ids']
            assert 'video_annotations' not in body['track'].get('extensions', {})
            assert body['enrichment_status']['sponsorblock'] == 'no_data'
            return {'title': body['track']['title'], 'video_annotations_absent': True}
        check('no_accepted_youtube_id', non_youtube)

        def no_highlight():
            body, _ = fetch(base + '/v1/resolve?youtube_id=tIjUfo30r4M&sources=ytmusic&debug=true')
            save('no-highlight', body)
            assert body.get('track'), 'YouTube metadata did not resolve'
            status = body['enrichment_status']['sponsorblock']
            assert status in ['ok', 'no_data'], 'SponsorBlock source unavailable'
            return {'title': body['track']['title'], 'enrichment_status': status}
        check('additional_video_coverage', no_highlight)

    with server(binary, output, False) as base:
        def disabled():
            body, _ = fetch(base + '/v1/resolve?youtube_id=' + VIDEO + '&sources=ytmusic&debug=true')
            save('disabled', body)
            assert body.get('track'), 'YouTube metadata did not resolve'
            assert 'sponsorblock' not in body.get('enrichment_status', {})
            assert 'video_annotations' not in body['track'].get('extensions', {})
            providers, _ = fetch(base + '/v1/providers')
            status = next(p for p in providers['providers'] if p['name'] == 'sponsorblock')
            assert status['state'] == 'disabled'
            return {'source_state': status['state'], 'metadata_resolved': True}
        check('source_disabled', disabled)

    save('live-summary', rows)
    return 0 if all(r['passed'] for r in rows) else 1


if __name__ == '__main__':
    raise SystemExit(main())
