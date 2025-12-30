package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"padel-cli/api"
	"padel-cli/storage"
)

func resolveLocation(ctx context.Context, input string) (float64, float64, error) {
	if lat, lon, ok := parseCoordinate(input); ok {
		return lat, lon, nil
	}
	return client.Geocode(ctx, input)
}

func parseCoordinate(input string) (float64, float64, bool) {
	parts := strings.Split(input, ",")
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return 0, 0, false
	}
	lon, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

func parseDateInput(input string) (time.Time, error) {
	if input == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	now := time.Now()
	switch strings.ToLower(input) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()), nil
	case "tomorrow":
		t := now.AddDate(0, 0, 1)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()), nil
	}
	parsed, err := time.Parse("2006-01-02", input)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", input)
	}
	return parsed, nil
}

func nextWeekendDates(now time.Time) []time.Time {
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	daysUntilSaturday := (6 - weekday + 7) % 7
	saturday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, daysUntilSaturday)
	sunday := saturday.AddDate(0, 0, 1)
	return []time.Time{saturday, sunday}
}

func parseTimeRange(input string) (int, int, error) {
	parts := strings.Split(input, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time range %q (expected HH:MM-HH:MM)", input)
	}
	start, err := parseClock(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, err
	}
	end, err := parseClock(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, err
	}
	if end <= start {
		return 0, 0, fmt.Errorf("time range end must be after start")
	}
	return start, end, nil
}

func parseClock(input string) (int, error) {
	parsed, err := time.Parse("15:04", input)
	if err != nil {
		return 0, fmt.Errorf("invalid time %q (expected HH:MM)", input)
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func slotMinutes(input string) (int, error) {
	if strings.Count(input, ":") == 2 {
		parsed, err := time.Parse("15:04:05", input)
		if err != nil {
			return 0, err
		}
		return parsed.Hour()*60 + parsed.Minute(), nil
	}
	parsed, err := time.Parse("15:04", input)
	if err != nil {
		return 0, err
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func formatAddress(address api.Address) string {
	parts := []string{}
	if address.Street != "" {
		parts = append(parts, address.Street)
	}
	if address.City != "" {
		parts = append(parts, address.City)
	}
	if address.Country != "" {
		parts = append(parts, address.Country)
	}
	return strings.Join(parts, ", ")
}

func writeJSON(v any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

func uniqueSortedTimes(times []string) []string {
	set := map[string]struct{}{}
	for _, t := range times {
		set[t] = struct{}{}
	}
	unique := make([]string, 0, len(set))
	for t := range set {
		unique = append(unique, t)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i] < unique[j]
	})
	return unique
}

func timeLabel(startTime string) string {
	if len(startTime) >= 5 {
		return startTime[:5]
	}
	return startTime
}

func lookupVenue(alias string) (storage.Venue, error) {
	venues, err := storage.LoadVenues()
	if err != nil {
		return storage.Venue{}, err
	}
	venue, ok := storage.FindVenueByAlias(venues, alias)
	if !ok {
		return storage.Venue{}, fmt.Errorf("venue alias %q not found", alias)
	}
	return venue, nil
}

func lookupVenues(aliases []string) ([]storage.Venue, error) {
	venues, err := storage.LoadVenues()
	if err != nil {
		return nil, err
	}
	resolved := make([]storage.Venue, 0, len(aliases))
	for _, alias := range aliases {
		venue, ok := storage.FindVenueByAlias(venues, alias)
		if !ok {
			return nil, fmt.Errorf("venue alias %q not found", alias)
		}
		resolved = append(resolved, venue)
	}
	return resolved, nil
}

func normalizeVenueTimezone(tz string) string {
	if strings.TrimSpace(tz) == "" {
		return storage.DefaultVenueTimezone
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return storage.DefaultVenueTimezone
	}
	return tz
}

func venueLocation(tz string) *time.Location {
	tz = normalizeVenueTimezone(tz)
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func apiUTCToLocal(apiTime string, tz string) (string, string, string, bool) {
	parsed, ok := parseAPIDateTime(apiTime)
	if !ok {
		return "", "", "", false
	}
	parsed = parsed.UTC()
	startUTC := parsed.Format(time.RFC3339)
	loc := venueLocation(tz)
	local := parsed.In(loc)
	return local.Format("2006-01-02"), local.Format("15:04"), startUTC, true
}

func apiUTCDateTimeToLocal(dateStr, timeStr, tz string) (string, string, string, bool) {
	if dateStr == "" || timeStr == "" {
		return "", "", "", false
	}
	if len(timeStr) == 5 {
		timeStr = timeStr + ":00"
	}
	parsed, err := time.Parse("2006-01-02T15:04:05", fmt.Sprintf("%sT%s", dateStr, timeStr))
	if err != nil {
		return "", "", "", false
	}
	parsed = parsed.UTC()
	startUTC := parsed.Format(time.RFC3339)
	loc := venueLocation(tz)
	local := parsed.In(loc)
	return local.Format("2006-01-02"), local.Format("15:04"), startUTC, true
}

func localToUTC(dateStr, timeStr, tz string) (string, error) {
	loc := venueLocation(tz)
	parsed, err := time.ParseInLocation("2006-01-02 15:04", fmt.Sprintf("%s %s", dateStr, timeStr), loc)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339), nil
}

func parseDateInputInLocation(input string, loc *time.Location) (time.Time, error) {
	if input == "" {
		return time.Time{}, fmt.Errorf("date is required")
	}
	now := time.Now().In(loc)
	switch strings.ToLower(input) {
	case "today":
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc), nil
	case "tomorrow":
		t := now.AddDate(0, 0, 1)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02", input, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", input)
	}
	return parsed, nil
}

func parsePriceAmount(input string) float64 {
	if input == "" {
		return 0
	}
	fields := strings.Fields(input)
	if len(fields) == 0 {
		return 0
	}
	value := strings.ReplaceAll(fields[0], ",", ".")
	amount, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return amount
}

func parseAPIDateTime(input string) (time.Time, bool) {
	if input == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05.000Z07:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, input)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func newBookingID() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("bk_%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("bk_%d_%s", time.Now().Unix(), hex.EncodeToString(buf))
}
