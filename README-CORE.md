# CBT Core API — Backend Service

**Go + Fiber v2 REST API for the Cloud Baja Tegal Proxmox Dashboard**

---

## Overview

The Core API is the central backend service for the Cloud Baja Tegal (CBT) platform. It handles:

- **User authentication** (JWT, Google OAuth2, password reset via email)
- **Order management** (VM order submission, activation, provisioning lifecycle)
- **Proxmox VE orchestration** (node status, VM lifecycle, snapshots, metrics, VNC proxy)
- **Admin operations** (dashboard summary, order approvals, cluster log aggregation)

---

## Tech Stack

| Component | Technology |
| :--- | :--- |
| Language | Go 1.25 |
| HTTP Framework | Fiber v2 (`github.com/gofiber/fiber/v2`) |
| Database | SQLite via GORM (`gorm.io/driver/sqlite`) |
| In-Memory Cache | `patrickmn/go-cache` (5s TTL on Proxmox calls) |
| JWT | `golang-jwt/jwt/v5` |
| OAuth2 | `golang.org/x/oauth2` |
| Password | bcrypt via `golang.org/x/crypto` |

---

## API Endpoints

Base prefix: `/api`

### Public
| Method | Path | Description |
| :--- | :--- | :--- |
| POST | `/auth/register` | Create account |
| POST | `/auth/login` | Login (returns JWT cookie) |
| POST | `/auth/forgot-password` | Initiate password reset |
| POST | `/auth/reset-password` | Complete password reset |
| GET | `/auth/google` | Start Google SSO |
| GET | `/auth/google/callback` | Google SSO callback |

### Protected (JWT Required)
| Method | Path | Description |
| :--- | :--- | :--- |
| GET | `/auth/me` | Get current user |
| POST | `/orders/` | Submit VM order |
| GET | `/orders/me` | Get my orders |
| POST | `/orders/:id/activate` | Activate with code |
| DELETE | `/orders/:id` | Cancel order |
| GET | `/proxmox/nodes` | List cluster nodes |
| GET | `/proxmox/nodes/:node/status` | Node resource status |
| GET | `/proxmox/nodes/:node/instances` | List VMs on node |
| GET | `/proxmox/nodes/:node/:type/:vmid/ip` | VM IP address |
| POST | `/proxmox/nodes/:node/qemu/:vmid/power` | VM power action |
| POST | `/proxmox/nodes/:node/qemu/:vmid/config` | Update VM config |
| POST | `/proxmox/nodes/:node/:type/:vmid/vncproxy` | Get VNC ticket |
| DELETE | `/proxmox/nodes/:node/:type/:vmid` | Delete VM |
| GET/POST | `/proxmox/nodes/:node/:type/:vmid/snapshots` | Snapshot management |
| POST | `/proxmox/nodes/:node/:type/:vmid/rebuild` | Rebuild VM |
| GET | `/proxmox/nodes/:node/:type/:vmid/rrddata` | VM metrics |
| GET | `/proxmox/nodes/:node/rrddata` | Node metrics |

### Admin Only (JWT + ADMIN role)
| Method | Path | Description |
| :--- | :--- | :--- |
| GET | `/admin/summary` | Dashboard stats |
| GET | `/admin/orders/` | All customer orders |
| POST | `/admin/orders/:id/generate` | Generate activation code |
| GET | `/proxmox/cluster/logs` | Cluster syslog |
| GET | `/proxmox/cluster/tasks` | Task log (all nodes) |
| POST | `/proxmox/vms` | Provision VM |

---

## Environment Variables

```env
PORT=3001
JWT_SECRET=<min-32-char-secret>
PROXMOX_HOST=https://proxmox.pbjt.web.id:8006
PROXMOX_TOKEN_ID=root@pam!mytoken
PROXMOX_TOKEN_SECRET=<token-secret>
DB_PATH=/app/data/proxmox.db
CORS_ALLOWED_ORIGINS=https://cloud-dashboard.pbjt.web.id,http://localhost:5173
BASE_VMID=<template-vmid>

# Optional — Google SSO
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=

# Optional — SMTP (password reset)
SMTP_HOST=
SMTP_PORT=
SMTP_USER=
SMTP_PASS=
SMTP_FROM=
```

---

## Local Development

```bash
go mod download
cp .env.example .env    # Fill credentials
go run .
# Listening on :3001
```

## Production Build

```bash
CGO_ENABLED=1 GOOS=linux go build -o core-api .
```

## Docker

```bash
# Build
docker build -t ghcr.io/ginganomercy/proxmox-core:latest .

# Run
docker run --env-file .env -p 3001:3001 ghcr.io/ginganomercy/proxmox-core:latest
```

---

## CI/CD

On every push to `main`:

1. **Go Vet & Lint** — `go vet ./...`
2. **Build & Push** → `ghcr.io/ginganomercy/proxmox-core:latest`
3. **Trivy Security Scan** — blocks on CRITICAL CVEs
4. **Deploy to Swarm** — via Tailscale + SSH → `docker service update --force`

---

## Key Design Decisions

| Decision | Rationale |
| :--- | :--- |
| **SQLite over PostgreSQL** | Single-node deployment simplicity; NFS-backed for persistence |
| **In-memory cache (5s TTL)** | Reduces Proxmox API polling load while maintaining near-real-time freshness |
| **`/nodes/{node}/tasks` per node** | The Proxmox VE REST API has no `/cluster/tasks` endpoint; tasks are archived per-node daemon |
| **10-minute Fiber timeouts** | VM provisioning (clone → resize → cloud-init → power on) can take up to 5 minutes |
| **6-minute graceful shutdown** | Allows in-flight VM provisioning requests to complete before process exit |
| **Fiber `recover` middleware** | Prevents a single panicking goroutine from crashing the entire API process |
