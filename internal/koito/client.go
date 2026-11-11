package koito

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxJSONBytes   = 4 << 20
	maxImageBytes  = 2 << 20
	imageCacheSize = 512
	nowPlayingTTL  = 15 * time.Second
)

var embeddableImageTypes = map[string]bool{
	"image/webp": true, "image/jpeg": true, "image/png": true, "image/gif": true, "image/avif": true,
}

type Client struct {
	baseURL    string
	httpClient *http.Client
	cacheTTL   time.Duration
	userAgent  string

	mu         sync.Mutex
	responses  map[string]cachedResponse // unbounded: the server only sends validated options
	imageCache *lruCache
}

type cachedResponse struct {
	body      []byte
	fetchedAt time.Time
}

// New returns a client that caches responses for cacheTTL and serves stale copies while Koito is down.
func New(baseURL string, cacheTTL time.Duration, userAgent string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{Timeout: 5 * time.Second},
		cacheTTL:   cacheTTL,
		userAgent:  userAgent,
		responses:  map[string]cachedResponse{},
		imageCache: newLRUCache(imageCacheSize),
	}
}

// NowPlaying returns the current track, else the latest listen, or nil if there are none.
func (c *Client) NowPlaying(ctx context.Context) (listen *Listen, playing bool, err error) {
	ttl := min(nowPlayingTTL, c.cacheTTL)
	var current nowPlayingResponse
	if err := c.getJSON(ctx, "now-playing", nil, ttl, &current); err != nil {
		return nil, false, err
	}
	if current.CurrentlyPlaying && current.Track != nil {
		return &Listen{Track: *current.Track}, true, nil
	}
	// Koito returns no listens unless a period is given.
	var latest pageResponse[Listen]
	query := url.Values{"period": {"all_time"}, "limit": {"1"}}
	if err := c.getJSON(ctx, "listens", query, ttl, &latest); err != nil {
		return nil, false, err
	}
	if len(latest.Items) == 0 {
		return nil, false, nil
	}
	return &latest.Items[0], false, nil
}

func (c *Client) Top(ctx context.Context, kind, period string, limit int) ([]Ranked, error) {
	var top pageResponse[Ranked]
	query := url.Values{"period": {period}, "limit": {strconv.Itoa(limit)}}
	err := c.getJSON(ctx, "top/"+kind, query, c.cacheTTL, &top)
	return top.Items, err
}

func (c *Client) Stats(ctx context.Context, period string) (Stats, error) {
	var stats Stats
	err := c.getJSON(ctx, "stats", url.Values{"period": {period}}, c.cacheTTL, &stats)
	return stats, err
}

// Activity's step is "day" or "month".
func (c *Client) Activity(ctx context.Context, step string, steps int) (Activity, error) {
	var activity Activity
	query := url.Values{"step": {step}, "range": {strconv.Itoa(steps)}}
	err := c.getJSON(ctx, "listen-activity", query, c.cacheTTL, &activity)
	return activity, err
}

// ImageDataURI returns the image inlined as a data: URI, because an SVG shown
// through <img> can't load external files. It returns "" on failure.
func (c *Client) ImageDataURI(ctx context.Context, image Image, size Size) string {
	path := image.Path(size)
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return ""
	}
	if uri, found := c.imageCache.get(path); found {
		return uri
	}
	body, contentType, err := c.fetch(ctx, path, "image/*", maxImageBytes)
	if err != nil {
		log.Printf("koito: image %s: %v", path, err)
		return ""
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if !embeddableImageTypes[mediaType] {
		mediaType = http.DetectContentType(body)
	}
	if !embeddableImageTypes[mediaType] {
		log.Printf("koito: image %s: unexpected content type %q", path, contentType)
		return ""
	}
	uri := "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(body)
	c.imageCache.add(path, uri)
	return uri
}

// ImageDataURIs fetches images in parallel.
func (c *Client) ImageDataURIs(ctx context.Context, images []Image, size Size) []string {
	uris := make([]string, len(images))
	var wg sync.WaitGroup
	for i, image := range images {
		wg.Add(1)
		go func() {
			defer wg.Done()
			uris[i] = c.ImageDataURI(ctx, image, size)
		}()
	}
	wg.Wait()
	return uris
}

func (c *Client) getJSON(ctx context.Context, endpoint string, query url.Values, ttl time.Duration, out any) error {
	path := "/apis/web/v1/" + endpoint
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	c.mu.Lock()
	cached, found := c.responses[path]
	c.mu.Unlock()
	if found && time.Since(cached.fetchedAt) < ttl {
		return json.Unmarshal(cached.body, out)
	}

	body, _, err := c.fetch(ctx, path, "application/json", maxJSONBytes)
	if err == nil {
		if err = json.Unmarshal(body, out); err != nil {
			err = fmt.Errorf("koito: GET %s: decoding response: %w", path, err)
		}
	}
	if err != nil {
		if found {
			log.Printf("%v; serving the copy from %s ago", err, time.Since(cached.fetchedAt).Round(time.Second))
			return json.Unmarshal(cached.body, out)
		}
		return err
	}
	c.mu.Lock()
	c.responses[path] = cachedResponse{body: body, fetchedAt: time.Now()}
	c.mu.Unlock()
	return nil
}

func (c *Client) fetch(ctx context.Context, path, accept string, maxBytes int64) (body []byte, contentType string, err error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, "", err
	}
	request.Header.Set("Accept", accept)
	request.Header.Set("User-Agent", c.userAgent)
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, "", fmt.Errorf("koito: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("koito: GET %s: %s", path, response.Status)
	}
	body, err = io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("koito: GET %s: %w", path, err)
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("koito: GET %s: response over %d bytes", path, maxBytes)
	}
	return body, response.Header.Get("Content-Type"), nil
}
