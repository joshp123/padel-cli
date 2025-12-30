# padel CLI

CLI tool for checking Playtomic padel availability.

## Install / Build

```bash
go build -o padel
```

## Usage

```bash
# List clubs near a location
padel clubs --near "Rotterdam"

# Availability for a club on a date
padel availability --club-id <id> --date 2025-01-05

# Search for available courts
padel search --location "Rotterdam" --date 2025-01-05 --time 18:00-22:00

# JSON output
padel clubs --near "Rotterdam" --json

# Save a venue with an alias
padel venues add --id "2f30bde8-9cee-411b-93d4-d3a487884d35" --alias blijdorp --name "Blijdorp" --indoor --timezone "Europe/Amsterdam"

# Use venue aliases in availability/search
padel availability --venue blijdorp --date 2025-01-05
padel search --venues blijdorp,capelle --date 2025-01-05 --time 09:00-11:00

# Add a booking
padel bookings add --venue blijdorp --date 2025-01-04 --time 10:30 --court "Court 5" --price 42

# Booking stats
padel bookings stats

# Login and book
padel auth login
padel book --venue blijdorp --date 2025-01-05 --time 10:30 --duration 90

# Sync bookings from Playtomic
padel bookings sync
```

## Output Formats

- Default: human-readable
- `--json`: structured JSON output
- `--compact`: single-line summaries (useful for chat/notifications)

Note: Playtomic availability responses only include available slots. Unavailable slots cannot be inferred, so output focuses on available times.

Timezone handling:
- Times are displayed in the venue-local timezone.
- JSON includes `start_utc` and `venue_timezone` for bookings, availability, and search slots.

## Configuration

Config file path:

```
~/.config/padel/config.json
```

Example:

```json
{
  "default_location": "Rotterdam",
  "favourite_clubs": [
    {"id": "abc123", "alias": "padel-city"}
  ],
  "preferred_times": ["18:00", "19:30"],
  "preferred_duration": 90
}
```

Venues file path:

```
~/.config/padel/venues.json
```

Bookings database path:

```
~/.config/padel/bookings.db
```

Credentials file path:

```
~/.config/padel/credentials.json
```

## API Notes

Uses the Playtomic endpoints from the reverse-engineering writeup:

- `GET /v1/tenants?sport_id=PADEL&coordinate=lat,lon&radius=50000`
- `GET /v1/availability?sport_id=PADEL&tenant_id=...&start_min=YYYY-MM-DDT00:00:00&start_max=YYYY-MM-DDT23:59:59`

The `availability` call works without authentication according to the blog post.

## Examples

```bash
padel clubs --near "51.9244,4.4777"

padel availability --club-id f9b8c1f4-15df-4e9b-9fa1-c63abb222248 --date 2025-12-31

padel search --location "Rotterdam" --date 2025-12-31 --time 18:00-22:00 --compact
```
