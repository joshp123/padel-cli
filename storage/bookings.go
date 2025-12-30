package storage

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Booking struct {
	ID            string  `json:"id"`
	VenueAlias    string  `json:"venue_alias"`
	VenueName     string  `json:"venue_name"`
	VenueID       string  `json:"venue_id"`
	Court         string  `json:"court"`
	Date          string  `json:"date"`
	Time          string  `json:"time"`
	StartUTC      string  `json:"start_utc"`
	VenueTimezone string  `json:"venue_timezone"`
	Duration      int     `json:"duration"`
	Price         float64 `json:"price"`
	BookedAt      string  `json:"booked_at"`
	Source        string  `json:"source"`
}

type BookingFilter struct {
	From     string
	To       string
	Past     bool
	Upcoming bool
	NowDate  string
	NowTime  string
}

func OpenBookingsDB() (*sql.DB, error) {
	if _, err := ensureConfigDir(); err != nil {
		return nil, err
	}
	path, err := BookingsPath()
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	if err := ensureBookingsSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

func ensureBookingsSchema(db *sql.DB) error {
	createTable := `
CREATE TABLE IF NOT EXISTS bookings (
  id TEXT PRIMARY KEY,
  venue_alias TEXT,
  venue_name TEXT,
  venue_id TEXT,
  court TEXT,
  date TEXT,
  time TEXT,
  start_utc TEXT,
  venue_timezone TEXT,
  duration INTEGER,
  price REAL,
  players TEXT,
  booked_by TEXT,
  booked_at TEXT,
  source TEXT
);`

	if _, err := db.Exec(createTable); err != nil {
		return fmt.Errorf("create bookings table: %w", err)
	}

	if _, err := db.Exec("CREATE INDEX IF NOT EXISTS idx_bookings_date ON bookings(date);"); err != nil {
		return fmt.Errorf("create bookings index: %w", err)
	}

	if err := ensureBookingsColumns(db, []string{"start_utc", "venue_timezone"}); err != nil {
		return err
	}

	return nil
}

func ensureBookingsColumns(db *sql.DB, columns []string) error {
	rows, err := db.Query("PRAGMA table_info(bookings);")
	if err != nil {
		return fmt.Errorf("inspect bookings table: %w", err)
	}
	defer rows.Close()

	existing := map[string]struct{}{}
	for rows.Next() {
		var cid int
		var name string
		var ctype string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return fmt.Errorf("inspect bookings columns: %w", err)
		}
		existing[name] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspect bookings columns: %w", err)
	}

	for _, column := range columns {
		if _, ok := existing[column]; ok {
			continue
		}
		_, err := db.Exec(fmt.Sprintf("ALTER TABLE bookings ADD COLUMN %s TEXT;", column))
		if err != nil {
			return fmt.Errorf("add bookings column %s: %w", column, err)
		}
	}
	return nil
}

func AddBooking(db *sql.DB, booking Booking) error {
	query := `
INSERT INTO bookings (
  id, venue_alias, venue_name, venue_id, court, date, time, start_utc, venue_timezone, duration, price, booked_at, source
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	_, err := db.Exec(
		query,
		booking.ID,
		booking.VenueAlias,
		booking.VenueName,
		booking.VenueID,
		booking.Court,
		booking.Date,
		booking.Time,
		booking.StartUTC,
		booking.VenueTimezone,
		booking.Duration,
		booking.Price,
		booking.BookedAt,
		booking.Source,
	)
	return err
}

func AddBookingIfNotExists(db *sql.DB, booking Booking) (bool, error) {
	query := `
INSERT OR IGNORE INTO bookings (
  id, venue_alias, venue_name, venue_id, court, date, time, start_utc, venue_timezone, duration, price, booked_at, source
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);`

	res, err := db.Exec(
		query,
		booking.ID,
		booking.VenueAlias,
		booking.VenueName,
		booking.VenueID,
		booking.Court,
		booking.Date,
		booking.Time,
		booking.StartUTC,
		booking.VenueTimezone,
		booking.Duration,
		booking.Price,
		booking.BookedAt,
		booking.Source,
	)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func RemoveBooking(db *sql.DB, id string) (bool, error) {
	res, err := db.Exec("DELETE FROM bookings WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func ListBookings(db *sql.DB, filter BookingFilter) ([]Booking, error) {
	base := `
SELECT id, venue_alias, venue_name, venue_id, court, date, time, start_utc, venue_timezone, duration, price, booked_at, source
FROM bookings`

	conds := []string{}
	args := []any{}

	if filter.From != "" {
		conds = append(conds, "date >= ?")
		args = append(args, filter.From)
	}
	if filter.To != "" {
		conds = append(conds, "date <= ?")
		args = append(args, filter.To)
	}
	if filter.From == "" && filter.To == "" {
		if filter.Past {
			conds = append(conds, "date <= ?")
			args = append(args, filter.NowDate)
		}
		if filter.Upcoming {
			conds = append(conds, "date >= ?")
			args = append(args, filter.NowDate)
		}
	}

	query := base
	if len(conds) > 0 {
		query += " WHERE " + strings.Join(conds, " AND ")
	}
	query += " ORDER BY date, time"

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookings := []Booking{}
	for rows.Next() {
		var booking Booking
		var startUTC sql.NullString
		var venueTZ sql.NullString
		var price sql.NullFloat64
		if err := rows.Scan(
			&booking.ID,
			&booking.VenueAlias,
			&booking.VenueName,
			&booking.VenueID,
			&booking.Court,
			&booking.Date,
			&booking.Time,
			&startUTC,
			&venueTZ,
			&booking.Duration,
			&price,
			&booking.BookedAt,
			&booking.Source,
		); err != nil {
			return nil, err
		}
		if startUTC.Valid {
			booking.StartUTC = startUTC.String
		}
		if venueTZ.Valid {
			booking.VenueTimezone = venueTZ.String
		}
		if price.Valid {
			booking.Price = price.Float64
		}
		bookings = append(bookings, booking)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if filter.From == "" && filter.To == "" {
		if filter.Past || filter.Upcoming {
			return filterByTime(bookings, filter), nil
		}
	}
	return bookings, nil
}

func filterByTime(bookings []Booking, filter BookingFilter) []Booking {
	filtered := make([]Booking, 0, len(bookings))
	for _, booking := range bookings {
		if booking.Date != filter.NowDate {
			filtered = append(filtered, booking)
			continue
		}
		if filter.NowTime == "" {
			filtered = append(filtered, booking)
			continue
		}
		if filter.Past {
			if booking.Time < filter.NowTime {
				filtered = append(filtered, booking)
			}
			continue
		}
		if filter.Upcoming {
			if booking.Time >= filter.NowTime {
				filtered = append(filtered, booking)
			}
		}
	}
	return filtered
}
