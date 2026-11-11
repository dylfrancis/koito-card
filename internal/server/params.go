package server

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

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
	if accent := strings.TrimSpace(query.Get("accent")); accent != "" {
		if !hexColorPattern.MatchString(accent) {
			return options, badRequest("accent must be a hex color, like accent=ff5500")
		}
		options.Accent = "#" + strings.TrimPrefix(accent, "#")
	}
	if options.Border, err = parseBool(query, "border", true); err != nil {
		return options, err
	}
	if options.MarginBottom, err = parseInt(query, "margin_bottom", 0, 0, 200); err != nil {
		return options, err
	}
	return options, nil
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
