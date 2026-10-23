# Acoustic fingerprint candidates

[API](../API_SPEC.md) · [Enrichment](../ENRICHMENT.md#fingerprint-api)

`POST /v1/fingerprint` looks up an existing Chromaprint fingerprint through
AcoustID. The server does not download audio, calculate the fingerprint or choose
an ambiguous recording automatically. Configure an AcoustID application key and
pass its startup probe before using the endpoint.

```sh
curl --request POST http://localhost:8080/v1/fingerprint \
  --header 'Content-Type: application/json' \
  --data '{"fingerprint":"YOUR_CHROMAPRINT_OUTPUT","duration":180}'
```

Replace the placeholder with the actual fingerprint. `duration` is the full file
length in **seconds**, integer 1–86400. Body limit: 128 KiB; fingerprint maximum:
65,536 characters. The response preserves AcoustID `status` and candidate `results`.

Bad input returns 400; unavailable configuration/source returns 503; upstream
lookup failure returns 502. The operator key stays on the server and is submitted
upstream in POST form data. Lookup POSTs are not cached. The free service has
noncommercial conditions; see [attribution/access](../guides/licensing.md).
