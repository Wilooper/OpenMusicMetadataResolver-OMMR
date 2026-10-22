package ytmusic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (p *Provider) CheckCredentials(ctx context.Context) error {
	if p.innerTube.cookie == "" {
		return fmt.Errorf("YouTube session is missing")
	}
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://music.youtube.com/youtubei/v1/account/account_menu", strings.NewReader(`{"context":{"client":{"clientName":"WEB_REMIX","clientVersion":"1.20251001.01.00"}}}`))
	req.Header.Set("Content-Type", "application/json")
	p.innerTube.setAuth(req)
	if req.Header.Get("Authorization") == "" {
		return fmt.Errorf("YouTube cookie is missing SAPISID")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("YouTube session probe failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("YouTube session probe status %d", resp.StatusCode)
	}
	var body map[string]any
	if json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&body) != nil {
		return fmt.Errorf("invalid YouTube account response")
	}
	actions, _ := body["actions"].([]any)
	for _, action := range actions {
		value, _ := action.(map[string]any)
		for _, key := range []string{"openPopupAction", "popup", "multiPageMenuRenderer", "header", "activeAccountHeaderRenderer"} {
			value, _ = value[key].(map[string]any)
		}
		if value != nil {
			name, _ := value["accountName"].(map[string]any)
			runs, _ := name["runs"].([]any)
			if len(runs) > 0 {
				run, _ := runs[0].(map[string]any)
				if text, _ := run["text"].(string); text != "" {
					return nil
				}
			}
		}
	}
	return fmt.Errorf("YouTube session is logged out")
}
