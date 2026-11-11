package server

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/dylfrancis/koito-card/internal/card"
)

// badRequest's message is shown on the card.
type badRequest string

func (e badRequest) Error() string { return string(e) }

var hexColorPattern = regexp.MustCompile(`^#?([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func parseOptions(query url.Values) (card.Options, error) {
	options := card.Options{Theme: "auto", Border: true}
	theme, err := parseChoice(query, "theme", "auto", "auto", "light", "dark")
	if err != nil {
		return options, err
	}
	options.Theme = theme
	if options.Accent, err = parseHexColor(query, "accent"); err != nil {
		return options, err
	}
	if options.Background, err = parseHexColor(query, "background"); err != nil {
		return options, err
	}
	if options.Text, err = parseHexColor(query, "text"); err != nil {
		return options, err
	}
	if options.Border, err = parseBool(query, "border", true); err != nil {
		return options, err
	}
	if options.MarginBottom, err = parseInt(query, "margin_bottom", 0, 0, 200); err != nil {
		return options, err
	}
	return options, nil
}

func parseHexColor(query url.Values, key string) (string, error) {
	value := strings.TrimSpace(query.Get(key))
	if value == "" {
		return "", nil
	}
	if !hexColorPattern.MatchString(value) {
		return "", badRequest(fmt.Sprintf("%s must be a hex color, like %s=ff5500", key, key))
	}
	return "#" + strings.TrimPrefix(value, "#"), nil
}

// parseHidden accepts hide=a,b and repeated hide parameters.
func parseHidden(query url.Values, hideable ...string) (map[string]bool, error) {
	hidden := map[string]bool{}
	for _, list := range query["hide"] {
		for _, part := range strings.Split(list, ",") {
			part = strings.ToLower(strings.TrimSpace(part))
			if part == "" {
				continue
			}
			if !slices.Contains(hideable, part) {
				return nil, badRequest("hide can include: " + strings.Join(hideable, ", "))
			}
			hidden[part] = true
		}
	}
	return hidden, nil
}

const maxTitleLength = 60

func parseTitle(query url.Values) (string, error) {
	title := strings.TrimSpace(query.Get("title"))
	if utf8.RuneCountInString(title) > maxTitleLength {
		return "", badRequest(fmt.Sprintf("title can be at most %d characters", maxTitleLength))
	}
	return title, nil
}

func parseChoice(query url.Values, key, fallback string, allowed ...string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(query.Get(key)))
	if value == "" {
		return fallback, nil
	}
	if slices.Contains(allowed, value) {
		return value, nil
	}
	return "", badRequest(fmt.Sprintf("%s must be one of: %s", key, strings.Join(allowed, ", ")))
}

func parsePeriod(query url.Values, fallback string) (string, error) {
	switch period := strings.ToLower(strings.TrimSpace(query.Get("period"))); period {
	case "":
		return fallback, nil
	case "day", "week", "month", "year", "all_time":
		return period, nil
	case "all", "alltime", "all-time":
		return "all_time", nil
	}
	return "", badRequest("period must be one of: day, week, month, year, all_time")
}

func parseBool(query url.Values, key string, fallback bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(query.Get(key))) {
	case "":
		return fallback, nil
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	}
	return fallback, badRequest(key + " must be true or false")
}

func parseInt(query url.Values, key string, fallback, lowest, highest int) (int, error) {
	value := strings.TrimSpace(query.Get(key))
	if value == "" {
		return fallback, nil
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < lowest || number > highest {
		return fallback, badRequest(fmt.Sprintf("%s must be a number from %d to %d", key, lowest, highest))
	}
	return number, nil
}
