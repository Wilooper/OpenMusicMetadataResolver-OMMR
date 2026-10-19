#!/usr/bin/env python3
"""Test songs, their source-owned albums and native artist IDs across styles/versions."""
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

CORPUS = [
    ('punjabi_pop', 'Karan Aujla', 'Boyfriend'),
    ('hindi_soundtrack', 'Arijit Singh', 'Tum Hi Ho'),
    ('english_pop', 'The Weeknd', 'Blinding Lights'),
    ('hip_hop', 'Kendrick Lamar', 'HUMBLE.'),
    ('k_pop', 'BTS', 'Dynamite'),
    ('japanese_pop', 'YOASOBI', 'Idol'),
    ('latin_pop', 'Luis Fonsi', 'Despacito'),
    ('rock', 'Queen', 'Bohemian Rhapsody'),
    ('electronic', 'Daft Punk', 'One More Time'),
    ('instrumental', 'Ludovico Einaudi', 'Experience'),
    ('classical', 'Glenn Gould', 'Goldberg Variations, BWV 988: Aria'),
    ('live', 'Eagles', 'Hotel California (Live)'),
    ('acoustic', 'Foo Fighters', 'Everlong (Acoustic Version)'),
    ('remix', 'Lana Del Rey', 'Summertime Sadness (Cedric Gervais Remix)'),
]


def graph_valid(graph):
    if not graph:
        return False
    nodes = {node['id'] for node in graph['nodes']}
    return graph['root'] in nodes and all(e['from'] in nodes and e['to'] in nodes for e in graph['edges'])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--server', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    args.output.mkdir(parents=True, exist_ok=True)
    rows = []
    with tempfile.TemporaryDirectory(prefix='ommr-entities-') as directory:
        root = Path(directory)
        with socket.socket() as sock:
            sock.bind(('127.0.0.1', 0))
            port = sock.getsockname()[1]
        config = root / 'test.yaml'
        config.write_text(f'port: {port}\ncredentials_dir: ""\ncache_dir: {root / "cache"}\nprovider_timeout: 15s\napple_storefront: us\n')
        config.chmod(0o600)
        env = {k: v for k, v in os.environ.items() if not k.startswith('OMMR_')}
        env['OMMR_ENV_FILE'] = ''
        base = f'http://127.0.0.1:{port}'

        def request(operation, params):
            start = time.monotonic()
            try:
                response = urllib.request.urlopen(base + '/v1/' + operation + '?' + urllib.parse.urlencode(params), timeout=25)
            except urllib.error.HTTPError as error:
                response = error
            except OSError as error:
                return 0, {'transport_error': type(error).__name__}, round(time.monotonic() - start, 3)
            with response:
                raw = response.read()
                try:
                    body = json.loads(raw)
                except ValueError:
                    body = {'invalid_json': True}
                return response.status, body, round(time.monotonic() - start, 3)

        with (args.output / 'server.log').open('w') as log:
            proc = subprocess.Popen([str(args.server.resolve()), '--config', str(config)], cwd=root, env=env, stdout=log, stderr=log)
            try:
                for _ in range(200):
                    if proc.poll() is not None:
                        raise RuntimeError('server failed to start')
                    if request('health', {})[0] == 200:
                        break
                    time.sleep(.1)
                else:
                    raise RuntimeError('server did not become ready')
                (args.output / 'providers-startup.json').write_text(json.dumps(request('providers', {})[1], indent=2) + '\n')
                for index, (category, artist, title) in enumerate(CORPUS, 1):
                    status, response, elapsed = request('resolve', {'artist': artist, 'title': title, 'debug': 'true'})
                    track = response.get('track') or {}
                    row = {'category': category, 'query_artist': artist, 'query_title': title,
                           'song_http': status, 'song_seconds': elapsed, 'song_resolved': bool(track.get('ids')),
                           'title': track.get('title'), 'artists': [a['name'] for a in track.get('artists', [])],
                           'ids': track.get('ids', {}), 'duration_ms': track.get('duration_ms'), 'genres': track.get('genres'),
                           'explicit': track.get('explicit'), 'credits_count': len(track.get('credits', [])),
                           'song_graph_valid': graph_valid(track.get('extensions', {}).get('music_graph')),
                           'album': {'status': 'no_native_id'}, 'artist': {'status': 'no_native_id'}}
                    (args.output / f'{index:02d}-song.json').write_text(json.dumps(response, indent=2, ensure_ascii=False) + '\n')
                    source_records=track.get('extensions',{}).get('provider_data',{})
                    row['source_observations']={name:{'title':data.get('metadata',{}).get('title'),
                                                    'duration_ms':data.get('metadata',{}).get('duration_ms'),
                                                    'isrc':data.get('metadata',{}).get('isrc')}
                                                for name,data in source_records.items() if isinstance(data,dict) and 'metadata' in data}
                    durations=[v['duration_ms'] for v in row['source_observations'].values() if v.get('duration_ms',0)>0]
                    row['material_duration_conflict']=bool(durations and max(durations)-min(durations)>max(10000,min(durations)//20))
                    apple = track.get('extensions', {}).get('provider_data', {}).get('applemusic', {})
                    metadata = apple.get('metadata', {})
                    native_album = metadata.get('album', {}).get('ids', {}).get('applemusic')
                    native_track = metadata.get('ids', {}).get('applemusic')
                    if native_album:
                        code, data, seconds = request('albums', {'provider': 'applemusic', 'id': native_album, 'debug': 'true'})
                        album = data.get('album') or {}
                        entries = album.get('tracks', [])
                        declared = metadata.get('album', {}).get('total_tracks')
                        total = album.get('track_list', {}).get('total')
                        row['album'] = {'http': code, 'seconds': seconds, 'id': native_album, 'title': album.get('title'),
                                        'track_list': album.get('track_list'), 'discs': len(album.get('discs', [])),
                                        'contains_source_track': any(t['id'] == native_track for t in entries),
                                        'graph_valid': graph_valid(album.get('music_graph')),
                                        'song_reported_album_total': declared, 'album_total': total,
                                        'reported_totals_consistent': declared is None or total is None or declared == total,
                                        'source_disc_count': apple.get('catalog', {}).get('discCount')}
                        (args.output / f'{index:02d}-album.json').write_text(json.dumps(data, indent=2, ensure_ascii=False) + '\n')
                    artists = metadata.get('artists', [])
                    native_artist = next((a.get('ids', {}).get('applemusic') for a in artists if a.get('ids', {}).get('applemusic')), None)
                    if native_artist:
                        code, data, seconds = request('artists', {'provider': 'applemusic', 'id': native_artist, 'limit': 5, 'debug': 'true'})
                        profile = next((p for p in data.get('profiles', []) if p['id'] == native_artist), {})
                        row['artist'] = {'http': code, 'seconds': seconds, 'id': native_artist, 'name': profile.get('name'),
                                         'identity_method': profile.get('identity_method'), 'images': len(profile.get('images', [])),
                                         'recent_tracks': len(profile.get('recent_tracks', [])), 'biography': bool(profile.get('biography')),
                                         'counts': profile.get('counts'), 'metrics': profile.get('metrics', []), 'sources': data.get('sources', [])}
                        (args.output / f'{index:02d}-artist.json').write_text(json.dumps(data, indent=2, ensure_ascii=False) + '\n')
                    if not native_artist:
                        code,data,seconds=request('artists',{'artist':artist,'sources':'applemusic','limit':5,'debug':'true'})
                        profile=next(iter(data.get('profiles',[])),{})
                        row['artist']={'http':code,'seconds':seconds,'id':profile.get('id'),'name':profile.get('name'),
                                       'identity_method':profile.get('identity_method'),'independent_name_lookup':True,
                                       'images':len(profile.get('images',[])),'recent_tracks':len(profile.get('recent_tracks',[])),
                                       'biography':bool(profile.get('biography')),'counts':profile.get('counts'),
                                       'metrics':profile.get('metrics',[]),'sources':data.get('sources',[])}
                        (args.output/f'{index:02d}-artist.json').write_text(json.dumps(data,indent=2,ensure_ascii=False)+'\n')
                    rows.append(row)
                    (args.output / 'summary.json').write_text(json.dumps(rows, indent=2, ensure_ascii=False) + '\n')
                    print(json.dumps(row, ensure_ascii=False), flush=True)
                (args.output / 'providers-final.json').write_text(json.dumps(request('providers', {})[1], indent=2) + '\n')
            finally:
                proc.terminate()
                try:
                    proc.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait()
    print(json.dumps({'songs_resolved': sum(r['song_resolved'] for r in rows),
                      'albums_returned': sum(r['album'].get('http') == 200 for r in rows),
                      'artists_returned': sum(bool(r['artist'].get('name')) for r in rows),
                      'total_cases': len(rows)}), flush=True)


if __name__ == '__main__':
    main()
