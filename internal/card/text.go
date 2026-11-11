package card

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/dylfrancis/koito-card/internal/koito"
)

// The server can't measure text in the viewer's font, so widths are Arial's
// (ASCII 32-126, 1/1000 em), padded to err wide for other system fonts.
var arialASCIIWidths = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278,
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556,
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778,
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556,
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556,
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584,
}

var arialPunctuationWidths = map[rune]int{
	'…': 1000, '‘': 222, '’': 222, '“': 333, '”': 333, '–': 556, '—': 1000, '·': 278, '•': 350, ' ': 278,
}

var cjkAndEmojiRanges = [][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF}, {0x3400, 0x4DBF}, {0x4E00, 0x9FFF},
	{0xA000, 0xA4CF}, {0xAC00, 0xD7A3}, {0xF900, 0xFAFF}, {0xFE30, 0xFE4F}, {0xFF00, 0xFF60},
	{0xFFE0, 0xFFE6}, {0x1F300, 0x1F64F}, {0x1F900, 0x1F9FF}, {0x1FA70, 0x1FAFF}, {0x20000, 0x3FFFD},
}

func runeWidth(r rune) int {
	if r >= 32 && r <= 126 {
		return arialASCIIWidths[r-32]
	}
	if width, found := arialPunctuationWidths[r]; found {
		return width
	}
	if unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf) {
		return 0
	}
	for _, span := range cjkAndEmojiRanges {
		if r >= span[0] && r <= span[1] {
			return 1000
		}
	}
	return 580
}

func pixelsPerUnit(fontSize float64, bold bool) float64 {
	safetyMargin := 1.08
	if bold {
		safetyMargin = 1.15
	}
	return fontSize * safetyMargin / 1000
}

func textWidth(s string, fontSize float64, bold bool) float64 {
	units := 0
	for _, r := range s {
		units += runeWidth(r)
	}
	return float64(units) * pixelsPerUnit(fontSize, bold)
}

// truncateToWidth also collapses whitespace.
func truncateToWidth(s string, maxWidth, fontSize float64, bold bool) string {
	s = strings.Join(strings.Fields(s), " ")
	if textWidth(s, fontSize, bold) <= maxWidth {
		return s
	}
	scale := pixelsPerUnit(fontSize, bold)
	width := float64(runeWidth('…')) * scale
	var kept strings.Builder
	for _, r := range s {
		runePixels := float64(runeWidth(r)) * scale
		if width+runePixels > maxWidth {
			break
		}
		kept.WriteRune(r)
		width += runePixels
	}
	return strings.TrimRight(kept.String(), " ,;:-–—·/&(") + "…"
}

// escapeXML also drops the characters XML forbids.
func escapeXML(s string) string {
	var escaped strings.Builder
	for _, r := range s {
		switch {
		case r == '&':
			escaped.WriteString("&amp;")
		case r == '<':
			escaped.WriteString("&lt;")
		case r == '>':
			escaped.WriteString("&gt;")
		case r == '"':
			escaped.WriteString("&quot;")
		case r == '\'':
			escaped.WriteString("&#39;")
		case r == '\t' || r == '\n' || r == '\r':
			escaped.WriteByte(' ')
		case r < 0x20, r == 0xFFFE, r == 0xFFFF:
		default:
			escaped.WriteRune(r)
		}
	}
	return escaped.String()
}

func joinArtistNames(artists []koito.Artist) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		if artist.Name != "" {
			names = append(names, artist.Name)
		}
	}
	return strings.Join(names, ", ")
}

func withCommas(n int64) string {
	digits := strconv.FormatInt(n, 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	var grouped strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			grouped.WriteByte(',')
		}
		grouped.WriteRune(digit)
	}
	return sign + grouped.String()
}

// compactNumber formats n as 99,999, 123k or 1.2M.
func compactNumber(n int64) string {
	switch {
	case n < 100_000:
		return withCommas(n)
	case n < 999_500:
		return strconv.FormatInt((n+500)/1000, 10) + "k"
	default:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	}
}

func playsLabel(plays int64) string {
	if plays == 1 {
		return "1 play"
	}
	return withCommas(plays) + " plays"
}

func formatListenTime(minutes int64) (value, unit string) {
	if minutes < 120 {
		return withCommas(minutes), "MINUTES"
	}
	hours := float64(minutes) / 60
	if hours < 100 {
		return strconv.FormatFloat(hours, 'f', 1, 64), "HOURS"
	}
	return compactNumber(int64(math.Round(hours))), "HOURS"
}

func timeAgo(then, now time.Time) string {
	if then.IsZero() {
		return ""
	}
	elapsed := now.Sub(then)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	case elapsed < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	default:
		return then.UTC().Format("Jan 2, 2006")
	}
}
