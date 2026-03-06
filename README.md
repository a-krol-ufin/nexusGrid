# NexusGrid

A Go microservices platform running on k3s with Keycloak authentication, RabbitMQ messaging, and a Next.js frontend.

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌──────────────┐
│  Frontend   │────▶│ API Gateway │────▶│ Auth Service │
│  (Next.js)  │     │    (Go)     │     │    (Go)      │
└─────────────┘     └─────────────┘     └──────┬───────┘
                                                │
                                         ┌──────▼───────┐
                                         │   Keycloak   │
                                         │  (OIDC/OAuth2)│
                                         └──────────────┘
                    ┌─────────────┐
                    │  RabbitMQ   │
                    │  (Messaging)│
                    └─────────────┘
```

| Service      | Language  | Port  | Description                          |
|-------------|-----------|-------|--------------------------------------|
| frontend    | Next.js   | 3000  | Web UI — login, dashboard            |
| api-gateway | Go        | 8080  | Reverse proxy, JWT validation        |
| auth-service| Go        | 8080  | Keycloak OIDC integration            |
| keycloak    | Java      | 8080  | Identity provider                    |
| rabbitmq    | Erlang    | 5672  | Message broker (management: 15672)   |

## Prerequisites

- [k3s](https://k3s.io/) or any Kubernetes cluster
- [Skaffold](https://skaffold.dev/) v2+
- [Docker](https://www.docker.com/)
- Go 1.22+
- Node.js 20+

## Quick Start

```bash
# 1. Deploy infrastructure (Keycloak + RabbitMQ)
kubectl apply -f infra/

# 2. Wait for Keycloak to be ready (~60s)
kubectl wait --for=condition=ready pod -l app=keycloak --timeout=120s

# 3. Start all services with hot-reload
skaffold dev
```

## Auth Flow

```
User → /auth/login → Keycloak login page
     ← redirect to /callback
     → auth-service exchanges code for tokens
     → sets access_token cookie
     → redirect to /dashboard
```

## API Gateway Routes

| Path       | Auth Required | Description                          |
|------------|--------------|--------------------------------------|
| `/auth/*`  | No           | Proxied to auth-service              |
| `/api/*`   | Yes (JWT)    | Protected API routes                 |
| `/api/me`  | Yes (JWT)    | Returns current user claims          |
| `/health`  | No           | Gateway health check                 |

Protected routes receive `X-User-ID` and `X-User-Email` headers forwarded from the validated token.

## Configuration

### Auth Service

| Env Var                  | Default                      | Description              |
|--------------------------|------------------------------|--------------------------|
| `KEYCLOAK_URL`           | `http://keycloak:8080`       | Keycloak base URL        |
| `KEYCLOAK_REALM`         | `nexusgrid`                  | Realm name               |
| `KEYCLOAK_CLIENT_ID`     | `nexusgrid-client`           | OAuth2 client ID         |
| `KEYCLOAK_CLIENT_SECRET` | *(from secret)*              | OAuth2 client secret     |
| `REDIRECT_URL`           | `http://localhost:8081/callback` | OAuth2 callback URL  |
| `FRONTEND_URL`           | `http://localhost:3000`      | Post-login redirect      |

### API Gateway

| Env Var            | Default                  | Description              |
|--------------------|--------------------------|--------------------------|
| `AUTH_SERVICE_URL` | `http://auth-service`    | Auth service URL         |
| `ALLOWED_ORIGIN`   | `http://localhost:3000`  | CORS allowed origin      |

### Frontend

| Env Var            | Default               | Description              |
|--------------------|-----------------------|--------------------------|
| `API_GATEWAY_URL`  | `http://api-gateway`  | API gateway URL          |
| `AUTH_SERVICE_URL` | `http://auth-service` | Auth service URL         |

## Default Credentials

> **Change these before any non-local deployment.**

| Service        | Username    | Password      |
|----------------|-------------|---------------|
| Keycloak admin | `admin`     | `admin`       |
| Keycloak user  | `admin`     | `admin123`    |
| RabbitMQ       | `nexusgrid` | `nexusgrid123`|

The Keycloak client secret is set in `infra/keycloak.yaml` → `keycloak-client-secret`.

## Project Structure

```
nexusGrid/
├── skaffold.yaml
├── infra/
│   ├── keycloak.yaml       # Keycloak deployment + realm config
│   └── rabbitmq.yaml       # RabbitMQ deployment
├── auth-service/
│   ├── main.go
│   ├── go.mod
│   ├── Dockerfile
│   └── k8s.yaml
├── api-gateway/
│   ├── main.go
│   ├── go.mod
│   ├── Dockerfile
│   └── k8s.yaml
└── frontend/
    ├── src/app/
    │   ├── page.tsx         # Login page
    │   └── dashboard/       # Protected dashboard
    ├── next.config.ts
    ├── Dockerfile
    └── k8s.yaml
```

## Adding a New Microservice

1. Create a directory `my-service/` with `main.go`, `Dockerfile`, `k8s.yaml`
2. Add to `skaffold.yaml` under `build.artifacts` and `manifests.rawYaml`
3. Add a route in `api-gateway/main.go` pointing to the new service
