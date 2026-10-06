package pandora

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/catalogapi"
)

const ProviderName = "pandora"

func New() adapters.ProviderAdapter {
	baseURL := os.Getenv("OMMR_PANDORA_BASE_URL")
	if baseURL == "" {
		baseURL = "https://ce.pandora.com/api/v1/graphql"
	}
	return NewWithConfig(baseURL, os.Getenv("OMMR_PANDORA_TOKEN"))
}

func NewWithConfig(baseURL, token string) adapters.ProviderAdapter {
	return catalogapi.New(catalogapi.Config{Name: ProviderName, Version: "pandora-graphql-v1", BaseURL: baseURL, Token: token,
		FetchRequest: func(id string) (string, string, []byte, error) {
			if token == "" {
				return "", "", nil, fmt.Errorf("Pandora requires an approved OAuth bearer token (OMMR_PANDORA_TOKEN)")
			}
			id = strings.TrimSpace(id)
			if !strings.HasPrefix(id, "TR:") {
				id = "TR:" + id
			}
			id = strings.ReplaceAll(id, "\\", "\\\\")
			id = strings.ReplaceAll(id, "\"", "\\\"")
			gql := `{ entity(id: "` + id + `") { ... on Track { name duration artist { name } album { name } art { url(size: WIDTH_500) } } } }`
			body, err := json.Marshal(map[string]string{"query": gql})
			return "POST", "graphql", body, err
		},
	})
}
