package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Venue struct {
	ID       string `json:"id"`
	Alias    string `json:"alias"`
	Name     string `json:"name"`
	Indoor   bool   `json:"indoor"`
	TimeZone string `json:"timezone"`
}

type VenuesFile struct {
	Venues []Venue `json:"venues"`
}

const DefaultVenueTimezone = "Europe/Madrid"

func LoadVenues() ([]Venue, error) {
	path, err := VenuesPath()
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []Venue{}, nil
		}
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("venues path is a directory: %s", path)
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var payload VenuesFile
	if err := json.NewDecoder(file).Decode(&payload); err != nil {
		return nil, err
	}
	for i := range payload.Venues {
		if payload.Venues[i].TimeZone == "" {
			payload.Venues[i].TimeZone = DefaultVenueTimezone
		}
	}
	return payload.Venues, nil
}

func SaveVenues(venues []Venue) error {
	if _, err := ensureConfigDir(); err != nil {
		return err
	}

	path, err := VenuesPath()
	if err != nil {
		return err
	}

	sorted := make([]Venue, len(venues))
	copy(sorted, venues)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i].Alias) < strings.ToLower(sorted[j].Alias)
	})
	for i := range sorted {
		if sorted[i].TimeZone == "" {
			sorted[i].TimeZone = DefaultVenueTimezone
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(VenuesFile{Venues: sorted})
}

func FindVenueByAlias(venues []Venue, alias string) (Venue, bool) {
	needle := strings.ToLower(strings.TrimSpace(alias))
	for _, venue := range venues {
		if strings.ToLower(venue.Alias) == needle {
			return venue, true
		}
	}
	return Venue{}, false
}
