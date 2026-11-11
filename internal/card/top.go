package card

import (
	"fmt"
	"math"
	"strings"

	"github.com/dylfrancis/koito-card/internal/koito"
)

var topListTitles = map[string]string{"artists": "Top artists", "albums": "Top albums", "tracks": "Top tracks"}

// Top expects one entry in artURIs per item ("" for none); rows spread out to fill minHeight.
func Top(kind, period string, items []koito.Ranked, artURIs []string, minHeight int, options Options) string {
	const listTop, contentRowHeight, bottomPadding = 50.0, 56.0, 10.0
	rowCount := float64(max(len(items), 1))
	rowHeight := math.Max(contentRowHeight, (float64(minHeight)-listTop-bottomPadding)/rowCount)
	height := int(math.Round(listTop + rowHeight*rowCount + bottomPadding))
	title := orDefault(options.Title, topListTitles[kind])

	var body strings.Builder
	header, _ := cardHeader(title, period, 0, options)
	body.WriteString(header)

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
	showRank, showArt, showPlays, showBars := !options.Hidden["rank"], !options.Hidden["art"], !options.Hidden["plays"], !options.Hidden["bars"]
	textX, rankX, artX := float64(padding), 0.0, 0.0
	if showRank {
		rankX, textX = textX+10, textX+24
	}
	if showArt {
		artX, textX = textX, textX+52
	}
	barWidth := cardWidth - padding - textX

	names := make([]string, 0, len(items))
	for i, ranked := range items {
		entity := ranked.Item
		rowY := listTop + rowHeight*float64(i) + (rowHeight-contentRowHeight)/2
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

		nameY, subtitleY, barY := rowY+22, rowY+37, rowY+44
		switch {
		case subtitle == "" && showBars:
			nameY, barY = rowY+27, rowY+35
		case subtitle == "":
			nameY = rowY + 33
		case !showBars:
			nameY, subtitleY = rowY+24, rowY+40
		}
		nameMaxWidth := barWidth
		if showPlays {
			nameMaxWidth -= textWidth(plays, 12, false) + 12
		}

		fmt.Fprintf(&body, `<g class="row" style="animation-delay:%dms">`, i*70)
		if showRank {
			fmt.Fprintf(&body, `<text class="muted small tabular rank" x="%g" y="%g" text-anchor="middle">%d</text>`, rankX, rowY+32, rank)
		}
		if showArt {
			body.WriteString(cover(artURI, artX, rowY+8, 40, artShape))
		}
		fmt.Fprintf(&body, `<text class="name" x="%g" y="%g">%s</text>`, textX, nameY, escapeXML(truncateToWidth(name, nameMaxWidth, 14, true)))
		if showPlays {
			fmt.Fprintf(&body, `<text class="muted small tabular" x="%d" y="%g" text-anchor="end">%s</text>`, cardWidth-padding, nameY, plays)
		}
		if subtitle != "" {
			fmt.Fprintf(&body, `<text class="muted small" x="%g" y="%g">%s</text>`, textX, subtitleY, escapeXML(truncateToWidth(subtitle, barWidth, 12, false)))
		}
		if showBars {
			fillWidth := math.Max(3, barWidth*float64(entity.ListenCount)/float64(maxPlays))
			fmt.Fprintf(&body, `<rect class="track" x="%g" y="%g" width="%g" height="3" rx="1.5"/>`, textX, barY, barWidth)
			fmt.Fprintf(&body, `<rect class="accent progress" x="%g" y="%g" width="%.1f" height="3" rx="1.5" style="animation-delay:%dms"/>`, textX, barY, fillWidth, 150+i*70)
		}
		body.WriteString(`</g>`)
	}
	return document(cardWidth, height, title+": "+strings.Join(names, ", "), options, "", body.String())
}
