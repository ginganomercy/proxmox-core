# proxmox-core

Go backend API for Proxmox Custom Dashboard.

## Tech Stack
- **Language:** Go (Golang)
- **Framework:** Fiber v2
- **Database:** SQLite (via go-sqlite3, CGO)
- **Auth:** JWT

## Running Locally
```bash
cp .env.example .env
# Fill in your Proxmox credentials
go run main.go
```

## CI/CD
Automated pipeline via GitHub Actions:
1. 🧪 **Go Vet & Lint** (golangci-lint)
2. 🐳 **Docker Build & Push** → `ghcr.io/ginganomercy/proxmox-core`
3. 🔒 **Trivy Security Scan** (blocks CRITICAL CVEs)
4. 🚀 **Auto-Deploy** to Docker Swarm via Tailscale SSH
