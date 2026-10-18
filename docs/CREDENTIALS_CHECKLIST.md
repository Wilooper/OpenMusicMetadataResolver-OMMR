# Credentials accepted by this OMMR implementation

Authentication setup was stopped at the operator's request. No new API keys were generated or cookies exported. The live song matrix used no credentials.

This table describes the current server inputs, not a promise of upstream account eligibility or additional private metadata. Place only credentials you are authorized to use in the private files below. Passwords, recovery codes and account login credentials are not server inputs.

All filenames are relative to `credentials_dir` (default `./credentials`). Set the directory to mode 700 and each file to mode 600. Restart the server after changes; inspect `/v1/providers` for the selected mode and probe outcome. Do not paste keys or cookies into chat or commit them.

| Source | Required inputs for credentialed mode | JSON file fields | Without valid credentials |
| --- | --- | --- | --- |
| Spotify, official catalog | Client ID **and** client secret for an authorized Spotify application | `spotify.json`: `client_id`, `client_secret` | Public/embed methods; anonymous text search currently fails in this environment |
| Spotify, alternative session | Current authorized bearer; optional `sp_dc` cookie and Unix expiry for renewal attempts. A cookie alone may fail the web-player challenge | Same `spotify.json`: `access_token`, `cookie`, `expires_at` | Next supported mode, then public methods |
| Apple Music catalog | Usable developer/catalog bearer JWT | `applemusic.json`: `developer_token` | Public iTunes catalog |
| Apple Music account session, optional | Developer/catalog bearer **plus** Music User Token; ordinary Apple ID cookies are insufficient | Same `applemusic.json`: `developer_token`, `music_user_token`; a `media-user-token` cookie can supply the latter | Catalog-only mode if validated, then public iTunes |
| YouTube Music session, optional | Authorized YouTube cookies including `SAPISID` or `__Secure-3PAPISID`, with the rest of the matching session cookies | `ytmusic.json`: `cookie`; supported header JSON or domain-filtered cookie export also accepted | Public InnerTube/oEmbed |
| TIDAL | Authorized OAuth bearer with working catalog access | `tidal.json`: `token` | Source unloaded |
| Qobuz | Issued application ID; optional token supported by the adapter's bearer mechanism (provider compatibility must be tested) | `qobuz.json`: `app_id`, optional `token` | Source unloaded without usable app access |
| Amazon Music | Approved Music API bearer **and** API key; ordinary Amazon cookies are not substitutes | `amazonmusic.json`: `token`, `api_key` | Source unloaded |
| Pandora | Approved OAuth bearer **and** an accessible track ID for validation | `pandora.json`: `token`; YAML: `providers.pandora_probe_id` | Source unloaded |
| Last.fm enrichment | API key | `lastfm.json`: `api_key` | Enricher unloaded |
| Discogs enrichment | Personal access token | `discogs.json`: `token` | Enricher unloaded |
| AcoustID fingerprint lookup | Application API key; lookup also needs a caller-supplied fingerprint and duration | `acoustid.json`: `api_key` | Fingerprint service unloaded |
| Bugs, Genie, Melon | None; public song search/detail adapters | No credential file consumed | Public access; timeouts and alias limits still apply |
| Deezer, JioSaavn, MusicBrainz, Cover Art Archive | None in this implementation | No credential file consumed for these public adapters | Public access, subject to network and service availability |
| SoundCloud | None for implemented public permalink/oEmbed lookup | No OAuth configuration currently implemented | Permalink lookup available; artist/title search unsupported |

Start with Spotify official credentials, then Apple catalog access, then optional YouTube session access. Required-key sources can be added individually; failed probes leave them unloaded. Enrichment also requires a track with the appropriate identifiers, such as a MusicBrainz release ID for Cover Art Archive. Supplying credentials does not guarantee extra fields for every recording.

Example placeholders only:

```json
{"client_id":"YOUR_CLIENT_ID","client_secret":"YOUR_CLIENT_SECRET"}
```

Save that shape as `credentials/spotify.json`; replace placeholders privately. Session import formats, precedence, protected startup probes, fallbacks and expiry limitations are documented in [PROVIDER_LIFECYCLE.md](PROVIDER_LIFECYCLE.md).

Optional Python YouTube extraction currently uses public mode; no new credentials are required. See [RICH_CATALOGS.md](RICH_CATALOGS.md).
