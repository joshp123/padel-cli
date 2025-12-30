# padel CLI

CLI tool for checking Playtomic padel court availability and booking.

## Install / Build

```bash
go build -o padel
```

## Usage

```bash
# List clubs near a location
padel clubs --near "Madrid"

# Check availability for a club on a date
padel availability --club-id <id> --date 2025-01-05

# Search for available courts
padel search --location "Barcelona" --date 2025-01-05 --time 18:00-22:00

# JSON output
padel clubs --near "Madrid" --json
```

## Venue Management

Save venues with aliases for quick access:

```bash
# Add a venue
padel venues add --id "<playtomic-id>" --alias myclub --name "My Club" --indoor --timezone "Europe/Madrid"

# List saved venues
padel venues list

# Use alias in commands
padel availability --venue myclub --date 2025-01-05

# Search multiple venues
padel search --venues myclub,otherclub --date 2025-01-05 --time 09:00-11:00
```

## Booking History

```bash
# List upcoming bookings
padel bookings list

# List past bookings
padel bookings list --past

# Add a booking manually
padel bookings add --venue myclub --date 2025-01-04 --time 10:30 --court "Court 5" --price 42

# Sync from Playtomic account
padel bookings sync

# View stats
padel bookings stats
```

## Authentication

```bash
# Login to Playtomic
padel auth login --email you@example.com --password yourpass

# Check status
padel auth status

# Book a court (requires auth)
padel book --venue myclub --date 2025-01-05 --time 10:30 --duration 90
```

## Indoor/Outdoor Filtering

Default shows indoor courts only:

```bash
# Indoor only (default)
padel search --venues myclub --date 2025-01-05

# Outdoor only
padel search --venues myclub --date 2025-01-05 --outdoor

# All courts
padel search --venues myclub --date 2025-01-05 --all
```

## Output Formats

- Default: human-readable tables
- `--json`: structured JSON output
- `--compact`: single-line summaries (useful for chat bots)

## Configuration

Config stored in `~/.config/padel/`:

```
~/.config/padel/
├── config.json          # preferences
├── credentials.json     # auth tokens
├── venues.json          # saved venues
└── bookings.db          # SQLite booking history
```

Example config.json:

```json
{
  "default_location": "Madrid",
  "favourite_clubs": [
    {"id": "abc123", "alias": "myclub"}
  ],
  "preferred_times": ["18:00", "19:30"],
  "preferred_duration": 90
}
```

## API Notes

Uses Playtomic API endpoints reverse-engineered from:
- https://mattrighetti.com/2025/03/03/reverse-engineering-playtomic
- https://github.com/ypk46/playtomic-scheduler

## License

MIT
