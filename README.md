# HostSwitch

HostSwitch is a local domain control center for developers, DevOps engineers, and homelab builders. It gives you a modern dashboard for hosts file management, environment switching, grouped local domains, Docker-aware discovery, backups, and automation.

![HostSwitch dashboard](./docs/images/dashboard.png)

## Why HostSwitch

Editing a hosts file by hand is easy to get wrong and hard to share. HostSwitch wraps that workflow in a safer system:

- Import existing hosts entries into SQLite.
- Categorize entries into environments and groups.
- Switch between DEV, STAGING, and PROD without hand-editing system files.
- Preview and apply managed hosts output with backups.
- Discover Docker services and suggest local domains.
- Automate common workflows through a REST API.

The goal is simple: you should not need to manually edit your hosts file again.

## Current Features

- Environment management for DEV, STAGING, and PROD.
- Group management with enable and disable controls.
- Hosts import, categorization, tagging, search, and filtering.
- Automatic backup records for imported and applied hosts.
- Docker discovery endpoint and UI flow.
- Safe development mode using `data/hosts.preview`.
- SQLite persistence.
- Docker deployment support.
- PowerShell bootstrap script for dependencies, build, test, package, and dev tasks.
- GitHub Pages documentation site under `docs/`.

## Quick Start

### Prerequisites

- Docker Desktop or a compatible Docker engine.
- Node.js 20 or newer.
- PowerShell 7 is recommended on Windows.

### Run the App for Development

```powershell
./scripts/bootstrap.ps1 -Task dev
```

The frontend runs at:

```text
http://localhost:5173
```

The backend API runs at:

```text
http://localhost:18080
```

In development, HostSwitch uses `data/hosts.preview` instead of your real system hosts file.

### Run with Docker Compose

```powershell
docker compose up -d --build
```

## Bootstrap Tasks

The main project entry point is `scripts/bootstrap.ps1`.

```powershell
./scripts/bootstrap.ps1 -Task deps
./scripts/bootstrap.ps1 -Task build
./scripts/bootstrap.ps1 -Task test
./scripts/bootstrap.ps1 -Task ui-smoke
./scripts/bootstrap.ps1 -Task dockerize
./scripts/bootstrap.ps1 -Task pack
./scripts/bootstrap.ps1 -Task deploy
./scripts/bootstrap.ps1 -Task dev
./scripts/bootstrap.ps1 -Task dev-down
```

Useful dependency repair commands:

```powershell
./scripts/bootstrap.ps1 -Task deps -ForceDeps
./scripts/bootstrap.ps1 -CleanInstall
```

If Windows locks `frontend/node_modules/@esbuild/win32-x64/esbuild.exe`, close running Vite/Node processes and rerun the dependency task. Antivirus and editors can also hold that file briefly.

## Import Model

HostSwitch treats the configured hosts file as the current machine state, not automatically as production.

- `Import Current` imports valid entries into the active environment.
- `Import All Envs` distributes categorized entries across DEV, STAGING, and PROD.
- Imported entries are saved to SQLite with source and tag metadata.
- System and malformed entries are filtered out before storage.

Examples of generated groups include:

- AI Stack
- Monitoring
- Developer Tools
- Docker & Kubernetes
- Local Development
- External Overrides
- Blocked Domains

## Safety Model

Hosts file updates require elevated permissions on real machines. HostSwitch keeps the privileged surface narrow:

- The web UI calls the backend API.
- The backend validates domains and IP addresses.
- Backups are created before managed hosts output is applied.
- Development mode writes to `data/hosts.preview`.
- Production deployments should mount the intended hosts path explicitly and run with only the permissions required for that path.

Typical hosts paths:

```text
Windows: C:\Windows\System32\drivers\etc\hosts
Linux/macOS: /etc/hosts
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `HOSTSWITCH_ADDR` | `:8080` | Backend listen address |
| `HOSTSWITCH_DB_PATH` | `./data/hostswitch.db` | SQLite database path |
| `HOSTSWITCH_DATA_DIR` | `./data` | Data directory for backups and hosts preview |
| `HOSTSWITCH_HOSTS_PATH` | `./data/hosts.preview` | Hosts file path managed by the backend |
| `HOSTSWITCH_ALLOWED_ORIGIN` | `http://localhost:5173` | Allowed frontend origin |

## API Snapshot

```http
GET  /api/health
GET  /api/environments
POST /api/environments
POST /api/environments/switch

GET   /api/groups?environment_id=1
POST  /api/groups
PATCH /api/groups/{id}

GET    /api/hosts?environment_id=1
POST   /api/hosts
POST   /api/hosts/import-system
PATCH  /api/hosts/{id}
DELETE /api/hosts/{id}

POST /api/apply
GET  /api/backups
POST /api/backups
POST /api/backups/restore
GET  /api/docker/discover
```

## Repository Structure

```text
backend/              Go API, storage, hosts manager, Docker discovery
frontend/             React dashboard
scripts/              Bootstrap, smoke tests, docs screenshot tooling
docs/                 GitHub Pages site
docs/images/          Public-safe screenshots
data/                 Local development data, ignored by git
docker-compose.yml    Container deployment
```

## Documentation Screenshots

The checked-in GitHub Pages screenshots were generated from a sanitized demo hosts file. When regenerating screenshots, point the script at a demo backend instead of your live local data.

```powershell
node ./scripts/capture-docs-screenshots.mjs
```

The script expects a running frontend and backend. Set these if you use non-default ports:

```powershell
$env:HOSTSWITCH_UI_URL = "http://localhost:5175"
$env:HOSTSWITCH_API_URL = "http://localhost:18082"
```

## Verification

Before opening a pull request or publishing docs, run:

```powershell
./scripts/bootstrap.ps1 -Task test
./scripts/bootstrap.ps1 -Task ui-smoke
```

## Roadmap

- Docker label import rules.
- API tokens and webhooks.
- Local DNS mode with wildcard domains and forwarding.
- Git sync for team sharing.
- Kubernetes discovery.
- RBAC, SSO, and audit logs.

## License

MIT License.
