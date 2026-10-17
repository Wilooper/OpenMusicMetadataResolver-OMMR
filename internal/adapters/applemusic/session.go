package applemusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"net/url"
)

func (p *Provider) CheckCredentials(ctx context.Context) error {
	if p.catalogToken == "" {
		return fmt.Errorf("Apple session requires a developer or browser bearer token")
	}
	if p.userToken != "" {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.catalogBaseURL+"/v1/me/storefront", nil)
		req.Header.Set("Authorization", "Bearer "+p.catalogToken)
		req.Header.Set("Music-User-Token", p.userToken)
		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("Apple session probe failed")
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("Apple session probe status %d", resp.StatusCode)
		}
		var body struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body) != nil || len(body.Data) == 0 || body.Data[0].ID == "" {
			return fmt.Errorf("Apple session probe returned no storefront")
		}
	}
	data, err := p.catalogRequest(ctx, "/search", url.Values{"term": {"Daft Punk Get Lucky"}, "types": {"songs"}, "limit": {"1"}})
	if err != nil {
		return err
	}
	var payload struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if json.Unmarshal(data, &payload) != nil || len(payload.Results["songs"]) == 0 {
		return fmt.Errorf("Apple catalog probe returned no search capability")
	}
	return nil
}
