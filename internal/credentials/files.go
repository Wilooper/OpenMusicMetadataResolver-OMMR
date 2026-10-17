package credentials

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Session struct {
	Cookie         string
	AccessToken    string
	DeveloperToken string
	UserToken      string
	ExpiresAt      int64
	Values         map[string]string
}

func ReadPrivate(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("credential file must be a regular owner-only file (chmod 600)")
	}
	if info.Size() > 1<<20 {
		return nil, fmt.Errorf("credential file exceeds 1 MiB")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("credential file cannot be read")
	}
	return data, nil
}
func Parse(data []byte, domain string) (Session, error) {
	session := Session{Values: map[string]string{}}
	text := strings.TrimSpace(string(data))
	var object map[string]json.RawMessage
	if strings.HasPrefix(text, "{") {
		if json.Unmarshal(data, &object) != nil {
			return session, fmt.Errorf("invalid credential JSON")
		}
		for key, value := range object {
			var s string
			if json.Unmarshal(value, &s) == nil {
				session.Values[strings.ToLower(key)] = s
			}
		}
		session.Cookie = session.Values["cookie"]
		if session.Cookie == "" {
			session.Cookie = session.Values["cookies"]
		}
		session.AccessToken = session.Values["access_token"]
		if session.AccessToken == "" {
			session.AccessToken = session.Values["accesstoken"]
		}
		session.DeveloperToken = session.Values["developer_token"]
		session.UserToken = session.Values["music_user_token"]
		_ = json.Unmarshal(object["expires_at"], &session.ExpiresAt)
		if session.ExpiresAt == 0 {
			var millis int64
			_ = json.Unmarshal(object["accessTokenExpirationTimestampMs"], &millis)
			session.ExpiresAt = millis / 1000
		}
		var headers map[string]string
		if json.Unmarshal(object["headers"], &headers) == nil {
			for name, value := range headers {
				switch strings.ToLower(name) {
				case "cookie":
					if session.Cookie == "" {
						session.Cookie = value
					}
				case "authorization":
					if strings.HasPrefix(value, "Bearer ") {
						if domain == "apple.com" {
							session.DeveloperToken = strings.TrimPrefix(value, "Bearer ")
						} else {
							session.AccessToken = strings.TrimPrefix(value, "Bearer ")
						}
					}
				case "music-user-token":
					session.UserToken = value
				}
			}
		}
		// ytmusicapi-style header JSON has headers at the top level.
		for key, value := range session.Values {
			switch key {
			case "authorization":
				if strings.HasPrefix(value, "Bearer ") {
					if domain == "apple.com" {
						session.DeveloperToken = strings.TrimPrefix(value, "Bearer ")
					} else {
						session.AccessToken = strings.TrimPrefix(value, "Bearer ")
					}
				}
			case "music-user-token":
				session.UserToken = value
			}
		}
		if raw := object["cookies"]; len(raw) > 0 && raw[0] == '[' {
			cookie, err := parseCookies(raw, domain)
			if err != nil {
				return session, err
			}
			session.Cookie = cookie
		}
	} else if strings.HasPrefix(text, "[") {
		cookie, err := parseCookies(data, domain)
		if err != nil {
			return session, err
		}
		session.Cookie = cookie
	} else if strings.Contains(text, "\t") {
		var parts []string
		for _, line := range strings.Split(text, "\n") {
			if strings.HasPrefix(line, "#HttpOnly_") {
				line = strings.TrimPrefix(line, "#HttpOnly_")
			} else if strings.HasPrefix(line, "#") {
				continue
			}
			fields := strings.Split(strings.TrimSpace(line), "\t")
			if len(fields) == 7 && domainAllowed(fields[0], domain) {
				parts = append(parts, fields[5]+"="+fields[6])
			}
		}
		session.Cookie = strings.Join(parts, "; ")
	} else {
		session.Cookie = text
	}
	for _, value := range []string{session.Cookie, session.AccessToken, session.DeveloperToken, session.UserToken} {
		if strings.ContainsAny(value, "\r\n") {
			return Session{}, fmt.Errorf("credential header contains a line break")
		}
	}
	if session.UserToken == "" && domain == "apple.com" {
		for _, cookie := range (&http.Request{Header: http.Header{"Cookie": []string{session.Cookie}}}).Cookies() {
			if cookie.Name == "media-user-token" || cookie.Name == "music-user-token" {
				session.UserToken, _ = url.PathUnescape(cookie.Value)
				break
			}
		}
	}
	return session, nil
}
func parseCookies(raw []byte, domain string) (string, error) {
	var cookies []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
	}
	if json.Unmarshal(raw, &cookies) != nil {
		return "", fmt.Errorf("invalid cookie array")
	}
	parts := []string{}
	for _, cookie := range cookies {
		if !domainAllowed(cookie.Domain, domain) {
			continue
		}
		if strings.ContainsAny(cookie.Name+cookie.Value, "\r\n;") {
			return "", fmt.Errorf("invalid cookie value")
		}
		parts = append(parts, cookie.Name+"="+cookie.Value)
	}
	return strings.Join(parts, "; "), nil
}
func domainAllowed(host, domain string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), ".")
	return host == domain || strings.HasSuffix(host, "."+domain)
}
func Load(path, domain string) (Session, error) {
	data, err := ReadPrivate(path)
	if err != nil {
		return Session{}, err
	}
	return Parse(data, domain)
}

// Discover reads only documented filenames inside the operator's credential directory.
func Discover(dir string) (map[string]string, error) {
	result := map[string]string{}
	if dir == "" {
		return result, nil
	}
	info, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return result, nil
	}
	if err != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("credential directory must be owner-only (chmod 700)")
	}
	domains := map[string]string{"spotify": "spotify.com", "applemusic": "apple.com", "ytmusic": "youtube.com", "qobuz": "qobuz.com", "tidal": "tidal.com", "amazonmusic": "amazon.com", "pandora": "pandora.com", "lastfm": "last.fm", "discogs": "discogs.com", "acoustid": "acoustid.org"}
	names := map[string]map[string]string{
		"spotify": {"client_id": "spotify_client_id", "client_secret": "spotify_client_secret"},
		"qobuz":   {"app_id": "qobuz_app_id", "token": "qobuz_token"}, "tidal": {"token": "tidal_token"},
		"amazonmusic": {"token": "amazon_music_token", "api_key": "amazon_music_api_key"}, "pandora": {"token": "pandora_token"},
		"lastfm": {"api_key": "lastfm_api_key"}, "discogs": {"token": "discogs_token"}, "acoustid": {"api_key": "acoustid_api_key"},
	}
	for provider, domain := range domains {
		path := filepath.Join(dir, provider+".json")
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			continue
		}
		session, err := Load(path, domain)
		if err != nil {
			return nil, err
		}
		if provider == "spotify" || provider == "applemusic" || provider == "ytmusic" {
			result[provider+"_session_file"] = path
			if session.Cookie != "" {
				result[provider+"_cookie"] = session.Cookie
			}
			if provider == "spotify" {
				if session.AccessToken != "" {
					result["spotify_access_token"] = session.AccessToken
				}
				if session.ExpiresAt > 0 {
					result["spotify_token_expiry"] = fmt.Sprint(session.ExpiresAt)
				}
			}
			if provider == "applemusic" {
				if session.DeveloperToken != "" {
					result["apple_developer_token"] = session.DeveloperToken
				}
				if session.UserToken != "" {
					result["apple_music_user_token"] = session.UserToken
				}
			}
		}
		for field, key := range names[provider] {
			if value := session.Values[field]; value != "" {
				result[key] = value
			}
		}
	}
	return result, nil
}
