#!/usr/bin/env python3
"""Bounded public connectivity probes. No credentials or third-party gateways."""
import concurrent.futures
import hashlib
import json
import time
import urllib.error
import urllib.request

BASE = 'https://www.jiosaavn.com/api.php?'
GROUPS = [
    [
        ('jio_legacy', BASE + '__call=search.getResults&q=Karan%20Aujla%20Boyfriend&_format=json&_marker=0&ctx=web6dot0'),
        ('jio_python_autocomplete', BASE + '__call=autocomplete.get&_format=json&_marker=0&cc=in&includeMetaTags=1&query=Karan%20Aujla%20Boyfriend'),
        ('jio_python_v4', BASE + '__call=search.getResults&p=0&_format=json&_marker=0&api_version=4&ctx=web6dot0&n=20&q=Karan%20Aujla%20Boyfriend'),
        ('jio_python_detail', BASE + '__call=song.getDetails&cc=in&_marker=0%3F_marker%3D0&_format=json&pids=e0kCEwoC'),
        ('jio_home', 'https://www.jiosaavn.com/'),
    ],
    [('deezer', 'https://api.deezer.com/search?q=Daft%20Punk')],
    [('musicbrainz', 'https://musicbrainz.org/ws/2/recording/?query=recording%3AGet%20Lucky&fmt=json&limit=1')],
    [('apple_control', 'https://itunes.apple.com/search?term=Karan%20Aujla%20Boyfriend&entity=song&limit=1')],
]


def probe(name, url):
    result = {'name': name, 'url': url}
    request = urllib.request.Request(url, headers={
        'Accept': 'application/json',
        'User-Agent': 'OMMR/1.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)',
    })
    try:
        try:
            response = urllib.request.urlopen(request, timeout=10)
        except urllib.error.HTTPError as error:
            response = error
        with response:
            body = response.read((1 << 20) + 1)
            result.update(status=response.status, bytes=len(body), truncated=len(body) > (1 << 20),
                          headers={key: response.headers[key] for key in ('Content-Type', 'Server', 'Content-Length') if key in response.headers},
                          sha256=hashlib.sha256(body).hexdigest())
            try:
                json.loads(body)
                result['json_valid'] = True
            except (ValueError, UnicodeError):
                result['json_valid'] = False
            result['site_unavailable'] = b'<h1>Site Unavailable</h1>' in body
    except (OSError, ValueError) as error:
        # Exception text can contain network/proxy information; only emit its class.
        result['error'] = type(error).__name__
    return result


def group_probes(group):
    results = []
    for index, (name, url) in enumerate(group):
        if index:
            time.sleep(1)
        results.append(probe(name, url))
    return results


if __name__ == '__main__':
    with concurrent.futures.ThreadPoolExecutor(max_workers=4) as pool:
        results = [result for group in pool.map(group_probes, GROUPS) for result in group]
    print(json.dumps(results, indent=2))
