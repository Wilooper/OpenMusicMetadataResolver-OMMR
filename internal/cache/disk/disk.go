package disk

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/cache"
)

type diskItem struct {
	Value     []byte    `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DiskCache implements cache.Cache storing items on local disk in subdivided subtrees:
// - cache/raw/{provider}/
// - cache/normalized/
// - cache/resolved/
type DiskCache struct {
	baseDir string
	mu      sync.RWMutex
}

var _ cache.Cache = (*DiskCache)(nil)

// NewDiskCache creates or opens a DiskCache at baseDir.
func NewDiskCache(baseDir string) (*DiskCache, error) {
	if baseDir == "" {
		baseDir = "./cache_data"
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create disk cache base dir: %w", err)
	}

	// Pre-create standard subtrees
	subtrees := []string{
		"resolved",
		"normalized",
		filepath.Join("raw", "deezer"),
		filepath.Join("raw", "applemusic"),
		filepath.Join("raw", "musicbrainz"),
		filepath.Join("raw", "ytmusic"),
		filepath.Join("raw", "spotify"),
	}
	for _, sub := range subtrees {
		if err := os.MkdirAll(filepath.Join(baseDir, sub), 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache subpath %s: %w", sub, err)
		}
	}

	return &DiskCache{baseDir: baseDir}, nil
}

func (d *DiskCache) resolvePath(key string) string {
	// Keys can be structured like "raw:spotify:12345", "normalized:query_hash", "resolved:query_hash"
	parts := strings.Split(key, ":")
	var relPath string
	if len(parts) >= 3 && parts[0] == "raw" {
		provider := parts[1]
		filename := hashKey(strings.Join(parts[2:], ":")) + ".json"
		relPath = filepath.Join("raw", provider, filename)
	} else if len(parts) >= 2 && (parts[0] == "normalized" || parts[0] == "resolved") {
		category := parts[0]
		filename := hashKey(strings.Join(parts[1:], ":")) + ".json"
		relPath = filepath.Join(category, filename)
	} else {
		relPath = hashKey(key) + ".json"
	}

	return filepath.Join(d.baseDir, relPath)
}

func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func (d *DiskCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	path := d.resolvePath(key)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var item diskItem
	if err := json.Unmarshal(data, &item); err != nil {
		_ = os.Remove(path)
		return nil, false, nil
	}

	if time.Now().After(item.ExpiresAt) {
		_ = os.Remove(path)
		return nil, false, nil
	}

	return item.Value, true, nil
}

func (d *DiskCache) Set(ctx context.Context, key string, val []byte, ttl time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := d.resolvePath(key)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	item := diskItem{
		Value:     val,
		ExpiresAt: time.Now().Add(ttl),
	}

	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	tmpFile := path + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, path)
}

func (d *DiskCache) Delete(ctx context.Context, key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	path := d.resolvePath(key)
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (d *DiskCache) Close() error {
	return nil
}
