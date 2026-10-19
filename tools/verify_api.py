#!/usr/bin/env python3
"""Credential-free API contract smoke; uses a supplied server binary and isolated caches."""
import argparse
import concurrent.futures
import json
import os
from pathlib import Path
import socket
import statistics
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

PROVIDERS = ['spotify', 'ytmusic', 'applemusic', 'deezer', 'jiosaavn', 'musicbrainz',
             'soundcloud', 'qobuz', 'tidal', 'amazonmusic', 'pandora', 'bugs', 'genie', 'melon']


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    binary = args.server.resolve()
    cases, timings = [], []
    with tempfile.TemporaryDirectory(prefix='ommr-contract-') as temp:
        root = Path(temp)
        env = {k: v for k, v in os.environ.items() if not k.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''

        def start(profile, policy=''):
            with socket.socket() as sock:
                sock.bind(('127.0.0.1', 0))
                port = sock.getsockname()[1]
            config = root / (profile + '.yaml')
            config.write_text(f'port: {port}\ncredentials_dir: ""\ncache_provider: disk\ncache_dir: {root / profile}\n'
                              'provider_timeout: 15s\nenable_enrichment: true\nbulk_max_items: 5\nbulk_workers: 2\nrate_limit_rps: 1000\n'
                              'providers:\n  enabled:\n' + ''.join(f'    {p}: {str(p == "applemusic").lower()}\n' for p in PROVIDERS) + policy)
            config.chmod(0o600)
            log = (args.output / (profile + '-server.log')).open('a')
            proc = subprocess.Popen([str(binary), '--config', str(config)], cwd=root, env=env, stdout=log, stderr=log)
            base = f'http://127.0.0.1:{port}'
            for _ in range(150):
                if proc.poll() is not None:
                    raise RuntimeError('server exited; inspect profile log')
                try:
                    with urllib.request.urlopen(base + '/v1/health', timeout=.4):
                        return proc, log, base
                except OSError:
                    time.sleep(.1)
            proc.terminate()
            proc.wait(timeout=10)
            log.close()
            raise RuntimeError('server startup timed out')

        def stop(proc, log):
            proc.terminate()
            try:
                proc.wait(timeout=10)
            except subprocess.TimeoutExpired:
                proc.kill()
                proc.wait()
            log.close()

        def request(base, path, payload=None, method=None):
            data = json.dumps(payload).encode() if payload is not None else None
            req = urllib.request.Request(base + path, data=data, method=method,
                                         headers={'Content-Type': 'application/json'})
            begin = time.monotonic()
            try:
                response = urllib.request.urlopen(req, timeout=25)
            except urllib.error.HTTPError as error:
                response = error
            with response:
                raw = response.read()
                try:
                    body = json.loads(raw)
                except ValueError:
                    body = {'text': raw.decode(errors='replace')[:200]}
                return response.status, body, (time.monotonic() - begin) * 1000

        def check(name, base, path, expected=200, payload=None, predicate=None, method=None):
            try:
                status, body, elapsed = request(base, path, payload, method)
                passed = status == expected and (predicate is None or bool(predicate(body)))
                row = {'case': name, 'expected_http': expected, 'actual_http': status,
                       'passed': passed, 'milliseconds': round(elapsed, 2)}
                if not passed:
                    row['response'] = body
            except Exception as error:
                body = {}
                row = {'case': name, 'passed': False, 'error_type': type(error).__name__}
            cases.append(row)
            print(json.dumps(row), flush=True)
            return body

        proc, log, base = start('public')
        try:
            check('health', base, '/v1/health')
            check('providers', base, '/v1/providers', predicate=lambda b: all(p['mode'] != 'browser_session' for p in b['providers']))
            check('capabilities', base, '/v1/capabilities', predicate=lambda b: b['api']['endpoints']['albums']['runtime_available'])
            check('capabilities_gateway', base, '/v1/query', payload={'operation': 'capabilities'})
            for name, path in [
                ('missing_input', '/v1/resolve'), ('bad_debug', '/v1/resolve?apple_id=1829591986&debug=invalid'),
                ('repeated_debug', '/v1/resolve?apple_id=1829591986&debug=true&debug=false'),
                ('foreign_url', '/v1/resolve?url=https://foreign.invalid/track/123'),
                ('typed_foreign_url', '/v1/resolve?apple_id=https://foreign.invalid/track/123'),
                ('mixed_url_id', '/v1/resolve?url=https://www.deezer.com/track/123&apple_id=1829591986'),
                ('bad_album_id', '/v1/albums?provider=musicbrainz&id=123'),
                ('repeated_album_id', '/v1/albums?provider=applemusic&id=123&id=456'),
                ('album_limit_overflow', '/v1/albums?provider=applemusic&id=123&limit=1001'),
                ('bad_artist_id', '/v1/artists?provider=applemusic&id=https://foreign.invalid'),
            ]:
                check(name, base, path, 400)
            for op in ['resolve', 'links', 'extract', 'search', 'artists', 'albums', 'capabilities', 'catalog', 'providers']:
                check(op + '_method', base, '/v1/' + op, 405, method='PUT')
            check('bulk_method', base, '/v1/bulk', 405)
            check('query_method', base, '/v1/query', 405)
            check('fingerprint_without_key', base, '/v1/fingerprint', 503, payload={'fingerprint': 'example', 'duration': 160})
            check('catalog_without_worker', base, '/v1/catalog?provider=ytmusic&type=album&id=MPREb_ToFqhXgz9A7', 503)
            check('album_disabled_source', base, '/v1/albums?provider=deezer&id=6575789', 503)
            for payload in [{'operation': 'no_such_operation'}, {'operation': 'resolve', 'unknown': 1}, {'operation': 'resolve', 'query': []}]:
                check('invalid_gateway_' + str(len(cases)), base, '/v1/query', 400, payload=payload)
            share = 'https://music.apple.com/us/album/p-pop-culture/1829591921?i=1829591986&ls=1'
            path = '/v1/resolve?' + urllib.parse.urlencode({'url': share})
            track_ok = lambda b: b.get('track', {}).get('ids', {}).get('applemusic') == '1829591986' and b['track']['title'] == 'Boyfriend'
            public = check('resolve_share_public', base, path, predicate=track_ok)
            debug = check('resolve_debug_cached', base, path + '&debug=true', predicate=lambda b: track_ok(b) and any(s['cached'] for s in b['provider_status']))
            graph = public.get('track', {}).get('music_graph')
            check('resolve_gateway_parity', base, '/v1/query', payload={'operation': 'resolve', 'apple_id': '1829591986'},
                  predicate=lambda b: b.get('track') == public.get('track'))
            cases.append({'case': 'graph_public_debug_parity', 'passed': bool(graph) and graph == debug.get('track', {}).get('extensions', {}).get('music_graph')})
            cases.append({'case': 'production_diagnostics_omitted', 'passed': 'provider_status' not in public and 'match_score' not in public.get('track', {})})
            check('links', base, '/v1/links?apple_id=1829591986', predicate=lambda b: bool(b))
            check('extract_exact', base, '/v1/extract?provider=applemusic&id=1829591986', predicate=lambda b: bool(b.get('results')))
            check('bulk_error_isolation', base, '/v1/bulk', payload={'queries': [{'apple_id': '1829591986'}, {'url': share}, {'url': 'https://foreign.invalid/track/123'}]},
                  predicate=lambda b: b['successful'] == 2 and b['failed'] == 1)
            check('bulk_empty', base, '/v1/bulk', 400, payload={'queries': []})
            check('bulk_limit', base, '/v1/bulk', 400, payload={'queries': [{'apple_id': '1829591986'}] * 6})
            album = check('album_complete', base, '/v1/albums?provider=applemusic&id=1829591921',
                          predicate=lambda b: b['album']['track_list'] == {'returned': 11, 'total': 11, 'complete': True} and len(b['album']['discs']) == 2)
            check('album_gateway_cached', base, '/v1/query', payload={'operation': 'albums', 'provider': 'applemusic', 'id': '1829591921', 'debug': True},
                  predicate=lambda b: b['album'] == album['album'] and b['source']['cached'])
            check('album_limit_and_graph', base, '/v1/albums?provider=applemusic&id=1829591921&limit=2',
                  predicate=lambda b: b['album']['track_list']['reason'] == 'limit' and len([n for n in b['album']['music_graph']['nodes'] if n['type'] == 'track_entry']) == 2)
            check('artist_native', base, '/v1/artists?provider=applemusic&id=914892130&limit=2', predicate=lambda b: b['profiles'][0]['id'] == '914892130')
            check('search', base, '/v1/search?q=Karan%20Aujla%20Boyfriend&sources=applemusic&limit=2', predicate=lambda b: isinstance(b.get('results'), list))
            check('search_large_page', base, '/v1/search?q=Karan%20Aujla%20Boyfriend&sources=applemusic&page=9223372036854775807', predicate=lambda b: b.get('results') == [])
            def warm(_):
                status, body, elapsed = request(base, path)
                return status == 200 and body.get('track') == public.get('track'), elapsed
            begin = time.monotonic()
            with concurrent.futures.ThreadPoolExecutor(max_workers=16) as pool:
                rows = list(pool.map(warm, range(32)))
            timings = sorted(row[1] for row in rows)
            cases.append({'case': '32_warm_requests_16_clients', 'passed': all(row[0] for row in rows), 'wall_seconds': round(time.monotonic() - begin, 3),
                          'median_ms': round(statistics.median(timings), 2), 'p95_ms': round(timings[int(.95 * (len(timings) - 1))], 2), 'maximum_ms': round(max(timings), 2)})
        finally:
            stop(proc, log)
        proc, log, base = start('public')
        try:
            check('disk_cache_survives_restart', base, '/v1/albums?provider=applemusic&id=1829591921&debug=true',
                  predicate=lambda b: b['source']['cached'] and b['album']['observed_at'] == album['album']['observed_at'])
        finally:
            stop(proc, log)
        policy = 'api:\n  default_operation: capabilities\n  allow_debug: false\n  endpoints:\n    albums: false\n    search: false\n    metrics: false\n'
        proc, log, base = start('policy', policy)
        try:
            check('configured_default_operation', base, '/v1/query', payload={}, predicate=lambda b: b['capabilities_version'] == 1)
            check('disabled_album_get', base, '/v1/albums?provider=applemusic&id=1829591921', 404)
            check('disabled_album_gateway', base, '/v1/query', 404, payload={'operation': 'albums', 'provider': 'applemusic', 'id': '1829591921'})
            check('disabled_search_get', base, '/v1/search?q=Song', 404)
            check('disabled_search_gateway', base, '/v1/query', 404, payload={'operation': 'search', 'q': 'Song'})
            check('disabled_metrics', base, '/metrics', 404)
            check('debug_policy_get', base, '/v1/resolve?apple_id=1829591986&debug=true', predicate=lambda b: track_ok(b) and 'provider_status' not in b)
            check('debug_policy_gateway', base, '/v1/query', payload={'operation': 'resolve', 'apple_id': '1829591986', 'debug': True}, predicate=lambda b: track_ok(b) and 'provider_status' not in b)
        finally:
            stop(proc, log)
        proc, log, base = start('locked', 'api:\n  default_operation: capabilities\n  allow_overrides: false\n')
        try:
            check('locked_operation', base, '/v1/query', payload={'operation': 'resolve', 'apple_id': '1829591986'}, predicate=lambda b: b['capabilities_version'] == 1)
        finally:
            stop(proc, log)
    report = {'cases': cases, 'passed': sum(bool(c['passed']) for c in cases), 'failed': [c for c in cases if not c['passed']],
              'scope': 'Isolated public Apple; real upstream for positive catalog cases; no credentials; warm-cache concurrency only'}
    (args.output / 'summary.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps({'passed': report['passed'], 'failed': len(report['failed'])}), flush=True)
    raise SystemExit(bool(report['failed']))


if __name__ == '__main__':
    main()
