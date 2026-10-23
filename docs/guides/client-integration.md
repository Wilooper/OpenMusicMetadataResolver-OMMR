# Integrate OMMR into an app

[Docs](../README.md) · [API](../API_SPEC.md) · [Responses](responses.md)

Host the service, then call its HTTP API from your application. These examples are
ordinary HTTP clients, not a bundled SDK. Keep provider credentials on the server.

## JavaScript / Node.js

```js
async function resolveSong(baseURL, artist, title) {
  const url = new URL('/v1/resolve', baseURL);
  url.search = new URLSearchParams({ artist, title, sources: 'applemusic,ytmusic' });
  const response = await fetch(url, { signal: AbortSignal.timeout(28000) });
  if (!response.ok) throw new Error(`OMMR HTTP ${response.status}`);
  const { track } = await response.json();
  return track; // null is a valid no-match outcome
}
const track = await resolveSong('http://localhost:8080', 'Karan Aujla', 'Boyfriend');
console.log(track?.title ?? 'No accepted match');
```

`fetch` clients normally negotiate/decode compression automatically. Set a client
budget appropriate to your server's operation deadline. Avoid logging complete
upstream records or credentials when reporting failures.

## Python, standard library only

```python
import json
import urllib.error
import urllib.parse
import urllib.request

params = urllib.parse.urlencode({
    'artist': 'Karan Aujla', 'title': 'Boyfriend',
    'sources': 'applemusic,ytmusic',
})
request = urllib.request.Request('http://localhost:8080/v1/resolve?' + params)
try:
    with urllib.request.urlopen(request, timeout=28) as response:
        track = json.load(response)['track']
    print(track['title'] if track else 'No accepted match')
except urllib.error.HTTPError as error:
    print('OMMR HTTP', error.code)
```

This standard-library example requests plain JSON. If you manually send
`Accept-Encoding: gzip`, also inspect `Content-Encoding` and decompress before
JSON parsing; urllib does not do that for you.

## Browser and Android access

The current router provides no CORS middleware or built-in API authentication.
For a browser app, proxy a same-origin path such as `/music-api/` to OMMR, stripping
that prefix, or configure CORS and OPTIONS handling at your gateway. Cross-origin
JSON POST requests need preflight handling. Allow only the origins you intend.

For Android development, device `localhost` means the device itself. Use the server's
LAN address or HTTPS domain when the API runs elsewhere. Keep an internet-facing
instance behind an authenticated gateway and use its HTTPS endpoint in the app.

## Handle partial and unavailable data

- Check HTTP status before parsing a success envelope. Do not blindly retry 400.
- Treat `track: null`, missing preview/credits/POI, partial album lists and artist
  ambiguity as valid UI states.
- On 429 or transient failures, use bounded backoff; avoid retry storms.
- Preserve provider IDs and selected edition tuples when saving player state.
- Use `/v1/bulk` for bounded independent jobs; do not submit unbounded parallel calls.
- Inspect `/v1/capabilities` for feature availability and `/v1/providers` for source state.

For a preview feature, the POI supplies only a video offset. Your playback layer
must choose a segment length and use the matching playable video timeline.
