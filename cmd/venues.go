package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"padel-cli/storage"

	"github.com/spf13/cobra"
)

func venuesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "venues",
		Short: "Manage saved venues",
	}

	cmd.AddCommand(venuesListCmd())
	cmd.AddCommand(venuesAddCmd())
	cmd.AddCommand(venuesRemoveCmd())
	return cmd
}

func venuesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List saved venues",
		RunE: func(cmd *cobra.Command, args []string) error {
			venues, err := storage.LoadVenues()
			if err != nil {
				return err
			}

			sort.Slice(venues, func(i, j int) bool {
				return strings.ToLower(venues[i].Alias) < strings.ToLower(venues[j].Alias)
			})

			if outputJSON {
				return writeJSON(venues)
			}

			if len(venues) == 0 {
				fmt.Println("No venues saved.")
				return nil
			}

			writer := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
			if !outputCompact {
				fmt.Fprintln(writer, "ALIAS\tNAME\tINDOOR")
			}
			for _, venue := range venues {
				indoor := "no"
				if venue.Indoor {
					indoor = "yes"
				}
				fmt.Fprintf(writer, "%s\t%s\t%s\n", venue.Alias, venue.Name, indoor)
			}
			return writer.Flush()
		},
	}

	return cmd
}

func venuesAddCmd() *cobra.Command {
	var id string
	var alias string
	var name string
	var indoor bool
	var timezone string

	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a saved venue",
		RunE: func(cmd *cobra.Command, args []string) error {
			alias = strings.TrimSpace(alias)
			if id == "" || alias == "" || name == "" {
				return fmt.Errorf("--id, --alias, and --name are required")
			}
			if timezone == "" {
				timezone = storage.DefaultVenueTimezone
			}

			venues, err := storage.LoadVenues()
			if err != nil {
				return err
			}

			if _, ok := storage.FindVenueByAlias(venues, alias); ok {
				return fmt.Errorf("venue alias %q already exists", alias)
			}

			venues = append(venues, storage.Venue{
				ID:     id,
				Alias:  alias,
				Name:   name,
				Indoor: indoor,
				TimeZone: timezone,
			})

			if err := storage.SaveVenues(venues); err != nil {
				return err
			}

			fmt.Printf("Saved venue %s (%s).\n", alias, name)
			return nil
		},
	}

	cmd.Flags().StringVar(&id, "id", "", "Venue (tenant) ID")
	cmd.Flags().StringVar(&alias, "alias", "", "Short alias")
	cmd.Flags().StringVar(&name, "name", "", "Venue name")
	cmd.Flags().BoolVar(&indoor, "indoor", false, "Indoor venue")
	cmd.Flags().StringVar(&timezone, "timezone", storage.DefaultVenueTimezone, "Venue timezone (IANA)")
	return cmd
}

func venuesRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <alias>",
		Short: "Remove a saved venue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			alias := strings.TrimSpace(args[0])
			venues, err := storage.LoadVenues()
			if err != nil {
				return err
			}

			index := -1
			for i, venue := range venues {
				if strings.EqualFold(venue.Alias, alias) {
					index = i
					break
				}
			}

			if index == -1 {
				return fmt.Errorf("venue alias %q not found", alias)
			}

			venues = append(venues[:index], venues[index+1:]...)
			if err := storage.SaveVenues(venues); err != nil {
				return err
			}

			fmt.Printf("Removed venue %s.\n", alias)
			return nil
		},
	}

	return cmd
}
