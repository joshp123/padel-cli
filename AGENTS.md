# AGENTS.md — padel-cli

This file describes the Openclaw plugin knobs and how agents should obtain the
values. It is written for automation agents, not end users.

## Plugin knobs (values)

### PADEL_AUTH_FILE (env value)
- **What it is**: A path to a file containing Playtomic credentials.
- **Format** (exact):
  ```
  [username]
  USER_EMAIL_OR_USERNAME
  [password]
  USER_PASSWORD
  ```
- **How to obtain**: Ask the operator for Playtomic login details and store the
  value in the secrets system (e.g. agenix). Reference the secret path in
  `PADEL_AUTH_FILE` (example path: `/run/agenix/padel-auth`).
- **Why it matters**: Required for `padel auth login` and any booking/sync flows.

### config.json values
Stored at: `~/.config/padel/config.json` (unless overridden by `PADEL_CONFIG_DIR`)

Required keys:
- `default_location` (string): Default city/area for searches.
- `favourite_clubs` (list of objects): Clubs preferred by default.
  - Each entry: `{ "id": "CLUB_ID", "alias": "CLUB_ALIAS" }`
- `preferred_times` (list of strings): Time windows like `"09:00"`, `"10:30"`.
- `preferred_duration` (int): Duration in minutes.

How to obtain:
- `default_location`: Ask the operator for the home city/area.
- `favourite_clubs`: Use `padel clubs --near "CITY_NAME"` to get club IDs, then
  pick preferred ones with an alias.
- `preferred_times` and `preferred_duration`: Ask the operator.

Example (use placeholders only):
```json
{
  "default_location": "CITY_NAME",
  "favourite_clubs": [
    { "id": "CLUB_ID", "alias": "CLUB_ALIAS" }
  ],
  "preferred_times": ["HH:MM", "HH:MM"],
  "preferred_duration": 90
}
```

### venues.json values
Stored at: `~/.config/padel/venues.json` (unless overridden by `PADEL_CONFIG_DIR`)

Schema:
```json
{
  "venues": [
    {
      "id": "VENUE_ID",
      "alias": "VENUE_ALIAS",
      "name": "VENUE_NAME",
      "indoor": true,
      "timezone": "TIMEZONE"
    }
  ]
}
```

How to obtain:
- Use `padel clubs --near "CITY_NAME"` to list venues and IDs.
- Use `padel venues add` to add them, or write `venues.json` directly.
- Ask the operator to confirm aliases and preferred indoor/outdoor settings.

## Optional override values
- `PADEL_CONFIG_DIR`: Override the config directory. Use only if explicitly
  requested; default is standard XDG (`~/.config/padel`).
- `XDG_CONFIG_HOME`: If set, config is stored under `$XDG_CONFIG_HOME/padel`.

## Validation / smoke checks
- `padel auth status` (verifies credentials)
- `padel venues list` (verifies venues load)
- `padel search --near "CITY_NAME" --date YYYY-MM-DD --time HH:MM-HH:MM`

## Notes
- All values are mutable at runtime and live outside the Nix store.
- Avoid hardcoding real locations or credentials in repo files; use placeholders.
