# dockpilot

Low-level Docker service executor. Deploys and manages individual containers via the Docker SDK. Exposes both a CLI and a REST API backed by the same engine layer.

```
User / stackpilot / AI
        ↓ REST API
    dockpilot
        ↓ Docker SDK
    Docker Engine
```

## Responsibility

dockpilot **only** manages individual containers. It has no knowledge of stacks, dependency graphs, or YAML orchestration files. That belongs to [stackpilot](../stackpilot).

## Project structure

```
dockpilot/
  main.go
  cmd/
    root.go       — Cobra root; Docker daemon pre-flight check
    deploy.go     — dockpilot deploy <service>
    remove.go     — dockpilot remove <service>
    status.go     — dockpilot status <service>
    list.go       — dockpilot list
    logs.go       — dockpilot logs <service>
    server.go     — dockpilot server --host 127.0.0.1 --port 8088
  internal/
    docker/
      client.go   — Docker SDK client (Client interface + SDKClient impl)
    engine/
      engine.go   — Core logic: Deploy, Remove, Status, List, Logs, Health
      service.go  — DeployRequest, DeployResult, ServiceStatus types
    api/
      server.go   — HTTP server with graceful 30s shutdown
      routes.go   — Route wiring + middleware chain
      handlers.go — HTTP handlers (parse → engine → write JSON)
      middleware.go — Request logger, panic recoverer
      response.go — writeJSON/writeError helpers + error code constants
    services/
      registry.go — Built-in service registry (Register/Get/Names/All)
      mongodb.go  — mongo:latest preset
      postgres.go — postgres:latest preset
      redis.go    — redis:latest preset
      nginx.go    — nginx:latest preset
    utils/
      output.go   — Coloured terminal output helpers
```

## Built-in services

| Name       | Image             | Default port | Volume                    |
|------------|-------------------|:------------:|---------------------------|
| `mongodb`  | `mongo:latest`    | 27017        | `dockpilot-mongodb-data`  |
| `postgres` | `postgres:latest` | 5432         | `dockpilot-postgres-data` |
| `redis`    | `redis:latest`    | 6379         | —                         |
| `nginx`    | `nginx:latest`    | 8080→80      | —                         |

## CLI usage

```bash
# Deploy a built-in service
dockpilot deploy mongodb
dockpilot deploy postgres

# Deploy any custom image
dockpilot deploy myapp --image nginx:alpine --port 8080:80 --env DEBUG=1

# List all managed containers
dockpilot list

# Runtime status
dockpilot status mongodb

# Tail logs (default 100 lines)
dockpilot logs mongodb --tail 50

# Remove (add --volumes to also delete data volumes)
dockpilot remove mongodb
dockpilot remove mongodb --volumes

# Start the REST API server (binds to 127.0.0.1 by default)
dockpilot server --port 8088
dockpilot server --host 0.0.0.0 --port 8088
```

## REST API

Start the server first:

```bash
dockpilot server --port 8088
```

### `GET /health`

```bash
curl http://127.0.0.1:8088/health
# → {"status":"ok"}
```

### `GET /v1/services`

```bash
curl http://127.0.0.1:8088/v1/services
# → [{"name":"mongodb","image":"mongo:latest","ports":["27017:27017"],...}, ...]
```

### `POST /v1/services/{service}/deploy`

Body is optional for built-in services. Pass `image` and overrides for custom images.

```bash
# Deploy a built-in service with defaults
curl -X POST http://127.0.0.1:8088/v1/services/mongodb/deploy
# → {"name":"mongodb","container":"dockpilot-mongodb","image":"mongo:latest","ports":["27017:27017"]}

# Deploy a custom image
curl -X POST http://127.0.0.1:8088/v1/services/myapp/deploy \
  -H 'Content-Type: application/json' \
  -d '{"image":"nginx:alpine","ports":["8080:80"],"env":["APP_ENV=prod"]}'

# Override a built-in service's port
curl -X POST http://127.0.0.1:8088/v1/services/postgres/deploy \
  -H 'Content-Type: application/json' \
  -d '{"ports":["5433:5432"]}'
```

### `GET /v1/services/{service}/status`

```bash
curl http://127.0.0.1:8088/v1/services/mongodb/status
# → {"name":"mongodb","container":"dockpilot-mongodb","image":"mongo:latest",
#    "state":"running","ports":"0.0.0.0:27017->27017/tcp","running":true}
```

### `GET /v1/services/{service}/logs`

```bash
curl 'http://127.0.0.1:8088/v1/services/mongodb/logs?tail=50'
# → {"container":"dockpilot-mongodb","logs":["line1","line2",...]}
```

### `DELETE /v1/services/{service}`

```bash
# Remove container only
curl -X DELETE http://127.0.0.1:8088/v1/services/mongodb

# Remove container + named volumes
curl -X DELETE 'http://127.0.0.1:8088/v1/services/mongodb?volumes=dockpilot-mongodb-data'
```

## Response format

**Success (2xx):** the resource object directly as JSON.

**Error (4xx/5xx):**
```json
{"error": "container \"dockpilot-mongodb\" not found", "code": "NOT_FOUND"}
```

Error codes: `BAD_REQUEST`, `NOT_FOUND`, `CONFLICT`, `INTERNAL_ERROR`, `SERVICE_UNAVAILABLE`

## Container naming

All containers managed by dockpilot are prefixed `dockpilot-`:

| Service arg  | Docker container name   |
|--------------|-------------------------|
| `mongodb`    | `dockpilot-mongodb`     |
| `myapp`      | `dockpilot-myapp`       |
| `backend-db` | `dockpilot-backend-db`  |

## Build

```bash
go build -o dockpilot .
./dockpilot --help
```
