#!/usr/bin/env python3
"""Private JSON-lines metadata worker. Go owns deadlines, pooling and caching."""
import json
import re
import sys
import requests
from ytmusicapi import YTMusic

MAX_ITEMS = 100
PRIVATE = ('token', 'cookie', 'authorization', 'tracking', 'feedback', 'streaming', 'playback', 'signature', 'cipher', 'visitor', 'clicktracking', 'inlibrary', 'likestatus', 'account', 'subscribed')


class TimeoutSession(requests.Session):
    def request(self, method, url, **kwargs):
        if kwargs.get('timeout') is None:
            kwargs['timeout'] = (3, 4)
        kwargs['stream'] = True
        response = super().request(method, url, **kwargs)
        chunks, size = [], 0
        try:
            for chunk in response.iter_content(65536):
                size += len(chunk)
                if size > 4*1024*1024:
                    raise ValueError('upstream_unavailable')
                chunks.append(chunk)
            response._content = b''.join(chunks)
            response._content_consumed = True
            return response
        finally:
            response.close()


def clean(value, depth=0):
    if depth > 16:
        return None
    if isinstance(value, dict):
        return {key: clean(item, depth+1) for key, item in value.items()
                if not any(word in key.lower().replace('_', '') for word in PRIVATE)}
    if isinstance(value, list):
        return [clean(item, depth+1) for item in value[:MAX_ITEMS]]
    if isinstance(value, str):
        return value[:12000]
    return value


def valid_id(kind, value):
    if not isinstance(value, str):
        return False
    kind = 'artist' if kind in ('artist_albums', 'artist_singles') else kind
    patterns = {'song': r'[A-Za-z0-9_-]{11}', 'album': r'MPRE[A-Za-z0-9_-]{1,150}',
                'artist': r'UC[A-Za-z0-9_-]{20,30}', 'playlist': r'(?:OLAK5uy_|PL|RD)[A-Za-z0-9_-]{5,150}'}
    return bool(re.fullmatch(patterns.get(kind, r'(?!)'), value))


class Extractor:
    def __init__(self, client=None):
        self.client = client or YTMusic(requests_session=TimeoutSession(), language='en', location='US')

    def execute(self, operation, entity_id, limit=20):
        limit = max(1, min(MAX_ITEMS, int(limit)))
        if operation == 'ping':
            return {'protocol': 1, 'provider': 'ytmusic', 'mode': 'public', 'capabilities': ['song', 'album', 'artist', 'playlist', 'artist_albums', 'artist_singles']}
        if not valid_id(operation, entity_id):
            raise ValueError('invalid_id')
        client = self.client
        if operation == 'song':
            result = {'id': entity_id, 'kind': 'song'}
            partial = False
            failed = False
            # The watch response often supplies structured artists and album refs
            # even when the separate player endpoint is unavailable.
            try:
                watch = client.get_watch_playlist(videoId=entity_id, limit=1)
                watch['tracks'] = [item for item in watch.get('tracks', []) if item.get('videoId') == entity_id]
                watch.pop('lyrics', None)
                if watch['tracks']:
                    result['watch'] = watch
            except Exception:
                partial = True
                failed = True
            try:
                song = client.get_song(entity_id)
                details = song.get('videoDetails') or {}
                if details.get('videoId') == entity_id:
                    result['song'] = {'videoDetails': details, 'microformat': song.get('microformat', {})}
                else:
                    partial = True
            except Exception:
                partial = True
                failed = True
            if 'watch' not in result and 'song' not in result:
                raise ValueError('upstream_unavailable' if failed else 'not_found')
            result['_partial'] = partial
            return clean(result)
        if operation in ('artist_albums', 'artist_singles'):
            profile = client.get_artist(entity_id)
            section = profile.get('albums' if operation == 'artist_albums' else 'singles') or {}
            results = section.get('results', [])
            if section.get('browseId') and section.get('params'):
                results = client.get_artist_albums(section['browseId'], section['params'], limit=limit)
            return clean({'id': entity_id, 'kind': operation, 'results': results[:limit], 'limit': limit, 'possibly_more': len(results) >= limit})
        if operation == 'album':
            result = client.get_album(entity_id)
            result['tracks'] = result.get('tracks', [])[:limit]
            return clean({'id': entity_id, 'kind': operation, 'metadata': result, 'limit': limit, 'truncated': int(result.get('trackCount') or 0) > limit})
        if operation == 'artist':
            result = client.get_artist(entity_id)
            # Top catalog sections retain browse IDs/params for further requests.
            for section in result.values():
                if isinstance(section, dict) and isinstance(section.get('results'), list):
                    section['results'] = section['results'][:limit]
            return clean({'id': entity_id, 'kind': operation, 'metadata': result, 'limit': limit, 'scope': 'profile_and_top_releases'})
        if operation == 'playlist':
            result = client.get_playlist(entity_id, limit=limit)
            result['tracks'] = result.get('tracks', [])[:limit]
            return clean({'id': entity_id, 'kind': operation, 'metadata': result, 'limit': limit})
        raise ValueError('unsupported')


def main():
    extractor = Extractor()
    for line in sys.stdin:
        try:
            if len(line) > 16384:
                raise ValueError('invalid_id')
            request = json.loads(line)
            data = extractor.execute(request.get('operation'), request.get('id'), request.get('limit', 20))
            partial = data.pop('_partial', False)
            response = {'ok': True, 'data': data, 'partial': partial}
        except ValueError as error:
            code = str(error)
            response = {'ok': False, 'error': code if code in ('invalid_id', 'not_found', 'unsupported') else 'upstream_unavailable'}
        except Exception:
            response = {'ok': False, 'error': 'upstream_unavailable'}
        encoded = json.dumps(response, ensure_ascii=False, separators=(',', ':'))
        if len(encoded.encode()) > 4*1024*1024:
            encoded = '{"ok":false,"error":"upstream_unavailable"}'
        print(encoded, flush=True)


if __name__ == '__main__':
    main()
