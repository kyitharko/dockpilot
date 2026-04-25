package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/volume"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/go-connections/nat"
	"github.com/moby/term"

	"myplatform/internal/services"
)

// DockerSDKClient implements RuntimeClient using the Docker Engine SDK.
// It communicates directly with the Docker daemon over its Unix socket (or
// TCP when DOCKER_HOST is set), without shelling out to the docker CLI.
type DockerSDKClient struct {
	cli *dockerclient.Client
}

// NewDockerClient creates a client configured from the environment
// (DOCKER_HOST, DOCKER_TLS_VERIFY, DOCKER_CERT_PATH) with automatic API
// version negotiation so the client works against any daemon version.
func NewDockerClient() (*DockerSDKClient, error) {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &DockerSDKClient{cli: cli}, nil
}

// Close releases the underlying HTTP connection pool.
func (d *DockerSDKClient) Close() error {
	return d.cli.Close()
}

// PullImage pulls an image and streams JSON progress to stdout.
// When stdout is a TTY, Docker's layer progress bars are shown.
// When piped (CI), plain text lines are printed instead.
func (d *DockerSDKClient) PullImage(ctx context.Context, ref string) error {
	rc, err := d.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %q: %w", ref, err)
	}
	defer rc.Close()

	fd, isTerm := term.GetFdInfo(os.Stdout)
	return jsonmessage.DisplayJSONMessagesStream(rc, os.Stdout, fd, isTerm, nil)
}

// CreateVolume creates a named Docker volume. The call is idempotent — if the
// volume already exists the daemon returns success without modifying it.
func (d *DockerSDKClient) CreateVolume(ctx context.Context, name string) error {
	_, err := d.cli.VolumeCreate(ctx, volume.CreateOptions{Name: name})
	if err != nil {
		return fmt.Errorf("creating volume %q: %w", name, err)
	}
	return nil
}

// RunContainer creates and starts a detached container from cfg.
// Port specs are parsed by the nat package so "host:container" and
// "ip:host:container" forms are both supported.
func (d *DockerSDKClient) RunContainer(ctx context.Context, cfg services.ServiceConfig) error {
	exposedPorts, portBindings, err := nat.ParsePortSpecs(cfg.Ports)
	if err != nil {
		return fmt.Errorf("parsing port specs for %q: %w", cfg.ContainerName, err)
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
		nil, // default bridge networking
		nil, // inherit daemon platform
		cfg.ContainerName,
	)
	if err != nil {
		return fmt.Errorf("creating container %q: %w", cfg.ContainerName, err)
	}

	if err := d.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting container %q: %w", cfg.ContainerName, err)
	}
	return nil
}

// ContainerExists reports whether a container with the exact name exists
// (running or stopped). Uses ContainerInspect for an exact-name lookup,
// avoiding the substring-match quirk of docker ps --filter name=.
func (d *DockerSDKClient) ContainerExists(ctx context.Context, name string) (bool, error) {
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
func (d *DockerSDKClient) InspectContainer(ctx context.Context, name string) (ContainerInfo, error) {
	resp, err := d.cli.ContainerInspect(ctx, name)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return ContainerInfo{}, fmt.Errorf("container %q not found", name)
		}
		return ContainerInfo{}, fmt.Errorf("inspecting container %q: %w", name, err)
	}

	// resp.NetworkSettings.Ports is nat.PortMap = map[nat.Port][]nat.PortBinding.
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
func (d *DockerSDKClient) StopContainer(ctx context.Context, name string) error {
	if err := d.cli.ContainerStop(ctx, name, container.StopOptions{}); err != nil {
		return fmt.Errorf("stopping container %q: %w", name, err)
	}
	return nil
}

// RemoveContainer removes a stopped container.
func (d *DockerSDKClient) RemoveContainer(ctx context.Context, name string) error {
	if err := d.cli.ContainerRemove(ctx, name, container.RemoveOptions{}); err != nil {
		return fmt.Errorf("removing container %q: %w", name, err)
	}
	return nil
}

// RemoveVolume removes a named volume. Fails if the volume is still in use.
func (d *DockerSDKClient) RemoveVolume(ctx context.Context, name string) error {
	if err := d.cli.VolumeRemove(ctx, name, false); err != nil {
		return fmt.Errorf("removing volume %q: %w", name, err)
	}
	return nil
}

// ListManagedContainers returns all containers whose names begin with
// "myplatform-", both running and stopped.
func (d *DockerSDKClient) ListManagedContainers(ctx context.Context) ([]ContainerInfo, error) {
	f := filters.NewArgs(filters.Arg("name", "myplatform-"))
	list, err := d.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
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
