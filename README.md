# koito-card

Live SVG cards of your [Koito](https://github.com/gabehf/Koito) listening history: what you're playing now, your top artists, albums and tracks, and your listening stats. Put them in your GitHub profile README, or anywhere else that shows images.

- **Now playing** shows the current track's cover with an animated equalizer and the blurred artwork behind it. When nothing is playing, it shows the last track and how long ago it played.
- **Top lists** show artists, albums or tracks for a period, with play counts and bars.
- **Stats** show plays, listening time, artists, albums, tracks, your current streak and a chart of daily, monthly or yearly activity.

Cards come in light, dark or automatic themes, take a custom accent color, and respect reduced-motion settings. Every card is a single self-contained SVG: styles are inline and cover art is embedded, which GitHub requires.

## Cards

| Card | URL | Options |
| --- | --- | --- |
| Now playing | `/now-playing.svg` | `hide`: `art`, `backdrop` |
| Top list | `/top.svg` | `type`: `artists` (default), `albums` or `tracks`<br>`period`<br>`limit`: 1 to 10, default 5<br>`height`: minimum card height; rows spread out to fill it<br>`title`<br>`hide`: `period`, `rank`, `art`, `plays`, `bars` |
| Stats | `/stats.svg` | `period`<br>`title`<br>`hide`: `streak`, `chart`, `period` |

`period` is `day` (past 24 hours), `week` (past 7 days, the default), `month` (past 30 days), `year` or `all_time`. `title` replaces the card's heading, up to 60 characters. `hide` takes a comma-separated list, like `hide=rank,plays`.

Every card also takes:

| Option | Values |
| --- | --- |
| `theme` | `auto` (default; follows the viewer's system), `light` or `dark` |
| `accent` | a hex color without `#`, like `accent=ff5500` |
| `background` | a hex color for the card background |
| `text` | a hex color for the main text |
| `border` | `true` (default) or `false` |
| `margin_bottom` | transparent space below the card, 0 to 200 (default 0), for spacing stacked cards |

For example, `/top.svg?type=albums&period=year&limit=3&theme=dark`.

Custom colors apply in both light and dark mode, while borders, bar tracks and secondary text keep the theme's colors. So pair them with the closest fixed theme, like `theme=dark&background=1e1b2e&text=f5e9ff`.

## Run it

koito-card reads Koito's web API, the same data Koito's own UI shows, so it needs no credentials. Add it next to Koito in the same compose file so it can reach Koito by service name:

```yaml
services:
  koito-card:
    image: ghcr.io/dylfrancis/koito-card:latest
    container_name: koito-card
    environment:
      KOITO_URL: http://koito:4110
    ports:
      - "4111:8080"
    restart: unless-stopped
```

Then open `http://<your server>:4111` for a preview of every card and a snippet to copy.

Without Docker, install the binary with `go install github.com/dylfrancis/koito-card@latest` and run `KOITO_URL=http://localhost:4110 koito-card`.

| Setting | Default | |
| --- | --- | --- |
| `KOITO_URL` | required | Address of your Koito server |
| `PORT` | `8080` | Port koito-card listens on |
| `CACHE_SECONDS` | `300` | How long top lists and stats are cached. Now playing is cached for at most 15 seconds. |

## Put it online

GitHub doesn't load README images from your browser: its image proxy fetches them from the public internet. So koito-card needs a public HTTPS address, for example a subdomain on your reverse proxy pointing at port 4111. Only koito-card has to be public; Koito itself can stay on your private network.

If Koito stops answering, koito-card keeps serving the last cards it built. If it has none yet, it shows an error card instead of a broken image.

## Add it to your README

GitHub picks the right image for the viewer's theme with `<picture>`:

```html
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="https://cards.example.com/now-playing.svg?theme=dark">
  <img alt="What I'm listening to" src="https://cards.example.com/now-playing.svg?theme=light">
</picture>
```

Or use plain Markdown with the default `theme=auto`, which follows the viewer's system theme rather than their GitHub theme:

```markdown
![Top artists this week](https://cards.example.com/top.svg?type=artists)
```

### Two columns

GitHub strips CSS from READMEs, but it keeps `align`, which floats images. Cards aligned left stack in the left column while the rest flow on the right, so each column fills from the top. `width="48%"` leaves room for the 20px padding GitHub adds to aligned images, `margin_bottom=40` gives about the same gap vertically as between the columns, and `height=428` ends the left column level with the right.

```html
<p align="right">
<a href="https://cards.example.com"><img src="https://cards.example.com/now-playing.svg?margin_bottom=40" width="48%" align="left"></a>
<a href="https://cards.example.com"><img src="https://cards.example.com/top.svg?type=artists&limit=6&height=428" width="48%" align="left"></a>
<a href="https://cards.example.com"><img src="https://cards.example.com/stats.svg?margin_bottom=40" width="48%" align="top"></a>
<a href="https://cards.example.com"><img src="https://cards.example.com/top.svg?type=tracks" width="48%" align="top"></a>
</p>
<br clear="all">
```

Keep each card a plain `<img>` inside its link. GitHub moves an `<img>` out of a `<picture>` inside a link, which breaks the theme switch.

### How fresh is it?

Now playing tells every cache, GitHub's proxy included, not to keep it, so it's fetched again on each page view. Top lists and stats may be cached for up to `CACHE_SECONDS`. A browser can still hold on to an image it already loaded; reloading the page shows the latest.

## Development

```sh
mise install                      # or Go 1.27.4+
KOITO_URL=http://localhost:4110 go run .
go test ./...
```

Open `http://localhost:8080` to see every card in your system theme.

## License

[MIT](LICENSE). koito-card is a separate project and isn't affiliated with Koito.
