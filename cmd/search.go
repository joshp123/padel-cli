package cmd

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"padel-cli/api"

	"github.com/spf13/cobra"
)

type SearchClubResult struct {
	ClubID   string             `json:"club_id"`
	ClubName string             `json:"club_name"`
	Slots    []AvailabilitySlot `json:"slots"`
}

type SearchResult struct {
	Date  string             `json:"date"`
	Clubs []SearchClubResult `json:"clubs"`
}

type searchTenant struct {
	Tenant   api.Tenant
	TimeZone string
}

const rateLimitDelay = 150 * time.Millisecond

func searchCmd() *cobra.Command {
	var location string
	var clubID string
	var venuesInput string
	var date string
	var timeRange string
	var weekend bool
	var radius int
	var showOutdoor bool
	var showAll bool

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search for available courts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clubID != "" && venuesInput != "" {
				return fmt.Errorf("use either --club-id or --venues, not both")
			}
			if showOutdoor && showAll {
				return fmt.Errorf("use either --outdoor or --all, not both")
			}
			if clubID == "" && venuesInput == "" {
				if location == "" {
					location = cfg.DefaultLocation
				}
				if location == "" {
					return fmt.Errorf("--location is required (or set default_location in config)")
				}
			}

			var dateInputs []string
			if weekend {
				for _, d := range nextWeekendDates(time.Now()) {
					dateInputs = append(dateInputs, d.Format("2006-01-02"))
				}
			} else {
				if date == "" {
					return fmt.Errorf("--date is required unless --weekend is set")
				}
				parsed, err := parseDateInput(date)
				if err != nil {
					return err
				}
				dateInputs = []string{parsed.Format("2006-01-02")}
			}

			var startMinutes, endMinutes int
			var hasTimeRange bool
			if timeRange != "" {
				parsedStart, parsedEnd, err := parseTimeRange(timeRange)
				if err != nil {
					return err
				}
				startMinutes, endMinutes = parsedStart, parsedEnd
				hasTimeRange = true
			}

			ctx := context.Background()
			var tenants []searchTenant
			if clubID != "" {
				tenant, err := client.GetTenant(ctx, clubID)
				if err != nil {
					return err
				}
				tenants = []searchTenant{{
					Tenant:   tenant,
					TimeZone: normalizeVenueTimezone(tenant.Address.TimeZone),
				}}
			} else if venuesInput != "" {
				aliases := splitAliases(venuesInput)
				if len(aliases) == 0 {
					return fmt.Errorf("--venues must include at least one alias")
				}
				venues, err := lookupVenues(aliases)
				if err != nil {
					return err
				}
				for idx, venue := range venues {
					tenant, err := client.GetTenant(ctx, venue.ID)
					if err != nil {
						return err
					}
					venueTimezone := venue.TimeZone
					if venueTimezone == "" {
						venueTimezone = tenant.Address.TimeZone
					}
					tenants = append(tenants, searchTenant{
						Tenant:   tenant,
						TimeZone: normalizeVenueTimezone(venueTimezone),
					})
					if idx < len(venues)-1 {
						time.Sleep(rateLimitDelay)
					}
				}
			} else {
				lat, lon, err := resolveLocation(ctx, location)
				if err != nil {
					return err
				}
				rawTenants, err := client.GetTenants(ctx, lat, lon, radius)
				if err != nil {
					return err
				}
				for _, tenant := range rawTenants {
					tenants = append(tenants, searchTenant{
						Tenant:   tenant,
						TimeZone: normalizeVenueTimezone(tenant.Address.TimeZone),
					})
				}
			}

			sort.Slice(tenants, func(i, j int) bool {
				return tenants[i].Tenant.TenantName < tenants[j].Tenant.TenantName
			})

			results := make([]SearchResult, 0, len(dateInputs))
			for _, dateInput := range dateInputs {

				clubResults := make([]SearchClubResult, 0, len(tenants))
				for idx, tenantInfo := range tenants {
					location := venueLocation(tenantInfo.TimeZone)
					target, err := parseDateInputInLocation(dateInput, location)
					if err != nil {
						return err
					}
					startLocal := time.Date(target.Year(), target.Month(), target.Day(), 0, 0, 0, 0, location)
					endLocal := time.Date(target.Year(), target.Month(), target.Day(), 23, 59, 59, 0, location)
					startUTC := startLocal.UTC()
					endUTC := endLocal.UTC()

					availability, err := client.GetAvailability(ctx, tenantInfo.Tenant.TenantID, startUTC, endUTC)
					if err != nil {
						return err
					}

					// Fetch resources to get indoor/outdoor info
					resources, err := client.GetResources(ctx, tenantInfo.Tenant.TenantID)
					if err != nil {
						// Fall back to tenant resources if GetResources fails
						resources = tenantInfo.Tenant.Resources
					}

					resourceInfo := map[string]api.Resource{}
					for _, resource := range resources {
						resourceInfo[resource.ResourceID] = resource
					}

					targetDate := target.Format("2006-01-02")
					slots := filterAvailabilityWithResources(availability, resourceInfo, startMinutes, endMinutes, hasTimeRange, targetDate, tenantInfo.TimeZone, showOutdoor, showAll)
					clubResults = append(clubResults, SearchClubResult{
						ClubID:   tenantInfo.Tenant.TenantID,
						ClubName: tenantInfo.Tenant.TenantName,
						Slots:    slots,
					})

					if idx < len(tenants)-1 {
						time.Sleep(rateLimitDelay)
					}
				}

				results = append(results, SearchResult{
					Date:  dateInput,
					Clubs: clubResults,
				})
			}

			if outputJSON {
				return writeJSON(results)
			}

			return renderSearch(results)
		},
	}

	cmd.Flags().StringVar(&location, "location", "", "Location name or lat,lon")
	cmd.Flags().StringVar(&clubID, "club-id", "", "Club (tenant) ID")
	cmd.Flags().StringVar(&venuesInput, "venues", "", "Comma-separated saved venue aliases")
	cmd.Flags().StringVar(&date, "date", "", "Date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&timeRange, "time", "", "Time range (HH:MM-HH:MM)")
	cmd.Flags().BoolVar(&weekend, "weekend", false, "Search the next Saturday and Sunday")
	cmd.Flags().IntVar(&radius, "radius", 50000, "Search radius in meters")
	cmd.Flags().BoolVar(&showOutdoor, "outdoor", false, "Show only outdoor courts")
	cmd.Flags().BoolVar(&showAll, "all", false, "Show all courts (indoor and outdoor)")
	return cmd
}

func splitAliases(input string) []string {
	parts := strings.Split(input, ",")
	aliases := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		aliases = append(aliases, trimmed)
	}
	return aliases
}

func filterAvailability(resources []api.AvailabilityResource, resourceNames map[string]string, startMinutes, endMinutes int, hasTimeRange bool, targetDate, venueTimezone string) []AvailabilitySlot {
	// Legacy function - shows all courts
	resourceInfo := map[string]api.Resource{}
	for id, name := range resourceNames {
		resourceInfo[id] = api.Resource{ResourceID: id, Name: name}
	}
	return filterAvailabilityWithResources(resources, resourceInfo, startMinutes, endMinutes, hasTimeRange, targetDate, venueTimezone, false, true)
}

func filterAvailabilityWithResources(resources []api.AvailabilityResource, resourceInfo map[string]api.Resource, startMinutes, endMinutes int, hasTimeRange bool, targetDate, venueTimezone string, showOutdoor, showAll bool) []AvailabilitySlot {
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
				continue
			}
			if !showOutdoor && !isIndoor {
				continue
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
			minutes, err := slotMinutes(localTime)
			if err != nil {
				continue
			}
			if hasTimeRange {
				if minutes < startMinutes || minutes > endMinutes {
					continue
				}
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

func renderSearch(results []SearchResult) error {
	for _, result := range results {
		if len(results) > 1 {
			fmt.Printf("%s\n", result.Date)
		}

		if outputCompact {
			fmt.Println(renderCompactSearch(result))
			if len(results) > 1 {
				fmt.Println()
			}
			continue
		}

		for _, club := range result.Clubs {
			fmt.Printf("%s\n", club.ClubName)
			if len(club.Slots) == 0 {
				fmt.Println("  No available slots.")
				continue
			}

			byCourt := map[string][]AvailabilitySlot{}
			for _, slot := range club.Slots {
				byCourt[slot.Court] = append(byCourt[slot.Court], slot)
			}
			courts := make([]string, 0, len(byCourt))
			for court := range byCourt {
				courts = append(courts, court)
			}
			sort.Strings(courts)
			for _, court := range courts {
				times := make([]string, 0, len(byCourt[court]))
				for _, slot := range byCourt[court] {
					times = append(times, slot.Time)
				}
				fmt.Printf("  %s: %s\n", court, strings.Join(uniqueSortedTimes(times), "  "))
			}
			fmt.Println()
		}
	}
	return nil
}

func renderCompactSearch(result SearchResult) string {
	candidateTimes := []string{}
	for _, club := range result.Clubs {
		for _, slot := range club.Slots {
			candidateTimes = append(candidateTimes, slot.Time)
		}
	}
	times := uniqueSortedTimes(candidateTimes)

	parts := []string{}
	for _, club := range result.Clubs {
		timeSet := map[string]struct{}{}
		for _, slot := range club.Slots {
			timeSet[slot.Time] = struct{}{}
		}

		if len(times) == 0 {
			parts = append(parts, fmt.Sprintf("%s: no slots", club.ClubName))
			continue
		}

		labels := make([]string, 0, len(times))
		for _, t := range times {
			if _, ok := timeSet[t]; ok {
				labels = append(labels, fmt.Sprintf("%s ✓", t))
			} else {
				labels = append(labels, fmt.Sprintf("%s ✗", t))
			}
		}
		parts = append(parts, fmt.Sprintf("%s: %s", club.ClubName, strings.Join(labels, " ")))
	}

	return strings.Join(parts, " | ")
}
