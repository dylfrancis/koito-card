package card

import (
	"fmt"
	"math"
	"strings"

	"github.com/dylfrancis/koito-card/internal/koito"
)

var topListTitles = map[string]string{"artists": "Top artists", "albums": "Top albums", "tracks": "Top tracks"}

// Top expects one entry in artURIs per item ("" for none).
func Top(kind, period string, items []koito.Ranked, artURIs []string, options Options) string {
	const listTop, rowHeight = 50.0, 56.0
	height := int(listTop + rowHeight*float64(max(len(items), 1)) + 10)
	title := topListTitles[kind]
	var body strings.Builder
	fmt.Fprintf(&body, `<text class="heading" x="%d" y="34">%s</text>`, padding, title)
	fmt.Fprintf(&body, `<text class="muted small" x="%d" y="34" text-anchor="end">%s</text>`, cardWidth-padding, periodLabels[period])

	if len(items) == 0 {
		fmt.Fprintf(&body, `<text class="muted body" x="%d" y="%g">No listens in this period</text>`, padding, listTop+30)
		return document(cardWidth, height, title+": none", options, "", body.String())
	}

	var maxPlays int64 = 1
	for _, ranked := range items {
		maxPlays = max(maxPlays, ranked.Item.ListenCount)
	}
	artShape := "rounded-square"
	if kind == "artists" {
		artShape = "circle"
	}

	names := make([]string, 0, len(items))
	for i, ranked := range items {
		entity := ranked.Item
		rowY := listTop + rowHeight*float64(i)
		name, subtitle := entity.Title, joinArtistNames(entity.Artists)
		if kind == "artists" {
			name, subtitle = entity.Name, ""
		}
		if name == "" {
			name = "Unknown"
		}
		names = append(names, name)
		rank := ranked.Rank
		if rank == 0 {
			rank = i + 1
		}
		plays := playsLabel(entity.ListenCount)
		artURI := ""
		if i < len(artURIs) {
			artURI = artURIs[i]
		}

		const textX = 96.0
		const barWidth = cardWidth - padding - textX
		nameY, subtitleY, barY := rowY+22, rowY+37, rowY+44
		if subtitle == "" {
			nameY, barY = rowY+27, rowY+35
		}
		nameMaxWidth := barWidth - textWidth(plays, 12, false) - 12
		fillWidth := math.Max(3, barWidth*float64(entity.ListenCount)/float64(maxPlays))

		fmt.Fprintf(&body, `<g class="row" style="animation-delay:%dms">`, i*70)
		fmt.Fprintf(&body, `<text class="muted small tabular rank" x="%d" y="%g" text-anchor="middle">%d</text>`, padding+10, rowY+32, rank)
		body.WriteString(cover(artURI, 44, rowY+8, 40, artShape))
		fmt.Fprintf(&body, `<text class="name" x="%g" y="%g">%s</text>`, textX, nameY, escapeXML(truncateToWidth(name, nameMaxWidth, 14, true)))
		fmt.Fprintf(&body, `<text class="muted small tabular" x="%d" y="%g" text-anchor="end">%s</text>`, cardWidth-padding, nameY, plays)
		if subtitle != "" {
			fmt.Fprintf(&body, `<text class="muted small" x="%g" y="%g">%s</text>`, textX, subtitleY, escapeXML(truncateToWidth(subtitle, barWidth, 12, false)))
		}
		fmt.Fprintf(&body, `<rect class="track" x="%g" y="%g" width="%g" height="3" rx="1.5"/>`, textX, barY, barWidth)
		fmt.Fprintf(&body, `<rect class="accent progress" x="%g" y="%g" width="%.1f" height="3" rx="1.5" style="animation-delay:%dms"/>`, textX, barY, fillWidth, 150+i*70)
		body.WriteString(`</g>`)
	}
	return document(cardWidth, height, title+": "+strings.Join(names, ", "), options, "", body.String())
}
