package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"padel-cli/api"

	"github.com/spf13/cobra"
)

type AvailabilitySlot struct {
	Court         string `json:"court"`
	Time          string `json:"time"`
	StartUTC      string `json:"start_utc"`
	VenueTimezone string `json:"venue_timezone"`
	Duration      int    `json:"duration"`
	Available     bool   `json:"available"`
	Price         string `json:"price"`
	Indoor        bool   `json:"indoor"`
	ResourceID    string `json:"resource_id,omitempty"`
}

type AvailabilityOutput struct {
	ClubID   string             `json:"club_id"`
	ClubName string             `json:"club_name"`
	Date     string             `json:"date"`
	Slots    []AvailabilitySlot `json:"slots"`
}

func availabilityCmd() *cobra.Command {
	var clubID string
	var venueAlias string
	var date string
	var showOutdoor bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "availability",
		Short: "Show availability for a club on a date",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clubID != "" && venueAlias != "" {
				return fmt.Errorf("use either --club-id or --venue, not both")
			}
			if clubID == "" && venueAlias == "" {
				return fmt.Errorf("--club-id or --venue is required")
			}
			if date == "" {
				return fmt.Errorf("--date is required")
			}
			if showOutdoor && showAll {
				return fmt.Errorf("use either --outdoor or --all, not both")
			}

			venueTimezone := ""
			if venueAlias != "" {
				venue, err := lookupVenue(venueAlias)
				if err != nil {
					return err
				}
				clubID = venue.ID
				venueTimezone = venue.TimeZone
			}

			ctx := context.Background()
			tenant, err := client.GetTenant(ctx, clubID)
			if err != nil {
				return err
			}

			// Fetch resources to get indoor/outdoor info
			resources, err := client.GetResources(ctx, clubID)
			if err != nil {
				// Fall back to tenant resources if GetResources fails
				resources = tenant.Resources
			}

			if venueTimezone == "" {
				venueTimezone = tenant.Address.TimeZone
			}
			venueTimezone = normalizeVenueTimezone(venueTimezone)
			location := venueLocation(venueTimezone)

			target, err := parseDateInputInLocation(date, location)
			if err != nil {
				return err
			}

			startLocal := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, location)
			endLocal := time.Date(target.Year(), target.Month(), target.Day(), 23, 59, 59, 0, location)
			startUTC := startLocal.UTC()
			endUTC := endLocal.UTC()

			availability, err := client.GetAvailability(ctx, clubID, startUTC, endUTC)
			if err != nil {
				return err
			}

			// Build resource map with indoor info
			resourceInfo := map[string]api.Resource{}
			for _, resource := range resources {
				resourceInfo[resource.ResourceID] = resource
			}

			targetDate := target.Format("2006-01-02")
			slots := flattenAvailabilityWithResources(availability, resourceInfo, targetDate, venueTimezone, showOutdoor, showAll)

			output := AvailabilityOutput{
				ClubID:   clubID,
				ClubName: tenant.TenantName,
				Date:     targetDate,
				Slots:    slots,
			}

			if outputJSON {
				return writeJSON(output)
			}

			return renderAvailability(output)
		},
	}

	cmd.Flags().StringVar(&clubID, "club-id", "", "Club (tenant) ID")
	cmd.Flags().StringVar(&venueAlias, "venue", "", "Saved venue alias")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD)")
	cmd.Flags().BoolVar(&showOutdoor, "outdoor", false, "Show only outdoor courts")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all courts (indoor and outdoor)")
	return cmd
}

func flattenAvailability(resources []api.AvailabilityResource, resourceNames map[string]string, targetDate, venueTimezone string) []AvailabilitySlot {
	// Legacy function for backward compatibility - shows all courts
	resourceInfo := map[string]api.Resource{}
	for id, name := range resourceNames {
		resourceInfo[id] = api.Resource{ResourceID: id, Name: name}
	}
	return flattenAvailabilityWithResources(resources, resourceInfo, targetDate, venueTimezone, false, true)
}

func flattenAvailabilityWithResources(resources []api.AvailabilityResource, resourceInfo map[string]api.Resource, targetDate, venueTimezone string, showOutdoor, showAll bool) []AvailabilitySlot {
	slots := []AvailabilitySlot{}
	for _, resource := range resources {
		resInfo, hasInfo := resourceInfo[resource.ResourceID]
		court := resource.ResourceID
		if hasInfo && resInfo.Name != "" {
			court = resInfo.Name
		}

		// Determine if indoor
		isIndoor := true // default to indoor if unknown
		if hasInfo {
			isIndoor = resInfo.IsIndoor()
		}

		// Filter by indoor/outdoor
		if !showAll {
			if showOutdoor && isIndoor {
				continue // skip indoor when showing outdoor only
			}
			if !showOutdoor && !isIndoor {
				continue // skip outdoor when showing indoor only (default)
			}
		}

		for _, slot := range resource.Slots {
			resourceDate := resource.StartDate
			if strings.Contains(resourceDate, "T") && len(resourceDate) >= 10 {
				resourceDate = resourceDate[:10]
			}
			localDate, localTime, startUTC, ok := apiUTCDateTimeToLocal(resourceDate, slot.StartTime, venueTimezone)
			if ok && targetDate != "" && localDate != targetDate {
				continue
			}
			if localTime == "" {
				localTime = timeLabel(slot.StartTime)
			}
			slots = append(slots, AvailabilitySlot{
				Court:         court,
				Time:          localTime,
				StartUTC:      startUTC,
				VenueTimezone: normalizeVenueTimezone(venueTimezone),
				Duration:      slot.Duration,
				Available:     true,
				Price:         slot.Price,
				Indoor:        isIndoor,
				ResourceID:    resource.ResourceID,
			})
		}
	}

	sort.Slice(slots, func(i, j int) bool {
		if slots[i].Court == slots[j].Court {
			return slots[i].Time < slots[j].Time
		}
		return slots[i].Court < slots[j].Court
	})
	return slots
}

func renderAvailability(output AvailabilityOutput) error {
	if len(output.Slots) == 0 {
		fmt.Printf("%s (%s)\nDate: %s\nNo available slots.\n", output.ClubName, output.ClubID, output.Date)
		return nil
	}

	fmt.Printf("%s (%s)\nDate: %s\n", output.ClubName, output.ClubID, output.Date)

	if outputCompact {
		byCourt := map[string][]AvailabilitySlot{}
		for _, slot := range output.Slots {
			byCourt[slot.Court] = append(byCourt[slot.Court], slot)
		}
		parts := []string{}
		for court, slots := range byCourt {
			times := make([]string, 0, len(slots))
			for _, slot := range slots {
				times = append(times, slot.Time)
			}
			parts = append(parts, fmt.Sprintf("%s: %s", court, strings.Join(uniqueSortedTimes(times), " ")))
		}
		sort.Strings(parts)
		fmt.Println(strings.Join(parts, " | "))
		return nil
	}

	writer := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
	fmt.Fprintln(writer, "COURT\tTIME\tDURATION\tPRICE")
	for _, slot := range output.Slots {
		fmt.Fprintf(writer, "%s\t%s\t%dm\t%s\n", slot.Court, slot.Time, slot.Duration, slot.Price)
	}
	return writer.Flush()
}
