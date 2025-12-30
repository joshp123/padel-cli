package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"padel-cli/api"

	"github.com/spf13/cobra"
)

var (
	outputJSON    bool
	outputCompact bool
	cfg           Config
	client        = api.NewClient()
)

type Config struct {
	DefaultLocation   string          `json:"default_location"`
	FavouriteClubs    []FavouriteClub `json:"favourite_clubs"`
	PreferredTimes    []string        `json:"preferred_times"`
	PreferredDuration int             `json:"preferred_duration"`
}

type FavouriteClub struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
}

var rootCmd = &cobra.Command{
	Use:   "padel",
	Short: "Padel CLI for Playtomic availability",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if outputJSON && outputCompact {
			return fmt.Errorf("choose either --json or --compact")
		}
		return nil
	},
	SilenceUsage: true,
}

func Execute() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(clubsCmd())
	rootCmd.AddCommand(availabilityCmd())
	rootCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(venuesCmd())
	rootCmd.AddCommand(bookingsCmd())
	rootCmd.AddCommand(authCmd())
	rootCmd.AddCommand(bookCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&outputJSON, "json", false, "Output JSON")
	rootCmd.PersistentFlags().BoolVar(&outputCompact, "compact", false, "Output compact text")
}

func initConfig() {
	loaded, err := loadConfig()
	if err == nil {
		cfg = loaded
	}
}

func loadConfig() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		return Config{}, err
	}
	if info.IsDir() {
		return Config{}, fmt.Errorf("config path is a directory: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var conf Config
	if err := json.NewDecoder(file).Decode(&conf); err != nil {
		return Config{}, err
	}
	return conf, nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "padel", "config.json"), nil
}
