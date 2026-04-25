package cmd

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"myplatform/internal/runtime"
	"myplatform/internal/services"
	"myplatform/internal/utils"
)

var (
	deployNameFlag  string
	deployImageFlag string
	deployPortFlags []string
	deployEnvFlags  []string
)

var deployCmd = &cobra.Command{
	Use:   "deploy <service|name>",
	Short: "Deploy a managed service or any Docker image",
	Long: `Deploy a service container using pre-defined defaults, or any Docker image from
Docker Hub or a private registry.

Built-in services (no --image needed): ` + strings.Join(services.Names(), ", ") + `

Custom image examples:
  myplatform deploy myapp --image nginx:alpine
  myplatform deploy myapp --image nginx:alpine --port 8080:80
  myplatform deploy myapp --image registry.example.com/myapp:latest --port 443:443 --env DEBUG=1`,
	Args:      cobra.ExactArgs(1),
	ValidArgs: services.Names(),
	RunE:      runDeploy,
}

func init() {
	deployCmd.Flags().StringVarP(
		&deployNameFlag, "name", "n", "",
		"Custom instance name suffix (e.g. --name=dev -> myplatform-myapp-dev)",
	)
	deployCmd.Flags().StringVar(
		&deployImageFlag, "image", "",
		"Docker image to deploy (e.g. nginx:alpine, registry.example.com/myapp:latest)",
	)
	deployCmd.Flags().StringArrayVar(
		&deployPortFlags, "port", nil,
		"Port mapping in host:container form, repeatable (e.g. --port 8080:80)",
	)
	deployCmd.Flags().StringArrayVar(
		&deployEnvFlags, "env", nil,
		"Environment variable in KEY=VALUE form, repeatable (e.g. --env DEBUG=1)",
	)
	rootCmd.AddCommand(deployCmd)
}

func runDeploy(cmd *cobra.Command, args []string) error {
	name := strings.ToLower(args[0])

	var cfg services.ServiceConfig
	if deployImageFlag != "" {
		cfg = services.ServiceConfig{
			Name:          name,
			Image:         deployImageFlag,
			ContainerName: "myplatform-" + name,
			Ports:         deployPortFlags,
			Env:           deployEnvFlags,
		}
	} else {
		var ok bool
		cfg, ok = services.Get(name)
		if !ok {
			return fmt.Errorf(
				"unknown service %q -- built-in services: %s\nTo deploy a custom image use: myplatform deploy %s --image <image>",
				name, strings.Join(services.Names(), ", "), name,
			)
		}
	}

	rt, err := runtime.NewDockerClient()
	if err != nil {
		return fmt.Errorf("connecting to Docker: %w", err)
	}
	defer rt.Close()
	ctx := cmd.Context()

	// Resolve container name: honour --name or auto-increment.
	containerName, err := resolveContainerName(ctx, rt, cfg.ContainerName, deployNameFlag)
	if err != nil {
		return err
	}

	// Derive per-instance volume names.
	volumes := remapVolumeNames(cfg.Volumes, cfg.ContainerName, containerName)

	// Auto-increment host ports if already bound.
	resolvedPorts, err := resolveHostPorts(cfg.Ports)
	if err != nil {
		return err
	}
	for i, orig := range cfg.Ports {
		if orig != resolvedPorts[i] {
			utils.PrintWarning(fmt.Sprintf("Port %s in use, remapped to %s", orig, resolvedPorts[i]))
		}
	}

	deployCfg := cfg
	deployCfg.ContainerName = containerName
	deployCfg.Volumes = volumes
	deployCfg.Ports = resolvedPorts

	if containerName != cfg.ContainerName {
		utils.PrintInfo(fmt.Sprintf("Container name: %q", containerName))
	}

	// Create named volumes (idempotent).
	for _, vol := range deployCfg.Volumes {
		volName := strings.SplitN(vol, ":", 2)[0]
		utils.PrintInfo(fmt.Sprintf("Creating volume %q...", volName))
		if err := rt.CreateVolume(ctx, volName); err != nil {
			return err
		}
	}

	// Pull image — streams layer progress to stdout.
	utils.PrintInfo(fmt.Sprintf("Pulling image %q...", deployCfg.Image))
	if err := rt.PullImage(ctx, deployCfg.Image); err != nil {
		return err
	}

	utils.PrintInfo(fmt.Sprintf("Starting container %q...", deployCfg.ContainerName))
	if err := rt.RunContainer(ctx, deployCfg); err != nil {
		return err
	}

	utils.PrintSuccess(fmt.Sprintf("Service %q deployed -> container %q", name, deployCfg.ContainerName))
	return nil
}

// resolveContainerName returns the container name for the new instance.
// If nameFlag is set it uses base-nameFlag and errors if already taken.
// Otherwise it tries base, base-2, … base-99 until a free slot is found.
func resolveContainerName(ctx context.Context, rt runtime.RuntimeClient, base, nameFlag string) (string, error) {
	if nameFlag != "" {
		candidate := base + "-" + nameFlag
		exists, err := rt.ContainerExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if exists {
			return "", fmt.Errorf("container %q already exists — choose a different --name", candidate)
		}
		return candidate, nil
	}

	for i := 0; i < 99; i++ {
		candidate := base
		if i > 0 {
			candidate = base + "-" + strconv.Itoa(i+1)
		}
		exists, err := rt.ContainerExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("all instance slots for %q are in use (max 99)", base)
}

// resolveHostPorts returns port mappings with free host ports.
// If the preferred host port is busy it scans upward up to +100.
// Supported forms: "hostPort:containerPort", "ip:hostPort:containerPort".
func resolveHostPorts(ports []string) ([]string, error) {
	resolved := make([]string, len(ports))
	for i, mapping := range ports {
		parts := strings.Split(mapping, ":")
		if len(parts) < 2 {
			resolved[i] = mapping
			continue
		}

		preferredHost := parts[len(parts)-2]
		containerPart := parts[len(parts)-1]

		actualHost, err := findFreePort(preferredHost)
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

// findFreePort returns preferred if available, otherwise increments until a
// free TCP port is found within preferred+100.
func findFreePort(preferred string) (string, error) {
	base, err := strconv.Atoi(preferred)
	if err != nil {
		return "", fmt.Errorf("invalid port %q: %w", preferred, err)
	}
	for offset := 0; offset <= 100; offset++ {
		candidate := strconv.Itoa(base + offset)
		ln, err := net.Listen("tcp", ":"+candidate)
		if err == nil {
			ln.Close()
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free port found in range %d-%d", base, base+100)
}
