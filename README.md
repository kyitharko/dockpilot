# myplatform

A CLI tool for deploying and managing Docker containers with sensible production defaults.
Supports built-in service shortcuts as well as any image from Docker Hub or a private registry.

Uses the **Docker Engine SDK for Go** (`github.com/docker/docker/client`) — no shell-out to the
`docker` CLI binary required at runtime.

## Requirements

- Go 1.22+
- Docker Engine 24+ (daemon must be running; API version negotiated automatically)

## Build

```bash
go build -o myplatform .
```

The binary is self-contained. No external runtime tools are needed.

## Test

```bash
# Unit tests (none yet — add under internal/runtime/ and cmd/)
go test ./...

# Vet — catches suspicious constructs
go vet ./...

# Build check across all packages
go build ./...
```

## Manual test plan

Start Docker, build the binary, then run through each scenario in order.

### 1. Daemon guard

```bash
# Stop Docker, then run any command — expect a clear error, not a panic
sudo systemctl stop docker
./myplatform list
# expected: "docker daemon is not reachable" error

sudo systemctl start docker
```

### 2. Deploy a built-in service

```bash
./myplatform deploy mongodb
# expected: pull progress printed layer by layer, then success message
# container name: myplatform-mongodb

./myplatform list
# expected: myplatform-mongodb shown as running with port 27017
```

### 3. Status

```bash
./myplatform status mongodb
# expected: Service/Container/Image/State/Ports printed, "Service is running"

./myplatform status myplatform-mongodb
# same result via full container name
```

### 4. Multiple instances

```bash
./myplatform deploy mongodb --name dev
# expected: myplatform-mongodb-dev, port auto-incremented if 27017 is taken

./myplatform list
# expected: both myplatform-mongodb and myplatform-mongodb-dev shown

./myplatform status mongodb --name dev
```

### 5. Custom image

```bash
./myplatform deploy webserver --image nginx:alpine --port 8080:80
./myplatform status myplatform-webserver
# expected: image=nginx:alpine, port 0.0.0.0:8080->80/tcp

# With env vars
./myplatform deploy myapp --image alpine --env HELLO=world
```

### 6. Remove

```bash
./myplatform remove mongodb
# expected: stop + remove messages, success

./myplatform remove mongodb --name dev --volumes
# expected: container removed, then volume removed

./myplatform list
# expected: removed containers no longer shown
```

### 7. Remove non-existent container

```bash
./myplatform remove mongodb
# expected: warning "container does not exist -- nothing to remove", exit 0
```

---

## Commands

| Command | Description |
|---------|-------------|
| `deploy` | Deploy a service container |
| `list` | List all managed containers |
| `status` | Show the runtime status of a container |
| `remove` | Stop and remove a container |
| `stack deploy` | Deploy all services from a YAML stack file |
| `stack remove` | Stop and remove all services in a stack file |
| `stack status` | Show runtime status of all stack services |
| `stack validate` | Validate a stack file without deploying |

---

## Deploy

```
myplatform deploy <service|name> [flags]
```

### Built-in services

The following services can be deployed without specifying an image:

| Service | Image | Default port | Volumes |
|---------|-------|-------------|---------|
| `mongodb` | `mongo:latest` | 27017 | `myplatform-mongodb-data:/data/db` |
| `postgres` | `postgres:latest` | 5432 | `myplatform-postgres-data:/var/lib/postgresql/data` |
| `redis` | `redis:latest` | 6379 | — |
| `nginx` | `nginx:latest` | 8080→80 | — |

```bash
myplatform deploy mongodb
myplatform deploy postgres
myplatform deploy redis
myplatform deploy nginx
```

### Custom images

Any image from Docker Hub or a private registry can be deployed using `--image`:

```bash
myplatform deploy myapp --image nginx:alpine
myplatform deploy myapp --image nginx:alpine --port 8080:80
myplatform deploy myapp --image registry.example.com/myapp:latest --port 443:443 --env DEBUG=1
```

### Flags

| Flag | Description |
|------|-------------|
| `--image <ref>` | Docker image to deploy (required for custom services) |
| `--port <host:container>` | Port mapping, repeatable |
| `--env <KEY=VALUE>` | Environment variable, repeatable |
| `--name <suffix>` | Instance name suffix — creates `myplatform-<name>-<suffix>` |

### Multiple instances

Use `--name` to run more than one instance of the same service:

```bash
myplatform deploy mongodb --name dev
myplatform deploy mongodb --name staging
# creates: myplatform-mongodb-dev, myplatform-mongodb-staging
```

When `--name` is omitted, the tool auto-increments: `myplatform-mongodb`, `myplatform-mongodb-2`, etc.

Host ports are also auto-incremented if the preferred port is already in use.

---

## Status

```
myplatform status <service|container> [--name <suffix>]
```

```bash
myplatform status mongodb
myplatform status mongodb --name staging
myplatform status myplatform-mongodb-2
myplatform status myplatform-myapp
```

---

## Remove

```
myplatform remove <service|container> [--name <suffix>] [--volumes]
```

```bash
myplatform remove mongodb
myplatform remove mongodb --name staging
myplatform remove myplatform-mongodb-2
myplatform remove myplatform-myapp --volumes
```

`--volumes` also deletes the named volume(s) associated with the instance. **Data will be lost.**

---

## Container naming

All containers are named `myplatform-<name>[-suffix]`.

| Deploy command | Container name |
|----------------|----------------|
| `deploy mongodb` | `myplatform-mongodb` |
| `deploy mongodb --name dev` | `myplatform-mongodb-dev` |
| `deploy myapp --image nginx:alpine` | `myplatform-myapp` |
| `deploy myapp --image nginx:alpine --name prod` | `myplatform-myapp-prod` |

Use the full container name with `status` and `remove` when targeting a specific instance.

---

## Stack deployment

A stack file describes multiple services in a single YAML document.

```
myplatform stack <subcommand> <stack.yaml>
```

### Stack file format

```yaml
name: backend-dev          # required — used to derive container names when container_name is omitted

services:
  mongodb:                 # service key — used in CLI output
    image: mongo:7         # required
    container_name: backend-mongodb   # optional; defaults to <name>-<key>
    ports:
      - "27017:27017"
    volumes:
      - "backend-mongodb-data:/data/db"
    env:
      - "MONGO_INITDB_DATABASE=appdb"

  redis:
    image: redis:7
    container_name: backend-redis
    ports:
      - "6379:6379"

  api:
    image: node:20
    container_name: backend-api
    depends_on:            # deploy after mongodb and redis
      - mongodb
      - redis
    environment:           # map form; merged with env list
      MONGO_URI: mongodb://mongodb:27017/appdb
      REDIS_HOST: redis
    command:               # overrides the image's default CMD
      - "node"
      - "server.js"
    ports:
      - "3000:3000"
```

### Service fields

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Docker image reference — **required** |
| `container_name` | string | Name for the container; defaults to `<stack-name>-<key>` |
| `ports` | list | Port mappings (`"host:container"`) |
| `volumes` | list | Named volume mounts (`"volume:path"`) |
| `env` | list | Environment variables in `KEY=VALUE` form |
| `environment` | map | Environment variables as a key/value map (merged with `env`) |
| `command` | list | Override the image's default command |
| `depends_on` | list | Service keys that must be deployed before this one |

Services are deployed in dependency order (topological sort). Among services with no ordering constraint, YAML document order is preserved.

### Stack commands

#### validate

Check the file for errors without touching Docker.
Also resolves and prints the deployment order so you can verify it before deploying.

```bash
myplatform stack validate examples/with-deps.yaml
# expected:
# ✔  Stack "backend-dev" is valid (3 service(s))
#   mongodb               image=mongo:7    container=backend-mongodb
#   redis                 image=redis:7    container=backend-redis
#   api                   image=node:20    container=backend-api
#
# →  Deployment order:
#   1. mongodb
#   2. redis
#   3. api  (depends on: mongodb, redis)
```

#### deploy

Pull images, create volumes, and start all containers.
Services whose container already exists are skipped.

```bash
myplatform stack deploy examples/with-deps.yaml
# expected:
# →  Deploying stack "backend-dev" (3 service(s))...
# →  Deployment order: mongodb → redis → api
#
# →  [mongodb] Creating volume "backend-mongodb-data"...
# →  [mongodb] Pulling image "mongo:7"...
# ✔  [mongodb] Deployed -> "backend-mongodb"
# →  [redis] Pulling image "redis:7"...
# ✔  [redis] Deployed -> "backend-redis"
# →  [api] Pulling image "node:20"...
# ✔  [api] Deployed -> "backend-api"
# ✔  Stack "backend-dev" deployed
```

#### status

```bash
myplatform stack status examples/backend-dev.yaml
# expected:
# Stack: backend-dev
#
# SERVICE              CONTAINER             STATE         PORTS
# -------              ---------             -----         -----
# mongodb              backend-mongodb       running       0.0.0.0:27017->27017/tcp
# redis                backend-redis         running       0.0.0.0:6379->6379/tcp
# nginx                backend-nginx         running       0.0.0.0:8080->80/tcp
```

#### remove

Stop and remove all containers. Pass `--volumes` to also delete named volumes.

```bash
myplatform stack remove examples/backend-dev.yaml
myplatform stack remove examples/backend-dev.yaml --volumes   # also deletes volumes
```

### Manual test plan — stacks

```bash
# 1. Validate the example file
./myplatform stack validate examples/backend-dev.yaml
# expected: "Stack "backend-dev" is valid (3 service(s))" + service table

# 2. Validate a broken file
cat > /tmp/bad-stack.yaml <<'EOF'
services:
  svc1:
    ports:
      - "8080:80"
EOF
./myplatform stack validate /tmp/bad-stack.yaml
# expected: validation failed with "missing required field 'name'" and "missing required field 'image'"

# 3. Deploy the example stack
./myplatform stack deploy examples/backend-dev.yaml
# expected: pull progress for each image, then success for each service

# 4. Status shows all running
./myplatform stack status examples/backend-dev.yaml
# expected: all three services show "running"

# 5. Re-deploy is idempotent
./myplatform stack deploy examples/backend-dev.yaml
# expected: all three services print "already exists — skipping"

# 6. Remove without volumes
./myplatform stack remove examples/backend-dev.yaml
# expected: stop + remove messages for each service; volumes preserved

# 7. Re-remove is safe
./myplatform stack remove examples/backend-dev.yaml
# expected: "does not exist — skipping" for each service, exit 0

# 8. Deploy again, then remove with volumes
./myplatform stack deploy examples/backend-dev.yaml
./myplatform stack remove examples/backend-dev.yaml --volumes
# expected: containers removed, then backend-mongodb-data volume removed
```

---

## Dependency ordering

Services are deployed in topological order derived from their `depends_on` lists.
Remove runs in the reverse of that order so dependents are torn down before their dependencies.

### How it works

`stack deploy` and `stack validate` both call `ResolveOrder` (Kahn's algorithm in `internal/stack/graph.go`).
Among services with no ordering constraint, YAML document order is preserved.

### Error messages

| Situation | Error |
|-----------|-------|
| Service depends on itself | `service "api": depends_on references itself` |
| Unknown dependency key | `service "api": depends_on references unknown service "cache"` |
| Circular dependency | `circular dependency detected involving: serviceA, serviceB, serviceC` |

### Example: circular dependency

```bash
./myplatform stack validate examples/circular.yaml
# expected:
# Error: circular dependency detected involving: serviceA, serviceB, serviceC
```

The `examples/circular.yaml` file contains a three-service cycle (A→B→C→A) plus a standalone service to illustrate that only the cycle members are reported.

### Manual test plan — dependency ordering

```bash
# 1. Validate a stack with deps — check printed order
./myplatform stack validate examples/with-deps.yaml
# expected: "Deployment order: 1. mongodb  2. redis  3. api (depends on: mongodb, redis)"

# 2. Circular dependency is rejected
./myplatform stack validate examples/circular.yaml
# expected: error "circular dependency detected involving: serviceA, serviceB, serviceC"

# 3. Self-dependency is rejected
cat > /tmp/self-dep.yaml <<'EOF'
name: self-dep-test
services:
  broken:
    image: alpine:latest
    depends_on:
      - broken
EOF
./myplatform stack validate /tmp/self-dep.yaml
# expected: 'service "broken": depends_on references itself'

# 4. Missing dependency is rejected
cat > /tmp/missing-dep.yaml <<'EOF'
name: missing-dep-test
services:
  api:
    image: alpine:latest
    depends_on:
      - cache
EOF
./myplatform stack validate /tmp/missing-dep.yaml
# expected: 'service "api": depends_on references unknown service "cache"'

# 5. Deploy respects dependency order
./myplatform stack deploy examples/with-deps.yaml
# expected: mongodb deployed, then redis, then api

# 6. Remove reverses deploy order
./myplatform stack remove examples/with-deps.yaml
# expected: api removed first, then redis, then mongodb
```

### Design notes

- `container_name` in the YAML is used exactly as-is; there is no `myplatform-` prefix for stack containers.
- Services are deployed in dependency order (topological sort via Kahn's algorithm); YAML order is the tiebreaker.
- Health-check waiting and readiness probes are not implemented — `depends_on` controls start order only, not service readiness.

---

## Architecture

```
internal/
  runtime/
    runtime.go   — RuntimeClient interface + ContainerInfo (runtime-agnostic)
    docker.go    — DockerSDKClient: implements RuntimeClient via Docker Engine SDK
    check.go     — CheckDaemon(): pings the daemon before any command runs
  services/
    services.go  — ServiceConfig type + registry
    mongodb.go / postgres.go / redis.go / nginx.go — built-in service definitions
  stack/
    config.go    — Stack, NamedService, ServiceDef types (includes DependsOn, Environment, Command)
    parser.go    — Parse(): reads YAML, preserves service order via yaml.Node
    validator.go — Validate(): structural checks (missing fields, self-dep, unknown dep)
    graph.go     — ResolveOrder(): Kahn's topological sort + cycle detection
    graph_test.go — unit tests for the graph resolver
    deployer.go  — Deploy / Remove / Status: operate on a Stack via RuntimeClient
  utils/
    output.go    — coloured terminal output helpers
cmd/
  root.go        — cobra root, PersistentPreRunE calls CheckDaemon
  deploy.go      — deploy command
  list.go        — list command
  status.go      — status command
  remove.go      — remove command
  instance.go    — shared arg-parsing helper (no Docker calls)
  stack.go       — stack subcommand (deploy / remove / status / validate)
examples/
  backend-dev.yaml  — three-service stack without dependencies
  with-deps.yaml    — three-service stack with depends_on (mongodb, redis → api)
  circular.yaml     — intentionally invalid stack showing circular-dep error
```

To add a containerd or podman backend, implement `RuntimeClient` in a new file under
`internal/runtime/` and swap it in where `runtime.NewDockerClient()` is called — no
changes to `cmd/` or `internal/services/` required.
