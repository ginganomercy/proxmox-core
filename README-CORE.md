# 🧠 Proxmox Core API

This is the central Backend Microservice for the **Proxmox Custom Dashboard**. It handles authentication, secure data storage, caching, and acts as the secure bridge to your underlying Proxmox VE Nodes.

## 🚀 Tech Stack

- **Go (v1.26)**: Core language, offering extreme performance and memory safety.
- **Fiber v2**: Web framework inspired by Express.js, built on top of `fasthttp` for maximum throughput.
- **SQLite (via go-sqlite3)**: Zero-configuration local database for storing user credentials and lightweight metadata.
- **GORM**: The most popular Object-Relational Mapper (ORM) for Go, ensuring safe and developer-friendly database queries.
- **JWT v5**: Stateless JSON Web Token authentication.
- **Go-Cache**: In-memory caching mechanism to prevent spamming the Proxmox API with redundant requests, resulting in instant dashboard loads.

---

## 📂 Folder Structure

```text
core-api/
├── .github/workflows/   # CI/CD Deployment pipelines (Trivy & Tailscale)
├── config/              # Environment variable parsers and configurations
├── controllers/         # HTTP Route Handlers (Auth, Nodes, VMs, Metrics)
├── database/            # SQLite connection setup and GORM initialization
├── middleware/          # Fiber Middlewares (JWT Guard for protected routes)
├── models/              # GORM Database Schemas (User table, etc.)
├── proxmox/             # Native HTTP Client & Cacher for Proxmox VE API
├── repositories/        # Database Access Layer (Domain logic isolation)
├── routes/              # API Route Registrations (/api/v1/...)
├── services/            # Core Business Logic (Auth validations)
├── main.go              # Entrypoint of the application
├── Dockerfile           # Multi-stage production Docker build
└── go.mod               # Go module dependencies
```

---

## 🛠️ Local Development Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/ginganomercy/proxmox-core.git
   cd proxmox-core
   ```

2. **Prepare Environment Variables:**
   Copy the example file and fill in your real Proxmox VE credentials.
   ```bash
   cp .env.example .env
   ```

3. **Install Dependencies & Run:**
   ```bash
   go mod download
   go run main.go
   ```
   *The server will start on `http://localhost:3001`.*

---

## 🔒 CI/CD & Deployment

This service utilizes an **Enterprise-Grade GitHub Actions Pipeline**:
1. **Linting & Code Quality**: Validates code using `golangci-lint` and `go vet`.
2. **Docker Build**: Packages the binary securely and pushes it to GitHub Container Registry (`ghcr.io`).
3. **DevSecOps**: Scans the Docker image using **Trivy** to block critical CVEs from being deployed.
4. **Zero-Trust Deployment**: Connects to your private Swarm Manager via **Tailscale** and automatically updates the service using SSH without exposing ports to the public internet.
