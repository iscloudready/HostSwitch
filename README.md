# HostSwitch

<div align="center">

# HostSwitch

### Modern Local Domain Control Center for Developers & DevOps

Manage hosts files, environments, Docker services, and local domain routing from one modern web interface.

---

<img src="./docs/images/dashboard-hero.png" alt="HostSwitch Dashboard" width="100%" />

</div>

---

# ✨ Features

## 🌍 Environment Switching

Instantly switch between:

- DEV
- STAGING
- PROD

without manually editing hosts files.

---

## 📁 Group-Based Organization

Organize domains into logical groups:

- AI Stack
- Monitoring
- Developer Tools
- Docker & Kubernetes
- Networking
- Blocked Domains

---

## 🐳 Docker Auto Discovery

Automatically detect running containers and generate local domains.

Example:

```text
grafana     → grafana.local
prometheus  → prometheus.local
ollama      → ollama.local
n8n         → n8n.local
```

---

## 💾 Automatic Backup & Restore

Every apply operation creates a backup automatically.

Rollback safely anytime.

---

## ⚡ Apply Hosts Safely

HostSwitch renders and applies hosts changes through a dedicated host manager.

No more manually editing:

```text
/etc/hosts
```

or

```text
C:\Windows\System32\drivers\etc\hosts
```

---

## 🔌 REST API

Automate HostSwitch using API endpoints.

Examples:

```http
GET  /api/environments
POST /api/environments/switch
GET  /api/hosts
POST /api/apply
```

---

## 🧠 Local DNS Mode (Roadmap)

Future versions will support:

- wildcard domains
- local DNS server
- DNS forwarding
- DNS caching

Example:

```text
*.local.dev
```

---

# 🚀 Why HostSwitch?

Most hosts management tools are:

- outdated
- desktop-only
- difficult to automate
- not container-aware
- lacking backups and organization

HostSwitch combines:

```text
Hosts Management
+ Docker Discovery
+ Environment Switching
+ Backups
+ API Automation
+ Modern UI
```

in one unified platform.

---

# 🖥 Dashboard Preview

## Main Dashboard

<img src="./docs/images/dashboard-hero.png" alt="Dashboard" width="100%" />

---

# ⚡ Quick Start

## Requirements

- Docker Desktop
- Node.js 20+
- PowerShell (Windows)

---

## Start Development Environment

```powershell
./scripts/bootstrap.ps1 -Task dev
```

Frontend:

```text
http://localhost:5173
```

Backend API:

```text
http://localhost:18080
```

---

# 🐳 Docker Deployment

## Run with Docker

```bash
docker compose up -d
```

---

# 🏗 Repository Structure

```text
backend/              Go API and backend services
frontend/             React dashboard
scripts/              Bootstrap and development scripts
data/                 Local development data and preview hosts
docs/images/          Screenshots and demo assets
docker-compose.yml
```

---

# 🛠 Local Development

The main entry point is the PowerShell bootstrap script:

```powershell
./scripts/bootstrap.ps1
```

---

## Common Tasks

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

---

## Clean Install

For CI-style clean frontend installs:

```powershell
./scripts/bootstrap.ps1 -CleanInstall
```

Force dependency refresh:

```powershell
./scripts/bootstrap.ps1 -Task deps -ForceDeps
```

---

## Backend Development

```bash
cd backend
go mod download
go run ./cmd/hostswitch
```

---

## Frontend Development

```bash
cd frontend
npm install
npm run dev
```

---

# 🔒 Safe Development Mode

> HostSwitch does NOT modify your real hosts file in development mode.

By default the backend uses:

```text
./data/hosts.preview
```

instead of the real system hosts file.

This allows:

- safe UI development
- testing apply operations
- backup validation
- preview rendering

without modifying system networking configuration.

---

## Refresh Hosts Preview

To refresh the preview from your current system hosts file:

```powershell
./scripts/bootstrap.ps1 -Task dev -RefreshHostsPreview
```

---

# 🧠 Smart Import System

On first startup, HostSwitch:

1. Reads the configured hosts file
2. Creates an `initial-import` backup
3. Imports valid non-system entries
4. Categorizes entries automatically

Examples:

- Local Development
- Docker & Kubernetes
- Monitoring
- Developer Tools
- Blocked Domains
- External Overrides

Imported entries retain tags and source tracking.

Later apply operations move imported domains into the HostSwitch managed block instead of duplicating them.

---

# 🌍 Environment Import Logic

The current hosts file is treated as the current machine state — not automatically as `PROD`.

Use:

```text
Import Current
```

to import into the active environment.

Or:

```text
Import All Envs
```

to distribute categorized entries across:

- DEV
- STAGING
- PROD

---

# 🐳 Docker Discovery

HostSwitch can automatically detect Docker containers.

Example:

```text
grafana     → grafana.local
prometheus  → prometheus.local
ollama      → ollama.local
n8n         → n8n.local
```

Future support includes:

- Docker labels
- auto import rules
- automatic grouping
- Kubernetes discovery

---

# ⚙ Configuration

| Variable | Default | Description |
|---|---|---|
| `HOSTSWITCH_ADDR` | `:8080` | Backend listen address |
| `HOSTSWITCH_DB_PATH` | `./data/hostswitch.db` | SQLite database path |
| `HOSTSWITCH_DATA_DIR` | `./data` | Data directory for backups and preview hosts |
| `HOSTSWITCH_HOSTS_PATH` | `./data/hosts.preview` | Hosts file target |
| `HOSTSWITCH_ALLOWED_ORIGIN` | `http://localhost:5173` | CORS origin for frontend |

---

# 📚 API Snapshot

## Health

```http
GET /api/health
```

---

## Environments

```http
GET  /api/environments
POST /api/environments
POST /api/environments/switch
```

---

## Groups

```http
GET   /api/groups?environment_id=...
POST  /api/groups
PATCH /api/groups/{id}
```

---

## Hosts

```http
GET    /api/hosts?environment_id=...
POST   /api/hosts
POST   /api/hosts/import-system
PATCH  /api/hosts/{id}
DELETE /api/hosts/{id}
```

---

## Apply Hosts

```http
POST /api/apply
```

---

## Backups

```http
GET  /api/backups
POST /api/backups
POST /api/backups/restore
```

---

## Docker Discovery

```http
GET /api/docker/discover
```

---

# 🏗 Architecture

```text
HostSwitch
│
├── Backend (Go)
│   ├── REST API
│   ├── Hosts Manager
│   ├── Backup Engine
│   ├── Docker Discovery
│   └── DNS Engine
│
├── Frontend (React + Tailwind)
│
├── Database (SQLite)
│
└── Deployment (Docker)
```

---

# 🛣 Roadmap

## Phase 1 — Core Platform

- hosts manager
- environment switching
- groups
- backups
- dashboard UI
- Docker deployment

---

## Phase 2 — Smart Discovery

- Docker auto discovery
- Docker label support
- service imports

---

## Phase 3 — Automation

- API tokens
- webhooks
- automation support

---

## Phase 4 — Local DNS Mode

- wildcard domains
- DNS forwarding
- local DNS server

---

## Phase 5 — Enterprise Features

- Git sync
- RBAC
- SSO
- audit logs
- Kubernetes discovery

---

# 🤝 Contributing

Contributions, ideas, and feature requests are welcome.

Please open:

- Issues
- Discussions
- Pull Requests

---

# ⭐ Vision

HostSwitch aims to become:

> The modern local domain control center for developers.

A unified platform for:

- hosts management
- Docker service routing
- local DNS
- environment switching
- developer productivity

---

# 📄 License

MIT License

---

<div align="center">

### HostSwitch

Modern local domain control center for developers and DevOps.

</div>
