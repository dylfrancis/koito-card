package card

import (
	"fmt"
	"math"
	"strings"

	"github.com/dylfrancis/koito-card/internal/koito"
)

type Series struct {
	Values   []int64
	From, To string
}

// Stats draws the activity chart only when series is non-nil.
func Stats(period string, totals koito.Stats, series *Series, streakDays int, options Options) string {
	const title = "Listening stats"
	hasChart := series != nil && len(series.Values) > 0
	height := 118
	if hasChart {
		height = 208
	}
	var body strings.Builder
	fmt.Fprintf(&body, `<text class="heading" x="%d" y="34">%s</text>`, padding, title)
	if streakDays >= 2 {
		chipText := fmt.Sprintf("%d-day streak", streakDays)
		chipX := padding + textWidth(title, 16, true) + 10
		chipWidth := textWidth(chipText, 11, true) + 18
		fmt.Fprintf(&body, `<rect class="tint" x="%.1f" y="19" width="%.1f" height="20" rx="10"/>`, chipX, chipWidth)
		fmt.Fprintf(&body, `<text class="accent chip" x="%.1f" y="33" text-anchor="middle">%s</text>`, chipX+chipWidth/2, chipText)
	}
	fmt.Fprintf(&body, `<text class="muted small" x="%d" y="34" text-anchor="end">%s</text>`, cardWidth-padding, periodLabels[period])

	listenTime, listenTimeUnit := formatListenTime(totals.MinutesListened)
	figures := []struct{ value, label string }{
		{compactNumber(totals.ListenCount), "PLAYS"},
		{listenTime, listenTimeUnit},
		{compactNumber(totals.ArtistCount), "ARTISTS"},
		{compactNumber(totals.AlbumCount), "ALBUMS"},
		{compactNumber(totals.TrackCount), "TRACKS"},
	}
	columnWidth := float64(cardWidth-2*padding) / float64(len(figures))
	for i, figure := range figures {
		x := padding + columnWidth*float64(i)
		fmt.Fprintf(&body, `<text class="figure tabular" x="%g" y="78">%s</text>`, x, figure.value)
		fmt.Fprintf(&body, `<text class="muted label" x="%g" y="97">%s</text>`, x, figure.label)
	}

	if hasChart {
		body.WriteString(barChart(series.Values, padding, 118, cardWidth-2*padding, 56))
		fmt.Fprintf(&body, `<text class="muted tiny" x="%d" y="193">%s</text>`, padding, escapeXML(series.From))
		fmt.Fprintf(&body, `<text class="muted tiny" x="%d" y="193" text-anchor="end">%s</text>`, cardWidth-padding, escapeXML(series.To))
	}
	ariaLabel := fmt.Sprintf("Koito listening stats, %s: %s plays, %s %s",
		strings.ToLower(periodLabels[period]), withCommas(totals.ListenCount), listenTime, strings.ToLower(listenTimeUnit))
	return document(cardWidth, height, ariaLabel, options, "", body.String())
}

// barChart gives empty buckets a grey stub so the timeline stays readable.
func barChart(values []int64, left, top, width, height float64) string {
	count := float64(len(values))
	gap := 8.0
	switch {
	case count > 20:
		gap = 2
	case count > 10:
		gap = 4
	}
	barWidth := (width - gap*(count-1)) / count
	var peak int64 = 1
	for _, value := range values {
		peak = max(peak, value)
	}
	var bars strings.Builder
	for i, value := range values {
		barHeight := math.Max(2, height*float64(value)/float64(peak))
		fillClass := "accent"
		if value == 0 {
			fillClass = "track"
		}
		fmt.Fprintf(&bars, `<rect class="%s column" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="%.1f" style="animation-delay:%dms"/>`,
			fillClass, left+float64(i)*(barWidth+gap), top+height-barHeight, barWidth, barHeight, math.Min(3, barWidth/2), i*400/len(values))
	}
	return bars.String()
}
