package engine

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"dockpilot/internal/docker"
	"dockpilot/internal/services"
)

const containerPrefix = "dockpilot-"

// Engine is the core business layer. Both the CLI commands and the REST API
// handlers call Engine methods — there is no duplicated deployment logic.
type Engine struct {
	docker docker.Client
}

// New returns an Engine backed by the given docker client.
func New(dc docker.Client) *Engine {
	return &Engine{docker: dc}
}

// Health checks that the Docker daemon is reachable.
func (e *Engine) Health(ctx context.Context) error {
	return e.docker.Ping(ctx)
}

// Deploy creates and starts a container for the given request.
// If req.Image is empty the built-in service registry is consulted.
// Port bindings are auto-incremented if the preferred host port is busy.
func (e *Engine) Deploy(ctx context.Context, req DeployRequest) (*DeployResult, error) {
	cfg, err := e.resolveConfig(req)
	if err != nil {
		return nil, err
	}

	exists, err := e.docker.ContainerExists(ctx, cfg.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("container %q already exists — remove it first or use a different service name", cfg.Name)
	}

	resolvedPorts, err := e.resolveHostPorts(ctx, cfg.Ports)
	if err != nil {
		return nil, err
	}
	cfg.Ports = resolvedPorts

	for _, vol := range cfg.Volumes {
		volName := strings.SplitN(vol, ":", 2)[0]
		if err := e.docker.CreateVolume(ctx, volName); err != nil {
			return nil, err
		}
	}

	if err := e.docker.PullImage(ctx, cfg.Image); err != nil {
		return nil, err
	}

	if err := e.docker.RunContainer(ctx, cfg); err != nil {
		return nil, err
	}

	return &DeployResult{
		Name:      req.Name,
		Container: cfg.Name,
		Image:     cfg.Image,
		Ports:     cfg.Ports,
	}, nil
}

// Remove stops and removes the container for the named service.
// volumes is an explicit list of named Docker volumes to also delete.
// An empty slice skips volume removal.
func (e *Engine) Remove(ctx context.Context, name string, volumes []string) error {
	containerName := containerPrefix + name

	exists, err := e.docker.ContainerExists(ctx, containerName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("container %q not found", containerName)
	}

	if err := e.docker.StopContainer(ctx, containerName); err != nil {
		return err
	}
	if err := e.docker.RemoveContainer(ctx, containerName); err != nil {
		return err
	}

	for _, vol := range volumes {
		if vol = strings.TrimSpace(vol); vol == "" {
			continue
		}
		if err := e.docker.RemoveVolume(ctx, vol); err != nil {
			// Non-fatal: container is already gone; log and continue.
			fmt.Printf("warning: could not remove volume %q: %v\n", vol, err)
		}
	}
	return nil
}

// Status returns the runtime state of the named service's container.
// Returns a "not found" status (not an error) when the container does not exist.
func (e *Engine) Status(ctx context.Context, name string) (*ServiceStatus, error) {
	containerName := containerPrefix + name

	exists, err := e.docker.ContainerExists(ctx, containerName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return &ServiceStatus{
			Name:      name,
			Container: containerName,
			State:     "not deployed",
		}, nil
	}

	info, err := e.docker.InspectContainer(ctx, containerName)
	if err != nil {
		return nil, err
	}

	return &ServiceStatus{
		Name:      name,
		Container: containerName,
		Image:     info.Image,
		State:     info.Status,
		Ports:     info.Ports,
		Running:   info.Running,
	}, nil
}

// List returns the runtime state of all dockpilot-managed containers.
func (e *Engine) List(ctx context.Context) ([]ServiceStatus, error) {
	containers, err := e.docker.ListContainers(ctx, containerPrefix)
	if err != nil {
		return nil, err
	}

	result := make([]ServiceStatus, len(containers))
	for i, c := range containers {
		result[i] = ServiceStatus{
			Name:      strings.TrimPrefix(c.Name, containerPrefix),
			Container: c.Name,
			Image:     c.Image,
			State:     c.Status,
			Ports:     c.Ports,
			Running:   c.Running,
		}
	}
	return result, nil
}

// Logs returns the last tail lines of combined stdout+stderr for a service container.
func (e *Engine) Logs(ctx context.Context, name string, tail int) ([]string, error) {
	containerName := containerPrefix + name

	exists, err := e.docker.ContainerExists(ctx, containerName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("container %q not found", containerName)
	}

	return e.docker.ContainerLogs(ctx, containerName, tail)
}

// --- internal helpers --------------------------------------------------------

// resolveConfig builds the final docker.ContainerConfig from a DeployRequest.
// If no image is provided, it falls back to the built-in services registry.
func (e *Engine) resolveConfig(req DeployRequest) (docker.ContainerConfig, error) {
	containerName := containerPrefix + req.Name

	if req.Image != "" {
		return docker.ContainerConfig{
			Name:    containerName,
			Image:   req.Image,
			Ports:   req.Ports,
			Volumes: req.Volumes,
			Env:     req.Env,
			Command: req.Command,
		}, nil
	}

	svc, ok := services.Get(req.Name)
	if !ok {
		return docker.ContainerConfig{}, fmt.Errorf(
			"no image specified and %q is not a built-in service; available: %s",
			req.Name, strings.Join(services.Names(), ", "),
		)
	}

	cfg := docker.ContainerConfig{
		Name:    containerName,
		Image:   svc.Image,
		Volumes: svc.Volumes,
		Command: svc.Command,
	}
	if len(req.Ports) > 0 {
		cfg.Ports = req.Ports
	} else {
		cfg.Ports = svc.Ports
	}
	cfg.Env = append(svc.Env, req.Env...)

	return cfg, nil
}

// resolveHostPorts returns port mappings with free host ports.
// It queries Docker for already-allocated ports (which may not be bound at the
// OS level when Docker uses iptables instead of userland-proxy) and combines
// that with an OS-level listener check. Scans up to +100 from the preferred port.
func (e *Engine) resolveHostPorts(ctx context.Context, ports []string) ([]string, error) {
	dockerPorts := e.dockerAllocatedPorts(ctx)

	resolved := make([]string, len(ports))
	for i, mapping := range ports {
		parts := strings.Split(mapping, ":")
		if len(parts) < 2 {
			resolved[i] = mapping
			continue
		}
		preferredHost := parts[len(parts)-2]
		containerPart := parts[len(parts)-1]

		actualHost, err := findFreePort(preferredHost, dockerPorts)
		if err != nil {
			return nil, fmt.Errorf("cannot find a free port near %s: %w", preferredHost, err)
		}

		if len(parts) == 3 {
			resolved[i] = parts[0] + ":" + actualHost + ":" + containerPart
		} else {
			resolved[i] = actualHost + ":" + containerPart
		}
	}
	return resolved, nil
}

// dockerAllocatedPorts returns the set of host port numbers currently allocated
// by any Docker container. This catches ports held via iptables that have no
// OS-level socket (i.e. when userland-proxy is disabled).
func (e *Engine) dockerAllocatedPorts(ctx context.Context) map[int]bool {
	used := map[int]bool{}
	containers, err := e.docker.ListContainers(ctx, "")
	if err != nil {
		return used // best-effort; fall back to OS-only check
	}
	for _, c := range containers {
		for _, binding := range strings.Split(c.Ports, ", ") {
			// Format: "IP:HostPort->ContainerPort/proto" (e.g. "0.0.0.0:27017->27017/tcp")
			arrowIdx := strings.Index(binding, "->")
			if arrowIdx == -1 {
				continue
			}
			hostPart := binding[:arrowIdx]
			colonIdx := strings.LastIndex(hostPart, ":")
			if colonIdx == -1 {
				continue
			}
			if port, err := strconv.Atoi(hostPart[colonIdx+1:]); err == nil {
				used[port] = true
			}
		}
	}
	return used
}

// findFreePort returns preferred if it is available, otherwise scans upward
// within preferred+100. A port is considered unavailable if it is in excluded
// (Docker-allocated) or cannot be bound at the OS level.
func findFreePort(preferred string, excluded map[int]bool) (string, error) {
	base, err := strconv.Atoi(preferred)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: %w", preferred, err)
	}
	for offset := 0; offset <= 100; offset++ {
		port := base + offset
		if excluded[port] {
			continue
		}
		candidate := strconv.Itoa(port)
		ln, err := net.Listen("tcp", ":"+candidate)
		if err == nil {
			ln.Close()
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free port in range %d–%d", base, base+100)
}
