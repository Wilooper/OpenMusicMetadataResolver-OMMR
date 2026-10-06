package amazonmusic

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/catalogapi"
)

const ProviderName = "amazonmusic"

func New() adapters.ProviderAdapter {
	return NewWithConfig(os.Getenv("OMMR_AMAZON_MUSIC_BASE_URL"), os.Getenv("OMMR_AMAZON_MUSIC_TOKEN"), os.Getenv("OMMR_AMAZON_MUSIC_API_KEY"))
}

func NewWithConfig(baseURL, token, apiKey string) adapters.ProviderAdapter {
	return catalogapi.New(catalogapi.Config{Name: ProviderName, Version: "amazonmusic-webapi-v2", BaseURL: baseURL, Token: token, APIKey: apiKey,
		FetchPath: func(id string) (string, error) {
			if token == "" || apiKey == "" {
				return "", fmt.Errorf("Amazon Music requires approved API credentials (OMMR_AMAZON_MUSIC_TOKEN and OMMR_AMAZON_MUSIC_API_KEY)")
			}
			return "/v2/tracks/" + catalogapi.EscapedID(id) + "?fields[track]=title,duration,isrc,album,artists,releaseDate,label,images", nil
		},
		Search: func(q adapters.Query) (string, string, []byte, error) {
			if token == "" || apiKey == "" {
				return "", "", nil, fmt.Errorf("Amazon Music requires approved API credentials (OMMR_AMAZON_MUSIC_TOKEN and OMMR_AMAZON_MUSIC_API_KEY)")
			}
			filters := []map[string]string{}
			if q.ISRC != "" {
				filters = append(filters, map[string]string{"field": "isrc", "query": q.ISRC})
			} else {
				if q.Title != "" {
					filters = append(filters, map[string]string{"field": "name", "query": q.Title})
				}
				if q.Artist != "" {
					filters = append(filters, map[string]string{"field": "artistName", "query": q.Artist})
				}
			}
			if len(filters) == 0 {
				return "", "", nil, nil
			}
			body, err := json.Marshal(map[string]any{"searchFilters": filters, "sortBy": "relevance"})
			return "POST", "/v2/search/tracks?first=5&fields[track]=title,duration,isrc,album,artists&fields[album]=title,releaseDate,images", body, err
		},
		ExtraHeader: map[string]string{"x-marketplace": "US"},
	})
}
