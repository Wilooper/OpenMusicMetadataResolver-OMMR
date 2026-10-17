package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ommr/ommr/internal/cache"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/models/canonical"
)

type WaitFunc func(context.Context, string) error

type Result struct {
	Data   any
	Images []canonical.Image
}
type Enricher interface {
	Name() string
	Enrich(context.Context, canonical.Track) (*Result, error)
}
type Client struct {
	HTTP  *http.Client
	Wait  WaitFunc
	Cache cache.Cache
}

func NewClient(wait WaitFunc) *Client {
	return &Client{HTTP: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Wait: wait}
}
func (c *Client) Request(ctx context.Context, provider, method, endpoint, token string, form url.Values, target any) error {
	cacheKey := ""
	if method == http.MethodGet && c.Cache != nil {
		sum := sha256.Sum256([]byte(provider + "\n" + endpoint + "\n" + token))
		cacheKey = "raw:catalog:" + hex.EncodeToString(sum[:])
		if data, found, err := c.Cache.Get(ctx, cacheKey); err == nil && found {
			if json.Unmarshal(data, target) == nil {
				return nil
			}
		}
	}
	if c.Wait != nil {
		if err := c.Wait(ctx, provider); err != nil {
			return err
		}
	}
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, body)
	if err != nil {
		return fmt.Errorf("invalid catalog request")
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "OMMR/1.0 (Open Music Metadata Resolver)")
	if token != "" {
		req.Header.Set("Authorization", token)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	client := c.HTTP
	if provider == "coverartarchive" {
		copyClient := *c.HTTP
		copyClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
			host := next.URL.Hostname()
			if len(via) > 3 || next.URL.Scheme != "https" || next.URL.User != nil || next.URL.Port() != "" || (host != "archive.org" && !strings.HasSuffix(host, ".archive.org")) {
				return fmt.Errorf("untrusted artwork redirect")
			}
			return nil
		}
		client = &copyClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("catalog request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("catalog HTTP %d", resp.StatusCode)
	}
	const maxBody = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(data) > maxBody {
		return fmt.Errorf("catalog response exceeds limit or cannot be read")
	}
	if err = json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("invalid catalog JSON")
	}
	var envelope map[string]any
	_ = json.Unmarshal(data, &envelope)
	applicationError := envelope["error"] != nil || envelope["errors"] != nil || envelope["status"] == "error"
	if cacheKey != "" && !applicationError {
		_ = c.Cache.Set(ctx, cacheKey, data, 24*time.Hour)
	}
	return nil
}
func exact(a, b string) bool {
	return strings.EqualFold(strings.Join(strings.Fields(a), " "), strings.Join(strings.Fields(b), " "))
}
func primary(track canonical.Track) string {
	if len(track.Artists) > 0 {
		return track.Artists[0].Name
	}
	return ""
}
func httpsURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}
