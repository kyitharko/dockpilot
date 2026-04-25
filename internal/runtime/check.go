package runtime

import (
	"context"
	"fmt"
	"time"

	dockerclient "github.com/docker/docker/client"
)

// CheckDaemon opens a short-lived Docker client, pings the daemon, and closes
// the client before returning. It is called from cmd/root.go's PersistentPreRunE
// so every subcommand gets a clean error before attempting any real work.
//
// Replaces the old IsInstalled() + IsDaemonReachable() pair: there is no longer
// a docker binary to detect, so connectivity is the only meaningful precondition.
func CheckDaemon(ctx context.Context) error {
	cli, err := dockerclient.NewClientWithOpts(
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return fmt.Errorf("cannot create docker client: %w", err)
	}
	defer cli.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if _, err := cli.Ping(pingCtx); err != nil {
		return fmt.Errorf("docker daemon is not reachable — is Docker running? (%w)", err)
	}
	return nil
}
