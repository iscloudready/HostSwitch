package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type Environment struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type Group struct {
	ID            int64  `json:"id"`
	EnvironmentID int64  `json:"environment_id"`
	Name          string `json:"name"`
	IsEnabled     bool   `json:"is_enabled"`
}

type HostEntry struct {
	ID        int64     `json:"id"`
	GroupID   int64     `json:"group_id"`
	GroupName string    `json:"group_name,omitempty"`
	Domain    string    `json:"domain"`
	IPAddress string    `json:"ip_address"`
	Status    string    `json:"status"`
	Source    string    `json:"source"`
	Tags      string    `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
}

type Backup struct {
	ID            int64     `json:"id"`
	FilePath      string    `json:"file_path"`
	CreatedAt     time.Time `json:"created_at"`
	TriggerReason string    `json:"trigger_reason"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS environments (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL UNIQUE,
	is_active INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS groups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	environment_id INTEGER NOT NULL,
	name TEXT NOT NULL,
	is_enabled INTEGER NOT NULL DEFAULT 1,
	UNIQUE(environment_id, name),
	FOREIGN KEY(environment_id) REFERENCES environments(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS host_entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	group_id INTEGER NOT NULL,
	domain TEXT NOT NULL,
	ip_address TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'enabled',
	source TEXT NOT NULL DEFAULT 'manual',
	tags TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	UNIQUE(group_id, domain),
	FOREIGN KEY(group_id) REFERENCES groups(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS backups (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	file_path TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	trigger_reason TEXT NOT NULL
);`)
	if err != nil {
		return err
	}
	return s.addColumnIfMissing("host_entries", "tags", "TEXT NOT NULL DEFAULT ''")
}

func (s *Store) addColumnIfMissing(table, column, definition string) error {
	rows, err := s.db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func (s *Store) SeedDefaults() error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM environments`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i, name := range []string{"DEV", "STAGING", "PROD"} {
		active := 0
		if i == 0 {
			active = 1
		}
		res, err := tx.Exec(`INSERT INTO environments(name, is_active) VALUES(?, ?)`, name, active)
		if err != nil {
			return err
		}
		envID, _ := res.LastInsertId()
		for _, group := range []string{"AI Stack", "Monitoring", "Tools"} {
			if _, err := tx.Exec(`INSERT INTO groups(environment_id, name, is_enabled) VALUES(?, ?, 1)`, envID, group); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func (s *Store) Environments() ([]Environment, error) {
	rows, err := s.db.Query(`SELECT id, name, is_active, created_at FROM environments ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Environment{}
	for rows.Next() {
		var item Environment
		if err := rows.Scan(&item.ID, &item.Name, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) HostCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM host_entries`).Scan(&count)
	return count, err
}

func (s *Store) ActiveEnvironment() (Environment, error) {
	var item Environment
	err := s.db.QueryRow(`SELECT id, name, is_active, created_at FROM environments WHERE is_active = 1 LIMIT 1`).Scan(&item.ID, &item.Name, &item.IsActive, &item.CreatedAt)
	return item, err
}

func (s *Store) CreateEnvironment(name string) (Environment, error) {
	res, err := s.db.Exec(`INSERT INTO environments(name) VALUES(?)`, name)
	if err != nil {
		return Environment{}, err
	}
	id, _ := res.LastInsertId()
	return s.Environment(id)
}

func (s *Store) Environment(id int64) (Environment, error) {
	var item Environment
	err := s.db.QueryRow(`SELECT id, name, is_active, created_at FROM environments WHERE id = ?`, id).Scan(&item.ID, &item.Name, &item.IsActive, &item.CreatedAt)
	return item, err
}

func (s *Store) SwitchEnvironment(name string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE environments SET is_active = CASE WHEN name = ? THEN 1 ELSE 0 END`, name)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return errors.New("environment not found")
	}
	return tx.Commit()
}

func (s *Store) Groups(environmentID int64) ([]Group, error) {
	rows, err := s.db.Query(`SELECT id, environment_id, name, is_enabled FROM groups WHERE environment_id = ? ORDER BY name`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Group{}
	for rows.Next() {
		var item Group
		if err := rows.Scan(&item.ID, &item.EnvironmentID, &item.Name, &item.IsEnabled); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateGroup(environmentID int64, name string) (Group, error) {
	res, err := s.db.Exec(`INSERT INTO groups(environment_id, name) VALUES(?, ?)`, environmentID, name)
	if err != nil {
		return Group{}, err
	}
	id, _ := res.LastInsertId()
	return s.Group(id)
}

func (s *Store) EnsureGroup(environmentID int64, name string) (Group, error) {
	var item Group
	err := s.db.QueryRow(`SELECT id, environment_id, name, is_enabled FROM groups WHERE environment_id = ? AND name = ?`, environmentID, name).Scan(&item.ID, &item.EnvironmentID, &item.Name, &item.IsEnabled)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Group{}, err
	}
	return s.CreateGroup(environmentID, name)
}

func (s *Store) Group(id int64) (Group, error) {
	var item Group
	err := s.db.QueryRow(`SELECT id, environment_id, name, is_enabled FROM groups WHERE id = ?`, id).Scan(&item.ID, &item.EnvironmentID, &item.Name, &item.IsEnabled)
	return item, err
}

func (s *Store) UpdateGroup(id int64, name *string, isEnabled *bool) (Group, error) {
	current, err := s.Group(id)
	if err != nil {
		return Group{}, err
	}
	if name != nil {
		current.Name = *name
	}
	if isEnabled != nil {
		current.IsEnabled = *isEnabled
	}
	_, err = s.db.Exec(`UPDATE groups SET name = ?, is_enabled = ? WHERE id = ?`, current.Name, current.IsEnabled, id)
	if err != nil {
		return Group{}, err
	}
	return s.Group(id)
}

func (s *Store) Hosts(environmentID int64) ([]HostEntry, error) {
	rows, err := s.db.Query(`
SELECT h.id, h.group_id, g.name, h.domain, h.ip_address, h.status, h.source, h.tags, h.created_at
FROM host_entries h
JOIN groups g ON g.id = h.group_id
WHERE g.environment_id = ?
ORDER BY g.name, h.domain`, environmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HostEntry{}
	for rows.Next() {
		var item HostEntry
		if err := rows.Scan(&item.ID, &item.GroupID, &item.GroupName, &item.Domain, &item.IPAddress, &item.Status, &item.Source, &item.Tags, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ActiveHosts() ([]HostEntry, error) {
	rows, err := s.db.Query(`
SELECT h.id, h.group_id, g.name, h.domain, h.ip_address, h.status, h.source, h.tags, h.created_at
FROM host_entries h
JOIN groups g ON g.id = h.group_id
JOIN environments e ON e.id = g.environment_id
WHERE e.is_active = 1 AND g.is_enabled = 1 AND h.status = 'enabled'
ORDER BY g.name, h.domain`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []HostEntry{}
	for rows.Next() {
		var item HostEntry
		if err := rows.Scan(&item.ID, &item.GroupID, &item.GroupName, &item.Domain, &item.IPAddress, &item.Status, &item.Source, &item.Tags, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CreateHost(input HostEntry) (HostEntry, error) {
	if input.Status == "" {
		input.Status = "enabled"
	}
	if input.Source == "" {
		input.Source = "manual"
	}
	res, err := s.db.Exec(`INSERT INTO host_entries(group_id, domain, ip_address, status, source, tags) VALUES(?, ?, ?, ?, ?, ?)`, input.GroupID, input.Domain, input.IPAddress, input.Status, input.Source, input.Tags)
	if err != nil {
		return HostEntry{}, err
	}
	id, _ := res.LastInsertId()
	return s.Host(id)
}

func (s *Store) ImportHosts(groupID int64, entries []HostEntry) (int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	imported := 0
	for _, entry := range entries {
		if entry.Status == "" {
			entry.Status = "enabled"
		}
		if entry.Source == "" {
			entry.Source = "system"
		}
		res, err := tx.Exec(`INSERT OR IGNORE INTO host_entries(group_id, domain, ip_address, status, source, tags) VALUES(?, ?, ?, ?, ?, ?)`, groupID, entry.Domain, entry.IPAddress, entry.Status, entry.Source, entry.Tags)
		if err != nil {
			return 0, err
		}
		affected, _ := res.RowsAffected()
		imported += int(affected)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return imported, nil
}

func (s *Store) Host(id int64) (HostEntry, error) {
	var item HostEntry
	err := s.db.QueryRow(`
SELECT h.id, h.group_id, g.name, h.domain, h.ip_address, h.status, h.source, h.tags, h.created_at
FROM host_entries h JOIN groups g ON g.id = h.group_id WHERE h.id = ?`, id).Scan(&item.ID, &item.GroupID, &item.GroupName, &item.Domain, &item.IPAddress, &item.Status, &item.Source, &item.Tags, &item.CreatedAt)
	return item, err
}

func (s *Store) UpdateHost(id int64, patch HostEntry) (HostEntry, error) {
	current, err := s.Host(id)
	if err != nil {
		return HostEntry{}, err
	}
	if patch.GroupID != 0 {
		current.GroupID = patch.GroupID
	}
	if patch.Domain != "" {
		current.Domain = patch.Domain
	}
	if patch.IPAddress != "" {
		current.IPAddress = patch.IPAddress
	}
	if patch.Status != "" {
		current.Status = patch.Status
	}
	if patch.Source != "" {
		current.Source = patch.Source
	}
	if patch.Tags != "" {
		current.Tags = patch.Tags
	}
	_, err = s.db.Exec(`UPDATE host_entries SET group_id = ?, domain = ?, ip_address = ?, status = ?, source = ?, tags = ? WHERE id = ?`, current.GroupID, current.Domain, current.IPAddress, current.Status, current.Source, current.Tags, id)
	if err != nil {
		return HostEntry{}, err
	}
	return s.Host(id)
}

func (s *Store) DeleteHost(id int64) error {
	res, err := s.db.Exec(`DELETE FROM host_entries WHERE id = ?`, id)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("host entry %d not found", id)
	}
	return nil
}

func (s *Store) CreateBackup(path, reason string) (Backup, error) {
	res, err := s.db.Exec(`INSERT INTO backups(file_path, trigger_reason) VALUES(?, ?)`, path, reason)
	if err != nil {
		return Backup{}, err
	}
	id, _ := res.LastInsertId()
	return s.Backup(id)
}

func (s *Store) Backup(id int64) (Backup, error) {
	var item Backup
	err := s.db.QueryRow(`SELECT id, file_path, created_at, trigger_reason FROM backups WHERE id = ?`, id).Scan(&item.ID, &item.FilePath, &item.CreatedAt, &item.TriggerReason)
	return item, err
}

func (s *Store) Backups() ([]Backup, error) {
	rows, err := s.db.Query(`SELECT id, file_path, created_at, trigger_reason FROM backups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Backup{}
	for rows.Next() {
		var item Backup
		if err := rows.Scan(&item.ID, &item.FilePath, &item.CreatedAt, &item.TriggerReason); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
