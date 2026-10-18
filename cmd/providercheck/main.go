package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/adapters/deezer"
	"github.com/ommr/ommr/internal/adapters/jiosaavn"
	"github.com/ommr/ommr/internal/adapters/musicbrainz"
	"github.com/ommr/ommr/internal/diagnostics"
)

func main() {
	provider := flag.String("provider", "", "jiosaavn, deezer or musicbrainz (required)")
	artist := flag.String("artist", "Karan Aujla", "search artist")
	title := flag.String("title", "Boyfriend", "search title")
	id := flag.String("id", "", "optional exact catalog ID instead of search")
	idType := flag.String("id-type", "", "optional ID kind; JioSaavn: jiosaavn_track_id, jiosaavn_share or jiosaavn_url")
	routes := flag.Bool("routes", false, "also compare fixed public homepage/search protocol routes")
	flag.Parse()
	if *provider != "jiosaavn" && *provider != "deezer" && *provider != "musicbrainz" {
		fmt.Fprintln(os.Stderr, "provider must be jiosaavn, deezer or musicbrainz")
		os.Exit(2)
	}
	if strings.TrimSpace(*title) == "" {
		fmt.Fprintln(os.Stderr, "title is required")
		os.Exit(2)
	}
	transport := &diagnostics.Transport{Base: http.DefaultTransport}
	// This is an isolated executable. Install the observer before constructing
	// adapters so their normal API/fallback clients use it, without new server hooks.
	http.DefaultTransport = transport
	var adapter adapters.ProviderAdapter
	switch *provider {
	case "jiosaavn":
		adapter = jiosaavn.New()
	case "deezer":
		adapter = deezer.New()
	case "musicbrainz":
		adapter = musicbrainz.New()
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 65*time.Second)
	defer cancel()
	report := diagnostics.Run(ctx, adapter, transport, adapters.Query{Artist: *artist, Title: *title}, diagnostics.Options{Routes: *routes, ID: *id, IDType: *idType})
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		os.Exit(1)
	}
	if report.AdapterStatus != "metadata_available" {
		os.Exit(1)
	}
}
