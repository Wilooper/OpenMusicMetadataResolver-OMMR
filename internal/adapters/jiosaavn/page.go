package jiosaavn

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/ommr/ommr/internal/adapters/webdata"
	"github.com/ommr/ommr/internal/models/canonical"
)

// ParseSongURL accepts only public song pages, never arbitrary fetch targets.
func ParseSongURL(raw string) (string, string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.RawQuery != "" || u.Fragment != "" || (u.Host != "www.jiosaavn.com" && u.Host != "jiosaavn.com") {
		return "", "", fmt.Errorf("provide an HTTPS JioSaavn song URL without query or fragment")
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "song" || parts[1] == "" || parts[1] == "." || parts[1] == ".." || !identifierPattern.MatchString(parts[2]) {
		return "", "", fmt.Errorf("invalid JioSaavn song URL")
	}
	canonical := url.URL{Scheme: "https", Host: "www.jiosaavn.com", Path: "/" + strings.Join(parts, "/")}
	return canonical.String(), parts[2], nil
}

func (p *Provider) fetchPage(ctx context.Context, rawURL string) (*canonical.TrackCandidate, error) {
	page, id, err := ParseSongURL(rawURL)
	if err != nil {
		return nil, err
	}
	if p.web != nil {
		body, err := p.web.Try(ctx, page, webdata.Request{Provider: ProviderName, Mode: "track", ID: id})
		if err == nil {
			detail, record, selectErr := selectDetail(body, id, true)
			if selectErr == nil {
				return &canonical.TrackCandidate{Provider: ProviderName, Track: p.normalize(detail), MatchScore: 1, RawResponse: record}, nil
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.FetchByID(ctx, "jiosaavn_share", id)
}
