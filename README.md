# HostSwitch

HostSwitch is a local domain control center for managing hosts file entries, environments, groups, backups, and Docker-aware service routing.

The Phase 1 scaffold in this repository includes:

- Go backend API with SQLite persistence
- Safe hosts file rendering behind a single host-file manager
- Automatic backup records before apply/restore actions
- Environment, group, and host entry CRUD
- React + Tailwind dashboard shell
- Docker and Docker Compose deployment files

## Repository Layout

```text
backend/      Go API, persistence, backup engine, host file manager
frontend/     React dashboard
docker-compose.yml
```

## Local Development

The main entry point is the PowerShell bootstrap script:

```powershell
./scripts/bootstrap.ps1
```

Common tasks:

```powershell
./scripts/bootstrap.ps1 -Task deps
./scripts/bootstrap.ps1 -Task build
./scripts/bootstrap.ps1 -Task test
./scripts/bootstrap.ps1 -Task ui-smoke
./scripts/bootstrap.ps1 -Task dockerize
./scripts/bootstrap.ps1 -Task pack
./scripts/bootstrap.ps1 -Task dev
./scripts/bootstrap.ps1 -Task dev-down
./scripts/bootstrap.ps1 -Task deploy
```

The default `bootstrap` task installs dependencies, runs checks, and builds the backend Docker image. If Go is not installed locally, backend build/test tasks fall back to Docker. For CI-style clean frontend installs, add `-CleanInstall`; normal local runs use `npm install` so Windows does not have to delete locked binaries from `node_modules`.

```powershell
./scripts/bootstrap.ps1 -CleanInstall
./scripts/bootstrap.ps1 -Task deps -ForceDeps
```

For local development, `dev` starts the backend container on `http://localhost:18080` and the frontend on `http://localhost:5173`:

```powershell
./scripts/bootstrap.ps1 -Task dev
```

For dev mode, the script seeds `data/hosts.preview` from your OS hosts file and mounts `data/` into the backend container. This lets the UI show current hosts data without writing to the real system hosts file. To refresh the preview from the system hosts file:

```powershell
./scripts/bootstrap.ps1 -Task dev -RefreshHostsPreview
```

Backend:

```bash
cd backend
go mod download
go run ./cmd/hostswitch
```

Frontend:

```bash
cd frontend
npm install
npm run dev
```

By default the backend uses a local data directory and writes to `./data/hosts.preview` instead of the real system hosts file. Set `HOSTSWITCH_HOSTS_PATH` only when you are ready to run with elevated permissions.

On first startup, HostSwitch reads the configured hosts file, creates an `initial-import` backup, and imports valid non-system entries into the active environment. Imported entries are categorized into groups such as `Local Development`, `Docker & Kubernetes`, `Monitoring`, `Developer Tools`, `Blocked Domains`, and `External Overrides`, with tags saved on each host entry. Later apply operations move those imported domains into the HostSwitch managed block instead of duplicating them.

The current hosts file is treated as the current machine state, not automatically as `PROD`. Use `Import Current` to import into the active environment, or `Import All Envs` to copy the categorized current state into DEV, STAGING, and PROD.

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `HOSTSWITCH_ADDR` | `:8080` | Backend listen address |
| `HOSTSWITCH_DB_PATH` | `./data/hostswitch.db` | SQLite database path |
| `HOSTSWITCH_DATA_DIR` | `./data` | Data directory for backups and preview hosts |
| `HOSTSWITCH_HOSTS_PATH` | `./data/hosts.preview` | Hosts file target |
| `HOSTSWITCH_ALLOWED_ORIGIN` | `http://localhost:5173` | CORS origin for the frontend |

## API Snapshot

- `GET /api/health`
- `GET /api/environments`
- `POST /api/environments`
- `POST /api/environments/switch`
- `GET /api/groups?environment_id=...`
- `POST /api/groups`
- `PATCH /api/groups/{id}`
- `GET /api/hosts?environment_id=...`
- `POST /api/hosts`
- `POST /api/hosts/import-system`
- `PATCH /api/hosts/{id}`
- `DELETE /api/hosts/{id}`
- `POST /api/apply`
- `GET /api/backups`
- `POST /api/backups`
- `POST /api/backups/restore`
- `GET /api/docker/discover`

## Roadmap

Phase 1 focuses on the usable core: hosts rendering, backup engine, environment switching, group structure, dashboard UI, Docker deployment, and SQLite integration.

Phase 2 adds Docker discovery imports and label support.

Phase 3 adds API tokens and webhooks.

Phase 4 adds local DNS mode with wildcard domains and forwarding.
