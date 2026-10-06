package tidal

import (
	"fmt"
	"net/url"
	"os"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/catalogapi"
)

const ProviderName = "tidal"

func New() adapters.ProviderAdapter {
	baseURL := os.Getenv("OMMR_TIDAL_BASE_URL")
	if baseURL == "" {
		baseURL = "https://openapi.tidal.com"
	}
	return NewWithConfig(baseURL, os.Getenv("OMMR_TIDAL_TOKEN"))
}

func NewWithConfig(baseURL, token string) adapters.ProviderAdapter {
	return catalogapi.New(catalogapi.Config{Name: ProviderName, Version: "tidal-webapi-v2", BaseURL: baseURL, Token: token,
		FetchPath: func(id string) (string, error) {
			if token == "" {
				return "", fmt.Errorf("TIDAL requires an authorized OAuth token (OMMR_TIDAL_TOKEN)")
			}
			return "/v2/tracks/" + catalogapi.EscapedID(id) + "?countryCode=US", nil
		},
		Search: func(q adapters.Query) (string, string, []byte, error) {
			if token == "" {
				return "", "", nil, fmt.Errorf("TIDAL requires an authorized OAuth token (OMMR_TIDAL_TOKEN)")
			}
			terms := q.Title
			if q.Artist != "" {
				terms = q.Artist + " " + terms
			}
			if terms == "" {
				return "", "", nil, nil
			}
			return "GET", "/v2/searchResults/" + url.PathEscape(terms) + "?countryCode=US", nil, nil
		},
	})
}
