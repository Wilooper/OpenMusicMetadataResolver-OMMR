package webdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Fallback struct {
	mu      sync.Mutex
	retryAt time.Time
	client  *http.Client
	host    string
	limiter *rate.Limiter
	wait    func(context.Context) error
}

func NewFallback(host string) *Fallback {
	return &Fallback{host: host, client: &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) > 3 || next.URL.Scheme != "https" || next.URL.User != nil || next.URL.Port() != "" || next.URL.Hostname() != host {
			return errors.New("untrusted public catalog redirect")
		}
		return nil
	}}, limiter: rate.NewLimiter(1, 1)}
}
func (f *Fallback) Try(ctx context.Context, endpoint string, r Request) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Hostname() != f.host || u.User != nil || u.Port() != "" {
		return nil, errors.New("invalid public catalog endpoint")
	}
	f.mu.Lock()
	if time.Now().Before(f.retryAt) {
		f.mu.Unlock()
		return nil, ErrUnavailable
	}
	f.retryAt = time.Now().Add(time.Minute)
	f.mu.Unlock()
	if f.wait != nil {
		if err = f.wait(ctx); err != nil {
			return nil, err
		}
	}
	if err = f.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, ErrNoData
	}
	req.Header.Set("User-Agent", "OMMR/1.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR)")
	req.Header.Set("Accept", "text/html")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, errors.New("public catalog request failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("public catalog HTTP %d", resp.StatusCode)
	}
	body, err := Read(resp, r)
	if err == nil {
		f.mu.Lock()
		f.retryAt = time.Time{}
		f.mu.Unlock()
	}
	return body, err
}
func Recoverable(err error) bool { return errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNoData) }

type LimitedTransport struct {
	Base    http.RoundTripper
	Limiter *rate.Limiter
}

func (t LimitedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if err := t.Limiter.Wait(r.Context()); err != nil {
		return nil, err
	}
	return t.Base.RoundTrip(r)
}

func (f *Fallback) SetTransport(transport http.RoundTripper) { f.client.Transport = transport }

func (f *Fallback) SetWait(wait func(context.Context) error) { f.wait = wait }
