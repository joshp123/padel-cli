package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type ClubSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
}

func clubsCmd() *cobra.Command {
	var near string
	var radius int

	cmd := &cobra.Command{
		Use:   "clubs",
		Short: "List padel clubs near a location",
		RunE: func(cmd *cobra.Command, args []string) error {
			if near == "" {
				near = cfg.DefaultLocation
			}
			if near == "" {
				return fmt.Errorf("--near is required (or set default_location in config)")
			}

			ctx := context.Background()
			lat, lon, err := resolveLocation(ctx, near)
			if err != nil {
				return err
			}

			tenants, err := client.GetTenants(ctx, lat, lon, radius)
			if err != nil {
				return err
			}

			sort.Slice(tenants, func(i, j int) bool {
				return tenants[i].TenantName < tenants[j].TenantName
			})

			clubs := make([]ClubSummary, 0, len(tenants))
			for _, tenant := range tenants {
				clubs = append(clubs, ClubSummary{
					ID:      tenant.TenantID,
					Name:    tenant.TenantName,
					Address: formatAddress(tenant.Address),
				})
			}

			if outputJSON {
				return writeJSON(clubs)
			}

			writer := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
			if !outputCompact {
				fmt.Fprintln(writer, "ID\tNAME\tADDRESS")
			}
			for _, club := range clubs {
				fmt.Fprintf(writer, "%s\t%s\t%s\n", club.ID, club.Name, club.Address)
			}
			return writer.Flush()
		},
	}

	cmd.Flags().StringVar(&near, "near", "", "Location name or lat,lon")
	cmd.Flags().IntVar(&radius, "radius", 50000, "Search radius in meters")
	return cmd
}
