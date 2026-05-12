package hosts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hostswitch/backend/internal/store"
)

func TestReadEntriesImportsValidNonSystemHosts(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	content := strings.Join([]string{
		"127.0.0.1 localhost",
		"127.0.0.1 voxera.local whisper.local # local stack",
		"192.168.1.20 grafana.local",
		"not-an-ip ignored.local",
		"# BEGIN HOSTSWITCH",
		"127.0.0.1 managed.local",
		"# END HOSTSWITCH",
	}, "\n")
	if err := os.WriteFile(hostsPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := Manager{HostsPath: hostsPath}.ReadEntries()
	if err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Domain] = entry.IPAddress
		if entry.Source != "system" || entry.Status != "enabled" {
			t.Fatalf("unexpected imported metadata: %+v", entry)
		}
	}

	for domain, ip := range map[string]string{
		"voxera.local":   "127.0.0.1",
		"whisper.local":  "127.0.0.1",
		"grafana.local":  "192.168.1.20",
	} {
		if got[domain] != ip {
			t.Fatalf("expected %s -> %s, got %q", domain, ip, got[domain])
		}
	}
	if _, ok := got["localhost"]; ok {
		t.Fatal("localhost should not be imported")
	}
	if _, ok := got["managed.local"]; ok {
		t.Fatal("existing HostSwitch managed entries should not be imported from the raw hosts file")
	}
}

func TestApplyMovesManagedDomainsOutOfUnmanagedBody(t *testing.T) {
	dir := t.TempDir()
	hostsPath := filepath.Join(dir, "hosts")
	content := "127.0.0.1 localhost\n127.0.0.1 voxera.local keep.local # local stack\n"
	if err := os.WriteFile(hostsPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	manager := Manager{HostsPath: hostsPath, BackupDir: filepath.Join(dir, "backups")}
	err := manager.Apply([]store.HostEntry{{
		Domain:    "voxera.local",
		IPAddress: "127.0.0.1",
		GroupName: "Imported Hosts",
		Status:    "enabled",
		Source:    "system",
	}})
	if err != nil {
		t.Fatal(err)
	}

	applied, err := os.ReadFile(hostsPath)
	if err != nil {
		t.Fatal(err)
	}
	text := string(applied)
	if !strings.Contains(text, "127.0.0.1\tkeep.local\t# local stack") {
		t.Fatalf("expected unmanaged domains to remain, got:\n%s", text)
	}
	if strings.Count(text, "voxera.local") != 1 {
		t.Fatalf("expected imported domain to appear once in managed block, got:\n%s", text)
	}
	if !strings.Contains(text, "# BEGIN HOSTSWITCH") || !strings.Contains(text, "# END HOSTSWITCH") {
		t.Fatalf("expected managed block, got:\n%s", text)
	}
}

func TestClassifyEntryAssignsCategoryAndTags(t *testing.T) {
	group, tags := ClassifyEntry(store.HostEntry{Domain: "kubernetes.docker.internal", IPAddress: "127.0.0.1"})
	if group != "Docker & Kubernetes" || !strings.Contains(tags, "container") {
		t.Fatalf("unexpected docker classification: %s %s", group, tags)
	}

	group, tags = ClassifyEntry(store.HostEntry{Domain: "sonarqube.local", IPAddress: "127.0.0.1"})
	if group != "Monitoring" || !strings.Contains(tags, "monitoring") {
		t.Fatalf("unexpected monitoring classification: %s %s", group, tags)
	}
}
