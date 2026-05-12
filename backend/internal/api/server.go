package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"hostswitch/backend/internal/hosts"
	"hostswitch/backend/internal/store"
)

type Config struct {
	Addr          string
	DBPath        string
	DataDir       string
	HostsPath     string
	AllowedOrigin string
}

type Server struct {
	cfg   Config
	store *store.Store
	hosts hosts.Manager
}

func NewServer(cfg Config, st *store.Store) *Server {
	return &Server{
		cfg:   cfg,
		store: st,
		hosts: hosts.Manager{
			HostsPath: cfg.HostsPath,
			BackupDir: filepath.Join(cfg.DataDir, "backups"),
		},
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/environments", s.listEnvironments)
	mux.HandleFunc("POST /api/environments", s.createEnvironment)
	mux.HandleFunc("POST /api/environments/switch", s.switchEnvironment)
	mux.HandleFunc("GET /api/groups", s.listGroups)
	mux.HandleFunc("POST /api/groups", s.createGroup)
	mux.HandleFunc("PATCH /api/groups/{id}", s.updateGroup)
	mux.HandleFunc("GET /api/hosts", s.listHosts)
	mux.HandleFunc("POST /api/hosts", s.createHost)
	mux.HandleFunc("POST /api/hosts/import-system", s.importSystemHosts)
	mux.HandleFunc("PATCH /api/hosts/{id}", s.updateHost)
	mux.HandleFunc("DELETE /api/hosts/{id}", s.deleteHost)
	mux.HandleFunc("POST /api/apply", s.applyHosts)
	mux.HandleFunc("GET /api/backups", s.listBackups)
	mux.HandleFunc("POST /api/backups", s.createBackup)
	mux.HandleFunc("POST /api/backups/restore", s.restoreBackup)
	mux.HandleFunc("GET /api/docker/discover", s.discoverDocker)
	return s.cors(mux)
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.cfg.AllowedOrigin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listEnvironments(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Environments()
	respond(w, items, err)
}

func (s *Server) createEnvironment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.CreateEnvironment(strings.TrimSpace(input.Name))
	respond(w, item, err)
}

func (s *Server) switchEnvironment(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Environment string `json:"environment"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.store.SwitchEnvironment(strings.TrimSpace(input.Environment)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.applyWithBackup(w, "environment-switch")
}

func (s *Server) listGroups(w http.ResponseWriter, r *http.Request) {
	environmentID, err := queryInt(r, "environment_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.store.Groups(environmentID)
	respond(w, items, err)
}

func (s *Server) createGroup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		EnvironmentID int64  `json:"environment_id"`
		Name          string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.CreateGroup(input.EnvironmentID, strings.TrimSpace(input.Name))
	respond(w, item, err)
}

func (s *Server) updateGroup(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input struct {
		Name      *string `json:"name"`
		IsEnabled *bool   `json:"is_enabled"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.UpdateGroup(id, input.Name, input.IsEnabled)
	respond(w, item, err)
}

func (s *Server) listHosts(w http.ResponseWriter, r *http.Request) {
	environmentID, err := queryInt(r, "environment_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	items, err := s.store.Hosts(environmentID)
	respond(w, items, err)
}

func (s *Server) createHost(w http.ResponseWriter, r *http.Request) {
	var input store.HostEntry
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.hosts.ValidateEntry(input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.store.CreateHost(input)
	respond(w, item, err)
}

func (s *Server) importSystemHosts(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Scope string `json:"scope"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}
	if input.Scope == "" {
		input.Scope = "active"
	}
	entries, err := s.hosts.ReadEntries()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if len(entries) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"imported": 0, "message": "no importable hosts found"})
		return
	}

	backupPath, err := s.hosts.Backup("manual-import")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if _, err := s.store.CreateBackup(backupPath, "manual-import"); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	targets, err := s.importTargets(input.Scope)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	imported, err := s.importEntriesIntoEnvironments(entries, targets)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"imported": imported, "scope": input.Scope, "environments": targets})
}

func (s *Server) importTargets(scope string) ([]store.Environment, error) {
	switch scope {
	case "active":
		activeEnv, err := s.store.ActiveEnvironment()
		if err != nil {
			return nil, err
		}
		return []store.Environment{activeEnv}, nil
	case "all":
		return s.store.Environments()
	default:
		return nil, errors.New("scope must be active or all")
	}
}

func (s *Server) importEntriesIntoEnvironments(entries []store.HostEntry, environments []store.Environment) (int, error) {
	total := 0
	for _, environment := range environments {
		grouped := map[string][]store.HostEntry{}
		for _, entry := range entries {
			groupName, tags := hosts.ClassifyEntry(entry)
			entry.Tags = tags
			grouped[groupName] = append(grouped[groupName], entry)
		}
		for groupName, groupEntries := range grouped {
			group, err := s.store.EnsureGroup(environment.ID, groupName)
			if err != nil {
				return 0, err
			}
			imported, err := s.store.ImportHosts(group.ID, groupEntries)
			if err != nil {
				return 0, err
			}
			total += imported
		}
	}
	return total, nil
}

func (s *Server) updateHost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	var input store.HostEntry
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if input.Domain != "" || input.IPAddress != "" || input.Status != "" {
		current, _ := s.store.Host(id)
		if input.Domain != "" {
			current.Domain = input.Domain
		}
		if input.IPAddress != "" {
			current.IPAddress = input.IPAddress
		}
		if input.Status != "" {
			current.Status = input.Status
		}
		if err := s.hosts.ValidateEntry(current); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	item, err := s.store.UpdateHost(id, input)
	respond(w, item, err)
}

func (s *Server) deleteHost(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	respond(w, map[string]bool{"deleted": true}, s.store.DeleteHost(id))
}

func (s *Server) applyHosts(w http.ResponseWriter, r *http.Request) {
	s.applyWithBackup(w, "manual-apply")
}

func (s *Server) applyWithBackup(w http.ResponseWriter, reason string) {
	backupPath, err := s.hosts.Backup(reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	backup, err := s.store.CreateBackup(backupPath, reason)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	entries, err := s.store.ActiveHosts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if err := s.hosts.Apply(entries); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"applied": true, "backup": backup})
}

func (s *Server) listBackups(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.Backups()
	respond(w, items, err)
}

func (s *Server) createBackup(w http.ResponseWriter, r *http.Request) {
	backupPath, err := s.hosts.Backup("manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	item, err := s.store.CreateBackup(backupPath, "manual")
	respond(w, item, err)
}

func (s *Server) restoreBackup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		BackupID int64 `json:"backup_id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	backup, err := s.store.Backup(input.BackupID)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	if err := s.hosts.Restore(backup.FilePath); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"restored": true})
}

func (s *Server) discoverDocker(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]string{
		{"name": "grafana", "suggested_domain": "grafana.local", "source": "docker-placeholder"},
		{"name": "prometheus", "suggested_domain": "prometheus.local", "source": "docker-placeholder"},
	})
}

func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	if err == nil {
		err = errors.New("unknown error")
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func queryInt(r *http.Request, key string) (int64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, errors.New("missing " + key)
	}
	return strconv.ParseInt(value, 10, 64)
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}
