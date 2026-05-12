package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"hostswitch/backend/internal/api"
	"hostswitch/backend/internal/hosts"
	"hostswitch/backend/internal/store"
)

func main() {
	cfg := api.Config{
		Addr:          env("HOSTSWITCH_ADDR", ":8080"),
		DBPath:        env("HOSTSWITCH_DB_PATH", filepath.Join("data", "hostswitch.db")),
		DataDir:       env("HOSTSWITCH_DATA_DIR", "data"),
		HostsPath:     env("HOSTSWITCH_HOSTS_PATH", filepath.Join("data", "hosts.preview")),
		AllowedOrigin: env("HOSTSWITCH_ALLOWED_ORIGIN", "http://localhost:5173"),
	}

	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	db, err := store.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer db.Close()

	if err := db.SeedDefaults(); err != nil {
		log.Fatalf("seed defaults: %v", err)
	}

	hostManager := hosts.Manager{
		HostsPath: cfg.HostsPath,
		BackupDir: filepath.Join(cfg.DataDir, "backups"),
	}
	if err := importExistingHosts(db, hostManager); err != nil {
		log.Printf("initial hosts import skipped: %v", err)
	}

	server := api.NewServer(cfg, db)
	log.Printf("HostSwitch listening on %s", cfg.Addr)
	if err := http.ListenAndServe(cfg.Addr, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func importExistingHosts(db *store.Store, hostManager hosts.Manager) error {
	count, err := db.HostCount()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	entries, err := hostManager.ReadEntries()
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	backupPath, err := hostManager.Backup("initial-import")
	if err != nil {
		return err
	}
	if _, err := db.CreateBackup(backupPath, "initial-import"); err != nil {
		return err
	}

	activeEnv, err := db.ActiveEnvironment()
	if err != nil {
		return err
	}
	imported := 0
	grouped := map[string][]store.HostEntry{}
	for _, entry := range entries {
		groupName, tags := hosts.ClassifyEntry(entry)
		entry.Tags = tags
		grouped[groupName] = append(grouped[groupName], entry)
	}
	for groupName, groupEntries := range grouped {
		group, err := db.EnsureGroup(activeEnv.ID, groupName)
		if err != nil {
			return err
		}
		count, err := db.ImportHosts(group.ID, groupEntries)
		if err != nil {
			return err
		}
		imported += count
	}
	log.Printf("imported %d existing hosts into %s", imported, activeEnv.Name)
	return nil
}

func env(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
