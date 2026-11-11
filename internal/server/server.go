package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/dylfrancis/koito-card/internal/card"
	"github.com/dylfrancis/koito-card/internal/koito"
)

type Server struct {
	koito        *koito.Client
	cacheSeconds int
}

type cachePolicy int

const (
	cacheable cachePolicy = iota
	neverCache
)

type renderCard func(ctx context.Context, query url.Values, options card.Options) (string, error)

func New(client *koito.Client, cacheSeconds int) http.Handler {
	s := &Server{koito: client, cacheSeconds: cacheSeconds}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /now-playing.svg", s.serveCard(s.nowPlayingCard, neverCache))
	mux.HandleFunc("GET /top.svg", s.serveCard(s.topCard, cacheable))
	mux.HandleFunc("GET /stats.svg", s.serveCard(s.statsCard, cacheable))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, "ok\n")
	})
	mux.HandleFunc("GET /{$}", servePreview)
	return mux
}

func (s *Server) serveCard(render renderCard, policy cachePolicy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		status := http.StatusOK
		cacheControl := fmt.Sprintf("public, max-age=%d", s.cacheSeconds)
		if policy == neverCache {
			cacheControl = "no-cache, no-store, must-revalidate, max-age=0"
		}

		options, err := parseOptions(query)
		var svg string
		if err == nil {
			svg, err = render(r.Context(), query, options)
		}
		var invalid badRequest
		switch {
		case errors.As(err, &invalid):
			status, cacheControl = http.StatusBadRequest, "no-store"
			svg = card.Error("Invalid card URL", invalid.Error(), options)
		case err != nil:
			log.Printf("%s: %v", r.URL.Path, err)
			cacheControl = "no-store"
			svg = card.Error("Couldn't load Koito stats", "The Koito server didn't answer. The card will try again on the next view.", options)
		}

		hash := sha256.Sum256([]byte(svg))
		etag := `"` + hex.EncodeToString(hash[:8]) + `"`
		header := w.Header()
		header.Set("Cache-Control", cacheControl)
		header.Set("ETag", etag)
		if status == http.StatusOK && r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		header.Set("Content-Type", "image/svg+xml; charset=utf-8")
		header.Set("Content-Security-Policy", "default-src 'none'; img-src data:; style-src 'unsafe-inline'")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Content-Length", strconv.Itoa(len(svg)))
		w.WriteHeader(status)
		io.WriteString(w, svg)
	}
}

func (s *Server) nowPlayingCard(ctx context.Context, query url.Values, options card.Options) (string, error) {
	var err error
	if options.Hidden, err = parseHidden(query, "art", "backdrop"); err != nil {
		return "", err
	}
	listen, playing, err := s.koito.NowPlaying(ctx)
	if err != nil {
		return "", err
	}
	var artURI, backdropURI string
	if listen != nil {
		if !options.Hidden["art"] {
			artURI = s.koito.ImageDataURI(ctx, listen.Track.Image, koito.SizeSmall)
		}
		if !options.Hidden["backdrop"] {
			backdropURI = s.koito.ImageDataURI(ctx, listen.Track.Image, koito.SizeXS)
		}
	}
	return card.NowPlaying(listen, playing, artURI, backdropURI, options, time.Now()), nil
}

func (s *Server) topCard(ctx context.Context, query url.Values, options card.Options) (string, error) {
	kind, err := parseChoice(query, "type", "artists", "artists", "albums", "tracks")
	if err != nil {
		return "", err
	}
	period, err := parsePeriod(query, "week")
	if err != nil {
		return "", err
	}
	limit, err := parseInt(query, "limit", 5, 1, 10)
	if err != nil {
		return "", err
	}
	minHeight, err := parseInt(query, "height", 0, 0, 1500)
	if err != nil {
		return "", err
	}
	if options.Title, err = parseTitle(query); err != nil {
		return "", err
	}
	if options.Hidden, err = parseHidden(query, "period", "rank", "art", "plays", "bars"); err != nil {
		return "", err
	}
	items, err := s.koito.Top(ctx, kind, period, limit)
	if err != nil {
		return "", err
	}
	artURIs := make([]string, len(items))
	if !options.Hidden["art"] {
		images := make([]koito.Image, len(items))
		for i, ranked := range items {
			images[i] = ranked.Item.Image
		}
		artURIs = s.koito.ImageDataURIs(ctx, images, koito.SizeSmall)
	}
	return card.Top(kind, period, items, artURIs, minHeight, options), nil
}

func (s *Server) statsCard(ctx context.Context, query url.Values, options card.Options) (string, error) {
	period, err := parsePeriod(query, "week")
	if err != nil {
		return "", err
	}
	if options.Title, err = parseTitle(query); err != nil {
		return "", err
	}
	if options.Hidden, err = parseHidden(query, "streak", "chart", "period"); err != nil {
		return "", err
	}
	totals, err := s.koito.Stats(ctx, period)
	if err != nil {
		return "", err
	}
	var chart *card.Series
	streakDays := 0
	if !options.Hidden["chart"] || !options.Hidden["streak"] {
		if chart, streakDays, err = s.activityChart(ctx, period); err != nil {
			return "", err
		}
	}
	if options.Hidden["chart"] {
		chart = nil
	}
	return card.Stats(period, totals, chart, streakDays, options), nil
}

func (s *Server) activityChart(ctx context.Context, period string) (chart *card.Series, streakDays int, err error) {
	step, steps := "day", 1
	switch period {
	case "week":
		steps = 7
	case "month":
		steps = 30
	case "year":
		step, steps = "month", 12
	case "all_time":
		step, steps = "month", 120
	}
	activity, err := s.koito.Activity(ctx, step, steps)
	if err != nil {
		return nil, 0, err
	}
	switch period {
	case "day":
		return nil, activity.StreakDays, nil
	case "week", "month":
		return toSeries(activity.Buckets, "Jan 2"), activity.StreakDays, nil
	case "year":
		return toSeries(activity.Buckets, "Jan 2006"), activity.StreakDays, nil
	default:
		return toSeries(sumByYear(activity.Buckets), "2006"), activity.StreakDays, nil
	}
}

func toSeries(buckets []koito.Bucket, labelLayout string) *card.Series {
	if len(buckets) == 0 {
		return nil
	}
	series := &card.Series{
		From: buckets[0].Start.UTC().Format(labelLayout),
		To:   buckets[len(buckets)-1].Start.UTC().Format(labelLayout),
	}
	for _, bucket := range buckets {
		series.Values = append(series.Values, bucket.Listens)
	}
	return series
}

// sumByYear starts at the first year with listens; Koito has no yearly step.
func sumByYear(months []koito.Bucket) []koito.Bucket {
	var years []koito.Bucket
	for _, month := range months {
		year := month.Start.UTC().Year()
		if len(years) == 0 || years[len(years)-1].Start.Year() != year {
			if len(years) == 0 && month.Listens == 0 {
				continue
			}
			years = append(years, koito.Bucket{Start: time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)})
		}
		years[len(years)-1].Listens += month.Listens
	}
	return years
}
