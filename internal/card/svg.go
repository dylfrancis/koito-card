package card

import (
	"fmt"
	"strings"
)

type Options struct {
	Theme        string
	Accent       string
	Border       bool
	MarginBottom int
}

type palette struct {
	background, border, text, muted, track, accent string
	tintOpacity, backdropOpacity                   float64
}

var (
	lightPalette = palette{"#ffffff", "#d0d7de", "#1f2328", "#59636e", "#eff2f5", "#bf3989", 0.12, 0.30}
	darkPalette  = palette{"#0d1117", "#30363d", "#e6edf3", "#9198a1", "#21262d", "#f778ba", 0.16, 0.40}
)

func (p palette) cssVariables(accentOverride string) string {
	accent := p.accent
	if accentOverride != "" {
		accent = accentOverride
	}
	return fmt.Sprintf("--background:%s;--border:%s;--text:%s;--muted:%s;--track:%s;--accent:%s;--tint:%.2f;--backdrop:%.2f",
		p.background, p.border, p.text, p.muted, p.track, accent, p.tintOpacity, p.backdropOpacity)
}

func themeCSS(options Options) string {
	light := "svg{" + lightPalette.cssVariables(options.Accent) + "}"
	dark := "svg{" + darkPalette.cssVariables(options.Accent) + "}"
	switch options.Theme {
	case "light":
		return light
	case "dark":
		return dark
	default:
		return light + "@media (prefers-color-scheme:dark){" + dark + "}"
	}
}

const fontStack = `-apple-system,BlinkMacSystemFont,"Segoe UI","Noto Sans",Helvetica,Arial,sans-serif,"Apple Color Emoji","Segoe UI Emoji"`

const cardCSS = `text{font-family:` + fontStack + `;fill:var(--text)}` +
	`.background{fill:var(--background)}.border{fill:none;stroke:var(--border)}` +
	`.muted{fill:var(--muted)}.accent{fill:var(--accent)}.track{fill:var(--track)}` +
	`.tint{fill:var(--accent);fill-opacity:var(--tint)}.backdrop{opacity:var(--backdrop)}` +
	`.placeholder-note{fill:var(--muted);stroke:var(--muted)}` +
	`.tabular{font-variant-numeric:tabular-nums}` +
	`.heading{font-size:16px;font-weight:600}.title{font-size:20px;font-weight:600}.figure{font-size:22px;font-weight:600}` +
	`.name{font-size:14px;font-weight:600}.body{font-size:14px}.small{font-size:12px}.tiny{font-size:10px}` +
	`.label{font-size:11px;font-weight:700;letter-spacing:.08em}.label-detail{font-weight:400;letter-spacing:0}` +
	`.chip{font-size:11px;font-weight:600}.rank{font-weight:600}` +
	`.equalizer,.column{transform-box:fill-box;transform-origin:50% 100%}` +
	`.equalizer{animation:bounce 1.1s ease-in-out infinite}` +
	`.equalizer-1{animation-duration:.9s;animation-delay:-.3s}.equalizer-2{animation-duration:1.3s;animation-delay:-.6s}.equalizer-3{animation-duration:1s;animation-delay:-.15s}` +
	`.progress{transform-box:fill-box;transform-origin:0 50%;animation:grow-right .8s cubic-bezier(.2,.7,.3,1) both}` +
	`.row{animation:fade-up .5s ease-out both}.column{animation:grow-up .6s cubic-bezier(.2,.7,.3,1) both}` +
	`@keyframes bounce{0%,100%{transform:scaleY(.35)}50%{transform:scaleY(1)}}` +
	`@keyframes fade-up{from{opacity:0;transform:translateY(4px)}}` +
	`@keyframes grow-up{from{transform:scaleY(0)}}` +
	`@keyframes grow-right{from{transform:scaleX(0)}}` +
	`@media (prefers-reduced-motion:reduce){.equalizer,.row,.column,.progress{animation:none}}`

const (
	cardWidth = 480
	padding   = 20
)

func document(width, height int, ariaLabel string, options Options, defs, body string) string {
	imageHeight := height + options.MarginBottom
	var svg strings.Builder
	fmt.Fprintf(&svg, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s">`, width, imageHeight, width, imageHeight, escapeXML(ariaLabel))
	fmt.Fprintf(&svg, `<title>%s</title><style>%s%s</style>`, escapeXML(ariaLabel), themeCSS(options), cardCSS)
	fmt.Fprintf(&svg, `<defs><clipPath id="card-shape"><rect width="%d" height="%d" rx="10"/></clipPath>`, width, height)
	svg.WriteString(`<clipPath id="rounded-square" clipPathUnits="objectBoundingBox"><rect width="1" height="1" rx=".12"/></clipPath>`)
	svg.WriteString(`<clipPath id="circle" clipPathUnits="objectBoundingBox"><circle cx=".5" cy=".5" r=".5"/></clipPath>`)
	svg.WriteString(defs)
	svg.WriteString(`</defs>`)
	fmt.Fprintf(&svg, `<rect class="background" width="%d" height="%d" rx="10"/>`, width, height)
	svg.WriteString(body)
	if options.Border {
		fmt.Fprintf(&svg, `<rect class="border" x=".5" y=".5" width="%d" height="%d" rx="9.5"/>`, width-1, height-1)
	}
	svg.WriteString(`</svg>`)
	return svg.String()
}

// cover draws a placeholder note when artURI is empty.
func cover(artURI string, x, y, size float64, clipShape string) string {
	if artURI != "" {
		return fmt.Sprintf(`<image href="%s" x="%g" y="%g" width="%g" height="%g" preserveAspectRatio="xMidYMid slice" clip-path="url(#%s)"/>`,
			escapeXML(artURI), x, y, size, size, clipShape)
	}
	noteScale := size * 0.5 / 24
	return fmt.Sprintf(`<rect class="track" x="%g" y="%g" width="%g" height="%g" clip-path="url(#%s)"/>`+
		`<g class="placeholder-note" transform="translate(%.1f %.1f) scale(%.3f)"><path d="M9 18V6l11-2.5v12" fill="none" stroke-width="2"/><circle cx="6.5" cy="18" r="2.5"/><circle cx="17.5" cy="15.5" r="2.5"/></g>`,
		x, y, size, size, clipShape, x+size*0.25, y+size*0.25, noteScale)
}

var periodLabels = map[string]string{
	"day": "Past 24 hours", "week": "Past 7 days", "month": "Past 30 days", "year": "Past year", "all_time": "All time",
}

// Error is served instead of a broken image.
func Error(title, detail string, options Options) string {
	textMaxWidth := float64(cardWidth - 2*padding)
	body := fmt.Sprintf(`<text class="heading" x="%d" y="32">%s</text><text class="muted small" x="%d" y="54">%s</text>`,
		padding, escapeXML(truncateToWidth(title, textMaxWidth, 16, true)), padding, escapeXML(truncateToWidth(detail, textMaxWidth, 12, false)))
	return document(cardWidth, 76, title, options, "", body)
}
