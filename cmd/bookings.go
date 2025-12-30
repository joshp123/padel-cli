package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"padel-cli/storage"

	"github.com/spf13/cobra"
)

type BookingStats struct {
	TotalBookings       int     `json:"total_bookings"`
	TotalSpent          float64 `json:"total_spent"`
	FavouriteVenue      string  `json:"favourite_venue"`
	FavouriteVenueCount int     `json:"favourite_venue_count"`
	UsualTime           string  `json:"usual_time"`
	LastPlayed          string  `json:"last_played"`
}

func bookingsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bookings",
		Short: "Manage bookings history",
	}

	cmd.AddCommand(bookingsListCmd())
	cmd.AddCommand(bookingsAddCmd())
	cmd.AddCommand(bookingsRemoveCmd())
	cmd.AddCommand(bookingsStatsCmd())
	cmd.AddCommand(bookingsSyncCmd())
	return cmd
}

func bookingsListCmd() *cobra.Command {
	var past bool
	var from string
	var to string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List bookings",
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := storage.BookingFilter{}

			if from != "" {
				date, err := parseDateInput(from)
				if err != nil {
					return err
				}
				filter.From = date.Format("2006-01-02")
			}
			if to != "" {
				date, err := parseDateInput(to)
				if err != nil {
					return err
				}
				filter.To = date.Format("2006-01-02")
			}
			if filter.From != "" && filter.To != "" && filter.From > filter.To {
				return fmt.Errorf("--from must be on or before --to")
			}

			now := time.Now()
			filter.NowDate = now.Format("2006-01-02")
			filter.NowTime = now.Format("15:04")

			if filter.From == "" && filter.To == "" {
				if past {
					filter.Past = true
				} else {
					filter.Upcoming = true
				}
			}

			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			bookings, err := storage.ListBookings(db, filter)
			if err != nil {
				return err
			}
			venueByID, venueByAlias := buildVenueLookups()
			for i := range bookings {
				ensureBookingTimezone(&bookings[i], venueByID, venueByAlias)
			}

			if outputJSON {
				return writeJSON(bookings)
			}

			if len(bookings) == 0 {
				fmt.Println("No bookings found.")
				return nil
			}

			writer := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
			if !outputCompact {
				fmt.Fprintln(writer, "DATE\tTIME\tVENUE\tCOURT\tPRICE")
			}
			for _, booking := range bookings {
				price := formatEUR(booking.Price)
				fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", booking.Date, booking.Time, booking.VenueName, booking.Court, price)
			}
			return writer.Flush()
		},
	}

	cmd.Flags().BoolVar(&past, "past", false, "List past bookings")
	cmd.Flags().StringVar(&from, "from", "", "Start date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "End date (YYYY-MM-DD)")
	return cmd
}

func bookingsAddCmd() *cobra.Command {
	var venueAlias string
	var date string
	var timeValue string
	var court string
	var price float64
	var duration int

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a booking",
		RunE: func(cmd *cobra.Command, args []string) error {
			if venueAlias == "" || date == "" || timeValue == "" || court == "" {
				return fmt.Errorf("--venue, --date, --time, and --court are required")
			}
			if price <= 0 {
				return fmt.Errorf("--price must be greater than 0")
			}
			if duration <= 0 {
				duration = 90
			}

			if _, err := parseClock(timeValue); err != nil {
				return err
			}

			venue, err := lookupVenue(venueAlias)
			if err != nil {
				return err
			}

			venueTZ := normalizeVenueTimezone(venue.TimeZone)
			location := venueLocation(venueTZ)
			parsedDate, err := parseDateInputInLocation(date, location)
			if err != nil {
				return err
			}
			startUTC, err := localToUTC(parsedDate.Format("2006-01-02"), timeValue, venueTZ)
			if err != nil {
				return err
			}

			booking := storage.Booking{
				ID:            newBookingID(),
				VenueAlias:    venue.Alias,
				VenueName:     venue.Name,
				VenueID:       venue.ID,
				Court:         court,
				Date:          parsedDate.Format("2006-01-02"),
				Time:          timeValue,
				StartUTC:      startUTC,
				VenueTimezone: venueTZ,
				Duration:      duration,
				Price:         price,
				BookedAt:      time.Now().UTC().Format(time.RFC3339),
				Source:        "manual",
			}

			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			if err := storage.AddBooking(db, booking); err != nil {
				return err
			}

			fmt.Printf("Added booking %s at %s on %s %s.\n", booking.ID, booking.VenueName, booking.Date, booking.Time)
			return nil
		},
	}

	cmd.Flags().StringVar(&venueAlias, "venue", "", "Saved venue alias")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&timeValue, "time", "", "Time (HH:MM)")
	cmd.Flags().StringVar(&court, "court", "", "Court name")
	cmd.Flags().Float64Var(&price, "price", 0, "Price")
	cmd.Flags().IntVar(&duration, "duration", 90, "Duration in minutes")
	return cmd
}

func bookingsRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a booking",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := strings.TrimSpace(args[0])
			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			removed, err := storage.RemoveBooking(db, id)
			if err != nil {
				return err
			}
			if !removed {
				return fmt.Errorf("booking %q not found", id)
			}

			fmt.Printf("Removed booking %s.\n", id)
			return nil
		},
	}

	return cmd
}

func bookingsStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show booking stats",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			bookings, err := storage.ListBookings(db, storage.BookingFilter{})
			if err != nil {
				return err
			}
			venueByID, venueByAlias := buildVenueLookups()
			for i := range bookings {
				ensureBookingTimezone(&bookings[i], venueByID, venueByAlias)
			}
			if len(bookings) == 0 {
				fmt.Println("No bookings found.")
				return nil
			}

			stats := computeBookingStats(bookings)
			if outputJSON {
				return writeJSON(stats)
			}

			fmt.Printf("Total bookings: %d\n", stats.TotalBookings)
			fmt.Printf("Total spent: %s\n", formatEUR(stats.TotalSpent))
			fmt.Printf("Favourite venue: %s (%d bookings)\n", stats.FavouriteVenue, stats.FavouriteVenueCount)
			fmt.Printf("Usual time: %s\n", stats.UsualTime)
			fmt.Printf("Last played: %s\n", stats.LastPlayed)
			return nil
		},
	}

	return cmd
}

func bookingsSyncCmd() *cobra.Command {
	var from string
	var size int

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync bookings from Playtomic",
		RunE: func(cmd *cobra.Command, args []string) error {
			creds, err := storage.LoadCredentials()
			if err != nil {
				return err
			}
			if creds == nil || creds.AccessToken == "" {
				return fmt.Errorf("not logged in. Run 'padel auth login' first")
			}
			if creds.AccessTokenExpired(time.Now()) {
				return fmt.Errorf("token expired. Run 'padel auth login' to re-authenticate")
			}
			client.AccessToken = creds.AccessToken

			fromDate := time.Time{}
			if from != "" {
				parsed, err := parseDateInput(from)
				if err != nil {
					return err
				}
				fromDate = parsed
			}

			if size <= 0 {
				size = 50
			}

			ctx := context.Background()
			matches, err := client.GetMatches(ctx, size, "start_date,DESC", creds.UserID)
			if err != nil {
				return err
			}

			venues, err := storage.LoadVenues()
			if err != nil {
				return err
			}
			venueByID := map[string]storage.Venue{}
			for _, venue := range venues {
				venueByID[venue.ID] = venue
			}

			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			total := 0
			added := 0
			skipped := 0
			for _, match := range matches {
				total++
				start, ok := parseAPIDateTime(match.StartDate)
				if ok && !fromDate.IsZero() && start.Before(fromDate) {
					continue
				}

				venueTZ := match.Tenant.Address.TimeZone
				if venue, ok := venueByID[match.Tenant.TenantID]; ok {
					venueTZ = venue.TimeZone
				}
				localDate, localTime, startUTC, _ := apiUTCToLocal(match.StartDate, venueTZ)
				if localDate == "" {
					localDate = dateFromMatch(match.StartDate)
				}
				if localTime == "" {
					localTime = timeFromMatch(match.StartDate)
				}

				booking := storage.Booking{
					ID:            match.MatchID,
					VenueName:     match.Tenant.TenantName,
					VenueID:       match.Tenant.TenantID,
					Court:         match.ResourceName,
					Date:          localDate,
					Time:          localTime,
					StartUTC:      startUTC,
					VenueTimezone: normalizeVenueTimezone(venueTZ),
					Duration:      durationFromMatch(match.StartDate, match.EndDate),
					Price:         parsePriceAmount(match.Price),
					BookedAt:      match.CreatedAt,
					Source:        "playtomic_sync",
				}

				if venue, ok := venueByID[booking.VenueID]; ok {
					booking.VenueAlias = venue.Alias
				}
				if booking.VenueName == "" {
					booking.VenueName = booking.VenueAlias
				}

				inserted, err := storage.AddBookingIfNotExists(db, booking)
				if err != nil {
					return err
				}
				if inserted {
					added++
				} else {
					skipped++
				}
			}

			if outputJSON {
				return writeJSON(map[string]int{
					"synced":           added,
					"skipped":          skipped,
					"total_in_account": total,
				})
			}

			fmt.Printf("Sync complete. Added %d, skipped %d (total %d).\n", added, skipped, total)
			return nil
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "Only sync bookings on/after this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&size, "size", 50, "Number of matches to fetch")
	return cmd
}

func computeBookingStats(bookings []storage.Booking) BookingStats {
	stats := BookingStats{TotalBookings: len(bookings)}

	venueCounts := map[string]int{}
	venueNames := map[string]string{}
	for _, booking := range bookings {
		stats.TotalSpent += booking.Price
		key := booking.VenueAlias
		if key == "" {
			key = booking.VenueName
		}
		venueCounts[key]++
		if booking.VenueName != "" {
			venueNames[key] = booking.VenueName
		}
	}

	stats.FavouriteVenue, stats.FavouriteVenueCount = topVenue(venueCounts, venueNames)
	stats.UsualTime = mostCommonTime(bookings)
	stats.LastPlayed = lastPlayedDate(bookings)
	return stats
}

func topVenue(counts map[string]int, names map[string]string) (string, int) {
	top := ""
	max := 0
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		count := counts[key]
		if count > max {
			max = count
			top = key
		}
	}
	if top == "" {
		return "N/A", 0
	}
	if name, ok := names[top]; ok && name != "" {
		return name, max
	}
	return top, max
}

func mostCommonTime(bookings []storage.Booking) string {
	counts := map[string]int{}
	labels := map[string]string{}
	for _, booking := range bookings {
		label, ok := bookingTimeLabel(booking)
		if !ok {
			continue
		}
		counts[label]++
		labels[label] = label
	}
	if len(counts) == 0 {
		return "N/A"
	}

	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	best := keys[0]
	for _, key := range keys {
		if counts[key] > counts[best] {
			best = key
		}
	}
	return labels[best]
}

func bookingTimeLabel(booking storage.Booking) (string, bool) {
	if booking.Date == "" || booking.Time == "" {
		return "", false
	}
	parsed, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", booking.Date, booking.Time))
	if err != nil {
		return "", false
	}
	weekday := parsed.Weekday().String()
	if booking.Duration > 0 {
		startMinutes, err := parseClock(booking.Time)
		if err != nil {
			return fmt.Sprintf("%s %s", weekday, booking.Time), true
		}
		endMinutes := startMinutes + booking.Duration
		endHour := endMinutes / 60
		endMin := endMinutes % 60
		return fmt.Sprintf("%s %s-%02d:%02d", weekday, booking.Time, endHour, endMin), true
	}
	return fmt.Sprintf("%s %s", weekday, booking.Time), true
}

func lastPlayedDate(bookings []storage.Booking) string {
	var last time.Time
	found := false
	now := time.Now()
	for _, booking := range bookings {
		if booking.Date == "" {
			continue
		}
		dateTime := booking.Date
		if booking.Time != "" {
			dateTime += " " + booking.Time
		} else {
			dateTime += " 00:00"
		}
		parsed, err := time.Parse("2006-01-02 15:04", dateTime)
		if err != nil {
			continue
		}
		if parsed.After(now) {
			continue
		}
		if !found || parsed.After(last) {
			last = parsed
			found = true
		}
	}
	if !found {
		return "N/A"
	}
	return last.Format("2006-01-02")
}

func formatEUR(amount float64) string {
	return fmt.Sprintf("EUR %.2f", amount)
}

func buildVenueLookups() (map[string]storage.Venue, map[string]storage.Venue) {
	venues, err := storage.LoadVenues()
	if err != nil {
		return map[string]storage.Venue{}, map[string]storage.Venue{}
	}
	byID := map[string]storage.Venue{}
	byAlias := map[string]storage.Venue{}
	for _, venue := range venues {
		byID[venue.ID] = venue
		if venue.Alias != "" {
			byAlias[strings.ToLower(venue.Alias)] = venue
		}
	}
	return byID, byAlias
}

func ensureBookingTimezone(booking *storage.Booking, venueByID, venueByAlias map[string]storage.Venue) {
	if booking.VenueTimezone == "" {
		if venue, ok := venueByID[booking.VenueID]; ok && venue.TimeZone != "" {
			booking.VenueTimezone = venue.TimeZone
		}
		if booking.VenueTimezone == "" && booking.VenueAlias != "" {
			if venue, ok := venueByAlias[strings.ToLower(booking.VenueAlias)]; ok && venue.TimeZone != "" {
				booking.VenueTimezone = venue.TimeZone
			}
		}
	}
	booking.VenueTimezone = normalizeVenueTimezone(booking.VenueTimezone)
	if booking.StartUTC == "" && booking.Date != "" && booking.Time != "" {
		if strings.EqualFold(booking.Source, "playtomic_sync") {
			if parsed, err := time.Parse("2006-01-02 15:04", fmt.Sprintf("%s %s", booking.Date, booking.Time)); err == nil {
				booking.StartUTC = parsed.UTC().Format(time.RFC3339)
			}
		} else {
			if startUTC, err := localToUTC(booking.Date, booking.Time, booking.VenueTimezone); err == nil {
				booking.StartUTC = startUTC
			}
		}
	}
	if booking.StartUTC != "" {
		if localDate, localTime, _, ok := apiUTCToLocal(booking.StartUTC, booking.VenueTimezone); ok {
			booking.Date = localDate
			booking.Time = localTime
		}
	}
}

func dateFromMatch(value string) string {
	if value == "" {
		return ""
	}
	if parsed, ok := parseAPIDateTime(value); ok {
		return parsed.Format("2006-01-02")
	}
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

func timeFromMatch(value string) string {
	if value == "" {
		return ""
	}
	if parsed, ok := parseAPIDateTime(value); ok {
		return parsed.Format("15:04")
	}
	if len(value) >= 16 {
		return value[11:16]
	}
	return ""
}

func durationFromMatch(startValue, endValue string) int {
	start, okStart := parseAPIDateTime(startValue)
	end, okEnd := parseAPIDateTime(endValue)
	if !okStart || !okEnd {
		return 0
	}
	if end.Before(start) {
		return 0
	}
	return int(end.Sub(start).Minutes())
}
