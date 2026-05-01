package docker

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/moby/term"
)

// ContainerConfig is the input for running a new container.
type ContainerConfig struct {
	Name    string
	Image   string
	Ports   []string
	Volumes []string
	Env     []string
	Command []string
}

// ContainerInfo holds the runtime details of a single container.
type ContainerInfo struct {
	Name    string
	Image   string
	Status  string
	Ports   string
	Running bool
}

// Client abstracts the Docker daemon. SDKClient is the only implementation;
// the interface exists so the engine layer can be unit-tested with a mock.
type Client interface {
	Ping(ctx context.Context) error
	PullImage(ctx context.Context, ref string) error
	CreateVolume(ctx context.Context, name string) error
	RunContainer(ctx context.Context, cfg ContainerConfig) error
	ContainerExists(ctx context.Context, name string) (bool, error)
	InspectContainer(ctx context.Context, name string) (ContainerInfo, error)
	StopContainer(ctx context.Context, name string) error
	RemoveContainer(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error)
	ContainerLogs(ctx context.Context, name string, tail int) ([]string, error)
	Close() error
}

// SDKClient implements Client using the Docker Engine SDK.
type SDKClient struct {
	cli *dockerclient.Client
}

// NewClient creates a Client from the environment (DOCKER_HOST, DOCKER_TLS_VERIFY,
// DOCKER_CERT_PATH) with automatic API version negotiation.
func NewClient() (Client, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &SDKClient{cli: cli}, nil
}

// CheckDaemon opens a short-lived client, pings the daemon, and closes it.
// Used as a pre-flight check before running any command.
func CheckDaemon(ctx context.Context) error {
	dc, err := NewClient()
	if err != nil {
		return fmt.Errorf("cannot create docker client: %w", err)
	}
	defer dc.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return dc.Ping(pingCtx)
}

func (d *SDKClient) Close() error {
	return d.cli.Close()
}

func (d *SDKClient) Ping(ctx context.Context) error {
	_, err := d.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("docker daemon is not reachable — is Docker running? (%w)", err)
	}
	return nil
}

// PullImage pulls an image and streams JSON progress to stdout.
func (d *SDKClient) PullImage(ctx context.Context, ref string) error {
	rc, err := d.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %q: %w", ref, err)
	}
	defer rc.Close()

	fd, isTerm := term.GetFdInfo(os.Stdout)
	return jsonmessage.DisplayJSONMessagesStream(rc, os.Stdout, fd, isTerm, nil)
}

// CreateVolume creates a named Docker volume (idempotent).
func (d *SDKClient) CreateVolume(ctx context.Context, name string) error {
	_, err := d.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("creating volume %q: %w", name, err)
	}
	return nil
}

// RunContainer creates and starts a detached container.
func (d *SDKClient) RunContainer(ctx context.Context, cfg ContainerConfig) error {
	exposedPorts, portBindings, err := nat.ParsePortSpecs(cfg.Ports)
	if err != nil {
		return fmt.Errorf("parsing port specs for %q: %w", cfg.Name, err)
	}

	createResp, err := d.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image:        cfg.Image,
			Env:          cfg.Env,
			Cmd:          cfg.Command,
			ExposedPorts: nat.PortSet(exposedPorts),
		},
		&container.HostConfig{
			PortBindings: nat.PortMap(portBindings),
			RestartPolicy: container.RestartPolicy{
				Name: container.RestartPolicyUnlessStopped,
			},
			Binds: cfg.Volumes,
		},
		nil, nil, cfg.Name,
	)
	if err != nil {
		return fmt.Errorf("creating container %q: %w", cfg.Name, err)
	}

	if err := d.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container %q: %w", cfg.Name, err)
	}
	return nil
}

// ContainerExists reports whether a container with the exact name exists.
func (d *SDKClient) ContainerExists(ctx context.Context, name string) (bool, error) {
	_, err := d.cli.ContainerInspect(ctx, name)
	if err == nil {
		return true, nil
	}
	if dockerclient.IsErrNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("checking container %q: %w", name, err)
}

// InspectContainer returns runtime details for the named container.
func (d *SDKClient) InspectContainer(ctx context.Context, name string) (ContainerInfo, error) {
	resp, err := d.cli.ContainerInspect(ctx, name)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return ContainerInfo{}, fmt.Errorf("container %q not found", name)
		}
		return ContainerInfo{}, fmt.Errorf("inspecting container %q: %w", name, err)
	}

	var ports []string
	for containerPort, bindings := range resp.NetworkSettings.Ports {
		for _, b := range bindings {
			ports = append(ports, fmt.Sprintf("%s:%s->%s", b.HostIP, b.HostPort, containerPort))
		}
	}

	return ContainerInfo{
		Name:    strings.TrimPrefix(resp.Name, "/"),
		Image:   resp.Config.Image,
		Status:  resp.State.Status,
		Ports:   strings.Join(ports, ", "),
		Running: resp.State.Running,
	}, nil
}

// StopContainer stops a running container with the daemon's default timeout.
func (d *SDKClient) StopContainer(ctx context.Context, name string) error {
	if err := d.cli.ContainerStop(ctx, name, container.StopOptions{}); err != nil {
		return fmt.Errorf("stopping container %q: %w", name, err)
	}
	return nil
}

// RemoveContainer removes a stopped container.
func (d *SDKClient) RemoveContainer(ctx context.Context, name string) error {
	if err := d.cli.ContainerRemove(ctx, name, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("removing container %q: %w", name, err)
	}
	return nil
}

// RemoveVolume removes a named volume. Fails if still in use.
func (d *SDKClient) RemoveVolume(ctx context.Context, name string) error {
	if err := d.cli.VolumeRemove(ctx, name, false); err != nil {
		return fmt.Errorf("removing volume %q: %w", name, err)
	}
	return nil
}

// ListContainers returns all containers whose names begin with prefix.
func (d *SDKClient) ListContainers(ctx context.Context, prefix string) ([]ContainerInfo, error) {
	f := filters.NewArgs(filters.Arg("name", prefix))
	list, err := d.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	result := make([]ContainerInfo, 0, len(list))
	for _, c := range list {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		var ports []string
		for _, p := range c.Ports {
			if p.PublicPort != 0 {
				ports = append(ports, fmt.Sprintf("%s:%d->%d/%s", p.IP, p.PublicPort, p.PrivatePort, p.Type))
			}
		}
		result = append(result, ContainerInfo{
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			Ports:   strings.Join(ports, ", "),
			Running: c.State == "running",
		})
	}
	return result, nil
}

// ContainerLogs returns the last tail lines of combined stdout+stderr for a container.
func (d *SDKClient) ContainerLogs(ctx context.Context, name string, tail int) ([]string, error) {
	rc, err := d.cli.ContainerLogs(ctx, name, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
	})
	if err != nil {
		return nil, fmt.Errorf("fetching logs for %q: %w", name, err)
	}
	defer rc.Close()

	var buf bytes.Buffer
	if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil {
		return nil, fmt.Errorf("reading logs for %q: %w", name, err)
	}

	var lines []string
	scanner := bufio.NewScanner(&buf)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}
