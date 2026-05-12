package hosts

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"hostswitch/backend/internal/store"
)

const (
	beginMarker = "# BEGIN HOSTSWITCH"
	endMarker   = "# END HOSTSWITCH"
)

var domainPattern = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$|^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.local$`)

type Manager struct {
	HostsPath string
	BackupDir string
}

func (m Manager) ReadEntries() ([]store.HostEntry, error) {
	content, err := os.ReadFile(m.HostsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	lines := strings.Split(stripManagedBlock(string(content)), "\n")
	seen := map[string]bool{}
	entries := []store.HostEntry{}
	for _, line := range lines {
		entryText := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
		if entryText == "" {
			continue
		}
		fields := strings.Fields(entryText)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			continue
		}
		for _, domain := range fields[1:] {
			domain = strings.TrimSpace(domain)
			if domain == "" || isSystemHost(domain) {
				continue
			}
			entry := store.HostEntry{
				Domain:    domain,
				IPAddress: fields[0],
				Status:    "enabled",
				Source:    "system",
			}
			if err := m.ValidateEntry(entry); err != nil {
				continue
			}
			key := entry.IPAddress + "|" + entry.Domain
			if seen[key] {
				continue
			}
			seen[key] = true
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

func ClassifyEntry(entry store.HostEntry) (string, string) {
	domain := strings.ToLower(entry.Domain)
	ip := net.ParseIP(entry.IPAddress)
	tags := []string{"system", "imported"}

	if entry.IPAddress == "0.0.0.0" {
		return "Blocked Domains", strings.Join(append(tags, "blocked"), ",")
	}
	if strings.Contains(domain, "docker.internal") || strings.Contains(domain, "kubernetes") {
		return "Docker & Kubernetes", strings.Join(append(tags, "container", "local"), ",")
	}
	if strings.Contains(domain, "grafana") || strings.Contains(domain, "prometheus") || strings.Contains(domain, "sonarqube") {
		return "Monitoring", strings.Join(append(tags, "monitoring"), ",")
	}
	if strings.Contains(domain, "mailhog") || strings.Contains(domain, "n8n") || strings.Contains(domain, "whisper") || strings.Contains(domain, "voxera") {
		return "Developer Tools", strings.Join(append(tags, "tooling"), ",")
	}
	if ip != nil && ip.IsLoopback() && strings.HasSuffix(domain, ".local") {
		return "Local Development", strings.Join(append(tags, "local", "loopback"), ",")
	}
	if ip != nil && ip.IsLoopback() {
		return "Local Overrides", strings.Join(append(tags, "loopback", "override"), ",")
	}
	return "External Overrides", strings.Join(append(tags, "external", "override"), ",")
}

func (m Manager) ValidateEntry(entry store.HostEntry) error {
	if net.ParseIP(entry.IPAddress) == nil {
		return fmt.Errorf("invalid IP address: %s", entry.IPAddress)
	}
	if !domainPattern.MatchString(entry.Domain) {
		return fmt.Errorf("invalid domain: %s", entry.Domain)
	}
	if entry.Status != "" && entry.Status != "enabled" && entry.Status != "disabled" {
		return fmt.Errorf("invalid status: %s", entry.Status)
	}
	return nil
}

func (m Manager) Backup(reason string) (string, error) {
	if err := os.MkdirAll(m.BackupDir, 0o755); err != nil {
		return "", err
	}
	content, err := os.ReadFile(m.HostsPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	name := fmt.Sprintf("%s-%s.hosts", time.Now().UTC().Format("20060102T150405Z"), slug(reason))
	path := filepath.Join(m.BackupDir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func (m Manager) Restore(backupPath string) error {
	content, err := os.ReadFile(backupPath)
	if err != nil {
		return err
	}
	return os.WriteFile(m.HostsPath, content, 0o644)
}

func (m Manager) Apply(entries []store.HostEntry) error {
	for _, entry := range entries {
		if err := m.ValidateEntry(entry); err != nil {
			return err
		}
	}
	existing, err := os.ReadFile(m.HostsPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	body := stripManagedBlock(string(existing))
	body = stripManagedEntries(body, entries)
	if strings.TrimSpace(body) != "" {
		body = strings.TrimRight(body, "\r\n") + "\n\n"
	}
	var managed strings.Builder
	managed.WriteString(beginMarker + "\n")
	managed.WriteString("# Managed by HostSwitch. Manual changes inside this block will be replaced.\n")
	for _, entry := range entries {
		managed.WriteString(fmt.Sprintf("%s\t%s\t# %s / %s\n", entry.IPAddress, entry.Domain, entry.GroupName, entry.Source))
	}
	managed.WriteString(endMarker + "\n")
	return os.WriteFile(m.HostsPath, []byte(body+managed.String()), 0o644)
}

func stripManagedBlock(input string) string {
	start := strings.Index(input, beginMarker)
	if start == -1 {
		return input
	}
	end := strings.Index(input[start:], endMarker)
	if end == -1 {
		return strings.TrimSpace(input[:start]) + "\n"
	}
	end += start + len(endMarker)
	return input[:start] + input[end:]
}

func stripManagedEntries(input string, entries []store.HostEntry) string {
	managedDomains := map[string]bool{}
	for _, entry := range entries {
		managedDomains[strings.ToLower(entry.Domain)] = true
	}
	if len(managedDomains) == 0 {
		return input
	}

	var output []string
	for _, line := range strings.Split(input, "\n") {
		rawLine := strings.TrimRight(line, "\r")
		entryPart, commentPart, hasComment := strings.Cut(rawLine, "#")
		fields := strings.Fields(entryPart)
		if len(fields) < 2 || net.ParseIP(fields[0]) == nil {
			output = append(output, rawLine)
			continue
		}

		remainingDomains := []string{}
		for _, domain := range fields[1:] {
			if !managedDomains[strings.ToLower(domain)] {
				remainingDomains = append(remainingDomains, domain)
			}
		}
		if len(remainingDomains) == 0 {
			if strings.TrimSpace(commentPart) != "" && !hasComment {
				output = append(output, rawLine)
			}
			continue
		}

		rebuilt := fields[0] + "\t" + strings.Join(remainingDomains, " ")
		if hasComment {
			rebuilt += "\t#" + commentPart
		}
		output = append(output, rebuilt)
	}
	return strings.Join(output, "\n")
}

func slug(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "manual"
	}
	return value
}

func isSystemHost(domain string) bool {
	switch strings.ToLower(domain) {
	case "localhost", "localhost.localdomain", "broadcasthost", "ip6-localhost", "ip6-loopback", "ip6-localnet", "ip6-mcastprefix", "ip6-allnodes", "ip6-allrouters":
		return true
	default:
		return false
	}
}
