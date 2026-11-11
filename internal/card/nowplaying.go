package card

import (
	"fmt"
	"strings"
	"time"

	"github.com/dylfrancis/koito-card/internal/koito"
)

func NowPlaying(listen *koito.Listen, playing bool, artURI, backdropURI string, options Options, now time.Time) string {
	const height = 120
	showArt := !options.Hidden["art"]
	textX := 118.0
	if !showArt {
		textX = padding
	}
	var body strings.Builder

	if listen == nil {
		if showArt {
			body.WriteString(cover("", padding, padding, 80, "rounded-square"))
		}
		fmt.Fprintf(&body, `<text class="muted label" x="%g" y="38">NOW PLAYING</text><text class="title" x="%g" y="68">Nothing played yet</text>`, textX, textX)
		return document(cardWidth, height, "Koito: nothing played yet", options, "", body.String())
	}

	title := orDefault(listen.Track.Title, "Unknown track")
	artists := orDefault(joinArtistNames(listen.Track.Artists), "Unknown artist")

	defs := ""
	if backdropURI != "" {
		defs = `<filter id="blur"><feGaussianBlur stdDeviation="22"/></filter>`
		fmt.Fprintf(&body, `<g clip-path="url(#card-shape)"><image class="backdrop" href="%s" x="-60" y="-240" width="600" height="600" preserveAspectRatio="xMidYMid slice" filter="url(#blur)"/></g>`, escapeXML(backdropURI))
	}
	if showArt {
		body.WriteString(cover(artURI, padding, padding, 80, "rounded-square"))
	}

	ariaLabel := "Last played on Koito: "
	if playing {
		ariaLabel = "Now playing on Koito: "
		for bar := range 4 {
			fmt.Fprintf(&body, `<rect class="accent equalizer equalizer-%d" x="%g" y="27" width="3" height="12" rx="1.5"/>`, bar, textX+float64(bar)*5)
		}
		fmt.Fprintf(&body, `<text class="accent label" x="%g" y="38">NOW PLAYING</text>`, textX+25)
	} else {
		fmt.Fprintf(&body, `<text class="muted label" x="%g" y="38">LAST PLAYED`, textX)
		if ago := timeAgo(listen.Time, now); ago != "" {
			fmt.Fprintf(&body, `<tspan class="label-detail"> · %s</tspan>`, escapeXML(ago))
		}
		body.WriteString(`</text>`)
	}

	textMaxWidth := cardWidth - textX - padding
	fmt.Fprintf(&body, `<text class="title" x="%g" y="68">%s</text>`, textX, escapeXML(truncateToWidth(title, textMaxWidth, 20, true)))
	fmt.Fprintf(&body, `<text class="muted body" x="%g" y="92">%s</text>`, textX, escapeXML(truncateToWidth(artists, textMaxWidth, 14, false)))
	return document(cardWidth, height, ariaLabel+title+" by "+artists, options, defs, body.String())
}
