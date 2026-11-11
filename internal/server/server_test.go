package server

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dylfrancis/koito-card/internal/koito"
)

const (
	nameNeedingEscapes = "Rock & Roll \a<Live> \"東京\""
	nameAsDisplayed    = "Rock & Roll <Live> \"東京\""
	uncached           = time.Duration(0)
)

type fakeKoito struct {
	mu      sync.Mutex
	playing bool
	down    bool
}

func (fake *fakeKoito) set(change func(*fakeKoito)) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	change(fake)
}

func (fake *fakeKoito) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fake.mu.Lock()
	down, playing := fake.down, fake.playing
	fake.mu.Unlock()
	if down {
		http.Error(w, "bad gateway", http.StatusBadGateway)
		return
	}
	nameJSON, _ := json.Marshal(nameNeedingEscapes)
	imageJSON := `{"xs":"/image/abc/64x64.webp","small":"/image/abc/128x128.webp"}`
	trackJSON := func(titleJSON string) string {
		return fmt.Sprintf(`{"id":1,"title":%s,"artists":[{"id":2,"name":%s}],"image":%s,"listen_count":3}`, titleJSON, nameJSON, imageJSON)
	}
	query := r.URL.Query()
	var body string
	switch path := r.URL.Path; {
	case path == "/apis/web/v1/now-playing" && playing:
		body = `{"currently_playing":true,"track":` + trackJSON(`"Current One"`) + `}`
	case path == "/apis/web/v1/now-playing":
		body = `{"currently_playing":false}`
	case path == "/apis/web/v1/listens" && query.Get("period") == "":
		body = `{"items":[],"total_record_count":0}` // what Koito returns without a period
	case path == "/apis/web/v1/listens":
		body = `{"items":[{"time":"2026-11-11T19:22:36Z","track":` + trackJSON(`"Last One"`) + `}]}`
	case path == "/apis/web/v1/top/artists":
		body = fmt.Sprintf(`{"items":[{"item":{"id":4,"name":%s,"image":%s,"listen_count":33},"rank":1}]}`, nameJSON, imageJSON)
	case path == "/apis/web/v1/top/tracks", path == "/apis/web/v1/top/albums":
		body = `{"items":[{"item":` + trackJSON(string(nameJSON)) + `,"rank":1}]}`
	case path == "/apis/web/v1/stats":
		body = `{"listen_count":214,"track_count":155,"album_count":68,"artist_count":50,"minutes_listened":685}`
	case path == "/apis/web/v1/listen-activity" && query.Get("step") == "month":
		body = `{"activity":[{"start_time":"2020-11-01T00:00:00Z","listens":0},{"start_time":"2020-12-01T00:00:00Z","listens":0},` +
			`{"start_time":"2021-01-01T00:00:00Z","listens":5},{"start_time":"2022-01-01T00:00:00Z","listens":7}],"streak":3}`
	case path == "/apis/web/v1/listen-activity":
		body = `{"activity":[{"start_time":"2026-11-10T00:00:00Z","listens":28},{"start_time":"2026-11-11T00:00:00Z","listens":1}],"streak":3}`
	case strings.HasPrefix(path, "/image/"):
		w.Header().Set("Content-Type", "image/webp")
		io.WriteString(w, "RIFF\x00\x00\x00\x00WEBPVP8 ")
		return
	default:
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, body)
}

func newServerWithFakeKoito(t *testing.T, cacheTTL time.Duration) (*fakeKoito, *httptest.Server) {
	t.Helper()
	fake := &fakeKoito{}
	upstream := httptest.NewServer(fake)
	t.Cleanup(upstream.Close)
	cards := httptest.NewServer(New(koito.New(upstream.URL, cacheTTL, "koito-card-test"), 300))
	t.Cleanup(cards.Close)
	return fake, cards
}

type renderedCard struct {
	status           int
	header           http.Header
	body             string
	texts            []string
	imageHrefs       []string
	imageHeight      string
	backgroundHeight string
}

func attribute(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}

func (rendered renderedCard) hasText(want string) bool {
	return slices.ContainsFunc(rendered.texts, func(text string) bool { return strings.Contains(text, want) })
}

func fetchValidSVG(t *testing.T, cards *httptest.Server, path string) renderedCard {
	t.Helper()
	response, err := http.Get(cards.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	rendered := renderedCard{status: response.StatusCode, header: response.Header, body: string(body)}
	if contentType := response.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "image/svg+xml") {
		t.Fatalf("GET %s: Content-Type %q, want image/svg+xml", path, contentType)
	}
	decoder := xml.NewDecoder(bytes.NewReader(body))
	textDepth, textContent := 0, ""
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("GET %s: invalid SVG: %v", path, err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			if token.Name.Local == "text" {
				textDepth, textContent = 1, ""
			} else if textDepth > 0 {
				textDepth++
			}
			switch {
			case token.Name.Local == "svg":
				rendered.imageHeight = attribute(token, "height")
			case token.Name.Local == "rect" && attribute(token, "class") == "background":
				rendered.backgroundHeight = attribute(token, "height")
			case token.Name.Local == "image":
				rendered.imageHrefs = append(rendered.imageHrefs, attribute(token, "href"))
			}
		case xml.CharData:
			if textDepth > 0 {
				textContent += string(token)
			}
		case xml.EndElement:
			if textDepth > 0 {
				if textDepth--; textDepth == 0 {
					rendered.texts = append(rendered.texts, textContent)
				}
			}
		}
	}
	return rendered
}

func TestCards(t *testing.T) {
	_, cards := newServerWithFakeKoito(t, time.Minute)
	for _, tc := range []struct {
		path, cacheControl, text string
	}{
		{"/now-playing.svg", "no-store", nameAsDisplayed},
		{"/top.svg", "max-age=300", nameAsDisplayed},
		{"/top.svg?type=tracks&period=month&limit=10", "max-age=300", nameAsDisplayed},
		{"/top.svg?type=albums&period=all_time&theme=dark&accent=ff5500", "max-age=300", nameAsDisplayed},
		{"/stats.svg", "max-age=300", "PLAYS"},
		{"/stats.svg?period=day&border=false", "max-age=300", "PLAYS"},
		{"/stats.svg?period=year&theme=light", "max-age=300", "PLAYS"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rendered := fetchValidSVG(t, cards, tc.path)
			if rendered.status != http.StatusOK {
				t.Fatalf("status %d, want 200", rendered.status)
			}
			if cacheControl := rendered.header.Get("Cache-Control"); !strings.Contains(cacheControl, tc.cacheControl) {
				t.Errorf("Cache-Control %q, want it to contain %q", cacheControl, tc.cacheControl)
			}
			if !rendered.hasText(tc.text) {
				t.Errorf("no text containing %q in %q", tc.text, rendered.texts)
			}
			for _, href := range rendered.imageHrefs {
				if !strings.HasPrefix(href, "data:image/webp;base64,") {
					t.Errorf("image href %.40q is not an embedded data URI", href)
				}
			}
		})
	}
}

func TestNowPlayingFallsBackToLastListen(t *testing.T) {
	fake, cards := newServerWithFakeKoito(t, uncached)

	rendered := fetchValidSVG(t, cards, "/now-playing.svg")
	if !rendered.hasText("LAST PLAYED") || !rendered.hasText("Last One") {
		t.Errorf("not playing: got %q, want the last listen", rendered.texts)
	}
	if len(rendered.imageHrefs) == 0 {
		t.Error("not playing: no cover art")
	}

	fake.set(func(state *fakeKoito) { state.playing = true })
	rendered = fetchValidSVG(t, cards, "/now-playing.svg")
	if !rendered.hasText("NOW PLAYING") || !rendered.hasText("Current One") {
		t.Errorf("playing: got %q, want the current track", rendered.texts)
	}
}

func TestInvalidOptions(t *testing.T) {
	_, cards := newServerWithFakeKoito(t, time.Minute)
	for _, path := range []string{
		"/top.svg?theme=neon",
		"/top.svg?type=genres",
		"/top.svg?limit=11",
		"/stats.svg?period=decade",
		"/stats.svg?accent=red",
		"/stats.svg?background=navy",
		"/stats.svg?margin_bottom=-5",
		"/stats.svg?hide=bars",
		"/stats.svg?title=" + strings.Repeat("a", maxTitleLength+1),
		"/now-playing.svg?hide=lyrics",
		"/top.svg?height=tall",
	} {
		t.Run(path, func(t *testing.T) {
			rendered := fetchValidSVG(t, cards, path)
			if rendered.status != http.StatusBadRequest {
				t.Errorf("status %d, want 400", rendered.status)
			}
			if !rendered.hasText("Invalid card URL") {
				t.Errorf("got %q, want the invalid-URL card", rendered.texts)
			}
		})
	}
}

func TestKoitoUnavailable(t *testing.T) {
	t.Run("serves the last good copy", func(t *testing.T) {
		fake, cards := newServerWithFakeKoito(t, uncached)
		if rendered := fetchValidSVG(t, cards, "/top.svg"); !rendered.hasText(nameAsDisplayed) {
			t.Fatalf("Koito up: got %q", rendered.texts)
		}
		fake.set(func(state *fakeKoito) { state.down = true })
		rendered := fetchValidSVG(t, cards, "/top.svg")
		if rendered.status != http.StatusOK || !rendered.hasText(nameAsDisplayed) {
			t.Errorf("Koito down: status %d, texts %q; want the cached top list", rendered.status, rendered.texts)
		}
	})
	t.Run("shows an error card when nothing is cached", func(t *testing.T) {
		fake, cards := newServerWithFakeKoito(t, uncached)
		fake.set(func(state *fakeKoito) { state.down = true })
		rendered := fetchValidSVG(t, cards, "/stats.svg")
		if !rendered.hasText("Couldn't load Koito stats") {
			t.Errorf("got %q, want the error card", rendered.texts)
		}
		if cacheControl := rendered.header.Get("Cache-Control"); cacheControl != "no-store" {
			t.Errorf("Cache-Control %q; an error card must not be cached", cacheControl)
		}
	})
}

func TestAllTimeChartStartsAtFirstListen(t *testing.T) {
	_, cards := newServerWithFakeKoito(t, time.Minute)
	labels := fetchValidSVG(t, cards, "/stats.svg?period=all_time").texts
	if !slices.Contains(labels, "2021") || !slices.Contains(labels, "2022") || slices.Contains(labels, "2020") {
		t.Errorf("chart labels %q; want 2021 to 2022, skipping the empty 2020 months", labels)
	}
}

func TestMarginBottomAddsSpaceBelowTheCard(t *testing.T) {
	_, cards := newServerWithFakeKoito(t, time.Minute)
	plain := fetchValidSVG(t, cards, "/stats.svg")
	spaced := fetchValidSVG(t, cards, "/stats.svg?margin_bottom=40")
	if spaced.backgroundHeight != plain.backgroundHeight {
		t.Errorf("card height %s, want it unchanged at %s", spaced.backgroundHeight, plain.backgroundHeight)
	}
	if plain.imageHeight != "208" || spaced.imageHeight != "248" {
		t.Errorf("image heights %s and %s, want 208 and 248", plain.imageHeight, spaced.imageHeight)
	}
}

func TestCardOptions(t *testing.T) {
	_, cards := newServerWithFakeKoito(t, time.Minute)
	for _, tc := range []struct {
		path         string
		shows        []string
		hides        []string
		cardHeight   string
		bodyContains []string
		noImages     bool
	}{
		{path: "/stats.svg?title=My%20week", shows: []string{"My week"}, hides: []string{"Listening stats"}},
		{path: "/top.svg?title=On%20repeat", shows: []string{"On repeat"}, hides: []string{"Top artists"}},
		{path: "/stats.svg?hide=streak,period", hides: []string{"3-day streak", "Past 7 days"}},
		{path: "/stats.svg?hide=chart", hides: []string{"Nov 10"}, cardHeight: "118"},
		{path: "/top.svg?type=tracks&hide=plays&hide=art", hides: []string{"3 plays"}, noImages: true},
		{path: "/now-playing.svg?hide=art,backdrop", shows: []string{"LAST PLAYED"}, noImages: true},
		{path: "/top.svg?height=400", cardHeight: "400"},
		{path: "/top.svg?background=123456&text=abcdef", bodyContains: []string{"#123456", "#abcdef"}},
	} {
		t.Run(tc.path, func(t *testing.T) {
			rendered := fetchValidSVG(t, cards, tc.path)
			if rendered.status != http.StatusOK {
				t.Fatalf("status %d, want 200; texts %q", rendered.status, rendered.texts)
			}
			for _, want := range tc.shows {
				if !rendered.hasText(want) {
					t.Errorf("no text %q in %q", want, rendered.texts)
				}
			}
			for _, unwanted := range tc.hides {
				if rendered.hasText(unwanted) {
					t.Errorf("text %q should be hidden; texts %q", unwanted, rendered.texts)
				}
			}
			if tc.cardHeight != "" && rendered.backgroundHeight != tc.cardHeight {
				t.Errorf("card height %s, want %s", rendered.backgroundHeight, tc.cardHeight)
			}
			for _, want := range tc.bodyContains {
				if !strings.Contains(rendered.body, want) {
					t.Errorf("SVG doesn't contain %q", want)
				}
			}
			if tc.noImages && len(rendered.imageHrefs) > 0 {
				t.Errorf("got %d images, want none", len(rendered.imageHrefs))
			}
		})
	}
}
