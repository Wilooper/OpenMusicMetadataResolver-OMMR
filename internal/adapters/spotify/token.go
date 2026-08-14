package spotify

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/models/provider"
)

// anonymousSpotifyWebClientID is the public client identifier used by the
// open.spotify.com web player. It is used only to obtain a read-only,
// anonymous access token for public metadata (the unofficial method).
const anonymousSpotifyWebClientID = "65b708073fc0480ea92a077233ca87bd"

// tokenManager caches an OAuth access token for the Spotify Web API and
// transparently falls back between the official client-credentials flow
// (when developer credentials are configured) and the anonymous web-player
// token endpoint (zero-key mode).
type tokenManager struct {
	mu       sync.Mutex
	client   *http.Client
	clientID string
	secret   string

	token  string
	expiry time.Time
}

func newTokenManager(client *http.Client, clientID, secret string) *tokenManager {
	return &tokenManager{
		client:   client,
		clientID: clientID,
		secret:   secret,
	}
}

// official reports whether the manager is configured with developer credentials.
func (t *tokenManager) official() bool {
	return t.clientID != "" && t.secret != ""
}

// Get returns a valid Bearer token, refreshing it when expired.
func (t *tokenManager) Get(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.token != "" && time.Now().Before(t.expiry) {
		return t.token, nil
	}

	token, expiresIn, err := t.fetchToken(ctx)
	if err != nil {
		return "", err
	}

	t.token = token
	t.expiry = time.Now().Add(time.Duration(expiresIn) * time.Second)
	return t.token, nil
}

func (t *tokenManager) fetchToken(ctx context.Context) (string, int, error) {
	if t.official() {
		return t.fetchOfficialToken(ctx)
	}
	return t.fetchAnonymousToken(ctx)
}

// fetchOfficialToken performs the client-credentials OAuth flow.
func (t *tokenManager) fetchOfficialToken(ctx context.Context) (string, int, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.spotify.com/api/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(t.clientID+":"+t.secret)))

	resp, err := t.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("spotify accounts token status %d: %s", resp.StatusCode, string(body))
	}

	var tok provider.SpotifyWebTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", 0, err
	}
	if tok.AccessToken == "" {
		return "", 0, fmt.Errorf("spotify accounts token response empty")
	}
	expiresIn := tok.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	return tok.AccessToken, expiresIn, nil
}

// fetchAnonymousToken obtains a read-only token from the public web player
// endpoint without requiring developer credentials.
func (t *tokenManager) fetchAnonymousToken(ctx context.Context) (string, int, error) {
	reqURL := "https://open.spotify.com/get_access_token?reason=transport&productType=web_player"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("spotify anonymous token status %d: %s", resp.StatusCode, string(body))
	}

	var tok provider.SpotifyAnonymousTokenResponse
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", 0, err
	}
	if tok.AccessToken == "" {
		return "", 0, fmt.Errorf("spotify anonymous token response empty")
	}

	expiresIn := 3600
	if tok.AccessTokenExpirationTimestampMs > 0 {
		ttl := time.Until(time.UnixMilli(tok.AccessTokenExpirationTimestampMs))
		if ttl > 30*time.Second {
			expiresIn = int(ttl.Seconds())
		}
	}
	return tok.AccessToken, expiresIn, nil
}
