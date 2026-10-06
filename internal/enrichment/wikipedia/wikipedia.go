package wikipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Context struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Summary     string `json:"summary,omitempty"`
	ArticleURL  string `json:"article_url"`
	Source      string `json:"source"`
	WikidataID  string `json:"wikidata_id,omitempty"`
	Revision    string `json:"revision,omitempty"`
}

type Client struct{ httpClient *http.Client }

func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}}
}

var wikidataID = regexp.MustCompile(`^Q[1-9][0-9]*$`)

// FetchFromWikidata resolves a MusicBrainz work's linked Wikidata entity to
// its English Wikipedia sitelink, then obtains contextual prose from Wikipedia.
func FetchFromWikidata(ctx context.Context, entityURL string) (*Context, error) {
	return New().FetchFromWikidata(ctx, entityURL)
}

func (c *Client) FetchFromWikidata(ctx context.Context, entityURL string) (*Context, error) {
	u, err := url.Parse(entityURL)
	if err != nil || u.Scheme != "https" || u.Host != "www.wikidata.org" {
		return nil, fmt.Errorf("unsupported Wikidata URL")
	}
	id := strings.TrimPrefix(u.Path, "/wiki/")
	if !wikidataID.MatchString(id) || u.Path != "/wiki/"+id {
		return nil, fmt.Errorf("invalid Wikidata entity ID")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.wikidata.org/wiki/Special:EntityData/"+id+".json", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OMMR/1.0 (https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR-)")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Wikidata entity status %d", resp.StatusCode)
	}
	var data struct {
		Entities map[string]struct {
			Sitelinks map[string]struct {
				Title string `json:"title"`
			} `json:"sitelinks"`
		} `json:"entities"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&data); err != nil {
		return nil, err
	}
	title := data.Entities[id].Sitelinks["enwiki"].Title
	if title == "" {
		return nil, fmt.Errorf("Wikidata entity has no English Wikipedia page")
	}
	context, err := c.Fetch(ctx, "https://en.wikipedia.org/wiki/"+url.PathEscape(strings.ReplaceAll(title, " ", "_")))
	if err == nil {
		if context.WikidataID != "" && context.WikidataID != id {
			return nil, fmt.Errorf("Wikipedia summary does not match linked Wikidata entity")
		}
		context.WikidataID = id
		context.Source = "musicbrainz_work_wikidata_link"
	}
	return context, err
}

// Fetch follows a pre-existing MusicBrainz work relationship to an English
// Wikipedia article. It never searches for a title or sends credentials.
func Fetch(ctx context.Context, articleURL string) (*Context, error) {
	return New().Fetch(ctx, articleURL)
}

func (c *Client) Fetch(ctx context.Context, articleURL string) (*Context, error) {
	u, err := url.Parse(articleURL)
	if err != nil || u.Scheme != "https" || u.Host != "en.wikipedia.org" || !strings.HasPrefix(u.EscapedPath(), "/wiki/") {
		return nil, fmt.Errorf("unsupported Wikipedia article URL")
	}
	title, err := url.PathUnescape(strings.TrimPrefix(u.EscapedPath(), "/wiki/"))
	if err != nil || title == "" || strings.Contains(title, "/") {
		return nil, fmt.Errorf("invalid Wikipedia article title")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://en.wikipedia.org/api/rest_v1/page/summary/"+url.PathEscape(title), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "OMMR/1.0 (music metadata; https://github.com/Wilooper/OpenMusicMetadataResolver-OMMR-)")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Wikipedia summary status %d", resp.StatusCode)
	}
	var data struct {
		Type        string      `json:"type"`
		WikidataID  string      `json:"wikibase_item"`
		Revision    json.Number `json:"revision"`
		Title       string      `json:"title"`
		Description string      `json:"description"`
		Extract     string      `json:"extract"`
		ContentURLs struct {
			Desktop struct {
				Page string `json:"page"`
			} `json:"desktop"`
		} `json:"content_urls"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 256<<10)).Decode(&data); err != nil {
		return nil, err
	}
	if data.Title == "" || data.Type != "standard" || strings.TrimSpace(data.Extract) == "" {
		return nil, fmt.Errorf("Wikipedia page is not a standard article with a summary")
	}
	return &Context{Title: data.Title, Description: data.Description, Summary: data.Extract, ArticleURL: articleURL, Source: "musicbrainz_work_wikipedia_link", WikidataID: data.WikidataID, Revision: data.Revision.String()}, nil
}
