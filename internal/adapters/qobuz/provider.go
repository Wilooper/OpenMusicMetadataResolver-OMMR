package qobuz

import (
	"fmt"
	"net/url"
	"os"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/catalogapi"
)

const ProviderName = "qobuz"

func New() adapters.ProviderAdapter {
	baseURL := os.Getenv("OMMR_QOBUZ_BASE_URL")
	if baseURL == "" {
		baseURL = "https://www.qobuz.com"
	}
	return NewWithConfig(baseURL, os.Getenv("OMMR_QOBUZ_APP_ID"), os.Getenv("OMMR_QOBUZ_TOKEN"))
}

func NewWithConfig(baseURL, appID, token string) adapters.ProviderAdapter {
	return catalogapi.New(catalogapi.Config{Name: ProviderName, Version: "qobuz-api-v1", BaseURL: baseURL, Token: token,
		FetchPath: func(id string) (string, error) {
			if appID == "" {
				return "", fmt.Errorf("Qobuz requires an issued application ID (OMMR_QOBUZ_APP_ID)")
			}
			return "/api.json/0.2/track/get?track_id=" + url.QueryEscape(id) + "&app_id=" + url.QueryEscape(appID), nil
		},
		Search: func(q adapters.Query) (string, string, []byte, error) {
			if appID == "" {
				return "", "", nil, fmt.Errorf("Qobuz requires an issued application ID (OMMR_QOBUZ_APP_ID)")
			}
			terms := q.Title
			if q.Artist != "" {
				terms = q.Artist + " " + terms
			}
			if q.ISRC != "" {
				terms = q.ISRC
			}
			if terms == "" {
				return "", "", nil, nil
			}
			path := "/api.json/0.2/track/search?query=" + url.QueryEscape(terms) + "&app_id=" + url.QueryEscape(appID) + "&limit=5"
			return "GET", path, nil, nil
		},
	})
}
