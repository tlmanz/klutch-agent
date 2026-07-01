// Package store is the agent's local persistence, backed by SQLite via the
// pure-Go modernc.org/sqlite driver (no CGO, so it does not disturb the build).
// It keeps a rolling history of print jobs, the last-seen printer set, key/value
// settings (server URL, update preferences), and self-update state, so the UI
// can show what happened even across restarts.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Settings keys persisted in the settings table.
const (
	KeyServer           = "server"
	KeyToken            = "device_token"
	KeyUpdateURL        = "update_url"
	KeyAutoUpdate       = "auto_update"       // "1" / "0"
	KeyAutostart        = "autostart"         // "1" / "0"
	KeyAvailableVersion = "available_version" // last version the manifest advertised
	KeyLastCheck        = "last_update_check" // unix seconds
)

// Store is a handle to the agent's SQLite database.
type Store struct {
	db *sql.DB
}

// JobRecord is one print job's outcome, as shown in the Jobs tab.
type JobRecord struct {
	ID          string
	Printer     string
	Kind        string
	DocumentRef string
	Bytes       int
	Status      string // "ok" | "failed"
	Error       string
	ReceivedAt  time.Time
	FinishedAt  time.Time
}

// PrinterRecord is a printer the agent last advertised.
type PrinterRecord struct {
	Name        string
	Description string
	LastSeen    time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS jobs (
    id           TEXT PRIMARY KEY,
    printer      TEXT NOT NULL,
    kind         TEXT NOT NULL,
    document_ref TEXT NOT NULL DEFAULT '',
    bytes        INTEGER NOT NULL DEFAULT 0,
    status       TEXT NOT NULL,
    error        TEXT NOT NULL DEFAULT '',
    received_at  INTEGER NOT NULL,
    finished_at  INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_finished ON jobs(finished_at DESC);
CREATE TABLE IF NOT EXISTS printers (
    name        TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT '',
    last_seen   INTEGER NOT NULL
);
`

// Open opens (creating if needed) the SQLite database at path and applies the
// schema. WAL mode + a busy timeout keep the UI reads from blocking the agent's
// writes.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one writer; simplest correct choice for an embedded store
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Setting returns a stored setting, or ("", nil) if it is not set.
func (s *Store) Setting(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

// SetSetting upserts a setting.
func (s *Store) SetSetting(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO settings(key, value) VALUES(?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// RecordJob inserts (or replaces) a job outcome. Replacing on the same id keeps
// a redelivered job (same idempotency key) from creating a duplicate row.
func (s *Store) RecordJob(j JobRecord) error {
	_, err := s.db.Exec(
		`INSERT INTO jobs(id, printer, kind, document_ref, bytes, status, error, received_at, finished_at)
		 VALUES(?,?,?,?,?,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   printer=excluded.printer, kind=excluded.kind, document_ref=excluded.document_ref,
		   bytes=excluded.bytes, status=excluded.status, error=excluded.error,
		   finished_at=excluded.finished_at`,
		j.ID, j.Printer, j.Kind, j.DocumentRef, j.Bytes, j.Status, j.Error,
		j.ReceivedAt.Unix(), j.FinishedAt.Unix())
	return err
}

// RecentJobs returns the most recently finished jobs, newest first.
func (s *Store) RecentJobs(limit int) ([]JobRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, printer, kind, document_ref, bytes, status, error, received_at, finished_at
		   FROM jobs ORDER BY finished_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []JobRecord
	for rows.Next() {
		var j JobRecord
		var recv, fin int64
		if err := rows.Scan(&j.ID, &j.Printer, &j.Kind, &j.DocumentRef, &j.Bytes, &j.Status, &j.Error, &recv, &fin); err != nil {
			return nil, err
		}
		j.ReceivedAt = time.Unix(recv, 0)
		j.FinishedAt = time.Unix(fin, 0)
		out = append(out, j)
	}
	return out, rows.Err()
}

// JobCounts returns totals of ok and failed jobs.
func (s *Store) JobCounts() (ok, failed int, err error) {
	err = s.db.QueryRow(
		`SELECT
		   COALESCE(SUM(CASE WHEN status='ok'     THEN 1 ELSE 0 END), 0),
		   COALESCE(SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END), 0)
		 FROM jobs`).Scan(&ok, &failed)
	return ok, failed, err
}

// UpsertPrinters records the currently-advertised printer set, stamping each with
// seenAt. Printers no longer present keep their older last_seen (the UI can show
// staleness); the agent only ever advertises what the OS reports.
func (s *Store) UpsertPrinters(printers []PrinterRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, p := range printers {
		if _, err := tx.Exec(
			`INSERT INTO printers(name, description, last_seen) VALUES(?,?,?)
			 ON CONFLICT(name) DO UPDATE SET description=excluded.description, last_seen=excluded.last_seen`,
			p.Name, p.Description, p.LastSeen.Unix()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Printers returns every printer ever advertised, most recently seen first.
func (s *Store) Printers() ([]PrinterRecord, error) {
	rows, err := s.db.Query(`SELECT name, description, last_seen FROM printers ORDER BY last_seen DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PrinterRecord
	for rows.Next() {
		var p PrinterRecord
		var seen int64
		if err := rows.Scan(&p.Name, &p.Description, &seen); err != nil {
			return nil, err
		}
		p.LastSeen = time.Unix(seen, 0)
		out = append(out, p)
	}
	return out, rows.Err()
}
