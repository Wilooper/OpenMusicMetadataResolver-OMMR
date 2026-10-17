package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (p *Provider) CheckCredentials(ctx context.Context) error {
	if !p.tokens.official() && p.tokens.cookie == "" && p.config.AccessToken == "" {
		return fmt.Errorf("Spotify credentials are missing")
	}
	token, err := p.tokens.Get(ctx)
	if err != nil {
		return fmt.Errorf("Spotify credential exchange failed")
	}
	endpoints := []string{apiBaseURL + "/tracks/4cOdK2wGLETKBW3PvgPWqT"}
	if !p.tokens.official() {
		endpoints = append([]string{apiBaseURL + "/me"}, endpoints...)
	}
	for _, endpoint := range endpoints {
		req, _ := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := p.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("Spotify credential probe failed")
		}
		if resp.StatusCode != 200 {
			resp.Body.Close()
			return fmt.Errorf("Spotify credential probe status %d", resp.StatusCode)
		}
		var data struct {
			ID string `json:"id"`
		}
		err = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data)
		resp.Body.Close()
		if err != nil || data.ID == "" {
			return fmt.Errorf("Spotify credential probe returned no identity")
		}
	}

	return nil
}
func (t *tokenManager) fetchCookieToken(ctx context.Context) (string, int, error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://open.spotify.com/api/token?reason=transport&productType=web-player", nil)
	req.Header.Set("Cookie", t.cookie)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := t.client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Spotify cookie exchange failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", 0, fmt.Errorf("Spotify cookie exchange status %d; import a current browser session token if a challenge is required", resp.StatusCode)
	}
	var data struct {
		Token     string `json:"accessToken"`
		Anonymous bool   `json:"isAnonymous"`
		Expiry    int64  `json:"accessTokenExpirationTimestampMs"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data) != nil || data.Token == "" || data.Anonymous {
		return "", 0, fmt.Errorf("Spotify cookie did not establish an authenticated session")
	}
	ttl := int(time.Until(time.UnixMilli(data.Expiry)).Seconds())
	if ttl <= 0 {
		return "", 0, fmt.Errorf("Spotify cookie returned an expired session")
	}
	return data.Token, ttl, nil
}
func validCookie(raw string) bool {
	if strings.ContainsAny(raw, "\r\n") {
		return false
	}
	for _, cookie := range (&http.Request{Header: http.Header{"Cookie": []string{raw}}}).Cookies() {
		if cookie.Name == "sp_dc" && cookie.Value != "" {
			return true
		}
	}
	return false
}
