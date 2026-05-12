package main

import (
	"os"
	"path/filepath"
	"testing"

	"hostswitch/backend/internal/hosts"
	"hostswitch/backend/internal/store"
)

func TestImportExistingHostsSeedsActiveEnvironment(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	if err := os.WriteFile(hostsPath, []byte("127.0.0.1 voxera.local\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := store.Open(filepath.Join(dir, "hostswitch.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SeedDefaults(); err != nil {
		t.Fatal(err)
	}

	manager := hosts.Manager{HostsPath: hostsPath, BackupDir: filepath.Join(dir, "backups")}
	if err := importExistingHosts(db, manager); err != nil {
		t.Fatal(err)
	}

	active, err := db.ActiveEnvironment()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := db.Hosts(active.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one imported host, got %d: %+v", len(entries), entries)
	}
	if entries[0].Domain != "voxera.local" || entries[0].GroupName != "Developer Tools" || entries[0].Source != "system" || entries[0].Tags == "" {
		t.Fatalf("unexpected imported host: %+v", entries[0])
	}

	backups, err := db.Backups()
	if err != nil {
		t.Fatal(err)
	}
	if len(backups) != 1 || backups[0].TriggerReason != "initial-import" {
		t.Fatalf("expected initial-import backup, got %+v", backups)
	}
}
