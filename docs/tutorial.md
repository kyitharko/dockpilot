# myplatform — Tutorial

This tutorial walks you through the tool from a fresh install to running a
multi-service stack with dependency ordering. Each section builds on the
previous one.

---

## Prerequisites

- **Go 1.22+** — run `go version` to check
- **Docker Engine 24+** — the daemon must be running; confirm with `docker info`
- The binary built from source (see below)

No other tools are required. `myplatform` talks directly to the Docker daemon
over its Unix socket; it never shells out to the `docker` CLI binary.

---

## 1 — Build and install

Clone the repo and build:

```bash
git clone <repo-url>
cd myplatform
go build -o myplatform .
```

Move the binary somewhere on your PATH, or run it in place with `./myplatform`.

Verify the daemon is reachable:

```bash
./myplatform list
# If Docker is not running you will see:
#   ✖  docker daemon is not reachable: ...
```

---

## 2 — Deploying a single service

### Built-in services

`myplatform` has built-in definitions for four common services. No image
flag or port mapping required — the defaults are applied automatically.

```bash
./myplatform deploy mongodb
```

Expected output:

```
→  Pulling image "mongo:latest"...
...layer progress...
✔  Service "mongodb" deployed -> container "myplatform-mongodb"
```

List everything that is running:

```bash
./myplatform list
```

```
NAME                    IMAGE           STATE     PORTS
----                    -----           -----     -----
myplatform-mongodb      mongo:latest    running   0.0.0.0:27017->27017/tcp
```

Check a specific service:

```bash
./myplatform status mongodb
```

```
Service:    mongodb
Container:  myplatform-mongodb
Image:      mongo:latest
State:      running
Ports:      0.0.0.0:27017->27017/tcp

Service is running
```

### Multiple instances of the same service

Use `--name` to give each instance a unique suffix:

```bash
./myplatform deploy mongodb --name dev
./myplatform deploy mongodb --name staging
```

This creates `myplatform-mongodb-dev` and `myplatform-mongodb-staging`.
Both containers get their own named volumes automatically.
If a port is already in use the next free port is chosen.

```bash
./myplatform list
```

```
NAME                        IMAGE           STATE     PORTS
----                        -----           -----     -----
myplatform-mongodb          mongo:latest    running   0.0.0.0:27017->27017/tcp
myplatform-mongodb-dev      mongo:latest    running   0.0.0.0:27018->27017/tcp
myplatform-mongodb-staging  mongo:latest    running   0.0.0.0:27019->27017/tcp
```

### Custom images

Any image from Docker Hub or a private registry works with `--image`:

```bash
./myplatform deploy webserver --image nginx:alpine --port 8080:80
./myplatform deploy myapp --image registry.example.com/myapp:v1.2.3 \
    --port 443:443 \
    --env APP_ENV=production \
    --env LOG_LEVEL=warn
```

Container names follow the pattern `myplatform-<name>`, so these create
`myplatform-webserver` and `myplatform-myapp`.

---

## 3 — Removing services

```bash
./myplatform remove mongodb
```

```
→  Stopping container "myplatform-mongodb"...
→  Removing container "myplatform-mongodb"...
✔  Container "myplatform-mongodb" removed
```

To also delete the associated named volume (all data is lost):

```bash
./myplatform remove mongodb --volumes
```

Removing a container that does not exist is safe — you get a warning and
exit 0, which makes it safe to call from scripts:

```bash
./myplatform remove mongodb
# ✔  warning: container "myplatform-mongodb" does not exist -- nothing to remove
```

---

## 4 — Stack deployment

A stack is a YAML file that describes multiple services as a group.
All the `stack` subcommands take a file path as their only required argument.

### A simple stack (no dependencies)

`examples/backend-dev.yaml`:

```yaml
name: backend-dev

services:
  mongodb:
    image: mongo:7
    container_name: backend-mongodb
    ports:
      - "27017:27017"
    volumes:
      - "backend-mongodb-data:/data/db"

  redis:
    image: redis:7
    container_name: backend-redis
    ports:
      - "6379:6379"

  nginx:
    image: nginx:latest
    container_name: backend-nginx
    ports:
      - "8080:80"
```

Always validate a stack file before deploying:

```bash
./myplatform stack validate examples/backend-dev.yaml
```

```
✔  Stack "backend-dev" is valid (3 service(s))
  mongodb               image=mongo:7          container=backend-mongodb
  redis                 image=redis:7          container=backend-redis
  nginx                 image=nginx:latest     container=backend-nginx

→  Deployment order:
  1. mongodb
  2. redis
  3. nginx
```

Deploy all services in one command:

```bash
./myplatform stack deploy examples/backend-dev.yaml
```

```
→  Deploying stack "backend-dev" (3 service(s))...
→  Deployment order: mongodb → redis → nginx

→  [mongodb] Creating volume "backend-mongodb-data"...
→  [mongodb] Pulling image "mongo:7"...
✔  [mongodb] Deployed -> "backend-mongodb"
→  [redis] Pulling image "redis:7"...
✔  [redis] Deployed -> "backend-redis"
→  [nginx] Pulling image "nginx:latest"...
✔  [nginx] Deployed -> "backend-nginx"
✔  Stack "backend-dev" deployed
```

Running deploy again is safe — existing containers are skipped:

```bash
./myplatform stack deploy examples/backend-dev.yaml
# !  [mongodb] Container "backend-mongodb" already exists — skipping
# ...
```

Check the status of all services at once:

```bash
./myplatform stack status examples/backend-dev.yaml
```

```
Stack: backend-dev

SERVICE    CONTAINER        STATE     PORTS
-------    ---------        -----     -----
mongodb    backend-mongodb  running   0.0.0.0:27017->27017/tcp
redis      backend-redis    running   0.0.0.0:6379->6379/tcp
nginx      backend-nginx    running   0.0.0.0:8080->80/tcp
```

Remove everything:

```bash
./myplatform stack remove examples/backend-dev.yaml          # keep volumes
./myplatform stack remove examples/backend-dev.yaml --volumes # also delete data
```

---

## 5 — Dependency ordering with `depends_on`

When some services must start before others, declare the ordering in the
stack file using `depends_on`. `myplatform` computes a topological sort and
deploys services in dependency-first order; removal runs in the reverse order
so dependents are torn down before their dependencies.

`examples/with-deps.yaml`:

```yaml
name: backend-dev

services:
  mongodb:
    image: mongo:7
    container_name: backend-mongodb
    ports:
      - "27017:27017"
    volumes:
      - "backend-mongodb-data:/data/db"

  redis:
    image: redis:7
    container_name: backend-redis
    ports:
      - "6379:6379"

  api:
    image: node:20
    container_name: backend-api
    depends_on:
      - mongodb
      - redis
    environment:
      MONGO_URI: mongodb://mongodb:27017/appdb
      REDIS_HOST: redis
    command:
      - "node"
      - "server.js"
    ports:
      - "3000:3000"
```

Note the two ways to pass environment variables:

- `env` — a list of `KEY=VALUE` strings (same as the single-container `deploy` command)
- `environment` — a YAML map; merged with `env` at deploy time

And `command` overrides the image's default `CMD` instruction.

Validate first to see the computed order:

```bash
./myplatform stack validate examples/with-deps.yaml
```

```
✔  Stack "backend-dev" is valid (3 service(s))
  mongodb               image=mongo:7    container=backend-mongodb
  redis                 image=redis:7    container=backend-redis
  api                   image=node:20    container=backend-api

→  Deployment order:
  1. mongodb
  2. redis
  3. api  (depends on: mongodb, redis)
```

Deploy:

```bash
./myplatform stack deploy examples/with-deps.yaml
```

MongoDB and Redis are started first; the API container is started last,
after both dependencies are running.

> **Note:** `depends_on` controls start *order*, not readiness. The API
> container will start as soon as MongoDB and Redis containers exist — it
> will not wait for them to finish initialising. Health-check waiting is not
> implemented yet; add a startup retry loop inside your application if needed.

---

## 6 — Dependency error messages

The validator catches all three common dependency mistakes before any Docker
call is made.

### Self-dependency

```yaml
services:
  broken:
    image: alpine:latest
    depends_on:
      - broken     # a service cannot depend on itself
```

```
✖  stack validation failed:
  service "broken": depends_on references itself
```

### Unknown service key

```yaml
services:
  api:
    image: alpine:latest
    depends_on:
      - cache      # no service named "cache" exists
```

```
✖  stack validation failed:
  service "api": depends_on references unknown service "cache"
```

### Circular dependency

See `examples/circular.yaml` for a three-node cycle (A→B→C→A):

```bash
./myplatform stack validate examples/circular.yaml
```

```
✖  circular dependency detected involving: serviceA, serviceB, serviceC
```

The standalone service in that file (`standalone`) is reported correctly as
not involved in the cycle.

---

## 7 — Naming conventions

### Single-container commands

All containers created by `deploy` are named `myplatform-<name>[-suffix]`.

| Command | Container name |
|---------|---------------|
| `deploy mongodb` | `myplatform-mongodb` |
| `deploy mongodb --name dev` | `myplatform-mongodb-dev` |
| `deploy myapp --image nginx:alpine` | `myplatform-myapp` |

The `status` and `remove` commands accept either the service name
(`mongodb`) or the full container name (`myplatform-mongodb`).

### Stack commands

Stack container names come from the `container_name` field in the YAML,
used exactly as written — there is no `myplatform-` prefix applied.
When `container_name` is omitted it defaults to `<stack-name>-<service-key>`.

---

## 8 — Extending the tool

### Adding a new built-in service

Create a file in `internal/services/` that calls `services.Register()` in
its `init()`:

```go
package services

func init() {
    Register(ServiceConfig{
        Name:          "rabbitmq",
        Image:         "rabbitmq:3-management",
        ContainerName: "myplatform-rabbitmq",
        Ports:         []string{"5672:5672", "15672:15672"},
        Volumes:       []string{"myplatform-rabbitmq-data:/var/lib/rabbitmq"},
    })
}
```

No other files need to change. The new service appears in `deploy`,
`status`, and `remove` automatically.

### Adding a new container runtime

Implement `RuntimeClient` in a new file under `internal/runtime/` and swap
it in where `runtime.NewDockerClient()` is called. Nothing in `cmd/` or
`internal/services/` needs to change.

```go
// internal/runtime/podman.go
type PodmanClient struct { ... }
func (p *PodmanClient) PullImage(...) error { ... }
// ... implement all RuntimeClient methods
```

---

## Quick-reference card

```
# Single containers
myplatform deploy <service>                         # built-in with defaults
myplatform deploy <name> --image <image>            # custom image
myplatform deploy <service> --name <suffix>         # named instance
myplatform deploy <name> --port <h:c> --env K=V     # custom port + env
myplatform list
myplatform status <service|container> [--name <s>]
myplatform remove <service|container> [--name <s>] [--volumes]

# Stacks
myplatform stack validate <stack.yaml>
myplatform stack deploy   <stack.yaml>
myplatform stack status   <stack.yaml>
myplatform stack remove   <stack.yaml> [--volumes]
```
