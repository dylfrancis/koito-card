package server

import (
	"html/template"
	"net/http"
	"strings"
)

var previewPage = template.Must(template.New("preview").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>koito-card</title>
<style>
:root{color-scheme:light dark;--bg:#f6f8fa;--fg:#1f2328;--muted:#59636e;--code:#eaeef2}
@media (prefers-color-scheme:dark){:root{--bg:#010409;--fg:#e6edf3;--muted:#9198a1;--code:#161b22}}
body{margin:0;padding:32px 16px;background:var(--bg);color:var(--fg);font:15px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans",Helvetica,Arial,sans-serif}
main{max-width:1000px;margin:0 auto}
h1{margin:0 0 4px}
p{color:var(--muted);margin:0 0 24px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(min(100%,480px),1fr));gap:16px;align-items:start}
img{display:block;max-width:100%;height:auto}
pre{background:var(--code);padding:12px 14px;border-radius:8px;overflow-x:auto;font-size:13px}
a{color:inherit}
</style>
</head>
<body>
<main>
<h1>koito-card</h1>
<p>Live SVG cards from this Koito history. They follow your system's light or dark theme here; add <code>?theme=light</code> or <code>?theme=dark</code> to fix one.</p>
<div class="grid">
<img src="now-playing.svg" width="480" height="120" alt="Now playing">
<img src="stats.svg" width="480" height="208" alt="Listening stats, past 7 days">
<img src="top.svg?type=artists" width="480" height="340" alt="Top artists">
<img src="top.svg?type=tracks" width="480" height="340" alt="Top tracks">
</div>
<h2>Embed in a GitHub README</h2>
<pre>{{.}}</pre>
<p>All options are listed in the <a href="https://github.com/dylfrancis/koito-card#cards">README</a>.</p>
</main>
</body>
</html>
`))

func servePreview(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host
	readmeSnippet := `<picture>
  <source media="(prefers-color-scheme: dark)" srcset="` + baseURL + `/now-playing.svg?theme=dark">
  <img alt="What I'm listening to" src="` + baseURL + `/now-playing.svg?theme=light">
</picture>`
	header := w.Header()
	header.Set("Content-Type", "text/html; charset=utf-8")
	header.Set("Content-Security-Policy", "default-src 'none'; img-src 'self'; style-src 'unsafe-inline'")
	header.Set("Cache-Control", "no-cache")
	previewPage.Execute(w, readmeSnippet)
}
