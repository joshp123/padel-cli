package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"padel-cli/api"
	"padel-cli/storage"

	"github.com/spf13/cobra"
)

func bookCmd() *cobra.Command {
	var venueAlias string
	var date string
	var timeValue string
	var duration int
	var court string
	var players int
	var paymentMethod string

	cmd := &cobra.Command{
		Use:   "book",
		Short: "Book a court",
		RunE: func(cmd *cobra.Command, args []string) error {
			if venueAlias == "" || date == "" || timeValue == "" {
				return fmt.Errorf("--venue, --date, and --time are required")
			}
			if duration <= 0 {
				duration = 90
			}
			if players <= 0 {
				players = 4
			}

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

			venue, err := lookupVenue(venueAlias)
			if err != nil {
				return err
			}

			requestedMinutes, err := parseClock(timeValue)
			if err != nil {
				return err
			}

			ctx := context.Background()
			tenant, err := client.GetTenant(ctx, venue.ID)
			if err != nil {
				return err
			}

			venueTimezone := venue.TimeZone
			if venueTimezone == "" {
				venueTimezone = tenant.Address.TimeZone
			}
			venueTimezone = normalizeVenueTimezone(venueTimezone)
			location := venueLocation(venueTimezone)

			targetDate, err := parseDateInputInLocation(date, location)
			if err != nil {
				return err
			}

			startLocal := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, location)
			endLocal := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 23, 59, 59, 0, location)
			startDay := startLocal.UTC()
			endDay := endLocal.UTC()

			availability, err := client.GetAvailability(ctx, venue.ID, startDay, endDay)
			if err != nil {
				return err
			}

			resourceNames := map[string]string{}
			for _, resource := range tenant.Resources {
				resourceNames[resource.ResourceID] = resource.Name
			}

			targetDateStr := targetDate.Format("2006-01-02")
			slot, resourceID, resourceName, err := selectSlot(availability, resourceNames, targetDateStr, venueTimezone, requestedMinutes, duration, court)
			if err != nil {
				return err
			}

			startTime := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), requestedMinutes/60, requestedMinutes%60, 0, 0, location)
			startUTC := startTime.UTC()
			startUTCString := startUTC.Format(time.RFC3339)

			intent := api.PaymentIntentRequest{
				AllowedPaymentMethodTypes: []string{"OFFER", "CASH", "MERCHANT_WALLET", "DIRECT", "SWISH", "IDEAL", "BANCONTACT", "PAYTRAIL", "CREDIT_CARD", "QUICK_PAY"},
				UserID:                    creds.UserID,
				Cart: api.PaymentIntentCart{
					RequestedItem: api.PaymentIntentItem{
						CartItemType:      "CUSTOMER_MATCH",
						CartItemVoucherID: nil,
						CartItemData: api.PaymentIntentItemData{
							SupportsSplitPayment: true,
							NumberOfPlayers:      players,
							TenantID:             venue.ID,
							ResourceID:           resourceID,
							Start:                startUTC.Format("2006-01-02T15:04:05"),
							Duration:             duration,
							MatchRegistrations: []api.MatchRegistration{
								{UserID: creds.UserID, PayNow: true},
							},
						},
					},
				},
			}

			intentResp, err := client.CreatePaymentIntent(ctx, intent)
			if err != nil {
				return err
			}

			availableMethods := extractPaymentMethods(intentResp.AvailablePaymentMethods)
			selected, err := choosePaymentMethod(availableMethods, paymentMethod)
			if err != nil {
				return err
			}

			if selected != "" {
				if err := client.UpdatePaymentIntent(ctx, intentResp.PaymentIntentID, api.PaymentIntentUpdateRequest{SelectedPaymentMethod: selected}); err != nil {
					return err
				}
			}

			confirmResp, err := client.ConfirmPaymentIntent(ctx, intentResp.PaymentIntentID)
			if err != nil {
				return err
			}

			bookingID := extractBookingID(confirmResp)
			if bookingID == "" {
				bookingID = newBookingID()
			}

			booking := storage.Booking{
				ID:            bookingID,
				VenueAlias:    venue.Alias,
				VenueName:     tenant.TenantName,
				VenueID:       venue.ID,
				Court:         resourceName,
				Date:          targetDateStr,
				Time:          timeValue,
				StartUTC:      startUTCString,
				VenueTimezone: venueTimezone,
				Duration:      duration,
				Price:         parsePriceAmount(slot.Price),
				BookedAt:      time.Now().UTC().Format(time.RFC3339),
				Source:        "cli_booked",
			}

			db, err := storage.OpenBookingsDB()
			if err != nil {
				return err
			}
			defer db.Close()

			_, err = storage.AddBookingIfNotExists(db, booking)
			if err != nil {
				return err
			}

			fmt.Printf("Booked: %s %s %s\n", tenant.TenantName, timeValue, targetDate.Format("Mon 2 Jan"))
			priceLabel := slot.Price
			if priceLabel == "" {
				priceLabel = formatEUR(booking.Price)
			}
			fmt.Printf("%s | %dmin | %s\n", resourceName, duration, priceLabel)
			fmt.Printf("Booking ID: %s\n", bookingID)
			return nil
		},
	}

	cmd.Flags().StringVar(&venueAlias, "venue", "", "Saved venue alias")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&timeValue, "time", "", "Time (HH:MM)")
	cmd.Flags().IntVar(&duration, "duration", 90, "Duration in minutes")
	cmd.Flags().StringVar(&court, "court", "", "Court name")
	cmd.Flags().IntVar(&players, "players", 4, "Number of players")
	cmd.Flags().StringVar(&paymentMethod, "payment-method", "", "Payment method code")
	return cmd
}

func selectSlot(resources []api.AvailabilityResource, resourceNames map[string]string, targetDate, venueTimezone string, targetMinutes int, duration int, court string) (api.Slot, string, string, error) {
	court = strings.TrimSpace(court)
	matches := []struct {
		Slot         api.Slot
		ResourceID   string
		ResourceName string
	}{}

	for _, resource := range resources {
		name := resource.ResourceID
		if label, ok := resourceNames[resource.ResourceID]; ok && label != "" {
			name = label
		}
		if court != "" && !strings.EqualFold(name, court) {
			continue
		}
		for _, slot := range resource.Slots {
			resourceDate := resource.StartDate
			if strings.Contains(resourceDate, "T") && len(resourceDate) >= 10 {
				resourceDate = resourceDate[:10]
			}
			localDate, localTime, _, ok := apiUTCDateTimeToLocal(resourceDate, slot.StartTime, venueTimezone)
			if ok && targetDate != "" && localDate != targetDate {
				continue
			}
			if localTime == "" {
				localTime = timeLabel(slot.StartTime)
			}
			minutes, err := slotMinutes(localTime)
			if err != nil {
				continue
			}
			if minutes != targetMinutes {
				continue
			}
			if duration > 0 && slot.Duration != duration {
				continue
			}
			matches = append(matches, struct {
				Slot         api.Slot
				ResourceID   string
				ResourceName string
			}{Slot: slot, ResourceID: resource.ResourceID, ResourceName: name})
		}
	}

	if len(matches) == 0 {
		return api.Slot{}, "", "", fmt.Errorf("slot not available for %s", courtOrAny(court))
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].ResourceName < matches[j].ResourceName
	})
	chosen := matches[0]
	return chosen.Slot, chosen.ResourceID, chosen.ResourceName, nil
}

func courtOrAny(court string) string {
	if court != "" {
		return fmt.Sprintf("court %s", court)
	}
	return "requested time"
}

func extractPaymentMethods(raw []any) []string {
	methods := []string{}
	for _, entry := range raw {
		switch v := entry.(type) {
		case string:
			methods = append(methods, v)
		case map[string]any:
			if code, ok := v["type"].(string); ok {
				methods = append(methods, code)
				continue
			}
			if code, ok := v["code"].(string); ok {
				methods = append(methods, code)
			}
		}
	}
	return uniqueSortedTimes(methods)
}

func choosePaymentMethod(available []string, requested string) (string, error) {
	if requested != "" {
		for _, method := range available {
			if strings.EqualFold(method, requested) {
				return method, nil
			}
		}
		return "", fmt.Errorf("payment method %q not available. Available: %s", requested, strings.Join(available, ", "))
	}

	if len(available) == 0 {
		return "", nil
	}
	preferred := []string{"CASH", "MERCHANT_WALLET", "OFFER", "DIRECT", "CREDIT_CARD", "IDEAL", "BANCONTACT", "PAYTRAIL", "SWISH", "QUICK_PAY"}
	for _, pref := range preferred {
		for _, method := range available {
			if strings.EqualFold(method, pref) {
				return method, nil
			}
		}
	}
	if len(available) == 1 {
		return available[0], nil
	}
	return "", fmt.Errorf("multiple payment methods available (%s). Use --payment-method", strings.Join(available, ", "))
}

func extractBookingID(payload map[string]any) string {
	keys := []string{"match_id", "reservation_id", "booking_id", "id"}
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if str, ok := value.(string); ok && str != "" {
				return str
			}
		}
	}
	for _, nestedKey := range []string{"match", "reservation", "booking"} {
		if nested, ok := payload[nestedKey].(map[string]any); ok {
			for _, key := range keys {
				if value, ok := nested[key]; ok {
					if str, ok := value.(string); ok && str != "" {
						return str
					}
				}
			}
		}
	}
	return ""
}
