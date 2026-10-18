boyfriend-search-captured.json is the public https://api.deezer.com/search?limit=5&q=Karan%20Aujla%20Boyfriend response captured in Termux on 2026-10-18, pretty-printed. No account data or credentials. The advanced artist/track query returned {"data":[],"total":0} in the same run; see docs/test-results/termux-2026-10-18/before-deezer.json.

get-lucky-search-captured.json is the same public search with q=Daft%20Punk%20Get%20Lucky and Accept-Language: en, captured on the same device/date. Keeps radio/album versions and unrelated tribute/cover rows to verify artist selection. Without the header, the original artist was returned as ダフト・パンク; lang=en query parameter did not change that response.

Both captures omit preview URLs, including ephemeral signed query parameters; no audio was requested.
