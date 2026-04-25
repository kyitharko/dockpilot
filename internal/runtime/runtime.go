package runtime

import (
	"context"

	"myplatform/internal/services"
)

// ContainerInfo holds the runtime details of a single container.
type ContainerInfo struct {
	Name    string
	Image   string
	Status  string // human-readable state, e.g. "running" or "exited"
	Ports   string // formatted as "0.0.0.0:27017->27017/tcp, ..."
	Running bool
}

// RuntimeClient abstracts the container runtime.
// DockerSDKClient is the current implementation.
// A future containerd or podman implementation would satisfy this interface
// without touching any code in cmd/.
type RuntimeClient interface {
	PullImage(ctx context.Context, image string) error
	CreateVolume(ctx context.Context, name string) error
	RunContainer(ctx context.Context, cfg services.ServiceConfig) error
	ContainerExists(ctx context.Context, name string) (bool, error)
	InspectContainer(ctx context.Context, name string) (ContainerInfo, error)
	StopContainer(ctx context.Context, name string) error
	RemoveContainer(ctx context.Context, name string) error
	RemoveVolume(ctx context.Context, name string) error
	ListManagedContainers(ctx context.Context) ([]ContainerInfo, error)
	Close() error
}
