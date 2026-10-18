package resolver

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/enrichment/catalog"
	"github.com/ommr/ommr/internal/merger"
	"github.com/ommr/ommr/internal/models/canonical"
)

// Configure is called once at startup, before accepting requests.
func (res *Resolver) Configure(policy merger.Policy, enrichers ...catalog.Enricher) {
	res.merger = merger.NewWithPolicy(policy)
	names := []string{}
	for _, enricher := range enrichers {
		names = append(names, enricher.Name())
	}
	sort.Strings(names)
	data, _ := json.Marshal(struct {
		Policy    merger.Policy
		Enrichers []string
	}{policy, names})
	sum := sha256.Sum256(data)
	res.cacheNamespace = "resolved:rich-v11:" + hex.EncodeToString(sum[:8]) + ":"
	res.enrichers = append([]catalog.Enricher(nil), enrichers...)
}

func (res *Resolver) enrichCatalogs(ctx context.Context, track *canonical.Track) map[string]string {
	statuses := map[string]string{}
	if track == nil {
		return statuses
	}
	type outcome struct {
		name   string
		result *catalog.Result
		err    error
	}
	results := make([]outcome, len(res.enrichers))
	var wg sync.WaitGroup
	for i, enricher := range res.enrichers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			work, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			result, err := enricher.Enrich(work, *track)
			results[i] = outcome{enricher.Name(), result, err}
		}()
	}
	wg.Wait()
	if track.Extensions == nil {
		track.Extensions = map[string]json.RawMessage{}
	}
	records := map[string]any{}
	_ = json.Unmarshal(track.Extensions["provider_data"], &records)
	images := map[string]bool{}
	for _, image := range track.Images {
		images[image.URL] = true
	}
	for _, item := range results {
		if item.err != nil {
			statuses[item.name] = "unavailable"
			continue
		}
		if item.result == nil {
			statuses[item.name] = "no_data"
			continue
		}
		statuses[item.name] = "ok"
		records[item.name] = item.result.Data
		for _, image := range item.result.Images {
			if !images[image.URL] {
				track.Images = append(track.Images, image)
				images[image.URL] = true
			}
		}
	}
	sort.Slice(track.Images, func(i, j int) bool {
		a, b := track.Images[i], track.Images[j]
		if a.Width*a.Height != b.Width*b.Height {
			return a.Width*a.Height < b.Width*b.Height
		}
		return a.URL < b.URL
	})
	track.Extensions["provider_data"], _ = json.Marshal(records)
	track.Completeness = canonical.CalculateCompleteness(*track)
	return statuses
}

func (res *Resolver) SetCacheVariant(variant string) {
	sum := sha256.Sum256([]byte(variant))
	res.cacheNamespace += hex.EncodeToString(sum[:8]) + ":"
}
